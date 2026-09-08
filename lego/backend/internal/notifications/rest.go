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
	mux.HandleFunc("GET /v1/notifications", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		limit, err := inboxLimit(r)
		if err != nil {
			return nil, err
		}
		return s.ListNotificationInbox(core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId")), limit)
	}))
	mux.HandleFunc("POST /v1/notifications/{id}/read", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		read, err := s.MarkPushNotificationRead(core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId")), r.PathValue("id"))
		return map[string]bool{"read": read}, err
	}))
	mux.HandleFunc("GET /v1/notification-settings", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.GetSettings(r.Context())
	}))
	mux.HandleFunc("PATCH /v1/notification-settings", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[SettingsView](r)
		if err != nil {
			return nil, err
		}
		return s.UpdateSettings(r.Context(), req.DeployStarted, req.DeploySucceeded, req.DeployFailed)
	}))
	mux.HandleFunc("GET /v1/notification-settings/push", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.GetPushSettings(r.Context())
	}))
	mux.HandleFunc("GET /v1/notification-settings/push/availability", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		available, err := s.IsPushAvailable(core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId")))
		if err != nil {
			return nil, err
		}
		webPush, err := s.IsWebPushAvailable(core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId")))
		if err != nil {
			return nil, err
		}
		key, err := s.WebPushVAPIDPublicKey(core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId")))
		if err != nil {
			return nil, err
		}
		out := map[string]any{"available": available, "webPushAvailable": webPush}
		if key != "" {
			out["vapidPublicKey"] = key
		}
		return out, nil
	}))
	mux.HandleFunc("PATCH /v1/notification-settings/push", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[PushSettingsView](r)
		if err != nil {
			return nil, err
		}
		return s.UpdatePushSettings(r.Context(), req)
	}))
	mux.HandleFunc("GET /v1/notification-device-subscriptions", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.ListDeviceSubscriptions(core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId")))
	}))
	mux.HandleFunc("POST /v1/notification-device-subscriptions", core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[RegisterDeviceInput](r)
		if err != nil {
			return nil, err
		}
		return s.RegisterDeviceSubscription(core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId")), req)
	}))
	mux.HandleFunc("DELETE /v1/notification-device-subscriptions", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		count, err := s.RevokeDeviceSubscriptions(core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId")))
		return map[string]int64{"revoked": count}, err
	}))
	mux.HandleFunc("DELETE /v1/notification-device-subscriptions/{deviceId}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		changed, err := s.UnregisterDeviceSubscription(core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId")), r.PathValue("deviceId"))
		return map[string]bool{"revoked": changed}, err
	}))
	mux.HandleFunc("GET /v1/notification-webpush-subscriptions", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.ListWebPushSubscriptions(r.Context())
	}))
	mux.HandleFunc("POST /v1/notification-webpush-subscriptions", core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[RegisterWebPushInput](r)
		if err != nil {
			return nil, err
		}
		return s.RegisterWebPushSubscription(r.Context(), req)
	}))
	mux.HandleFunc("DELETE /v1/notification-webpush-subscriptions/{browserId}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		changed, err := s.UnregisterWebPushSubscription(r.Context(), r.PathValue("browserId"))
		return map[string]bool{"revoked": changed}, err
	}))
}

// inboxLimit reads the optional inbox page size, rejecting anything outside the
// supported range rather than silently clamping.
func inboxLimit(r *http.Request) (int, error) {
	if !r.URL.Query().Has("limit") {
		return defaultNotificationInboxLimit, nil
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > maxNotificationInboxLimit {
		return 0, core.ErrBadRequest
	}
	return limit, nil
}
