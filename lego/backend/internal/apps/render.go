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
	ID             string         `json:"id"` // Render ids are opaque; bex uses the App name
	Name           string         `json:"name"`
	DisplayName    string         `json:"displayName"`
	Type           string         `json:"type"` // serviceType enum: web_service | private_service | background_worker | cron_job
	Suspended      string         `json:"suspended"`
	DashboardURL   string         `json:"dashboardUrl,omitempty"`
	CreatedAt      string         `json:"createdAt,omitempty"`
	ServiceDetails map[string]any `json:"serviceDetails,omitempty"`

	// OwnerID is Render's workspace-scoping field (w6/m2/t004) — omitted for
	// Apps the control-plane projector never labeled (see AppView.OwnerID).
	OwnerID string `json:"ownerId,omitempty"`

	// bex-native superset (ignored by Render clients).
	Phase    string   `json:"phase,omitempty"`
	Replicas int32    `json:"replicas"`
	Revision string   `json:"revision,omitempty"`
	URLs     []string `json:"urls,omitempty"`
	// Schedule/Command/Runs describe a cron_job (Render nests schedule/command
	// under cronJobDetails and exposes runs at /cron-jobs/{id}/runs); empty
	// otherwise.
	Schedule       string        `json:"schedule,omitempty"`
	Command        string        `json:"command,omitempty"`
	Runs           []CronRunView `json:"runs,omitempty"`
	IdleTTLSeconds int32         `json:"idleTTLSeconds"` // free-tier auto-sleep window (bex extension; 0 = default)
	// RootDir is the subdirectory of the repo this service builds from (Render's
	// Root Directory setting, monorepo support). Empty is the repo root.
	RootDir string `json:"rootDir,omitempty"`
	// Repo/Branch are the build-from-git source, empty for an image-backed App.
	Repo   string `json:"repo,omitempty"`
	Branch string `json:"branch,omitempty"`
	// Autoscaling mirrors Render's autoscaling sub-object (minInstances /
	// maxInstances / targetCPUPercent / targetMemoryPercent); omitted when
	// autoscaling is not configured.
	Autoscaling *renderAutoscaling `json:"autoscaling,omitempty"`
	// AutoDeploy is Render's Auto-Deploy toggle: whether a signed git push
	// redeploys this service (spec.autoDeploy).
	AutoDeploy bool `json:"autoDeploy"`
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

func toRenderService(a AppView) renderService {
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
	if a.Schedule != "" {
		set("schedule", a.Schedule) // cronJobDetails.schedule (render-public-api-1.json)
	}
	if a.Command != "" {
		set("command", a.Command) // cronJobDetails.command (render-public-api-1.json)
	}
	if a.PublishPath != "" {
		set("publishPath", a.PublishPath) // staticSiteDetails.publishPath (render-public-api-1.json)
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
		ID:              a.Name,
		Name:            a.Name,
		DisplayName:     a.DisplayName,
		Type:            svcType,
		Suspended:       core.SuspendedEnum(a.Suspended),
		DashboardURL:    a.URL,
		CreatedAt:       a.CreatedAt,
		ServiceDetails:  details,
		OwnerID:         a.OwnerID,
		Phase:           a.Phase,
		Replicas:        a.Replicas,
		Revision:        a.Revision,
		URLs:            a.URLs,
		Schedule:        a.Schedule,
		Command:         a.Command,
		Runs:            a.Runs,
		IdleTTLSeconds:  a.IdleTTLSeconds,
		RootDir:         a.RootDir,
		Repo:            a.Repo,
		Branch:          a.Branch,
		Autoscaling:     ras,
		AutoDeploy:      a.AutoDeploy,
		HealthCheckPath: a.HealthCheckPath,
	}
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
