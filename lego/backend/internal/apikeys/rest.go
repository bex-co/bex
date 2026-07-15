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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RegisterREST adds the machine-credential endpoints (bex extension — Render
// manages API keys only via its dashboard; naming follows Render's kebab-case
// noun style). The secret appears once, in the create response.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	// POST /v1/oauth/revoke is Render's own REST endpoint (docs/render-artifacts
	// aren't the source here — this one is only reachable by reading the
	// official CLI's source, pkg/client/oauth/oauth.go: `render logout` calls
	// it with the caller's OWN access token as the Bearer credential, expects
	// 204 No Content, and its contract is exactly "invalidate the credential
	// that's calling this"). bex has no client_secret-authenticated revocation
	// (Hydra's public /oauth2/revoke needs it, and this caller only ever holds
	// the bearer token, never the secret), so self-revoke is implemented as
	// deleting the caller's OWN underlying Hydra client — the same effect
	// RevokeAPIKey gives a dashboard user revoking their own key, just aimed at
	// the caller instead of a named id. A session (Kratos) caller has no Hydra
	// client to delete, so it 400s rather than silently no-op'ing; the official
	// CLI never sends a session credential here (it revokes a device-flow OAuth
	// access token, always an oauth2-method identity in bex's model).
	mux.HandleFunc("POST /v1/oauth/revoke", func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.IdentityFrom(r.Context())
		if !ok || id.Method != "oauth2" {
			core.WriteErr(w, fmt.Errorf("%w: no OAuth2 credential to revoke", core.ErrBadRequest))
			return
		}
		if err := s.RevokeAPIKey(r.Context(), "", id.Subject); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name    string `json:"name"`
			OwnerID string `json:"ownerId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
