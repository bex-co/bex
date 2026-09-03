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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the projects REST fragment (bex extension matching Render's project
// API shape: GET/POST /v1/projects, GET/PATCH/DELETE /v1/projects/{id},
// PUT /v1/projects/{id}/service-links).

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
	ids, err := s.environmentIDs(ctx, p.ID)
	if err != nil {
		return renderProject{}, err
	}
	return toRenderProject(p, ids), nil
}

// RegisterREST mounts the project CRUD endpoints.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	identity := func(p ProjectView) ProjectView { return p }

	mux.HandleFunc("GET /v1/projects", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		ownerID := r.URL.Query().Get("ownerId")
		if ownerID == "" {
			// Render marks ownerId optional here; bex requires it (the w6/m126
			// parity rework owns that divergence). Name the param so the caller
			// gets the same actionable 400 the validator-gated siblings emit,
			// rather than a bare "bad request" (w4/038).
			return nil, fmt.Errorf("%w: invalid query parameter %q", core.ErrBadRequest, "ownerId")
		}
		ps, err := s.List(r.Context(), ownerID)
		if err != nil {
			return nil, err
		}
		after, limit := core.PageParams(r.URL.Query())
		ps = core.StablePage(ps, after, limit, true, func(p ProjectView) string { return p.ID })
		out := make([]renderProjectWithCursor, 0, len(ps))
		for _, p := range ps {
			rendered, err := s.renderProject(r.Context(), p)
			if err != nil {
				return nil, err
			}
			out = append(out, renderProjectWithCursor{Project: rendered, Cursor: p.ID})
		}
		return out, nil
	}))

	mux.HandleFunc("POST /v1/projects", core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[struct {
			Name    string `json:"name"`
			OwnerID string `json:"ownerId"`
		}](r)
		if err != nil || strings.TrimSpace(req.Name) == "" || req.OwnerID == "" {
			return nil, core.ErrBadRequest
		}
		return s.Create(r.Context(), req.OwnerID, strings.TrimSpace(req.Name))
	}))

	mux.HandleFunc("GET /v1/projects/{id}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		p, err := s.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		return s.renderProject(r.Context(), p)
	}))

	mux.HandleFunc("PATCH /v1/projects/{id}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[struct {
			Name string `json:"name"`
		}](r)
		if err != nil || strings.TrimSpace(req.Name) == "" {
			return nil, core.ErrBadRequest
		}
		return s.Rename(r.Context(), r.PathValue("id"), strings.TrimSpace(req.Name))
	}))

	mux.HandleFunc("DELETE /v1/projects/{id}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.Delete(r.Context(), r.PathValue("id"))
	}))

	// The three link routes replace a project's full membership list for one
	// resource kind; an absent array clears it. Legacy service names remain
	// accepted on service-links during the stable-id transition, and
	// databaseIds/keyValueIds are immutable CR names (w1/m31 extension).
	mux.HandleFunc("PUT /v1/projects/{id}/service-links", core.HandleLinks[core.ServiceLinks](s.SetServices, identity))
	mux.HandleFunc("PUT /v1/projects/{id}/database-links", core.HandleLinks[core.DatabaseLinks](s.SetDatabases, identity))
	mux.HandleFunc("PUT /v1/projects/{id}/keyvalue-links", core.HandleLinks[core.KeyValueLinks](s.SetKeyValues, identity))
}
