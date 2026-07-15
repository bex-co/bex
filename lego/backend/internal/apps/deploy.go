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
	"reflect"
	"strconv"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// deploy.go is the deploy-from-chat mapper (pillar 4): it turns a repo + a
// render.yaml-shaped bex.yml into a stack of CreateRequests + Database specs and
// rides Core.Create, so "deploy this repo (web + worker + postgres)" is one agent
// call — no bespoke deploy endpoint (t001 amended 2026-07-08; multi-service form
// w1/m24). The single-service field mapping mirrors scripts/app-apply.sh, the
// reference bex.yml → CR projection.
//
// A bex.yml carries the render.yaml Blueprint shape: top-level `services:` +
// `databases:` + `envVarGroups:`. The legacy single-service `apps:` list is
// accepted as an alias for `services:` so pre-existing files parse
// byte-identically. All validation is all-or-nothing: one invalid entry rejects
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

// DeployRequest is the deploy-from-chat input: a git repo plus its bex.yml
// (render.yaml-shaped manifest). Repo/Branch, when set, override the manifest's
// per-service repo/branch — an agent that already knows the checkout it is
// deploying need not duplicate it in the file.
type DeployRequest struct {
	// OwnerID is the workspace to deploy INTO — the same optional, membership-checked
	// `ownerId` contract Create has (w6/m14). Empty means the caller's default
	// workspace. Deploy creates a service, so an agent that selected a workspace
	// (MCP select_workspace) must land its deploy there, not in whichever workspace
	// the caller happens to resolve to.
	OwnerID  string
	Repo     string
	Branch   string
	Manifest string
	// Confirm, when set, arms a direct-deploy-override on an EXISTING member
	// service of a protectedStatus=protected Environment (w6/m19,
	// apps/protection.go's requireUnprotected) — the phrase
	// ProtectedConfirmation("deploy", name) for the specific service being
	// overridden. Irrelevant (never checked) for a brand-new service or an
	// unprotected one.
	Confirm string
}

// EnvGroupApplier is the blueprint apply path's seam onto the env-groups feature
// (*envgroups.Service satisfies it structurally): materialize a bex.yml's
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
}

// EnvSeeder is the blueprint apply path's seam onto the env-vars feature
// (*secrets.Service satisfies it structurally): seed a service's sync:false +
// generateValue vars into the mutable env store SEED-ONCE.
type EnvSeeder interface {
	SeedEnvVars(ctx context.Context, service string, literals map[string]string, generates []string) error
}

// StackResult is the set of resources one stack deploy created (or converged):
// databases are applied first (dependents reference them via fromDatabase),
// then services. Both are individually pollable to Ready via their status.
type StackResult struct {
	Services  []AppView           `json:"services"`
	Databases []StackDatabaseView `json:"databases"`
}

// StackDatabaseView is the minimal managed-Postgres projection a stack deploy
// returns — enough for an agent to poll (name + phase). The full Render-shaped
// postgres object (connection info, replicas) is the postgres feature's
// get_postgres / GET /v1/postgres/{id} surface; a stack deploy does not duplicate it.
type StackDatabaseView struct {
	Name   string `json:"name"`
	Status string `json:"status"` // Database phase (Pending|Provisioning|Ready|Failed)
}

// --- manifest shape (render.yaml Blueprint vocabulary) ---

// bexManifest is the render.yaml-shaped bex.yml. Services may be declared under
// either the Blueprint key `services:` or the legacy alias `apps:` (mutually
// exclusive); databases live under `databases:`; env groups under
// `envVarGroups:` (materialized by name at apply, w1/m35).
type bexManifest struct {
	Apps         []bexService  `json:"apps"`         // legacy alias for services (single-service files)
	Services     []bexService  `json:"services"`     // Blueprint services list
	Databases    []bexDatabase `json:"databases"`    // Blueprint databases list
	EnvVarGroups []bexEnvGroup `json:"envVarGroups"` // Blueprint env-groups list
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

// bexService is one entry in services:/apps:. It accepts render.yaml's field
// names (plan, numInstances, type, runtime, domains, staticPublishPath, …) and
// the bex aliases a legacy bex.yml uses (tier, replicas, port, publishPath).
// Fields bex does not honor are parsed elsewhere or rejected with a clear error
// (see docs/ADR018-render-parity.md's Blueprint row for the field-by-field map).
type bexService struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`    // render.yaml short type: web|pserv|worker|cron (empty=web); runtime:static => static_site
	Runtime   string    `json:"runtime"` // render.yaml runtime; "static" selects static_site, "image" => prebuilt
	Plan      string    `json:"plan"`    // render.yaml plan (Render spelling)
	Tier      string    `json:"tier"`    // bex alias for plan
	Repo      string    `json:"repo"`
	Branch    string    `json:"branch"`
	Image     *bexImage `json:"image,omitempty"` // render.yaml image: {url} OR a bare image string (legacy)
	ImagePath string    `json:"imagePath"`       // bex alias: bare prebuilt image
	Builder   string    `json:"builder"`         // bex builder (auto|buildpack|dockerfile)
	RootDir   string    `json:"rootDir"`
	// BuildFilter is render.yaml's Build Filters (paths/ignoredPaths globs) — the
	// same {paths, ignoredPaths} shape every surface uses (BuildFilterView).
	BuildFilter       *BuildFilterView `json:"buildFilter"`
	BuildCommand      string           `json:"buildCommand"`
	StartCommand      string           `json:"startCommand"`
	DockerfilePath    string           `json:"dockerfilePath"` // Render's Dockerfile Path, relative to rootDir; docker runtime only
	NumInstances      int32            `json:"numInstances"`   // render.yaml; alias for replicas
	Replicas          int32            `json:"replicas"`       // bex alias
	Port              int32            `json:"port"`           // bex (Render infers PORT env)
	HealthCheckPath   string           `json:"healthCheckPath"`
	Domains           []string         `json:"domains"`
	Schedule          string           `json:"schedule"`          // cron expression, required when type is cron
	PreDeployCommand  string           `json:"preDeployCommand"`  // render.yaml Pre-Deploy Command (spec.preDeployCommand)
	AutoDeploy        *bool            `json:"autoDeploy"`        // deprecated render.yaml bool; nil => default
	AutoDeployTrigger string           `json:"autoDeployTrigger"` // render.yaml: commit|checksPass|off
	StaticPublishPath string           `json:"staticPublishPath"` // render.yaml static-site publish dir
	PublishPath       string           `json:"publishPath"`       // bex alias
	EnvVars           []bexEnvVar      `json:"envVars"`
}

// bexImage is render.yaml's `image: {url, creds}` — bex honors just the url.
// It also accepts a bare image string (the legacy bex.yml spelling), so a
// service entry can be either `image: nginx:1` or `image: {url: nginx:1}`.
type bexImage struct {
	URL string `json:"url"`
}

// UnmarshalJSON accepts image as either a bare string (legacy) or {url} object
// (render.yaml). sigs.k8s.io/yaml routes YAML through JSON unmarshaling, so this
// covers both the `apps:` legacy form and the `services:` Blueprint form.
func (i *bexImage) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		i.URL = s
		return nil
	}
	var obj struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("image must be a string or {url: ...}: %w", err)
	}
	i.URL = obj.URL
	return nil
}

// bexDatabase is one entry in databases:. Field names follow render.yaml; bex
// honors plan/diskSizeGB/postgresMajorVersion/ipAllowList/readReplicas/
// highAvailability and documents the rest (databaseName, user, region,
// storageAutoscalingEnabled) as omissions.
type bexDatabase struct {
	Name                 string                            `json:"name"`
	Plan                 string                            `json:"plan"`
	DiskSizeGB           int32                             `json:"diskSizeGB"`
	PostgresMajorVersion string                            `json:"postgresMajorVersion"`
	IPAllowList          []bexIPEntry                      `json:"ipAllowList"`
	ReadReplicas         []appv1alpha1.DatabaseReadReplica `json:"readReplicas"`
	HighAvailability     *bexHA                            `json:"highAvailability"`
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
// (fromService only, not yet wired).
type bexFromRef struct {
	Name      string `json:"name"`
	Type      string `json:"type"`      // fromService: the referenced service's type
	Property  string `json:"property"`  // connectionString|host|port|user|password|database|hostport
	EnvVarKey string `json:"envVarKey"` // fromService alternate: copy another env var
}

// --- the parsed stack (validated, env fully resolved) ---

// parsedStack is the all-or-nothing-validated, env-resolved projection of a
// bex.yml: an ordered env-group, database, then service set, ready to apply. Every
// same-file cross-resource reference is already resolved, so apply is a straight
// loop; env groups apply first (services' fromGroup links reference them).
type parsedStack struct {
	envGroups []parsedEnvGroup
	services  []parsedService
	databases []parsedDatabase
}

// parsedEnvGroup is a validated envVarGroups[] entry: literal vars keyed to their
// value, plus the keys whose value the apply mints (generateValue).
type parsedEnvGroup struct {
	name      string
	literals  map[string]string
	generates []string
}

// parsedService is a service ready to apply plus the blueprint env work deferred
// to apply time: fromGroup links, and the sync:false/generateValue vars seeded
// once into the mutable env store (never spec.Env, so a dashboard edit wins).
type parsedService struct {
	req           CreateRequest
	groupLinks    []string          // fromGroup names to link after create
	seedLiterals  map[string]string // sync:false literals, seeded once
	seedGenerates []string          // generateValue keys, minted + seeded once
}

type parsedDatabase struct {
	name string
	spec appv1alpha1.DatabaseSpec
}

// dbPropertyKey maps a render.yaml fromDatabase property to the key in the CNPG
// "<name>-app" connection Secret (the Secret vocabulary
// docs/ADR009-postgresql-management.md documents: username, password, dbname,
// host, port, uri).
var dbPropertyKey = map[string]string{
	"connectionString": "uri",
	"host":             "host",
	"port":             "port",
	"user":             "username",
	"password":         "password",
	"database":         "dbname",
}

// serviceRefProperty is the set of fromService properties bex honors for a
// web/private service (host/port/hostport), resolved to literal env values from
// the same-file service's in-cluster DNS name + declared port. Bare <name>
// resolves because every bex Service is named after its App in one namespace.
var serviceRefProperty = map[string]bool{
	"host": true, "port": true, "hostport": true,
}

// Deploy maps a repo + single-service bex.yml onto one Create — the legacy
// contract (a repo maps to one service). It delegates to DeployStack and returns
// the single service for a one-service, zero-database manifest; a multi-resource
// manifest is rejected here with a pointer to the stack form (the MCP `deploy`
// tool is the stack entry point). Re-applying an unchanged file is an idempotent
// no-op, not a forced redeploy (w1/m24 DoD).
func (s *Service) Deploy(ctx context.Context, req DeployRequest) (AppView, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return AppView{}, err
	}
	res, err := s.deployStack(ctx, req)
	if err != nil {
		return AppView{}, err
	}
	if len(res.Services) != 1 || len(res.Databases) != 0 {
		return AppView{}, fmt.Errorf("%w: bex.yml declares %d service(s) + %d database(s); a single-service deploy takes exactly one service and no databases (use the stack form for multi-resource manifests)", core.ErrBadRequest, len(res.Services), len(res.Databases))
	}
	return res.Services[0], nil
}

// DeployStack applies a whole bex.yml in one call: databases first (dependents
// reference them via fromDatabase), then services, each as an idempotent upsert.
// All validation runs in parseStack before any write — one invalid entry rejects
// the whole apply with a per-entry error, nothing partially created.
func (s *Service) DeployStack(ctx context.Context, req DeployRequest) (StackResult, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return StackResult{}, err
	}
	return s.deployStack(ctx, req)
}

// deployStack is the unauthorized core (Authorize runs once in each entry point)
// — shared by Deploy (single-service) and DeployStack (stack).
func (s *Service) deployStack(ctx context.Context, req DeployRequest) (StackResult, error) {
	st, err := parseStack(req)
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
	// req.Confirm rides the context from here on — applyCreate's protection
	// guard (requireUnprotected) reads it via confirmFrom, the same
	// context seam Delete/Suspend's REST/GraphQL/MCP adapters use.
	ctx = withConfirm(ctx, req.Confirm)
	res := StackResult{}
	// Env groups first: a service's fromGroup links one, which needs the group
	// (and its projection Secret) to exist before the service is patched.
	for _, g := range st.envGroups {
		if err := s.EnvGroups.ApplyEnvGroup(ctx, g.name, g.literals, g.generates); err != nil {
			return res, fmt.Errorf("env group %q: %w", g.name, err)
		}
	}
	// Databases next: a service's fromDatabase env points (via secretRef) at the
	// CNPG "<name>-app" Secret, which only exists once the Database converges —
	// applying the dependent first would leave it Pending on a missing Secret
	// anyway, but applying the DB first starts its provisioning immediately.
	for _, db := range st.databases {
		v, err := s.applyDatabase(ctx, db)
		if err != nil {
			return res, err
		}
		res.Databases = append(res.Databases, v)
	}
	for _, svc := range st.services {
		v, err := s.applyCreate(ctx, svc.req)
		if err != nil {
			return res, err
		}
		// Link fromGroup groups (idempotent) and seed sync:false/generateValue vars
		// (seed-once) now that the service exists.
		for _, g := range svc.groupLinks {
			if err := s.EnvGroups.LinkEnvGroup(ctx, g, svc.req.Name); err != nil {
				return res, fmt.Errorf("linking env group %q to %q: %w", g, svc.req.Name, err)
			}
		}
		if len(svc.seedLiterals) > 0 || len(svc.seedGenerates) > 0 {
			if err := s.EnvSeeder.SeedEnvVars(ctx, svc.req.Name, svc.seedLiterals, svc.seedGenerates); err != nil {
				return res, fmt.Errorf("seeding env for %q: %w", svc.req.Name, err)
			}
		}
		res.Services = append(res.Services, v)
	}
	// Auto-register a blueprint row when called with a repo (w2/m15): lets
	// list_blueprints surface it and sync_blueprint re-apply it later without
	// the caller needing to register it separately.
	if req.Repo != "" {
		s.upsertBlueprint(ctx, req)
	}
	return res, nil
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
		return fmt.Errorf("%w: bex.yml uses envVarGroups/fromGroup but env groups are unavailable (OpenBao not configured)", core.ErrBadRequest)
	}
	if usesSeed && s.EnvSeeder == nil {
		return fmt.Errorf("%w: bex.yml uses sync:false/generateValue but the env-vars store is unavailable (OpenBao not configured)", core.ErrBadRequest)
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

// parseStack parses + fully validates + env-resolves a bex.yml into the ordered
// resource set, with no writes and no cluster access — the all-or-nothing gate.
// Every per-entry problem is reported with the offending name so the caller can
// list it; the whole parse fails on the first error (w1/m24 DoD: nothing
// half-created).
func parseStack(req DeployRequest) (parsedStack, error) {
	var m bexManifest
	if err := yaml.Unmarshal([]byte(req.Manifest), &m); err != nil {
		return parsedStack{}, fmt.Errorf("%w: bex.yml is not valid YAML: %v", core.ErrBadRequest, err)
	}
	services := m.Services
	if len(m.Apps) > 0 {
		if len(m.Services) > 0 {
			return parsedStack{}, fmt.Errorf("%w: use either services: or apps:, not both", core.ErrBadRequest)
		}
		services = m.Apps
	}
	if len(services) == 0 && len(m.Databases) == 0 && len(m.EnvVarGroups) == 0 {
		return parsedStack{}, fmt.Errorf("%w: bex.yml must define at least one service under services: (or apps:), one database under databases:, or one env group under envVarGroups:", core.ErrBadRequest)
	}
	st := parsedStack{}

	// Env groups first (validated + name-deduped); a service's fromGroup links one.
	groupNames := make(map[string]bool, len(m.EnvVarGroups))
	for _, g := range m.EnvVarGroups {
		pg, err := parseEnvGroup(g)
		if err != nil {
			return parsedStack{}, err
		}
		if groupNames[pg.name] {
			return parsedStack{}, fmt.Errorf("%w: duplicate env group name %q", core.ErrBadRequest, pg.name)
		}
		groupNames[pg.name] = true
		st.envGroups = append(st.envGroups, pg)
	}

	// Databases next into the stack (and into the name index services reference).
	names := make(map[string]bool, len(services)+len(m.Databases))
	for _, d := range m.Databases {
		ds, err := parseDatabase(d)
		if err != nil {
			return parsedStack{}, err
		}
		if names[ds.name] {
			return parsedStack{}, fmt.Errorf("%w: duplicate name %q — every service and database name in a bex.yml must be unique", core.ErrBadRequest, ds.name)
		}
		names[ds.name] = true
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
	// servicePorts: name -> effective port, for services that expose a k8s Service
	// (web/private only — workers, cron jobs and static sites have no Service, so a
	// fromService reference to them has no DNS name to resolve). Effective port is
	// the declared one, or the 3000 default when omitted.
	servicePorts := make(map[string]int32, len(services))
	// serviceKnownEnv: name -> the statically-known env of that service (plain
	// literals + sync:false seed values), the resolvable target set for a
	// fromService.envVarKey copy (a from*/generateValue var has no parse-time value).
	serviceKnownEnv := make(map[string]map[string]string, len(services))
	for _, a := range services {
		req, se, err := parseService(req, a)
		if err != nil {
			return parsedStack{}, err
		}
		if names[req.Name] {
			return parsedStack{}, fmt.Errorf("%w: duplicate name %q — every service and database name in a bex.yml must be unique", core.ErrBadRequest, req.Name)
		}
		names[req.Name] = true
		pendings = append(pendings, pending{
			svc:     parsedService{req: req, groupLinks: se.groupLinks, seedLiterals: se.seedLiterals, seedGenerates: se.seedGenerates},
			refVars: se.refVars,
		})
		serviceKnownEnv[req.Name] = se.known
		if req.Type == appv1alpha1.TypeWebService || req.Type == appv1alpha1.TypePrivateService {
			port := req.Port
			if port <= 0 {
				port = 3000
			}
			servicePorts[req.Name] = port
		}
	}

	// Resolve reference env (fromDatabase -> secretRef, fromService property ->
	// literal, fromService.envVarKey -> sibling's value) now that every name, port,
	// and known-env is available; an unknown target names the offender.
	for _, p := range pendings {
		svc := p.svc
		for _, r := range p.refVars {
			ev, err := resolveRef(r, names, servicePorts, serviceKnownEnv)
			if err != nil {
				return parsedStack{}, fmt.Errorf("%w: service %q: %v", core.ErrBadRequest, svc.req.Name, err)
			}
			svc.req.Env = append(svc.req.Env, ev)
		}
		st.services = append(st.services, svc)
	}
	return st, nil
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
	known         map[string]string // this service's parse-time-known env (for a sibling's fromService.envVarKey)
}

// parseService maps one services[] entry onto a CreateRequest (with repo/branch
// overrides applied) plus its classified env (serviceEnv). Structural validation
// (type, schedule, plan, private+domains) happens here.
func parseService(dep DeployRequest, a bexService) (CreateRequest, serviceEnv, error) {
	if a.Name == "" {
		return CreateRequest{}, serviceEnv{}, fmt.Errorf("%w: a service entry is missing its name", core.ErrBadRequest)
	}
	repo := a.Repo
	if dep.Repo != "" {
		repo = dep.Repo // the explicit deploy target wins over the manifest
	}
	branch := a.Branch
	if dep.Branch != "" {
		branch = dep.Branch
	}
	svcType, err := manifestType(a.Type, a.Runtime)
	if err != nil {
		return CreateRequest{}, serviceEnv{}, err
	}
	// Only a web service is exposed; private/worker/cron/static have no ingress,
	// so a manifest that lists domains for one is a mistake worth catching here
	// with a manifest-shaped message.
	if svcType != appv1alpha1.TypeWebService && len(a.Domains) > 0 {
		return CreateRequest{}, serviceEnv{}, fmt.Errorf("%w: %s has no ingress and cannot list domains", core.ErrBadRequest, a.Name)
	}

	// Plan: render.yaml `plan` or the bex `tier` alias (Render spelling accepted).
	plan := a.Plan
	if plan == "" {
		plan = a.Tier
	}
	// Replicas: render.yaml `numInstances` or the bex `replicas` alias.
	replicas := a.NumInstances
	if replicas == 0 {
		replicas = a.Replicas
	}
	// Image: render.yaml `image.url` or the bex bare alias.
	image := a.ImagePath
	if a.Image != nil && a.Image.URL != "" {
		image = a.Image.URL
	}
	// Static publish dir: render.yaml `staticPublishPath` or the bex alias.
	publish := a.StaticPublishPath
	if publish == "" {
		publish = a.PublishPath
	}
	runtime := a.Runtime
	if strings.EqualFold(runtime, "static") {
		runtime = "" // static is represented by the service type, not an App runtime
	}

	autoDeploy := a.AutoDeploy
	if a.AutoDeployTrigger != "" {
		autoDeploy = triggerToAutoDeploy(a.AutoDeployTrigger)
	}

	// Classify each env var (all-or-nothing: a bad form rejects the whole apply).
	// A keyless {fromGroup} links a whole group. Plain literals (sync unset/true)
	// ride spec.Env and re-sync each apply. sync:false + generateValue seed the
	// mutable env store once — never spec.Env — so a dashboard edit wins and a
	// later sync neither overwrites nor re-mints. fromDatabase/fromService are
	// resolved in pass 2 once every name is known.
	se := serviceEnv{seedLiterals: map[string]string{}, known: map[string]string{}}
	literal := make([]appv1alpha1.EnvVar, 0, len(a.EnvVars))
	for _, e := range a.EnvVars {
		if e.FromGroup != "" {
			if err := validateGroupLink(e); err != nil {
				return CreateRequest{}, serviceEnv{}, fmt.Errorf("%w: %s: %v", core.ErrBadRequest, a.Name, err)
			}
			se.groupLinks = append(se.groupLinks, e.FromGroup)
			continue
		}
		if err := validateKeyedEnv(e); err != nil {
			return CreateRequest{}, serviceEnv{}, fmt.Errorf("%w: %s env %q: %v", core.ErrBadRequest, a.Name, e.Key, err)
		}
		switch {
		case e.FromDatabase != nil, e.FromService != nil:
			se.refVars = append(se.refVars, e)
		case e.GenerateValue:
			se.seedGenerates = append(se.seedGenerates, e.Key)
		case e.Sync != nil && !*e.Sync:
			// sync:false with no value: nothing to seed (bex has no dashboard
			// prompt) — accepted, sync-exempt, the user sets it via the env-vars API.
			if e.Value != "" {
				se.seedLiterals[e.Key] = e.Value
				se.known[e.Key] = e.Value
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

	return CreateRequest{
		Name:             a.Name,
		Type:             svcType,
		Schedule:         a.Schedule,
		Repo:             repo,
		Image:            image,
		Branch:           branch,
		Builder:          a.Builder,
		Runtime:          runtime,
		BuildCommand:     a.BuildCommand,
		StartCommand:     a.StartCommand,
		RootDir:          a.RootDir,
		BuildFilter:      a.BuildFilter,
		DockerfilePath:   a.DockerfilePath,
		Port:             a.Port,
		Replicas:         replicas,
		Plan:             plan,
		HealthCheckPath:  a.HealthCheckPath,
		Env:              literal,
		Hosts:            a.Domains,
		AutoDeploy:       autoDeploy,
		PreDeployCommand: a.PreDeployCommand,
		PublishPath:      publish,
	}, se, nil
}

// parseDatabase maps one databases[] entry onto a DatabaseSpec. Names are
// validated when applied (specFromCreate's sibling for DBs); plan/ipAllowList
// are validated here against the bex catalog.
func parseDatabase(d bexDatabase) (parsedDatabase, error) {
	if d.Name == "" {
		return parsedDatabase{}, fmt.Errorf("%w: a database entry is missing its name", core.ErrBadRequest)
	}
	plan := d.Plan
	if plan != "" {
		if _, ok := tiers.Postgres.ByID(plan); !ok {
			return parsedDatabase{}, fmt.Errorf("%w: database %q plan %q is not a bex Postgres plan (one of %s)", core.ErrBadRequest, d.Name, plan, strings.Join(tiers.Postgres.IDs(), "|"))
		}
	}
	var allow []string
	for _, e := range d.IPAllowList {
		if e.Source == "" {
			return parsedDatabase{}, fmt.Errorf("%w: database %q has an ipAllowList entry without a source", core.ErrBadRequest, d.Name)
		}
		if err := core.ValidateCIDRs([]string{e.Source}); err != nil {
			return parsedDatabase{}, fmt.Errorf("%w: database %q ipAllowList: %v", core.ErrBadRequest, d.Name, err)
		}
		allow = append(allow, e.Source)
	}
	for _, r := range d.ReadReplicas {
		if r.Name == "" {
			return parsedDatabase{}, fmt.Errorf("%w: database %q has a readReplica without a name", core.ErrBadRequest, d.Name)
		}
	}
	ha := d.HighAvailability != nil && d.HighAvailability.Enabled
	spec := appv1alpha1.DatabaseSpec{
		Plan:             plan,
		Version:          d.PostgresMajorVersion,
		StorageGB:        d.DiskSizeGB,
		IPAllowList:      allow,
		ReadReplicas:     d.ReadReplicas,
		HighAvailability: ha,
	}
	return parsedDatabase{name: d.Name, spec: spec}, nil
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
		case ref.Type == "keyvalue" || ref.Type == "redis":
			return fmt.Errorf("fromService to a keyvalue connection is not supported")
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

// resolveRef turns one fromDatabase / fromService env entry into a concrete
// EnvVar now that every resource name, service port, and parse-time-known env is
// available. fromDatabase becomes a secretRef into the CNPG "<name>-app"
// connection Secret — never a plaintext copy (survives credential rotation;
// nothing sensitive in the spec). fromService with a property becomes a literal
// (the same-file service's in-cluster host/port); fromService.envVarKey copies the
// sibling's declared var by value. Unknown target names the offender (all-or-nothing).
func resolveRef(e bexEnvVar, names map[string]bool, ports map[string]int32, knownEnv map[string]map[string]string) (appv1alpha1.EnvVar, error) {
	switch {
	case e.FromDatabase != nil:
		ref := e.FromDatabase
		if !names[ref.Name] {
			return appv1alpha1.EnvVar{}, fmt.Errorf("fromDatabase references unknown database %q (declare it under databases: in the same file)", ref.Name)
		}
		return appv1alpha1.EnvVar{
			Name: e.Key,
			ValueFrom: &appv1alpha1.EnvVarSource{
				SecretKeyRef: &appv1alpha1.SecretKeySelector{
					Name: ref.Name + "-app",
					Key:  dbPropertyKey[ref.Property],
				},
			},
		}, nil
	case e.FromService != nil:
		ref := e.FromService
		if !names[ref.Name] {
			return appv1alpha1.EnvVar{}, fmt.Errorf("fromService references unknown service %q (declare it under services: in the same file)", ref.Name)
		}
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
		// Only web/private services expose a k8s Service (a DNS name); referencing
		// a worker/cron/static_site has no resolvable host.
		port, hasService := ports[ref.Name]
		switch ref.Property {
		case "host":
			if !hasService {
				return appv1alpha1.EnvVar{}, fmt.Errorf("fromService host references %q which has no network address (only web/private services are addressable)", ref.Name)
			}
			return appv1alpha1.EnvVar{Name: e.Key, Value: ref.Name}, nil
		case "port":
			return appv1alpha1.EnvVar{Name: e.Key, Value: strconv.Itoa(int(port))}, nil
		case "hostport":
			return appv1alpha1.EnvVar{Name: e.Key, Value: fmt.Sprintf("%s:%d", ref.Name, port)}, nil
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

// triggerToAutoDeploy maps render.yaml's autoDeployTrigger to the spec.autoDeploy
// bool: commit/checksPass => true, off => false.
func triggerToAutoDeploy(trigger string) *bool {
	switch trigger {
	case "off":
		b := false
		return &b
	case "commit", "checksPass":
		b := true
		return &b
	default:
		return nil
	}
}

// applyCreate is the stack path's idempotent service upsert: create when absent,
// else re-apply the request's owned fields and — unlike the interactive Create
// path — only bump restartedAt (and open a deploy record) when something
// actually changed. An unchanged re-apply is a true no-op: zero spec diff, zero
// new deploy records, no clone-token churn (w1/m24 DoD).
func (s *Service) applyCreate(ctx context.Context, req CreateRequest) (AppView, error) {
	desired, err := specFromCreate(req)
	if err != nil {
		return AppView{}, err
	}
	existing, err := s.GetApp(ctx, core.RelCanCreate, req.Name)
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		return AppView{}, err
	}
	if errors.Is(err, core.ErrNotFound) {
		return s.createNewApp(ctx, req, desired)
	}
	// Idempotent update: short-circuit when the create-owned fields already match.
	if !createOwnedSpecChanged(existing.Spec, desired) {
		return view(existing), nil
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
	base := client.MergeFrom(existing.DeepCopy())
	applyCreateToSpec(&existing.Spec, desired)
	if s.Store != nil {
		if id := existing.Labels[store.LabelAppID]; id != "" {
			commit := s.resolveDeployCommit(ctx, s.deployWorkspace(ctx, existing), existing.Spec.Repo, existing.Spec.Branch)
			if _, err := s.Store.CreateDeploy(ctx, id, "blueprint", existing.Spec.Image, existing.Generation, commit); err != nil {
				return AppView{}, fmt.Errorf("recording redeploy: %w", err)
			}
		}
	}
	secretName, err := s.ensureCloneSecret(ctx, existing)
	if err != nil {
		return AppView{}, err
	}
	if secretName != "" {
		existing.Spec.CloneSecret = secretName
	}
	existing.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
	if err := s.Client.Patch(ctx, existing, base); err != nil {
		return AppView{}, err
	}
	if s.Kick != nil {
		s.Kick()
	}
	return view(existing), nil
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

// applyDatabase is the stack path's idempotent Database upsert: create when
// absent (stamped with the caller's tenant labels, mirroring CreatePostgres),
// else merge-patch the owned spec fields only when they changed. An unchanged
// re-apply is a no-op.
func (s *Service) applyDatabase(ctx context.Context, db parsedDatabase) (StackDatabaseView, error) {
	var existing appv1alpha1.Database
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: db.name}, &existing)
	if err == nil {
		if !databaseOwnedSpecChanged(existing.Spec, db.spec) {
			return stackDatabaseView(&existing), nil
		}
		base := client.MergeFrom(existing.DeepCopy())
		applyDatabaseSpec(&existing.Spec, db.spec)
		if err := s.Client.Patch(ctx, &existing, base); err != nil {
			return StackDatabaseView{}, err
		}
		return stackDatabaseView(&existing), nil
	}
	if !apierrors.IsNotFound(err) {
		return StackDatabaseView{}, err
	}
	d := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: db.name, Namespace: s.Namespace},
		Spec:       db.spec,
	}
	if tenantID, ok := s.Tenant(ctx); ok {
		d.Labels = map[string]string{core.LabelTenant: tenantID, core.LabelWorkspace: tenantID}
	}
	if err := s.Client.Create(ctx, d); err != nil {
		return StackDatabaseView{}, err
	}
	return stackDatabaseView(d), nil
}

// applyDatabaseSpec copies the Blueprint-owned Database fields onto dst (the set
// a bex.yml databases[] entry can carry). Fields owned by other verbs (users,
// recovery, suspended, restartedAt, failoverAt) are left untouched, mirroring
// applyCreateToSpec's discipline.
func applyDatabaseSpec(dst *appv1alpha1.DatabaseSpec, want appv1alpha1.DatabaseSpec) {
	dst.Plan = want.Plan
	dst.Version = want.Version
	dst.StorageGB = want.StorageGB
	dst.IPAllowList = want.IPAllowList
	dst.ReadReplicas = want.ReadReplicas
	dst.HighAvailability = want.HighAvailability
}

func stackDatabaseView(d *appv1alpha1.Database) StackDatabaseView {
	return StackDatabaseView{Name: d.Name, Status: string(d.Status.Phase)}
}
