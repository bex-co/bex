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

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the notifications REST fragment (bex extension — Render exposes
// notification settings only via its dashboard GraphQL). GET/PATCH
// /v1/notification-settings read/write the CALLER's own preferences, the same
// self-service shape GET /v1/usage uses (no path param — always "me").

// RegisterREST mounts the notification-settings endpoints on the shared mux.
func (s *Service) RegisterREST(mux *http.ServeMux) {
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
}
