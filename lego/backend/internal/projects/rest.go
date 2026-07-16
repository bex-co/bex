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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// rest.go is the projects REST fragment (bex extension matching Render's project
// API shape: GET/POST /v1/projects, GET/PATCH/DELETE /v1/projects/{id},
// PUT /v1/projects/{id}/service-links).

// writeErr maps the projects feature's package-local unavailable sentinel to
// the same 503 response as the unavailable sentinels declared in core. Core is
// a leaf and cannot import this package, so core.WriteErr cannot recognize
// ErrProjectsUnavailable directly.
func writeErr(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrProjectsUnavailable) {
		core.WriteErrStatus(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	core.WriteErr(w, err)
}

type renderOwner struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Type  string `json:"type"`
}

type renderProject struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Owner          renderOwner `json:"owner"`
	EnvironmentIDs []string    `json:"environmentIds"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}

type renderProjectWithCursor struct {
	Project renderProject `json:"project"`
	Cursor  string        `json:"cursor"`
}

type projectEnvironmentLister interface {
	ListEnvironments(ctx context.Context, projectID string) ([]store.Environment, error)
}

func toRenderProject(p ProjectView, environmentIDs []string) renderProject {
	if environmentIDs == nil {
		environmentIDs = []string{}
	}
	return renderProject{
		ID:             p.ID,
		Name:           p.Name,
		Owner:          renderOwner{ID: p.OwnerID, Type: "team"},
		EnvironmentIDs: environmentIDs,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.CreatedAt,
	}
}

func (s *Service) renderProject(ctx context.Context, p ProjectView) (renderProject, error) {
	var ids []string
	if lister, ok := s.Store.(projectEnvironmentLister); ok {
		environments, err := lister.ListEnvironments(ctx, p.ID)
		if err != nil {
			return renderProject{}, err
		}
		ids = make([]string, len(environments))
		for i := range environments {
			ids[i] = environments[i].ID
		}
	}
	return toRenderProject(p, ids), nil
}

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
			writeErr(w, err)
			return
		}
		out := make([]renderProjectWithCursor, 0, len(ps))
		for _, p := range ps {
			rendered, err := s.renderProject(r.Context(), p)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			out = append(out, renderProjectWithCursor{Project: rendered, Cursor: p.ID})
		}
		core.WriteJSON(w, http.StatusOK, out)
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
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, p)
	})

	mux.HandleFunc("GET /v1/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		p, err := s.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			writeErr(w, err)
			return
		}
		rendered, err := s.renderProject(r.Context(), p)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, rendered)
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
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, p)
	})

	mux.HandleFunc("DELETE /v1/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Delete(r.Context(), r.PathValue("id")); err != nil {
			writeErr(w, err)
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
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, p)
	})

	// PUT /v1/projects/{id}/database-links replaces the full list of managed
	// Postgres databases in a project. Body: {"databaseIds": ["dpg-..."]}, where
	// databaseIds are immutable Database CR names (w1/m31 extension).
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
			writeErr(w, err)
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
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, p)
	})
}
