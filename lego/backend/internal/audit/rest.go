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

package audit

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go mounts Render's owner-scoped audit-logs path
// (GET /owners/{ownerId}/audit-logs, docs/ADR018-render-parity.md "Audit logs" row) —
// NOT a bex-own /v1/audit-events noun (t003's explicit instruction). Render's
// dashboard documents each entry as Timestamp/Actor/Event/Status/Metadata
// columns (render.com/docs/audit-logs); its exact JSON field names weren't
// resolvable from public docs at authoring time (api-docs.render.com's
// reference page lists only the query parameters), so the field names below
// are bex's best-effort rendering of that vocabulary — divergence tracked by
// t007/docs/ADR018-render-parity.md, not silently assumed byte-identical.
//
// renderAuditLog renders one Event in that vocabulary. status is "success"/
// "denied" (bex's outcome is binary allow/deny, not Render's success/error —
// a documented divergence: an authorization denial isn't the same thing as an
// action that started and then errored). resource is a bex extra (the OpenFGA
// object authorized against) Render has no equivalent field for.
type renderAuditLog struct {
	ID          string               `json:"id"`
	Timestamp   string               `json:"timestamp"`
	Actor       string               `json:"actor"`
	ActorMethod string               `json:"actorMethod,omitempty"` // bex extra: "oauth2" | "session"
	Action      string               `json:"action"`
	Status      string               `json:"status"`             // "success" | "denied"
	Resource    string               `json:"resource,omitempty"` // bex extra
	Metadata    *renderAuditMetadata `json:"metadata,omitempty"`
}

// renderAuditMetadata is intentionally closed: Render defines `to` for a
// MaintenanceModeEnabledEvent, and no arbitrary verb argument can be added.
type renderAuditMetadata struct {
	To *bool `json:"to,omitempty"`
}

func renderStatus(outcome string) string {
	if outcome == string(core.AuditDenied) {
		return "denied"
	}
	return "success"
}

func renderAction(verb string) string {
	switch verb {
	case core.AuditVerbMaintenanceModeEnabled:
		return "MaintenanceModeEnabledEvent"
	case core.AuditVerbMaintenanceModeURIUpdated:
		return "MaintenanceModeURIUpdatedEvent"
	default:
		return verb
	}
}

func toRenderAuditLog(e Event) renderAuditLog {
	out := renderAuditLog{
		ID:          e.ID,
		Timestamp:   e.At.UTC().Format(time.RFC3339),
		Actor:       e.Caller,
		ActorMethod: e.CallerMethod,
		Action:      renderAction(e.Verb),
		Status:      renderStatus(e.Outcome),
		Resource:    e.Resource,
	}
	if e.MaintenanceModeTo != nil {
		out.Metadata = &renderAuditMetadata{To: e.MaintenanceModeTo}
	} else if e.Verb == core.AuditVerbMaintenanceModeURIUpdated {
		out.Metadata = &renderAuditMetadata{}
	}
	return out
}

// auditLogWithCursor is the list-item envelope — the same {object, cursor}
// shape deploys/env-vars use (bex's own established pagination story; Render's
// own audit-logs response envelope wasn't resolvable from public docs — see
// the package doc above). The event id is a stable, opaque cursor.
type auditLogWithCursor struct {
	AuditLog renderAuditLog `json:"auditLog"`
	Cursor   string         `json:"cursor"`
}

func toAuditLogList(events []Event) []auditLogWithCursor {
	out := make([]auditLogWithCursor, 0, len(events))
	for _, e := range events {
		out = append(out, auditLogWithCursor{AuditLog: toRenderAuditLog(e), Cursor: e.ID})
	}
	return out
}

// filterFromQuery translates Render's startTime/endTime/cursor/limit query
// params into Filter. direction is accepted and ignored (bex always returns
// newest-first — the safe-superset rule every Render-shaped list here
// follows, same as deploys' clearCache).
func filterFromQuery(q url.Values) Filter {
	var f Filter
	if v := q.Get("startTime"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Since = t
		}
	}
	if v := q.Get("endTime"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Until = t
		}
	}
	f.Cursor = q.Get("cursor")
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	}
	return f
}

// RegisterREST mounts GET /v1/owners/{ownerId}/audit-logs. Store unconfigured
// => the Service returns core.ErrAuditUnavailable => 503; non-admin caller =>
// 403 (core.ErrForbidden) — both via the shared core.WriteErr mapper.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/owners/{ownerId}/audit-logs", func(w http.ResponseWriter, r *http.Request) {
		events, err := s.List(r.Context(), r.PathValue("ownerId"), filterFromQuery(r.URL.Query()))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toAuditLogList(events))
	})
}
