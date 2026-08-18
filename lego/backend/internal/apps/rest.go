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
	"strconv"
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
	// AutoDeployTrigger is Render's newer autoDeployTrigger enum for the same
	// toggle (off|commit|checksPass, w5/m53). It takes precedence over autoDeploy
	// when both are sent (Render's precedence); "checksPass" is rejected — bex has
	// no CI-gated deploy (documented divergence); "" => absent.
	AutoDeployTrigger string `json:"autoDeployTrigger"`
	// NotifyOnFail is Render's exact per-service notifyOnFail enum (default |
	// notify | ignore, docs/render-artifacts/notify-on-fail.md). A pointer so
	// "absent" (leave unchanged) is distinct from an explicit value; an
	// unrecognized value is core.ErrBadRequest.
	NotifyOnFail *string `json:"notifyOnFail"`
	// RenderSubdomainPolicy is Render's renderSubdomainPolicy field: "enabled"
	// or "disabled". A pointer so "absent" (leave unchanged) is distinct from
	// an explicit value. "disabled" without a custom domain is core.ErrBadRequest.
	RenderSubdomainPolicy *string `json:"renderSubdomainPolicy"`
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
	// AutoDeployTrigger is Render's off|commit|checksPass enum (w5/m53); takes
	// precedence over autoDeploy when both sent; "checksPass" rejected; "" => default.
	AutoDeployTrigger string `json:"autoDeployTrigger"`
	// NotifyOnFail is Render's exact per-service notifyOnFail enum (default |
	// notify | ignore); "" => default (docs/render-artifacts/notify-on-fail.md).
	NotifyOnFail   string             `json:"notifyOnFail"`
	EnvVars        []envVarInput      `json:"envVars"`
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
	Routes  []StaticRouteView  `json:"routes"`
	Headers []StaticHeaderView `json:"headers"`
	// RenderSubdomainPolicy is Render's renderSubdomainPolicy field: "enabled"
	// (default) or "disabled". Cannot be "disabled" without at least one custom
	// domain in Domains.
	RenderSubdomainPolicy string `json:"renderSubdomainPolicy"`
	// DryRun, when true, resolves the spec and returns a preview without any
	// Kubernetes or store writes — zero side effects (w2/m29). Response status
	// is 200 (not 201 Created) to signal that no resource was actually created.
	// Can also be set via the ?dryRun=true query parameter.
	DryRun bool `json:"dryRun,omitempty"`
}

// imageRef is Render's prebuilt-image object: the image path lives under
// `image.imagePath`, not a bare top-level string. The official CLI always
// serializes ownerId because its generated Image.OwnerId field is required;
// create/update currently send the empty string and rely on the top-level or
// existing service owner. registryCredentialId is a RawMessage so omission
// remains distinct from an explicit empty-string clear.
type imageRef struct {
	ImagePath            string          `json:"imagePath"`
	OwnerID              string          `json:"ownerId"`
	RegistryCredentialID json.RawMessage `json:"registryCredentialId"`
}

// serviceDetailsReq is where Render nests plan, numInstances and healthCheckPath
// on create — the same location PATCH and GET report them. schedule is Render's
// cronJobDetails.schedule (accepted here or at the top level, top level wins).
type serviceDetailsReq struct {
	Plan string `json:"plan"`
	// Region is emitted by the official CLI on service creation. bex is a
	// single-region platform, so the pinned Render schema validates the value
	// and the adapter intentionally normalizes it to the configured placement.
	Region                  string        `json:"region"`
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

// decodeMaintenanceMode preserves Render's required-key semantics: when the
// object is supplied, both enabled and uri must be present (including explicit
// false and the empty string).
func decodeMaintenanceMode(ctx context.Context, raw json.RawMessage) (*MaintenanceModeView, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var wire struct {
		Enabled *bool   `json:"enabled"`
		URI     *string `json:"uri"`
	}
	if string(raw) == "null" || core.UnmarshalJSON(ctx, raw, &wire) != nil || wire.Enabled == nil || wire.URI == nil {
		return nil, fmt.Errorf("%w: serviceDetails.maintenanceMode requires enabled and uri", core.ErrBadRequest)
	}
	return &MaintenanceModeView{Enabled: *wire.Enabled, URI: *wire.URI}, nil
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
	if r.ServiceDetails == nil {
		return ""
	}
	return unsupportedPreviewsField(r.ServiceDetails.Previews)
}

func (r patchServiceRequest) unsupportedField() string {
	if r.ServiceDetails == nil {
		return ""
	}
	return unsupportedPreviewsField(r.ServiceDetails.Previews)
}

// unsupportedPreviewsField names the Render serviceDetails field bex cannot
// honor, so create and patch refuse it identically rather than one silently
// dropping it. The two requests spell serviceDetails with different Go types,
// so the shared part is the field, not the struct.
func unsupportedPreviewsField(previews json.RawMessage) string {
	if len(previews) > 0 {
		return "previews"
	}
	return ""
}

// decodeRegistryCredentialID preserves the PATCH tri-state: omitted => nil,
// string => set/change, empty string => clear. Render's OpenAPI declares a
// string (not nullable), so null and non-string values are named bad requests.
func decodeRegistryCredentialID(raw json.RawMessage) (*string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if string(raw) == "null" {
		return nil, fmt.Errorf("%w: registryCredentialId must be a string", core.ErrBadRequest)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("%w: registryCredentialId must be a string", core.ErrBadRequest)
	}
	value = strings.TrimSpace(value)
	return &value, nil
}

func oneRegistryCredentialID(first, second json.RawMessage) (*string, error) {
	a, err := decodeRegistryCredentialID(first)
	if err != nil {
		return nil, err
	}
	b, err := decodeRegistryCredentialID(second)
	if err != nil {
		return nil, err
	}
	if a != nil && b != nil && *a != *b {
		return nil, fmt.Errorf("%w: registryCredentialId is set to conflicting values", core.ErrBadRequest)
	}
	if a != nil {
		return a, nil
	}
	return b, nil
}

// effectiveImageOwnerID reconciles the two owner spellings in Render's create
// payload. The generated CLI sends image.ownerId:"" while setting the actual
// workspace in the top-level ownerId field, so blank nested ownership inherits
// the top-level/default value. A non-blank nested value is validation only: it
// must match that effective owner and never becomes a second workspace selector.
func effectiveImageOwnerID(ownerID, defaultOwnerID string, image *imageRef) (string, error) {
	ownerID = strings.TrimSpace(ownerID)
	effectiveOwnerID := ownerID
	if effectiveOwnerID == "" {
		effectiveOwnerID = strings.TrimSpace(defaultOwnerID)
	}
	if image == nil {
		return effectiveOwnerID, nil
	}
	imageOwnerID := strings.TrimSpace(image.OwnerID)
	if imageOwnerID == "" {
		return effectiveOwnerID, nil
	}
	if effectiveOwnerID == "" || effectiveOwnerID != imageOwnerID {
		return "", fmt.Errorf("%w: image.ownerId conflicts with ownerId", core.ErrBadRequest)
	}
	return effectiveOwnerID, nil
}

// ipAllowEntry is Render's components.schemas.cidrBlockAndDescription. Alias
// the shared Core entry so services, Postgres, KeyValue, GraphQL, and MCP use
// one description-preserving shape.
type ipAllowEntry = core.IPAllowListEntry

// cloneIPAllowEntries copies allowlist entries, preserving nil for empty so
// omitempty stays wire-accurate.
func cloneIPAllowEntries(entries []core.IPAllowListEntry) []core.IPAllowListEntry {
	if len(entries) == 0 {
		return nil
	}
	return slices.Clone(entries)
}

// decodeIPAllowList decodes a serviceDetails.ipAllowList RawMessage; provided
// reports whether the field carried a value (absent/null leaves it unchanged).
func decodeIPAllowList(ctx context.Context, raw json.RawMessage) (entries []core.IPAllowListEntry, provided bool, err error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false, nil
	}
	var in []ipAllowEntry
	if err := core.UnmarshalJSON(ctx, raw, &in); err != nil {
		return nil, false, fmt.Errorf("%w: ipAllowList: %v", core.ErrBadRequest, err)
	}
	return cloneIPAllowEntries(in), true, nil
}

// preferTopLevel resolves a field sent both at the top level and nested under
// serviceDetails: the top-level value wins, the nested one is the fallback.
func preferTopLevel(top, nested string) string {
	if top != "" {
		return top
	}
	return nested
}

// toCreateRequest folds the Render-nested and bex top-level fields into the
// neutral CreateRequest. serviceDetails is Render's canonical location for
// plan/numInstances/healthCheckPath; the top-level plan is a bex convenience
// fallback. type:private_service maps to the in-cluster-only flag.
func (r createServiceRequest) toCreateRequest(ctx context.Context, defaultOwnerID string) (CreateRequest, error) {
	plan, health, schedule, command, publishPath := r.Plan, "", r.Schedule, r.Command, r.PublishPath
	rootDir := r.RootDir
	var runtime, buildCommand, startCommand, dockerfilePath, dockerContext string
	var nestedRegistryCredentialID json.RawMessage
	preDeploy := r.PreDeployCommand
	var replicas int32
	var maxShutdownDelaySeconds *int32
	var maintenanceMode *MaintenanceModeView
	if r.ServiceDetails != nil {
		var err error
		if maintenanceMode, err = decodeMaintenanceMode(ctx, r.ServiceDetails.MaintenanceMode); err != nil {
			return CreateRequest{}, err
		}
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
			nestedRegistryCredentialID = r.ServiceDetails.EnvSpecificDetails.RegistryCredentialID
			if strings.EqualFold(runtime, "docker") {
				startCommand = r.ServiceDetails.EnvSpecificDetails.DockerCommand
				dockerfilePath = r.ServiceDetails.EnvSpecificDetails.DockerfilePath
				// dockerContext is its own spec field (repo-root-relative,
				// independent of rootDir) — the pre-w8/m19 rootDir fold was a
				// lossy approximation.
				dockerContext = r.ServiceDetails.EnvSpecificDetails.DockerContext
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
		schedule = preferTopLevel(schedule, r.ServiceDetails.Schedule)
		command = preferTopLevel(command, r.ServiceDetails.Command)
		if command == "" && r.Type == appv1alpha1.TypeCronJob {
			command = startCommand // official CLI encodes cron command in envSpecificDetails.startCommand
		}
		publishPath = preferTopLevel(publishPath, r.ServiceDetails.PublishPath)
		preDeploy = preferTopLevel(preDeploy, r.ServiceDetails.PreDeployCommand)
	}
	image := ""
	var imageRegistryCredentialID json.RawMessage
	if r.Image != nil {
		image = r.Image.ImagePath
		imageRegistryCredentialID = r.Image.RegistryCredentialID
	}
	registryCredentialID, err := oneRegistryCredentialID(imageRegistryCredentialID, nestedRegistryCredentialID)
	if err != nil {
		return CreateRequest{}, err
	}
	ownerID, err := effectiveImageOwnerID(r.OwnerID, defaultOwnerID, r.Image)
	if err != nil {
		return CreateRequest{}, err
	}
	env := toEnvVars(r.EnvVars)
	secretFiles := toSecretFiles(r.SecretFiles)
	var ipAllowList []core.IPAllowListEntry
	if r.ServiceDetails != nil {
		var err error
		if ipAllowList, _, err = decodeIPAllowList(ctx, r.ServiceDetails.IPAllowList); err != nil {
			return CreateRequest{}, err
		}
	}
	// Auto-Deploy: autoDeployTrigger (off|commit) wins over the legacy autoDeploy
	// enum when both are sent; checksPass is rejected (w5/m53).
	autoDeploy, err := parseAutoDeploy(r.AutoDeploy, r.AutoDeployTrigger)
	if err != nil {
		return CreateRequest{}, err
	}
	return CreateRequest{
		OwnerID:                 ownerID,
		EnvironmentID:           r.EnvironmentID,
		Name:                    r.Name,
		Type:                    r.Type,
		Schedule:                schedule,
		Command:                 command,
		Repo:                    r.Repo,
		Image:                   image,
		RegistryCredentialID:    registryCredentialID,
		Branch:                  r.Branch,
		Builder:                 r.Builder,
		Runtime:                 runtime,
		BuildCommand:            buildCommand,
		StartCommand:            startCommand,
		RootDir:                 rootDir,
		BuildFilter:             r.BuildFilter,
		MaintenanceMode:         maintenanceMode,
		DockerfilePath:          dockerfilePath,
		DockerContext:           dockerContext,
		Port:                    r.Port,
		Replicas:                replicas,
		Plan:                    plan,
		HealthCheckPath:         health,
		MaxShutdownDelaySeconds: maxShutdownDelaySeconds,
		Env:                     env,
		SecretFiles:             secretFiles,
		Hosts:                   r.Domains,
		AutoDeploy:              autoDeploy,
		NotifyOnFail:            r.NotifyOnFail,
		SubdomainPolicy:         r.RenderSubdomainPolicy,
		PreDeployCommand:        preDeploy,
		PublishPath:             publishPath,
		Routes:                  r.Routes,
		Headers:                 r.Headers,
		IPAllowList:             ipAllowList,
		DryRun:                  r.DryRun,
	}, nil
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

// parseTrigger maps Render's autoDeployTrigger enum ("off"|"commit") to a
// tri-state *bool. "checksPass" is rejected — bex deploys on every matching push
// or not at all, with no CI-checks-gated trigger (a documented divergence,
// docs/ADR018-render-parity.md); any other non-empty value is a bad request too.
// "" => nil (not provided).
func parseTrigger(s string) (*bool, error) {
	switch s {
	case "":
		return nil, nil
	case "commit":
		t := true
		return &t, nil
	case "off":
		f := false
		return &f, nil
	case "checksPass":
		return nil, fmt.Errorf("%w: autoDeployTrigger %q is not supported — bex deploys on every matching push (%q) or not at all (%q); it has no CI-checks-gated deploy trigger", core.ErrBadRequest, s, "commit", "off")
	default:
		return nil, fmt.Errorf("%w: autoDeployTrigger must be %q or %q", core.ErrBadRequest, "off", "commit")
	}
}

// parseAutoDeploy resolves Render's two Auto-Deploy fields into one tri-state
// *bool: autoDeployTrigger takes precedence over the legacy autoDeploy enum when
// both are present (Render's documented precedence), validating the trigger.
func parseAutoDeploy(autoDeploy, trigger string) (*bool, error) {
	t, err := parseTrigger(trigger)
	if err != nil {
		return nil, err
	}
	if t != nil {
		return t, nil
	}
	return parseYesNo(autoDeploy), nil
}

// servicesBase is Render's canonical /v1/services route prefix, shared by the
// registrars that mount /v1/services/{id}/… subresources.
const servicesBase = "/v1/services"

// RegisterREST mounts the App-lifecycle routes — Render-public-API compatible.
// Paths, the {service, cursor} list envelope, the string suspended enum, and the
// verb status codes (suspend/resume 202, restart 200) all match Render's OpenAPI
// spec. Served at Render's canonical /v1/services route;
// it holds no logic beyond routing + Render serialization. Each registrar
// mounts one route family into the same shared mux — the internal/api layer
// requires one root — in the same family order the routes were registered in
// before the split.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	s.registerCronRunRoutes(mux)
	s.registerBlueprintRoutes(mux)
	s.registerNotificationOverrideRoutes(mux)
	s.registerServiceRoutes(mux)
	s.registerDomainRoutes(mux)
}

// registerServiceRoutes mounts the /v1/services core: list · create · get ·
// patch · delete · the lifecycle verbs (suspend/resume/restart) · scale ·
// instances · shell tickets.
func (s *Service) registerServiceRoutes(mux *http.ServeMux) {
	list := core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		q := r.URL.Query()
		apps, err := s.List(r.Context(), q.Get("ownerId"))
		if err != nil {
			return nil, err
		}
		// name filters by exact name, OR'd across repeated ?name= values (Render's
		// documented "Filter by name" — the official CLI resolves a bare
		// name/id argument to a service id by calling this with ?name=, and
		// requires it to narrow to exactly one match). environmentId and Render's
		// serviceType enum (type=, w2/m52) OR the same way.
		names := core.QueryList(q, "name")
		environmentIDs := core.QueryList(q, "environmentId")
		types := core.QueryList(q, "type")
		// suspended= is Render's boolean string filter ("true"/"false"). Unknown
		// values return a named 400; absent means unfiltered.
		suspended, err := core.ParseEnum("suspended", q.Get("suspended"), "true", "false")
		if err != nil {
			return nil, err
		}
		// Time-window filters (w2/m52): Render's createdBefore/createdAfter and
		// updatedBefore/updatedAfter RFC3339 params.
		created, err := core.QueryTimeWindow(q, "createdBefore", "createdAfter")
		if err != nil {
			return nil, err
		}
		updated, err := core.QueryTimeWindow(q, "updatedBefore", "updatedAfter")
		if err != nil {
			return nil, err
		}
		apps = core.Filter(apps, func(a AppView) bool {
			return (len(names) == 0 || slices.Contains(names, a.Name) || slices.Contains(names, renderServiceName(a))) &&
				(len(environmentIDs) == 0 || slices.Contains(environmentIDs, a.EnvironmentID)) &&
				(len(types) == 0 || slices.Contains(types, effectiveType(a.Type))) &&
				(suspended == "" || a.Suspended == (suspended == "true")) &&
				created.Contains(a.CreatedAt) && updated.Contains(a.UpdatedAt)
		})
		// Render's cursor pagination (docs/render-artifacts/owners-api.md): a
		// service's cursor is its name; `cursor`/`limit` page the result.
		after, limit := core.PageParams(q)
		page := core.Page(apps, after, limit, func(a AppView) string { return a.Name })
		return s.restServiceList(r.Context(), page), nil // [{service, cursor}, ...]
	})
	listInstances := core.HandleByID(s.ListInstances)
	// shellTicket mints a Browser Web Shell exec ticket (docs/ADR035-ssh.md
	// § Browser Web Shell). bex extension over Render's REST — the dashboard
	// terminal opens the gateway WebSocket with it. Optional ?instance=<id>
	// pins one Ready replica; omitted selects a random one, matching SSH.
	shellTicket := core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		return s.CreateShellSession(r.Context(), r.PathValue("id"), r.URL.Query().Get("instance"))
	})
	// scale handles POST /v1/services/{id}/scale — sets the running instance
	// count (numInstances); out-of-range is core.ErrBadRequest => 400.
	scale := core.HandleJSON(http.StatusAccepted, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[scaleRequest](r)
		if err != nil {
			return nil, err
		}
		app, err := s.Scale(r.Context(), r.PathValue("id"), req.NumInstances)
		if err != nil {
			return nil, err
		}
		return s.restService(r.Context(), app), nil
	})

	// deleteSvc handles DELETE /v1/services/{id} — remove the service and let the
	// operator's ownerRefs cascade its derived resources. Render returns 204 No
	// Content with an empty body; unknown id => core.ErrNotFound => 404.
	// ?confirm=<phrase> arms the delete when the service is a member of a
	// protectedStatus=protected Environment (w6/m19, ProtectedConfirmation);
	// ignored (harmless) otherwise.
	deleteSvc := core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		ctx := core.WithConfirm(r.Context(), r.URL.Query().Get("confirm"))
		return s.Delete(ctx, r.PathValue("id"))
	})

	mux.HandleFunc("GET "+servicesBase, list)
	mux.HandleFunc("POST "+servicesBase, s.createService) // Render: create => 201
	mux.HandleFunc("GET "+servicesBase+"/{id}", s.getService)
	// Render's official CLI uses /services.
	mux.HandleFunc("GET "+servicesBase+"/{id}/instances", listInstances)
	mux.HandleFunc("POST "+servicesBase+"/{id}/shell-ticket", shellTicket)
	mux.HandleFunc("PATCH "+servicesBase+"/{id}", s.patchService)
	mux.HandleFunc("DELETE "+servicesBase+"/{id}", deleteSvc) // Render: delete => 204
	mux.HandleFunc("POST "+servicesBase+"/{id}/suspend", s.restVerb(http.StatusAccepted, s.Suspend))
	mux.HandleFunc("POST "+servicesBase+"/{id}/resume", s.restVerb(http.StatusAccepted, s.Resume))
	mux.HandleFunc("POST "+servicesBase+"/{id}/restart", s.restVerb(http.StatusOK, s.Restart)) // Render: restart => 200
	mux.HandleFunc("POST "+servicesBase+"/{id}/scale", scale)                                  // Render: scale => 202
}

// restVerb maps a Service action to a handler with a Render-accurate status
// code. ?confirm=<phrase> rides the context on every verb (core.WithConfirm) —
// a no-op for most of them, but it's what arms Suspend's protected-
// environment guard (w6/m19, ProtectedConfirmation) without needing a
// bespoke handler alongside Restart/Resume.
func (s *Service) restVerb(status int, fn func(context.Context, string) (AppView, error)) http.HandlerFunc {
	return core.HandleJSON(status, func(r *http.Request) (any, error) {
		ctx := core.WithConfirm(r.Context(), r.URL.Query().Get("confirm"))
		app, err := fn(ctx, r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		return s.restService(r.Context(), app), nil
	})
}

// getService serves GET /v1/services/{id}. The PATCH handler also answers with
// it for a dry run carrying no plan and for a body with no supported field —
// both read-only no-ops that reflect current state unchanged.
func (s *Service) getService(w http.ResponseWriter, r *http.Request) {
	s.restVerb(http.StatusOK, s.Get)(w, r)
}

// patchFields is the PATCH /v1/services/{id} body coalesced to one value per
// setting by resolveFields, so patchService's ops table reads a single source
// of truth per field.
type patchFields struct {
	dryRun                                                  bool
	plan                                                    string
	idleTTL                                                 *int32
	maxShutdownDelay                                        optionalInt32
	publishPath, buildCommand, startCommand, dockerfilePath *string
	healthCheckPath, preDeployCommand, schedule             *string
	displayName                                             *string
	image, imageOwnerID                                     *string
	registryCredentialID                                    *string
	ipAllowList                                             *[]core.IPAllowListEntry // nil = not provided (leave unchanged); non-nil = replace
	autoDeploy                                              *bool
}

// resolveFields decodes and coalesces the wire fields into patchFields.
// Fields with both a top-level and a nested serviceDetails spelling
// coalesce to one apply; the nested spelling wins when a body carries
// both.
func (req patchServiceRequest) resolveFields(r *http.Request) (patchFields, error) {
	f := patchFields{
		dryRun:           core.DryRunRequested(r, req.DryRun),
		healthCheckPath:  req.HealthCheckPath,
		preDeployCommand: req.PreDeployCommand,
		schedule:         req.Schedule,
		displayName:      req.DisplayName,
	}
	var nestedRegistryCredentialID json.RawMessage
	if req.ServiceDetails != nil {
		f.plan, f.idleTTL = req.ServiceDetails.Plan, req.ServiceDetails.IdleTTLSeconds
		f.maxShutdownDelay = req.ServiceDetails.MaxShutdownDelaySeconds
		if req.ServiceDetails.HealthCheckPath != nil {
			f.healthCheckPath = req.ServiceDetails.HealthCheckPath
		}
		if req.ServiceDetails.PreDeployCommand != nil {
			f.preDeployCommand = req.ServiceDetails.PreDeployCommand
		}
		if req.ServiceDetails.Schedule != nil {
			f.schedule = req.ServiceDetails.Schedule
		}
		f.publishPath = req.ServiceDetails.PublishPath
		f.buildCommand = req.ServiceDetails.BuildCommand
		allowList, provided, err := decodeIPAllowList(r.Context(), req.ServiceDetails.IPAllowList)
		if err != nil {
			return patchFields{}, err
		}
		if provided {
			f.ipAllowList = &allowList
		}
		if len(req.ServiceDetails.EnvSpecific) > 0 && string(req.ServiceDetails.EnvSpecific) != "null" {
			var envSpecific struct {
				BuildCommand         *string         `json:"buildCommand"`
				StartCommand         *string         `json:"startCommand"`
				DockerCommand        *string         `json:"dockerCommand"`
				DockerfilePath       *string         `json:"dockerfilePath"`
				RegistryCredentialID json.RawMessage `json:"registryCredentialId"`
			}
			if err := core.UnmarshalJSON(r.Context(), req.ServiceDetails.EnvSpecific, &envSpecific); err != nil {
				return patchFields{}, core.ErrBadRequest
			}
			if envSpecific.BuildCommand != nil {
				f.buildCommand = envSpecific.BuildCommand
			}
			f.startCommand = envSpecific.StartCommand
			if envSpecific.DockerCommand != nil {
				f.startCommand = envSpecific.DockerCommand
			}
			f.dockerfilePath = envSpecific.DockerfilePath
			nestedRegistryCredentialID = envSpecific.RegistryCredentialID
		}
	}
	var imageRegistryCredentialID json.RawMessage
	if req.Image != nil {
		imageRegistryCredentialID = req.Image.RegistryCredentialID
		f.image = &req.Image.ImagePath
		f.imageOwnerID = &req.Image.OwnerID
	}
	registryCredentialID, registryErr := oneRegistryCredentialID(imageRegistryCredentialID, nestedRegistryCredentialID)
	if registryErr != nil {
		return patchFields{}, registryErr
	}
	f.registryCredentialID = registryCredentialID
	// Auto-Deploy: autoDeployTrigger (off|commit) wins over the legacy
	// autoDeploy enum when both are sent; checksPass is rejected (w5/m53).
	autoDeploy, autoDeployErr := parseAutoDeploy(req.AutoDeploy, req.AutoDeployTrigger)
	if autoDeployErr != nil {
		return patchFields{}, autoDeployErr
	}
	f.autoDeploy = autoDeploy
	if req.Name != nil {
		f.displayName = req.Name
	}
	return f, nil
}

// patchService handles PATCH /v1/services/{id} — a plan change (serviceDetails.plan),
// an idle-timeout change (serviceDetails.idleTTLSeconds), and/or a root
// directory change (rootDir); an unknown plan or a rootDir on an image-backed
// App is core.ErrBadRequest => 400.
// Pass `dryRun: true` in the body or `?dryRun=true` to preview the plan
// change without writing; returns 200 with the resolved spec (w2/m29).
func (s *Service) patchService(w http.ResponseWriter, r *http.Request) {
	var req patchServiceRequest
	if err := core.DecodeJSON(r, &req); err != nil {
		core.WriteErr(w, fmt.Errorf("%w: %v", core.ErrBadRequest, err))
		return
	}
	if field := req.unsupportedField(); field != "" {
		core.WriteErr(w, fmt.Errorf("%w: services %s is not supported by this platform", core.ErrBadRequest, field))
		return
	}
	id := r.PathValue("id")
	f, err := req.resolveFields(r)
	if err != nil {
		core.WriteErr(w, err)
		return
	}

	// Dry-run: preview plan change only; no writes at all (w2/m29).
	if f.dryRun {
		if f.plan == "" {
			s.getService(w, r) // no plan => reflect current state unchanged
			return
		}
		app, err := s.PreviewSetPlan(r.Context(), id, f.plan)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, s.restService(r.Context(), app))
		return
	}

	var maintenanceMode *MaintenanceModeView
	if req.ServiceDetails != nil {
		var maintenanceErr error
		maintenanceMode, maintenanceErr = decodeMaintenanceMode(r.Context(), req.ServiceDetails.MaintenanceMode)
		if maintenanceErr != nil {
			core.WriteErr(w, maintenanceErr)
			return
		}
	}
	// Collect the supported fields as an ordered list of verb calls; an empty
	// list is the read-only no-op (the guard is derived from the same list, so
	// a new field can't fall out of sync with it).
	var ops []func() (AppView, error)
	addOp := func(present bool, op func() (AppView, error)) {
		if present {
			ops = append(ops, op)
		}
	}
	addOp(f.displayName != nil, func() (AppView, error) {
		return s.SetDisplayName(r.Context(), id, *f.displayName)
	})
	addOp(req.Repo != nil || f.image != nil || req.Branch != nil || f.registryCredentialID != nil, func() (AppView, error) {
		return s.SetSourceAndRegistryCredential(r.Context(), id, sourcePatch{Repo: req.Repo, Image: f.image, Branch: req.Branch, RegistryCredentialID: f.registryCredentialID, ImageOwnerID: f.imageOwnerID})
	})
	// A simultaneous downgrade must disable maintenance first; every other
	// combination applies the plan first so validation sees the final plan.
	maintenanceBeforePlan := maintenanceMode != nil && !maintenanceMode.Enabled && f.plan == "free"
	addOp(maintenanceBeforePlan, func() (AppView, error) {
		return s.ConfigureMaintenanceMode(r.Context(), id, *maintenanceMode)
	})
	addOp(f.plan != "", func() (AppView, error) {
		return s.SetPlan(r.Context(), id, f.plan)
	})
	addOp(f.idleTTL != nil, func() (AppView, error) {
		return s.SetIdleTTL(r.Context(), id, *f.idleTTL)
	})
	addOp(f.maxShutdownDelay.Set, func() (AppView, error) {
		return s.SetMaxShutdownDelay(r.Context(), id, f.maxShutdownDelay.Value)
	})
	addOp(req.RootDir != nil, func() (AppView, error) {
		return s.SetRootDir(r.Context(), id, *req.RootDir)
	})
	addOp(req.BuildFilter != nil, func() (AppView, error) {
		return s.SetBuildFilter(r.Context(), id, req.BuildFilter)
	})
	addOp(f.autoDeploy != nil, func() (AppView, error) {
		return s.SetAutoDeploy(r.Context(), id, *f.autoDeploy)
	})
	addOp(f.schedule != nil || req.Command != nil, func() (AppView, error) {
		return s.SetCronJob(r.Context(), id, f.schedule, req.Command)
	})
	addOp(f.healthCheckPath != nil, func() (AppView, error) {
		return s.SetHealthCheckPath(r.Context(), id, *f.healthCheckPath)
	})
	addOp(f.preDeployCommand != nil, func() (AppView, error) {
		return s.SetPreDeployCommand(r.Context(), id, *f.preDeployCommand)
	})
	addOp(f.publishPath != nil, func() (AppView, error) {
		return s.SetPublishPath(r.Context(), id, *f.publishPath)
	})
	addOp(f.buildCommand != nil || f.startCommand != nil, func() (AppView, error) {
		return s.SetCommands(r.Context(), id, f.buildCommand, f.startCommand)
	})
	addOp(f.dockerfilePath != nil, func() (AppView, error) {
		return s.SetDockerfilePath(r.Context(), id, *f.dockerfilePath)
	})
	addOp(req.NotifyOnFail != nil, func() (AppView, error) {
		return s.SetNotifyOnFail(r.Context(), id, *req.NotifyOnFail)
	})
	addOp(req.RenderSubdomainPolicy != nil, func() (AppView, error) {
		return s.SetSubdomainPolicy(r.Context(), id, *req.RenderSubdomainPolicy)
	})
	addOp(f.ipAllowList != nil, func() (AppView, error) {
		return s.SetIPAllowList(r.Context(), id, *f.ipAllowList)
	})
	addOp(maintenanceMode != nil && !maintenanceBeforePlan, func() (AppView, error) {
		return s.ConfigureMaintenanceMode(r.Context(), id, *maintenanceMode)
	})
	if len(ops) == 0 {
		s.getService(w, r) // no supported field present => read-only no-op
		return
	}
	var app AppView
	for _, op := range ops {
		var err error
		if app, err = op(); err != nil {
			core.WriteErr(w, err)
			return
		}
	}
	core.WriteJSON(w, http.StatusOK, s.restService(r.Context(), app))
}

// createService handles POST /v1/services — create-or-update a service from a
// Render-shaped body; deploy-from-chat rides this with a repo (no bespoke
// deploy endpoint). Render returns 201 Created on success. `ownerId` names the
// workspace to create in (w6/m14) — carried on CreateRequest, membership-checked
// by the verb: a non-member gets 403, not a service in the wrong workspace.
// Pass `dryRun: true` in the body or `?dryRun=true` in the query to preview
// the resolved spec without any writes; response is 200 (not 201) (w2/m29).
func (s *Service) createService(w http.ResponseWriter, r *http.Request) {
	var req createServiceRequest
	if err := core.DecodeJSON(r, &req); err != nil {
		core.WriteErr(w, fmt.Errorf("%w: %v", core.ErrBadRequest, err))
		return
	}
	if field := req.unsupportedField(); field != "" {
		core.WriteErr(w, fmt.Errorf("%w: services %s is not supported by this platform", core.ErrBadRequest, field))
		return
	}
	req.DryRun = core.DryRunRequested(r, req.DryRun)
	defaultOwnerID, _ := s.Tenant(r.Context())
	createReq, err := req.toCreateRequest(r.Context(), defaultOwnerID)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	app, err := s.Create(r.Context(), createReq)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	if req.DryRun {
		core.WriteJSON(w, http.StatusOK, s.restService(r.Context(), app)) // dry-run: 200 (nothing created)
		return
	}
	// Render: create => 201, body wraps the service under serviceAndDeploy.
	core.WriteJSON(w, http.StatusCreated, serviceAndDeploy{Service: s.restService(r.Context(), app), DeployID: app.LatestDeployID})
}

// registerCronRunRoutes mounts the cron-job run routes, under both Render's
// canonical /v1/cron-jobs noun and the /v1/services/{id}/runs subresource form.
func (s *Service) registerCronRunRoutes(mux *http.ServeMux) {
	// runCron handles Render's current POST /cron-jobs/{id}/runs contract. The
	// deterministic pending run is returned immediately; if another run is active,
	// the same intent patch asks the operator to cancel it before replacement.
	runCron := core.HandleMapped(http.StatusOK, func(r *http.Request) (CronRunView, error) {
		return s.TriggerCronRun(r.Context(), r.PathValue("id"))
	}, toRenderCronJobRun)
	listCronRuns := core.HandleMapped(http.StatusOK, func(r *http.Request) ([]CronRunView, error) {
		cursor, limit := core.PageParams(r.URL.Query())
		return s.ListCronRuns(r.Context(), r.PathValue("id"), cursor, limit)
	}, toCronJobRunList)
	getCronRun := core.HandleMapped(http.StatusOK, func(r *http.Request) (CronRunView, error) {
		return s.GetCronRun(r.Context(), r.PathValue("id"), r.PathValue("runId"))
	}, toRenderCronJobRun)
	cancelCronRun := core.HandleMapped(http.StatusOK, func(r *http.Request) (CronRunView, error) {
		return s.CancelCronRun(r.Context(), r.PathValue("id"), r.PathValue("runId"))
	}, toRenderCronJobRun)
	// Render's current cancel route addresses the active run implicitly and
	// returns 204. The per-run POST route above is a documented bex extension.
	cancelCurrentCronRun := core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		_, err := s.CancelCurrentCronRun(r.Context(), r.PathValue("id"))
		return err
	})
	// Render's canonical cron-job noun. The service subresource form is also
	// registered below; the retired public /v1/apps family is not.
	mux.HandleFunc("GET /v1/cron-jobs/{id}/runs", listCronRuns)
	mux.HandleFunc("POST /v1/cron-jobs/{id}/runs", runCron)
	mux.HandleFunc("DELETE /v1/cron-jobs/{id}/runs", cancelCurrentCronRun)
	mux.HandleFunc("GET /v1/cron-jobs/{id}/runs/{runId}", getCronRun)
	mux.HandleFunc("POST /v1/cron-jobs/{id}/runs/{runId}/cancel", cancelCronRun)
	mux.HandleFunc("GET "+servicesBase+"/{id}/runs", listCronRuns)
	mux.HandleFunc("POST "+servicesBase+"/{id}/runs", runCron)
	mux.HandleFunc("DELETE "+servicesBase+"/{id}/runs", cancelCurrentCronRun)
	mux.HandleFunc("GET "+servicesBase+"/{id}/runs/{runId}", getCronRun)
	mux.HandleFunc("POST "+servicesBase+"/{id}/runs/{runId}/cancel", cancelCronRun)
}

// registerDomainRoutes mounts the per-service decoration sub-resources:
// custom domains, autoscaling, and the static-site edge rules.
func (s *Service) registerDomainRoutes(mux *http.ServeMux) {
	// Custom-domains sub-resource (Render-compatible).
	listDomains := core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		domains, err := s.ListDomains(r.Context(), r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		q := r.URL.Query()
		domains, err = filterDomains(domains, q.Get("verificationStatus"), q.Get("domainType"))
		if err != nil {
			return nil, err
		}
		// Cursor/limit pagination — cursor is the domain name, matching the per-item cursor
		// emitted by toCustomDomainList. Pagination is applied only when either cursor or
		// limit is explicitly provided (StablePage's "requested" flag).
		after, limit := core.PageParams(q)
		domains = core.StablePage(domains, after, limit, q.Has("cursor") || q.Has("limit"),
			func(d DomainView) string { return d.Name })
		return toCustomDomainList(domains), nil
	})
	addDomain := core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		var req struct {
			Name string `json:"name"`
		}
		if err := core.DecodeJSON(r, &req); err != nil || req.Name == "" {
			return nil, core.ErrBadRequest
		}
		d, err := s.AddDomain(r.Context(), r.PathValue("id"), req.Name)
		if err != nil {
			return nil, err
		}
		return toRenderCustomDomain(d), nil
	})
	getDomain := core.HandleMapped(http.StatusOK, func(r *http.Request) (DomainView, error) {
		return s.GetDomain(r.Context(), r.PathValue("id"), r.PathValue("name"))
	}, toRenderCustomDomain)
	deleteDomain := core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteDomain(r.Context(), r.PathValue("id"), r.PathValue("name"))
	})
	// verifyDomain promotes a pending ownership claim after an exact TXT match,
	// then returns the fresh ownership/TLS/serving state.
	verifyDomain := core.HandleMapped(http.StatusOK, func(r *http.Request) (DomainView, error) {
		return s.VerifyDomain(r.Context(), r.PathValue("id"), r.PathValue("name"))
	}, toRenderCustomDomain)

	// Autoscaling sub-resource (Render-compatible).
	// GET   …/autoscaling — current config (bex extension; Render has no GET)
	// PUT   …/autoscaling — upsert autoscaling (Render: PUT, 200)
	// DELETE …/autoscaling — disable autoscaling (Render: DELETE, 204)
	getAutoscaling := core.HandleByID(s.GetAutoscaling)
	putAutoscaling := core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[SetAutoscalingRequest](r)
		if err != nil {
			return nil, err
		}
		return s.SetAutoscaling(r.Context(), r.PathValue("id"), req)
	})
	deleteAutoscaling := core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteAutoscaling(r.Context(), r.PathValue("id"))
	})

	// Static-site edge rules (Render-compatible): /routes (redirects/rewrites) and
	// /headers (custom response headers). GET lists; PUT replaces the whole list.
	listRoutes := core.HandleMapped(http.StatusOK, func(r *http.Request) ([]StaticRouteView, error) {
		return s.ListRoutes(r.Context(), r.PathValue("id"))
	}, toRenderRoutes)
	putRoutes := core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		body, err := core.DecodeBody[[]StaticRouteView](r)
		if err != nil {
			return nil, err
		}
		app, err := s.SetRoutes(r.Context(), r.PathValue("id"), body)
		if err != nil {
			return nil, err
		}
		return toRenderRoutes(app.Routes), nil
	})
	listHeaders := core.HandleMapped(http.StatusOK, func(r *http.Request) ([]StaticHeaderView, error) {
		return s.ListHeaders(r.Context(), r.PathValue("id"))
	}, toRenderHeaders)
	putHeaders := core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		body, err := core.DecodeBody[[]StaticHeaderView](r)
		if err != nil {
			return nil, err
		}
		app, err := s.SetHeaders(r.Context(), r.PathValue("id"), body)
		if err != nil {
			return nil, err
		}
		return toRenderHeaders(app.Headers), nil
	})

	mux.HandleFunc("GET "+servicesBase+"/{id}/autoscaling", getAutoscaling)
	mux.HandleFunc("PUT "+servicesBase+"/{id}/autoscaling", putAutoscaling)
	mux.HandleFunc("DELETE "+servicesBase+"/{id}/autoscaling", deleteAutoscaling)
	mux.HandleFunc("GET "+servicesBase+"/{id}/custom-domains", listDomains)
	mux.HandleFunc("POST "+servicesBase+"/{id}/custom-domains", addDomain)
	mux.HandleFunc("GET "+servicesBase+"/{id}/custom-domains/{name}", getDomain)
	mux.HandleFunc("DELETE "+servicesBase+"/{id}/custom-domains/{name}", deleteDomain)
	mux.HandleFunc("POST "+servicesBase+"/{id}/custom-domains/{name}/verify", verifyDomain)
	mux.HandleFunc("GET "+servicesBase+"/{id}/routes", listRoutes)
	mux.HandleFunc("PUT "+servicesBase+"/{id}/routes", putRoutes)
	mux.HandleFunc("GET "+servicesBase+"/{id}/headers", listHeaders)
	mux.HandleFunc("PUT "+servicesBase+"/{id}/headers", putHeaders)
}

// registerBlueprintRoutes mounts the Blueprint routes (w2/m15 + w2/m41 +
// w2/m62): validate · create · list · get-by-id · sync · list-syncs · update ·
// disconnect.
// POST /v1/blueprints/validate is registered before POST /v1/blueprints/{id}/sync
// — Go 1.22+ ServeMux resolves the more specific (literal) path first.
func (s *Service) registerBlueprintRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/blueprints", core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		var body struct {
			Repo         string            `json:"repo"`
			Branch       string            `json:"branch"`
			Path         string            `json:"path"`
			Name         string            `json:"name"`
			OwnerID      string            `json:"ownerId"`
			EnvVarValues map[string]string `json:"envVarValues"`
			Confirm      string            `json:"confirm"`
		}
		if err := core.DecodeJSON(r, &body); err != nil {
			return nil, core.ErrBadRequest
		}
		return s.CreateBlueprint(r.Context(), body.OwnerID, CreateBlueprintRequest{
			Repo:         body.Repo,
			Branch:       body.Branch,
			Path:         body.Path,
			Name:         body.Name,
			EnvVarValues: body.EnvVarValues,
			Confirm:      body.Confirm,
		})
	}))
	mux.HandleFunc("GET /v1/blueprints/{id}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.GetBlueprintByID(r.Context(), r.PathValue("id"), r.URL.Query().Get("ownerId"))
	}))
	mux.HandleFunc("PATCH /v1/blueprints/{id}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		var body struct {
			Name     *string `json:"name"`
			AutoSync *bool   `json:"autoSync"`
			Path     *string `json:"path"`
			OwnerID  string  `json:"ownerId"`
		}
		if err := core.DecodeJSON(r, &body); err != nil {
			return nil, core.ErrBadRequest
		}
		return s.UpdateBlueprint(r.Context(), r.PathValue("id"), body.OwnerID, UpdateBlueprintRequest{
			Name:     body.Name,
			AutoSync: body.AutoSync,
			Path:     body.Path,
		})
	}))
	mux.HandleFunc("DELETE /v1/blueprints/{id}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DisconnectBlueprint(r.Context(), r.PathValue("id"), r.URL.Query().Get("ownerId"))
	}))
	mux.HandleFunc("GET /v1/blueprints/{id}/syncs", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		return s.ListBlueprintSyncs(r.Context(), r.PathValue("id"), q.Get("ownerId"), q.Get("cursor"), limit)
	}))
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
	mux.HandleFunc("POST /v1/blueprints/deploy", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		var body struct {
			OwnerID string `json:"ownerId"`
			Repo    string `json:"repo"`
			Branch  string `json:"branch"`
			BexYAML string `json:"bexYaml"`
			Confirm string `json:"confirm"`
		}
		if err := core.DecodeJSON(r, &body); err != nil || strings.TrimSpace(body.BexYAML) == "" {
			return nil, core.ErrBadRequest
		}
		return s.DeployStack(r.Context(), DeployRequest{
			OwnerID: body.OwnerID, Repo: body.Repo, Branch: body.Branch, Manifest: body.BexYAML, Confirm: body.Confirm,
		})
	}))
	mux.HandleFunc("POST /v1/blueprints/generate", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		var body struct {
			OwnerID     string   `json:"ownerId"`
			ServiceIDs  []string `json:"serviceIds"`
			PostgresIDs []string `json:"postgresIds"`
			KeyValueIDs []string `json:"keyValueIds"`
		}
		if err := core.DecodeJSON(r, &body); err != nil {
			return nil, err
		}
		return s.GenerateBlueprint(r.Context(), GenerateBlueprintRequest{
			OwnerID:     body.OwnerID,
			ServiceIDs:  body.ServiceIDs,
			PostgresIDs: body.PostgresIDs,
			KeyValueIDs: body.KeyValueIDs,
		})
	}))
	mux.HandleFunc("POST /v1/blueprints/preview", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		var body struct {
			Repo    string `json:"repo"`
			Branch  string `json:"branch"`
			Path    string `json:"path"`
			OwnerID string `json:"ownerId"`
		}
		if err := core.DecodeJSON(r, &body); err != nil {
			return nil, core.ErrBadRequest
		}
		return s.PreviewBlueprint(r.Context(), body.OwnerID, body.Repo, body.Branch, body.Path)
	}))
	mux.HandleFunc("GET /v1/blueprints", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.ListBlueprints(r.Context(), r.URL.Query().Get("ownerId"))
	}))
	mux.HandleFunc("POST /v1/blueprints/{id}/sync", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		var body struct {
			BexYAML string `json:"bexYaml"`
			OwnerID string `json:"ownerId"`
			Confirm string `json:"confirm"`
		}
		_ = core.DecodeJSON(r, &body)
		return s.SyncBlueprint(r.Context(), r.PathValue("id"), body.OwnerID, body.BexYAML, body.Confirm)
	}))
}

// notificationOverrideResponse is the per-service notification-override wire
// shape served under /v1/notification-settings/overrides/services/{id}.
type notificationOverrideResponse struct {
	NotificationsToSend         string `json:"notificationsToSend"`
	PreviewNotificationsEnabled string `json:"previewNotificationsEnabled"`
}

func toNotificationOverride(app AppView) notificationOverrideResponse {
	return notificationOverrideResponse{
		NotificationsToSend:         app.NotificationsToSend,
		PreviewNotificationsEnabled: "default",
	}
}

// registerNotificationOverrideRoutes mounts the per-service notification
// override pair: read the effective override, and PATCH notificationsToSend.
func (s *Service) registerNotificationOverrideRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/notification-settings/overrides/services/{id}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		app, err := s.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		return toNotificationOverride(app), nil
	}))
	mux.HandleFunc("PATCH /v1/notification-settings/overrides/services/{id}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		var body struct {
			NotificationsToSend         *string `json:"notificationsToSend"`
			PreviewNotificationsEnabled *string `json:"previewNotificationsEnabled"`
		}
		if err := core.DecodeJSON(r, &body); err != nil {
			return nil, fmt.Errorf("%w: invalid notification override body", core.ErrBadRequest)
		}
		if body.PreviewNotificationsEnabled != nil && *body.PreviewNotificationsEnabled != "default" {
			return nil, fmt.Errorf("%w: previewNotificationsEnabled is not supported; use default", core.ErrBadRequest)
		}
		if body.NotificationsToSend == nil {
			app, err := s.Get(r.Context(), r.PathValue("id"))
			if err != nil {
				return nil, err
			}
			return toNotificationOverride(app), nil
		}
		app, err := s.SetNotificationsToSend(r.Context(), r.PathValue("id"), *body.NotificationsToSend)
		if err != nil {
			return nil, err
		}
		return toNotificationOverride(app), nil
	}))
}

// Render permits a Blueprint file of up to 10 MiB. The multipart envelope gets
// a small separate allowance; the file itself is checked after decoding so a
// valid 10 MiB upload is not rejected merely for its MIME headers.
const (
	maxBlueprintValidationFileBytes = 10 << 20
	maxBlueprintValidationBodyBytes = maxBlueprintValidationFileBytes + (1 << 20)
)

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
		if err := core.DecodeJSON(r, &body); err != nil || body.BexYAML == "" {
			return "", "", core.ErrBadRequest
		}
		return body.OwnerID, body.BexYAML, nil
	}

	// Keep a local bound because feature-level tests and embedders can mount this
	// router without the composition root's middleware. It is intentionally the
	// Blueprint-specific file/envelope cap, not the stricter global API default.
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
	if len(content) > maxBlueprintValidationFileBytes {
		return "", "", fmt.Errorf("Blueprint file exceeds the 10 MiB validation limit")
	}
	return ownerID, string(content), nil
}
