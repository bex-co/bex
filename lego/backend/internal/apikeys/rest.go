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

package apikeys

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RegisterREST adds the machine-credential endpoints (bex extension — Render
// manages API keys only via its dashboard; naming follows Render's kebab-case
// noun style). The secret, and the bearer `key` that carries it, appear once,
// in the create response.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name    string `json:"name"`
			OwnerID string `json:"ownerId"`
			// Permissions is the key's grant (w9/m496); omitted means All.
			Permissions *core.KeyGrant `json:"permissions"`
		}
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		key, err := s.CreateAPIKey(r.Context(), req.OwnerID, req.Name, req.Permissions)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, key)
	})
	// The caller's own keys; ?allMembers=true is an Admin's view of every
	// key in the workspace (w9/m539).
	mux.HandleFunc("GET /v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		allMembers := false
		if raw := q.Get("allMembers"); raw != "" {
			var err error
			if allMembers, err = strconv.ParseBool(raw); err != nil {
				core.WriteErr(w, fmt.Errorf("%w: allMembers must be true or false", core.ErrBadRequest))
				return
			}
		}
		keys, err := s.ListAPIKeys(r.Context(), q.Get("ownerId"), allMembers)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, keys)
	})
	// The permission picker's catalog: Render's areas, permissions and
	// levels, each level marked with whether the caller may grant it.
	mux.HandleFunc("GET /v1/api-keys/permissions", func(w http.ResponseWriter, r *http.Request) {
		areas, err := s.PermissionCatalog(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, areas)
	})
	// Change a key's permissions (Render's gear icon). The body's
	// permissions replace the key's grant whole.
	mux.HandleFunc("PATCH /v1/api-keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Permissions *core.KeyGrant `json:"permissions"`
		}
		if err := core.DecodeJSON(r, &req); err != nil || req.Permissions == nil {
			core.WriteErr(w, fmt.Errorf("%w: permissions: required", core.ErrBadRequest))
			return
		}
		key, err := s.UpdateAPIKeyPermissions(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id"), *req.Permissions)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, key)
	})
	mux.HandleFunc("DELETE /v1/api-keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.RevokeAPIKey(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
