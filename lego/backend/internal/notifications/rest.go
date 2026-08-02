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

package notifications

import (
	"net/http"
	"strconv"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the notifications REST fragment (bex extension — Render exposes
// notification settings only via its dashboard GraphQL). GET/PATCH
// /v1/notification-settings read/write the CALLER's own preferences, the same
// self-service shape GET /v1/usage uses (no path param — always "me").

// RegisterREST mounts the notification-settings endpoints on the shared mux.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/notifications", func(w http.ResponseWriter, r *http.Request) {
		limit := defaultNotificationInboxLimit
		if r.URL.Query().Has("limit") {
			var err error
			limit, err = strconv.Atoi(r.URL.Query().Get("limit"))
			if err != nil || limit < 1 || limit > maxNotificationInboxLimit {
				core.WriteErr(w, core.ErrBadRequest)
				return
			}
		}
		items, err := s.ListNotificationInbox(r.Context(), limit)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, items)
	})
	mux.HandleFunc("POST /v1/notifications/{id}/read", func(w http.ResponseWriter, r *http.Request) {
		read, err := s.MarkPushNotificationRead(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, map[string]bool{"read": read})
	})
	mux.HandleFunc("GET /v1/notification-settings", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.GetSettings(r.Context())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("PATCH /v1/notification-settings", func(w http.ResponseWriter, r *http.Request) {
		var req SettingsView
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		v, err := s.UpdateSettings(r.Context(), req.DeployStarted, req.DeploySucceeded, req.DeployFailed)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("GET /v1/notification-settings/push", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.GetPushSettings(r.Context())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("GET /v1/notification-settings/push/availability", func(w http.ResponseWriter, r *http.Request) {
		available, err := s.IsPushAvailable(r.Context())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, map[string]bool{"available": available})
	})
	mux.HandleFunc("PATCH /v1/notification-settings/push", func(w http.ResponseWriter, r *http.Request) {
		var req PushSettingsView
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		v, err := s.UpdatePushSettings(r.Context(), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("GET /v1/notification-device-subscriptions", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.ListDeviceSubscriptions(r.Context())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("POST /v1/notification-device-subscriptions", func(w http.ResponseWriter, r *http.Request) {
		var req RegisterDeviceInput
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		v, err := s.RegisterDeviceSubscription(r.Context(), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, v)
	})
	mux.HandleFunc("DELETE /v1/notification-device-subscriptions", func(w http.ResponseWriter, r *http.Request) {
		count, err := s.RevokeDeviceSubscriptions(r.Context())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, map[string]int64{"revoked": count})
	})
	mux.HandleFunc("DELETE /v1/notification-device-subscriptions/{deviceId}", func(w http.ResponseWriter, r *http.Request) {
		changed, err := s.UnregisterDeviceSubscription(r.Context(), r.PathValue("deviceId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, map[string]bool{"revoked": changed})
	})
}
