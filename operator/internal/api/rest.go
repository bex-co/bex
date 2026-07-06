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

	s.registerPostgresRoutes(mux)
	s.registerLogRoutes(mux)
	s.registerMetricRoutes(mux)
	s.registerAPIKeyRoutes(mux)
	return mux
}

// registerAPIKeyRoutes adds the machine-credential endpoints (bex extension —
// Render manages API keys only via its dashboard; naming follows Render's
// kebab-case noun style). The secret appears once, in the create response.
func (s *Server) registerAPIKeyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, ErrBadRequest)
			return
		}
		key, err := s.Core.CreateAPIKey(r.Context(), req.Name)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, key)
	})
	mux.HandleFunc("GET /v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		keys, err := s.Core.ListAPIKeys(r.Context())
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, keys)
	})
	mux.HandleFunc("DELETE /v1/api-keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Core.RevokeAPIKey(r.Context(), r.PathValue("id")); err != nil {
			writeErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// registerPostgresRoutes adds the managed-Postgres endpoints, Render-shaped
// (/v1/postgres) plus a bex-native /v1/databases alias.
func (s *Server) registerPostgresRoutes(mux *http.ServeMux) {
	for _, base := range []string{"/v1/postgres", "/v1/databases"} {
		mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
			out, err := s.Core.ListPostgres(r.Context())
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
			var req CreatePostgresRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
				return
			}
			pg, err := s.Core.CreatePostgres(r.Context(), req)
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, pg) // Render: create => 201
		})
		mux.HandleFunc("GET "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
			pg, err := s.Core.GetPostgres(r.Context(), r.PathValue("id"))
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, pg)
		})
		mux.HandleFunc("DELETE "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
			if err := s.Core.DeletePostgres(r.Context(), r.PathValue("id")); err != nil {
				writeErr(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent) // Render: delete => 204
		})
		mux.HandleFunc("GET "+base+"/{id}/connection-info", func(w http.ResponseWriter, r *http.Request) {
			info, err := s.Core.PostgresConnectionInfo(r.Context(), r.PathValue("id"))
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, info)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, ErrLogsUnavailable), errors.Is(err, ErrAPIKeysUnavailable):
		code = http.StatusServiceUnavailable
	case errors.Is(err, ErrMetricsUnavailable):
		code = http.StatusServiceUnavailable
	case errors.Is(err, ErrBadRequest):
		code = http.StatusBadRequest
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
