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
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	URL            string   `json:"url"`
	OwnerID        string   `json:"ownerId"`
	EventFilter    []string `json:"eventFilter"`
	Enabled        bool     `json:"enabled"`
	DisabledReason string   `json:"disabledReason,omitempty"`
	Secret         string   `json:"secret,omitempty"`
	CreatedBy      string   `json:"createdBy,omitempty"`
	CreatedAt      string   `json:"createdAt,omitempty"`
	UpdatedAt      string   `json:"updatedAt,omitempty"`
}

func toWire(v EndpointView) endpointWire {
	return endpointWire{
		ID: v.ID, Name: v.Name, URL: v.URL, OwnerID: v.OwnerID,
		EventFilter: v.EventTypes,
		Enabled:     v.Enabled, DisabledReason: v.DisabledReason, Secret: v.Secret,
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

// createEndpointRequest is POST /v1/webhooks' canonical Render body.
type createEndpointRequest struct {
	OwnerID     string   `json:"ownerId"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	EventFilter []string `json:"eventFilter"`
}

// updateEndpointRequest is PATCH /v1/webhooks/{id}'s body — Render's full-body
// PATCH: any combination of name/url/enabled/eventFilter may be supplied;
// omitted fields keep their current values.
type updateEndpointRequest struct {
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Enabled     *bool    `json:"enabled"`
	EventFilter []string `json:"eventFilter"`
}

// RegisterREST mounts Render's webhook CRUD + delivery-history route family.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	listHandler := func(w http.ResponseWriter, r *http.Request) {
		views, err := s.List(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toWireList(views))
	}
	createHandler := func(w http.ResponseWriter, r *http.Request) {
		var req createEndpointRequest
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		v, err := s.Create(r.Context(), CreateRequest{
			OwnerID:    req.OwnerID,
			Name:       req.Name,
			URL:        req.URL,
			EventTypes: req.EventFilter,
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
		cursor, limit := core.PageParams(r.URL.Query())
		views, err := s.ListDeliveries(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id"), cursor, limit)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toDeliveryList(views))
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

	// The subscribable vocabulary — what the dashboard's event-type picker
	// lists, served rather than duplicated client-side.
	mux.HandleFunc("GET /v1/webhooks/event-types", func(w http.ResponseWriter, r *http.Request) {
		core.WriteJSON(w, http.StatusOK, EventTypes)
	})
}
