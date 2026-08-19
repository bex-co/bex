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
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go mounts Render's owner-scoped audit-logs path
// (GET /owners/{ownerId}/audit-logs, docs/ADR018-render-parity.md "Audit logs" row) —
// NOT a bex-own /v1/audit-events noun (t003's explicit instruction). The wire
// shape below matches Render's published `auditLog` schema, captured from the
// public OpenAPI 2026-07-15 (w4/m26 — the schema was unresolvable from public
// docs when this surface was built; docs/render-artifacts/audit-logs-api.md
// records the capture): required id/timestamp/event/status/actor/metadata,
// actor an object {type, email?, id?}, metadata a string map, and status the
// closed success|error enum.
//
// renderAuditLog renders one Event in that vocabulary. resource is a bex
// extra (the OpenFGA object authorized against) Render has no equivalent
// field for; extra fields are tolerated by Render's open object schema.
type renderAuditLog struct {
	ID        string            `json:"id"`
	Timestamp string            `json:"timestamp"`
	Event     string            `json:"event"`
	Status    string            `json:"status"` // "success" | "error" (Render's closed enum)
	Actor     renderAuditActor  `json:"actor"`
	Metadata  map[string]string `json:"metadata"`           // always present, string values (Render's additionalProperties: string)
	Resource  string            `json:"resource,omitempty"` // bex extra
}

// renderAuditActor is Render's actor object. type is the closed
// user|rest_api|system enum (actorType); id carries bex's caller subject.
// email is omitted: the audit row deliberately stores only the subject
// (redaction-by-structure), and Render marks the field optional.
type renderAuditActor struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
}

// actorType maps bex's stored caller method onto Render's closed actor.type
// enum: a Kratos browser session is a human "user"; an oauth2 bearer (API
// key or OAuth client) is "rest_api"; bex's own system rows — and an
// unattributed caller (e.g. a denied unauthenticated request), which has no
// better bucket in Render's vocabulary — are "system".
func actorType(method string) string {
	switch method {
	case "session":
		return "user"
	case "oauth2":
		return "rest_api"
	default:
		return "system"
	}
}

// renderStatus maps bex's binary allow/deny outcome onto Render's closed
// success|error enum: a denial is an action that did not succeed. The
// distinction Render's enum cannot carry is preserved in metadata
// ("outcome": "denied") — nothing accepted is dropped. GraphQL (bex's own
// dialect, not a Render surface) keeps the richer "denied" value directly.
func renderStatus(e Event) string {
	if denied(e) {
		return "error"
	}
	return "success"
}

// renderMetadata builds Render's required string-map metadata: the audited
// target keyed by its kind (Render's own example is {"service": "srv-…"}),
// the maintenance-mode `to` flag, and the denied marker renderStatus's enum
// mapping would otherwise lose. Always non-nil so the field serializes as {}
// rather than null (the schema requires it).
func renderMetadata(e Event) map[string]string {
	md := map[string]string{}
	if kind, name, ok := core.SplitTarget(e.Target); ok {
		md[kind] = name
	}
	// Target display name (generic across all target kinds).
	if e.TargetName != "" {
		md["targetName"] = e.TargetName
	}
	if e.MaintenanceModeTo != nil {
		md["to"] = strconv.FormatBool(*e.MaintenanceModeTo)
	}
	// The invited/changed role pair (w1/m33).
	if e.RoleFrom != nil {
		md["roleFrom"] = *e.RoleFrom
	}
	if e.RoleTo != nil {
		md["roleTo"] = *e.RoleTo
	}
	if e.Relation != "" {
		md["relation"] = e.Relation
	}
	if e.OAuthClientID != "" {
		md["oauthClientId"] = e.OAuthClientID
	}
	if e.OAuthAudience != "" {
		md["oauthAudience"] = e.OAuthAudience
	}
	if len(e.OAuthScopes) > 0 {
		md["oauthScopes"] = strings.Join(e.OAuthScopes, " ")
	}
	if denied(e) {
		md["outcome"] = "denied"
	}
	return md
}

func toRenderAuditLog(e Event) renderAuditLog {
	return renderAuditLog{
		ID:        e.ID,
		Timestamp: e.At.UTC().Format(time.RFC3339),
		Event:     renderEvent(e.Verb),
		Status:    renderStatus(e),
		Actor:     renderAuditActor{Type: actorType(e.CallerMethod), ID: e.Caller},
		Metadata:  renderMetadata(e),
		Resource:  e.Resource,
	}
}

// auditLogWithCursor is Render's list-item envelope — the response is an
// array of {cursor, auditLog}, both required (auditLogWithCursor in the
// captured OpenAPI; the same {object, cursor} pattern deploys/env-vars use).
// The event id is a stable, opaque cursor.
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

// filterFromQuery translates Render's startTime/endTime/direction/cursor/
// limit query params into Filter, over the one shared translator (FilterOf)
// the GraphQL fragment also uses. direction is honored (w4/013) and validated
// in Service.List; the time bounds are validated in FilterOf.
//
// limit keeps the tolerant Atoi reading: this list clamps rather than rejects
// (see Filter.Limit), so an unparseable one falls through to the store's real
// default page. Contrast deploys and jobs, whose stores read <=0 as absent and
// answer with the MAX page — which is why they reject instead.
func filterFromQuery(q url.Values) (Filter, error) {
	limit := 0
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	return FilterOf(q.Get("startTime"), q.Get("endTime"), q.Get("direction"), q.Get("cursor"), limit)
}

// RegisterREST mounts GET /v1/owners/{ownerId}/audit-logs. Store unconfigured
// => the Service returns core.ErrAuditUnavailable => 503; non-admin caller =>
// 403 (core.ErrForbidden) — both via the shared core.WriteErr mapper.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/owners/{ownerId}/audit-logs", func(w http.ResponseWriter, r *http.Request) {
		filter, err := filterFromQuery(r.URL.Query())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		events, err := s.List(r.Context(), r.PathValue("ownerId"), filter)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toAuditLogList(events))
	})
}
