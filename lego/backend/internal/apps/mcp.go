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

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// mcp.go is the MCP fragment for services. Tool names track Render's official
// MCP server (render-oss/render-mcp-server): list_services / get_service are 1:1;
// the lifecycle verbs (restart/suspend/resume_service) are bex extensions named
// after Render's REST verbs. Every tool delegates to the same Service method
// REST/GraphQL call, so the three surfaces cannot drift.

// serviceArgs is the shared single-service argument. Render's tools key on
// `serviceId` (see get_service); for bex that id is the App name (opaque,
// round-tripped from list_services).
type serviceArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
}

// listServicesResult wraps the array — MCP tool outputs must be JSON objects.
type listServicesResult struct {
	Services []renderService `json:"services"`
}

// listServicesArgs is list_services' input — the ownerId scoping filter
// (w6/m2/t004), mirroring the REST/GraphQL surfaces. Empty => unscoped.
type listServicesArgs struct {
	OwnerID string `json:"ownerId,omitempty" jsonschema:"restrict the list to this workspace id (tea-…); omit to use the session's selected workspace, if any"`
}

// updatePlanArgs is update_service_plan's input — Render's plan spelling
// (e.g. "pro_plus"), same as the REST/GraphQL surfaces.
type updatePlanArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Plan      string `json:"plan" jsonschema:"the new instance plan, e.g. starter, standard, pro, pro_plus, pro_max, pro_ultra"`
	DryRun    bool   `json:"dryRun,omitempty" jsonschema:"if true, return the resolved spec preview without any writes — zero side effects (w2/m29)"`
}

// scaleArgs is scale_service's input — the desired running instance count,
// keyed on numInstances like Render's REST/GraphQL surfaces.
type scaleArgs struct {
	ServiceID    string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	NumInstances int32  `json:"numInstances" jsonschema:"the desired number of running instances (1-100)"`
}

// idleTimeoutArgs is update_idle_timeout's input — the free-tier auto-sleep
// window in seconds (0 = controller default). A bex extension, no Render tool.
type idleTimeoutArgs struct {
	ServiceID      string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	IdleTTLSeconds int32  `json:"idleTTLSeconds" jsonschema:"seconds a free-tier service may idle before it auto-sleeps; 0 restores the controller default"`
}

// rootDirArgs is set_root_directory's input — Render's Root Directory setting:
// the subdirectory of a build-from-git service's repo to build from.
type rootDirArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	RootDir   string `json:"rootDir" jsonschema:"subdirectory of the repo to build from; empty builds from the repo root"`
}

// startCommandArgs and dockerfilePathArgs expose the two remaining Build &
// Deploy command/path settings through the same scalar-setter grammar as
// set_root_directory. Render's official MCP has no update tools for either.
type startCommandArgs struct {
	ServiceID    string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	StartCommand string `json:"startCommand" jsonschema:"the command used to start the service; empty restores the image default where supported"`
}

type dockerfilePathArgs struct {
	ServiceID      string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	DockerfilePath string `json:"dockerfilePath" jsonschema:"path to the Dockerfile relative to rootDir; empty restores Dockerfile"`
}

type registryCredentialArgs struct {
	ServiceID            string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	RegistryCredentialID string `json:"registryCredentialId" jsonschema:"the registry credential id to bind; empty clears the binding"`
}

// buildFilterArg is Render's Build Filters object, shared by set_build_filter and
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

// buildFilterArgs is set_build_filter's input — Render's Build Filters setting.
type buildFilterArgs struct {
	ServiceID   string         `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	BuildFilter buildFilterArg `json:"buildFilter" jsonschema:"the glob patterns gating git-push auto-deploys; pass empty paths and ignoredPaths to clear the filter"`
}

// maintenanceModeArg is Render's maintenanceMode object, shared by
// set_maintenance_mode and create_web_service (docs/render-artifacts/
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

// maintenanceModeArgs is set_maintenance_mode's input.
type maintenanceModeArgs struct {
	ServiceID       string             `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	MaintenanceMode maintenanceModeArg `json:"maintenanceMode" jsonschema:"the maintenanceMode object to set"`
}

// updateCronJobArgs is update_cron_job's input — bex's functional implementation
// of the verb Render ships as a non-functional stub. schedule is the 5-field
// crontab expression (required); command is a pointer so nil means "keep the
// existing override" and an empty string means "clear it."
type updateCronJobArgs struct {
	ServiceID string  `json:"serviceId" jsonschema:"the cron job id (bex App name), as returned by list_services"`
	Schedule  string  `json:"schedule" jsonschema:"the new cron schedule (5-field crontab, e.g. '0 0 * * *'); required"`
	Command   *string `json:"command,omitempty" jsonschema:"overrides the image's default entrypoint for each run, e.g. 'npm run report'; omit to keep the existing override, empty string to clear it"`
}

// autoDeployArgs is set_auto_deploy's input — Render's Auto-Deploy toggle.
type autoDeployArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Enabled   bool   `json:"enabled" jsonschema:"true = a git push to the tracked branch redeploys; false = only explicit deploys"`
}

// notifyOnFailArgs is set_notify_on_fail's input — Render's per-service
// deploy-failure notification override (docs/render-artifacts/notify-on-fail.md).
type notifyOnFailArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Value     string `json:"value" jsonschema:"default (defer to each member's own preference), notify (always email every member on a failed deploy), or ignore (never email anyone for this service)"`
}

type notificationsToSendArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Value     string `json:"value" jsonschema:"default (inherit failure-only workspace/member settings), failure, all, or none"`
}

// displayNameArgs is set_display_name's input. The service id remains the
// immutable App name; displayName is only the mutable human-facing label.
type displayNameArgs struct {
	ServiceID   string `json:"serviceId" jsonschema:"the immutable service id (bex App name), as returned by list_services"`
	DisplayName string `json:"displayName" jsonschema:"the human-facing service label; empty clears it and falls back to the immutable service name"`
}

// healthCheckPathArgs is set_health_check_path's input.
type healthCheckPathArgs struct {
	ServiceID       string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	HealthCheckPath string `json:"healthCheckPath" jsonschema:"HTTP path the platform GETs to gate pod readiness; must start with / or be empty to restore the platform default /"`
}

// maxShutdownDelayArgs is set_max_shutdown_delay's input. Render's official
// MCP currently has no setter for this REST/Blueprint field, so the tool is a
// bex extension following the existing scalar setter grammar.
type maxShutdownDelayArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Seconds   int32  `json:"seconds" jsonschema:"maximum seconds to wait after SIGTERM before SIGKILL; must be 1-300"`
}

// preDeployCommandArgs is set_pre_deploy_command's input.
type preDeployCommandArgs struct {
	ServiceID        string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	PreDeployCommand string `json:"preDeployCommand" jsonschema:"a command run to completion against the new image before it serves traffic (Render's Pre-Deploy Command, e.g. a DB migration); empty clears the step"`
}

// subdomainPolicyArgs is set_subdomain_policy's input — Render's
// renderSubdomainPolicy field (enabled|disabled).
type subdomainPolicyArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Policy    string `json:"policy" jsonschema:"enabled (platform subdomain <slug>.onbex.co is active) or disabled (platform host dropped; only custom domains serve the App)"`
}

// serviceIPAllowListArgs is set_service_ip_allow_list's input.
type serviceIPAllowListArgs struct {
	ServiceID string   `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	CIDRs     []string `json:"cidrs,omitempty" jsonschema:"CIDR blocks to allow (e.g. '1.2.3.4/32'); empty or null clears the allowlist (open to all source IPs)"`
}

// createWebServiceArgs is create_web_service's input — Render's MCP tool name.
// name/repo/branch/plan/envVars track Render's tool; image/port/replicas are bex
// extensions (Render's tool is git-only and has no port/replicas). One of
// repo/image is required. Runtime/buildCommand/startCommand use Render's native
// contract; builder and image are bex extensions. Region remains a one-region
// platform concern and is intentionally absent.
type createWebServiceArgs struct {
	OwnerID                 string              `json:"ownerId,omitempty" jsonschema:"the workspace to create in (an owner id, tea-...); omit to use the workspace selected with select_workspace, else your default workspace"`
	EnvironmentID           string              `json:"environmentId,omitempty" jsonschema:"an environment id (env-...) in the target workspace; assignment also joins its project"`
	Name                    string              `json:"name" jsonschema:"the service name (a DNS label, 1-30 chars)"`
	Type                    string              `json:"type,omitempty" jsonschema:"service type: web_service (default), private_service, or background_worker. Use create_cron_job for a cron_job"`
	Repo                    string              `json:"repo,omitempty" jsonschema:"git repository URL to build from (build-from-git); omit if using image"`
	Image                   string              `json:"image,omitempty" jsonschema:"a prebuilt OCI image to run directly; omit if using repo"`
	RegistryCredentialID    *string             `json:"registryCredentialId,omitempty" jsonschema:"stored registry credential id for a private prebuilt image; omit for automatic host matching, empty to explicitly use none"`
	Branch                  string              `json:"branch,omitempty" jsonschema:"branch to track when building from a repo (default main)"`
	RootDir                 string              `json:"rootDir,omitempty" jsonschema:"subdirectory of the repo to build from, for monorepos (default the repo root)"`
	BuildFilter             *buildFilterArg     `json:"buildFilter,omitempty" jsonschema:"Render's Build Filters: glob patterns (paths/ignoredPaths) gating git-push auto-deploys; omit for no filter"`
	Runtime                 string              `json:"runtime" jsonschema:"Render runtime: node, python, go, rust, ruby, elixir, or docker"`
	BuildCommand            string              `json:"buildCommand" jsonschema:"command used to build a native-runtime service; ignored for docker"`
	StartCommand            string              `json:"startCommand" jsonschema:"command used to start a native-runtime service; ignored for docker"`
	DockerfilePath          string              `json:"dockerfilePath,omitempty" jsonschema:"path to the Dockerfile, relative to rootDir; only applies when runtime is docker (default Dockerfile)"`
	Builder                 string              `json:"builder,omitempty" jsonschema:"repo build strategy: auto (default), buildpack, or dockerfile"`
	Plan                    string              `json:"plan,omitempty" jsonschema:"instance plan, e.g. free, starter, standard, pro, pro_plus, pro_max, pro_ultra (default free)"`
	EnvVars                 []envVarArg         `json:"envVars,omitempty" jsonschema:"literal (non-secret) environment variables to set on the service"`
	SecretFiles             []secretFileArg     `json:"secretFiles,omitempty" jsonschema:"secret files mounted under /etc/secrets from first boot"`
	AutoDeploy              string              `json:"autoDeploy,omitempty" jsonschema:"redeploy on a git push to the branch: yes or no (default yes for a repo)"`
	NotifyOnFail            string              `json:"notifyOnFail,omitempty" jsonschema:"deploy-failure notification override: default (defer to each member's own preference), notify (always email every member), or ignore (never email anyone for this service); default if omitted"`
	HealthCheckPath         string              `json:"healthCheckPath,omitempty" jsonschema:"HTTP path the platform GETs to gate pod readiness (spec.healthCheckPath); must start with / or be empty to use the platform default /"`
	MaxShutdownDelaySeconds *int32              `json:"maxShutdownDelaySeconds,omitempty" jsonschema:"maximum seconds to wait after SIGTERM before SIGKILL (1-300; default 30)"`
	PreDeployCommand        string              `json:"preDeployCommand,omitempty" jsonschema:"a command run to completion against the new image before it serves traffic (Render's Pre-Deploy Command, e.g. a DB migration); a non-zero exit fails the deploy"`
	MaintenanceMode         *maintenanceModeArg `json:"maintenanceMode,omitempty" jsonschema:"Render's maintenanceMode object at create time; web_service only, omit for disabled"`
	Port                    int32               `json:"port,omitempty" jsonschema:"the port the app listens on (default 3000; ignored for a background_worker)"`
	Replicas                int32               `json:"replicas,omitempty" jsonschema:"desired running instances (default 1)"`
	DryRun                  bool                `json:"dryRun,omitempty" jsonschema:"if true, return the resolved spec preview without any writes — zero side effects (w2/m29)"`
	IPAllowList             []string            `json:"ipAllowList,omitempty" jsonschema:"CIDR blocks to restrict inbound HTTP to (e.g. '203.0.113.0/24'); empty = open to all source IPs (Render default). Only applies to web_service and static_site."`
}

// envVarArg is Render's {key, value} env-var shape, shared by the create tool.
type envVarArg struct {
	Key   string `json:"key" jsonschema:"the environment variable name"`
	Value string `json:"value" jsonschema:"the literal value"`
}

type secretFileArg struct {
	Name    string `json:"name" jsonschema:"file name beneath /etc/secrets"`
	Content string `json:"content" jsonschema:"secret file contents"`
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
		RegistryCredentialID:    cloneStringPtr(a.RegistryCredentialID),
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
		MaxShutdownDelaySeconds: cloneInt32(a.MaxShutdownDelaySeconds),
		PreDeployCommand:        a.PreDeployCommand,
		MaintenanceMode:         a.MaintenanceMode.toView(),
		Port:                    a.Port,
		Replicas:                a.Replicas,
		DryRun:                  a.DryRun,
		IPAllowList:             a.IPAllowList,
	}
}

// createCronJobArgs is create_cron_job's input — Render's MCP tool name. It
// tracks create_web_service but requires a schedule and has no port/replicas
// (a cron runs its command to completion on the schedule, not as a server).
type createCronJobArgs struct {
	OwnerID              string          `json:"ownerId,omitempty" jsonschema:"the workspace to create in (an owner id, tea-...); omit to use the workspace selected with select_workspace, else your default workspace"`
	EnvironmentID        string          `json:"environmentId,omitempty" jsonschema:"an environment id (env-...) in the target workspace; assignment also joins its project"`
	Name                 string          `json:"name" jsonschema:"the cron job name (a DNS label, 1-30 chars)"`
	Schedule             string          `json:"schedule" jsonschema:"the cron schedule (standard 5-field crontab, e.g. '0 * * * *')"`
	Command              string          `json:"command,omitempty" jsonschema:"overrides the image's default entrypoint for each run, e.g. 'npm run report'; omit to run the image's own command"`
	Repo                 string          `json:"repo,omitempty" jsonschema:"git repository URL to build from (build-from-git); omit if using image"`
	Image                string          `json:"image,omitempty" jsonschema:"a prebuilt OCI image to run directly; omit if using repo"`
	RegistryCredentialID *string         `json:"registryCredentialId,omitempty" jsonschema:"stored registry credential id for a private prebuilt image; omit for automatic host matching, empty to explicitly use none"`
	Branch               string          `json:"branch,omitempty" jsonschema:"branch to track when building from a repo (default main)"`
	RootDir              string          `json:"rootDir,omitempty" jsonschema:"subdirectory of the repo to build from, for monorepos (default the repo root)"`
	Runtime              string          `json:"runtime" jsonschema:"Render runtime: node, python, go, rust, ruby, elixir, or docker"`
	BuildCommand         string          `json:"buildCommand" jsonschema:"command used to build a native-runtime cron job; ignored for docker"`
	StartCommand         string          `json:"startCommand" jsonschema:"command run by the native-runtime cron job; ignored for docker"`
	DockerfilePath       string          `json:"dockerfilePath,omitempty" jsonschema:"path to the Dockerfile, relative to rootDir; only applies when runtime is docker (default Dockerfile)"`
	Builder              string          `json:"builder,omitempty" jsonschema:"repo build strategy: auto (default), buildpack, or dockerfile"`
	Plan                 string          `json:"plan,omitempty" jsonschema:"instance plan, e.g. free, starter, standard, pro (default free)"`
	EnvVars              []envVarArg     `json:"envVars,omitempty" jsonschema:"literal (non-secret) environment variables to set on the job"`
	SecretFiles          []secretFileArg `json:"secretFiles,omitempty" jsonschema:"secret files mounted under /etc/secrets from first boot"`
	AutoDeploy           string          `json:"autoDeploy,omitempty" jsonschema:"redeploy on a git push to the branch: yes or no (default yes for a repo)"`
	NotifyOnFail         string          `json:"notifyOnFail,omitempty" jsonschema:"deploy-failure notification override: default (defer to each member's own preference), notify (always email every member), or ignore (never email anyone for this service); default if omitted"`
	DryRun               bool            `json:"dryRun,omitempty" jsonschema:"if true, return the resolved spec preview without any writes — zero side effects (w2/m29)"`
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
		RegistryCredentialID: cloneStringPtr(a.RegistryCredentialID),
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

// toEnvVars maps the Render {key,value} env-var shape onto the CR EnvVar type,
// shared by the create tools.
func toEnvVars(in []envVarArg) []appv1alpha1.EnvVar {
	var env []appv1alpha1.EnvVar
	for _, e := range in {
		env = append(env, appv1alpha1.EnvVar{Name: e.Key, Value: e.Value})
	}
	return env
}

func toSecretFiles(in []secretFileArg) []core.SecretFile {
	var files []core.SecretFile
	for _, f := range in {
		files = append(files, core.SecretFile{Name: f.Name, Content: f.Content})
	}
	return files
}

// deployArgs is the deploy tool's input: a repo + its bex.yml. Deploy-from-chat
// is create with a manifest — one agent call takes code (one service or a whole
// stack) to live URLs.
type deployArgs struct {
	OwnerID string `json:"ownerId,omitempty" jsonschema:"the workspace to deploy into (an owner id, tea-...); omit to use the workspace selected with select_workspace, else your default workspace"`
	Repo    string `json:"repo,omitempty" jsonschema:"git repository URL to deploy (overrides the repo in bexYaml, if any)"`
	Branch  string `json:"branch,omitempty" jsonschema:"branch to deploy (overrides the branch in bexYaml, if any)"`
	BexYAML string `json:"bexYaml" jsonschema:"the project's bex.yml — a render.yaml Blueprint manifest. May declare a whole stack: services: (web/worker/cron) + databases:, wired by fromDatabase env references. One call converges all of it; validation is all-or-nothing; re-applying an unchanged file is a no-op"`
	Confirm string `json:"confirm,omitempty" jsonschema:"required only when this call would change an EXISTING service that belongs to a protectedStatus=protected Environment (w6/m19): the exact phrase from the error message of a first, unconfirmed call"`
}

// renderStack is the deploy tool's result for a multi-resource bex.yml: the
// services + databases one deploy call created (databases applied first, then
// services — dependents reference databases via fromDatabase). A single-service
// bex.yml returns a one-element services list and no databases. Poll each
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

// validateBlueprintArgs is validate_bex_yml's input (w2/m15).
type validateBlueprintArgs struct {
	BexYAML string `json:"bexYaml" jsonschema:"the bex.yml content to validate (render.yaml Blueprint shape); parsed and checked for per-entry errors with no apply"`
	OwnerID string `json:"ownerId,omitempty" jsonschema:"workspace id (tea-...); omit to use the session's selected workspace"`
}

// listBlueprintsArgs is list_blueprints' input (w2/m15).
type listBlueprintsArgs struct {
	OwnerID string `json:"ownerId,omitempty" jsonschema:"restrict to this workspace id (tea-...); omit to use the session's selected workspace"`
}

// listBlueprintsResult wraps the array — MCP tool outputs must be JSON objects.
type listBlueprintsResult struct {
	Blueprints []BlueprintView `json:"blueprints"`
}

// syncBlueprintArgs is sync_blueprint's input (w2/m15).
type syncBlueprintArgs struct {
	ID      string `json:"id" jsonschema:"the blueprint id (blp-…), as returned by list_blueprints or a prior deploy call"`
	BexYAML string `json:"bexYaml,omitempty" jsonschema:"optional updated bex.yml to store and apply; omit to re-apply the stored manifest unchanged"`
	OwnerID string `json:"ownerId,omitempty" jsonschema:"workspace id (tea-...); omit to use the session's selected workspace"`
	Confirm string `json:"confirm,omitempty" jsonschema:"exact confirmation phrase returned by a protected-environment error when the sync overrides an existing service"`
}

// autoscalingArgs is set_autoscaling's input — mirrors Render's PUT
// /v1/services/{id}/autoscaling request body (minInstances / maxInstances /
// targetCPUPercent / targetMemoryPercent).
type autoscalingArgs struct {
	ServiceID           string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	MinInstances        int32  `json:"minInstances" jsonschema:"minimum running instances (≥ 0; default 1)"`
	MaxInstances        int32  `json:"maxInstances" jsonschema:"maximum running instances (≥ 1; must be ≥ minInstances)"`
	TargetCPUPercent    *int32 `json:"targetCPUPercent,omitempty" jsonschema:"target average CPU utilization % of tier limit (1-100); required if targetMemoryPercent is absent"`
	TargetMemoryPercent *int32 `json:"targetMemoryPercent,omitempty" jsonschema:"target average memory utilization % of tier limit (1-100); required if targetCPUPercent is absent"`
}

// domainArgs is the shared custom-domain argument (serviceId + domain name).
type domainArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Name      string `json:"name" jsonschema:"the custom domain FQDN, e.g. www.example.com"`
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
	OwnerID       string            `json:"ownerId,omitempty" jsonschema:"the workspace to create in (an owner id, tea-...); omit to use the workspace selected with select_workspace, else your default workspace"`
	EnvironmentID string            `json:"environmentId,omitempty" jsonschema:"an environment id (env-...) in the target workspace; assignment also joins its project"`
	Name          string            `json:"name" jsonschema:"the static site name (a DNS label, 1-30 chars)"`
	Repo          string            `json:"repo,omitempty" jsonschema:"git repository URL to build from; omit if using image"`
	Image         string            `json:"image,omitempty" jsonschema:"a prebuilt OCI image whose publishPath holds the built site; omit if using repo"`
	Branch        string            `json:"branch,omitempty" jsonschema:"branch to track when building from a repo (default main)"`
	RootDir       string            `json:"rootDir,omitempty" jsonschema:"subdirectory of the repo to build from, for monorepos (default the repo root)"`
	PublishPath   string            `json:"publishPath" jsonschema:"the built output directory to serve as the site root, e.g. dist, build, or public"`
	EnvVars       []envVarArg       `json:"envVars,omitempty" jsonschema:"literal (non-secret) build-time environment variables"`
	SecretFiles   []secretFileArg   `json:"secretFiles,omitempty" jsonschema:"secret files available to the static-site build from first boot"`
	Domains       []string          `json:"domains,omitempty" jsonschema:"custom domains to serve the site at, in addition to the platform hostname"`
	Routes        []staticRouteArg  `json:"routes,omitempty" jsonschema:"ordered redirect/rewrite rules (first match wins), e.g. an SPA fallback rewrite of /* to /index.html"`
	Headers       []staticHeaderArg `json:"headers,omitempty" jsonschema:"custom response-header rules scoped by request path"`
	DryRun        bool              `json:"dryRun,omitempty" jsonschema:"if true, return the resolved spec preview without any writes — zero side effects (w2/m29)"`
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
		DryRun:        a.DryRun,
	}
}

// routesArgs / headersArgs / publishPathArgs are the static-site edge-rule tool
// inputs; the set tools replace the whole list (Render's bulk update).
type routesArgs struct {
	ServiceID string           `json:"serviceId" jsonschema:"the static site id (bex App name), as returned by list_services"`
	Routes    []staticRouteArg `json:"routes" jsonschema:"the full ordered list of redirect/rewrite rules to set (replaces the existing routes)"`
}

type headersArgs struct {
	ServiceID string            `json:"serviceId" jsonschema:"the static site id (bex App name), as returned by list_services"`
	Headers   []staticHeaderArg `json:"headers" jsonschema:"the full list of custom response-header rules to set (replaces the existing headers)"`
}

type publishPathArgs struct {
	ServiceID   string `json:"serviceId" jsonschema:"the static site id (bex App name), as returned by list_services"`
	PublishPath string `json:"publishPath" jsonschema:"the built output directory to serve as the site root, e.g. dist"`
}

// routesResult / headersResult wrap the arrays — MCP tool outputs must be objects.
type routesResult struct {
	Routes []renderRoute `json:"routes"`
}

type headersResult struct {
	Headers []renderHeader `json:"headers"`
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
type domainListResult struct {
	CustomDomains []renderCustomDomain `json:"customDomains"`
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
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Confirm   string `json:"confirm,omitempty" jsonschema:"required only if this service belongs to a protectedStatus=protected Environment: the exact phrase from the error message of a first, unconfirmed call"`
}

// RegisterMCP adds the service and custom-domain tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_services",
		Description: "List all services (bex Apps) in the workspace with their status. Scoped to ownerId if given, else to the session's selected workspace (select_workspace), else unscoped.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listServicesArgs) (*mcp.CallToolResult, listServicesResult, error) {
		apps, err := s.List(ctx, core.SelectedWorkspace(s.Selections, req, in.OwnerID))
		if err != nil {
			return nil, listServicesResult{}, err
		}
		return nil, listServicesResult{Services: toRenderServices(apps)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_service",
		Description: "Get details about a specific service by id.",
	}, s.serviceTool(s.Get))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_web_service",
		Description: "Create a web service from a repo or a prebuilt image and get back the service to poll until its url is live. A name already used in the target workspace is rejected (name already in use) rather than redeployed — use restart_service to redeploy an existing one. Tracks Render's MCP tool.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createWebServiceArgs) (*mcp.CallToolResult, renderService, error) {
		in.OwnerID = core.SelectedWorkspace(s.Selections, req, in.OwnerID)
		app, err := s.Create(ctx, in.toCreateRequest())
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_cron_job",
		Description: "Create a cron job that runs a repo/image's command on a schedule, and get back the service. A name already used in the target workspace is rejected (name already in use) rather than redeployed — use restart_service to redeploy an existing one. Tracks Render's MCP tool.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createCronJobArgs) (*mcp.CallToolResult, renderService, error) {
		in.OwnerID = core.SelectedWorkspace(s.Selections, req, in.OwnerID)
		app, err := s.Create(ctx, in.toCreateRequest())
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_static_site",
		Description: "Create a static site: build a repo and serve its publishPath output from the object-store origin (no running container). Redirects/rewrites (routes) and custom response headers apply at the edge. A name already used in the target workspace is rejected (name already in use) rather than republished — use restart_service to republish an existing one. Tracks Render's MCP tool.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createStaticSiteArgs) (*mcp.CallToolResult, renderService, error) {
		in.OwnerID = core.SelectedWorkspace(s.Selections, req, in.OwnerID)
		app, err := s.Create(ctx, in.toCreateRequest())
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "run_cron_job",
		Description: "Trigger a one-off run of a cron job now, canceling an active run first like Render. Returns the deterministic pending run. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, renderCronJobRun, error) {
		run, err := s.TriggerCronRun(ctx, in.ServiceID)
		return nil, toRenderCronJobRun(run), err
	})

	mcp.AddTool(srv, &mcp.Tool{
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

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_cron_job_run",
		Description: "bex extension: get one cron job run by its crr- id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in cronJobRunArgs) (*mcp.CallToolResult, renderCronJobRun, error) {
		run, err := s.GetCronRun(ctx, in.ServiceID, in.RunID)
		return nil, toRenderCronJobRun(run), err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "cancel_cron_job_run",
		Description: "bex extension: cancel one pending cron job run by crr- id. The operator terminates its Kubernetes Job; a terminal run returns a conflict instead of silently succeeding.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in cronJobRunArgs) (*mcp.CallToolResult, renderCronJobRun, error) {
		run, err := s.CancelCronRun(ctx, in.ServiceID, in.RunID)
		return nil, toRenderCronJobRun(run), err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_cron_job",
		Description: "Change a cron job's schedule and/or command. Render ships a non-functional stub for this tool; bex makes it real. schedule is the 5-field crontab expression (required); command overrides the image's entrypoint (optional — omit to keep the existing override, empty to clear it).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateCronJobArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetCronJob(ctx, in.ServiceID, &in.Schedule, in.Command)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "deploy",
		Description: "Deploy a project from a git repo and its bex.yml (render.yaml Blueprint) in one call. A bex.yml may declare a whole stack — several services (web/worker/cron) plus managed databases, wired by fromDatabase env references — and one call converges all of it, databases first. Validation is all-or-nothing: one invalid entry rejects the whole deploy. Re-applying an unchanged bex.yml is an idempotent no-op (changed services redeploy, unchanged ones don't). Returns the services (poll each to a live url via get_service) and databases (poll via get_postgres). A change to an EXISTING service that belongs to a protectedStatus=protected Environment (w6/m19) requires confirm — retry with the phrase from the error message. bex extension (pillar 4, deploy-from-chat at stack scale).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deployArgs) (*mcp.CallToolResult, renderStack, error) {
		res, err := s.DeployStack(ctx, DeployRequest{Repo: in.Repo, Branch: in.Branch, Manifest: in.BexYAML, Confirm: in.Confirm})
		if err != nil {
			return nil, renderStack{}, err
		}
		return nil, toRenderStack(res), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_service",
		Description: "Delete a service permanently, cascading everything the operator derived from it (Deployment, Service, Ingress). This is irreversible. A member of a protectedStatus=protected Environment (w6/m19) refuses without confirm — retry with the phrase from the error message. bex extension over Render's MCP (Render's official server ships no delete tool), named after the REST delete verb.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceConfirmArgs) (*mcp.CallToolResult, deletedResult, error) {
		err := s.Delete(withConfirm(ctx, in.Confirm), in.ServiceID)
		return nil, deletedResult{Deleted: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "restart_service",
		Description: "Restart a service (rolling restart, no downtime). bex extension over Render's MCP.",
	}, s.serviceTool(s.Restart))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "suspend_service",
		Description: "Suspend a service: scale to zero, keeping host and certificates. A member of a protectedStatus=protected Environment (w6/m19) refuses without confirm — retry with the phrase from the error message. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceConfirmArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.Suspend(withConfirm(ctx, in.Confirm), in.ServiceID)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "resume_service",
		Description: "Resume a suspended service, restoring its replicas. bex extension over Render's MCP.",
	}, s.serviceTool(s.Resume))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_service_plan",
		Description: "Change a service's instance plan/size (e.g. to starter, standard, pro, pro_plus, pro_max, pro_ultra). Resizes the pod's resources and rolls it. Pass dryRun:true to preview the change without any writes. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updatePlanArgs) (*mcp.CallToolResult, renderService, error) {
		var (
			app AppView
			err error
		)
		if in.DryRun {
			app, err = s.PreviewSetPlan(ctx, in.ServiceID, in.Plan)
		} else {
			app, err = s.SetPlan(ctx, in.ServiceID, in.Plan)
		}
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "scale_service",
		Description: "Scale a service to a specific number of running instances (numInstances, 1-100). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in scaleArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.Scale(ctx, in.ServiceID, in.NumInstances)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_idle_timeout",
		Description: "Set a service's idle timeout: seconds a free-tier service may idle before it auto-sleeps (0 = controller default). bex extension over Render's MCP (Render's spin-down window is fixed).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in idleTimeoutArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetIdleTTL(ctx, in.ServiceID, in.IdleTTLSeconds)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_root_directory",
		Description: "Set a build-from-git service's Root Directory: the subdirectory of its repo to build from (monorepo support). Triggers a fresh build scoped to that subdirectory. Tracks Render's Root Directory setting.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in rootDirArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetRootDir(ctx, in.ServiceID, in.RootDir)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_start_command",
		Description: "Change the command used to start an existing service. For a Docker service this overrides the image CMD (Render's Docker Command); for a native runtime it is Render's Start Command. Empty restores the image default where supported. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in startCommandArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetCommands(ctx, in.ServiceID, nil, &in.StartCommand)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_dockerfile_path",
		Description: "Change a repo-backed Docker service's Dockerfile Path, relative to its Root Directory. Empty restores the default Dockerfile. Triggers a fresh build. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in dockerfilePathArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetDockerfilePath(ctx, in.ServiceID, in.DockerfilePath)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_registry_credential",
		Description: "Bind an image-backed service to a stored private-registry credential. The credential must belong to the service workspace and match the image registry host. Pass an empty registryCredentialId to clear the binding.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in registryCredentialArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetRegistryCredential(ctx, in.ServiceID, in.RegistryCredentialID)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_build_filter",
		Description: "Set a build-from-git service's Build Filters: repository-root-relative glob patterns (paths/ignoredPaths) deciding whether a git push triggers an auto-deploy. A push deploys only when a changed file matches an include path (or paths is empty) and is not ignored; ignored wins over included. Pass empty paths and ignoredPaths to clear the filter. Tracks Render's Build Filters setting. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in buildFilterArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetBuildFilter(ctx, in.ServiceID, in.BuildFilter.toView())
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_maintenance_mode",
		Description: "Take a web service offline behind an interstitial page without suspending it — pods keep running, only public traffic is redirected to the maintenance page. Every host the service serves answers 503. An empty uri serves bex's default page; a non-empty uri must be an absolute http(s) URL to a custom page, fetched and served in place of the default. web_service only. Tracks Render's maintenanceMode setting.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in maintenanceModeArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetMaintenanceMode(ctx, in.ServiceID, MaintenanceModeView{Enabled: in.MaintenanceMode.Enabled, URI: in.MaintenanceMode.URI})
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	// Autoscaling tools — tracking Render's PUT/DELETE .../autoscaling contract.
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_autoscaling",
		Description: "Get the autoscaling configuration for a service (minInstances, maxInstances, targetCPUPercent, targetMemoryPercent). Returns enabled:false when autoscaling is not configured. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, AutoscalingView, error) {
		av, err := s.GetAutoscaling(ctx, in.ServiceID)
		if err != nil {
			return nil, AutoscalingView{}, err
		}
		return nil, av, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_autoscaling",
		Description: "Enable or update autoscaling for a service. The operator adjusts replicas within [minInstances, maxInstances] to hold the target CPU and/or memory utilization. Tracks Render's PUT /v1/services/{id}/autoscaling.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in autoscalingArgs) (*mcp.CallToolResult, AutoscalingView, error) {
		av, err := s.SetAutoscaling(ctx, in.ServiceID, SetAutoscalingRequest{
			MinInstances:        in.MinInstances,
			MaxInstances:        in.MaxInstances,
			TargetCPUPercent:    in.TargetCPUPercent,
			TargetMemoryPercent: in.TargetMemoryPercent,
		})
		if err != nil {
			return nil, AutoscalingView{}, err
		}
		return nil, av, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "disable_autoscaling",
		Description: "Disable autoscaling for a service, reverting it to its fixed spec.replicas count. Tracks Render's DELETE /v1/services/{id}/autoscaling.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, deletedResult, error) {
		err := s.DeleteAutoscaling(ctx, in.ServiceID)
		return nil, deletedResult{Deleted: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_auto_deploy",
		Description: "Turn a service's Auto-Deploy on or off: whether a signed git push to its tracked branch redeploys it (Render's Auto-Deploy toggle). Off leaves only explicit deploys. Does not itself redeploy.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in autoDeployArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetAutoDeploy(ctx, in.ServiceID, in.Enabled)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_notify_on_fail",
		Description: "Change a service's deploy-failure notification override (Render's exact notifyOnFail field/enum): default defers to each workspace member's own notification preference, notify always emails every member on a failed deploy, ignore never emails anyone for this service. Governs failure notifications only — success emails always follow each member's own preference.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in notifyOnFailArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetNotifyOnFail(ctx, in.ServiceID, in.Value)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_notifications_to_send",
		Description: "Set a service notification policy: default inherits workspace/member preferences (failure-only by default), failure sends only failed-deploy mail, all sends every deploy lifecycle mail, and none suppresses deploy mail.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in notificationsToSendArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetNotificationsToSend(ctx, in.ServiceID, in.Value)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_display_name",
		Description: "Change the human-facing label for a service without changing its immutable service id, platform hostname, or derived Kubernetes resources. Pass an empty displayName to restore the immutable-name fallback. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in displayNameArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetDisplayName(ctx, in.ServiceID, in.DisplayName)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_health_check_path",
		Description: "Change the HTTP path the platform GETs to decide whether a web or private service is ready to receive traffic (spec.healthCheckPath → ReadinessProbe). A 2xx/3xx response marks the pod ready. Pass an empty string to restore the platform default /. Has no effect on cron_job or background_worker services.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in healthCheckPathArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetHealthCheckPath(ctx, in.ServiceID, in.HealthCheckPath)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_max_shutdown_delay",
		Description: "Set the graceful-shutdown window for a web, private, or background-worker service: seconds after SIGTERM before Kubernetes sends SIGKILL (1-300; default 30). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in maxShutdownDelayArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetMaxShutdownDelay(ctx, in.ServiceID, in.Seconds)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_pre_deploy_command",
		Description: "Change the pre-deploy command (spec.preDeployCommand → Render's Pre-Deploy Command): a command run to completion against the new revision's image before it serves traffic (typically a database migration). A non-zero exit fails the deploy and leaves the previous revision serving. Pass an empty string to clear the step. Has no effect on cron_job or static_site services.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in preDeployCommandArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetPreDeployCommand(ctx, in.ServiceID, in.PreDeployCommand)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_subdomain_policy",
		Description: "Control whether the platform subdomain (<slug>.onbex.co) is active for a web or static-site service (Render's renderSubdomainPolicy field). 'enabled' (default) keeps the platform host in the Ingress and status URL; 'disabled' drops it so only custom domains configured on the service receive traffic. Requires at least one custom domain to be already configured before disabling — otherwise the service becomes unreachable.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in subdomainPolicyArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetSubdomainPolicy(ctx, in.ServiceID, in.Policy)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_service_ip_allow_list",
		Description: "Set the inbound IP allowlist for a web service or static site. Each entry is a CIDR in IPv4 or IPv6 notation (e.g. '1.2.3.4/32', '::/0'). An empty or null cidrs list clears the allowlist — opens the service to all source IPs (Render's default). Tracks Render's ipAllowList on webServiceDetails / staticSiteDetails.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceIPAllowListArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetIPAllowList(ctx, in.ServiceID, in.CIDRs)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	// Custom domain tools — tracking render-oss/render-mcp-server tool names.
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_custom_domains",
		Description: "List all custom domains configured for a service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, domainListResult, error) {
		domains, err := s.ListDomains(ctx, in.ServiceID)
		if err != nil {
			return nil, domainListResult{}, err
		}
		out := make([]renderCustomDomain, 0, len(domains))
		for _, d := range domains {
			out = append(out, toRenderCustomDomain(d))
		}
		return nil, domainListResult{CustomDomains: out}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_custom_domain",
		Description: "Get details about a specific custom domain on a service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in domainArgs) (*mcp.CallToolResult, renderCustomDomain, error) {
		d, err := s.GetDomain(ctx, in.ServiceID, in.Name)
		if err != nil {
			return nil, renderCustomDomain{}, err
		}
		return nil, toRenderCustomDomain(d), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "add_custom_domain",
		Description: "Add a custom domain to a service. The domain must be CNAME'd to the service's platform hostname before TLS can be issued.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in domainArgs) (*mcp.CallToolResult, renderCustomDomain, error) {
		d, err := s.AddDomain(ctx, in.ServiceID, in.Name)
		if err != nil {
			return nil, renderCustomDomain{}, err
		}
		return nil, toRenderCustomDomain(d), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_custom_domain",
		Description: "Remove a custom domain from a service. The operator will remove the Ingress rule and let the TLS certificate expire.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in domainArgs) (*mcp.CallToolResult, deletedResult, error) {
		err := s.DeleteDomain(ctx, in.ServiceID, in.Name)
		return nil, deletedResult{Deleted: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "verify_custom_domain",
		Description: "Re-check a custom domain's DNS/certificate state now and return its fresh verification/serving status plus the DNS record to create (Render's verify). Verification is automatic on bex, so this is a re-read, not a trigger.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in domainArgs) (*mcp.CallToolResult, renderCustomDomain, error) {
		d, err := s.VerifyDomain(ctx, in.ServiceID, in.Name)
		if err != nil {
			return nil, renderCustomDomain{}, err
		}
		return nil, toRenderCustomDomain(d), nil
	})

	// Static-site edge-rule tools. Render's official MCP ships only a
	// non-functional update_static_site stub; bex makes routes/headers/publishPath
	// real, delegating to the same Service verbs REST/GraphQL use.
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_static_routes",
		Description: "List a static site's redirect/rewrite rules (in order, first match wins).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, routesResult, error) {
		routes, err := s.ListRoutes(ctx, in.ServiceID)
		if err != nil {
			return nil, routesResult{}, err
		}
		return nil, routesResult{Routes: toRenderRoutes(routes)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_static_routes",
		Description: "Replace a static site's redirect/rewrite rules with the given ordered list (Render's routes). The change takes effect without a rebuild. Rejected for a non-static-site service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in routesArgs) (*mcp.CallToolResult, routesResult, error) {
		app, err := s.SetRoutes(ctx, in.ServiceID, routeArgViews(in.Routes))
		if err != nil {
			return nil, routesResult{}, err
		}
		return nil, routesResult{Routes: toRenderRoutes(app.Routes)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_static_headers",
		Description: "List a static site's custom response-header rules.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, headersResult, error) {
		headers, err := s.ListHeaders(ctx, in.ServiceID)
		if err != nil {
			return nil, headersResult{}, err
		}
		return nil, headersResult{Headers: toRenderHeaders(headers)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_static_headers",
		Description: "Replace a static site's custom response-header rules with the given list (Render's headers). The change takes effect without a rebuild. Rejected for a non-static-site service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in headersArgs) (*mcp.CallToolResult, headersResult, error) {
		app, err := s.SetHeaders(ctx, in.ServiceID, headerArgViews(in.Headers))
		if err != nil {
			return nil, headersResult{}, err
		}
		return nil, headersResult{Headers: toRenderHeaders(app.Headers)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_publish_path",
		Description: "Change the built output directory a static site serves (its publishPath) and republish. Rejected for a non-static-site service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in publishPathArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetPublishPath(ctx, in.ServiceID, in.PublishPath)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	// Blueprint verbs (w2/m15): validate_bex_yml · list_blueprints · sync_blueprint.
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "validate_bex_yml",
		Description: "Dry-run parse a bex.yml (render.yaml Blueprint) and return structured per-entry errors plus a resource plan without applying anything — the safe pre-flight check before a deploy call. Returns {valid, errors: [{error, line?, column?, path?}], plan?}. Requires no store; always available. bex extension (pillar 4 agent safety).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in validateBlueprintArgs) (*mcp.CallToolResult, BlueprintValidation, error) {
		v, err := s.ValidateBlueprint(ctx, core.SelectedWorkspace(s.Selections, req, in.OwnerID), in.BexYAML)
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_blueprints",
		Description: "List known bex.yml stack sources (blueprints) for a workspace. Blueprints are auto-registered on the first deploy call that includes a repo+bexYaml. Returns {blueprints: [{id, name, repo, branch, status, createdAt, updatedAt}]}. bex extension.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listBlueprintsArgs) (*mcp.CallToolResult, listBlueprintsResult, error) {
		views, err := s.ListBlueprints(ctx, core.SelectedWorkspace(s.Selections, req, in.OwnerID))
		return nil, listBlueprintsResult{Blueprints: views}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "sync_blueprint",
		Description: "Re-apply a stored blueprint idempotently — same all-or-nothing semantics as deploy, but sourced from the stored manifest. If bex_yaml is provided, the stored manifest is replaced before re-apply. Returns {blueprint, stack: {services, databases}}. Use validate_bex_yml first to catch errors with no side effects. bex extension (pillar 4, validate-then-deploy flow).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in syncBlueprintArgs) (*mcp.CallToolResult, SyncBlueprintResult, error) {
		res, err := s.SyncBlueprint(ctx, in.ID, core.SelectedWorkspace(s.Selections, req, in.OwnerID), in.BexYAML, in.Confirm)
		return nil, res, err
	})
}

// serviceTool adapts a single-service verb (Get/Restart/Suspend/Resume) into an
// MCP tool handler returning the Render service object — the same mapping REST's
// verb handlers use, so the surfaces stay identical.
func (s *Service) serviceTool(fn func(context.Context, string) (AppView, error)) mcp.ToolHandlerFor[serviceArgs, renderService] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := fn(ctx, in.ServiceID)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	}
}
