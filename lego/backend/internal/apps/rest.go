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
	"fmt"
	"io"
	"mime"
	"net/http"
	"slices"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// patchServiceRequest is the subset of Render's PATCH /v1/services/{id} body
// bex honors: plan and type-specific details nested under serviceDetails plus
// Render's top-level identity, source, and deploy settings. PATCH is partial —
// recognized fields that bex cannot honor are rejected explicitly, so an
// official CLI command never reports success after silently dropping intent;
// omitting a field leaves it unchanged.
type patchServiceRequest struct {
	ServiceDetails *struct {
		Plan             string          `json:"plan"`
		HealthCheckPath  *string         `json:"healthCheckPath"`
		PreDeployCommand *string         `json:"preDeployCommand"`
		Schedule         *string         `json:"schedule"`
		PublishPath      *string         `json:"publishPath"`
		BuildCommand     *string         `json:"buildCommand"`
		EnvSpecific      json.RawMessage `json:"envSpecificDetails"`
		Previews         json.RawMessage `json:"previews"`
		MaintenanceMode  json.RawMessage `json:"maintenanceMode"`
		IPAllowList      json.RawMessage `json:"ipAllowList"`
		// IdleTTLSeconds is a bex extra (Render has no idle-timeout field) — the
		// free-tier auto-sleep window. A pointer so "absent" (leave unchanged) is
		// distinct from an explicit 0 (restore the controller default).
		IdleTTLSeconds *int32 `json:"idleTTLSeconds"`
		// MaxShutdownDelaySeconds is Render's graceful SIGTERM window. The
		// custom optional integer preserves omission for PATCH and returns a
		// field-named 400 for strings, fractions, booleans, or null.
		MaxShutdownDelaySeconds optionalInt32 `json:"maxShutdownDelaySeconds"`
	} `json:"serviceDetails"`
	// Render calls the mutable human-facing label `name`; displayName remains a
	// bex extension accepted for dashboard/backward compatibility.
	Name *string `json:"name"`
	Repo *string `json:"repo"`
	// Image is present when the official CLI updates an image-backed service.
	Image  *imageRef `json:"image"`
	Branch *string   `json:"branch"`
	// DisplayName is the mutable human label. A pointer distinguishes omission
	// (leave unchanged) from an explicit empty string (clear and fall back to the
	// immutable App name).
	DisplayName *string `json:"displayName"`
	// RootDir is a pointer so "absent" (leave unchanged) is distinct from an
	// explicit "" (restore the repo root) — Render's Root Directory setting,
	// the Settings → Build & Deploy save flow (w5/m13).
	RootDir *string `json:"rootDir"`
	// BuildFilter is Render's Build Filters object (top-level, verified against
	// Render's servicePATCH schema). A pointer so "absent" (leave unchanged) is
	// distinct from an explicit object (set, or clear when both arrays are empty)
	// — the Settings → Build & Deploy save flow (w1/m34).
	BuildFilter *BuildFilterView `json:"buildFilter"`
	// AutoDeploy is Render's "yes"/"no" (or bool-ish) toggle for push-to-deploy
	// (spec.autoDeploy). "" => absent (leave unchanged); parseYesNo maps the rest
	// to a tri-state so the Settings → Build & Deploy toggle can flip it (w2/m9).
	AutoDeploy string `json:"autoDeploy"`
	// NotifyOnFail is Render's exact per-service notifyOnFail enum (default |
	// notify | ignore, docs/render-artifacts/notify-on-fail.md). A pointer so
	// "absent" (leave unchanged) is distinct from an explicit value; an
	// unrecognized value is core.ErrBadRequest.
	NotifyOnFail *string `json:"notifyOnFail"`
	// Schedule + Command are a cron_job's schedule expression and entrypoint
	// override (w5/m18). Only honored when the target App is a cron_job
	// (core.ErrBadRequest otherwise). Pointers so "absent" (leave unchanged) is
	// distinct from an explicit ""; schedule must be a non-empty 5-field crontab.
	Schedule *string `json:"schedule"`
	Command  *string `json:"command"`
	// HealthCheckPath is the HTTP path the ReadinessProbe pings (w1/m23/t001 +
	// w5/009). A pointer so "absent" is distinct from an explicit "" (reset to "/").
	HealthCheckPath *string `json:"healthCheckPath"`
	// PreDeployCommand is Render's Pre-Deploy Command (w1/m33). A pointer so
	// "absent" (leave unchanged) is distinct from an explicit "" (clear the step).
	PreDeployCommand *string `json:"preDeployCommand"`
	// DryRun, when true, previews the plan change without writing (w2/m29).
	// Honored when serviceDetails.plan is set; other PATCH fields are not previewed.
	// Can also be set via the ?dryRun=true query parameter.
	DryRun bool `json:"dryRun,omitempty"`
}

// scaleRequest is Render's POST /v1/services/{id}/scale body: the desired
// running instance count. numInstances < 1 or > 100 is core.ErrBadRequest.
type scaleRequest struct {
	NumInstances int32 `json:"numInstances"`
}

// optionalInt32 distinguishes an omitted JSON property from an explicit value
// without accepting encoding/json's usual float/string coercion. It is used by
// maxShutdownDelaySeconds on both create and PATCH so malformed types get the
// same named error before reaching the shared range/type validation.
type optionalInt32 struct {
	Value int32
	Set   bool
}

func (v *optionalInt32) UnmarshalJSON(data []byte) error {
	var value int32
	if string(data) == "null" || json.Unmarshal(data, &value) != nil {
		return fmt.Errorf("maxShutdownDelaySeconds must be an integer")
	}
	v.Value = value
	v.Set = true
	return nil
}

// createServiceRequest is the POST /v1/services body — shaped to Render's create
// schema (verified against its public API: top-level name/repo/branch/image
// object/envVars, plan + numInstances + healthCheckPath nested under
// serviceDetails, type used to pick private_service). bex reads the Render
// fields it can honor, including native runtime/build/start commands, and adds
// a few extensions. Region remains a one-region platform concern. One of
// repo/image is required.
type createServiceRequest struct {
	// OwnerID is the workspace to create the service IN (Render's `ownerId`,
	// w6/m14). Render requires it; bex keeps it OPTIONAL — omitted, the service
	// lands in the caller's default workspace (their oldest membership), which
	// keeps every single-workspace client working unchanged. Naming a workspace
	// the caller is not a member of is 403, never a silent create somewhere else.
	OwnerID string `json:"ownerId"`
	// Render fields.
	Type          string    `json:"type"`     // web_service (default) | private_service | background_worker | cron_job
	Schedule      string    `json:"schedule"` // cron expression, required when type is cron_job
	Command       string    `json:"command"`  // overrides the image's entrypoint for a cron_job; empty runs its own command
	Name          string    `json:"name"`
	Repo          string    `json:"repo"`
	Image         *imageRef `json:"image"` // prebuilt image: Render nests the path in an object
	Branch        string    `json:"branch"`
	EnvironmentID string    `json:"environmentId"`
	AutoDeploy    string    `json:"autoDeploy"` // Render's "yes"|"no"; "" => default
	// NotifyOnFail is Render's exact per-service notifyOnFail enum (default |
	// notify | ignore); "" => default (docs/render-artifacts/notify-on-fail.md).
	NotifyOnFail   string             `json:"notifyOnFail"`
	EnvVars        []keyValue         `json:"envVars"`
	SecretFiles    []secretFileInput  `json:"secretFiles"`
	ServiceDetails *serviceDetailsReq `json:"serviceDetails"`
	// bex extensions (no Render create-body equivalent): the build strategy, the
	// listen port (Render auto-detects it; bex's App CR needs it explicitly),
	// custom domains in one call, and a top-level plan convenience.
	Builder string `json:"builder"`
	// RootDir scopes build-from-git to a subdirectory of Repo, mirroring
	// Render's Root Directory setting (monorepo support). Empty is the repo root.
	RootDir string `json:"rootDir"`
	// BuildFilter is Render's Build Filters object (top-level, verified against
	// Render's servicePOST schema): glob patterns gating git-push auto-deploys.
	BuildFilter *BuildFilterView `json:"buildFilter"`
	Port        int32            `json:"port"`
	Plan        string           `json:"plan"`
	Domains     []string         `json:"domains"`
	// PublishPath is a static_site's publish directory; a top-level convenience
	// mirroring serviceDetails.publishPath (top level wins).
	PublishPath string `json:"publishPath"`
	// PreDeployCommand is Render's Pre-Deploy Command (w1/m33); a top-level
	// convenience mirroring serviceDetails.preDeployCommand (top level wins).
	PreDeployCommand string `json:"preDeployCommand"`
	// Routes/Headers are a static_site's edge rules at create time (Render sets
	// these via separate endpoints; bex also accepts them in the create body).
	Routes  []renderRoute  `json:"routes"`
	Headers []renderHeader `json:"headers"`
	// DryRun, when true, resolves the spec and returns a preview without any
	// Kubernetes or store writes — zero side effects (w2/m29). Response status
	// is 200 (not 201 Created) to signal that no resource was actually created.
	// Can also be set via the ?dryRun=true query parameter.
	DryRun bool `json:"dryRun,omitempty"`
}

type keyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type secretFileInput struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// imageRef is Render's prebuilt-image object: the image path lives under
// `image.imagePath`, not a bare top-level string. An explicit registry
// credential is decoded so unsupportedField can reject it honestly.
type imageRef struct {
	ImagePath            string          `json:"imagePath"`
	RegistryCredentialID json.RawMessage `json:"registryCredentialId"`
}

// serviceDetailsReq is where Render nests plan, numInstances and healthCheckPath
// on create — the same location PATCH and GET report them. schedule is Render's
// cronJobDetails.schedule (accepted here or at the top level, top level wins).
type serviceDetailsReq struct {
	Plan                    string        `json:"plan"`
	NumInstances            int32         `json:"numInstances"`
	HealthCheckPath         string        `json:"healthCheckPath"`
	MaxShutdownDelaySeconds optionalInt32 `json:"maxShutdownDelaySeconds"`
	Runtime                 string        `json:"runtime"`
	// Env is Render's deprecated spelling for Runtime; Runtime wins.
	Env                string                 `json:"env"`
	EnvSpecificDetails *envSpecificDetailsReq `json:"envSpecificDetails"`
	Schedule           string                 `json:"schedule"`
	Command            string                 `json:"command"`
	// BuildCommand is staticSiteDetails.buildCommand. Runtime-backed service
	// types carry it inside envSpecificDetails instead.
	BuildCommand string `json:"buildCommand"`
	// PublishPath is Render's staticSiteDetails.publishPath — the built output
	// directory a static_site serves. Accepted here or at the top level (top
	// level wins), mirroring schedule/command.
	PublishPath string `json:"publishPath"`
	// PreDeployCommand is Render's serviceDetails.preDeployCommand — the command
	// run against the new image before it serves traffic (w1/m33). Accepted here
	// (Render-faithful) or at the top level (top level wins).
	PreDeployCommand string          `json:"preDeployCommand"`
	Previews         json.RawMessage `json:"previews"`
	MaintenanceMode  json.RawMessage `json:"maintenanceMode"`
	IPAllowList      json.RawMessage `json:"ipAllowList"`
}

type envSpecificDetailsReq struct {
	BuildCommand         string          `json:"buildCommand"`
	StartCommand         string          `json:"startCommand"`
	DockerCommand        string          `json:"dockerCommand"`
	DockerContext        string          `json:"dockerContext"`
	DockerfilePath       string          `json:"dockerfilePath"`
	RegistryCredentialID json.RawMessage `json:"registryCredentialId"`
}

// unsupportedField names an official-CLI field that the platform cannot yet
// enforce. Rejecting it is materially safer than accepting a command whose
// requested runtime behavior is then absent.
func (r createServiceRequest) unsupportedField() string {
	if r.Image != nil && len(r.Image.RegistryCredentialID) > 0 {
		return "registryCredentialId"
	}
	if r.ServiceDetails == nil {
		return ""
	}
	if len(r.ServiceDetails.Previews) > 0 {
		return "previews"
	}
	if len(r.ServiceDetails.MaintenanceMode) > 0 {
		return "maintenanceMode"
	}
	if len(r.ServiceDetails.IPAllowList) > 0 {
		return "ipAllowList"
	}
	if r.ServiceDetails.EnvSpecificDetails != nil && len(r.ServiceDetails.EnvSpecificDetails.RegistryCredentialID) > 0 {
		return "registryCredentialId"
	}
	return ""
}

func (r patchServiceRequest) unsupportedField() string {
	if r.Image != nil && len(r.Image.RegistryCredentialID) > 0 {
		return "registryCredentialId"
	}
	if r.ServiceDetails == nil {
		return ""
	}
	if len(r.ServiceDetails.Previews) > 0 {
		return "previews"
	}
	if len(r.ServiceDetails.MaintenanceMode) > 0 {
		return "maintenanceMode"
	}
	if len(r.ServiceDetails.IPAllowList) > 0 {
		return "ipAllowList"
	}
	if len(r.ServiceDetails.EnvSpecific) > 0 {
		var fields map[string]json.RawMessage
		if json.Unmarshal(r.ServiceDetails.EnvSpecific, &fields) == nil && len(fields["registryCredentialId"]) > 0 {
			return "registryCredentialId"
		}
	}
	return ""
}

// toCreateRequest folds the Render-nested and bex top-level fields into the
// neutral CreateRequest. serviceDetails is Render's canonical location for
// plan/numInstances/healthCheckPath; the top-level plan is a bex convenience
// fallback. type:private_service maps to the in-cluster-only flag.
func (r createServiceRequest) toCreateRequest() CreateRequest {
	plan, health, schedule, command, publishPath := r.Plan, "", r.Schedule, r.Command, r.PublishPath
	rootDir := r.RootDir
	var runtime, buildCommand, startCommand, dockerfilePath string
	preDeploy := r.PreDeployCommand
	var replicas int32
	var maxShutdownDelaySeconds *int32
	if r.ServiceDetails != nil {
		if r.ServiceDetails.Plan != "" {
			plan = r.ServiceDetails.Plan
		}
		health = r.ServiceDetails.HealthCheckPath
		runtime = r.ServiceDetails.Runtime
		if runtime == "" {
			runtime = r.ServiceDetails.Env
		}
		if r.ServiceDetails.EnvSpecificDetails != nil {
			buildCommand = r.ServiceDetails.EnvSpecificDetails.BuildCommand
			startCommand = r.ServiceDetails.EnvSpecificDetails.StartCommand
			if strings.EqualFold(runtime, "docker") {
				startCommand = r.ServiceDetails.EnvSpecificDetails.DockerCommand
				dockerfilePath = r.ServiceDetails.EnvSpecificDetails.DockerfilePath
				if rootDir == "" {
					rootDir = r.ServiceDetails.EnvSpecificDetails.DockerContext
				}
			}
		}
		if r.ServiceDetails.BuildCommand != "" {
			buildCommand = r.ServiceDetails.BuildCommand
		}
		replicas = r.ServiceDetails.NumInstances
		if r.ServiceDetails.MaxShutdownDelaySeconds.Set {
			value := r.ServiceDetails.MaxShutdownDelaySeconds.Value
			maxShutdownDelaySeconds = &value
		}
		if schedule == "" {
			schedule = r.ServiceDetails.Schedule // top-level schedule wins over the nested one
		}
		if command == "" {
			command = r.ServiceDetails.Command // top-level command wins over the nested one
		}
		if command == "" && r.Type == appv1alpha1.TypeCronJob {
			command = startCommand // official CLI encodes cron command in envSpecificDetails.startCommand
		}
		if publishPath == "" {
			publishPath = r.ServiceDetails.PublishPath // top-level publishPath wins over the nested one
		}
		if preDeploy == "" {
			preDeploy = r.ServiceDetails.PreDeployCommand // top-level preDeployCommand wins over the nested one
		}
	}
	image := ""
	if r.Image != nil {
		image = r.Image.ImagePath
	}
	var env []appv1alpha1.EnvVar
	for _, e := range r.EnvVars {
		env = append(env, appv1alpha1.EnvVar{Name: e.Key, Value: e.Value})
	}
	secretFiles := make([]core.SecretFile, 0, len(r.SecretFiles))
	for _, f := range r.SecretFiles {
		secretFiles = append(secretFiles, core.SecretFile{Name: f.Name, Content: f.Content})
	}
	return CreateRequest{
		OwnerID:                 r.OwnerID,
		EnvironmentID:           r.EnvironmentID,
		Name:                    r.Name,
		Type:                    r.Type,
		Schedule:                schedule,
		Command:                 command,
		Repo:                    r.Repo,
		Image:                   image,
		Branch:                  r.Branch,
		Builder:                 r.Builder,
		Runtime:                 runtime,
		BuildCommand:            buildCommand,
		StartCommand:            startCommand,
		RootDir:                 rootDir,
		BuildFilter:             r.BuildFilter,
		DockerfilePath:          dockerfilePath,
		Port:                    r.Port,
		Replicas:                replicas,
		Plan:                    plan,
		HealthCheckPath:         health,
		MaxShutdownDelaySeconds: maxShutdownDelaySeconds,
		Env:                     env,
		SecretFiles:             secretFiles,
		Hosts:                   r.Domains,
		AutoDeploy:              parseYesNo(r.AutoDeploy),
		NotifyOnFail:            r.NotifyOnFail,
		PreDeployCommand:        preDeploy,
		PublishPath:             publishPath,
		Routes:                  routeViewsFromRender(r.Routes),
		Headers:                 headerViewsFromRender(r.Headers),
		DryRun:                  r.DryRun,
	}
}

// routeViewsFromRender / headerViewsFromRender convert the Render-shaped decode
// structs into the neutral surface views the service layer accepts.
func routeViewsFromRender(routes []renderRoute) []StaticRouteView {
	if len(routes) == 0 {
		return nil
	}
	out := make([]StaticRouteView, len(routes))
	for i, r := range routes {
		out[i] = StaticRouteView{Type: r.Type, Source: r.Source, Destination: r.Destination}
	}
	return out
}

func headerViewsFromRender(headers []renderHeader) []StaticHeaderView {
	if len(headers) == 0 {
		return nil
	}
	out := make([]StaticHeaderView, len(headers))
	for i, h := range headers {
		out[i] = StaticHeaderView{Path: h.Path, Name: h.Name, Value: h.Value}
	}
	return out
}

// parseYesNo maps Render's autoDeploy enum ("yes"/"no", or the bool-ish
// "true"/"false") to a tri-state *bool; "" => nil (use the platform default).
func parseYesNo(s string) *bool {
	switch s {
	case "yes", "true":
		t := true
		return &t
	case "no", "false":
		f := false
		return &f
	default:
		return nil
	}
}

// renderListParam expands Render's form-style array encoding. The generated
// CLI sends a multi-value flag as one comma-separated value, while hand-written
// clients commonly repeat the query key; accept both forms.
func renderListParam(values []string) []string {
	var out []string
	for _, raw := range values {
		for _, value := range strings.Split(raw, ",") {
			if value = strings.TrimSpace(value); value != "" {
				out = append(out, value)
			}
		}
	}
	return out
}

// RegisterREST mounts the App-lifecycle routes — Render-public-API compatible.
// Paths, the {service, cursor} list envelope, the string suspended enum, and the
// verb status codes (suspend/resume 202, restart 200) all match Render's OpenAPI
// spec. Served at both /v1/services (Render's noun) and /v1/apps (bex's noun);
// it holds no logic beyond routing + Render serialization.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	list := func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		apps, err := s.List(r.Context(), q.Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		// name filters by exact name, OR'd across repeated ?name= values (Render's
		// documented "Filter by name" — the official CLI resolves a bare
		// name/id argument to a service id by calling this with ?name=, and
		// requires it to narrow to exactly one match).
		if names := renderListParam(q["name"]); len(names) > 0 {
			filtered := make([]AppView, 0, len(apps))
			for _, a := range apps {
				if slices.Contains(names, a.Name) || slices.Contains(names, renderServiceName(a)) {
					filtered = append(filtered, a)
				}
			}
			apps = filtered
		}
		if environmentIDs := renderListParam(q["environmentId"]); len(environmentIDs) > 0 {
			filtered := make([]AppView, 0, len(apps))
			for _, a := range apps {
				if slices.Contains(environmentIDs, a.EnvironmentID) {
					filtered = append(filtered, a)
				}
			}
			apps = filtered
		}
		// Render's cursor pagination (docs/render-artifacts/owners-api.md): a
		// service's cursor is its name; `cursor`/`limit` page the result.
		after, limit := core.PageParams(q)
		page := core.Page(apps, after, limit, func(a AppView) string { return a.Name })
		core.WriteJSON(w, http.StatusOK, toServiceList(page)) // [{service, cursor}, ...]
	}
	get := func(w http.ResponseWriter, r *http.Request) {
		app, err := s.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderService(app))
	}
	listInstances := func(w http.ResponseWriter, r *http.Request) {
		instances, err := s.ListInstances(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, instances)
	}
	// verb maps a Service action to a handler with a Render-accurate status
	// code. ?confirm=<phrase> rides the context on every verb (withConfirm) —
	// a no-op for most of them, but it's what arms Suspend's protected-
	// environment guard (w6/m19, ProtectedConfirmation) without needing a
	// bespoke handler alongside Restart/Resume.
	verb := func(status int, fn func(context.Context, string) (AppView, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx := withConfirm(r.Context(), r.URL.Query().Get("confirm"))
			app, err := fn(ctx, r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, status, toRenderService(app))
		}
	}
	// patch handles PATCH /v1/services/{id} — a plan change (serviceDetails.plan),
	// an idle-timeout change (serviceDetails.idleTTLSeconds), and/or a root
	// directory change (rootDir); an unknown plan or a rootDir on an image-backed
	// App is core.ErrBadRequest => 400.
	// Pass `dryRun: true` in the body or `?dryRun=true` to preview the plan
	// change without writing; returns 200 with the resolved spec (w2/m29).
	patch := func(w http.ResponseWriter, r *http.Request) {
		var req patchServiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, fmt.Errorf("%w: %v", core.ErrBadRequest, err))
			return
		}
		if field := req.unsupportedField(); field != "" {
			core.WriteErr(w, fmt.Errorf("%w: services %s is not supported by this platform", core.ErrBadRequest, field))
			return
		}
		id := r.PathValue("id")
		dryRun := req.DryRun || r.URL.Query().Get("dryRun") == "true"
		var plan string
		var idleTTL *int32
		var maxShutdownDelay optionalInt32
		var healthCheckPath, preDeployCommand, schedule, publishPath, buildCommand, startCommand *string
		if req.ServiceDetails != nil {
			plan, idleTTL = req.ServiceDetails.Plan, req.ServiceDetails.IdleTTLSeconds
			maxShutdownDelay = req.ServiceDetails.MaxShutdownDelaySeconds
			healthCheckPath = req.ServiceDetails.HealthCheckPath
			preDeployCommand = req.ServiceDetails.PreDeployCommand
			schedule = req.ServiceDetails.Schedule
			publishPath = req.ServiceDetails.PublishPath
			buildCommand = req.ServiceDetails.BuildCommand
			if len(req.ServiceDetails.EnvSpecific) > 0 && string(req.ServiceDetails.EnvSpecific) != "null" {
				var envSpecific struct {
					BuildCommand  *string `json:"buildCommand"`
					StartCommand  *string `json:"startCommand"`
					DockerCommand *string `json:"dockerCommand"`
				}
				if err := json.Unmarshal(req.ServiceDetails.EnvSpecific, &envSpecific); err != nil {
					core.WriteErr(w, core.ErrBadRequest)
					return
				}
				if envSpecific.BuildCommand != nil {
					buildCommand = envSpecific.BuildCommand
				}
				startCommand = envSpecific.StartCommand
				if envSpecific.DockerCommand != nil {
					startCommand = envSpecific.DockerCommand
				}
			}
		}
		autoDeploy := parseYesNo(req.AutoDeploy) // nil => not provided (don't change)

		// Dry-run: preview plan change only; no writes at all (w2/m29).
		if dryRun {
			if plan == "" {
				get(w, r) // no plan => reflect current state unchanged
				return
			}
			app, err := s.PreviewSetPlan(r.Context(), id, plan)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, toRenderService(app))
			return
		}

		displayName := req.DisplayName
		if req.Name != nil {
			displayName = req.Name
		}
		if plan == "" && idleTTL == nil && !maxShutdownDelay.Set && displayName == nil && req.Repo == nil && req.Image == nil && req.Branch == nil && req.RootDir == nil && req.BuildFilter == nil && autoDeploy == nil && req.Schedule == nil && req.Command == nil && req.HealthCheckPath == nil && req.PreDeployCommand == nil && req.NotifyOnFail == nil && healthCheckPath == nil && preDeployCommand == nil && schedule == nil && publishPath == nil && buildCommand == nil && startCommand == nil {
			get(w, r) // no supported field present => read-only no-op
			return
		}
		// Apply the supported fields in turn; the no-op guard above guarantees at
		// least one runs, so app is always set by the time we serialize.
		var app AppView
		var err error
		if displayName != nil {
			if app, err = s.SetDisplayName(r.Context(), id, *displayName); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		var image *string
		if req.Image != nil {
			image = &req.Image.ImagePath
		}
		if req.Repo != nil || image != nil || req.Branch != nil {
			if app, err = s.SetSource(r.Context(), id, req.Repo, image, req.Branch); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if plan != "" {
			if app, err = s.SetPlan(r.Context(), id, plan); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if idleTTL != nil {
			if app, err = s.SetIdleTTL(r.Context(), id, *idleTTL); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if maxShutdownDelay.Set {
			if app, err = s.SetMaxShutdownDelay(r.Context(), id, maxShutdownDelay.Value); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if req.RootDir != nil {
			if app, err = s.SetRootDir(r.Context(), id, *req.RootDir); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if req.BuildFilter != nil {
			if app, err = s.SetBuildFilter(r.Context(), id, req.BuildFilter); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if autoDeploy != nil {
			if app, err = s.SetAutoDeploy(r.Context(), id, *autoDeploy); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if req.Schedule != nil || req.Command != nil {
			if app, err = s.SetCronJob(r.Context(), id, req.Schedule, req.Command); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if req.HealthCheckPath != nil {
			if app, err = s.SetHealthCheckPath(r.Context(), id, *req.HealthCheckPath); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if req.PreDeployCommand != nil {
			if app, err = s.SetPreDeployCommand(r.Context(), id, *req.PreDeployCommand); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if healthCheckPath != nil {
			if app, err = s.SetHealthCheckPath(r.Context(), id, *healthCheckPath); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if preDeployCommand != nil {
			if app, err = s.SetPreDeployCommand(r.Context(), id, *preDeployCommand); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if schedule != nil {
			if app, err = s.SetCronJob(r.Context(), id, schedule, nil); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if publishPath != nil {
			if app, err = s.SetPublishPath(r.Context(), id, *publishPath); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if buildCommand != nil || startCommand != nil {
			if app, err = s.SetCommands(r.Context(), id, buildCommand, startCommand); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if req.NotifyOnFail != nil {
			if app, err = s.SetNotifyOnFail(r.Context(), id, *req.NotifyOnFail); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		core.WriteJSON(w, http.StatusOK, toRenderService(app))
	}
	// scale handles POST /v1/services/{id}/scale — sets the running instance
	// count (numInstances); out-of-range is core.ErrBadRequest => 400.
	scale := func(w http.ResponseWriter, r *http.Request) {
		var req scaleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		app, err := s.Scale(r.Context(), r.PathValue("id"), req.NumInstances)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusAccepted, toRenderService(app)) // Render: scale => 202
	}

	// create handles POST /v1/services — create-or-update a service from a
	// Render-shaped body; deploy-from-chat rides this with a repo (no bespoke
	// deploy endpoint). Render returns 201 Created on success. `ownerId` names the
	// workspace to create in (w6/m14) — carried on CreateRequest, membership-checked
	// by the verb: a non-member gets 403, not a service in the wrong workspace.
	// Pass `dryRun: true` in the body or `?dryRun=true` in the query to preview
	// the resolved spec without any writes; response is 200 (not 201) (w2/m29).
	create := func(w http.ResponseWriter, r *http.Request) {
		var req createServiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, fmt.Errorf("%w: %v", core.ErrBadRequest, err))
			return
		}
		if field := req.unsupportedField(); field != "" {
			core.WriteErr(w, fmt.Errorf("%w: services %s is not supported by this platform", core.ErrBadRequest, field))
			return
		}
		if !req.DryRun && r.URL.Query().Get("dryRun") == "true" {
			req.DryRun = true
		}
		app, err := s.Create(r.Context(), req.toCreateRequest())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		if req.DryRun {
			core.WriteJSON(w, http.StatusOK, toRenderService(app)) // dry-run: 200 (nothing created)
			return
		}
		// Render: create => 201, body wraps the service under serviceAndDeploy.
		core.WriteJSON(w, http.StatusCreated, serviceAndDeploy{Service: toRenderService(app)})
	}

	// deleteSvc handles DELETE /v1/services/{id} — remove the service and let the
	// operator's ownerRefs cascade its derived resources. Render returns 204 No
	// Content with an empty body; unknown id => core.ErrNotFound => 404.
	// ?confirm=<phrase> arms the delete when the service is a member of a
	// protectedStatus=protected Environment (w6/m19, ProtectedConfirmation);
	// ignored (harmless) otherwise.
	deleteSvc := func(w http.ResponseWriter, r *http.Request) {
		ctx := withConfirm(r.Context(), r.URL.Query().Get("confirm"))
		if err := s.Delete(ctx, r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent) // Render: delete => 204, empty body
	}

	// runCron handles Render's current POST /cron-jobs/{id}/runs contract. The
	// deterministic pending run is returned immediately; if another run is active,
	// the same intent patch asks the operator to cancel it before replacement.
	runCron := func(w http.ResponseWriter, r *http.Request) {
		run, err := s.TriggerCronRun(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderCronJobRun(run))
	}
	listCronRuns := func(w http.ResponseWriter, r *http.Request) {
		cursor, limit := core.PageParams(r.URL.Query())
		runs, err := s.ListCronRuns(r.Context(), r.PathValue("id"), cursor, limit)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toCronJobRunList(runs))
	}
	getCronRun := func(w http.ResponseWriter, r *http.Request) {
		run, err := s.GetCronRun(r.Context(), r.PathValue("id"), r.PathValue("runId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderCronJobRun(run))
	}
	cancelCronRun := func(w http.ResponseWriter, r *http.Request) {
		run, err := s.CancelCronRun(r.Context(), r.PathValue("id"), r.PathValue("runId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderCronJobRun(run))
	}
	// Render's current cancel route addresses the active run implicitly and
	// returns 204. The per-run POST route above is a documented bex extension.
	cancelCurrentCronRun := func(w http.ResponseWriter, r *http.Request) {
		if _, err := s.CancelCurrentCronRun(r.Context(), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
	// Render's canonical noun. bex also accepts these under /services and /apps
	// (registered in the base loop below) for its historical noun aliases.
	mux.HandleFunc("GET /v1/cron-jobs/{id}/runs", listCronRuns)
	mux.HandleFunc("POST /v1/cron-jobs/{id}/runs", runCron)
	mux.HandleFunc("DELETE /v1/cron-jobs/{id}/runs", cancelCurrentCronRun)
	mux.HandleFunc("GET /v1/cron-jobs/{id}/runs/{runId}", getCronRun)
	mux.HandleFunc("POST /v1/cron-jobs/{id}/runs/{runId}/cancel", cancelCronRun)

	// Custom-domains sub-resource (Render-compatible).
	listDomains := func(w http.ResponseWriter, r *http.Request) {
		domains, err := s.ListDomains(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toCustomDomainList(domains))
	}
	addDomain := func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		d, err := s.AddDomain(r.Context(), r.PathValue("id"), req.Name)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, toRenderCustomDomain(d))
	}
	getDomain := func(w http.ResponseWriter, r *http.Request) {
		d, err := s.GetDomain(r.Context(), r.PathValue("id"), r.PathValue("name"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderCustomDomain(d))
	}
	deleteDomain := func(w http.ResponseWriter, r *http.Request) {
		if err := s.DeleteDomain(r.Context(), r.PathValue("id"), r.PathValue("name")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
	// verifyDomain re-checks DNS/cert state now (Render's POST …/verify) and returns
	// the fresh domain. 200 OK — bex verification is automatic, so this is a re-read.
	verifyDomain := func(w http.ResponseWriter, r *http.Request) {
		d, err := s.VerifyDomain(r.Context(), r.PathValue("id"), r.PathValue("name"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderCustomDomain(d))
	}

	// Autoscaling sub-resource (Render-compatible).
	// GET   …/autoscaling — current config (bex extension; Render has no GET)
	// PUT   …/autoscaling — upsert autoscaling (Render: PUT, 200)
	// DELETE …/autoscaling — disable autoscaling (Render: DELETE, 204)
	getAutoscaling := func(w http.ResponseWriter, r *http.Request) {
		av, err := s.GetAutoscaling(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, av)
	}
	putAutoscaling := func(w http.ResponseWriter, r *http.Request) {
		var req SetAutoscalingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		av, err := s.SetAutoscaling(r.Context(), r.PathValue("id"), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, av)
	}
	deleteAutoscaling := func(w http.ResponseWriter, r *http.Request) {
		if err := s.DeleteAutoscaling(r.Context(), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}

	// Static-site edge rules (Render-compatible): /routes (redirects/rewrites) and
	// /headers (custom response headers). GET lists; PUT replaces the whole list.
	listRoutes := func(w http.ResponseWriter, r *http.Request) {
		routes, err := s.ListRoutes(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderRoutes(routes))
	}
	putRoutes := func(w http.ResponseWriter, r *http.Request) {
		var body []renderRoute
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		app, err := s.SetRoutes(r.Context(), r.PathValue("id"), routeViewsFromRender(body))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderRoutes(app.Routes))
	}
	listHeaders := func(w http.ResponseWriter, r *http.Request) {
		headers, err := s.ListHeaders(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderHeaders(headers))
	}
	putHeaders := func(w http.ResponseWriter, r *http.Request) {
		var body []renderHeader
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		app, err := s.SetHeaders(r.Context(), r.PathValue("id"), headerViewsFromRender(body))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderHeaders(app.Headers))
	}

	// Blueprint routes (w2/m15): validate · list · sync.
	// POST /v1/blueprints/validate is registered before POST /v1/blueprints/{id}/sync
	// — Go 1.22+ ServeMux resolves the more specific (literal) path first.
	mux.HandleFunc("POST /v1/blueprints/validate", func(w http.ResponseWriter, r *http.Request) {
		ownerID, bexYAML, err := decodeBlueprintValidationRequest(w, r)
		if err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		v, err := s.ValidateBlueprint(r.Context(), ownerID, bexYAML)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("GET /v1/blueprints", func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.URL.Query().Get("ownerId")
		views, err := s.ListBlueprints(r.Context(), ownerID)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, views)
	})
	mux.HandleFunc("POST /v1/blueprints/{id}/sync", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			BexYAML string `json:"bexYaml"`
			OwnerID string `json:"ownerId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		res, err := s.SyncBlueprint(r.Context(), r.PathValue("id"), body.OwnerID, body.BexYAML)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, res)
	})

	// Register the same handlers under Render's /v1/services and bex's /v1/apps.
	for _, base := range []string{"/v1/services", "/v1/apps"} {
		mux.HandleFunc("GET "+base, list)
		mux.HandleFunc("POST "+base, create) // Render: create => 201
		mux.HandleFunc("GET "+base+"/{id}", get)
		// Render's official CLI uses /services. The /apps alias follows this
		// package's established rule that every service subresource is also
		// reachable under bex's native noun.
		mux.HandleFunc("GET "+base+"/{id}/instances", listInstances)
		mux.HandleFunc("PATCH "+base+"/{id}", patch)
		mux.HandleFunc("DELETE "+base+"/{id}", deleteSvc) // Render: delete => 204
		mux.HandleFunc("POST "+base+"/{id}/suspend", verb(http.StatusAccepted, s.Suspend))
		mux.HandleFunc("POST "+base+"/{id}/resume", verb(http.StatusAccepted, s.Resume))
		mux.HandleFunc("POST "+base+"/{id}/restart", verb(http.StatusOK, s.Restart)) // Render: restart => 200
		mux.HandleFunc("POST "+base+"/{id}/scale", scale)                            // Render: scale => 202
		mux.HandleFunc("GET "+base+"/{id}/runs", listCronRuns)
		mux.HandleFunc("POST "+base+"/{id}/runs", runCron)
		mux.HandleFunc("DELETE "+base+"/{id}/runs", cancelCurrentCronRun)
		mux.HandleFunc("GET "+base+"/{id}/runs/{runId}", getCronRun)
		mux.HandleFunc("POST "+base+"/{id}/runs/{runId}/cancel", cancelCronRun)
		mux.HandleFunc("GET "+base+"/{id}/autoscaling", getAutoscaling)
		mux.HandleFunc("PUT "+base+"/{id}/autoscaling", putAutoscaling)
		mux.HandleFunc("DELETE "+base+"/{id}/autoscaling", deleteAutoscaling)
		mux.HandleFunc("GET "+base+"/{id}/custom-domains", listDomains)
		mux.HandleFunc("POST "+base+"/{id}/custom-domains", addDomain)
		mux.HandleFunc("GET "+base+"/{id}/custom-domains/{name}", getDomain)
		mux.HandleFunc("DELETE "+base+"/{id}/custom-domains/{name}", deleteDomain)
		mux.HandleFunc("POST "+base+"/{id}/custom-domains/{name}/verify", verifyDomain)
		mux.HandleFunc("GET "+base+"/{id}/routes", listRoutes)
		mux.HandleFunc("PUT "+base+"/{id}/routes", putRoutes)
		mux.HandleFunc("GET "+base+"/{id}/headers", listHeaders)
		mux.HandleFunc("PUT "+base+"/{id}/headers", putHeaders)
	}
}

const maxBlueprintValidationBodyBytes = 2 << 20

// decodeBlueprintValidationRequest accepts Render's multipart contract used by
// the official CLI (ownerId field + file part), while retaining bex's original
// JSON contract for the dashboard and direct API callers.
func decodeBlueprintValidationRequest(w http.ResponseWriter, r *http.Request) (ownerID, bexYAML string, err error) {
	contentType := r.Header.Get("Content-Type")
	mediaType := "application/json"
	if contentType != "" {
		mediaType, _, err = mime.ParseMediaType(contentType)
		if err != nil {
			return "", "", err
		}
	}
	if mediaType != "multipart/form-data" {
		if mediaType != "application/json" {
			return "", "", fmt.Errorf("unsupported content type %q", mediaType)
		}
		var body struct {
			BexYAML string `json:"bexYaml"`
			OwnerID string `json:"ownerId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.BexYAML == "" {
			return "", "", core.ErrBadRequest
		}
		return body.OwnerID, body.BexYAML, nil
	}

	// The global request-body guard defaults to the same 2 MiB. Keep a local
	// bound too because feature-level tests and embedders can mount this router
	// without the composition root's middleware.
	r.Body = http.MaxBytesReader(w, r.Body, maxBlueprintValidationBodyBytes)
	if err := r.ParseMultipartForm(maxBlueprintValidationBodyBytes); err != nil {
		return "", "", err
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll() //nolint:errcheck // best-effort temp-file cleanup
	}
	ownerID = strings.TrimSpace(r.FormValue("ownerId"))
	if ownerID == "" {
		return "", "", fmt.Errorf("ownerId is required")
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		return "", "", err
	}
	defer file.Close() //nolint:errcheck // read-only multipart file
	content, err := io.ReadAll(file)
	if err != nil || len(content) == 0 {
		return "", "", core.ErrBadRequest
	}
	return ownerID, string(content), nil
}
