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

package members

import (
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the members REST fragment (bex extension — Render manages members
// only via its dashboard GraphQL; naming follows Render's kebab-case nouns,
// scoped under the workspace they belong to).

// RegisterREST mounts the member + invite endpoints under a workspace.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/members", func(w http.ResponseWriter, r *http.Request) {
		ms, err := s.List(r.Context(), r.PathValue("workspaceId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, ms)
	})
	mux.HandleFunc("POST /v1/workspaces/{workspaceId}/members", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		inv, err := s.Invite(r.Context(), r.PathValue("workspaceId"), req.Email, req.Role)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, inv)
	})
	mux.HandleFunc("PATCH /v1/workspaces/{workspaceId}/members/{subject}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Role string `json:"role"`
		}
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		m, err := s.ChangeRole(r.Context(), r.PathValue("workspaceId"), r.PathValue("subject"), req.Role)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, m)
	})
	mux.HandleFunc("DELETE /v1/workspaces/{workspaceId}/members/{subject}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Remove(r.Context(), r.PathValue("workspaceId"), r.PathValue("subject")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/invites", func(w http.ResponseWriter, r *http.Request) {
		invs, err := s.ListInvites(r.Context(), r.PathValue("workspaceId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, invs)
	})
	mux.HandleFunc("DELETE /v1/workspaces/{workspaceId}/invites/{inviteId}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.RevokeInvite(r.Context(), r.PathValue("workspaceId"), r.PathValue("inviteId")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// Seat usage — Render's owner.usage.users {used, limit} as a REST read
	// (w1/m33); limit 0 means unlimited (the paid plans).
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/seat-usage", func(w http.ResponseWriter, r *http.Request) {
		u, err := s.SeatUsage(r.Context(), r.PathValue("workspaceId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, u)
	})
	// Resend a pending invite (w1/m33): fresh email, refreshed expiry, same id;
	// the token rotates (w1/041) so the new mail's link supersedes the original.
	mux.HandleFunc("POST /v1/workspaces/{workspaceId}/invites/{inviteId}/resend", func(w http.ResponseWriter, r *http.Request) {
		inv, err := s.ResendInvite(r.Context(), r.PathValue("workspaceId"), r.PathValue("inviteId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, inv)
	})
	// The caller's own effective permissions in a workspace (w9/m84, bex
	// extension) — ownerId query param optional (absent => the caller's default
	// workspace). The dashboard reads this to disable controls the server would
	// refuse; a non-member gets the same 403 as any workspace-scoped read.
	mux.HandleFunc("GET /v1/viewer/capabilities", func(w http.ResponseWriter, r *http.Request) {
		caps, err := s.Capabilities(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, caps)
	})
	// Redeem an invite token for the authenticated caller (w1/m33). Not
	// workspace-scoped: the token identifies the invite; the caller may not
	// (yet) be a member of the workspace it joins them to.
	mux.HandleFunc("POST /v1/invites/accept", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Token string `json:"token"`
		}
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		acc, err := s.AcceptInvite(r.Context(), req.Token)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, acc)
	})
}
