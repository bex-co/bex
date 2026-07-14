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

package environments

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the environments REST fragment, mirroring internal/projects/
// rest.go's shape exactly: GET/POST /v1/environments, GET/PATCH/DELETE
// /v1/environments/{id}, PUT /v1/environments/{id}/service-links. Scoped by
// projectId (the project's own workspace scoping already gates authorization,
// so no separate ownerId param is needed here).

// writeErr maps ErrEnvironmentsUnavailable to 503 before falling back to
// core.WriteErr: that shared mapper only recognizes error sentinels declared
// in package core (a leaf core cannot import a feature package to add this
// one directly, matching every other "…Unavailable" convention in the
// codebase — e.g. core.ErrWorkspacesUnavailable, core.ErrEventsUnavailable),
// so a package-local sentinel like this one would otherwise fall through to
// core.WriteErr's default 500.
func writeErr(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrEnvironmentsUnavailable) {
		core.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	core.WriteErr(w, err)
}

// RegisterREST mounts the environment CRUD endpoints.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/environments", func(w http.ResponseWriter, r *http.Request) {
		projectID := r.URL.Query().Get("projectId")
		if projectID == "" {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		es, err := s.List(r.Context(), projectID)
		if err != nil {
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, es)
	})

	mux.HandleFunc("POST /v1/environments", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name      string `json:"name"`
			ProjectID string `json:"projectId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" || req.ProjectID == "" {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		e, err := s.Create(r.Context(), req.ProjectID, strings.TrimSpace(req.Name))
		if err != nil {
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, e)
	})

	mux.HandleFunc("GET /v1/environments/{id}", func(w http.ResponseWriter, r *http.Request) {
		e, err := s.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, e)
	})

	mux.HandleFunc("PATCH /v1/environments/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		e, err := s.Rename(r.Context(), r.PathValue("id"), strings.TrimSpace(req.Name))
		if err != nil {
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, e)
	})

	mux.HandleFunc("DELETE /v1/environments/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Delete(r.Context(), r.PathValue("id")); err != nil {
			writeErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// PUT /v1/environments/{id}/service-links replaces the full list of
	// services in an environment. Body: {"serviceIds": ["name1", "name2"]}
	// where serviceIds are App CR names (same as projects' service-links).
	mux.HandleFunc("PUT /v1/environments/{id}/service-links", func(w http.ResponseWriter, r *http.Request) {
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
		e, err := s.SetServices(r.Context(), r.PathValue("id"), req.ServiceIDs)
		if err != nil {
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, e)
	})

	// PUT /v1/environments/{id}/database-links replaces the full list of
	// managed Postgres databases in an environment. Body:
	// {"databaseIds": ["name1", "name2"]} where databaseIds are Database CR
	// names (w6/m20 extension, mirroring projects' database-links).
	mux.HandleFunc("PUT /v1/environments/{id}/database-links", func(w http.ResponseWriter, r *http.Request) {
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
		e, err := s.SetDatabases(r.Context(), r.PathValue("id"), req.DatabaseIDs)
		if err != nil {
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, e)
	})

	// PUT /v1/environments/{id}/keyvalue-links is database-links' KeyValue-CR
	// counterpart. Body: {"keyValueIds": ["name1", "name2"]}.
	mux.HandleFunc("PUT /v1/environments/{id}/keyvalue-links", func(w http.ResponseWriter, r *http.Request) {
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
		e, err := s.SetKeyValues(r.Context(), r.PathValue("id"), req.KeyValueIDs)
		if err != nil {
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, e)
	})
}
