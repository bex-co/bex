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

package webhooks

import (
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the REST fragment. It serves Render's public route family
// (POST/GET /v1/webhooks, GET/PATCH/DELETE /v1/webhooks/{id},
// GET /v1/webhooks/{id}/events — per the live spec re-fetched w3/m27).
// /v1/webhooks/git (the inbound push
// webhook) is a sibling route registered by the webhooks-git package and is
// unaffected by any pattern below.

// endpointWire is the wire shape every surface renders. Secret is present on
// the create response ONLY — the mint-once read; every other render omits it
// (empty + omitempty, and no read query ever selects the column).
type endpointWire struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	EventFilter []string `json:"eventFilter"`
	Enabled     bool     `json:"enabled"`
	Secret      string   `json:"secret,omitempty"`
}

func toWire(v EndpointView) endpointWire {
	eventFilter := v.EventTypes
	if eventFilter == nil {
		eventFilter = []string{}
	}
	return endpointWire{
		ID: v.ID, Name: v.Name, URL: v.URL,
		EventFilter: eventFilter,
		Enabled:     v.Enabled, Secret: v.Secret,
	}
}

type endpointWithCursor struct {
	Webhook endpointWire `json:"webhook"`
	Cursor  string       `json:"cursor"`
}

func toWirePage(views []EndpointView) []endpointWithCursor {
	out := make([]endpointWithCursor, 0, len(views))
	for _, v := range views {
		out = append(out, endpointWithCursor{Webhook: toWire(v), Cursor: v.Cursor})
	}
	return out
}

func toWireList(views []EndpointView) []endpointWire {
	out := make([]endpointWire, 0, len(views))
	for _, v := range views {
		out = append(out, toWire(v))
	}
	return out
}

// webhookEventWire is Render's public delivery-history item. Rich bex retry
// state remains on GraphQL/MCP; REST uses Render's exact supported fields.
type webhookEventWire struct {
	ID           string `json:"id"`
	EventID      string `json:"eventId"`
	EventType    string `json:"eventType"`
	SentAt       string `json:"sentAt"`
	StatusCode   int    `json:"statusCode,omitempty"`
	ResponseBody string `json:"responseBody,omitempty"`
	Error        string `json:"error,omitempty"`
}

type webhookEventWithCursor struct {
	WebhookEvent webhookEventWire `json:"webhookEvent"`
	Cursor       string           `json:"cursor"`
}

func toWebhookEventWire(v DeliveryView) webhookEventWire {
	w := webhookEventWire{
		ID: v.ID, EventID: v.EventID, EventType: v.EventType, SentAt: v.SentAt,
		StatusCode: v.StatusCode, ResponseBody: v.ResponseBody,
	}
	// Render's `error` is for failures without an HTTP response. A non-2xx
	// response is represented by statusCode + responseBody alone.
	if v.StatusCode == 0 {
		w.Error = v.TransportError
	}
	return w
}

func toWebhookEventList(views []DeliveryView) []webhookEventWithCursor {
	out := make([]webhookEventWithCursor, 0, len(views))
	for _, v := range views {
		out = append(out, webhookEventWithCursor{WebhookEvent: toWebhookEventWire(v), Cursor: v.Cursor})
	}
	return out
}

// createEndpointRequest is POST /v1/webhooks' canonical Render body.
type createEndpointRequest struct {
	OwnerID     *string   `json:"ownerId"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	EventFilter *[]string `json:"eventFilter"`
	Enabled     *bool     `json:"enabled"`
}

// updateEndpointRequest is PATCH /v1/webhooks/{id}'s body — Render's full-body
// PATCH: any combination of name/url/enabled/eventFilter may be supplied;
// omitted fields keep their current values.
type updateEndpointRequest struct {
	Name        *string   `json:"name"`
	URL         *string   `json:"url"`
	Enabled     *bool     `json:"enabled"`
	EventFilter *[]string `json:"eventFilter"`
}

// RegisterREST mounts Render's webhook CRUD + delivery-history route family.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	listHandler := func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		cursor, limit := core.PageParams(q)
		ownerIDs := append(core.QueryList(q, "ownerId"), core.QueryList(q, "ownerId[]")...)
		views, err := s.ListPage(r.Context(), ownerIDs, cursor, limit)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toWirePage(views))
	}
	createHandler := func(w http.ResponseWriter, r *http.Request) {
		var req createEndpointRequest
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		if req.Enabled == nil {
			core.WriteErr(w, core.NewBadRequestError("WEBHOOK_ENABLED_REQUIRED", "enabled is required", map[string]any{"field": "enabled"}))
			return
		}
		if req.OwnerID == nil || *req.OwnerID == "" {
			core.WriteErr(w, core.NewBadRequestError("WEBHOOK_OWNER_REQUIRED", "ownerId is required", map[string]any{"field": "ownerId"}))
			return
		}
		if req.EventFilter == nil {
			core.WriteErr(w, core.NewBadRequestError("WEBHOOK_EVENT_FILTER_REQUIRED", "eventFilter is required", map[string]any{"field": "eventFilter"}))
			return
		}
		v, err := s.Create(r.Context(), CreateRequest{
			OwnerID:    *req.OwnerID,
			Name:       req.Name,
			URL:        req.URL,
			EventTypes: *req.EventFilter,
			Enabled:    *req.Enabled,
		})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, toWire(v))
	}
	getHandler := func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Get(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toWire(v))
	}
	deleteHandler := func(w http.ResponseWriter, r *http.Request) {
		if err := s.Delete(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
	deliveriesHandler := func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		cursor, limit := core.PageParams(q)
		window, err := core.QueryTimeWindow(q, "sentBefore", "sentAfter")
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		views, err := s.ListDeliveriesFiltered(r.Context(), q.Get("ownerId"), r.PathValue("id"), DeliveryFilter{
			Cursor: cursor, Limit: limit, SentAfter: window.After, SentBefore: window.Before,
			Status: q.Get("status"),
		})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toWebhookEventList(views))
	}
	resendHandler := func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Resend(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id"),
			r.PathValue("attemptId"), r.Header.Get("Idempotency-Key"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusAccepted, v)
	}

	// Render's public route family (w3/m27).
	mux.HandleFunc("GET /v1/webhooks", listHandler)
	mux.HandleFunc("POST /v1/webhooks", createHandler)
	mux.HandleFunc("GET /v1/webhooks/{id}", getHandler)
	mux.HandleFunc("PATCH /v1/webhooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req updateEndpointRequest
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		v, err := s.Update(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id"), UpdateRequest{
			Name:       req.Name,
			URL:        req.URL,
			Enabled:    req.Enabled,
			EventTypes: req.EventFilter,
		})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toWire(v))
	})
	mux.HandleFunc("DELETE /v1/webhooks/{id}", deleteHandler)
	// /events is Render's name for the delivery-history read.
	mux.HandleFunc("GET /v1/webhooks/{id}/events", deliveriesHandler)
	// Render exposes this action only in its dashboard. The authenticated route
	// is a labeled bex extension over the same core attempt semantics.
	mux.HandleFunc("POST /v1/webhooks/{id}/events/{attemptId}/resend", resendHandler)

	// The subscribable vocabulary — what the dashboard's event-type picker
	// lists, served rather than duplicated client-side.
	mux.HandleFunc("GET /v1/webhooks/event-types", func(w http.ResponseWriter, r *http.Request) {
		core.WriteJSON(w, http.StatusOK, EventTypes)
	})
}
