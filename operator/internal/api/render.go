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

import (
	"fmt"
	"hash/fnv"
)

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

// renderWebService is Render's serviceType for an HTTP service. Every bex App
// serves HTTP, so it's the only type bex reports.
const renderWebService = "web_service"

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
		Type:           renderWebService,
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

// --- Logs (Render logs-API shape) ---
//
// The MCP list_logs tool returns Core's LogEntry (map labels) verbatim, matching
// Render's MCP server. The REST logs API instead uses Render's public-API log
// object: a required id, and labels as a [{name,value}] array. These helpers map
// LogEntry onto that shape.

// renderLogTypeApp is Render's `type` label value for application logs. bex only
// sources application logs, so every REST log line is tagged with it.
const renderLogTypeApp = "app"

// renderLabel is Render's logLabel ({name, value}); the REST logs API returns
// labels as an ordered array rather than Core's map.
type renderLabel struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// renderLog is Render's public-API log object (id/message/timestamp/labels all
// required by the spec).
type renderLog struct {
	ID        string        `json:"id"`
	Message   string        `json:"message"`
	Timestamp string        `json:"timestamp"`
	Labels    []renderLabel `json:"labels"`
}

// renderLogList is the logs envelope; Render marks all four fields required, so
// all four keys are always present. The cursors bound the returned batch:
// nextStartTime = newest line, nextEndTime = oldest (the backward-page cursor).
type renderLogList struct {
	HasMore       bool        `json:"hasMore"`
	NextStartTime string      `json:"nextStartTime"`
	NextEndTime   string      `json:"nextEndTime"`
	Logs          []renderLog `json:"logs"`
}

// logID synthesizes a stable, unique id (Render ids are opaque; bex derives one
// from instance + timestamp + a message hash so the same line is always the same
// id).
func logID(e LogEntry) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(e.Message))
	return fmt.Sprintf("%s-%s-%08x", e.Labels["instance"], e.Timestamp, h.Sum32())
}

func toRenderLog(e LogEntry) renderLog {
	// Fixed label order (Render's names): type, then resource (Core's "service"),
	// instance, container — deterministic output, and service => Render's resource.
	labels := []renderLabel{{Name: "type", Value: renderLogTypeApp}}
	if v := e.Labels["service"]; v != "" {
		labels = append(labels, renderLabel{Name: "resource", Value: v})
	}
	if v := e.Labels["instance"]; v != "" {
		labels = append(labels, renderLabel{Name: "instance", Value: v})
	}
	if v := e.Labels["container"]; v != "" {
		labels = append(labels, renderLabel{Name: "container", Value: v})
	}
	return renderLog{ID: logID(e), Message: e.Message, Timestamp: e.Timestamp, Labels: labels}
}

func toRenderLogList(entries []LogEntry, limit int64) renderLogList {
	out := renderLogList{Logs: make([]renderLog, 0, len(entries))}
	for _, e := range entries {
		out.Logs = append(out.Logs, toRenderLog(e))
	}
	// entries are timestamp-sorted (oldest-first); cursors bound the batch.
	if n := len(entries); n > 0 {
		out.NextStartTime = entries[n-1].Timestamp // newest
		out.NextEndTime = entries[0].Timestamp     // oldest
	}
	out.HasMore = limit > 0 && int64(len(entries)) >= limit
	return out
}
