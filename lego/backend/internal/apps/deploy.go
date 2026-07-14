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
// `databases:` (+ `envVarGroups:`, classified not yet wired). The legacy
// single-service `apps:` list is accepted as an alias for `services:` so
// pre-existing files parse byte-identically. All validation is all-or-nothing:
// one invalid entry rejects the whole apply with a per-entry error before any
// resource is written (w1/m24 DoD). Re-applying an unchanged file is a
// per-resource idempotent no-op — no spurious restarts, no new deploy records.

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
// exclusive); databases live under `databases:`. An `envVarGroups:` key, if
// present, is ignored (yaml.Unmarshal drops unknown fields) — the m16
// env-groups feature exists but isn't name-keyed the Blueprint way; a `fromGroup`
// env reference is rejected in validateEnvForm (w1/m24 documented omission).
type bexManifest struct {
	Apps      []bexService  `json:"apps"`      // legacy alias for services (single-service files)
	Services  []bexService  `json:"services"`  // Blueprint services list
	Databases []bexDatabase `json:"databases"` // Blueprint databases list
}

// bexService is one entry in services:/apps:. It accepts render.yaml's field
// names (plan, numInstances, type, runtime, domains, staticPublishPath, …) and
// the bex aliases a legacy bex.yml uses (tier, replicas, port, publishPath).
// Fields bex does not honor are parsed elsewhere or rejected with a clear error
// (see docs/ADR018-render-parity.md's Blueprint row for the field-by-field map).
type bexService struct {
	Name              string      `json:"name"`
	Type              string      `json:"type"`    // render.yaml short type: web|pserv|worker|cron (empty=web); runtime:static => static_site
	Runtime           string      `json:"runtime"` // render.yaml runtime; "static" selects static_site, "image" => prebuilt
	Plan              string      `json:"plan"`    // render.yaml plan (Render spelling)
	Tier              string      `json:"tier"`    // bex alias for plan
	Repo              string      `json:"repo"`
	Branch            string      `json:"branch"`
	Image             *bexImage   `json:"image,omitempty"` // render.yaml image: {url} OR a bare image string (legacy)
	ImagePath         string      `json:"imagePath"`       // bex alias: bare prebuilt image
	Builder           string      `json:"builder"`         // bex builder (auto|buildpack|dockerfile)
	RootDir           string      `json:"rootDir"`
	BuildCommand      string      `json:"buildCommand"`
	StartCommand      string      `json:"startCommand"`
	DockerfilePath    string      `json:"dockerfilePath"` // Render's Dockerfile Path, relative to rootDir; docker runtime only
	NumInstances      int32       `json:"numInstances"`   // render.yaml; alias for replicas
	Replicas          int32       `json:"replicas"`       // bex alias
	Port              int32       `json:"port"`           // bex (Render infers PORT env)
	HealthCheckPath   string      `json:"healthCheckPath"`
	Domains           []string    `json:"domains"`
	Schedule          string      `json:"schedule"`          // cron expression, required when type is cron
	PreDeployCommand  string      `json:"preDeployCommand"`  // render.yaml Pre-Deploy Command (spec.preDeployCommand)
	AutoDeploy        *bool       `json:"autoDeploy"`        // deprecated render.yaml bool; nil => default
	AutoDeployTrigger string      `json:"autoDeployTrigger"` // render.yaml: commit|checksPass|off
	StaticPublishPath string      `json:"staticPublishPath"` // render.yaml static-site publish dir
	PublishPath       string      `json:"publishPath"`       // bex alias
	EnvVars           []bexEnvVar `json:"envVars"`
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

// bexEnvVar is one envVars entry. A literal {key,value} maps to a plain env var;
// fromDatabase / fromService reference another resource in the same file and
// resolve to a secretRef (database) or a literal (service host/port). The other
// render.yaml forms (generateValue, sync:false, fromGroup) are rejected with a
// named error — bex can't honor them at blueprint time, and silently dropping a
// variable the app needs is worse than a clear all-or-nothing rejection.
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
// bex.yml: an ordered database set then service set, ready to apply. Every
// cross-resource reference is already resolved, so apply is a straight loop.
type parsedStack struct {
	services  []parsedService
	databases []parsedDatabase
}

type parsedService struct {
	req CreateRequest
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
	// req.Confirm rides the context from here on — applyCreate's protection
	// guard (requireUnprotected) reads it via confirmFrom, the same
	// context seam Delete/Suspend's REST/GraphQL/MCP adapters use.
	ctx = withConfirm(ctx, req.Confirm)
	res := StackResult{}
	// Databases first: a service's fromDatabase env points (via secretRef) at the
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
	if len(services) == 0 && len(m.Databases) == 0 {
		return parsedStack{}, fmt.Errorf("%w: bex.yml must define at least one service under services: (or apps:) or one database under databases:", core.ErrBadRequest)
	}
	st := parsedStack{}

	// Databases first into the stack (and into the name index services reference).
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
		req     CreateRequest
		refVars []bexEnvVar
	}
	pendings := make([]pending, 0, len(services))
	// servicePorts: name -> effective port, for services that expose a k8s Service
	// (web/private only — workers, cron jobs and static sites have no Service, so a
	// fromService reference to them has no DNS name to resolve). Effective port is
	// the declared one, or the 3000 default when omitted.
	servicePorts := make(map[string]int32, len(services))
	for _, a := range services {
		req, refs, err := parseService(req, a)
		if err != nil {
			return parsedStack{}, err
		}
		if names[req.Name] {
			return parsedStack{}, fmt.Errorf("%w: duplicate name %q — every service and database name in a bex.yml must be unique", core.ErrBadRequest, req.Name)
		}
		names[req.Name] = true
		pendings = append(pendings, pending{req: req, refVars: refs})
		if req.Type == appv1alpha1.TypeWebService || req.Type == appv1alpha1.TypePrivateService {
			port := req.Port
			if port <= 0 {
				port = 3000
			}
			servicePorts[req.Name] = port
		}
	}

	// Resolve reference env (fromDatabase -> secretRef, fromService -> literal)
	// now that every name + port is known; an unknown target names the offender.
	for _, p := range pendings {
		env := p.req.Env
		for _, r := range p.refVars {
			ev, err := resolveRef(r, names, servicePorts)
			if err != nil {
				return parsedStack{}, fmt.Errorf("%w: service %q: %v", core.ErrBadRequest, p.req.Name, err)
			}
			env = append(env, ev)
		}
		p.req.Env = env
		st.services = append(st.services, parsedService{req: p.req})
	}
	return st, nil
}

// parseService maps one services[] entry onto a CreateRequest (with repo/branch
// overrides applied) plus the list of its reference envVars still to resolve.
// Structural validation (type, schedule, plan, private+domains) happens here.
func parseService(dep DeployRequest, a bexService) (CreateRequest, []bexEnvVar, error) {
	if a.Name == "" {
		return CreateRequest{}, nil, fmt.Errorf("%w: a service entry is missing its name", core.ErrBadRequest)
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
		return CreateRequest{}, nil, err
	}
	// Only a web service is exposed; private/worker/cron/static have no ingress,
	// so a manifest that lists domains for one is a mistake worth catching here
	// with a manifest-shaped message.
	if svcType != appv1alpha1.TypeWebService && len(a.Domains) > 0 {
		return CreateRequest{}, nil, fmt.Errorf("%w: %s has no ingress and cannot list domains", core.ErrBadRequest, a.Name)
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

	// Split env into literals (carried now) and references (resolved later once
	// every name is known). Unsupported forms are rejected here (all-or-nothing).
	literal := make([]appv1alpha1.EnvVar, 0, len(a.EnvVars))
	var refs []bexEnvVar
	for _, e := range a.EnvVars {
		if err := validateEnvForm(e); err != nil {
			return CreateRequest{}, nil, fmt.Errorf("%w: %s env %q: %v", core.ErrBadRequest, a.Name, e.Key, err)
		}
		switch {
		case e.FromDatabase != nil, e.FromService != nil:
			refs = append(refs, e)
		default:
			literal = append(literal, appv1alpha1.EnvVar{Name: e.Key, Value: e.Value})
		}
	}
	if len(literal) == 0 {
		literal = nil
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
	}, refs, nil
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

// validateEnvForm rejects env-var shapes bex can't honor at blueprint time so a
// user isn't surprised by a missing variable: generateValue (no blueprint-time
// secret gen — use the env-vars API), sync:false (dashboard-prompted secrets,
// no bex equivalent), and fromGroup (env-groups not name-keyed the Blueprint
// way). Supported forms (literal, fromDatabase, fromService) are left for the
// caller to split.
func validateEnvForm(e bexEnvVar) error {
	if e.Key == "" {
		return fmt.Errorf("an envVars entry is missing its key")
	}
	if e.GenerateValue {
		return fmt.Errorf("generateValue is not supported (set secret values via the env-vars API)")
	}
	if e.Sync != nil && !*e.Sync {
		return fmt.Errorf("sync:false is not supported (set secret values via the env-vars API)")
	}
	if e.FromGroup != "" {
		return fmt.Errorf("fromGroup is not supported yet (envVarGroups are not wired into stack deploys)")
	}
	if e.FromService != nil {
		if e.FromService.EnvVarKey != "" {
			return fmt.Errorf("fromService.envVarKey is not supported yet")
		}
		if e.FromService.Type == "keyvalue" || e.FromService.Type == "redis" {
			return fmt.Errorf("fromService to a keyvalue connection is not supported yet")
		}
		if e.FromService.Property != "" && !serviceRefProperty[e.FromService.Property] {
			return fmt.Errorf("fromService property %q is not supported (want host, port, or hostport)", e.FromService.Property)
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
// EnvVar now that every resource name (and service port) is known. fromDatabase
// becomes a secretRef into the CNPG "<name>-app" connection Secret — never a
// plaintext copy (survives credential rotation; nothing sensitive in the spec).
// fromService becomes a literal (the same-file service's in-cluster host/port).
// Unknown target names the offender (all-or-nothing).
func resolveRef(e bexEnvVar, names map[string]bool, ports map[string]int32) (appv1alpha1.EnvVar, error) {
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
			if _, err := s.Store.CreateDeploy(ctx, id, "blueprint", existing.Spec.Image, existing.Generation); err != nil {
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
