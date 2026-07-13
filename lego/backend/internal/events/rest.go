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

package events

import (
	"net/http"
	"net/url"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the events REST fragment: Render's GET /services/{serviceId}/events,
// verified field-by-field against its OpenAPI (render-public-api-1.json,
// operationId list-events) rather than the rendered docs.
//
// Honored, exactly as Render spells them:
//
//	query  type       one event type to filter to
//	query  startTime  RFC3339; DEFAULT now-1h (Render's own default — an
//	                  unparameterized call means the same thing on both)
//	query  endTime    RFC3339; default now
//	query  cursor     opaque; resume after the last item of the previous page
//	query  limit      default 20, clamped to [1,100] (core.PageParams)
//	200               a BARE ARRAY of {event, cursor} — no outer object, no
//	                  hasMore/nextCursor (Render's list envelope for this endpoint)
//	event             id (evt-…), timestamp, serviceId, type, details — all five
//	                  present on every item, as Render marks them required
//
// Diverging, deliberately:
//
//   - details omits the from/to fields Render marks required on plan_changed /
//     instance_count_changed / autoscaling_config_changed — see the package doc's
//     Redaction section. A missing field, never a fabricated one.
//   - Render's `status` (deprecated integer) is not emitted on deploy_ended;
//     deployStatus, its replacement, is.
//   - Rejecting a malformed cursor with 400 (Render's behavior here is unspecified).
//
// Mounted under both /v1/services and /v1/apps, like every other apps-adjacent
// route.

// renderEvent is Render's event object.
type renderEvent struct {
	ID        string        `json:"id"`
	Timestamp string        `json:"timestamp"`
	ServiceID string        `json:"serviceId"`
	Type      string        `json:"type"`
	Details   renderDetails `json:"details"`
}

// renderDetails is the per-type payload. Every field is omitempty, so each type
// serializes exactly the members Render defines for it and a payload-less type
// (service_resumed, maintenance_ended, and bex's config-change types) serializes
// as `{}` — Render's own shape for those.
type renderDetails struct {
	DeployID        string         `json:"deployId,omitempty"`
	DeployStatus    string         `json:"deployStatus,omitempty"`
	Trigger         *renderTrigger `json:"trigger,omitempty"`
	Actor           string         `json:"actor,omitempty"`
	TriggeredByUser string         `json:"triggeredByUser,omitempty"`
}

// renderTrigger is deploy_started's trigger object — all six booleans always
// present, as Render marks them required.
type renderTrigger struct {
	FirstBuild       bool `json:"firstBuild"`
	EnvUpdated       bool `json:"envUpdated"`
	Manual           bool `json:"manual"`
	DeployedByRender bool `json:"deployedByRender"`
	ClearCache       bool `json:"clearCache"`
	Rollback         bool `json:"rollback"`
}

func toRenderEvent(e Event) renderEvent {
	d := renderDetails{
		DeployID:        e.Details.DeployID,
		DeployStatus:    e.Details.DeployStatus,
		Actor:           e.Details.Actor,
		TriggeredByUser: e.Details.TriggeredByUser,
	}
	if t := e.Details.Trigger; t != nil {
		d.Trigger = &renderTrigger{
			FirstBuild:       t.FirstBuild,
			EnvUpdated:       t.EnvUpdated,
			Manual:           t.Manual,
			DeployedByRender: t.DeployedByRender,
			ClearCache:       t.ClearCache,
			Rollback:         t.Rollback,
		}
	}
	return renderEvent{
		ID:        e.ID,
		Timestamp: e.At.UTC().Format(time.RFC3339),
		ServiceID: e.ServiceID,
		Type:      e.Type,
		Details:   d,
	}
}

// eventWithCursor is Render's list-item envelope for this endpoint: the cursor is
// a SIBLING of the event, not a member of it.
type eventWithCursor struct {
	Event  renderEvent `json:"event"`
	Cursor string      `json:"cursor"`
}

func toEventList(events []Event) []eventWithCursor {
	out := make([]eventWithCursor, 0, len(events))
	for _, e := range events {
		out = append(out, eventWithCursor{Event: toRenderEvent(e), Cursor: e.Cursor})
	}
	return out
}

// filterFromQuery translates Render's type/startTime/endTime/cursor/limit query
// params into Filter, over the one shared translator (FilterOf) the GraphQL and
// MCP fragments also use.
func filterFromQuery(q url.Values) Filter {
	cursor, limit := core.PageParams(q)
	return FilterOf(q.Get("type"), q.Get("startTime"), q.Get("endTime"), cursor, limit)
}

// RegisterREST mounts GET /v1/{services,apps}/{id}/events. Store unwired ⇒ the
// Service returns core.ErrEventsUnavailable ⇒ 503 on these routes only.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	for _, base := range []string{"/v1/services", "/v1/apps"} {
		mux.HandleFunc("GET "+base+"/{id}/events", func(w http.ResponseWriter, r *http.Request) {
			events, err := s.List(r.Context(), r.PathValue("id"), filterFromQuery(r.URL.Query()))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, toEventList(events))
		})
	}
}
