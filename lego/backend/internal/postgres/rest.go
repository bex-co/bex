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

package postgres

import (
	"encoding/json"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RegisterREST adds the managed-Postgres endpoints, Render-shaped (/v1/postgres)
// plus a bex-native /v1/databases alias.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	for _, base := range []string{"/v1/postgres", "/v1/databases"} {
		mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
			out, err := s.ListPostgres(r.Context(), r.URL.Query().Get("ownerId"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
			var req CreatePostgresRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
				return
			}
			pg, err := s.CreatePostgres(r.Context(), req)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusCreated, pg) // Render: create => 201
		})
		mux.HandleFunc("GET "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
			pg, err := s.GetPostgres(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, pg)
		})
		mux.HandleFunc("DELETE "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
			if err := s.DeletePostgres(r.Context(), r.PathValue("id")); err != nil {
				core.WriteErr(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent) // Render: delete => 204
		})
		mux.HandleFunc("GET "+base+"/{id}/connection-info", func(w http.ResponseWriter, r *http.Request) {
			info, err := s.PostgresConnectionInfo(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, info)
		})
	}
}
