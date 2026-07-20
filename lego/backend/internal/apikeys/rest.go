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
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RegisterREST adds the machine-credential endpoints (bex extension — Render
// manages API keys only via its dashboard; naming follows Render's kebab-case
// noun style). The secret appears once, in the create response.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name    string `json:"name"`
			OwnerID string `json:"ownerId"`
		}
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		key, err := s.CreateAPIKey(r.Context(), req.OwnerID, req.Name)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, key)
	})
	mux.HandleFunc("GET /v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		keys, err := s.ListAPIKeys(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, keys)
	})
	mux.HandleFunc("DELETE /v1/api-keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.RevokeAPIKey(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
