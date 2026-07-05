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

package api

// render.go maps bex's neutral AppView onto Render's public-API "service" shape,
// verified against Render's OpenAPI spec (render-public-api-1.json). This is the
// compatibility layer: a client written for Render's REST/GraphQL API sees the
// field names, enum values and envelopes it expects. bex-only facts (phase,
// revision) are added as extra fields — Render clients ignore unknown keys, so
// the object is a superset, never a break.

// Suspended enum values (Render: service.suspended is a string enum, NOT a bool).
const (
	renderSuspended    = "suspended"
	renderNotSuspended = "not_suspended"
)

// renderService mirrors components.schemas.service (the fields bex has a real
// equivalent for) plus bex-native extras.
type renderService struct {
	ID             string         `json:"id"` // Render ids are opaque; bex uses the App name
	Name           string         `json:"name"`
	Type           string         `json:"type"` // serviceType enum; bex Apps serve HTTP => web_service
	Suspended      string         `json:"suspended"`
	DashboardURL   string         `json:"dashboardUrl,omitempty"`
	CreatedAt      string         `json:"createdAt,omitempty"`
	ServiceDetails map[string]any `json:"serviceDetails,omitempty"`

	// bex-native superset (ignored by Render clients).
	Phase    string   `json:"phase,omitempty"`
	Replicas int32    `json:"replicas"`
	Revision string   `json:"revision,omitempty"`
	URLs     []string `json:"urls,omitempty"`
}

// serviceWithCursor is components.schemas.serviceWithCursor — the list-item
// envelope. Render's GET /v1/services returns an array of these.
type serviceWithCursor struct {
	Service renderService `json:"service"`
	Cursor  string        `json:"cursor"`
}

func toRenderService(a AppView) renderService {
	susp := renderNotSuspended
	if a.Suspended {
		susp = renderSuspended
	}
	var details map[string]any
	if a.URL != "" {
		details = map[string]any{"url": a.URL} // Render web_service exposes the live URL here
	}
	return renderService{
		ID:             a.Name,
		Name:           a.Name,
		Type:           "web_service",
		Suspended:      susp,
		DashboardURL:   a.URL,
		CreatedAt:      a.CreatedAt,
		ServiceDetails: details,
		Phase:          a.Phase,
		Replicas:       a.Replicas,
		Revision:       a.Revision,
		URLs:           a.URLs,
	}
}

func toServiceList(apps []AppView) []serviceWithCursor {
	out := make([]serviceWithCursor, 0, len(apps))
	for _, a := range apps {
		// cursor is opaque in Render; the App name is a stable, valid cursor.
		out = append(out, serviceWithCursor{Service: toRenderService(a), Cursor: a.Name})
	}
	return out
}
