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

// projectEnvironmentInput mirrors Render's projectPOSTEnvironmentInput so the
// strict Render decoder accepts a full create body (w6/m126 t002). bex does not
// yet provision these — see the POST handler for why the field is accepted but
// not honored.
type projectEnvironmentInput struct {
	Name                    string                  `json:"name"`
	ProtectedStatus         string                  `json:"protectedStatus"`
	NetworkIsolationEnabled bool                    `json:"networkIsolationEnabled"`
	IPAllowList             []core.IPAllowListEntry `json:"ipAllowList"`
}

// RegisterREST mounts the project CRUD endpoints. Every handler that returns a
// project funnels its result through renderProject so the wire shape is Render's
// `project` object (id, name, owner, environmentIds, createdAt, updatedAt) — one
// shape per resource, whichever verb produced it (w6/m126). The read paths
// always did this; the five write paths (POST, PATCH, and the three link PUTs)
// used to emit the internal ProjectView instead, which is the defect this
// milestone closes.
func (s *Service) RegisterREST(mux *http.ServeMux) {
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
			// Render's projectPOSTInput requires an `environments` array and
			// creates those environments with the project. bex records the
			// divergence (w6/m126 t002, docs/ADR018-render-parity.md §Projects):
			// a project is created on its own and its environments are created
			// separately via POST /v1/environments. The field is accepted — not
			// rejected as an unknown field by the strict Render decoder — so a
			// client that mints its types from Render's schema (the official CLI,
			// DO_NOT_DO #31) is not 400'd. It is not honored, and the response's
			// environmentIds truthfully reports the project's real (initially
			// empty) environment set rather than faking the requested ones.
			Environments []projectEnvironmentInput `json:"environments"`
		}](r)
		if err != nil || strings.TrimSpace(req.Name) == "" || req.OwnerID == "" {
			return nil, core.ErrBadRequest
		}
		p, err := s.Create(r.Context(), req.OwnerID, strings.TrimSpace(req.Name))
		if err != nil {
			return nil, err
		}
		return s.renderProject(r.Context(), p)
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
		p, err := s.Rename(r.Context(), r.PathValue("id"), strings.TrimSpace(req.Name))
		if err != nil {
			return nil, err
		}
		return s.renderProject(r.Context(), p)
	}))

	mux.HandleFunc("DELETE /v1/projects/{id}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.Delete(r.Context(), r.PathValue("id"))
	}))

	// The three link routes replace a project's full membership list for one
	// resource kind; an absent array clears it. Legacy service names remain
	// accepted on service-links during the stable-id transition, and
	// databaseIds/keyValueIds are immutable CR names (w1/m31 extension). Each
	// projects its result through renderProject (via HandleLinksMapped, whose
	// ctx+error view the plain HandleLinks cannot express) so the reply is the
	// same Render shape a read returns (w6/m126).
	mux.HandleFunc("PUT /v1/projects/{id}/service-links", core.HandleLinksMapped[core.ServiceLinks](s.SetServices, s.renderProject))
	mux.HandleFunc("PUT /v1/projects/{id}/database-links", core.HandleLinksMapped[core.DatabaseLinks](s.SetDatabases, s.renderProject))
	mux.HandleFunc("PUT /v1/projects/{id}/keyvalue-links", core.HandleLinksMapped[core.KeyValueLinks](s.SetKeyValues, s.renderProject))
}
