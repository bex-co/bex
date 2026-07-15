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

package workspaces

import (
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RegisterREST mounts Render's read-only /v1/owners surface
// (docs/render-artifacts/owners-api.md, w6/m2): list, retrieve (with the
// own-→default-workspace quirk), and members. No POST/PATCH/DELETE exists
// here — Render exposes none, and bex's workspace-mutation surface is the
// dashboard GraphQL (w6/m1's create/rename/delete).
func (s *Service) RegisterREST(mux *http.ServeMux) {
	// GET /v1/users — Render's "who am I" endpoint (components.schemas.user):
	// the CLI's `render whoami` and any client resolving its own identity.
	mux.HandleFunc("GET /v1/users", func(w http.ResponseWriter, r *http.Request) {
		u, err := s.CurrentUser(r.Context())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderUser(u))
	})
	mux.HandleFunc("GET /v1/owners", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		owners, err := s.ListOwners(r.Context(), OwnerFilter{Names: q["name"], Emails: q["email"]})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		// Render's cursor pagination (docs/render-artifacts/owners-api.md): each
		// item's cursor is its workspace id; `cursor`/`limit` page the result.
		after, limit := core.PageParams(q)
		page := core.Page(owners, after, limit, func(o OwnerView) string { return o.ID })
		core.WriteJSON(w, http.StatusOK, toOwnerList(page)) // [{owner, cursor}, ...]
	})
	mux.HandleFunc("GET /v1/owners/{ownerId}", func(w http.ResponseWriter, r *http.Request) {
		ws, err := s.GetWorkspace(r.Context(), r.PathValue("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderOwner(OwnerView{WorkspaceView: ws, Email: s.ownerEmail(r.Context(), ws.ID)}))
	})
	mux.HandleFunc("GET /v1/owners/{ownerId}/members", func(w http.ResponseWriter, r *http.Request) {
		members, err := s.ListMembers(r.Context(), r.PathValue("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderTeamMembers(members)) // bare array, no cursor
	})
	// GET /v1/owners/{ownerId}/limits — bex extension (w7/m9): resource caps
	// (services/postgres/keyValues used vs. limit, 0 = unlimited). Authorizes
	// can_view on the workspace, same as GET /v1/owners/{ownerId}.
	mux.HandleFunc("GET /v1/owners/{ownerId}/limits", func(w http.ResponseWriter, r *http.Request) {
		limits, err := s.ResourceLimits(r.Context(), r.PathValue("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, limits)
	})
}
