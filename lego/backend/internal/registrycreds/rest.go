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

package registrycreds

import (
	"net/http"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the REST fragment, verified against Render's registryCredential
// endpoints (POST/GET https://api.render.com/v1/registrycredentials — live
// docs check, 2026-07-13). Render's object nests a "registry" enum
// (GITHUB|GITLAB|DOCKER|GOOGLE_ARTIFACT|AWS_ECR) rather than a raw hostname;
// bex deliberately generalizes this to `host` (an arbitrary registry
// hostname, e.g. "ghcr.io") because the operator's pull-secret mechanism is
// genuinely provider-agnostic dockerconfigjson — a closed enum would either
// under-serve self-hosted registries or need per-provider host-mapping data
// (account/region for AWS_ECR, project for GOOGLE_ARTIFACT) this milestone
// has no evidence Render even fully documents. `authToken` (the secret) and
// `name` match Render's field names exactly; `ownerId` matches Render's
// workspace-scoping convention (apps/keyvalue's create pattern).

// credentialWire is the wire shape every surface renders — Render-shaped
// (id/name/username/ownerId/expiresAt/createdAt/updatedAt) plus bex's own
// `host` (see the divergence note above) and `status` (w2/m14/t007 expiry
// surfacing; Render has no equivalent field). Never carries the secret.
type credentialWire struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	Username  string `json:"username"`
	OwnerID   string `json:"ownerId,omitempty"`
	ExpiresAt string `json:"expiresAt,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

func toWire(v CredentialView) credentialWire {
	return credentialWire{
		ID: v.ID, Name: v.Name, Host: v.Host, Username: v.Username, OwnerID: v.OwnerID,
		ExpiresAt: v.ExpiresAt, Status: v.Status, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
}

func toWireList(views []CredentialView) []credentialWire {
	out := make([]credentialWire, 0, len(views))
	for _, v := range views {
		out = append(out, toWire(v))
	}
	return out
}

// createCredentialRequest is POST /v1/registry-credentials' body.
type createCredentialRequest struct {
	OwnerID   string `json:"ownerId"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	Username  string `json:"username"`
	AuthToken string `json:"authToken"`
	// ExpiresAt is RFC3339; empty means the credential never expires.
	ExpiresAt string `json:"expiresAt"`
}

// patchCredentialRequest is PATCH /v1/registry-credentials/{id}'s body.
// Pointer fields follow the codebase's nil-means-unchanged PATCH convention
// (mirrors apps' patchServiceRequest). ExpiresAt is a pointer to a string so
// "absent" (nil, leave unchanged) is distinct from an explicit "" (clear the
// expiry) or an RFC3339 value (set it).
type patchCredentialRequest struct {
	Name      *string `json:"name"`
	Username  *string `json:"username"`
	AuthToken *string `json:"authToken"`
	ExpiresAt *string `json:"expiresAt"`
}

// parseExpiresAt parses an RFC3339 timestamp, "" => nil (no expiry).
func parseExpiresAt(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, core.ErrBadRequest
	}
	return &t, nil
}

// RegisterREST mounts the registry-credentials CRUD surface. The canonical
// Render spelling is /registrycredentials; the hyphenated route predates the
// official-CLI harness and remains as a bex alias.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/registry-credentials", func(w http.ResponseWriter, r *http.Request) {
		views, err := s.List(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toWireList(views))
	})

	mux.HandleFunc("POST /v1/registry-credentials", func(w http.ResponseWriter, r *http.Request) {
		var req createCredentialRequest
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		expiresAt, err := parseExpiresAt(req.ExpiresAt)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		v, err := s.Create(r.Context(), CreateRequest{
			OwnerID: req.OwnerID, Name: req.Name, Host: req.Host, Username: req.Username,
			Secret: req.AuthToken, ExpiresAt: expiresAt,
		})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, toWire(v))
	})

	mux.HandleFunc("GET /v1/registry-credentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toWire(v))
	})

	mux.HandleFunc("PATCH /v1/registry-credentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req patchCredentialRequest
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		upd := UpdateRequest{Name: req.Name, Username: req.Username, Secret: req.AuthToken}
		if req.ExpiresAt != nil {
			expiresAt, err := parseExpiresAt(*req.ExpiresAt)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			upd.ExpiresAtSet = true
			upd.ExpiresAt = expiresAt
		}
		v, err := s.Update(r.Context(), r.PathValue("id"), upd)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toWire(v))
	})

	mux.HandleFunc("DELETE /v1/registry-credentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Delete(r.Context(), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// The official CLI resolves --registry-credential through the canonical
	// Render path before it sends the service mutation. Forwarding through this
	// same mux means both spellings use byte-identical handlers and policies.
	canonical := func(w http.ResponseWriter, r *http.Request) {
		clone := r.Clone(r.Context())
		clone.URL.Path = strings.Replace(r.URL.Path, "/v1/registrycredentials", "/v1/registry-credentials", 1)
		mux.ServeHTTP(w, clone)
	}
	mux.HandleFunc("/v1/registrycredentials", canonical)
	mux.HandleFunc("/v1/registrycredentials/", canonical)
}
