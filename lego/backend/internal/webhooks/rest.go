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
	"encoding/json"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the REST fragment, under /v1/webhooks/endpoints — a bex-chosen
// noun: Render's public docs manage outbound webhooks from the dashboard only
// (checked live 2026-07-12, "see the API reference" without detail), so this
// is bex filling the gap in its own API-first style (the api-keys precedent),
// and /v1/webhooks alone already belongs to the inbound git-push webhook.

// endpointWire is the wire shape every surface renders. Secret is present on
// the create response ONLY — the mint-once read; every other render omits it
// (empty + omitempty, and no read query ever selects the column).
type endpointWire struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	URL            string   `json:"url"`
	OwnerID        string   `json:"ownerId"`
	EventTypes     []string `json:"eventTypes"`
	Enabled        bool     `json:"enabled"`
	DisabledReason string   `json:"disabledReason,omitempty"`
	Secret         string   `json:"secret,omitempty"`
	CreatedBy      string   `json:"createdBy,omitempty"`
	CreatedAt      string   `json:"createdAt,omitempty"`
	UpdatedAt      string   `json:"updatedAt,omitempty"`
}

func toWire(v EndpointView) endpointWire {
	return endpointWire{
		ID: v.ID, Name: v.Name, URL: v.URL, OwnerID: v.OwnerID, EventTypes: v.EventTypes,
		Enabled: v.Enabled, DisabledReason: v.DisabledReason, Secret: v.Secret,
		CreatedBy: v.CreatedBy, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
}

func toWireList(views []EndpointView) []endpointWire {
	out := make([]endpointWire, 0, len(views))
	for _, v := range views {
		out = append(out, toWire(v))
	}
	return out
}

// deliveryWire is one delivery-history entry; deliveryWithCursor is the
// Render list envelope (the deploys/events {item, cursor} sibling shape).
type deliveryWire struct {
	ID              string `json:"id"`
	EventID         string `json:"eventId"`
	EventType       string `json:"eventType"`
	ServiceID       string `json:"serviceId,omitempty"`
	Status          string `json:"status"`
	AttemptCount    int    `json:"attemptCount"`
	LastStatusCode  int    `json:"lastStatusCode,omitempty"`
	LastError       string `json:"lastError,omitempty"`
	NextAttemptAt   string `json:"nextAttemptAt,omitempty"`
	LastAttemptedAt string `json:"lastAttemptedAt,omitempty"`
	DeliveredAt     string `json:"deliveredAt,omitempty"`
	CreatedAt       string `json:"createdAt"`
}

type deliveryWithCursor struct {
	Delivery deliveryWire `json:"delivery"`
	Cursor   string       `json:"cursor"`
}

func toDeliveryWire(v DeliveryView) deliveryWire {
	return deliveryWire{
		ID: v.ID, EventID: v.EventID, EventType: v.EventType, ServiceID: v.ServiceID,
		Status: v.Status, AttemptCount: v.AttemptCount, LastStatusCode: v.LastStatusCode,
		LastError: v.LastError, NextAttemptAt: v.NextAttemptAt,
		LastAttemptedAt: v.LastAttemptedAt, DeliveredAt: v.DeliveredAt, CreatedAt: v.CreatedAt,
	}
}

func toDeliveryList(views []DeliveryView) []deliveryWithCursor {
	out := make([]deliveryWithCursor, 0, len(views))
	for _, v := range views {
		out = append(out, deliveryWithCursor{Delivery: toDeliveryWire(v), Cursor: v.Cursor})
	}
	return out
}

// createEndpointRequest is POST /v1/webhooks/endpoints' body.
type createEndpointRequest struct {
	OwnerID    string   `json:"ownerId"`
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	EventTypes []string `json:"eventTypes"`
}

// patchEndpointRequest is PATCH /v1/webhooks/endpoints/{id}'s body — enabled
// only (URL/event edits are delete + recreate for v1, several resources'
// own PATCH omission).
type patchEndpointRequest struct {
	Enabled *bool `json:"enabled"`
}

// RegisterREST mounts the webhook-endpoint CRUD + delivery-history surface.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/webhooks/endpoints", func(w http.ResponseWriter, r *http.Request) {
		views, err := s.List(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toWireList(views))
	})

	mux.HandleFunc("POST /v1/webhooks/endpoints", func(w http.ResponseWriter, r *http.Request) {
		var req createEndpointRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		v, err := s.Create(r.Context(), CreateRequest{
			OwnerID: req.OwnerID, Name: req.Name, URL: req.URL, EventTypes: req.EventTypes,
		})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, toWire(v))
	})

	// The subscribable vocabulary — what the dashboard's event-type picker
	// lists, served rather than duplicated client-side.
	mux.HandleFunc("GET /v1/webhooks/event-types", func(w http.ResponseWriter, r *http.Request) {
		core.WriteJSON(w, http.StatusOK, EventTypes)
	})

	mux.HandleFunc("GET /v1/webhooks/endpoints/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Get(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toWire(v))
	})

	mux.HandleFunc("PATCH /v1/webhooks/endpoints/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req patchEndpointRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		v, err := s.SetEnabled(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id"), *req.Enabled)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toWire(v))
	})

	mux.HandleFunc("DELETE /v1/webhooks/endpoints/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Delete(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /v1/webhooks/endpoints/{id}/deliveries", func(w http.ResponseWriter, r *http.Request) {
		cursor, limit := core.PageParams(r.URL.Query())
		views, err := s.ListDeliveries(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id"), cursor, limit)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toDeliveryList(views))
	})
}
