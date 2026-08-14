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
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	// initialDeployHookAnnotation persists the blueprint-declared initialDeployHook
	// command on the App CR so reads can echo it back. CR-annotation-only: the
	// operator need not know about it, keeping the feature CRD-neutral.
	initialDeployHookAnnotation = "bex.co/initial-deploy-hook"
	// initialDeployHookRanAnnotation is set to "true" after the hook's pre-deploy
	// Job succeeded on the first deploy. Its presence gates subsequent blueprint
	// syncs so the hook never reruns (w2/m45).
	initialDeployHookRanAnnotation = "bex.co/initial-deploy-hook-ran"
)

// deploy.go is the deploy-from-chat mapper (pillar 4): it turns a repo + a
// Render Blueprint-shaped render.yaml into a stack of CreateRequests + Database specs and
// rides Core.Create, so "deploy this repo (web + worker + postgres)" is one agent
// call — no bespoke deploy endpoint (t001 amended 2026-07-08; multi-service form
// w1/m24). The single-service field mapping mirrors scripts/app-apply.sh, the
// reference render.yaml → CR projection.
//
// A render.yaml carries the Render Blueprint shape: top-level `services:` +
// `databases:` + `envVarGroups:`. All validation is all-or-nothing: one invalid entry rejects
// the whole apply with a per-entry error before any resource is written (w1/m24
// DoD). Re-applying an unchanged file is a per-resource idempotent no-op — no
// spurious restarts, no new deploy records.
//
// Blueprint field completeness (w1/m35): the five render.yaml env forms bex once
// rejected now work — `envVarGroups:` blocks materialize env groups (create/update
// by name), a service's `{fromGroup: <name>}` links a group's vars, `sync: false`
// and `generateValue` seed the mutable env-vars store SEED-ONCE (so a later sync
// never clobbers a dashboard edit or re-mints a secret), and
// `fromService.envVarKey` copies a sibling service's declared var by value. The
// env-groups + env-vars work rides the existing feature services through two
// narrow seams (EnvGroupApplier, EnvSeeder), engaged only when a manifest uses
// those forms.

// DeployRequest is the deploy-from-chat input: a git repo plus its render.yaml
// (render.yaml-shaped manifest). Repo/Branch, when set, override the manifest's
// per-service repo/branch — an agent that already knows the checkout it is
// deploying need not duplicate it in the file.
type DeployRequest struct {
	// OwnerID is the workspace to deploy INTO — the same optional, membership-checked
	// `ownerId` contract Create has (w6/m14). Empty means the caller's default
	// workspace. Deploy creates a service, so an MCP call's explicit workspaceId
	// must land its deploy there, not in whichever workspace the caller happens
	// to resolve to.
	OwnerID  string
	Repo     string
	Branch   string
	Manifest string
	// EnvVarValues supplies sync:false values collected by an interactive
	// Blueprint create flow. They are never included in a validation plan or an
	// App spec; apply seeds them once into the mutable env store.
	EnvVarValues map[string]string
	// Confirm, when set, arms a direct-deploy-override on an EXISTING member
	// service of a protectedStatus=protected Environment (w6/m19,
	// apps/protection.go's requireUnprotected) — the phrase
	// ProtectedConfirmation("deploy", name) for the specific service being
	// overridden. Irrelevant (never checked) for a brand-new service or an
	// unprotected one.
	Confirm string
}

// EnvGroupApplier is the blueprint apply path's seam onto the env-groups feature
// (*envgroups.Service satisfies it structurally): materialize a render.yaml's
// envVarGroups: by name and link them to services via fromGroup. Kept to the three
// methods the apply path needs so apps never imports envgroups.
type EnvGroupApplier interface {
	// GroupNames returns every existing env group's name, for pre-flighting an
	// unknown fromGroup reference before any write (all-or-nothing).
	GroupNames(ctx context.Context) ([]string, error)
	// ApplyEnvGroup upserts a group by name (create if absent, reconcile its vars);
	// literals re-sync to their value, generates mint once. Idempotent.
	ApplyEnvGroup(ctx context.Context, name string, literals map[string]string, generates []string) error
	// LinkEnvGroup links the named group to a service. Idempotent (an already-linked
	// service is not re-patched, so a re-apply doesn't roll the pod).
	LinkEnvGroup(ctx context.Context, name, service string) error
	// SetGroupEnvironment assigns the named group to an environment. Called after
	// ApplyEnvGroup when a Blueprint env group is declared under a
	// projects[].environments[] nesting. Idempotent when already assigned.
	SetGroupEnvironment(ctx context.Context, name, environmentID string) error
}

// EnvSeeder is the blueprint apply path's seam onto the env-vars feature
// (*secrets.Service satisfies it structurally): seed a service's sync:false +
// generateValue vars into the mutable env store SEED-ONCE.
type EnvSeeder interface {
	SeedEnvVars(ctx context.Context, service string, literals map[string]string, generates []string) error
}

// BlueprintGroupingStore is the narrow store seam used by Render's canonical
// projects[].environments[] Blueprint vocabulary. The apply path resolves or
// creates groups by name, then threads the resulting environment id through the
// same resource-create paths as REST/GraphQL/MCP.
type BlueprintGroupingStore interface {
	ListProjects(ctx context.Context, tenantID string) ([]store.Project, error)
	CreateProject(ctx context.Context, tenantID, name string) (store.Project, error)
	ListEnvironments(ctx context.Context, projectID string) ([]store.Environment, error)
	CreateEnvironment(ctx context.Context, projectID, tenantID, name string) (store.Environment, error)
	SetEnvironmentACL(ctx context.Context, id, protectedStatus string, networkIsolationEnabled bool, ipAllowList []core.IPAllowListEntry) error
}

// EnvironmentAssigner is the narrow store seam the Blueprint apply path uses to
// record a service's project/environment assignment on its store row; a Store
// without it makes an environment-changing apply ErrWorkspacesUnavailable.
type EnvironmentAssigner interface {
	SetAppEnvironment(context.Context, string, string, string) error
}

// StackResult is the set of resources one stack deploy created (or converged):
// databases are applied first (dependents reference them via fromDatabase),
// then services. Both are individually pollable to Ready via their status.
type StackResult struct {
	Services  []AppView           `json:"services"`
	Databases []StackDatabaseView `json:"databases"`
	KeyValues []StackKeyValueView `json:"keyValues"`
}

// StackDatabaseView is the minimal managed-Postgres projection a stack deploy
// returns — enough for an agent to poll (stable id + name + phase). The full Render-shaped
// postgres object (connection info, replicas) is the postgres feature's
// get_postgres / GET /v1/postgres/{id} surface; a stack deploy does not duplicate it.
type StackDatabaseView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"` // Database phase (Pending|Provisioning|Ready|Failed)
}

type StackKeyValueView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// --- manifest shape (render.yaml Blueprint vocabulary) ---

// bexManifest is the legacy Go type used to adapt compiler-validated render.yaml
// source. Services may be declared under
// the Blueprint key `services:`; databases live under `databases:`; env groups under
// `envVarGroups:` (materialized by name at apply, w1/m35).
type bexManifest struct {
	Services     []bexService  `json:"services"`     // Blueprint services list
	Databases    []bexDatabase `json:"databases"`    // Blueprint databases list
	EnvVarGroups []bexEnvGroup `json:"envVarGroups"` // Blueprint env-groups list
	Projects     []bexProject  `json:"projects"`
	Ungrouped    *bexResources `json:"ungrouped"`
}

type bexResources struct {
	Services     []bexService  `json:"services"`
	Databases    []bexDatabase `json:"databases"`
	EnvVarGroups []bexEnvGroup `json:"envVarGroups"`
}

type bexProject struct {
	Name         string           `json:"name"`
	Environments []bexEnvironment `json:"environments"`
}

type bexEnvironment struct {
	bexResources
	Name        string                    `json:"name"`
	Networking  bexEnvironmentNetworking  `json:"networking"`
	Permissions bexEnvironmentPermissions `json:"permissions"`
}

type bexEnvironmentNetworking struct {
	Isolation string `json:"isolation"`
}

type bexEnvironmentPermissions struct {
	Protection string `json:"protection"`
}

// bexEnvGroup is one entry in envVarGroups: a named, reusable set of env vars a
// service links via `{fromGroup: <name>}`. Per Render, a group var is a literal
// ({key,value}) or a `generateValue: true` — a group may NOT reference services or
// other groups (no fromService/fromDatabase/fromGroup) and may NOT carry
// `sync: false` (Render ignores such a var); bex rejects those with a clear
// per-entry error rather than silently dropping the variable.
type bexEnvGroup struct {
	Name    string      `json:"name"`
	EnvVars []bexEnvVar `json:"envVars"`
}

// bexService is one entry in services:. It accepts Render Blueprint field names
// (plan, numInstances, type, runtime, domains, staticPublishPath, …).
// Fields bex does not honor are parsed elsewhere or rejected with a clear error
// (see docs/ADR018-render-parity.md's Blueprint row for the field-by-field map).
type bexService struct {
	Name    string    `json:"name"`
	Type    string    `json:"type"`    // render.yaml short type: web|pserv|worker|cron (empty=web); runtime:static => static_site
	Runtime string    `json:"runtime"` // render.yaml runtime; "static" selects static_site, "image" => prebuilt
	Plan    string    `json:"plan"`    // render.yaml plan (Render spelling)
	Repo    string    `json:"repo"`
	Branch  string    `json:"branch"`
	Image   *bexImage `json:"image,omitempty"` // render.yaml image: {url}
	// Builder is deliberately retained only to issue a migration error. New
	// Blueprint build strategies live under x-bex.builder so an official Render
	// field can never be mistaken for a bex extension.
	Builder string        `json:"builder"`
	XBex    *bexExtension `json:"x-bex,omitempty"`
	RootDir string        `json:"rootDir"`
	// BuildFilter is render.yaml's Build Filters (paths/ignoredPaths globs) — the
	// same {paths, ignoredPaths} shape every surface uses (BuildFilterView).
	BuildFilter             *BuildFilterView    `json:"buildFilter"`
	BuildCommand            string              `json:"buildCommand"`
	StartCommand            string              `json:"startCommand"`
	DockerCommand           string              `json:"dockerCommand"`
	DockerfilePath          string              `json:"dockerfilePath"` // Render's Dockerfile Path, relative to rootDir; docker runtime only
	Routes                  []StaticRouteView   `json:"routes"`
	Headers                 []StaticHeaderView  `json:"headers"`
	NumInstances            int32               `json:"numInstances"` // render.yaml manual instance count
	HealthCheckPath         string              `json:"healthCheckPath"`
	Domains                 []string            `json:"domains"`
	Domain                  string              `json:"domain"`                  // deprecated single-domain spelling
	Schedule                string              `json:"schedule"`                // cron expression, required when type is cron
	MaxShutdownDelaySeconds *int32              `json:"maxShutdownDelaySeconds"` // Render's graceful SIGTERM window (1-300; default 30)
	PreDeployCommand        string              `json:"preDeployCommand"`        // render.yaml Pre-Deploy Command (spec.preDeployCommand)
	InitialDeployHook       string              `json:"initialDeployHook"`       // render.yaml one-time first-deploy command (w2/m45)
	Scaling                 *bexScaling         `json:"scaling"`                 // render.yaml autoscaling block (w2/m49)
	MaintenanceMode         *bexMaintenanceMode `json:"maintenanceMode"`         // Render: paid web services only; uri optional
	AutoDeploy              *bool               `json:"autoDeploy"`              // deprecated render.yaml bool; nil => default
	AutoDeployTrigger       string              `json:"autoDeployTrigger"`       // render.yaml: commit|checksPass|off
	RenderSubdomainPolicy   string              `json:"renderSubdomainPolicy"`
	StaticPublishPath       string              `json:"staticPublishPath"` // render.yaml static-site publish dir
	EnvVars                 []bexEnvVar         `json:"envVars"`
	IPAllowList             []bexIPEntry        `json:"ipAllowList"`
	MaxmemoryPolicy         string              `json:"maxmemoryPolicy"`
	PersistenceMode         string              `json:"persistenceMode"`
	PreviewPlan             string              `json:"previewPlan"`
	PullRequestPreviews     *bool               `json:"pullRequestPreviewsEnabled"`
	// Neither field exists in Render's Blueprint service schema. Keeping them
	// explicit lets bex reject the common create-body/Blueprint mix-up by name
	// instead of silently dropping it.
	EnvironmentID string            `json:"environmentId"`
	SecretFiles   []secretFileInput `json:"secretFiles"`
}

type bexMaintenanceMode struct {
	Enabled bool   `json:"enabled"`
	URI     string `json:"uri"`
}

// bexScaling is render.yaml's `scaling:` block — Render's autoscaling config at
// Blueprint create time (w2/m49). Mirrors Render's minInstances/maxInstances/
// targetCPUPercent/targetMemoryPercent fields; at least one target is required
// when the block is present (validated in specFromCreate via SetAutoscalingRequest).
type bexScaling struct {
	MinInstances        int32  `json:"minInstances"`
	MaxInstances        int32  `json:"maxInstances"`
	TargetCPUPercent    *int32 `json:"targetCPUPercent"`
	TargetMemoryPercent *int32 `json:"targetMemoryPercent"`
}

// bexImage is render.yaml's `image: {url, creds}` — bex honors just the url.
type bexImage struct {
	URL   string `json:"url"`
	Creds string `json:"creds"`
}

type bexExtension struct {
	Builder string `json:"builder"`
}

// bexDatabase is one entry in databases:. Field names follow render.yaml; bex
// honors plan/diskSizeGB/postgresMajorVersion/ipAllowList/readReplicas/
// highAvailability plus the create-time physical databaseName/user. region and
// storageAutoscalingEnabled remain documented omissions.
type bexDatabase struct {
	Name                      string                            `json:"name"`
	DatabaseName              string                            `json:"databaseName"`
	User                      string                            `json:"user"`
	Plan                      string                            `json:"plan"`
	DiskSizeGB                int32                             `json:"diskSizeGB"`
	StorageAutoscalingEnabled *bool                             `json:"storageAutoscalingEnabled"`
	ConnectionPool            string                            `json:"connectionPool"`
	PostgresMajorVersion      string                            `json:"postgresMajorVersion"`
	IPAllowList               []bexIPEntry                      `json:"ipAllowList"`
	ReadReplicas              []appv1alpha1.DatabaseReadReplica `json:"readReplicas"`
	HighAvailability          *bexHA                            `json:"highAvailability"`
	Region                    string                            `json:"region"`
	PreviewPlan               string                            `json:"previewPlan"`
	PreviewDiskSizeGB         *int32                            `json:"previewDiskSizeGB"`
	EnvironmentID             string                            `json:"environmentId"`
}

// bexIPEntry is one CIDR in render.yaml's ipAllowList: [{source, description}].
type bexIPEntry struct {
	Source      string `json:"source"`
	Description string `json:"description"`
}

// bexHA is render.yaml's highAvailability: {enabled}.
type bexHA struct {
	Enabled bool `json:"enabled"`
}

// bexEnvVar is one envVars entry. The forms bex honors (w1/m35 closed the last
// gaps): a literal {key,value} → a plain spec.Env var (re-synced each apply);
// {fromDatabase} → a secretRef into the DB's connection Secret; {fromService}
// with a property (host/port/hostport) → a literal, or with envVarKey → a copy of
// a sibling service's declared var; {generateValue: true} → a server-minted secret
// seeded once; {key, sync:false} → a literal seeded once into the mutable env
// store; and a KEYLESS {fromGroup: <name>} → links every var of the named env
// group. sync:false and generateValue land in the mutable env-vars store (not
// spec.Env), so a later dashboard edit wins and a re-sync never overwrites them.
type bexEnvVar struct {
	Key           string      `json:"key"`
	Value         string      `json:"value"`
	GenerateValue bool        `json:"generateValue"`
	Sync          *bool       `json:"sync"`
	FromDatabase  *bexFromRef `json:"fromDatabase"`
	FromService   *bexFromRef `json:"fromService"`
	FromGroup     string      `json:"fromGroup"`
}

// bexFromRef is the fromDatabase / fromService target. property is the
// connectionString/host/port/... to pull; envVarKey references another env var
// (fromService only) — resolved by resolveRef, which copies the sibling
// service's declared value at parse time.
type bexFromRef struct {
	Name      string `json:"name"`
	Type      string `json:"type"`      // fromService: the referenced service's type
	Property  string `json:"property"`  // connectionString|host|port|user|password|database|hostport
	EnvVarKey string `json:"envVarKey"` // fromService alternate: copy another env var
}

// --- the parsed stack (validated, env fully resolved) ---

// parsedStack is the all-or-nothing-validated projection of a render.yaml: an
// ordered env-group, database, then service set, ready to apply. Same-file
// cross-resource references are validated here; the ones whose value depends
// on an applied resource (fromDatabase secret names, fromService host slugs)
// are carried as deferred refs and resolved during apply.
type parsedStack struct {
	envGroups []parsedEnvGroup
	services  []parsedService
	databases []parsedDatabase
	keyValues []parsedKeyValue
	groupings []parsedGrouping
}

type parsedGrouping struct {
	projectName             string
	environmentName         string
	networkIsolationEnabled bool
	protectedStatus         string
}

// parsedEnvGroup is a validated envVarGroups[] entry: literal vars keyed to their
// value, plus the keys whose value the apply mints (generateValue). grouping is set
// when the group is declared under a projects[].environments[] nesting and carries
// the blueprintGroupingKey used to look up the environment ID after apply.
type parsedEnvGroup struct {
	name      string
	literals  map[string]string
	generates []string
	grouping  string
}

// parsedService is a service ready to apply plus the blueprint env work deferred
// to apply time: fromGroup links, and the sync:false/generateValue vars seeded
// once into the mutable env store (never spec.Env, so a dashboard edit wins).
type parsedService struct {
	req           CreateRequest
	fields        map[string]BlueprintField
	databaseRefs  []bexEnvVar       // resolved after database IDs are known
	kvRefs        []bexEnvVar       // resolved after keyvalue CR names are known
	hostRefs      []hostRef         // fromService host/hostport — resolved after sibling slugs are known
	groupLinks    []string          // fromGroup names to link after create
	seedLiterals  map[string]string // sync:false literals, seeded once
	seedGenerates []string          // generateValue keys, minted + seeded once
	promptKeys    []string          // sync:false keys without a manifest value
	grouping      string
	ungrouped     bool
}

// hostRef is a deferred fromService host/hostport reference: the env key, the
// sibling render.yaml name it targets, that sibling's effective port (declared or
// the 3000 default — known at parse), and whether the literal is host-only or
// host:port. The hostname half is the sibling's slug, minted at create, so
// resolution happens at apply (docs/ADR041-service-addresses.md D3).
type hostRef struct {
	key      string
	target   string
	port     int32
	hostport bool
}

// env materializes the reference once the target's slug is known.
func (ref hostRef) env(slug string) appv1alpha1.EnvVar {
	value := slug
	if ref.hostport {
		value = fmt.Sprintf("%s:%d", slug, ref.port)
	}
	return appv1alpha1.EnvVar{Name: ref.key, Value: value}
}

type parsedDatabase struct {
	name      string
	spec      appv1alpha1.DatabaseSpec
	fields    map[string]BlueprintField
	grouping  string
	ungrouped bool
}

type parsedKeyValue struct {
	name      string
	spec      appv1alpha1.KeyValueSpec
	fields    map[string]BlueprintField
	grouping  string
	ungrouped bool
}

func blueprintGroupingKey(projectName, environmentName string) string {
	return projectName + "\x00" + environmentName
}

// dbPropertyKey maps a render.yaml fromDatabase property to the key in the CNPG
// "<stable-id>-app" connection Secret (the Secret vocabulary
// docs/ADR009-postgresql-management.md documents: username, password, dbname,
// host, port, uri).
var dbPropertyKey = map[string]string{
	"connectionString":     "uri",
	"connectionPoolString": "uri",
	"host":                 "host",
	"port":                 "port",
	"user":                 "username",
	"password":             "password",
	"database":             "dbname",
}

const (
	serviceRefPropertyHost     = "host"
	serviceRefPropertyPort     = "port"
	serviceRefPropertyHostPort = "hostport"
)

// serviceRefProperty is the set of fromService properties bex honors for a
// web/private service (host/port/hostport), resolved to literal env values:
// port at parse (the platform default); host/hostport at apply, injecting
// the sibling's slug — the hostname the operator's slug-named Service makes
// resolvable in-cluster (docs/ADR041-service-addresses.md D3, w9/m57).
var serviceRefProperty = map[string]bool{
	serviceRefPropertyHost: true, serviceRefPropertyPort: true, serviceRefPropertyHostPort: true,
}

// kvPropertyKey maps a render.yaml fromService (type:keyvalue/redis) property to
// the key in the keyvalue credentials Secret (the operator writes uri/host/port/
// password/username; docs/ADR021-keyvalue-management.md).
var kvPropertyKey = map[string]string{
	"connectionString": "uri",
	"host":             "host",
	"port":             "port",
	"password":         "password",
}

// DeployStack applies a whole render.yaml in one call: databases first (dependents
// reference them via fromDatabase), then services, each as an idempotent upsert.
// All validation runs in parseStack before any write — one invalid entry rejects
// the whole apply with a per-entry error, nothing partially created.
func (s *Service) DeployStack(ctx context.Context, req DeployRequest) (StackResult, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return StackResult{}, err
	}
	return s.deployStack(ctx, req)
}

// deployStack is the unauthorized core; DeployStack authorizes once before
// parsing and applying the stack.
func (s *Service) deployStack(ctx context.Context, req DeployRequest) (StackResult, error) {
	st, ir, err := compileStack(req)
	if err != nil {
		return StackResult{}, err
	}
	if _, _, err := s.blueprintActionPlan(ctx, ir, st); err != nil {
		return StackResult{}, err
	}
	return s.deployParsedStack(ctx, req, st)
}

// listWorkspaceDatabases fetches the workspace-scoped Database snapshot the
// Blueprint flows share (deployParsedStack, the existing-reference resolver,
// and the action-plan resolver).
func (s *Service) listWorkspaceDatabases(ctx context.Context, tenantID string) (*appv1alpha1.DatabaseList, error) {
	list := &appv1alpha1.DatabaseList{}
	if err := s.Client.List(ctx, list, s.DatastoreListOptions(tenantID)...); err != nil {
		return nil, err
	}
	return list, nil
}

// listWorkspaceKeyValues is listWorkspaceDatabases' KeyValue twin.
func (s *Service) listWorkspaceKeyValues(ctx context.Context, tenantID string) (*appv1alpha1.KeyValueList, error) {
	list := &appv1alpha1.KeyValueList{}
	if err := s.Client.List(ctx, list, s.DatastoreListOptions(tenantID)...); err != nil {
		return nil, err
	}
	return list, nil
}

// deployParsedStack applies the exact validated stack produced by compileStack.
// It is deliberately separate from deployStack for flows (Blueprint create and
// sync) that must preflight before recording state, then apply that same result.
func (s *Service) deployParsedStack(ctx context.Context, req DeployRequest, st parsedStack) (StackResult, error) {
	if err := s.validateBlueprintServices(ctx, st); err != nil {
		return StackResult{}, err
	}
	if err := s.requireStackPaymentMethod(ctx, st); err != nil {
		return StackResult{}, err
	}
	databases, keyValues, err := s.stackDatastoreSnapshots(ctx, st)
	if err != nil {
		return StackResult{}, err
	}
	databaseIDs, kvCRNames, err := s.resolveExistingBlueprintReferences(ctx, st, databases, keyValues)
	if err != nil {
		return StackResult{}, err
	}
	// Pre-flight the env-groups + env-vars seams BEFORE any write (all-or-nothing):
	// a manifest that uses envVarGroups/fromGroup or sync:false/generateValue but
	// whose backing store isn't wired is rejected here, as is an unknown fromGroup
	// name — so nothing is partially created.
	if err := s.preflightBlueprintEnv(ctx, st); err != nil {
		return StackResult{}, err
	}
	assignments, err := s.applyBlueprintGroupings(ctx, st.groupings)
	if err != nil {
		return StackResult{}, err
	}
	// req.Confirm rides the context from here on — applyCreate's protection
	// guard (requireUnprotected) reads it via core.ConfirmFrom, the same
	// context seam Delete/Suspend's REST/GraphQL/MCP adapters use.
	ctx = core.WithConfirm(ctx, req.Confirm)
	res := StackResult{}
	if err := s.applyStackEnvGroups(ctx, st.envGroups, assignments); err != nil {
		return res, err
	}
	if err := s.applyStackDatastores(ctx, st, assignments, databases, keyValues, databaseIDs, kvCRNames, &res); err != nil {
		return res, err
	}
	// Sibling slugs for fromService host/hostport refs (ADR041 D3), filled
	// lazily and memoized: a target that already exists (re-sync, or declared
	// earlier in this apply) resolves before its referrer is written — no spec
	// churn; a ref to a service created later in this same first apply is
	// deferred to the post-create patch pass below. "" memoizes known-absent so
	// a shared forward target is looked up at most once.
	serviceSlugs := make(map[string]string, len(st.services))
	lookupSlug := func(target string) (string, error) {
		if slug, ok := serviceSlugs[target]; ok {
			return slug, nil
		}
		existing, err := s.GetApp(ctx, core.RelCanCreate, target)
		if errors.Is(err, core.ErrNotFound) {
			serviceSlugs[target] = ""
			return "", nil
		}
		if err != nil {
			return "", err
		}
		slug := existing.Spec.PlatformSubdomain(existing.Name)
		serviceSlugs[target] = slug
		return slug, nil
	}
	deferred, err := s.applyStackServices(ctx, st, assignments, databaseIDs, kvCRNames, serviceSlugs, lookupSlug, &res)
	if err != nil {
		return res, err
	}
	if err := s.patchDeferredStackServices(ctx, deferred, serviceSlugs); err != nil {
		return res, err
	}
	// Auto-register a blueprint row when called with a repo (w2/m15): lets
	// list_blueprints surface it and sync_blueprint re-apply it later without
	// the caller needing to register it separately.
	if req.Repo != "" {
		s.upsertBlueprint(ctx, req)
	}
	return res, nil
}

// stackDatastoreSnapshots fetches one workspace-scoped Database/KeyValue List
// each for the whole apply: the display-name lookups
// (resolveExistingBlueprintReferences and the applyDatabase/applyKeyValue
// upserts) share these snapshots instead of re-Listing per stack entry. Safe
// because stack entry names are unique (registerUniqueName), so no lookup
// targets an object this same apply creates.
func (s *Service) stackDatastoreSnapshots(ctx context.Context, st parsedStack) (*appv1alpha1.DatabaseList, *appv1alpha1.KeyValueList, error) {
	tenantID, _ := s.Tenant(ctx)
	var databases *appv1alpha1.DatabaseList
	if len(st.databases) > 0 {
		var err error
		if databases, err = s.listWorkspaceDatabases(ctx, tenantID); err != nil {
			return nil, nil, err
		}
	}
	var keyValues *appv1alpha1.KeyValueList
	if len(st.keyValues) > 0 {
		var err error
		if keyValues, err = s.listWorkspaceKeyValues(ctx, tenantID); err != nil {
			return nil, nil, err
		}
	}
	return databases, keyValues, nil
}

// applyStackEnvGroups applies the stack's env groups. Env groups first: a
// service's fromGroup links one, which needs the group (and its projection
// Secret) to exist before the service is patched.
func (s *Service) applyStackEnvGroups(ctx context.Context, envGroups []parsedEnvGroup, assignments map[string]core.EnvironmentAssignment) error {
	for _, g := range envGroups {
		if err := s.EnvGroups.ApplyEnvGroup(ctx, g.name, g.literals, g.generates); err != nil {
			return fmt.Errorf("env group %q: %w", g.name, err)
		}
		if g.grouping != "" {
			if assignment := assignments[g.grouping]; assignment.ID != "" {
				if err := s.EnvGroups.SetGroupEnvironment(ctx, g.name, assignment.ID); err != nil {
					return fmt.Errorf("assigning env group %q to environment: %w", g.name, err)
				}
			}
		}
	}
	return nil
}

// applyStackDatastores upserts the stack's databases and key values, appending
// each applied view onto res and recording its name → ID/CR-name for the
// service reference resolution that follows. Databases next: a service's
// fromDatabase env points (via secretRef) at the CNPG "<stable-id>-app"
// Secret, which only exists once the Database converges — applying the
// dependent first would leave it Pending on a missing Secret anyway, but
// applying the DB first starts its provisioning immediately.
func (s *Service) applyStackDatastores(ctx context.Context, st parsedStack, assignments map[string]core.EnvironmentAssignment, databases *appv1alpha1.DatabaseList, keyValues *appv1alpha1.KeyValueList, databaseIDs, kvCRNames map[string]string, res *StackResult) error {
	for _, db := range st.databases {
		v, err := s.applyDatabase(ctx, db, assignments[db.grouping], databases.Items)
		if err != nil {
			return err
		}
		res.Databases = append(res.Databases, v)
		databaseIDs[db.name] = v.ID
	}
	for _, kv := range st.keyValues {
		v, err := s.applyKeyValue(ctx, kv, assignments[kv.grouping], keyValues.Items)
		if err != nil {
			return err
		}
		res.KeyValues = append(res.KeyValues, v)
		kvCRNames[kv.name] = v.ID // CR name = Secret name (kv.Status.SecretName = kv.Name)
	}
	return nil
}

// deferredService is a service whose host refs pointed at a not-yet-created
// sibling on this first apply; patched with the resolved env after the
// first-pass create loop.
type deferredService struct {
	req    CreateRequest
	fields map[string]BlueprintField
	refs   []hostRef
}

// applyStackServices is the first service pass: each stack service is applied
// with every already-resolvable reference on its env and its applied view
// appended onto res; the services whose host refs were forward references are
// returned for the post-create patch pass.
func (s *Service) applyStackServices(ctx context.Context, st parsedStack, assignments map[string]core.EnvironmentAssignment, databaseIDs, kvCRNames, serviceSlugs map[string]string, lookupSlug func(string) (string, error), res *StackResult) ([]deferredService, error) {
	var deferred []deferredService
	for _, svc := range st.services {
		laterRefs, err := resolveServiceRefs(&svc, databaseIDs, kvCRNames, lookupSlug)
		if err != nil {
			return nil, err
		}
		if assignment := assignments[svc.grouping]; assignment.ID != "" {
			svc.req.EnvironmentID = assignment.ID
			svc.req.EnvironmentSpecified = true
		} else if svc.ungrouped {
			svc.req.EnvironmentSpecified = true
		}
		v, err := s.applyBlueprintCreate(ctx, svc.req, svc.fields)
		if err != nil {
			return nil, err
		}
		serviceSlugs[svc.req.Name] = v.Slug
		if len(laterRefs) > 0 {
			deferred = append(deferred, deferredService{req: svc.req, fields: svc.fields, refs: laterRefs})
		}
		// Link fromGroup groups (idempotent) and seed sync:false/generateValue vars
		// (seed-once) now that the service exists.
		for _, g := range svc.groupLinks {
			if err := s.EnvGroups.LinkEnvGroup(ctx, g, svc.req.Name); err != nil {
				return nil, fmt.Errorf("linking env group %q to %q: %w", g, svc.req.Name, err)
			}
		}
		if len(svc.seedLiterals) > 0 || len(svc.seedGenerates) > 0 {
			if err := s.EnvSeeder.SeedEnvVars(ctx, svc.req.Name, svc.seedLiterals, svc.seedGenerates); err != nil {
				return nil, fmt.Errorf("seeding env for %q: %w", svc.req.Name, err)
			}
		}
		res.Services = append(res.Services, v)
	}
	return deferred, nil
}

// patchDeferredStackServices is the second pass: every service now exists,
// every slug is known — patch the services whose host refs were forward
// references (first apply only).
func (s *Service) patchDeferredStackServices(ctx context.Context, deferred []deferredService, serviceSlugs map[string]string) error {
	for _, d := range deferred {
		for _, ref := range d.refs {
			slug := serviceSlugs[ref.target]
			if slug == "" { // unreachable: every stack service was created above
				return fmt.Errorf("%w: service %q: fromService references service %q whose address is not yet known", core.ErrBadRequest, d.req.Name, ref.target)
			}
			d.req.Env = append(d.req.Env, ref.env(slug))
		}
		if _, err := s.applyBlueprintCreate(ctx, d.req, d.fields); err != nil {
			return err
		}
	}
	return nil
}

// resolveServiceRefs is one stack service's reference-resolution prelude: the
// resolved database/key-value env vars and every already-resolvable host ref
// are appended onto svc.req.Env; the forward host refs (sibling not created
// yet) are returned for the post-create patch pass.
func resolveServiceRefs(svc *parsedService, databaseIDs, kvCRNames map[string]string, lookupSlug func(string) (string, error)) ([]hostRef, error) {
	for _, ref := range svc.databaseRefs {
		ev, err := resolveDatabaseRef(ref, databaseIDs)
		if err != nil {
			return nil, fmt.Errorf("%w: service %q: %v", core.ErrBadRequest, svc.req.Name, err)
		}
		svc.req.Env = append(svc.req.Env, ev)
	}
	for _, ref := range svc.kvRefs {
		ev, err := resolveKeyValueRef(ref, kvCRNames)
		if err != nil {
			return nil, fmt.Errorf("%w: service %q: %v", core.ErrBadRequest, svc.req.Name, err)
		}
		svc.req.Env = append(svc.req.Env, ev)
	}
	var laterRefs []hostRef
	for _, ref := range svc.hostRefs {
		slug, err := lookupSlug(ref.target)
		if err != nil {
			return nil, err
		}
		if slug == "" {
			laterRefs = append(laterRefs, ref) // forward ref — sibling not created yet
			continue
		}
		svc.req.Env = append(svc.req.Env, ref.env(slug))
	}
	return laterRefs, nil
}

// resolveExistingBlueprintReferences provides the one allowed cross-declaration
// lookup: a database or Key Value target not declared in this Blueprint may be
// named only when it resolves uniquely inside the caller's workspace. It runs
// before group or resource writes and yields only CR/Secret identities, never a
// credential value. Service envVarKey copying remains same-file-only because
// its source value is intentionally not a generally readable resource field.
// databases/keyValues are deployParsedStack's pre-fetched workspace snapshots;
// nil means fetch here when a lookup actually needs one.
func (s *Service) resolveExistingBlueprintReferences(ctx context.Context, st parsedStack, databases *appv1alpha1.DatabaseList, keyValues *appv1alpha1.KeyValueList) (map[string]string, map[string]string, error) {
	databaseIDs := make(map[string]string, len(st.databases))
	keyValueIDs := make(map[string]string, len(st.keyValues))
	declaredDatabases := make(map[string]bool, len(st.databases))
	declaredKeyValues := make(map[string]bool, len(st.keyValues))
	for _, database := range st.databases {
		declaredDatabases[database.name] = true
	}
	for _, keyValue := range st.keyValues {
		declaredKeyValues[keyValue.name] = true
	}

	neededDatabases := map[string]bool{}
	neededKeyValues := map[string]bool{}
	for _, service := range st.services {
		for _, ref := range service.databaseRefs {
			if ref.FromDatabase != nil && !declaredDatabases[ref.FromDatabase.Name] {
				neededDatabases[ref.FromDatabase.Name] = true
			}
		}
		for _, ref := range service.kvRefs {
			if ref.FromService != nil && !declaredKeyValues[ref.FromService.Name] {
				neededKeyValues[ref.FromService.Name] = true
			}
		}
	}
	if len(neededDatabases) == 0 && len(neededKeyValues) == 0 {
		return databaseIDs, keyValueIDs, nil
	}

	tenantID, scoped := s.Tenant(ctx)
	if len(neededDatabases) > 0 {
		if databases == nil {
			var err error
			if databases, err = s.listWorkspaceDatabases(ctx, tenantID); err != nil {
				return nil, nil, err
			}
		}
		for name := range neededDatabases {
			found, duplicate := uniqueDatabaseByDisplayName(databases.Items, scoped, tenantID, name)
			if duplicate {
				return nil, nil, fmt.Errorf("%w: fromDatabase reference %q is ambiguous in this workspace", core.ErrConflict, name)
			}
			if found == nil {
				return nil, nil, fmt.Errorf("%w: fromDatabase references unknown database %q in this workspace", core.ErrBadRequest, name)
			}
			databaseIDs[name] = found.Name
			for _, service := range st.services {
				for _, ref := range service.databaseRefs {
					if ref.FromDatabase != nil && ref.FromDatabase.Name == name && ref.FromDatabase.Property == "connectionPoolString" && !found.Spec.Pooler {
						return nil, nil, poolerRequiredError(service.req.Name, name)
					}
				}
			}
		}
	}
	if len(neededKeyValues) > 0 {
		if keyValues == nil {
			var err error
			if keyValues, err = s.listWorkspaceKeyValues(ctx, tenantID); err != nil {
				return nil, nil, err
			}
		}
		for name := range neededKeyValues {
			found, duplicate := uniqueKeyValueByDisplayName(keyValues.Items, scoped, tenantID, name)
			if duplicate {
				return nil, nil, fmt.Errorf("%w: fromService Key Value reference %q is ambiguous in this workspace", core.ErrConflict, name)
			}
			if found == nil {
				return nil, nil, fmt.Errorf("%w: fromService references unknown Key Value %q in this workspace", core.ErrBadRequest, name)
			}
			keyValueIDs[name] = found.Name
		}
	}
	return databaseIDs, keyValueIDs, nil
}

// uniqueDatabaseByDisplayName resolves a Blueprint display name against a
// fetched Database snapshot: the single item whose Spec.Name matches (scoped
// to tenantID when scoped), or duplicate=true when a second item also matches
// — each caller owns the error wording for its surface, and an ambiguous name
// only errors when it is actually looked up.
func uniqueDatabaseByDisplayName(items []appv1alpha1.Database, scoped bool, tenantID, name string) (*appv1alpha1.Database, bool) {
	return uniqueMatching(items, func(candidate *appv1alpha1.Database) bool {
		return (!scoped || candidate.Labels[core.LabelTenant] == tenantID) && candidate.Spec.Name == name
	})
}

// uniqueKeyValueByDisplayName is uniqueDatabaseByDisplayName's KeyValue twin.
func uniqueKeyValueByDisplayName(items []appv1alpha1.KeyValue, scoped bool, tenantID, name string) (*appv1alpha1.KeyValue, bool) {
	return uniqueMatching(items, func(candidate *appv1alpha1.KeyValue) bool {
		return (!scoped || candidate.Labels[core.LabelTenant] == tenantID) && candidate.Spec.Name == name
	})
}

func uniqueMatching[T any](items []T, match func(*T) bool) (*T, bool) {
	var found *T
	for index := range items {
		candidate := &items[index]
		if !match(candidate) {
			continue
		}
		if found != nil {
			return nil, true
		}
		found = candidate
	}
	return found, false
}

func (s *Service) requireStackPaymentMethod(ctx context.Context, st parsedStack) error {
	if !stackHasPaidPlan(st) {
		return nil
	}
	tenantID, _ := s.Tenant(ctx)
	return s.RequirePaymentMethod(ctx, tenantID)
}

func stackHasPaidPlan(st parsedStack) bool {
	for _, service := range st.services {
		tier, err := normalizeTierOrPlan(service.req.Plan)
		if err == nil && core.PaidPlan(tier) {
			return true
		}
	}
	for _, database := range st.databases {
		if core.PaidPlan(database.spec.Plan) {
			return true
		}
	}
	for _, keyValue := range st.keyValues {
		if core.PaidPlan(keyValue.spec.Plan) {
			return true
		}
	}
	return false
}

// validateBlueprintServices runs the ordinary create boundary over every
// parsed service before deployStack writes groupings or resources. URL
// ownership needs the configured base domain, so it lives here rather than in
// the context-free YAML parser and is shared by ValidateBlueprint.
func (s *Service) validateBlueprintServices(ctx context.Context, st parsedStack) error {
	for _, svc := range st.services {
		desired, err := specFromCreate(svc.req)
		if err != nil {
			return fmt.Errorf("service %q: %w", svc.req.Name, err)
		}
		if err := s.validateNewSpecMaintenanceMode(ctx, svc.req.Name, desired); err != nil {
			return fmt.Errorf("service %q: %w", svc.req.Name, err)
		}
	}
	return nil
}

// applyBlueprintGroupings resolves or creates the Project/Environment rows
// named by the canonical Render Blueprint nesting. It runs after the stateless
// parse/preflight gate and before resource writes, so every newborn resource
// receives the real environment id through its normal create path.
func (s *Service) applyBlueprintGroupings(ctx context.Context, groupings []parsedGrouping) (map[string]core.EnvironmentAssignment, error) {
	out := make(map[string]core.EnvironmentAssignment, len(groupings))
	if len(groupings) == 0 {
		return out, nil
	}
	if s.BlueprintGroups == nil {
		return nil, core.ErrWorkspacesUnavailable
	}
	tenantID, ok := s.Tenant(ctx)
	if !ok {
		return nil, core.ErrWorkspacesUnavailable
	}
	projects, err := s.BlueprintGroups.ListProjects(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing Blueprint projects: %w", err)
	}
	byName := make(map[string]store.Project, len(projects))
	for _, project := range projects {
		byName[project.Name] = project
	}
	// One ListEnvironments per project for the whole loop: groupings under the
	// same project share the memoized slice, extended in place after each
	// CreateEnvironment so a later grouping still sees the new row.
	environmentsByProject := map[string][]store.Environment{}
	for _, grouping := range groupings {
		project, exists := byName[grouping.projectName]
		if !exists {
			project, err = s.BlueprintGroups.CreateProject(ctx, tenantID, grouping.projectName)
			if err != nil {
				return nil, fmt.Errorf("creating Blueprint project %q: %w", grouping.projectName, err)
			}
			byName[grouping.projectName] = project
		}
		environments, listed := environmentsByProject[project.ID]
		if !listed {
			environments, err = s.BlueprintGroups.ListEnvironments(ctx, project.ID)
			if err != nil {
				return nil, fmt.Errorf("listing Blueprint environments for project %q: %w", grouping.projectName, err)
			}
			environmentsByProject[project.ID] = environments
		}
		var environment store.Environment
		for _, candidate := range environments {
			if candidate.Name == grouping.environmentName {
				environment = candidate
				break
			}
		}
		if environment.ID == "" {
			environment, err = s.BlueprintGroups.CreateEnvironment(ctx, project.ID, tenantID, grouping.environmentName)
			if err != nil {
				return nil, fmt.Errorf("creating Blueprint environment %q: %w", grouping.environmentName, err)
			}
			environmentsByProject[project.ID] = append(environmentsByProject[project.ID], environment)
		}
		// Render's Blueprint schema declares protection/isolation but no
		// environment IP rules, so the ACL write PRESERVES the row's current
		// ipAllowList (the fresh-create seed, or the rules an operator set) —
		// passing nil would full-replace to empty, which is deny-all since
		// w4/m28 and would silently cut every blueprint-grouped service off.
		if err := s.BlueprintGroups.SetEnvironmentACL(ctx, environment.ID, grouping.protectedStatus, grouping.networkIsolationEnabled, environment.IPAllowList); err != nil {
			return nil, fmt.Errorf("applying Blueprint environment %q controls: %w", grouping.environmentName, err)
		}
		out[blueprintGroupingKey(grouping.projectName, grouping.environmentName)] = core.EnvironmentAssignment{
			ID:                      environment.ID,
			ProjectID:               project.ID,
			WorkspaceID:             tenantID,
			NetworkIsolationEnabled: grouping.networkIsolationEnabled,
			IPAllowList:             environment.IPAllowList,
		}
	}
	return out, nil
}

// preflightBlueprintEnv validates the blueprint's env-groups + env-vars needs
// against the wired seams before any resource is written, preserving the
// all-or-nothing contract: a manifest that uses envVarGroups/fromGroup needs the
// env-groups seam; one using sync:false/generateValue needs the env-seeder seam;
// and every fromGroup name must resolve to a group declared in-file OR
// pre-existing in the workspace. Any failure here means nothing was created.
func (s *Service) preflightBlueprintEnv(ctx context.Context, st parsedStack) error {
	declared := make(map[string]bool, len(st.envGroups))
	for _, g := range st.envGroups {
		declared[g.name] = true
	}
	usesGroups := len(st.envGroups) > 0
	usesSeed := false
	var groupLinks []string
	for _, svc := range st.services {
		if len(svc.groupLinks) > 0 {
			usesGroups = true
			groupLinks = append(groupLinks, svc.groupLinks...)
		}
		if len(svc.seedLiterals) > 0 || len(svc.seedGenerates) > 0 {
			usesSeed = true
		}
	}
	if usesGroups && s.EnvGroups == nil {
		return fmt.Errorf("%w: render.yaml uses envVarGroups/fromGroup but env groups are unavailable (OpenBao not configured)", core.ErrBadRequest)
	}
	if usesSeed && s.EnvSeeder == nil {
		return fmt.Errorf("%w: render.yaml uses sync:false/generateValue but the env-vars store is unavailable (OpenBao not configured)", core.ErrBadRequest)
	}
	if len(groupLinks) == 0 {
		return nil
	}
	// A fromGroup target must exist: declared in-file, or already in the workspace.
	existing, err := s.EnvGroups.GroupNames(ctx)
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(existing)+len(declared))
	for _, n := range existing {
		known[n] = true
	}
	for n := range declared {
		known[n] = true
	}
	for _, ref := range groupLinks {
		if !known[ref] {
			return fmt.Errorf("%w: fromGroup references unknown env group %q (declare it under envVarGroups: or create it first)", core.ErrBadRequest, ref)
		}
	}
	return nil
}

// parseStack parses + fully validates + env-resolves a render.yaml into the ordered
// resource set, with no writes and no cluster access — the all-or-nothing gate.
// Every per-entry problem is reported with the offending name so the caller can
// list it; the whole parse fails on the first error (w1/m24 DoD: nothing
// half-created).
func parseStack(req DeployRequest) (parsedStack, error) {
	st, _, err := compileStack(req)
	return st, err
}

// compileStack is the single strict source-to-apply preparation boundary. It
// returns both views so callers needing resource inventory never recompile a
// manifest that has already passed the canonical compiler.
func compileStack(req DeployRequest) (parsedStack, BlueprintIR, error) {
	source, ir, problems := CompileBlueprintIR(req.Manifest)
	if len(problems) > 0 {
		return parsedStack{}, BlueprintIR{}, blueprintSourceProblemsError(problems)
	}
	st, err := parseCompiledStack(blueprintParseOverrides{repo: req.Repo, branch: req.Branch, envVarValues: req.EnvVarValues}, source, ir)
	if err != nil {
		return parsedStack{}, BlueprintIR{}, err
	}
	if err := validateBlueprintEnvVarValues(st, req.EnvVarValues); err != nil {
		return parsedStack{}, BlueprintIR{}, err
	}
	return st, ir, nil
}

// validateBlueprintEnvVarValues rejects values that do not name a sync:false
// prompt before any write. parseService has already attached matching values to
// its seed-once work; validation deliberately never reports the supplied values.
func validateBlueprintEnvVarValues(st parsedStack, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	matched := make(map[string]bool, len(values))
	for _, svc := range st.services {
		for _, key := range svc.promptKeys {
			if _, ok := values[key]; ok {
				matched[key] = true
			}
		}
	}
	for key := range values {
		if !matched[key] {
			return fmt.Errorf("%w: envVarValues contains %q, which is not a sync:false Blueprint prompt", core.ErrBadRequest, key)
		}
	}
	return nil
}

type blueprintParseOverrides struct {
	repo         string
	branch       string
	envVarValues map[string]string
}

// validateUniqueName trims, validates non-empty, and checks for duplicates.
func validateUniqueName(raw, entityType string, seen map[string]bool) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: %s is missing its name", core.ErrBadRequest, entityType)
	}
	if seen[trimmed] {
		return "", fmt.Errorf("%w: duplicate %s name %q", core.ErrBadRequest, entityType, trimmed)
	}
	seen[trimmed] = true
	return trimmed, nil
}

// poolerRequiredError is the shared wording for a connectionPoolString
// reference to a database without connectionPool: pgbouncer, so the
// parse-time and resolve-time checks can't drift.
func poolerRequiredError(service, database string) error {
	return fmt.Errorf("%w: service %q: fromDatabase connectionPoolString requires database %q connectionPool: pgbouncer", core.ErrBadRequest, service, database)
}

// registerUniqueName checks for duplicate names across services and databases.
func registerUniqueName(name string, seen map[string]bool) error {
	if seen[name] {
		return fmt.Errorf("%w: duplicate name %q — every service and database name in a render.yaml must be unique", core.ErrBadRequest, name)
	}
	seen[name] = true
	return nil
}

// decl wraps one manifest declaration with where it was declared: a
// project/environment grouping key, or the legacy top-level "ungrouped" list.
type decl[T any] struct {
	value     T
	grouping  string
	ungrouped bool
}

func appendDecls[T any](decls []decl[T], values []T, ungrouped bool) []decl[T] {
	for _, value := range values {
		decls = append(decls, decl[T]{value: value, ungrouped: ungrouped})
	}
	return decls
}

// mergeDecls seeds one manifest section's decl list: the grouped (top-level)
// declarations first, then the legacy `ungrouped:` extras, marked ungrouped
// only when the section distinguishes them (env groups intentionally do not).
func mergeDecls[T any](grouped, extra []T, markUngrouped bool) []decl[T] {
	decls := make([]decl[T], 0, len(grouped)+len(extra))
	decls = appendDecls(decls, grouped, false)
	return appendDecls(decls, extra, markUngrouped)
}

// isKeyValueType reports whether a manifest service type names the key-value
// flavor (Render accepts both spellings).
func isKeyValueType(t string) bool {
	return t == "keyvalue" || t == "redis"
}

// validateEnabledDisabled checks a manifest tri-state toggle ("", enabled,
// disabled); what names the field in the error.
func validateEnabledDisabled(value, what string) error {
	if value != "" && value != "enabled" && value != "disabled" {
		return fmt.Errorf("%w: %s must be enabled or disabled", core.ErrBadRequest, what)
	}
	return nil
}

// manifestAllowEntries validates and maps one manifest ipAllowList; noun and
// name identify the resource in errors.
func manifestAllowEntries(entries []bexIPEntry, noun, name string) ([]core.IPAllowListEntry, error) {
	var allow []core.IPAllowListEntry
	for _, e := range entries {
		if strings.TrimSpace(e.Source) == "" {
			return nil, fmt.Errorf("%w: %s %q has an ipAllowList entry without a source", core.ErrBadRequest, noun, name)
		}
		allow = append(allow, core.IPAllowListEntry{CIDRBlock: e.Source, Description: e.Description})
	}
	if err := core.ValidateAllowList(allow); err != nil {
		return nil, fmt.Errorf("%w: %s %q ipAllowList: %v", core.ErrBadRequest, noun, name, err)
	}
	return allow, nil
}

// parseCompiledStack is the typed adapter boundary after the one strict
// Blueprint compiler pass. Callers that already own a compiled source/IR (such
// as validation) use this directly so no later helper can parse the submitted
// YAML again with different semantics.
func parseCompiledStack(overrides blueprintParseOverrides, source *BlueprintSource, ir BlueprintIR) (parsedStack, error) {
	fieldsByResource := blueprintFieldsByResource(ir)
	m, err := decodeCompiledBlueprintManifest(source)
	if err != nil {
		return parsedStack{}, err
	}
	var ungrouped bexResources
	if m.Ungrouped != nil {
		ungrouped = *m.Ungrouped
	}
	services := mergeDecls(m.Services, ungrouped.Services, true)
	databases := mergeDecls(m.Databases, ungrouped.Databases, true)
	envGroups := mergeDecls(m.EnvVarGroups, ungrouped.EnvVarGroups, false)
	services, databases, envGroups, groupings, err := flattenProjectDecls(m.Projects, services, databases, envGroups)
	if err != nil {
		return parsedStack{}, err
	}
	st := parsedStack{groupings: groupings}
	if len(services) == 0 && len(databases) == 0 && len(envGroups) == 0 {
		return parsedStack{}, fmt.Errorf("%w: render.yaml must define at least one service, database, env group, or project environment resource", core.ErrBadRequest)
	}

	// Env groups first (validated + name-deduped); a service's fromGroup links one.
	groupNames := make(map[string]bool, len(envGroups))
	for _, egDecl := range envGroups {
		pg, err := parseEnvGroup(egDecl.value)
		if err != nil {
			return parsedStack{}, err
		}
		if err := registerUniqueName(pg.name, groupNames); err != nil {
			return parsedStack{}, fmt.Errorf("%w: duplicate env group name %q", core.ErrBadRequest, pg.name)
		}
		pg.grouping = egDecl.grouping
		st.envGroups = append(st.envGroups, pg)
	}

	// Databases next into the stack (and into the name index services reference).
	idx := newStackIndex(len(services), len(databases))
	for _, d := range databases {
		ds, err := parseDatabase(d.value)
		if err != nil {
			return parsedStack{}, err
		}
		if err := registerUniqueName(ds.name, idx.names); err != nil {
			return parsedStack{}, err
		}
		idx.databaseNames[ds.name] = true
		idx.databasePoolers[ds.name] = ds.spec.Pooler
		ds.grouping = d.grouping
		ds.ungrouped = d.ungrouped
		ds.fields = fieldsByResource[BlueprintResourcePostgres][ds.name]
		st.databases = append(st.databases, ds)
	}

	// Services: build each CreateRequest, collecting the literal env now and
	// deferring reference resolution until all names are known (a fromService may
	// reference a service declared later).
	type pending struct {
		svc     parsedService
		refVars []bexEnvVar // fromDatabase / fromService — resolved in pass 2
	}
	pendings := make([]pending, 0, len(services))
	for _, a := range services {
		if isKeyValueType(a.value.Type) {
			kv, err := parseKeyValue(a.value)
			if err != nil {
				return parsedStack{}, err
			}
			if err := registerUniqueName(kv.name, idx.names); err != nil {
				return parsedStack{}, err
			}
			kv.grouping = a.grouping
			kv.ungrouped = a.ungrouped
			kv.fields = fieldsByResource[BlueprintResourceKeyValue][kv.name]
			st.keyValues = append(st.keyValues, kv)
			continue
		}
		req, se, err := parseService(overrides, a.value)
		if err != nil {
			return parsedStack{}, err
		}
		if err := registerUniqueName(req.Name, idx.names); err != nil {
			return parsedStack{}, err
		}
		pendings = append(pendings, pending{
			svc:     parsedService{req: req, fields: blueprintServiceFields(fieldsByResource[BlueprintResourceService][req.Name]), groupLinks: se.groupLinks, seedLiterals: se.seedLiterals, seedGenerates: se.seedGenerates, promptKeys: se.promptKeys, grouping: a.grouping, ungrouped: a.ungrouped},
			refVars: se.refVars,
		})
		idx.serviceKnownEnv[req.Name] = se.known
		if req.Type == appv1alpha1.TypeWebService || req.Type == appv1alpha1.TypePrivateService {
			port := req.Port
			if port <= 0 {
				port = appv1alpha1.DefaultPort
			}
			idx.servicePorts[req.Name] = port
		}
	}

	// Pass 2: classify each pending service's deferred references now that
	// every name, port, and known-env is available.
	for _, p := range pendings {
		svc, err := idx.classifyRefs(p.svc, p.refVars)
		if err != nil {
			return parsedStack{}, err
		}
		st.services = append(st.services, svc)
	}
	return st, nil
}

// flattenProjectDecls walks Render's canonical projects[].environments[]
// nesting (validating names and the tri-state toggles) and flattens each
// environment's services/databases/env groups into the section decl lists,
// tagged with their grouping key, plus one parsedGrouping per environment.
func flattenProjectDecls(projects []bexProject, services []decl[bexService], databases []decl[bexDatabase], envGroups []decl[bexEnvGroup]) ([]decl[bexService], []decl[bexDatabase], []decl[bexEnvGroup], []parsedGrouping, error) {
	var groupings []parsedGrouping
	projectNames := map[string]bool{}
	for _, project := range projects {
		projectName, err := validateUniqueName(project.Name, "projects entry", projectNames)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		environmentNames := map[string]bool{}
		for _, environment := range project.Environments {
			environmentName, err := validateUniqueName(environment.Name, fmt.Sprintf("project %q environment", projectName), environmentNames)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			if err := validateEnabledDisabled(environment.Networking.Isolation, fmt.Sprintf("project %q environment %q networking.isolation", projectName, environmentName)); err != nil {
				return nil, nil, nil, nil, err
			}
			if err := validateEnabledDisabled(environment.Permissions.Protection, fmt.Sprintf("project %q environment %q permissions.protection", projectName, environmentName)); err != nil {
				return nil, nil, nil, nil, err
			}
			grouping := blueprintGroupingKey(projectName, environmentName)
			for _, eg := range environment.EnvVarGroups {
				envGroups = append(envGroups, decl[bexEnvGroup]{value: eg, grouping: grouping})
			}
			protectedStatus := core.ProtectedStatusUnprotected
			if environment.Permissions.Protection == "enabled" {
				protectedStatus = core.ProtectedStatusProtected
			}
			groupings = append(groupings, parsedGrouping{
				projectName:             projectName,
				environmentName:         environmentName,
				networkIsolationEnabled: environment.Networking.Isolation == "enabled",
				protectedStatus:         protectedStatus,
			})
			for _, service := range environment.Services {
				services = append(services, decl[bexService]{value: service, grouping: grouping})
			}
			for _, database := range environment.Databases {
				databases = append(databases, decl[bexDatabase]{value: database, grouping: grouping})
			}
		}
	}
	return services, databases, envGroups, groupings, nil
}

// stackIndex is the cross-declaration lookup state parseCompiledStack's pass 1
// builds and pass 2's reference classification (classifyRefs) consumes.
type stackIndex struct {
	// names is the unique-name index services reference; the services decl
	// slice includes both regular services and keyvalue resources.
	names map[string]bool
	// databaseNames marks the databases declared in this stack;
	// databasePoolers records whether each declares connectionPool: pgbouncer.
	databaseNames   map[string]bool
	databasePoolers map[string]bool
	// servicePorts: name -> effective port, for services that expose a k8s Service
	// (web/private only — workers, cron jobs and static sites have no Service, so a
	// fromService reference to them has no DNS name to resolve). Effective port is
	// the declared one, or the 3000 default when omitted.
	servicePorts map[string]int32
	// serviceKnownEnv: name -> the statically-known env of that service (plain
	// literals + sync:false seed values), the resolvable target set for a
	// fromService.envVarKey copy (a from*/generateValue var has no parse-time value).
	serviceKnownEnv map[string]map[string]string
}

func newStackIndex(serviceCount, databaseCount int) *stackIndex {
	return &stackIndex{
		names:           make(map[string]bool, serviceCount+databaseCount),
		databaseNames:   make(map[string]bool, databaseCount),
		databasePoolers: make(map[string]bool, databaseCount),
		servicePorts:    make(map[string]int32, serviceCount),
		serviceKnownEnv: make(map[string]map[string]string, serviceCount),
	}
}

// classifyRefs resolves one service's deferred reference env vars now that
// every name, port, and known-env is available. Database references are
// validated here but retained until apply: only then do we know the immutable
// dpg-... id that names the CNPG Secret.
func (idx *stackIndex) classifyRefs(svc parsedService, refVars []bexEnvVar) (parsedService, error) {
	for _, r := range refVars {
		if r.FromDatabase != nil {
			if idx.databaseNames[r.FromDatabase.Name] && r.FromDatabase.Property == "connectionPoolString" && !idx.databasePoolers[r.FromDatabase.Name] {
				return parsedService{}, poolerRequiredError(svc.req.Name, r.FromDatabase.Name)
			}
			svc.databaseRefs = append(svc.databaseRefs, r)
			continue
		}
		if r.FromService != nil && isKeyValueType(r.FromService.Type) {
			svc.kvRefs = append(svc.kvRefs, r)
			continue
		}
		// One target-existence check for every remaining fromService form
		// (host/hostport/port/envVarKey), so the message lives in one place.
		allowExistingServiceHost := r.FromService != nil && r.FromService.EnvVarKey == "" && r.FromService.Property == serviceRefPropertyHost
		if r.FromService != nil && !idx.names[r.FromService.Name] && !allowExistingServiceHost {
			return parsedService{}, fmt.Errorf("%w: service %q: fromService references unknown service %q (declare it under services: in the same file)", core.ErrBadRequest, svc.req.Name, r.FromService.Name)
		}
		// host/hostport inject the sibling's slug — minted at create (w4/m19),
		// so resolution is deferred to apply like databaseRefs above.
		// Addressability is validated here, all-or-nothing; the port half is
		// already known and resolved into the ref now.
		if r.FromService != nil && r.FromService.EnvVarKey == "" &&
			(r.FromService.Property == serviceRefPropertyHost || r.FromService.Property == serviceRefPropertyHostPort) {
			port, addressable := idx.servicePorts[r.FromService.Name]
			if !addressable && r.FromService.Property == serviceRefPropertyHost && idx.names[r.FromService.Name] {
				return parsedService{}, fmt.Errorf("%w: service %q: fromService host references %q which has no network address (only web/private services are addressable)", core.ErrBadRequest, svc.req.Name, r.FromService.Name)
			}
			svc.hostRefs = append(svc.hostRefs, hostRef{
				key: r.Key, target: r.FromService.Name, port: port,
				hostport: r.FromService.Property == serviceRefPropertyHostPort,
			})
			continue
		}
		ev, err := resolveRef(r, idx.servicePorts, idx.serviceKnownEnv)
		if err != nil {
			return parsedService{}, fmt.Errorf("%w: service %q: %v", core.ErrBadRequest, svc.req.Name, err)
		}
		svc.req.Env = append(svc.req.Env, ev)
	}
	return svc, nil
}

// decodeCompiledBlueprintManifest is a temporary typed-adapter boundary for
// the existing create request mappers. It never reparses the submitted YAML:
// source.Value is the strict compiler's JSON-compatible AST, so the only
// remaining decode cannot reintroduce aliases, duplicate keys, implicit YAML
// scalars, or ignored source fields.
func decodeCompiledBlueprintManifest(source *BlueprintSource) (bexManifest, error) {
	if source == nil {
		return bexManifest{}, fmt.Errorf("%w: Blueprint source is required", core.ErrBadRequest)
	}
	canonicalJSON, err := json.Marshal(source.Value)
	if err != nil {
		return bexManifest{}, fmt.Errorf("%w: encode compiled Blueprint: %v", core.ErrBadRequest, err)
	}
	var manifest bexManifest
	if err := json.Unmarshal(canonicalJSON, &manifest); err != nil {
		return bexManifest{}, fmt.Errorf("%w: decode compiled Blueprint: %v", core.ErrBadRequest, err)
	}
	return manifest, nil
}

func blueprintFieldsByResource(ir BlueprintIR) map[BlueprintResourceKind]map[string]map[string]BlueprintField {
	byKind := make(map[BlueprintResourceKind]map[string]map[string]BlueprintField)
	for _, resource := range ir.Resources {
		byName := byKind[resource.Kind]
		if byName == nil {
			byName = map[string]map[string]BlueprintField{}
			byKind[resource.Kind] = byName
		}
		byName[resource.Name] = resource.Fields
	}
	return byKind
}

// blueprintServiceFields maps the local extension's build strategy onto the
// AppSpec field it controls while retaining the source field's presence. The
// compiler has already constrained x-bex.builder to the extension vocabulary.
func blueprintServiceFields(fields map[string]BlueprintField) map[string]BlueprintField {
	if fields == nil {
		return nil
	}
	result := maps.Clone(fields)
	if extension, ok := fields["x-bex"]; ok {
		if values, ok := extension.Value.(map[string]any); ok {
			if _, declared := values["builder"]; declared {
				result["builder"] = extension
			}
		}
	}
	return result
}

// parseEnvGroup validates one envVarGroups[] entry and classifies its vars into
// literals + generateValue keys. Per Render, a group var may be only a literal or
// a generateValue — a group cannot reference services/groups (fromService/
// fromDatabase/fromGroup) and cannot carry sync:false; each is rejected with a
// clear per-entry error rather than silently dropped.
func parseEnvGroup(g bexEnvGroup) (parsedEnvGroup, error) {
	name := strings.TrimSpace(g.Name)
	if name == "" {
		return parsedEnvGroup{}, fmt.Errorf("%w: an envVarGroups entry is missing its name", core.ErrBadRequest)
	}
	pg := parsedEnvGroup{name: name, literals: map[string]string{}}
	for _, e := range g.EnvVars {
		if e.Key == "" {
			return parsedEnvGroup{}, fmt.Errorf("%w: env group %q has an env var without a key", core.ErrBadRequest, name)
		}
		switch {
		case e.FromGroup != "", e.FromDatabase != nil, e.FromService != nil:
			return parsedEnvGroup{}, fmt.Errorf("%w: env group %q var %q: a group cannot reference services or other groups", core.ErrBadRequest, name, e.Key)
		case e.Sync != nil && !*e.Sync:
			return parsedEnvGroup{}, fmt.Errorf("%w: env group %q var %q: sync:false is not allowed inside an env group", core.ErrBadRequest, name, e.Key)
		case e.GenerateValue:
			if e.Value != "" {
				return parsedEnvGroup{}, fmt.Errorf("%w: env group %q var %q sets both value and generateValue", core.ErrBadRequest, name, e.Key)
			}
			pg.generates = append(pg.generates, e.Key)
		default:
			pg.literals[e.Key] = e.Value
		}
	}
	return pg, nil
}

// serviceEnv is the classified env of one service entry: plain literals ride
// req.Env directly; the rest is deferred to apply time or resolved in pass 2.
type serviceEnv struct {
	refVars       []bexEnvVar       // fromDatabase / fromService — resolved once every name is known
	groupLinks    []string          // fromGroup names to link after create
	seedLiterals  map[string]string // sync:false literals, seeded once into the mutable env store
	seedGenerates []string          // generateValue keys, minted + seeded once
	promptKeys    []string          // sync:false keys with no manifest value
	known         map[string]string // this service's parse-time-known env (for a sibling's fromService.envVarKey)
}

// parseService maps one services[] entry onto a CreateRequest (with repo/branch
// overrides applied) plus its classified env (serviceEnv). Structural validation
// (type, schedule, plan, private+domains) happens here.
func parseService(overrides blueprintParseOverrides, a bexService) (CreateRequest, serviceEnv, error) {
	if err := validateManifestServiceFields(a); err != nil {
		return CreateRequest{}, serviceEnv{}, err
	}
	repo := a.Repo
	if overrides.repo != "" && a.Image == nil {
		repo = overrides.repo // the explicit deploy target wins over the manifest
	}
	branch := a.Branch
	if overrides.branch != "" && a.Image == nil {
		branch = overrides.branch
	}
	svcType, err := manifestType(a.Type, a.Runtime)
	if err != nil {
		return CreateRequest{}, serviceEnv{}, err
	}
	if err := validateManifestIngressFields(a, svcType); err != nil {
		return CreateRequest{}, serviceEnv{}, err
	}
	hosts := a.Domains
	if len(hosts) == 0 && a.Domain != "" {
		hosts = []string{a.Domain}
	}

	plan, maintenanceMode, err := manifestPlanAndMaintenance(a)
	if err != nil {
		return CreateRequest{}, serviceEnv{}, err
	}
	replicas := a.NumInstances
	image := ""
	if a.Image != nil && a.Image.URL != "" {
		image = a.Image.URL
	}
	publish := a.StaticPublishPath
	runtime := a.Runtime
	if strings.EqualFold(runtime, "static") {
		runtime = "" // static is represented by the service type, not an App runtime
	}
	startCommand, autoDeploy, err := manifestStartAndAutoDeploy(a)
	if err != nil {
		return CreateRequest{}, serviceEnv{}, err
	}

	literal, se, err := classifyServiceEnv(overrides, a)
	if err != nil {
		return CreateRequest{}, serviceEnv{}, err
	}
	allowList, err := manifestAllowEntries(a.IPAllowList, "service", a.Name)
	if err != nil {
		return CreateRequest{}, serviceEnv{}, err
	}

	return CreateRequest{
		Name:                    a.Name,
		Type:                    svcType,
		Schedule:                a.Schedule,
		Repo:                    repo,
		Image:                   image,
		Branch:                  branch,
		Builder:                 extensionBuilder(a.XBex),
		Runtime:                 runtime,
		BuildCommand:            a.BuildCommand,
		StartCommand:            startCommand,
		RootDir:                 a.RootDir,
		BuildFilter:             a.BuildFilter,
		DockerfilePath:          a.DockerfilePath,
		Port:                    0,
		Replicas:                replicas,
		Plan:                    plan,
		HealthCheckPath:         a.HealthCheckPath,
		Env:                     literal,
		Hosts:                   hosts,
		AutoDeploy:              autoDeploy,
		PreDeployCommand:        a.PreDeployCommand,
		InitialDeployHook:       strings.TrimSpace(a.InitialDeployHook),
		MaintenanceMode:         maintenanceMode,
		PublishPath:             publish,
		MaxShutdownDelaySeconds: a.MaxShutdownDelaySeconds,
		Autoscaling:             scalingToAutoscalingRequest(a.Scaling),
		IPAllowList:             allowList,
		SubdomainPolicy:         a.RenderSubdomainPolicy,
		Routes:                  a.Routes,
		Headers:                 a.Headers,
	}, se, nil
}

// validateManifestServiceFields rejects a services[] entry with no name or
// with a manifest field bex does not accept (environmentId, secretFiles,
// builder, image.creds), each with a manifest-shaped message.
func validateManifestServiceFields(a bexService) error {
	if a.Name == "" {
		return fmt.Errorf("%w: a service entry is missing its name", core.ErrBadRequest)
	}
	if a.EnvironmentID != "" {
		return fmt.Errorf("%w: service %q uses environmentId, which is a create-API field, not a Render Blueprint field; nest the service under projects[].environments[].services instead", core.ErrBadRequest, a.Name)
	}
	if len(a.SecretFiles) > 0 {
		return fmt.Errorf("%w: service %q uses secretFiles, which Render's Blueprint schema does not support; use createService secretFiles or the secret-files API", core.ErrBadRequest, a.Name)
	}
	if a.Builder != "" {
		return fmt.Errorf("%w: service %q uses retired Blueprint field builder; move it to x-bex.builder", core.ErrBadRequest, a.Name)
	}
	if a.Image != nil && a.Image.Creds != "" {
		return fmt.Errorf("%w: service %q image.creds is unsupported; bind an authorized registry credential through the service API", core.ErrBadRequest, a.Name)
	}
	return nil
}

// validateManifestIngressFields rejects the fields tied to ingress (or to a
// scalable type) on a services[] entry whose type cannot carry them. Only a
// web service is exposed; private/worker/cron/static have no ingress, so a
// manifest that lists domains for one is a mistake worth catching here with a
// manifest-shaped message.
func validateManifestIngressFields(a bexService, svcType string) error {
	hasIngress := svcType == appv1alpha1.TypeWebService || svcType == appv1alpha1.TypeStaticSite
	if !hasIngress && (len(a.Domains) > 0 || a.Domain != "") {
		return fmt.Errorf("%w: %s has no ingress and cannot list domains", core.ErrBadRequest, a.Name)
	}
	if a.RenderSubdomainPolicy != "" && !hasIngress {
		return fmt.Errorf("%w: service %q renderSubdomainPolicy applies only to web services and static sites", core.ErrBadRequest, a.Name)
	}
	if len(a.IPAllowList) > 0 && !hasIngress {
		return fmt.Errorf("%w: service %q ipAllowList applies only to web services and static sites", core.ErrBadRequest, a.Name)
	}
	if a.Scaling != nil && svcType == appv1alpha1.TypeBackgroundWorker {
		return fmt.Errorf("%w: service %q scaling is not available on background workers", core.ErrBadRequest, a.Name)
	}
	return nil
}

// manifestPlanAndMaintenance resolves one services[] entry's plan and optional
// maintenance-mode block (URI validated here).
func manifestPlanAndMaintenance(a bexService) (string, *MaintenanceModeView, error) {
	plan := a.Plan
	// Render Blueprints default a service carrying this paid-only field to a
	// paid starter plan when no plan is declared.
	if plan == "" && a.MaintenanceMode != nil {
		plan = "starter"
	}
	var maintenanceMode *MaintenanceModeView
	if a.MaintenanceMode != nil {
		maintenanceMode = &MaintenanceModeView{
			Enabled: a.MaintenanceMode.Enabled,
			URI:     a.MaintenanceMode.URI,
		}
		if _, err := parseMaintenanceURI(maintenanceMode.URI); err != nil {
			return "", nil, fmt.Errorf("%w: service %q maintenanceMode.uri: %v", core.ErrBadRequest, a.Name, err)
		}
	}
	return plan, maintenanceMode, nil
}

// manifestStartAndAutoDeploy resolves one services[] entry's effective start
// command (dockerCommand stands in for startCommand on a docker runtime) and
// its auto-deploy setting (autoDeployTrigger wins over the deprecated
// autoDeploy bool).
func manifestStartAndAutoDeploy(a bexService) (string, *bool, error) {
	startCommand := a.StartCommand
	if a.DockerCommand != "" {
		if !strings.EqualFold(a.Runtime, "docker") {
			return "", nil, fmt.Errorf("%w: service %q dockerCommand requires runtime: docker", core.ErrBadRequest, a.Name)
		}
		if startCommand != "" {
			return "", nil, fmt.Errorf("%w: service %q cannot set both dockerCommand and startCommand", core.ErrBadRequest, a.Name)
		}
		startCommand = a.DockerCommand
	}

	autoDeploy := a.AutoDeploy
	if a.AutoDeployTrigger != "" {
		var err error
		autoDeploy, err = parseTrigger(a.AutoDeployTrigger)
		if err != nil {
			return "", nil, fmt.Errorf("service %q: %w", a.Name, err)
		}
	}
	return startCommand, autoDeploy, nil
}

// classifyServiceEnv classifies each env var of one services[] entry
// (all-or-nothing: a bad form rejects the whole apply). A keyless {fromGroup}
// links a whole group. Plain literals (sync unset/true) ride spec.Env and
// re-sync each apply. sync:false + generateValue seed the mutable env store
// once — never spec.Env — so a dashboard edit wins and a later sync neither
// overwrites nor re-mints. fromDatabase/fromService are resolved in pass 2 once
// every name is known.
func classifyServiceEnv(overrides blueprintParseOverrides, a bexService) ([]appv1alpha1.EnvVar, serviceEnv, error) {
	se := serviceEnv{seedLiterals: map[string]string{}, known: map[string]string{}}
	literal := make([]appv1alpha1.EnvVar, 0, len(a.EnvVars))
	for _, e := range a.EnvVars {
		if e.FromGroup != "" {
			if err := validateGroupLink(e); err != nil {
				return nil, serviceEnv{}, fmt.Errorf("%w: %s: %v", core.ErrBadRequest, a.Name, err)
			}
			se.groupLinks = append(se.groupLinks, e.FromGroup)
			continue
		}
		if err := validateKeyedEnv(e); err != nil {
			return nil, serviceEnv{}, fmt.Errorf("%w: %s env %q: %v", core.ErrBadRequest, a.Name, e.Key, err)
		}
		switch {
		case e.FromDatabase != nil, e.FromService != nil:
			se.refVars = append(se.refVars, e)
		case e.GenerateValue:
			se.seedGenerates = append(se.seedGenerates, e.Key)
		case e.Sync != nil && !*e.Sync:
			// A literal seeds immediately; an omitted value is a create-time prompt
			// that DeployRequest.EnvVarValues may seed exactly once.
			if e.Value != "" {
				se.seedLiterals[e.Key] = e.Value
				se.known[e.Key] = e.Value
			} else {
				se.promptKeys = append(se.promptKeys, e.Key)
				if value, ok := overrides.envVarValues[e.Key]; ok {
					se.seedLiterals[e.Key] = value
					se.known[e.Key] = value
				}
			}
		default:
			literal = append(literal, appv1alpha1.EnvVar{Name: e.Key, Value: e.Value})
			se.known[e.Key] = e.Value
		}
	}
	if len(literal) == 0 {
		literal = nil
	}
	if len(se.seedLiterals) == 0 {
		se.seedLiterals = nil
	}
	return literal, se, nil
}

// scalingToAutoscalingRequest converts a render.yaml scaling block into the
// SetAutoscalingRequest that specFromCreate passes to spec.autoscaling.
// nil scaling (absent block) returns nil (no autoscaling on the new service).
func scalingToAutoscalingRequest(s *bexScaling) *SetAutoscalingRequest {
	if s == nil {
		return nil
	}
	return &SetAutoscalingRequest{
		MinInstances:        s.MinInstances,
		MaxInstances:        s.MaxInstances,
		TargetCPUPercent:    s.TargetCPUPercent,
		TargetMemoryPercent: s.TargetMemoryPercent,
	}
}

func extensionBuilder(extension *bexExtension) string {
	if extension == nil {
		return ""
	}
	return extension.Builder
}

// parseDatabase maps one databases[] entry onto a DatabaseSpec. Names are
// validated when applied (specFromCreate's sibling for DBs); plan/ipAllowList
// are validated here against the bex catalog.
func parseDatabase(d bexDatabase) (parsedDatabase, error) {
	if d.Name == "" {
		return parsedDatabase{}, fmt.Errorf("%w: a database entry is missing its name", core.ErrBadRequest)
	}
	if !appv1alpha1.ValidDatabaseName(d.Name) {
		return parsedDatabase{}, fmt.Errorf("%w: database %q name must use lowercase letters, digits, and hyphens, be at most 30 characters, and not start or end with a hyphen", core.ErrBadRequest, d.Name)
	}
	if d.EnvironmentID != "" {
		return parsedDatabase{}, fmt.Errorf("%w: database %q uses environmentId, which is a create-API field, not a Render Blueprint field; nest the database under projects[].environments[].databases instead", core.ErrBadRequest, d.Name)
	}
	for _, unsupported := range []struct {
		name    string
		present bool
	}{
		{"region", d.Region != ""},
		{"previewPlan", d.PreviewPlan != ""},
		{"previewDiskSizeGB", d.PreviewDiskSizeGB != nil},
	} {
		if unsupported.present {
			return parsedDatabase{}, fmt.Errorf("%w: database %q field %q is unsupported because bex has no preview environments or per-resource region placement", core.ErrBadRequest, d.Name, unsupported.name)
		}
	}
	if d.DatabaseName != "" && !appv1alpha1.ValidPostgresIdentifier(d.DatabaseName) {
		return parsedDatabase{}, fmt.Errorf("%w: database %q databaseName must start with a lowercase letter or underscore, contain only lowercase letters, digits, and underscores, and be at most 63 bytes", core.ErrBadRequest, d.Name)
	}
	if d.User != "" && !appv1alpha1.ValidPostgresIdentifier(d.User) {
		return parsedDatabase{}, fmt.Errorf("%w: database %q user must start with a lowercase letter or underscore, contain only lowercase letters, digits, and underscores, and be at most 63 bytes", core.ErrBadRequest, d.Name)
	}
	plan := d.Plan
	// Render's current Blueprint default is a paid basic-256mb instance. This
	// is intentionally a create default; a presence-aware sync leaves an
	// existing database plan unchanged when plan is omitted.
	if plan == "" {
		plan = "basic-256mb"
	}
	if _, ok := tiers.Postgres.ByID(plan); !ok {
		return parsedDatabase{}, fmt.Errorf("%w: database %q plan %q is not a supported Render Postgres plan (one of %s)", core.ErrBadRequest, d.Name, plan, strings.Join(tiers.Postgres.IDs(), "|"))
	}
	allow, err := parseBlueprintIPAllowList("database", d.Name, d.IPAllowList)
	if err != nil {
		return parsedDatabase{}, err
	}
	for _, r := range d.ReadReplicas {
		if r.Name == "" {
			return parsedDatabase{}, fmt.Errorf("%w: database %q has a readReplica without a name", core.ErrBadRequest, d.Name)
		}
	}
	ha := d.HighAvailability != nil && d.HighAvailability.Enabled
	if d.ConnectionPool != "" && d.ConnectionPool != "none" && d.ConnectionPool != "pgbouncer" {
		return parsedDatabase{}, fmt.Errorf("%w: database %q connectionPool must be pgbouncer or none", core.ErrBadRequest, d.Name)
	}
	diskAutoscaling := d.StorageAutoscalingEnabled != nil && *d.StorageAutoscalingEnabled
	spec := appv1alpha1.DatabaseSpec{
		Name:             d.Name,
		DatabaseName:     d.DatabaseName,
		DatabaseUser:     d.User,
		Plan:             plan,
		Version:          d.PostgresMajorVersion,
		StorageGB:        d.DiskSizeGB,
		DiskAutoscaling:  diskAutoscaling,
		Pooler:           d.ConnectionPool == "pgbouncer",
		IPAllowList:      allow,
		ReadReplicas:     d.ReadReplicas,
		HighAvailability: ha,
	}
	return parsedDatabase{name: d.Name, spec: spec}, nil
}

func parseKeyValue(k bexService) (parsedKeyValue, error) {
	if k.Name == "" {
		return parsedKeyValue{}, fmt.Errorf("%w: a key-value service entry is missing its name", core.ErrBadRequest)
	}
	if k.EnvironmentID != "" {
		return parsedKeyValue{}, fmt.Errorf("%w: key-value %q uses environmentId, which is a create-API field, not a Render Blueprint field; nest it under projects[].environments[].services instead", core.ErrBadRequest, k.Name)
	}
	if len(k.SecretFiles) > 0 {
		return parsedKeyValue{}, fmt.Errorf("%w: key-value %q uses secretFiles, which Render's Blueprint schema does not support", core.ErrBadRequest, k.Name)
	}
	if k.Plan != "" {
		if _, ok := tiers.Valkey.ByID(k.Plan); !ok {
			return parsedKeyValue{}, fmt.Errorf("%w: key-value %q plan %q is not a bex Key Value plan (one of %s)", core.ErrBadRequest, k.Name, k.Plan, strings.Join(tiers.Valkey.IDs(), "|"))
		}
	}
	allow, err := parseBlueprintIPAllowList("key-value", k.Name, k.IPAllowList)
	if err != nil {
		return parsedKeyValue{}, err
	}
	if allow == nil {
		// A KeyValue spec carries a non-nil empty list for zero entries — the
		// shape blueprintKeyValueSpecChanged's DeepEqual idempotency compares
		// against, unlike parseDatabase's nil.
		allow = []appv1alpha1.IPAllowEntry{}
	}
	validMaxmemory := map[string]bool{
		"noeviction": true, "allkeys-lru": true, "allkeys-lfu": true,
		"volatile-lru": true, "volatile-lfu": true, "allkeys-random": true,
		"volatile-random": true, "volatile-ttl": true,
	}
	if k.MaxmemoryPolicy != "" && !validMaxmemory[k.MaxmemoryPolicy] {
		return parsedKeyValue{}, fmt.Errorf("%w: key-value %q maxmemoryPolicy %q is invalid", core.ErrBadRequest, k.Name, k.MaxmemoryPolicy)
	}
	if k.PersistenceMode != "" && k.PersistenceMode != "journal-snapshot" && k.PersistenceMode != "snapshot" && k.PersistenceMode != "off" {
		return parsedKeyValue{}, fmt.Errorf("%w: key-value %q persistenceMode %q is invalid", core.ErrBadRequest, k.Name, k.PersistenceMode)
	}
	return parsedKeyValue{name: k.Name, spec: appv1alpha1.KeyValueSpec{
		Name:            k.Name,
		Plan:            k.Plan,
		IPAllowList:     allow,
		MaxmemoryPolicy: k.MaxmemoryPolicy,
		PersistenceMode: k.PersistenceMode,
	}}, nil
}

// parseBlueprintIPAllowList validates one databases[]/key-value ipAllowList
// entry list and projects it onto the CR entry type; label ("database" or
// "key-value") keys the per-entry error wording. Zero entries yield nil — the
// parseKeyValue call site restores its non-nil empty slice.
func parseBlueprintIPAllowList(label, name string, entries []bexIPEntry) ([]appv1alpha1.IPAllowEntry, error) {
	allow, err := manifestAllowEntries(entries, label, name)
	if err != nil {
		return nil, err
	}
	return core.AllowListToSpec(allow), nil
}

// validateGroupLink checks a keyless {fromGroup: <name>} entry: it links a whole
// group and carries no other field. Its target's existence is pre-flighted in
// deployStack (declared in-file OR pre-existing in the workspace).
func validateGroupLink(e bexEnvVar) error {
	if e.Key != "" || e.Value != "" || e.GenerateValue || e.Sync != nil || e.FromDatabase != nil || e.FromService != nil {
		return fmt.Errorf("fromGroup %q must be the only field on its envVars entry (it links the whole group)", e.FromGroup)
	}
	return nil
}

// validateKeyedEnv validates one keyed env var (every form but the keyless
// fromGroup). It enforces Render's field rules: a key is required; value +
// generateValue is a contradiction; generateValue can't combine with a reference;
// a fromService takes EITHER a property (host/port/hostport) OR an envVarKey, not
// both; a fromDatabase property must map onto the CNPG Secret vocabulary.
func validateKeyedEnv(e bexEnvVar) error {
	if e.Key == "" {
		return fmt.Errorf("an envVars entry is missing its key")
	}
	if e.GenerateValue {
		if e.Value != "" {
			return fmt.Errorf("sets both value and generateValue — pick one")
		}
		if e.FromDatabase != nil || e.FromService != nil {
			return fmt.Errorf("generateValue cannot be combined with fromDatabase/fromService")
		}
	}
	if e.FromService != nil {
		ref := e.FromService
		switch {
		case ref.EnvVarKey != "":
			if ref.Property != "" {
				return fmt.Errorf("fromService sets both envVarKey and property — pick one")
			}
		case isKeyValueType(ref.Type):
			if ref.Property == "" {
				return fmt.Errorf("fromService keyvalue needs a property (connectionString, host, port, or password)")
			}
			if _, ok := kvPropertyKey[ref.Property]; !ok {
				return fmt.Errorf("fromService keyvalue property %q is not supported (want connectionString, host, port, or password)", ref.Property)
			}
		case ref.Property == "":
			return fmt.Errorf("fromService needs a property (host, port, hostport) or an envVarKey")
		case !serviceRefProperty[ref.Property]:
			return fmt.Errorf("fromService property %q is not supported (want host, port, or hostport)", ref.Property)
		}
	}
	if e.FromDatabase != nil {
		if _, ok := dbPropertyKey[e.FromDatabase.Property]; !ok {
			return fmt.Errorf("fromDatabase property %q is not supported (want connectionString, host, port, user, password, or database)", e.FromDatabase.Property)
		}
	}
	return nil
}

// resolveDatabaseRef turns a validated fromDatabase entry into a Secret ref
// after applyDatabase has resolved the display name to an immutable resource id.
// Credentials remain indirect so rotation never writes plaintext into an App.
func resolveDatabaseRef(e bexEnvVar, databaseIDs map[string]string) (appv1alpha1.EnvVar, error) {
	ref := e.FromDatabase
	if ref == nil {
		return appv1alpha1.EnvVar{}, fmt.Errorf("not a database reference")
	}
	databaseID := databaseIDs[ref.Name]
	if databaseID == "" {
		return appv1alpha1.EnvVar{}, fmt.Errorf("fromDatabase references unresolved database %q", ref.Name)
	}
	secretName := databaseID + "-app"
	if ref.Property == "connectionPoolString" {
		secretName = databaseID + "-pooler-app"
	}
	return appv1alpha1.EnvVar{
		Name: e.Key,
		ValueFrom: &appv1alpha1.EnvVarSource{
			SecretKeyRef: &appv1alpha1.SecretKeySelector{
				Name: secretName,
				Key:  dbPropertyKey[ref.Property],
			},
		},
	}, nil
}

// resolveKeyValueRef turns a validated fromService (type:keyvalue/redis) entry
// into a SecretKeyRef pointing at the key-value credentials Secret. kvCRNames
// maps display name → CR name; the operator names the Secret after the CR
// (kv.Status.SecretName = kv.Name, docs/ADR021-keyvalue-management.md).
func resolveKeyValueRef(e bexEnvVar, kvCRNames map[string]string) (appv1alpha1.EnvVar, error) {
	ref := e.FromService
	if ref == nil {
		return appv1alpha1.EnvVar{}, fmt.Errorf("not a keyvalue reference")
	}
	crName := kvCRNames[ref.Name]
	if crName == "" {
		return appv1alpha1.EnvVar{}, fmt.Errorf("fromService references unresolved key value %q", ref.Name)
	}
	key := kvPropertyKey[ref.Property]
	return appv1alpha1.EnvVar{
		Name: e.Key,
		ValueFrom: &appv1alpha1.EnvVarSource{
			SecretKeyRef: &appv1alpha1.SecretKeySelector{
				Name: crName,
				Key:  key,
			},
		},
	}, nil
}

// resolveRef turns one fromService env entry into a concrete EnvVar now that
// every service port and parse-time-known env is available. Target existence
// is already checked once by the caller's shared guard.
func resolveRef(e bexEnvVar, ports map[string]int32, knownEnv map[string]map[string]string) (appv1alpha1.EnvVar, error) {
	switch {
	case e.FromService != nil:
		ref := e.FromService
		// envVarKey copies a sibling service's declared var by value (Render's
		// copy-by-value; re-copied on each sync, not live). Same-file resolution:
		// the target must be a service (not a database) with a parse-time-known
		// value (a plain literal or sync:false var — not itself a reference/generate).
		if ref.EnvVarKey != "" {
			env, ok := knownEnv[ref.Name]
			if !ok {
				return appv1alpha1.EnvVar{}, fmt.Errorf("fromService.envVarKey references %q, which is not a service", ref.Name)
			}
			val, ok := env[ref.EnvVarKey]
			if !ok {
				return appv1alpha1.EnvVar{}, fmt.Errorf("fromService.envVarKey references env %q on service %q, which has no such plainly-defined variable", ref.EnvVarKey, ref.Name)
			}
			return appv1alpha1.EnvVar{Name: e.Key, Value: val}, nil
		}
		// host/hostport are classified into parsedService.hostRefs at parse and
		// resolved at apply (the hostname is the sibling's slug, minted at create
		// — docs/ADR041-service-addresses.md D3); only port resolves here.
		if ref.Property == "port" {
			return appv1alpha1.EnvVar{Name: e.Key, Value: strconv.Itoa(int(ports[ref.Name]))}, nil
		}
	}
	return appv1alpha1.EnvVar{}, fmt.Errorf("unsupported env reference")
}

// manifestType maps a render.yaml-style service type (+ optional runtime) to the
// App serviceType. Empty/web => web_service; private and Render's "pserv" =>
// private_service; worker => background_worker; cron => cron_job. A web service
// with runtime "static" => static_site (render.yaml models static sites as
// type:web+runtime:static). An unknown type is rejected with a clear message.
func manifestType(t, runtime string) (string, error) {
	switch t {
	case "", "web", "web_service":
		if strings.EqualFold(runtime, "static") {
			return appv1alpha1.TypeStaticSite, nil
		}
		return appv1alpha1.TypeWebService, nil
	case "private", "pserv", "private_service":
		return appv1alpha1.TypePrivateService, nil
	case "worker", "background_worker":
		return appv1alpha1.TypeBackgroundWorker, nil
	case "cron", "cron_job":
		return appv1alpha1.TypeCronJob, nil
	case "static", "static_site":
		return appv1alpha1.TypeStaticSite, nil
	default:
		return "", fmt.Errorf("%w: unknown service type %q (want web, pserv, worker, or cron)", core.ErrBadRequest, t)
	}
}

// applyCreate is the stack path's idempotent service upsert: create when absent,
// else re-apply the request's owned fields and — unlike the interactive Create
// path — only bump restartedAt (and open a deploy record) when something
// actually changed. An unchanged re-apply is a true no-op: zero spec diff, zero
// new deploy records, no clone-token churn (w1/m24 DoD).
func (s *Service) applyCreate(ctx context.Context, req CreateRequest) (AppView, error) {
	return s.applyCreateWithFields(ctx, req, nil)
}

// applyBlueprintCreate applies a compiled Blueprint service. Unlike the
// interactive create surface, a Blueprint retains source-field presence, so
// an omitted field can preserve an existing value (except buildFilter, whose
// documented omission semantics clear it).
func (s *Service) applyBlueprintCreate(ctx context.Context, req CreateRequest, fields map[string]BlueprintField) (AppView, error) {
	return s.applyCreateWithFields(ctx, req, fields)
}

func (s *Service) applyCreateWithFields(ctx context.Context, req CreateRequest, fields map[string]BlueprintField) (AppView, error) {
	desired, err := specFromCreate(req)
	if err != nil {
		return AppView{}, err
	}
	existing, err := s.GetApp(ctx, core.RelCanCreate, req.Name)
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		return AppView{}, err
	}
	if errors.Is(err, core.ErrNotFound) {
		return s.createFromStack(ctx, req, desired)
	}
	if effectiveType(existing.Spec.Type) != effectiveType(desired.Type) {
		return AppView{}, fmt.Errorf("%w: spec.type is immutable; delete and recreate the service to change type", core.ErrBadRequest)
	}
	markHookRan, initialHookChanged := initialDeployHookState(req, existing, &desired)
	final := existing.DeepCopy()
	specChanged := applyBlueprintServiceSpec(&final.Spec, desired, fields)
	if final.Spec.MaintenanceMode != nil {
		if err := s.validateMaintenanceMode(ctx, final, maintenanceModeView(final.Spec.MaintenanceMode)); err != nil {
			return AppView{}, err
		}
	}
	assignment, environmentChanged, err := s.stackEnvironmentChange(ctx, req, existing)
	if err != nil {
		return AppView{}, err
	}
	// Idempotent update: short-circuit when both create-owned spec and explicit
	// Blueprint grouping already match and no ran-once annotation needs writing.
	if !specChanged && !environmentChanged && !markHookRan && !initialHookChanged {
		return s.view(existing), nil
	}
	// A real change to an EXISTING service via the stack-apply path is the
	// "direct-deploy-override" w6/m19 names: a manual apply overriding what's
	// already running, as opposed to the git-push auto-deploy pipeline
	// (unexported redeploy, webhook-triggered, not guarded). Guarded here, not
	// at deployStack's top, so an unchanged re-apply and a brand-new service
	// (createNewApp above) never need a confirmation.
	if err := s.requireUnprotected(ctx, existing, "deploy"); err != nil {
		return AppView{}, err
	}
	return s.patchChangedStackService(ctx, req, existing, final, stackChanges{
		specChanged:        specChanged,
		environmentChanged: environmentChanged,
		markHookRan:        markHookRan,
		initialHookChanged: initialHookChanged,
		assignment:         assignment,
	})
}

// createFromStack is the stack upsert's create-when-absent half.
func (s *Service) createFromStack(ctx context.Context, req CreateRequest, desired appv1alpha1.AppSpec) (AppView, error) {
	if err := s.validateNewSpecMaintenanceMode(ctx, req.Name, desired); err != nil {
		return AppView{}, err
	}
	// initialDeployHook: use the one-time command as preDeployCommand on first
	// create so the operator runs it on the first deploy (w2/m45).
	if req.InitialDeployHook != "" {
		desired.PreDeployCommand = req.InitialDeployHook
	}
	return s.createNewApp(ctx, req, desired)
}

// stackEnvironmentChange probes the environment half of a stack re-apply:
// whether the request's explicit grouping differs from the existing service's
// environment/project/isolation labels. An unspecified environment reports no
// change.
func (s *Service) stackEnvironmentChange(ctx context.Context, req CreateRequest, existing *appv1alpha1.App) (core.EnvironmentAssignment, bool, error) {
	if !req.EnvironmentSpecified {
		return core.EnvironmentAssignment{}, false, nil
	}
	tenantID := existing.Labels[core.LabelTenant]
	assignment, err := s.resolveEnvironmentForCreate(ctx, req.EnvironmentID, tenantID)
	if err != nil {
		return core.EnvironmentAssignment{}, false, err
	}
	desiredIsolation := ""
	if assignment.NetworkIsolationEnabled {
		desiredIsolation = assignment.ID
	}
	environmentChanged := existing.Labels[core.LabelEnvironment] != assignment.ID ||
		existing.Labels[core.LabelProject] != assignment.ProjectID ||
		existing.Labels[core.LabelNetworkIsolation] != desiredIsolation
	return assignment, environmentChanged, nil
}

// stackChanges is what applyCreateWithFields' change probes computed about an
// existing service: which halves changed and the resolved environment
// assignment to apply.
type stackChanges struct {
	specChanged        bool
	environmentChanged bool
	markHookRan        bool
	initialHookChanged bool
	assignment         core.EnvironmentAssignment
}

// patchChangedStackService stages and patches an existing service the stack
// apply changed: a maintenance-only spec change delegates to
// ConfigureMaintenanceMode; everything else lands in one merge patch, with the
// deploy record, clone Secret, and restartedAt bump reserved for a
// deploy-worthy spec change.
func (s *Service) patchChangedStackService(ctx context.Context, req CreateRequest, existing, final *appv1alpha1.App, changes stackChanges) (AppView, error) {
	maintenanceOnly := changes.specChanged && serviceSpecChangedOnlyByMaintenance(existing.Spec, final.Spec)
	if maintenanceOnly && !changes.environmentChanged {
		return s.ConfigureMaintenanceMode(ctx, req.Name, maintenanceModeView(final.Spec.MaintenanceMode))
	}
	maintenanceChanged := !reflect.DeepEqual(existing.Spec.MaintenanceMode, final.Spec.MaintenanceMode)
	var currentMaintenance *appv1alpha1.MaintenanceModeSpec
	if existing.Spec.MaintenanceMode != nil {
		currentMaintenance = existing.Spec.MaintenanceMode.DeepCopy()
	}
	deploySpecChanged := changes.specChanged && !maintenanceOnly
	base := client.MergeFrom(existing.DeepCopy())
	if deploySpecChanged {
		metav1.SetMetaDataAnnotation(&existing.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(existing.Generation+1, 10))
		// final already holds desired applied over existing (the change probe
		// above), so adopt it instead of running a second apply pass.
		existing.Spec = final.Spec
		if maintenanceChanged {
			existing.Spec.MaintenanceMode = currentMaintenance
		}
	}
	if changes.initialHookChanged {
		metav1.SetMetaDataAnnotation(&existing.ObjectMeta, initialDeployHookAnnotation, req.InitialDeployHook)
	}
	if changes.environmentChanged {
		if err := s.applyEnvironmentChange(ctx, existing, changes.assignment); err != nil {
			return AppView{}, err
		}
	}
	if deploySpecChanged {
		if err := s.recordBlueprintRedeploy(ctx, existing); err != nil {
			return AppView{}, err
		}
		secretName, err := s.ensureCloneSecret(ctx, existing)
		if err != nil {
			return AppView{}, err
		}
		if secretName != "" {
			existing.Spec.CloneSecret = secretName
		}
		existing.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
	}
	// Mark the initialDeployHook as ran so subsequent syncs skip it (w2/m45).
	if changes.markHookRan {
		metav1.SetMetaDataAnnotation(&existing.ObjectMeta, initialDeployHookRanAnnotation, "true")
	}
	resourcemeta.Touch(existing, s.Now())
	if err := s.Client.Patch(ctx, existing, base); err != nil {
		return AppView{}, err
	}
	if s.Kick != nil {
		s.Kick()
	}
	if maintenanceChanged {
		return s.ConfigureMaintenanceMode(ctx, req.Name, maintenanceModeView(final.Spec.MaintenanceMode))
	}
	return s.view(existing), nil
}

// recordBlueprintRedeploy records the blueprint redeploy row for a changed
// existing service; a nil Store or an unmanaged App records nothing.
func (s *Service) recordBlueprintRedeploy(ctx context.Context, existing *appv1alpha1.App) error {
	if s.Store == nil {
		return nil
	}
	if id := managedAppID(existing); id != "" {
		commit := s.resolveDeployCommit(ctx, s.deployWorkspace(ctx, existing), existing.Spec.Repo, existing.Spec.Branch)
		if _, err := s.Store.CreateDeploy(ctx, id, "blueprint", existing.Spec.Image, existing.Generation+1, commit); err != nil {
			return fmt.Errorf("recording redeploy: %w", err)
		}
	}
	return nil
}

// initialDeployHookState is the initialDeployHook ran-once gate (w2/m45) for an
// existing service: once the ran-once annotation is set, desired.PreDeployCommand
// stays the regular one from req. markHookRan reports the first pre-deploy just
// succeeded (mark ran, revert to the regular command); a still-pending hook is
// kept as desired.PreDeployCommand instead. hookChanged reports a not-yet-ran
// hook whose command differs from the recorded annotation.
func initialDeployHookState(req CreateRequest, existing *appv1alpha1.App, desired *appv1alpha1.AppSpec) (markHookRan, hookChanged bool) {
	if req.InitialDeployHook != "" && existing.Annotations[initialDeployHookRanAnnotation] == "" {
		if pd := existing.Status.PreDeploy; pd != nil && pd.Status == appv1alpha1.PreDeploySucceeded {
			// First pre-deploy just succeeded: mark ran, revert to regular command.
			markHookRan = true
		} else {
			// Hook still pending: keep it as the preDeployCommand.
			desired.PreDeployCommand = req.InitialDeployHook
		}
	}
	hookChanged = req.InitialDeployHook != "" &&
		existing.Annotations[initialDeployHookRanAnnotation] == "" &&
		existing.Annotations[initialDeployHookAnnotation] != req.InitialDeployHook
	return markHookRan, hookChanged
}

// applyEnvironmentChange applies the environment half of a Blueprint re-apply
// onto the App's labels/spec and records the assignment on its store row.
func (s *Service) applyEnvironmentChange(ctx context.Context, existing *appv1alpha1.App, assignment core.EnvironmentAssignment) error {
	if assignment.ID == "" {
		delete(existing.Labels, core.LabelProject)
		delete(existing.Labels, core.LabelEnvironment)
		delete(existing.Labels, core.LabelNetworkIsolation)
		// Leaving the environment sheds its inbound-IP layer (w4/m28).
		existing.Spec.EnvironmentIPAllowList = nil
	} else {
		// Joining (or switching) inherits the environment's labels +
		// inbound-IP layer (w4/m28); the delete covers switching to an
		// environment without isolation, which the stamp leaves alone.
		delete(existing.Labels, core.LabelNetworkIsolation)
		stampEnvironmentMembership(existing, assignment)
	}
	if appID := managedAppID(existing); appID != "" {
		environmentStore, ok := s.Store.(EnvironmentAssigner)
		if !ok {
			return core.ErrWorkspacesUnavailable
		}
		if err := environmentStore.SetAppEnvironment(ctx, appID, assignment.ProjectID, assignment.ID); err != nil {
			return fmt.Errorf("assigning Blueprint environment: %w", err)
		}
	}
	return nil
}

func createOwnedSpecChangedOnlyByMaintenance(cur, want appv1alpha1.AppSpec) bool {
	probe := cur
	applyCreateToSpec(&probe, want)
	if reflect.DeepEqual(probe.MaintenanceMode, cur.MaintenanceMode) {
		return false
	}
	probe.MaintenanceMode = cur.MaintenanceMode
	return reflect.DeepEqual(probe, cur)
}

func serviceSpecChangedOnlyByMaintenance(cur, want appv1alpha1.AppSpec) bool {
	if reflect.DeepEqual(cur.MaintenanceMode, want.MaintenanceMode) {
		return false
	}
	probe := want.DeepCopy()
	probe.MaintenanceMode = cur.MaintenanceMode
	return reflect.DeepEqual(*probe, cur)
}

func applyBlueprintServiceSpec(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec, fields map[string]BlueprintField) bool {
	if fields == nil {
		before := *dst
		applyCreateToSpec(dst, want)
		return !reflect.DeepEqual(*dst, before)
	}
	return ApplyBlueprintServiceSpec(dst, want, fields)
}

// createOwnedSpecChanged reports whether applying `want`'s create-owned fields
// onto a copy of `cur` would change it. applyCreateToSpec touches only the
// create-owned set, so the copy equals cur exactly when those fields already
// match want — the idempotency gate (a non-owned field like EnvFromSecret,
// owned by the secrets feature, never trips it).
func createOwnedSpecChanged(cur, want appv1alpha1.AppSpec) bool {
	probe := cur
	applyCreateToSpec(&probe, want)
	return !reflect.DeepEqual(probe, cur)
}

// databaseOwnedSpecChanged is the Database analogue: it reports whether the
// Blueprint-owned Database fields differ, ignoring fields other verbs own
// (suspended/restartedAt/users/recovery/...). Comparing the whole spec would
// see those foreign fields as a change and patch on every re-apply.
func databaseOwnedSpecChanged(cur, want appv1alpha1.DatabaseSpec) bool {
	probe := cur
	applyDatabaseSpec(&probe, want)
	return !reflect.DeepEqual(probe, cur)
}

func blueprintDatabaseSpecChanged(cur, want appv1alpha1.DatabaseSpec, fields map[string]BlueprintField) (bool, error) {
	probe := cur
	return ApplyBlueprintDatabaseSpec(&probe, want, fields)
}

// keyValueOwnedSpecChanged is the KeyValue analogue. Blueprint re-apply owns
// only the fields its schema can declare and must not clear lifecycle state
// such as Suspended or fields managed by another API.
func keyValueOwnedSpecChanged(cur, want appv1alpha1.KeyValueSpec) bool {
	probe := cur
	applyKeyValueSpec(&probe, want)
	return !reflect.DeepEqual(probe, cur)
}

func blueprintKeyValueSpecChanged(cur, want appv1alpha1.KeyValueSpec, fields map[string]BlueprintField) bool {
	probe := cur
	return ApplyBlueprintKeyValueSpec(&probe, want, fields)
}

// applyDatabase is the stack path's idempotent Database upsert: create when
// absent (stamped with the caller's tenant labels, mirroring CreatePostgres),
// else merge-patch the owned spec fields only when they changed. An unchanged
// re-apply is a no-op. databases is deployParsedStack's pre-fetched workspace
// snapshot.
func (s *Service) applyDatabase(ctx context.Context, db parsedDatabase, assignment core.EnvironmentAssignment, databases []appv1alpha1.Database) (StackDatabaseView, error) {
	tenantID, scoped := s.Tenant(ctx)
	existing, duplicate := uniqueDatabaseByDisplayName(databases, scoped, tenantID, db.name)
	if duplicate {
		return StackDatabaseView{}, fmt.Errorf("%w: database name %q is already used more than once in this workspace; run the Postgres name migration before applying this Blueprint", core.ErrConflict, db.name)
	}
	if existing != nil {
		if db.spec.DatabaseName != "" && db.spec.DatabaseName != existing.Spec.EffectiveDatabaseName(existing.Name) {
			return StackDatabaseView{}, fmt.Errorf("%w: database %q databaseName is immutable after creation", core.ErrBadRequest, db.name)
		}
		if db.spec.DatabaseUser != "" && db.spec.DatabaseUser != existing.Spec.EffectiveDatabaseUser(existing.Name) {
			return StackDatabaseView{}, fmt.Errorf("%w: database %q user is immutable after creation", core.ErrBadRequest, db.name)
		}
		if _, diskSizeDeclared := db.fields["diskSizeGB"]; diskSizeDeclared {
			plan, ok := tiers.Postgres.ByID(existing.Spec.Plan)
			if !ok {
				plan = tiers.Postgres.Default()
			}
			allocated := max(plan.StorageGB, existing.Spec.StorageGB, existing.Status.AllocatedStorageGB)
			if db.spec.StorageGB < allocated {
				return StackDatabaseView{}, fmt.Errorf("%w: database %q storage is grow-only: requested %d GB is below the allocated %d GB", core.ErrBadRequest, db.name, db.spec.StorageGB, allocated)
			}
		}
		specChanged, err := blueprintDatabaseSpecChanged(existing.Spec, db.spec, db.fields)
		if err != nil {
			return StackDatabaseView{}, fmt.Errorf("%w: database %q %v", core.ErrBadRequest, db.name, err)
		}
		groupingSpecified := db.grouping != "" || db.ungrouped
		groupingChanged := groupingSpecified && (existing.Labels[core.LabelEnvironment] != assignment.ID || existing.Labels[core.LabelProject] != assignment.ProjectID)
		if !specChanged && !groupingChanged {
			return stackDatabaseView(existing), nil
		}
		base := client.MergeFrom(existing.DeepCopy())
		if specChanged {
			if _, err := ApplyBlueprintDatabaseSpec(&existing.Spec, db.spec, db.fields); err != nil {
				return StackDatabaseView{}, fmt.Errorf("%w: database %q %v", core.ErrBadRequest, db.name, err)
			}
		}
		if groupingChanged {
			existing.Spec.EnvironmentIPAllowList = applyGrouping(existing, assignment)
		}
		resourcemeta.Touch(existing, s.Now())
		if err := s.Client.Patch(ctx, existing, base); err != nil {
			return StackDatabaseView{}, err
		}
		return stackDatabaseView(existing), nil
	}
	d := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: id.New(id.Postgres), Namespace: s.TenantNamespace(tenantID)},
		Spec:       db.spec,
	}
	if scoped {
		d.Labels = map[string]string{core.LabelTenant: tenantID, core.LabelWorkspace: tenantID}
	}
	if assignment.ID != "" {
		d.Spec.EnvironmentIPAllowList = applyGrouping(d, assignment)
	}
	resourcemeta.Touch(d, s.Now())
	if err := s.Client.Create(ctx, d); err != nil {
		return StackDatabaseView{}, err
	}
	return stackDatabaseView(d), nil
}

// applyGrouping stamps the environment/project labels of a Blueprint grouping
// onto a datastore object and returns the environment's projected inbound-IP
// layer (w4/m28) for the caller's Spec.EnvironmentIPAllowList — a grouped
// resource inherits the layer, an ungrouped one sheds labels and layer.
func applyGrouping(obj client.Object, assignment core.EnvironmentAssignment) []string {
	labels := obj.GetLabels()
	if assignment.ID == "" {
		delete(labels, core.LabelProject)
		delete(labels, core.LabelEnvironment)
		return nil
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels[core.LabelProject] = assignment.ProjectID
	labels[core.LabelEnvironment] = assignment.ID
	obj.SetLabels(labels)
	return core.EnvironmentLayerCIDRs(assignment.IPAllowList)
}

func (s *Service) applyKeyValue(ctx context.Context, kv parsedKeyValue, assignment core.EnvironmentAssignment, keyValues []appv1alpha1.KeyValue) (StackKeyValueView, error) {
	// Resolve an existing store by its mutable DISPLAY name within the workspace,
	// not by metadata.name — a store now carries an opaque red- id, so a re-apply
	// of the same render.yaml entry must match on the user-facing name (w9/m6,
	// mirroring applyDatabase). keyValues is deployParsedStack's pre-fetched
	// workspace snapshot.
	tenantID, scoped := s.Tenant(ctx)
	existing, duplicate := uniqueKeyValueByDisplayName(keyValues, scoped, tenantID, kv.name)
	if duplicate {
		return StackKeyValueView{}, fmt.Errorf("%w: key-value name %q is already used more than once in this workspace; run the key-value name migration before applying this Blueprint", core.ErrConflict, kv.name)
	}
	if existing != nil {
		specChanged := blueprintKeyValueSpecChanged(existing.Spec, kv.spec, kv.fields)
		groupingSpecified := kv.grouping != "" || kv.ungrouped
		groupingChanged := groupingSpecified && (existing.Labels[core.LabelEnvironment] != assignment.ID || existing.Labels[core.LabelProject] != assignment.ProjectID)
		if !specChanged && !groupingChanged {
			return stackKeyValueView(existing), nil
		}
		base := client.MergeFrom(existing.DeepCopy())
		if specChanged {
			ApplyBlueprintKeyValueSpec(&existing.Spec, kv.spec, kv.fields)
		}
		if groupingChanged {
			existing.Spec.EnvironmentIPAllowList = applyGrouping(existing, assignment)
		}
		resourcemeta.Touch(existing, s.Now())
		if err := s.Client.Patch(ctx, existing, base); err != nil {
			return StackKeyValueView{}, err
		}
		return stackKeyValueView(existing), nil
	}
	resource := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: id.New(id.KeyValue), Namespace: s.TenantNamespace(tenantID)},
		Spec:       kv.spec,
	}
	if scoped {
		resource.Labels = map[string]string{core.LabelTenant: tenantID, core.LabelWorkspace: tenantID}
	}
	if assignment.ID != "" {
		resource.Spec.EnvironmentIPAllowList = applyGrouping(resource, assignment)
	}
	resourcemeta.Touch(resource, s.Now())
	if err := s.Client.Create(ctx, resource); err != nil {
		return StackKeyValueView{}, err
	}
	return stackKeyValueView(resource), nil
}

func stackKeyValueView(kv *appv1alpha1.KeyValue) StackKeyValueView {
	return StackKeyValueView{ID: kv.Name, Name: kv.Spec.Name, Status: string(kv.Status.Phase)}
}

// applyDatabaseSpec copies the Blueprint-owned Database fields onto dst (the set
// a render.yaml databases[] entry can carry). Fields owned by other verbs (users,
// recovery, suspended, restartedAt, failoverAt) are left untouched, mirroring
// applyCreateToSpec's discipline.
func applyDatabaseSpec(dst *appv1alpha1.DatabaseSpec, want appv1alpha1.DatabaseSpec) {
	dst.Name = want.Name
	dst.Plan = want.Plan
	dst.Version = want.Version
	dst.StorageGB = want.StorageGB
	dst.IPAllowList = want.IPAllowList
	dst.ReadReplicas = want.ReadReplicas
	dst.HighAvailability = want.HighAvailability
}

func applyKeyValueSpec(dst *appv1alpha1.KeyValueSpec, want appv1alpha1.KeyValueSpec) {
	dst.Name = want.Name
	dst.Plan = want.Plan
	dst.IPAllowList = want.IPAllowList
	dst.MaxmemoryPolicy = want.MaxmemoryPolicy
	dst.PersistenceMode = want.PersistenceMode
}

func stackDatabaseView(d *appv1alpha1.Database) StackDatabaseView {
	return StackDatabaseView{ID: d.Name, Name: d.Spec.Name, Status: string(d.Status.Phase)}
}
