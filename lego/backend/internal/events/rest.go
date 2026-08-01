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
//   - Render's `status` (deprecated integer) is not emitted on deploy_ended;
//     deployStatus, its replacement, is.
//   - Rejecting a malformed cursor with 400 (Render's behavior here is unspecified).
//
// Mounted under /v1/services, like every other apps-adjacent
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
//
// from/to use `any` so the same JSON keys work for string (plan_changed) and
// int (instance_count_changed): only one set is non-nil per event, omitempty
// suppresses the other. autoscaling_config_changed uses previous/current objects.
//
// Deploy detail enrichment (w1/m47): image, commit info, timing — populated for
// deploy events to enable rich dashboard display without additional queries.
type renderDetails struct {
	DeployID        string         `json:"deployId,omitempty"`
	DeployStatus    string         `json:"deployStatus,omitempty"`
	PreDeployStatus string         `json:"preDeployStatus,omitempty"` // bex extra (w1/m33): the deploy's pre-deploy step outcome
	// Status is the terminal outcome of a lifecycle-step event (w7/m66):
	// build_ended / pre_deploy_ended / job_run_ended carry succeeded|failed|canceled.
	Status  string         `json:"status,omitempty"`
	Trigger *renderTrigger `json:"trigger,omitempty"`
	// Deploy enrichment for dashboard (w1/m47)
	Image           string `json:"image,omitempty"`
	CommitID        string `json:"commitId,omitempty"`
	CommitMessage   string `json:"commitMessage,omitempty"`
	StartedAt       string `json:"startedAt,omitempty"`  // RFC3339; nil → omitted
	FinishedAt      string `json:"finishedAt,omitempty"` // RFC3339; nil → omitted
	Actor           string `json:"actor,omitempty"`
	TriggeredByUser string `json:"triggeredByUser,omitempty"`
	ReasonCode      string `json:"reasonCode,omitempty"`
	InstanceID      string `json:"instanceId,omitempty"`
	CommitURL       string `json:"commitUrl,omitempty"`
	FromInstances   *int32 `json:"fromInstances,omitempty"`
	ToInstances     *int32 `json:"toInstances,omitempty"`
	// plan_changed and instance_count_changed both use "from"/"to" — the value
	// type (string vs int pointer) differs, so `any` is required for a flat struct.
	From any `json:"from,omitempty"`
	To   any `json:"to,omitempty"`
	// autoscaling_config_changed uses nested previous/current objects.
	Previous *autoscalingState `json:"previous,omitempty"`
	Current  *autoscalingState `json:"current,omitempty"`
}

// autoscalingState is the per-config snapshot for autoscaling_config_changed
// details — previous and current, matching Render's shape.
type autoscalingState struct {
	Enabled      bool   `json:"enabled"`
	MinInstances *int32 `json:"minInstances,omitempty"`
	MaxInstances *int32 `json:"maxInstances,omitempty"`
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
		PreDeployStatus: e.Details.PreDeployStatus,
		Status:          e.Details.Status,
		Image:           e.Details.Image,
		CommitID:        e.Details.CommitID,
		CommitMessage:   e.Details.CommitMessage,
		StartedAt:       formatTime(e.Details.StartedAt),
		FinishedAt:      formatTime(e.Details.FinishedAt),
		Actor:           e.Details.Actor,
		TriggeredByUser: e.Details.TriggeredByUser,
		ReasonCode:      e.Details.ReasonCode,
		InstanceID:      e.Details.InstanceID,
		CommitURL:       e.Details.CommitURL,
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
	switch e.Type {
	case TypePlanChanged:
		d.From = e.Details.PlanFrom
		d.To = e.Details.PlanTo
	case TypeInstanceCountChanged:
		d.From = e.Details.InstanceCountFrom
		d.To = e.Details.InstanceCountTo
	case TypeAutoscalingConfigChanged:
		if e.Details.AutoscalingMinFrom != nil || e.Details.AutoscalingMaxFrom != nil {
			d.Previous = &autoscalingState{Enabled: true, MinInstances: e.Details.AutoscalingMinFrom, MaxInstances: e.Details.AutoscalingMaxFrom}
		} else {
			d.Previous = &autoscalingState{Enabled: false}
		}
		if e.Details.AutoscalingMinTo != nil || e.Details.AutoscalingMaxTo != nil {
			d.Current = &autoscalingState{Enabled: true, MinInstances: e.Details.AutoscalingMinTo, MaxInstances: e.Details.AutoscalingMaxTo}
		} else {
			d.Current = &autoscalingState{Enabled: false}
		}
	case TypeBranchChanged:
		d.From = e.Details.BranchFrom
		d.To = e.Details.BranchTo
	case TypeAutoscalingStarted, TypeAutoscalingEnded:
		// Render's event-detail contract calls these fromInstances/toInstances,
		// unlike manual scaling's generic from/to pair.
		d.FromInstances = e.Details.FromCount
		d.ToInstances = e.Details.ToCount
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

// RegisterREST mounts GET /v1/services/{id}/events. Store unwired ⇒ the
// Service returns core.ErrEventsUnavailable ⇒ 503 on these routes only.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	const base = "/v1/services"
	mux.HandleFunc("GET "+base+"/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		events, err := s.List(r.Context(), r.PathValue("id"), filterFromQuery(r.URL.Query()))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toEventList(events))
	})
}
