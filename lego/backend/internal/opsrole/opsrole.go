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

// Package opsrole is the ADR088 §4 server-only ops-role verb: "what is subject
// S's role in the pinned ops workspace" — the ADR024 role tuples read from
// OpenFGA plus a Kratos admin identity lookup, guarded by a static bearer.
//
// It mounts ONLY on bex-api's cluster-internal listener (the :8091 mux beside
// the control-plane API), never the public :8090 mux: api.bex.co routes the
// whole `/` prefix straight to :8090, so a public mount would leave the static
// bearer as the route's only protection against the internet. The consent
// acceptor (dashboard SSR) calls this verb to gate the Grafana OAuth client
// without talking to OpenFGA itself; the verb reports the RAW role — the
// contributor/billing deny policy lives in the acceptor, so policy stays in
// one place (docs/ADR088-platform-observability-ui.md §4).
package opsrole

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Path is the verb's route on the internal mux (GET only). Deliberately not a
// /v1 path: it is no part of the public REST/GraphQL/MCP surface and carries
// no Render parity obligation.
const Path = "/internal/ops-role"

// IdentityLookup resolves a Kratos identity's email + display-name traits (the
// workspaces.KratosIdentities admin reader, adapted by cmd/api). ok=false
// covers "identity missing" and "Kratos unreachable" alike — the verb fails
// closed (503) either way, because a subject holding an ops-workspace role
// must resolve to a live identity for the consent acceptor to stamp claims.
type IdentityLookup func(ctx context.Context, subject string) (email, name string, ok bool)

// Handler answers GET /internal/ops-role?subject=<kratos-identity-id>. It is
// machine-to-machine only: the static bearer is its whole authentication — no
// session/IdentityFrom auth ever applies.
type Handler struct {
	// Workspace is the pinned ops workspace id (BEX_OPS_WORKSPACE, a tea-* id).
	Workspace string
	// Token is the static bearer (BEX_OPS_ROLE_TOKEN), compared constant-time
	// BEFORE any OpenFGA or Kratos work.
	Token string
	// Authz reads the ADR024 role tuples (user:<kratos-id> <role>
	// workspace:<ops>). CheckFresh is preferred when implemented: the verb
	// admits operators into an admin UI, so a just-revoked membership must not
	// ride a ≤PositiveTTL cached allow (the destructive-verb pattern, round-5
	// finding 4). nil (OpenFGA unwired) fails closed with 503.
	Authz core.Checker
	// Identity resolves a member's email/name. nil (BEX_KRATOS_ADMIN_URL
	// unset) fails closed with 503 for members; a non-member answer never
	// needs it.
	Identity IdentityLookup
}

// Register mounts h on the internal mux only when the feature is fully
// configured (both BEX_OPS_WORKSPACE and BEX_OPS_ROLE_TOKEN). Otherwise the
// mux is left untouched, so the path answers the router's normal 404 — zero
// behavior change, the fixed contract with the consent acceptor.
func Register(mux *http.ServeMux, h *Handler) {
	if h == nil || h.Workspace == "" || h.Token == "" {
		return
	}
	mux.Handle("GET "+Path, h)
}

// roleLadder is ADR024's five workspace roles, highest authority first — the
// verb answers the FIRST role the subject holds. The role relations are direct
// ("this") in the OpenFGA model (deploy/gitops/authz/model.json): an admin
// does not satisfy a "viewer" check (only the can_* relations union the
// roles), so at most five reads, short-circuited at the first hit, is what
// "highest role" costs.
var roleLadder = [...]string{"admin", "developer", "contributor", "billing", "viewer"}

// memberAnswer is the member wire shape — all four keys always present ("name"
// may be the empty string). The non-member answer is exactly {"member":false}
// with no other keys; both shapes are pinned by the consent acceptor (t005).
type memberAnswer struct {
	Member bool   `json:"member"`
	Role   string `json:"role"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Bearer first, constant-time: a wrong or missing token is refused before
	// any OpenFGA/Kratos work, so the route cannot be used to probe either
	// backend. ConstantTimeCompare leaks at most the token length — the same
	// documented trade store.API.bearer makes for BEX_CP_TOKEN.
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
	role, err := h.highestRole(r.Context(), subject)
	if err != nil {
		// Fail closed (the caller rejects the login): an unreachable OpenFGA
		// must read as "cannot answer", never as "non-member".
		core.WriteErrStatus(w, http.StatusServiceUnavailable, "role lookup unavailable")
		return
	}
	if role == "" {
		core.WriteJSON(w, http.StatusOK, struct {
			Member bool `json:"member"`
		}{Member: false})
		return
	}
	if h.Identity == nil {
		core.WriteErrStatus(w, http.StatusServiceUnavailable, "identity lookup unavailable")
		return
	}
	email, name, ok := h.Identity(r.Context(), subject)
	if !ok {
		core.WriteErrStatus(w, http.StatusServiceUnavailable, "identity lookup unavailable")
		return
	}
	core.WriteJSON(w, http.StatusOK, memberAnswer{Member: true, Role: role, Email: email, Name: name})
}

// highestRole walks the ladder and returns the first role subject holds in the
// ops workspace, "" for a non-member. An error means the authorization backend
// could not answer — the caller maps it to 503.
func (h *Handler) highestRole(ctx context.Context, subject string) (string, error) {
	if h.Authz == nil {
		return "", core.Err("authorization checker not configured")
	}
	check := h.Authz.Check
	if fresh, ok := h.Authz.(core.FreshChecker); ok {
		check = fresh.CheckFresh
	}
	object := core.WorkspaceObject(h.Workspace)
	for _, role := range roleLadder {
		held, err := check(ctx, "user:"+subject, role, object)
		if err != nil {
			return "", err
		}
		if held {
			return role, nil
		}
	}
	return "", nil
}
