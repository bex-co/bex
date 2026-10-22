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
	"context"
	"fmt"
	"net/http"
	"slices"
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

// createCredentialRequest is POST /v1/registrycredentials' body.
type createCredentialRequest struct {
	OwnerID   string `json:"ownerId"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	Username  string `json:"username"`
	AuthToken string `json:"authToken"`
	// ExpiresAt is RFC3339; empty means the credential never expires.
	ExpiresAt string `json:"expiresAt"`
}

// patchCredentialRequest is PATCH /v1/registrycredentials/{id}'s body.
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

// parseExpiresAt parses an RFC3339 timestamp, "" => nil (no expiry). The nil
// is why this wraps core.ParseTime rather than being replaced by it: here
// "absent" has to stay distinguishable from the zero time, because a nil means
// "never expires" while a zero time would mean "expired at the epoch".
func parseExpiresAt(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := core.ParseTime("expiresAt", s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ownerScoped binds `?ownerId` for the by-id routes, so a credential created
// with `ownerId: B` stays readable, updatable and deletable in B. Omitting the
// parameter keeps the caller's default workspace, exactly as before; the
// membership check rides core.Base's named-workspace resolution (w4/128).
func restOwnerScoped(r *http.Request) context.Context {
	return core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId"))
}

// RegisterREST mounts Render's canonical registrycredentials CRUD surface.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/registrycredentials", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		q := r.URL.Query()
		// Every filter Render's pinned spec declares passes the request
		// validator, so one this handler ignored would return an unfiltered
		// list the caller believes is filtered (w5/163). Same helpers and order
		// as the disks and Key Value lists.
		if q.Has("type") {
			// Render's type is its registry enum, which bex replaces with an
			// arbitrary host (see the note at the top of this file): refuse it
			// by name rather than answer every credential.
			return nil, fmt.Errorf("%w: type filters by Render's registry enum, which bex does not store (credentials carry a registry host); filter by name or username instead", core.ErrBadRequest)
		}
		names := core.QueryNameList(r.URL, "name")
		usernames := core.QueryNameList(r.URL, "username")
		created, err := core.QueryTimeWindow(q, "createdBefore", "createdAfter")
		if err != nil {
			return nil, err
		}
		updated, err := core.QueryTimeWindow(q, "updatedBefore", "updatedAfter")
		if err != nil {
			return nil, err
		}
		views, err := core.ListOwners(r.Context(), core.QueryList(q, "ownerId"), s.List)
		if err != nil {
			return nil, err
		}
		views = core.Filter(views, func(v CredentialView) bool {
			return (len(names) == 0 || slices.Contains(names, v.Name)) &&
				(len(usernames) == 0 || slices.Contains(usernames, v.Username)) &&
				created.Contains(v.CreatedAt) && updated.Contains(v.UpdatedAt)
		})
		after, limit := core.PageParams(q)
		views = core.StablePage(views, after, limit, true,
			func(v CredentialView) string { return v.ID })
		return toWireList(views), nil
	}))

	mux.HandleFunc("POST /v1/registrycredentials", func(w http.ResponseWriter, r *http.Request) {
		var req createCredentialRequest
		if err := core.DecodeJSON(r, &req); err != nil {
			// The strict decoder's message is safe and names the field, e.g.
			// Render's `registry`, which bex's wire has no place for (w8/135).
			core.WriteErr(w, fmt.Errorf("%w: %v", core.ErrBadRequest, err))
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

	mux.HandleFunc("GET /v1/registrycredentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Get(restOwnerScoped(r), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toWire(v))
	})

	mux.HandleFunc("PATCH /v1/registrycredentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req patchCredentialRequest
		if err := core.DecodeJSON(r, &req); err != nil {
			// The strict decoder's message is safe and names the field, e.g.
			// Render's `registry`, which bex's wire has no place for (w8/135).
			core.WriteErr(w, fmt.Errorf("%w: %v", core.ErrBadRequest, err))
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
		v, err := s.Update(restOwnerScoped(r), r.PathValue("id"), upd)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toWire(v))
	})

	mux.HandleFunc("DELETE /v1/registrycredentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Delete(restOwnerScoped(r), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
