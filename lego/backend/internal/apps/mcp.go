/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apps

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// cronMCPError keeps Core's stable action code visible when the MCP SDK turns
// an error into text. REST and GraphQL retain the same code structurally.
// mcp.go is the MCP fragment for services. Tool names track Render's official
// MCP server (render-oss/render-mcp-server): list_services / get_service are 1:1;
// the lifecycle verbs (restart/suspend/resume_service) are bex extensions named
// after Render's REST verbs. Every tool delegates to the same Service method
// REST/GraphQL call, so the three surfaces cannot drift.

// serviceArgs is the shared single-service argument. Render's tools key on
// `serviceId` (see get_service); for bex that id is the minted srv-… id
// (opaque, round-tripped from list_services; legacy hand-applied CRs fall
// back to the App name — w1/m46).
type serviceArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id, as returned by list_services"`
}

// listServicesResult wraps the array — MCP tool outputs must be JSON objects.
type listServicesResult struct {
	Services []renderService `json:"services"`
}

type listServicesArgs struct{}

// scaleArgs is scale_service's input — the desired running instance count,
// keyed on numInstances like Render's REST/GraphQL surfaces.
type scaleArgs struct {
	ServiceID    string `json:"serviceId" jsonschema:"the service id, as returned by list_services"`
	NumInstances int32  `json:"numInstances" jsonschema:"the desired number of running instances (1-100)"`
}

// updateServiceArgs is update_service's input: the patch-shaped fold of the
// eighteen per-field service setters w1/m71 retired (set_root_directory,
// set_branch, set_build_command, …). Every settable field is a POINTER, and the
// pointer is the whole contract:
//
//   - absent  => that setting is not touched at all (no write, no build, no roll)
//   - present => that setting is written to exactly this value, INCLUDING the
//     empty value, which is how each old setter cleared a field
//
// Argument names mirror PATCH /v1/services/{id}, and both surfaces reduce to
// the one ordered op table (ApplyServicePatch, settings.go, w1/m78), so the
// same combination behaves identically on both — structurally, now that MCP
// no longer has one tool per field.
//
// This family is a bex invention throughout: upstream ships no update tools for
// any of these fields, and REMOVED its placeholder update_web_service /
// update_static_site / update_cron_job in #89 (2026-07-23) rather than making
// them work. The parity pin classifies update_service as Extension — see
// internal/api/mcp_parity.go.
type updateServiceArgs struct {
	ServiceID   string  `json:"serviceId" jsonschema:"the service id, as returned by list_services"`
	DisplayName *string `json:"displayName,omitempty" jsonschema:"the human-facing service label; empty clears it and falls back to the immutable service name"`
	// Plan is a BILLING change: it resizes the pod and rolls it, and the
	// workspace is charged at the new rate. Folded from update_service_plan
	// (w1/m74) because REST carries it in the same PATCH body; the payment and
	// plan-billing gates in the Service layer are unchanged by the fold.
	Plan *string `json:"plan,omitempty" jsonschema:"the instance plan/size, e.g. starter, standard, pro, pro_plus, pro_max, pro_ultra. Changing it resizes the pod, rolls the service, and CHANGES WHAT THE WORKSPACE IS BILLED. Pass dryRun:true to preview it without any writes"`
	// DryRun mirrors PATCH /v1/services/{id}: it previews a PLAN change with
	// zero writes. Unlike REST, which silently drops the rest of a dry-run body,
	// this refuses a dryRun call carrying any other settable field — an agent
	// that asked to preview a command change should be told the tool cannot,
	// not handed back an unchanged object that implies it did.
	DryRun                  bool                     `json:"dryRun,omitempty" jsonschema:"if true, preview the plan change without any writes (zero side effects). Valid alone or with plan only"`
	IdleTTLSeconds          *int32                   `json:"idleTTLSeconds,omitempty" jsonschema:"seconds a free-tier service may idle before it auto-sleeps; 0 restores the controller default"`
	PublishPath             *string                  `json:"publishPath,omitempty" jsonschema:"static sites only: the built output directory served as the site root, e.g. dist, build, or public"`
	Schedule                *string                  `json:"schedule,omitempty" jsonschema:"cron jobs only: the 5-field crontab expression, e.g. '0 0 * * *'"`
	Command                 *string                  `json:"command,omitempty" jsonschema:"cron jobs only: the command each run executes, overriding the image entrypoint; empty clears the override"`
	Branch                  *string                  `json:"branch,omitempty" jsonschema:"the Git branch to build and deploy; empty restores the default main"`
	RegistryCredentialID    *string                  `json:"registryCredentialId,omitempty" jsonschema:"stored private-registry credential id to bind to an image-backed service or Dockerfile build; empty clears the binding"`
	RootDir                 *string                  `json:"rootDir,omitempty" jsonschema:"subdirectory of the repo to build from (monorepo support); empty builds from the repo root. Triggers a fresh build scoped to that subdirectory"`
	BuildCommand            *string                  `json:"buildCommand,omitempty" jsonschema:"the build command (e.g. npm run build) for static sites and native-runtime services; empty clears it (builder default)"`
	StartCommand            *string                  `json:"startCommand,omitempty" jsonschema:"the command used to start the service — Render's Docker Command for a Docker service, Start Command for a native runtime; empty restores the image default where supported"`
	DockerfilePath          *string                  `json:"dockerfilePath,omitempty" jsonschema:"path to the Dockerfile relative to rootDir; empty restores Dockerfile. Triggers a fresh build"`
	HealthCheckPath         *string                  `json:"healthCheckPath,omitempty" jsonschema:"HTTP path the platform GETs to gate pod readiness and liveness; must start with /. Empty CLEARS the path, switching the service to a TCP check that only verifies the process is listening — the right choice for a service with no cheap 2xx route. Probed every 10s, so point it at a cheap endpoint. No effect on cron_job or background_worker"`
	PreDeployCommand        *string                  `json:"preDeployCommand,omitempty" jsonschema:"a command run to completion against the new revision's image before it serves traffic (typically a database migration); a non-zero exit fails the deploy and leaves the previous revision serving. Empty clears the step. No effect on cron_job or static_site"`
	MaxShutdownDelaySeconds *int32                   `json:"maxShutdownDelaySeconds,omitempty" jsonschema:"seconds after SIGTERM before Kubernetes sends SIGKILL (1-300; default 30); web, private, and background-worker services"`
	AutoDeploy              *bool                    `json:"autoDeploy,omitempty" jsonschema:"true = a signed git push to the tracked branch redeploys (Render's Auto-Deploy); false = only explicit deploys. Setting it does not itself redeploy"`
	BuildFilter             *buildFilterArg          `json:"buildFilter,omitempty" jsonschema:"Render's Build Filters: repository-root-relative globs (paths/ignoredPaths) deciding whether a git push triggers an auto-deploy; ignored wins over included. Pass empty paths and ignoredPaths to clear the filter"`
	NotifyOnFail            *string                  `json:"notifyOnFail,omitempty" jsonschema:"deploy-failure notification override: default (defer to each member's own preference), notify (always email every member on a failed deploy), or ignore (never email anyone for this service). Governs failure mail only"`
	NotificationsToSend     *string                  `json:"notificationsToSend,omitempty" jsonschema:"service notification policy: default (inherit workspace/member preferences, failure-only), failure, all, or none"`
	MaintenanceMode         *maintenanceModeArg      `json:"maintenanceMode,omitempty" jsonschema:"take a web service offline behind an interstitial page without suspending it — pods keep running, every host answers 503. web_service only"`
	RenderSubdomainPolicy   *string                  `json:"renderSubdomainPolicy,omitempty" jsonschema:"enabled (the platform subdomain <slug>.<BEX_BASE_DOMAIN> serves this service) or disabled (platform host dropped; only custom domains serve it — requires at least one custom domain first)"`
	IPAllowList             *[]core.IPAllowListEntry `json:"ipAllowList,omitempty" jsonschema:"replaces the inbound allowlist for a web service or static site with these {cidrBlock, description} entries; pass [] to clear it"`
	IPAllowListCidrs        *[]string                `json:"ipAllowListCidrs,omitempty" jsonschema:"the plain-CIDR-string form of ipAllowList, for callers with no descriptions to keep; setting both to conflicting values is rejected"`
	Autoscaling             *autoscalingArg          `json:"autoscaling,omitempty" jsonschema:"enable or update autoscaling: the operator holds the target utilization by moving replicas within [minInstances, maxInstances]. Use disable_autoscaling to turn it off"`
}

// autoscalingArg is the autoscaling object update_service accepts. It is the
// same shape set_autoscaling took as flat arguments; nesting it keeps the
// enable/update verb distinguishable from a field write in one patch call.
type autoscalingArg struct {
	MinInstances        int32  `json:"minInstances" jsonschema:"minimum running instances (≥ 0; default 1)"`
	MaxInstances        int32  `json:"maxInstances" jsonschema:"maximum running instances (≥ 1; must be ≥ minInstances)"`
	TargetCPUPercent    *int32 `json:"targetCPUPercent,omitempty" jsonschema:"target average CPU utilization % of tier limit (1-100); required if targetMemoryPercent is absent"`
	TargetMemoryPercent *int32 `json:"targetMemoryPercent,omitempty" jsonschema:"target average memory utilization % of tier limit (1-100); required if targetCPUPercent is absent"`
}

// buildFilterArg is Render's Build Filters object, shared by update_service and
// create_web_service: repository-root-relative globs deciding whether a git push
// triggers an auto-deploy. Patterns support *, **, ?, and [class] wildcards.
type buildFilterArg struct {
	Paths        []string `json:"paths,omitempty" jsonschema:"include globs (e.g. 'src/**'); a push deploys only when a changed file matches one — empty means every path is included"`
	IgnoredPaths []string `json:"ignoredPaths,omitempty" jsonschema:"exclude globs (e.g. 'docs/**'); a changed file matching one never triggers a deploy, even if it also matches paths"`
}

// toView converts the tool arg to the neutral view; a nil receiver (absent arg)
// projects as nil so create leaves the filter unset.
func (a *buildFilterArg) toView() *BuildFilterView {
	if a == nil {
		return nil
	}
	return &BuildFilterView{Paths: a.Paths, IgnoredPaths: a.IgnoredPaths}
}

// maintenanceModeArg is Render's maintenanceMode object, shared by
// update_service and create_web_service (docs/render-artifacts/
// maintenance-mode.md): web_service only.
type maintenanceModeArg struct {
	Enabled bool   `json:"enabled" jsonschema:"true takes every host this service serves offline behind an interstitial page (pods keep running); false restores normal serving"`
	URI     string `json:"uri,omitempty" jsonschema:"an absolute http(s) URL to a custom maintenance page, fetched and served in place of the default page; empty uses the default page"`
}

// toView converts the tool arg to the neutral view; a nil receiver (absent
// arg) projects as nil so create leaves maintenanceMode unset (disabled).
func (a *maintenanceModeArg) toView() *MaintenanceModeView {
	if a == nil {
		return nil
	}
	return &MaintenanceModeView{Enabled: a.Enabled, URI: a.URI}
}

// createWebServiceArgs is create_web_service's input — Render's MCP tool name.
// name/repo/branch/plan/envVars track Render's tool; image/port/replicas are bex
// extensions (Render's tool is git-only and has no port/replicas). One of
// repo/image is required. Runtime/buildCommand/startCommand use Render's native
// contract; builder and image are bex extensions. Region remains a one-region
// platform concern and is intentionally absent.
type createWebServiceArgs struct {
	OwnerID                 string                  `json:"-"`
	EnvironmentID           string                  `json:"environmentId,omitempty" jsonschema:"an environment id (env-...) in the target workspace; assignment also joins its project"`
	Name                    string                  `json:"name" jsonschema:"the service name (a DNS label, 1-30 chars)"`
	Type                    string                  `json:"type,omitempty" jsonschema:"service type: web_service (default), private_service, or background_worker. Use create_cron_job for a cron_job"`
	Repo                    string                  `json:"repo,omitempty" jsonschema:"git repository URL to build from (build-from-git); omit if using image"`
	Image                   string                  `json:"image,omitempty" jsonschema:"a prebuilt OCI image to run directly; omit if using repo"`
	RegistryCredentialID    *string                 `json:"registryCredentialId,omitempty" jsonschema:"stored registry credential id for a private prebuilt image or Dockerfile FROM; omit for automatic image-host matching, empty to explicitly use none"`
	Branch                  string                  `json:"branch,omitempty" jsonschema:"branch to track when building from a repo (default main)"`
	RootDir                 string                  `json:"rootDir,omitempty" jsonschema:"subdirectory of the repo to build from, for monorepos (default the repo root)"`
	BuildFilter             *buildFilterArg         `json:"buildFilter,omitempty" jsonschema:"Render's Build Filters: glob patterns (paths/ignoredPaths) gating git-push auto-deploys; omit for no filter"`
	Runtime                 string                  `json:"runtime" jsonschema:"Render runtime: node, python, go, rust, ruby, elixir, or docker"`
	BuildCommand            string                  `json:"buildCommand" jsonschema:"command used to build a native-runtime service; ignored for docker"`
	StartCommand            string                  `json:"startCommand" jsonschema:"command used to start a native-runtime service; ignored for docker"`
	DockerfilePath          string                  `json:"dockerfilePath,omitempty" jsonschema:"path to the Dockerfile, relative to rootDir; only applies when runtime is docker (default Dockerfile)"`
	Builder                 string                  `json:"builder,omitempty" jsonschema:"repo build strategy: auto (default), buildpack, or dockerfile"`
	Plan                    string                  `json:"plan,omitempty" jsonschema:"instance plan, e.g. free, starter, standard, pro, pro_plus, pro_max, pro_ultra (default free)"`
	EnvVars                 []envVarInput           `json:"envVars,omitempty" jsonschema:"literal (non-secret) environment variables to set on the service"`
	SecretFiles             []secretFileInput       `json:"secretFiles,omitempty" jsonschema:"secret files mounted under /etc/secrets from first boot"`
	AutoDeploy              string                  `json:"autoDeploy,omitempty" jsonschema:"redeploy on a git push to the branch: yes or no (default yes for a repo)"`
	NotifyOnFail            string                  `json:"notifyOnFail,omitempty" jsonschema:"deploy-failure notification override: default (defer to each member's own preference), notify (always email every member), or ignore (never email anyone for this service); default if omitted"`
	HealthCheckPath         string                  `json:"healthCheckPath,omitempty" jsonschema:"HTTP path the platform GETs to gate pod readiness (spec.healthCheckPath); must start with / or be empty to use the platform default /"`
	MaxShutdownDelaySeconds *int32                  `json:"maxShutdownDelaySeconds,omitempty" jsonschema:"maximum seconds to wait after SIGTERM before SIGKILL (1-300; default 30)"`
	PreDeployCommand        string                  `json:"preDeployCommand,omitempty" jsonschema:"a command run to completion against the new image before it serves traffic (Render's Pre-Deploy Command, e.g. a DB migration); a non-zero exit fails the deploy"`
	MaintenanceMode         *maintenanceModeArg     `json:"maintenanceMode,omitempty" jsonschema:"Render's maintenanceMode object at create time; web_service only, omit for disabled"`
	Port                    int32                   `json:"port,omitempty" jsonschema:"the port the app listens on (default 3000; ignored for a background_worker)"`
	Replicas                int32                   `json:"replicas,omitempty" jsonschema:"desired running instances (default 1)"`
	DryRun                  bool                    `json:"dryRun,omitempty" jsonschema:"if true, return the resolved spec preview without any writes — zero side effects (w2/m29)"`
	IPAllowList             []string                `json:"ipAllowList,omitempty" jsonschema:"CIDR blocks to restrict inbound HTTP to (e.g. '203.0.113.0/24'); empty = open to all source IPs (Render default). Only applies to web_service and static_site."`
	IPAllowListEntries      []core.IPAllowListEntry `json:"ipAllowListEntries,omitempty" jsonschema:"description-preserving allowlist entries as {cidrBlock, description}; use instead of ipAllowList"`
}

type listCronJobRunsArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the cron job service id, as returned by list_services"`
	Limit     int    `json:"limit,omitempty" jsonschema:"page size, 1-100 (default 10)"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"resume after this cursor; omit for the first page"`
}

type cronJobRunArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the cron job service id, as returned by list_services"`
	RunID     string `json:"runId" jsonschema:"the crr- run id, as returned by list_cron_job_runs"`
}

type listCronJobRunsResult struct {
	CronJobRuns []renderCronJobRun `json:"cronJobRuns"`
	Cursor      string             `json:"cursor"`
}

func (a createWebServiceArgs) toCreateRequest() CreateRequest {
	return CreateRequest{
		OwnerID:                 a.OwnerID,
		EnvironmentID:           a.EnvironmentID,
		Name:                    a.Name,
		Type:                    a.Type,
		Repo:                    a.Repo,
		Image:                   a.Image,
		RegistryCredentialID:    clonePtr(a.RegistryCredentialID),
		Branch:                  a.Branch,
		RootDir:                 a.RootDir,
		BuildFilter:             a.BuildFilter.toView(),
		Runtime:                 a.Runtime,
		BuildCommand:            a.BuildCommand,
		StartCommand:            a.StartCommand,
		DockerfilePath:          a.DockerfilePath,
		Builder:                 a.Builder,
		Plan:                    a.Plan,
		Env:                     toEnvVars(a.EnvVars),
		SecretFiles:             toSecretFiles(a.SecretFiles),
		AutoDeploy:              parseYesNo(a.AutoDeploy),
		NotifyOnFail:            a.NotifyOnFail,
		HealthCheckPath:         a.HealthCheckPath,
		MaxShutdownDelaySeconds: clonePtr(a.MaxShutdownDelaySeconds),
		PreDeployCommand:        a.PreDeployCommand,
		MaintenanceMode:         a.MaintenanceMode.toView(),
		Port:                    a.Port,
		Replicas:                a.Replicas,
		DryRun:                  a.DryRun,
		IPAllowList:             core.AllowListFromCIDRs(a.IPAllowList),
	}
}

// createCronJobArgs is create_cron_job's input — Render's MCP tool name. It
// tracks create_web_service but requires a schedule and has no port/replicas
// (a cron runs its command to completion on the schedule, not as a server).
type createCronJobArgs struct {
	OwnerID              string            `json:"-"`
	EnvironmentID        string            `json:"environmentId,omitempty" jsonschema:"an environment id (env-...) in the target workspace; assignment also joins its project"`
	Name                 string            `json:"name" jsonschema:"the cron job name (a DNS label, 1-30 chars)"`
	Schedule             string            `json:"schedule" jsonschema:"the cron schedule (standard 5-field crontab, e.g. '0 * * * *')"`
	Command              string            `json:"command,omitempty" jsonschema:"overrides the image's default entrypoint for each run, e.g. 'npm run report'; omit to run the image's own command"`
	Repo                 string            `json:"repo,omitempty" jsonschema:"git repository URL to build from (build-from-git); omit if using image"`
	Image                string            `json:"image,omitempty" jsonschema:"a prebuilt OCI image to run directly; omit if using repo"`
	RegistryCredentialID *string           `json:"registryCredentialId,omitempty" jsonschema:"stored registry credential id for a private prebuilt image or Dockerfile FROM; omit for automatic image-host matching, empty to explicitly use none"`
	Branch               string            `json:"branch,omitempty" jsonschema:"branch to track when building from a repo (default main)"`
	RootDir              string            `json:"rootDir,omitempty" jsonschema:"subdirectory of the repo to build from, for monorepos (default the repo root)"`
	Runtime              string            `json:"runtime" jsonschema:"Render runtime: node, python, go, rust, ruby, elixir, or docker"`
	BuildCommand         string            `json:"buildCommand" jsonschema:"command used to build a native-runtime cron job; ignored for docker"`
	StartCommand         string            `json:"startCommand" jsonschema:"command run by the native-runtime cron job; ignored for docker"`
	DockerfilePath       string            `json:"dockerfilePath,omitempty" jsonschema:"path to the Dockerfile, relative to rootDir; only applies when runtime is docker (default Dockerfile)"`
	Builder              string            `json:"builder,omitempty" jsonschema:"repo build strategy: auto (default), buildpack, or dockerfile"`
	Plan                 string            `json:"plan,omitempty" jsonschema:"instance plan, e.g. free, starter, standard, pro (default free)"`
	EnvVars              []envVarInput     `json:"envVars,omitempty" jsonschema:"literal (non-secret) environment variables to set on the job"`
	SecretFiles          []secretFileInput `json:"secretFiles,omitempty" jsonschema:"secret files mounted under /etc/secrets from first boot"`
	AutoDeploy           string            `json:"autoDeploy,omitempty" jsonschema:"redeploy on a git push to the branch: yes or no (default yes for a repo)"`
	NotifyOnFail         string            `json:"notifyOnFail,omitempty" jsonschema:"deploy-failure notification override: default (defer to each member's own preference), notify (always email every member), or ignore (never email anyone for this service); default if omitted"`
	DryRun               bool              `json:"dryRun,omitempty" jsonschema:"if true, return the resolved spec preview without any writes — zero side effects (w2/m29)"`
}

func (a createCronJobArgs) toCreateRequest() CreateRequest {
	return CreateRequest{
		OwnerID:              a.OwnerID,
		EnvironmentID:        a.EnvironmentID,
		Name:                 a.Name,
		Type:                 appv1alpha1.TypeCronJob,
		Schedule:             a.Schedule,
		Command:              a.Command,
		Repo:                 a.Repo,
		Image:                a.Image,
		RegistryCredentialID: clonePtr(a.RegistryCredentialID),
		Branch:               a.Branch,
		RootDir:              a.RootDir,
		Runtime:              a.Runtime,
		BuildCommand:         a.BuildCommand,
		StartCommand:         a.StartCommand,
		DockerfilePath:       a.DockerfilePath,
		Builder:              a.Builder,
		Plan:                 a.Plan,
		Env:                  toEnvVars(a.EnvVars),
		SecretFiles:          toSecretFiles(a.SecretFiles),
		AutoDeploy:           parseYesNo(a.AutoDeploy),
		NotifyOnFail:         a.NotifyOnFail,
		DryRun:               a.DryRun,
	}
}

// deployArgs is the deploy tool's input: a repo + its render.yaml Blueprint. Deploy-from-chat
// is create with a manifest — one agent call takes code (one service or a whole
// stack) to live URLs.
type deployArgs struct {
	Repo    string `json:"repo,omitempty" jsonschema:"git repository URL to deploy (overrides the repo in bexYaml, if any)"`
	Branch  string `json:"branch,omitempty" jsonschema:"branch to deploy (overrides the branch in bexYaml, if any)"`
	BexYAML string `json:"bexYaml" jsonschema:"the project's render.yaml Blueprint manifest. May declare a whole stack: services: (web/worker/cron) + databases:, wired by fromDatabase env references. One call converges all of it; validation is all-or-nothing; re-applying an unchanged file is a no-op"`
	Confirm string `json:"confirm,omitempty" jsonschema:"required only when this call would change an EXISTING service that belongs to a protectedStatus=protected Environment (w6/m19): the exact phrase from the error message of a first, unconfirmed call"`
}

// renderStack is the deploy tool's result for a multi-resource render.yaml: the
// services + databases one deploy call created (databases applied first, then
// services — dependents reference databases via fromDatabase). A single-service
// render.yaml returns a one-element services list and no databases. Poll each
// service to a live URL via get_service; poll databases via get_postgres.
type renderStack struct {
	Services  []renderService     `json:"services"`
	Databases []StackDatabaseView `json:"databases,omitempty"`
	KeyValues []StackKeyValueView `json:"keyValues,omitempty"`
}

// toRenderStack maps a StackResult onto the MCP deploy result shape.
func toRenderStack(res StackResult) renderStack {
	return renderStack{Services: toRenderServices(res.Services), Databases: res.Databases, KeyValues: res.KeyValues}
}

// validateBlueprintArgs is validate_bex_yml's input; the wire name remains
// compatible while the Blueprint filename contract is render.yaml.
type validateBlueprintArgs struct {
	BexYAML string `json:"bexYaml" jsonschema:"the render.yaml content to validate; parsed and checked for per-entry errors with no apply (the wire field name is retained for compatibility)"`
}

type listBlueprintsArgs struct{}

// listBlueprintsResult wraps the array — MCP tool outputs must be JSON objects.
type listBlueprintsResult struct {
	Blueprints []BlueprintView `json:"blueprints"`
}

// getBlueprintArgs is get_blueprint's input (w2/m41).
type getBlueprintArgs struct {
	ID string `json:"id" jsonschema:"the blueprint id (blp-…), as returned by list_blueprints or a prior deploy call"`
}

// syncBlueprintArgs is sync_blueprint's input (w2/m15).
type syncBlueprintArgs struct {
	ID      string `json:"id" jsonschema:"the blueprint id (blp-…), as returned by list_blueprints or a prior deploy call"`
	BexYAML string `json:"bexYaml,omitempty" jsonschema:"optional updated render.yaml content to store and apply; omit to re-apply the stored manifest unchanged"`
	Confirm string `json:"confirm,omitempty" jsonschema:"exact confirmation phrase returned by a protected-environment error when the sync overrides an existing service"`
}

// previewBlueprintArgs is preview_blueprint's input.
type previewBlueprintArgs struct {
	Repo   string `json:"repo" jsonschema:"Git repo URL (https://github.com/org/repo)"`
	Branch string `json:"branch" jsonschema:"branch holding render.yaml"`
	Path   string `json:"path,omitempty" jsonschema:"path to a Blueprint within the repo (default render.yaml)"`
}

// generateBlueprintArgs is generate_blueprint's input (w8/m22).
type generateBlueprintArgs struct {
	ServiceIDs  []string `json:"serviceIds,omitempty" jsonschema:"service ids (srv-…) to export"`
	PostgresIDs []string `json:"postgresIds,omitempty" jsonschema:"Postgres ids (dpg-…) to export"`
	KeyValueIDs []string `json:"keyValueIds,omitempty" jsonschema:"Key Value ids (red-…) to export"`
}

// createBlueprintArgs is create_blueprint's input (w2/m62).
type createBlueprintArgs struct {
	Repo         string            `json:"repo" jsonschema:"Git repo URL (https://github.com/org/repo)"`
	Branch       string            `json:"branch" jsonschema:"branch to track"`
	Path         string            `json:"path,omitempty" jsonschema:"path to a Blueprint within the repo (default render.yaml)"`
	Name         string            `json:"name,omitempty" jsonschema:"human-readable name (default: repo basename)"`
	EnvVarValues map[string]string `json:"envVarValues,omitempty" jsonschema:"values for sync:false Blueprint env-var prompts; never returned"`
	Confirm      string            `json:"confirm,omitempty" jsonschema:"confirmation phrase for protected-environment overrides"`
}

// listBlueprintSyncsArgs is list_blueprint_syncs's input (w2/m62).
type listBlueprintSyncsArgs struct {
	ID     string `json:"id" jsonschema:"blueprint id (blp-…)"`
	Cursor string `json:"cursor,omitempty" jsonschema:"opaque cursor from a prior call for pagination"`
	Limit  int    `json:"limit,omitempty" jsonschema:"max runs to return (1–100, default 20)"`
}

// listBlueprintSyncsResult wraps the array.
type listBlueprintSyncsResult struct {
	Syncs []BlueprintSyncView `json:"syncs"`
}

// updateBlueprintArgs is update_blueprint's input (w2/m62).
type updateBlueprintArgs struct {
	ID       string  `json:"id" jsonschema:"blueprint id (blp-…)"`
	Name     *string `json:"name,omitempty" jsonschema:"new display name"`
	AutoSync *bool   `json:"autoSync,omitempty" jsonschema:"enable or disable auto-sync on push"`
	Path     *string `json:"path,omitempty" jsonschema:"new Blueprint path within the repo"`
}

// disconnectedBlueprintResult is disconnect_blueprint's output.
type disconnectedBlueprintResult struct {
	Disconnected bool `json:"disconnected" jsonschema:"true when the blueprint was disconnected"`
}

// disconnectBlueprintArgs is disconnect_blueprint's input (w2/m62).
type disconnectBlueprintArgs struct {
	ID string `json:"id" jsonschema:"blueprint id (blp-…) to disconnect"`
}

// domainArgs is the shared custom-domain argument (serviceId + domain name).
type domainArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id, as returned by list_services"`
	Name      string `json:"name" jsonschema:"the custom domain FQDN, e.g. www.example.com"`
}

// listCustomDomainsArgs extends serviceId with the pagination + filter params
// w7/m40 added to match REST (w7/m38).
type listCustomDomainsArgs struct {
	ServiceID          string `json:"serviceId" jsonschema:"the service id, as returned by list_services"`
	Cursor             string `json:"cursor,omitempty" jsonschema:"resume after this cursor; omit for the first page"`
	Limit              int    `json:"limit,omitempty" jsonschema:"page size, 1-100 (default 20)"`
	VerificationStatus string `json:"verificationStatus,omitempty" jsonschema:"filter by verification status: unverified/pending or verified"`
	DomainType         string `json:"domainType,omitempty" jsonschema:"filter by domain type: apex or subdomain"`
}

// staticRouteArg / staticHeaderArg are the MCP shapes of a static_site's edge
// rules — Render's route (type/source/destination) and header (path/name/value).
type staticRouteArg struct {
	Type        string `json:"type" jsonschema:"redirect (301 to destination) or rewrite (serve destination's content with 200)"`
	Source      string `json:"source" jsonschema:"request path pattern to match, e.g. /old or /app/* (trailing /* is a wildcard)"`
	Destination string `json:"destination" jsonschema:"target path, e.g. /new or /index.html; :splat or a trailing /* substitutes the wildcard capture"`
}

type staticHeaderArg struct {
	Path  string `json:"path" jsonschema:"request path pattern the header applies to, e.g. /* or /assets/*"`
	Name  string `json:"name" jsonschema:"response header name, e.g. X-Frame-Options"`
	Value string `json:"value" jsonschema:"response header value, e.g. DENY"`
}

// createStaticSiteArgs is create_static_site's input — Render's MCP tool name.
// A static site builds a repo and serves its publishPath output from the
// object-store origin (no running container). publishPath is required; routes and
// headers are the optional edge rules.
type createStaticSiteArgs struct {
	OwnerID            string                  `json:"-"`
	EnvironmentID      string                  `json:"environmentId,omitempty" jsonschema:"an environment id (env-...) in the target workspace; assignment also joins its project"`
	Name               string                  `json:"name" jsonschema:"the static site name (a DNS label, 1-30 chars)"`
	Repo               string                  `json:"repo,omitempty" jsonschema:"git repository URL to build from; omit if using image"`
	Image              string                  `json:"image,omitempty" jsonschema:"a prebuilt OCI image whose publishPath holds the built site; omit if using repo"`
	Branch             string                  `json:"branch,omitempty" jsonschema:"branch to track when building from a repo (default main)"`
	RootDir            string                  `json:"rootDir,omitempty" jsonschema:"subdirectory of the repo to build from, for monorepos (default the repo root)"`
	PublishPath        string                  `json:"publishPath" jsonschema:"the built output directory to serve as the site root, e.g. dist, build, or public"`
	EnvVars            []envVarInput           `json:"envVars,omitempty" jsonschema:"literal (non-secret) build-time environment variables"`
	SecretFiles        []secretFileInput       `json:"secretFiles,omitempty" jsonschema:"secret files available to the static-site build from first boot"`
	Domains            []string                `json:"domains,omitempty" jsonschema:"custom domains to serve the site at, in addition to the platform hostname"`
	Routes             []staticRouteArg        `json:"routes,omitempty" jsonschema:"ordered redirect/rewrite rules (first match wins), e.g. an SPA fallback rewrite of /* to /index.html"`
	Headers            []staticHeaderArg       `json:"headers,omitempty" jsonschema:"custom response-header rules scoped by request path"`
	IPAllowList        []string                `json:"ipAllowList,omitempty" jsonschema:"legacy CIDR allowlist; use ipAllowListEntries to preserve descriptions"`
	IPAllowListEntries []core.IPAllowListEntry `json:"ipAllowListEntries,omitempty" jsonschema:"description-preserving allowlist entries as {cidrBlock, description}"`
	DryRun             bool                    `json:"dryRun,omitempty" jsonschema:"if true, return the resolved spec preview without any writes — zero side effects (w2/m29)"`
}

func (a createStaticSiteArgs) toCreateRequest() CreateRequest {
	return CreateRequest{
		OwnerID:       a.OwnerID,
		EnvironmentID: a.EnvironmentID,
		Name:          a.Name,
		Type:          appv1alpha1.TypeStaticSite,
		Repo:          a.Repo,
		Image:         a.Image,
		Branch:        a.Branch,
		RootDir:       a.RootDir,
		PublishPath:   a.PublishPath,
		Env:           toEnvVars(a.EnvVars),
		SecretFiles:   toSecretFiles(a.SecretFiles),
		Hosts:         a.Domains,
		Routes:        routeArgViews(a.Routes),
		Headers:       headerArgViews(a.Headers),
		IPAllowList:   core.AllowListFromCIDRs(a.IPAllowList),
		DryRun:        a.DryRun,
	}
}

// routesArgs / headersArgs are the static-site edge-rule tool inputs; each tool
// replaces the whole list (Render's bulk update, and REST's own PUT routes —
// which is why w1/m74 left them standalone while folding publishPath, a plain
// PATCH field, into update_service).
type routesArgs struct {
	ServiceID string           `json:"serviceId" jsonschema:"the static site id, as returned by list_services"`
	Routes    []staticRouteArg `json:"routes" jsonschema:"the full ordered list of redirect/rewrite rules to set (replaces the existing routes)"`
}

type headersArgs struct {
	ServiceID string            `json:"serviceId" jsonschema:"the static site id, as returned by list_services"`
	Headers   []staticHeaderArg `json:"headers" jsonschema:"the full list of custom response-header rules to set (replaces the existing headers)"`
}

// routesResult / headersResult wrap the arrays — MCP tool outputs must be objects.
type routesResult struct {
	Routes []StaticRouteView `json:"routes"`
}

type headersResult struct {
	Headers []StaticHeaderView `json:"headers"`
}

func routeArgViews(in []staticRouteArg) []StaticRouteView {
	if len(in) == 0 {
		return nil
	}
	out := make([]StaticRouteView, len(in))
	for i, r := range in {
		out[i] = StaticRouteView{Type: r.Type, Source: r.Source, Destination: r.Destination}
	}
	return out
}

func headerArgViews(in []staticHeaderArg) []StaticHeaderView {
	if len(in) == 0 {
		return nil
	}
	out := make([]StaticHeaderView, len(in))
	for i, h := range in {
		out[i] = StaticHeaderView{Path: h.Path, Name: h.Name, Value: h.Value}
	}
	return out
}

// domainListResult wraps the array — MCP tool outputs must be JSON objects.
// Cursor is the last item's domain name; omit to get the next page.
type domainListResult struct {
	CustomDomains []renderCustomDomain `json:"customDomains"`
	Cursor        string               `json:"cursor,omitempty"`
}

// deletedResult is delete_custom_domain's return object.
type deletedResult struct {
	Deleted bool `json:"deleted"`
}

// serviceConfirmArgs is serviceArgs plus an optional confirm — used by
// delete_service/suspend_service, the two lifecycle verbs w6/m19's protected-
// environment guard (apps.ProtectedConfirmation) can block. confirm is
// ignored (harmless) when the service isn't a member of a protected
// Environment.
type serviceConfirmArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id, as returned by list_services"`
	Confirm   string `json:"confirm,omitempty" jsonschema:"required only if this service belongs to a protectedStatus=protected Environment: the exact phrase from the error message of a first, unconfirmed call"`
}

// RegisterMCP adds the service and custom-domain tools to the shared MCP server.
// The tools group into five independent families; each registers its own so no
// one function carries the whole surface.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	s.registerServiceTools(srv)
	s.registerAutoscalingTools(srv)
	s.registerCustomDomainTools(srv)
	s.registerStaticSiteTools(srv)
	s.registerBlueprintTools(srv)
	s.registerDiskTools(srv)
	s.registerDiskSnapshotTools(srv)
}

func (s *Service) registerServiceTools(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_services",
		Description: "List all services (bex Apps) in a workspace with their status.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ listServicesArgs) (*mcp.CallToolResult, listServicesResult, error) {
		apps, err := s.List(ctx, core.NamedWorkspace(ctx))
		if err != nil {
			return nil, listServicesResult{}, err
		}
		return nil, listServicesResult{Services: toRenderServices(apps)}, nil
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_service",
		Description: "Get details about a specific service by id.",
	}, s.serviceTool(s.Get))

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "create_web_service",
		Description: "Create a web service from a repo or a prebuilt image and get back the service to poll until its url is live. A name already used in the target workspace is rejected (name already in use) rather than redeployed — use restart_service to redeploy an existing one. Tracks Render's MCP tool.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createWebServiceArgs) (*mcp.CallToolResult, renderService, error) {
		in.OwnerID = core.NamedWorkspace(ctx)
		return s.createWithAllowList(ctx, in.toCreateRequest(), in.IPAllowListEntries, in.IPAllowList)
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "create_cron_job",
		Description: "Create a cron job that runs a repo/image's command on a schedule, and get back the service. A name already used in the target workspace is rejected (name already in use) rather than redeployed — use restart_service to redeploy an existing one. Tracks Render's MCP tool.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createCronJobArgs) (*mcp.CallToolResult, renderService, error) {
		in.OwnerID = core.NamedWorkspace(ctx)
		return renderServiceResult(s.Create(ctx, in.toCreateRequest()))
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "create_static_site",
		Description: "Create a static site: build a repo and serve its publishPath output from the object-store origin (no running container). Redirects/rewrites (routes) and custom response headers apply at the edge. A name already used in the target workspace is rejected (name already in use) rather than republished — use restart_service to republish an existing one. Tracks Render's MCP tool.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createStaticSiteArgs) (*mcp.CallToolResult, renderService, error) {
		in.OwnerID = core.NamedWorkspace(ctx)
		return s.createWithAllowList(ctx, in.toCreateRequest(), in.IPAllowListEntries, in.IPAllowList)
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "run_cron_job",
		Description: "Trigger a one-off run of a cron job now, canceling an active run first like Render. Returns the deterministic pending run. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, renderCronJobRun, error) {
		run, err := s.TriggerCronRun(ctx, in.ServiceID)
		return nil, toRenderCronJobRun(run), err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_cron_job_runs",
		Description: "bex extension: list a cron job's runs newest first. Returns up to limit runs (default 10) plus a cursor to pass to the next call.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listCronJobRunsArgs) (*mcp.CallToolResult, listCronJobRunsResult, error) {
		limit := in.Limit
		if limit < 1 {
			limit = 10
		}
		runs, err := s.ListCronRuns(ctx, in.ServiceID, in.Cursor, limit)
		if err != nil {
			return nil, listCronJobRunsResult{}, err
		}
		out := listCronJobRunsResult{CronJobRuns: make([]renderCronJobRun, len(runs))}
		for i, run := range runs {
			out.CronJobRuns[i] = toRenderCronJobRun(run)
		}
		if len(runs) > 0 {
			out.Cursor = runs[len(runs)-1].ID
		}
		return nil, out, nil
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_cron_job_run",
		Description: "bex extension: get one cron job run by its crr- id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in cronJobRunArgs) (*mcp.CallToolResult, renderCronJobRun, error) {
		run, err := s.GetCronRun(ctx, in.ServiceID, in.RunID)
		return nil, toRenderCronJobRun(run), err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "cancel_cron_job_run",
		Description: "bex extension: cancel one pending cron job run by crr- id. The operator terminates its Kubernetes Job; a terminal run returns a conflict instead of silently succeeding.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in cronJobRunArgs) (*mcp.CallToolResult, renderCronJobRun, error) {
		run, err := s.CancelCronRun(ctx, in.ServiceID, in.RunID)
		return nil, toRenderCronJobRun(run), err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "deploy",
		Description: "Deploy a project from a git repo and render.yaml content in one call. A Blueprint may declare a whole stack — several services (web/worker/cron) plus managed databases, wired by fromDatabase env references — and one call converges all of it, databases first. Validation is all-or-nothing: one invalid entry rejects the whole deploy. Re-applying unchanged content is an idempotent no-op (changed services redeploy, unchanged ones don't). Returns the services (poll each to a live url via get_service) and databases (poll via get_postgres). A change to an EXISTING service that belongs to a protectedStatus=protected Environment (w6/m19) requires confirm — retry with the phrase from the error message. bex extension (pillar 4, deploy-from-chat at stack scale).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deployArgs) (*mcp.CallToolResult, renderStack, error) {
		res, err := s.DeployStack(ctx, DeployRequest{Repo: in.Repo, Branch: in.Branch, Manifest: in.BexYAML, Confirm: in.Confirm})
		if err != nil {
			return nil, renderStack{}, err
		}
		return nil, toRenderStack(res), nil
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "delete_service",
		Description: "Delete a service permanently, cascading everything the operator derived from it (Deployment, Service, Ingress). This is irreversible. A member of a protectedStatus=protected Environment (w6/m19) refuses without confirm — retry with the phrase from the error message. bex extension over Render's MCP (Render's official server ships no delete tool), named after the REST delete verb.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceConfirmArgs) (*mcp.CallToolResult, deletedResult, error) {
		err := s.Delete(core.WithConfirm(ctx, in.Confirm), in.ServiceID)
		return nil, deletedResult{Deleted: err == nil}, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "restart_service",
		Description: "Restart a service (rolling restart, no downtime). bex extension over Render's MCP.",
	}, s.serviceTool(s.Restart))

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "suspend_service",
		Description: "Suspend a service: scale to zero, keeping host and certificates. A member of a protectedStatus=protected Environment (w6/m19) refuses without confirm — retry with the phrase from the error message. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceConfirmArgs) (*mcp.CallToolResult, renderService, error) {
		return renderServiceResult(s.Suspend(core.WithConfirm(ctx, in.Confirm), in.ServiceID))
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "resume_service",
		Description: "Resume a suspended service, restoring its replicas. bex extension over Render's MCP.",
	}, s.serviceTool(s.Resume))

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "scale_service",
		Description: "Scale a service to a specific number of running instances (numInstances, 1-100). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in scaleArgs) (*mcp.CallToolResult, renderService, error) {
		return renderServiceResult(s.Scale(ctx, in.ServiceID, in.NumInstances))
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "update_service",
		Description: "Update a service's settings in one call. Pass only the settings you want to change: an omitted argument is left exactly as it is, and a present argument is written to exactly the value given — including the empty value, which is how you clear a command, a path, or a list. Covers source (branch, registryCredentialId), build (rootDir, buildCommand, startCommand, dockerfilePath, buildFilter), runtime (startCommand, healthCheckPath, preDeployCommand, maxShutdownDelaySeconds, maintenanceMode, autoscaling), delivery (autoDeploy), naming (displayName), networking (renderSubdomainPolicy, ipAllowList), and notifications (notifyOnFail, notificationsToSend). rootDir and dockerfilePath trigger a fresh build. Static sites also take publishPath here; cron jobs take schedule and command. A plan change is billable — pass dryRun:true to preview it (valid alone or with plan only). Verbs REST keeps behind their own routes keep their own tools: scale_service (instance count), update_static_routes / update_static_headers (edge rules), disable_autoscaling. This tool replaces the retired set_* setters (w1/m71) plus update_service_plan / update_idle_timeout / update_publish_path / update_cron_job (w1/m74). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateServiceArgs) (*mcp.CallToolResult, renderService, error) {
		return renderServiceResult(s.applyServicePatch(ctx, in))
	})

}

// nonPlanFields names the settable arguments present besides plan, so a dryRun
// call that cannot honour them fails with a message that says which ones.
func (in updateServiceArgs) nonPlanFields() string {
	present := map[string]bool{
		"displayName":             in.DisplayName != nil,
		"branch":                  in.Branch != nil,
		"registryCredentialId":    in.RegistryCredentialID != nil,
		"rootDir":                 in.RootDir != nil,
		"buildCommand":            in.BuildCommand != nil,
		"startCommand":            in.StartCommand != nil,
		"dockerfilePath":          in.DockerfilePath != nil,
		"healthCheckPath":         in.HealthCheckPath != nil,
		"preDeployCommand":        in.PreDeployCommand != nil,
		"maxShutdownDelaySeconds": in.MaxShutdownDelaySeconds != nil,
		"autoDeploy":              in.AutoDeploy != nil,
		"buildFilter":             in.BuildFilter != nil,
		"notifyOnFail":            in.NotifyOnFail != nil,
		"notificationsToSend":     in.NotificationsToSend != nil,
		"maintenanceMode":         in.MaintenanceMode != nil,
		"renderSubdomainPolicy":   in.RenderSubdomainPolicy != nil,
		"ipAllowList":             in.IPAllowList != nil,
		"ipAllowListCidrs":        in.IPAllowListCidrs != nil,
		"autoscaling":             in.Autoscaling != nil,
		"idleTTLSeconds":          in.IdleTTLSeconds != nil,
		"publishPath":             in.PublishPath != nil,
		"schedule":                in.Schedule != nil,
		"command":                 in.Command != nil,
	}
	names := make([]string, 0, len(present))
	for name, ok := range present {
		if ok {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

// applyServicePatch maps update_service's tool arguments onto the neutral
// ServicePatch and delegates to ApplyServicePatch — the single ordered op
// table PATCH /v1/services/{id} shares (settings.go, w1/m78) — so a
// multi-field call behaves identically on both surfaces by construction, not
// by parallel maintenance. A call with no settable field is a read-only no-op
// that reflects current state, exactly as the REST handler does.
func (s *Service) applyServicePatch(ctx context.Context, in updateServiceArgs) (AppView, error) {
	allowList, err := core.ResolveAllowListPatch(in.IPAllowList, in.IPAllowListCidrs)
	if err != nil {
		return AppView{}, err
	}

	// Dry run previews the PLAN change and writes nothing — PATCH
	// /v1/services/{id}'s rule. Anything else in the same call is refused rather
	// than silently dropped (the one deliberate divergence from REST here).
	if in.DryRun {
		if other := in.nonPlanFields(); other != "" {
			return AppView{}, fmt.Errorf("%w: dryRun previews a plan change only; remove %s or drop dryRun", core.ErrBadRequest, other)
		}
		if in.Plan == nil {
			return s.Get(ctx, in.ServiceID) // no plan to preview => reflect current state
		}
		return s.PreviewSetPlan(ctx, in.ServiceID, *in.Plan)
	}

	p := ServicePatch{
		DisplayName: in.DisplayName,
		// REST-only (w1/073 routing): Repo/Image/ImageOwnerID — Render's
		// PATCH source object. update_service has no repo/image argument;
		// rest.go's toServicePatch fills them. Branch + registryCredentialId
		// still apply here.
		Branch:               in.Branch,
		RegistryCredentialID: in.RegistryCredentialID,
		// Same reorder REST arms: disable-maintenance + free downgrade must
		// apply maintenance first (SetPlan refuses a free plan while
		// maintenance is still on). The condition lives in the core table.
		MaintenanceBeforeFreeDowngrade: true,
		MaintenanceMode:                in.MaintenanceMode.toView(),
		Plan:                           in.Plan,
		IdleTTLSeconds:                 in.IdleTTLSeconds,
		MaxShutdownDelaySeconds:        in.MaxShutdownDelaySeconds,
		RootDir:                        in.RootDir,
		BuildFilter:                    in.BuildFilter.toView(),
		AutoDeploy:                     in.AutoDeploy,
		Schedule:                       in.Schedule,
		Command:                        in.Command,
		HealthCheckPath:                in.HealthCheckPath,
		PreDeployCommand:               in.PreDeployCommand,
		PublishPath:                    in.PublishPath,
		BuildCommand:                   in.BuildCommand,
		StartCommand:                   in.StartCommand,
		DockerfilePath:                 in.DockerfilePath,
		NotifyOnFail:                   in.NotifyOnFail,
		// MCP-only (divergence): notificationsToSend is an update_service
		// argument Render's PATCH body has no spelling for; REST's
		// toServicePatch leaves it nil.
		NotificationsToSend:   in.NotificationsToSend,
		RenderSubdomainPolicy: in.RenderSubdomainPolicy,
		IPAllowList:           allowList,
	}
	// MCP-only (divergence): autoscaling is an update_service argument REST
	// keeps behind its own PUT /v1/services/{id}/autoscaling route, so REST's
	// toServicePatch leaves it nil.
	if in.Autoscaling != nil {
		p.Autoscaling = &SetAutoscalingRequest{
			MinInstances:        in.Autoscaling.MinInstances,
			MaxInstances:        in.Autoscaling.MaxInstances,
			TargetCPUPercent:    in.Autoscaling.TargetCPUPercent,
			TargetMemoryPercent: in.Autoscaling.TargetMemoryPercent,
		}
	}
	return s.ApplyServicePatch(ctx, in.ServiceID, p)
}

// registerAutoscalingTools tracks Render's PUT/DELETE .../autoscaling contract.
func (s *Service) registerAutoscalingTools(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_autoscaling",
		Description: "Get the autoscaling configuration for a service (minInstances, maxInstances, targetCPUPercent, targetMemoryPercent). Returns enabled:false when autoscaling is not configured. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, AutoscalingView, error) {
		av, err := s.GetAutoscaling(ctx, in.ServiceID)
		if err != nil {
			return nil, AutoscalingView{}, err
		}
		return nil, av, nil
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "disable_autoscaling",
		Description: "Disable autoscaling for a service, reverting it to its fixed spec.replicas count. Tracks Render's DELETE /v1/services/{id}/autoscaling.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, deletedResult, error) {
		err := s.DeleteAutoscaling(ctx, in.ServiceID)
		return nil, deletedResult{Deleted: err == nil}, err
	})

}

// registerCustomDomainTools tracks render-oss/render-mcp-server tool names.
func (s *Service) registerCustomDomainTools(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_custom_domains",
		Description: "List custom domains configured for a service. Optional verificationStatus (unverified|pending|verified; pending is the bex alias) and domainType (apex|subdomain) filters narrow the result; cursor/limit page it (default 20 per page).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listCustomDomainsArgs) (*mcp.CallToolResult, domainListResult, error) {
		domains, err := s.ListDomains(ctx, in.ServiceID)
		if err != nil {
			return nil, domainListResult{}, err
		}
		domains, err = filterDomains(domains, in.VerificationStatus, in.DomainType)
		if err != nil {
			return nil, domainListResult{}, err
		}
		limit := core.PageLimitOrDefault(in.Limit)
		page := core.StablePage(domains, in.Cursor, limit, in.Cursor != "" || in.Limit != 0,
			func(d DomainView) string { return d.Name })
		out := make([]renderCustomDomain, 0, len(page))
		for _, d := range page {
			out = append(out, toRenderCustomDomain(d))
		}
		result := domainListResult{CustomDomains: out}
		if len(page) > 0 {
			result.Cursor = page[len(page)-1].Name
		}
		return nil, result, nil
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_custom_domain",
		Description: "Get details about a specific custom domain on a service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in domainArgs) (*mcp.CallToolResult, renderCustomDomain, error) {
		return customDomainResult(s.GetDomain(ctx, in.ServiceID, in.Name))
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "add_custom_domain",
		Description: "Create a pending custom-domain claim. The result includes the exact ownership TXT record; the domain is not routed until verify_custom_domain promotes it.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in domainArgs) (*mcp.CallToolResult, renderCustomDomain, error) {
		return customDomainResult(s.AddDomain(ctx, in.ServiceID, in.Name))
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "delete_custom_domain",
		Description: "Remove a custom domain from a service. The operator will remove the Ingress rule and let the TLS certificate expire.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in domainArgs) (*mcp.CallToolResult, deletedResult, error) {
		err := s.DeleteDomain(ctx, in.ServiceID, in.Name)
		return nil, deletedResult{Deleted: err == nil}, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "verify_custom_domain",
		Description: "Check a pending custom domain's exact ownership TXT record and atomically promote it into serving intent. Already-verified claims are idempotent.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in domainArgs) (*mcp.CallToolResult, renderCustomDomain, error) {
		return customDomainResult(s.VerifyDomain(ctx, in.ServiceID, in.Name))
	})

}

// registerStaticSiteTools adds the static-site edge-rule tools. Render's
// official MCP ships only a non-functional update_static_site stub; bex makes
// routes/headers/publishPath real, delegating to the same Service verbs
// REST/GraphQL use.
func (s *Service) registerStaticSiteTools(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_static_routes",
		Description: "List a static site's redirect/rewrite rules (in order, first match wins).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, routesResult, error) {
		routes, err := s.ListRoutes(ctx, in.ServiceID)
		if err != nil {
			return nil, routesResult{}, err
		}
		return nil, routesResult{Routes: toRenderRoutes(routes)}, nil
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "update_static_routes",
		Description: "Replace a static site's redirect/rewrite rules with the given ordered list (Render's routes). The change takes effect without a rebuild. Rejected for a non-static-site service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in routesArgs) (*mcp.CallToolResult, routesResult, error) {
		app, err := s.SetRoutes(ctx, in.ServiceID, routeArgViews(in.Routes))
		if err != nil {
			return nil, routesResult{}, err
		}
		return nil, routesResult{Routes: toRenderRoutes(app.Routes)}, nil
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_static_headers",
		Description: "List a static site's custom response-header rules.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, headersResult, error) {
		headers, err := s.ListHeaders(ctx, in.ServiceID)
		if err != nil {
			return nil, headersResult{}, err
		}
		return nil, headersResult{Headers: toRenderHeaders(headers)}, nil
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "update_static_headers",
		Description: "Replace a static site's custom response-header rules with the given list (Render's headers). The change takes effect without a rebuild. Rejected for a non-static-site service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in headersArgs) (*mcp.CallToolResult, headersResult, error) {
		app, err := s.SetHeaders(ctx, in.ServiceID, headerArgViews(in.Headers))
		if err != nil {
			return nil, headersResult{}, err
		}
		return nil, headersResult{Headers: toRenderHeaders(app.Headers)}, nil
	})

}

// registerBlueprintTools adds the Blueprint verbs (w2/m15 + w2/m41 + w2/m62).
func (s *Service) registerBlueprintTools(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "validate_bex_yml",
		Description: "Dry-run parse a render.yaml Blueprint and return structured per-entry errors plus a resource plan without applying anything — the safe pre-flight check before a deploy call. bex.yml remains a filename-only alias. Returns {valid, errors: [{code?, error, line?, column?, path?}], plan?, estimatedPricing?: {totalUsd, lines, variable}} — the pricing object is the always-on monthly cost projection on bex's price sheet (free tiers filtered; cron/autoscaling/multi-instance listed as variable, excluded from the total). Requires no store; always available. bex extension (pillar 4 agent safety).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in validateBlueprintArgs) (*mcp.CallToolResult, BlueprintValidation, error) {
		v, err := s.ValidateBlueprint(ctx, core.NamedWorkspace(ctx), in.BexYAML)
		return nil, v, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "preview_blueprint",
		Description: "Fetch a repo's render.yaml and dry-run validate it WITHOUT creating anything — the pre-flight for create_blueprint. Empty path discovers render.yaml first, then the legacy bex.yml alias (with a warning); both files require an explicit path. Returns {found, manifest?, commitId?, warning?, error?, validation?: {valid, errors, plan, estimatedPricing?}}; estimatedPricing is the always-on monthly cost projection on bex's price sheet. A missing file reports found=false with the fetch error instead of failing. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in previewBlueprintArgs) (*mcp.CallToolResult, BlueprintPreview, error) {
		p, err := s.PreviewBlueprint(ctx, core.NamedWorkspace(ctx), in.Repo, in.Branch, in.Path)
		return nil, p, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "generate_blueprint",
		Description: "Export selected existing resources (services, Postgres, Key Value) as a render.yaml Blueprint manifest — Render's dashboard-only Generate Blueprint as an API. Env vars keep literal values; secret-backed ones emit sync:false name-only (no secret value is ever included); datastore wiring emits fromDatabase/fromService references when the target is in the same selection. The manifest is self-validated against bex's own Blueprint contract before it is returned, and creating it as a blueprint against a repo adopts the same resources by name. bex extension (Render has no generate API).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in generateBlueprintArgs) (*mcp.CallToolResult, GenerateBlueprintResult, error) {
		out, err := s.GenerateBlueprint(ctx, GenerateBlueprintRequest{
			OwnerID:     core.NamedWorkspace(ctx),
			ServiceIDs:  in.ServiceIDs,
			PostgresIDs: in.PostgresIDs,
			KeyValueIDs: in.KeyValueIDs,
		})
		return nil, out, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "create_blueprint",
		Description: "Create a Git-connected Blueprint by fetching render.yaml from a repo, validating, and applying the full stack. bex.yml is a filename-only alias; if both files exist, specify path explicitly. Returns the new blueprint and deployed resources. The repo must be accessible via the workspace's GitHub connection or be public. bex extension (w2/m62).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createBlueprintArgs) (*mcp.CallToolResult, BlueprintView, error) {
		view, err := s.CreateBlueprint(ctx, core.NamedWorkspace(ctx), CreateBlueprintRequest{
			Repo:         in.Repo,
			Branch:       in.Branch,
			Path:         in.Path,
			Name:         in.Name,
			EnvVarValues: in.EnvVarValues,
			Confirm:      in.Confirm,
		})
		return nil, view, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_blueprints",
		Description: "List Git-connected Blueprint instances for a workspace. Returns {blueprints: [{id, name, repo, branch, path, autoSync, status, lastSync, createdAt, updatedAt}]}. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ listBlueprintsArgs) (*mcp.CallToolResult, listBlueprintsResult, error) {
		views, err := s.ListBlueprints(ctx, core.NamedWorkspace(ctx))
		return nil, listBlueprintsResult{Blueprints: views}, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_blueprint",
		Description: "Get a single blueprint by its id, including managed resources (id/name/type). bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getBlueprintArgs) (*mcp.CallToolResult, BlueprintView, error) {
		view, err := s.GetBlueprintByID(ctx, in.ID, core.NamedWorkspace(ctx))
		return nil, view, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_blueprint_syncs",
		Description: "List sync run history for a blueprint, newest first. Each run records state (running/success/error), commitId, startedAt, and completedAt. bex extension (w2/m62).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listBlueprintSyncsArgs) (*mcp.CallToolResult, listBlueprintSyncsResult, error) {
		syncs, err := s.ListBlueprintSyncs(ctx, in.ID, core.NamedWorkspace(ctx), in.Cursor, in.Limit)
		return nil, listBlueprintSyncsResult{Syncs: syncs}, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "sync_blueprint",
		Description: "Re-apply a Blueprint by pulling the latest render.yaml from its Git repo (or from the stored manifest if no fetcher is configured). Records a sync run. Returns {blueprint, stack: {services, databases}}. bex extension (pillar 4, validate-then-deploy flow).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in syncBlueprintArgs) (*mcp.CallToolResult, SyncBlueprintResult, error) {
		res, err := s.SyncBlueprint(ctx, in.ID, core.NamedWorkspace(ctx), in.BexYAML, in.Confirm)
		return nil, res, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "update_blueprint",
		Description: "Update a Blueprint's name, autoSync flag, or render.yaml path. Setting autoSync=false pauses auto-sync on push; true re-enables it. bex extension (w2/m62).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateBlueprintArgs) (*mcp.CallToolResult, BlueprintView, error) {
		view, err := s.UpdateBlueprint(ctx, in.ID, core.NamedWorkspace(ctx), UpdateBlueprintRequest{
			Name:     in.Name,
			AutoSync: in.AutoSync,
			Path:     in.Path,
		})
		return nil, view, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "disconnect_blueprint",
		Description: "Disconnect a Blueprint from its Git repo: stops auto-sync on push and hides it from list_blueprints. Resources created by the blueprint remain untouched; empty project/environment groupings the blueprint minted (no member services or datastores) are reclaimed (w8/m20). bex extension (w2/m62).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in disconnectBlueprintArgs) (*mcp.CallToolResult, disconnectedBlueprintResult, error) {
		err := s.DisconnectBlueprint(ctx, in.ID, core.NamedWorkspace(ctx))
		return nil, disconnectedBlueprintResult{Disconnected: err == nil}, err
	})
}

// serviceTool adapts a single-service verb (Get/Restart/Suspend/Resume) into an
// MCP tool handler returning the Render service object — the same mapping REST's
// verb handlers use, so the surfaces stay identical.
func (s *Service) serviceTool(fn func(context.Context, string) (AppView, error)) mcp.ToolHandlerFor[serviceArgs, renderService] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, renderService, error) {
		return renderServiceResult(fn(ctx, in.ServiceID))
	}
}

// createWithAllowList is the shared tail of the create tools that accept an IP
// allow list: reconcile Render's two spellings (a nil slice means the caller
// omitted that spelling) onto the request, then create.
func (s *Service) createWithAllowList(ctx context.Context, req CreateRequest, entries []core.IPAllowListEntry, cidrs []string) (*mcp.CallToolResult, renderService, error) {
	allowList, err := core.ResolveAllowListInputs(entries, entries != nil, cidrs, cidrs != nil)
	if err != nil {
		return nil, renderService{}, err
	}
	req.IPAllowList = allowList
	return renderServiceResult(s.Create(ctx, req))
}

// renderServiceResult adapts a service verb's (AppView, error) return into the
// MCP tool result shape — serviceTool's value-level sibling for handlers that
// take extra arguments.
func renderServiceResult(app AppView, err error) (*mcp.CallToolResult, renderService, error) {
	if err != nil {
		return nil, renderService{}, err
	}
	return nil, toRenderService(app), nil
}

// customDomainResult is renderServiceResult's twin for the domain verbs.
func customDomainResult(d DomainView, err error) (*mcp.CallToolResult, renderCustomDomain, error) {
	if err != nil {
		return nil, renderCustomDomain{}, err
	}
	return nil, toRenderCustomDomain(d), nil
}
