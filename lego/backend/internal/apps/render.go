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

import "github.com/bex-co/bex/lego/backend/internal/core"

// render.go maps AppView onto Render's public-API "service" shape, verified
// against Render's OpenAPI spec (render-public-api-1.json). A client written for
// Render sees the field names/enums/envelopes it expects; bex-only facts (phase,
// revision) are extra fields Render clients ignore — a superset, never a break.

// renderWebService is Render's default serviceType — reported for an App with no
// explicit spec.type set (view() already defaults empty => web_service).
const renderWebService = "web_service"

// renderService mirrors components.schemas.service (the fields bex has a real
// equivalent for) plus bex-native extras.
type renderService struct {
	ID             string         `json:"id"` // stable Render-shaped srv- id; legacy hand-applied CRs fall back to their name
	Name           string         `json:"name"`
	Slug           string         `json:"slug"` // globally-unique platform-host segment (w4/m19/w4/m20)
	DisplayName    string         `json:"displayName"`
	Type           string         `json:"type"` // serviceType enum: web_service | private_service | background_worker | cron_job
	Suspended      string         `json:"suspended"`
	DashboardURL   string         `json:"dashboardUrl,omitempty"`
	CreatedAt      string         `json:"createdAt,omitempty"`
	UpdatedAt      string         `json:"updatedAt,omitempty"`
	ServiceDetails map[string]any `json:"serviceDetails,omitempty"`
	ImagePath      string         `json:"imagePath,omitempty"`
	Suspenders     []string       `json:"suspenders"`

	// OwnerID is Render's workspace-scoping field (w6/m2/t004) — omitted for
	// Apps the control-plane projector never labeled (see AppView.OwnerID).
	OwnerID string `json:"ownerId,omitempty"`
	// EnvironmentID is Render's top-level environment membership field.
	EnvironmentID string `json:"environmentId,omitempty"`

	// bex-native superset (ignored by Render clients).
	Phase    string   `json:"phase,omitempty"`
	Replicas int32    `json:"replicas"`
	Revision string   `json:"revision,omitempty"`
	URLs     []string `json:"urls,omitempty"`
	// Schedule/Command/Runs describe a cron_job (Render nests schedule/command
	// under cronJobDetails and exposes runs at /cron-jobs/{id}/runs); empty
	// otherwise.
	Schedule            string        `json:"schedule,omitempty"`
	Command             string        `json:"command,omitempty"`
	Runs                []CronRunView `json:"runs,omitempty"`
	LastSuccessfulRunAt string        `json:"lastSuccessfulRunAt,omitempty"`
	IdleTTLSeconds      int32         `json:"idleTTLSeconds"` // free-tier auto-sleep window (bex extension; 0 = default)
	// RootDir is the subdirectory of the repo this service builds from (Render's
	// Root Directory setting, monorepo support). Empty is the repo root.
	RootDir string `json:"rootDir,omitempty"`
	// BuildFilter is Render's Build Filters object ({paths, ignoredPaths}, verified
	// against Render's service schema as a top-level field); omitted when unset.
	BuildFilter *BuildFilterView `json:"buildFilter,omitempty"`
	// Repo/Branch are the build-from-git source, empty for an image-backed App.
	Repo   string `json:"repo,omitempty"`
	Branch string `json:"branch,omitempty"`
	// Autoscaling mirrors Render's autoscaling sub-object (minInstances /
	// maxInstances / targetCPUPercent / targetMemoryPercent); omitted when
	// autoscaling is not configured.
	Autoscaling *renderAutoscaling `json:"autoscaling,omitempty"`
	// AutoDeploy is Render's Auto-Deploy toggle: whether a signed git push
	// redeploys this service (spec.autoDeploy). Render's public API represents
	// this as the string enum "yes"|"no" (components.schemas.autoDeploy), not a
	// JSON boolean — a client generated from Render's OpenAPI spec (e.g. the
	// official CLI) fails to unmarshal a bool here.
	AutoDeploy string `json:"autoDeploy"`
	// NotifyOnFail is Render's per-service deploy-failure notification override
	// (spec.notifyOnFail): default | notify | ignore. Required on Render's
	// service object (never omitted) — docs/render-artifacts/notify-on-fail.md.
	NotifyOnFail string `json:"notifyOnFail"`
	// HealthCheckPath is the HTTP path the ReadinessProbe pings (w1/m23/t001);
	// empty means the default "/". Render's healthCheckPath field.
	HealthCheckPath string `json:"healthCheckPath,omitempty"`
}

// renderAutoscaling is Render's autoscaling sub-object shape (verified against
// Render's PUT /v1/services/{id}/autoscaling request/response contract).
type renderAutoscaling struct {
	Enabled             bool   `json:"enabled"`
	MinInstances        int32  `json:"minInstances"`
	MaxInstances        int32  `json:"maxInstances"`
	TargetCPUPercent    *int32 `json:"targetCPUPercent,omitempty"`
	TargetMemoryPercent *int32 `json:"targetMemoryPercent,omitempty"`
}

// serviceWithCursor is components.schemas.serviceWithCursor — the list-item
// envelope. Render's GET /v1/services returns an array of these.
type serviceWithCursor struct {
	Service renderService `json:"service"`
	Cursor  string        `json:"cursor"`
}

// serviceAndDeploy is components.schemas.serviceAndDeploy — the envelope
// Render's POST /v1/services (create) returns, verified against the
// render-oss/cli generated client (ServiceAndDeploy: {deployId?, service?}). A
// client that unwraps `.service` from the create response (the official CLI's
// ServiceRepo.CreateService does exactly this) breaks against a bare service
// object. DeployId is omitted (bex's create doesn't yet mint a deploy record
// id inline) rather than faked — an honest subset, not a wrong value.
type serviceAndDeploy struct {
	Service renderService `json:"service"`
}

// renderServiceInstance is the official CLI's serviceInstance wire shape.
type renderServiceInstance struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
}

// yesNoEnum renders Render's autoDeploy bool as its wire enum ("yes"/"no").
func yesNoEnum(autoDeploy bool) string {
	if autoDeploy {
		return "yes"
	}
	return "no"
}

func toRenderService(a AppView) renderService {
	publicID := a.ID
	if publicID == "" {
		publicID = a.Name // hand-applied/store-less compatibility
	}
	svcType := a.Type
	if svcType == "" {
		svcType = renderWebService // defensive; view() already defaults this
	}
	var details map[string]any
	set := func(k string, v any) {
		if details == nil {
			details = map[string]any{}
		}
		details[k] = v
	}
	if a.URL != "" {
		set("url", a.URL) // Render web_service exposes the live URL here
	}
	if a.Plan != "" {
		set("plan", a.Plan) // webServiceDetails.plan (render-public-api-1.json)
	}
	if a.Runtime != "" {
		set("runtime", a.Runtime)
		set("env", a.Runtime) // deprecated Render response field, still required by its schema
		set("region", "oregon")
	}
	if a.Runtime == "docker" {
		dockerDetails := map[string]any{
			"dockerCommand":  a.StartCommand,
			"dockerContext":  a.RootDir,
			"dockerfilePath": a.DockerfilePath,
		}
		if a.PreDeployCommand != "" {
			dockerDetails["preDeployCommand"] = a.PreDeployCommand
		}
		set("envSpecificDetails", dockerDetails)
	} else if a.Runtime != "" {
		startCommand := a.StartCommand
		if svcType == "cron_job" {
			startCommand = a.Command
		}
		set("envSpecificDetails", map[string]any{
			"buildCommand":     a.BuildCommand,
			"startCommand":     startCommand,
			"preDeployCommand": a.PreDeployCommand,
		})
	}
	if a.Schedule != "" {
		set("schedule", a.Schedule) // cronJobDetails.schedule (render-public-api-1.json)
	}
	if a.Command != "" {
		set("command", a.Command) // cronJobDetails.command (render-public-api-1.json)
	}
	if a.LastSuccessfulRunAt != "" {
		set("lastSuccessfulRunAt", a.LastSuccessfulRunAt) // cronJobDetails.lastSuccessfulRunAt
	}
	if a.PublishPath != "" {
		set("publishPath", a.PublishPath) // staticSiteDetails.publishPath (render-public-api-1.json)
	}
	if svcType == "static_site" {
		set("buildCommand", a.BuildCommand)
	}
	if a.PreDeployCommand != "" {
		set("preDeployCommand", a.PreDeployCommand) // webServiceDetails.preDeployCommand (w1/m33)
	}
	if a.MaxShutdownDelaySeconds > 0 {
		// Render places this on web/private/background-worker serviceDetails.
		// view() supplies the shared Render/Kubernetes default (30) when the CR
		// pointer is unset, so legacy services read back exactly like Render.
		set("maxShutdownDelaySeconds", a.MaxShutdownDelaySeconds)
	}
	set("numInstances", int(a.Replicas))
	if a.HealthCheckPath != "" {
		set("healthCheckPath", a.HealthCheckPath)
	}
	var ras *renderAutoscaling
	if a.Autoscaling != nil {
		ras = &renderAutoscaling{
			Enabled:             a.Autoscaling.Enabled,
			MinInstances:        a.Autoscaling.MinInstances,
			MaxInstances:        a.Autoscaling.MaxInstances,
			TargetCPUPercent:    a.Autoscaling.TargetCPUPercent,
			TargetMemoryPercent: a.Autoscaling.TargetMemoryPercent,
		}
	}
	return renderService{
		ID:                  publicID,
		Name:                renderServiceName(a),
		Slug:                a.Slug,
		DisplayName:         a.DisplayName,
		Type:                svcType,
		Suspended:           core.SuspendedEnum(a.Suspended),
		DashboardURL:        a.URL,
		CreatedAt:           a.CreatedAt,
		UpdatedAt:           a.CreatedAt,
		ServiceDetails:      details,
		ImagePath:           a.SourceImage,
		Suspenders:          []string{},
		OwnerID:             a.OwnerID,
		EnvironmentID:       a.EnvironmentID,
		Phase:               a.Phase,
		Replicas:            a.Replicas,
		Revision:            a.Revision,
		URLs:                a.URLs,
		Schedule:            a.Schedule,
		Command:             a.Command,
		Runs:                a.Runs,
		LastSuccessfulRunAt: a.LastSuccessfulRunAt,
		IdleTTLSeconds:      a.IdleTTLSeconds,
		RootDir:             a.RootDir,
		BuildFilter:         a.BuildFilter,
		Repo:                a.Repo,
		Branch:              a.Branch,
		Autoscaling:         ras,
		AutoDeploy:          yesNoEnum(a.AutoDeploy),
		NotifyOnFail:        a.NotifyOnFail,
		HealthCheckPath:     a.HealthCheckPath,
	}
}

// renderServiceName maps bex's immutable public name + mutable display label
// onto Render's contract, where service.name itself is mutable. The typed id
// remains stable and continues to address the App CR through LabelAppID.
func renderServiceName(a AppView) string {
	if a.DisplayName != "" {
		return a.DisplayName
	}
	return a.Name
}

// renderCronJobRun mirrors Render's cronJobRun schema. triggeredBy/canceledBy
// are omitted because Kubernetes Jobs do not retain the caller identity; an
// honest subset is preferable to fabricating an actor.
type renderCronJobRun struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	StartedAt   string `json:"startedAt,omitempty"`
	FinishedAt  string `json:"finishedAt,omitempty"`
	TriggeredBy string `json:"triggeredBy,omitempty"`
	CanceledBy  string `json:"canceledBy,omitempty"`
}

func toRenderCronJobRun(run CronRunView) renderCronJobRun {
	return renderCronJobRun{
		ID: run.ID, Status: run.Status,
		StartedAt: run.StartedAt, FinishedAt: run.FinishedAt,
	}
}

// cronJobRunWithCursor is the standard Render list-item envelope. Render's
// current public spec does not expose this historical list route; bex keeps the
// milestone's first-class list as a documented extension using Render's normal
// {resource,cursor} grammar.
type cronJobRunWithCursor struct {
	CronJobRun renderCronJobRun `json:"cronJobRun"`
	Cursor     string           `json:"cursor"`
}

func toCronJobRunList(runs []CronRunView) []cronJobRunWithCursor {
	out := make([]cronJobRunWithCursor, 0, len(runs))
	for _, run := range runs {
		out = append(out, cronJobRunWithCursor{CronJobRun: toRenderCronJobRun(run), Cursor: run.ID})
	}
	return out
}

// renderRoute mirrors Render's static-site route shape
// (components.schemas.route: type/source/destination) for the /routes endpoints.
type renderRoute struct {
	Type        string `json:"type"` // redirect | rewrite
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// renderHeader mirrors Render's static-site custom-header shape
// (path/name/value) for the /headers endpoints.
type renderHeader struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

func toRenderRoutes(routes []StaticRouteView) []renderRoute {
	out := make([]renderRoute, 0, len(routes))
	for _, r := range routes {
		out = append(out, renderRoute{Type: r.Type, Source: r.Source, Destination: r.Destination})
	}
	return out
}

func toRenderHeaders(headers []StaticHeaderView) []renderHeader {
	out := make([]renderHeader, 0, len(headers))
	for _, h := range headers {
		out = append(out, renderHeader{Path: h.Path, Name: h.Name, Value: h.Value})
	}
	return out
}

// toRenderServices maps a slice of AppViews to bare Render service objects (no
// cursor envelope) — the shape the MCP list_services tool returns.
func toRenderServices(apps []AppView) []renderService {
	out := make([]renderService, 0, len(apps))
	for _, a := range apps {
		out = append(out, toRenderService(a))
	}
	return out
}

func toServiceList(apps []AppView) []serviceWithCursor {
	out := make([]serviceWithCursor, 0, len(apps))
	for _, a := range apps {
		// cursor is opaque in Render; the App name is a stable, valid cursor.
		out = append(out, serviceWithCursor{Service: toRenderService(a), Cursor: a.Name})
	}
	return out
}
