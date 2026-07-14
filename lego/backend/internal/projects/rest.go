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

package projects

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the projects REST fragment (bex extension matching Render's project
// API shape: GET/POST /v1/projects, GET/PATCH/DELETE /v1/projects/{id},
// PUT /v1/projects/{id}/service-links).

// RegisterREST mounts the project CRUD endpoints.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/projects", func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.URL.Query().Get("ownerId")
		if ownerID == "" {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		ps, err := s.List(r.Context(), ownerID)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, ps)
	})

	mux.HandleFunc("POST /v1/projects", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name    string `json:"name"`
			OwnerID string `json:"ownerId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" || req.OwnerID == "" {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		p, err := s.Create(r.Context(), req.OwnerID, strings.TrimSpace(req.Name))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, p)
	})

	mux.HandleFunc("GET /v1/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		p, err := s.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, p)
	})

	mux.HandleFunc("PATCH /v1/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		p, err := s.Rename(r.Context(), r.PathValue("id"), strings.TrimSpace(req.Name))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, p)
	})

	mux.HandleFunc("DELETE /v1/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Delete(r.Context(), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// PUT /v1/projects/{id}/service-links replaces the full list of services in a
	// project. Body: {"serviceIds": ["name1", "name2"]} where serviceIds are App
	// CR names (same as the GraphQL / REST service id field).
	mux.HandleFunc("PUT /v1/projects/{id}/service-links", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ServiceIDs []string `json:"serviceIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		if req.ServiceIDs == nil {
			req.ServiceIDs = []string{}
		}
		p, err := s.SetServices(r.Context(), r.PathValue("id"), req.ServiceIDs)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, p)
	})

	// PUT /v1/projects/{id}/database-links replaces the full list of managed
	// Postgres databases in a project. Body: {"databaseIds": ["name1", "name2"]}
	// where databaseIds are Database CR names (w1/m31 extension).
	mux.HandleFunc("PUT /v1/projects/{id}/database-links", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			DatabaseIDs []string `json:"databaseIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		if req.DatabaseIDs == nil {
			req.DatabaseIDs = []string{}
		}
		p, err := s.SetDatabases(r.Context(), r.PathValue("id"), req.DatabaseIDs)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, p)
	})

	// PUT /v1/projects/{id}/keyvalue-links is database-links' KeyValue-CR
	// counterpart. Body: {"keyValueIds": ["name1", "name2"]}.
	mux.HandleFunc("PUT /v1/projects/{id}/keyvalue-links", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			KeyValueIDs []string `json:"keyValueIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		if req.KeyValueIDs == nil {
			req.KeyValueIDs = []string{}
		}
		p, err := s.SetKeyValues(r.Context(), r.PathValue("id"), req.KeyValueIDs)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, p)
	})
}
