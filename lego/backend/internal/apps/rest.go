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

package apps

import (
	"context"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RegisterREST mounts the App-lifecycle routes — Render-public-API compatible.
// Paths, the {service, cursor} list envelope, the string suspended enum, and the
// verb status codes (suspend/resume 202, restart 200) all match Render's OpenAPI
// spec. Served at both /v1/services (Render's noun) and /v1/apps (bex's noun);
// it holds no logic beyond routing + Render serialization.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	list := func(w http.ResponseWriter, r *http.Request) {
		apps, err := s.List(r.Context())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toServiceList(apps)) // [{service, cursor}, ...]
	}
	get := func(w http.ResponseWriter, r *http.Request) {
		app, err := s.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderService(app))
	}
	// verb maps a Service action to a handler with a Render-accurate status code.
	verb := func(status int, fn func(context.Context, string) (AppView, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			app, err := fn(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, status, toRenderService(app))
		}
	}

	// Register the same handlers under Render's /v1/services and bex's /v1/apps.
	for _, base := range []string{"/v1/services", "/v1/apps"} {
		mux.HandleFunc("GET "+base, list)
		mux.HandleFunc("GET "+base+"/{id}", get)
		mux.HandleFunc("POST "+base+"/{id}/suspend", verb(http.StatusAccepted, s.Suspend))
		mux.HandleFunc("POST "+base+"/{id}/resume", verb(http.StatusAccepted, s.Resume))
		mux.HandleFunc("POST "+base+"/{id}/restart", verb(http.StatusOK, s.Restart)) // Render: restart => 200
	}
}
