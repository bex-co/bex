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

// Package identityclaims is the server-only identity-claims verb: "what are
// subject S's email, verification state, and display name" — a Kratos admin
// identity lookup, guarded by a static bearer. The dashboard consent acceptor
// calls it for identity-claims clients (OAUTH_IDENTITY_CLIENTS, today the
// forum.bex.co Discourse client) and stamps the answer into the id_token, the
// way it stamps ops claims for Grafana via the ops-role verb.
//
// Same posture as the ops-role verb (internal/opsrole): mounted ONLY on
// bex-api's cluster-internal listener, never the public :8090 mux, and
// authenticated by the same BEX_OPS_ROLE_TOKEN bearer — the caller is the
// same dashboard SSR over the same internal hop, so a second secret would add
// rotation work without separating any trust.
package identityclaims

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Path is the verb's route on the internal mux (GET only). Not a /v1 path: it
// is no part of the public REST/GraphQL/MCP surface.
const Path = "/internal/identity-claims"

// Claims is one identity's OIDC profile claims. ok=false from a Lookup covers
// "identity missing" and "Kratos unreachable" alike; the verb fails closed
// (503) either way, so the consent acceptor never accepts without an email.
type Claims struct {
	Email         string
	EmailVerified bool
	Name          string
}

// Lookup resolves a Kratos identity's claims (the workspaces.KratosIdentities
// admin reader, adapted by cmd/api).
type Lookup func(ctx context.Context, subject string) (Claims, bool)

// Handler answers GET /internal/identity-claims?subject=<kratos-identity-id>.
// Machine-to-machine only: the static bearer is its whole authentication.
type Handler struct {
	// Token is the static bearer (BEX_OPS_ROLE_TOKEN), compared constant-time
	// BEFORE any Kratos work.
	Token string
	// Identity resolves the claims. nil (BEX_KRATOS_ADMIN_URL unset) fails
	// closed with 503.
	Identity Lookup
}

// Register mounts h on the internal mux only when it has a bearer. Otherwise
// the mux is left untouched and the path answers the router's normal 404.
func Register(mux *http.ServeMux, h *Handler) {
	if h == nil || h.Token == "" {
		return
	}
	mux.Handle("GET "+Path, h)
}

// answer is the wire shape — every key always present ("name" may be "").
type answer struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	got := []byte(r.Header.Get("Authorization"))
	want := []byte("Bearer " + h.Token)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		core.WriteErrStatus(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	subject := r.URL.Query().Get("subject")
	if subject == "" {
		core.WriteErrStatus(w, http.StatusBadRequest, "subject query parameter is required")
		return
	}
	if h.Identity == nil {
		core.WriteErrStatus(w, http.StatusServiceUnavailable, "identity lookup unavailable")
		return
	}
	c, ok := h.Identity(r.Context(), subject)
	if !ok || c.Email == "" {
		// An identity with no email cannot become a forum account; refusing
		// here keeps "no email" from ever reaching an accept.
		core.WriteErrStatus(w, http.StatusServiceUnavailable, "identity lookup unavailable")
		return
	}
	core.WriteJSON(w, http.StatusOK, answer{Email: c.Email, EmailVerified: c.EmailVerified, Name: c.Name})
}
