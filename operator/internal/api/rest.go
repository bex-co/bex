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

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// restHandler is the REST adapter — Render-public-API compatible. Paths, the
// {service, cursor} list envelope, the string suspended enum, and the verb
// status codes (suspend/resume 202, restart 200) all match Render's OpenAPI
// spec so a Render-shaped client works. Served at both /v1/services (Render's
// noun) and /v1/apps (bex's noun) over the same handlers. It holds no logic
// beyond routing + Render serialization; behavior lives in Core.
func (s *Server) restHandler() http.Handler {
	mux := http.NewServeMux()

	list := func(w http.ResponseWriter, r *http.Request) {
		apps, err := s.Core.List(r.Context())
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toServiceList(apps)) // [{service, cursor}, ...]
	}
	get := func(w http.ResponseWriter, r *http.Request) {
		app, err := s.Core.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toRenderService(app))
	}
	// verb maps a Core action to a handler with a Render-accurate status code.
	verb := func(status int, fn func(context.Context, string) (AppView, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			app, err := fn(r.Context(), r.PathValue("id"))
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, status, toRenderService(app))
		}
	}

	// Register the same handlers under Render's /v1/services and bex's /v1/apps.
	for _, base := range []string{"/v1/services", "/v1/apps"} {
		mux.HandleFunc("GET "+base, list)
		mux.HandleFunc("GET "+base+"/{id}", get)
		mux.HandleFunc("POST "+base+"/{id}/suspend", verb(http.StatusAccepted, s.Core.Suspend))
		mux.HandleFunc("POST "+base+"/{id}/resume", verb(http.StatusAccepted, s.Core.Resume))
		mux.HandleFunc("POST "+base+"/{id}/restart", verb(http.StatusOK, s.Core.Restart)) // Render: restart => 200
	}

	return mux
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	if errors.Is(err, ErrNotFound) {
		code = http.StatusNotFound
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
