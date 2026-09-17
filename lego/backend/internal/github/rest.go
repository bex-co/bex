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

package github

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RegisterREST mounts the GitHub-connect surface. `GET /v1/repos` and the
// connection verbs are bex extensions (Render exposes repos only via its private
// dashboard API); naming follows Render's kebab-case noun style. The callback is
// GitHub's post-install "Setup URL" redirect target. Browser callbacks carry a
// short-lived signed state credential instead of a dashboard cookie. Every
// binding also carries GitHub's single-use user-authorization code; bearer
// authentication alone is never an installation-ownership proof.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/git/connect", func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.StartConnect(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, conn)
	})

	// POST /v1/git/claim — start the ADR078 §3a claim flow: bind an installation
	// that already exists on GitHub (where the install URL strips the state) via
	// the OAuth user-authorization round trip. Returns {claimUrl}. The optional
	// ?installationId= narrows the candidate set the callback proves.
	mux.HandleFunc("POST /v1/git/claim", func(w http.ResponseWriter, r *http.Request) {
		installationID, err := optionalInstallationID(r.URL.Query().Get("installationId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		claim, err := s.StartClaim(r.Context(), r.URL.Query().Get("ownerId"), installationID)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, claim)
	})

	// GET /v1/git/claim/selections/{id} — render an ambiguous claim's proved
	// options (ADR078 §3a). Authenticated and subject-matched: the id names a
	// pending choice, it is not a capability.
	mux.HandleFunc("GET /v1/git/claim/selections/{id}", func(w http.ResponseWriter, r *http.Request) {
		sel, err := s.GetClaimSelection(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, sel)
	})

	// POST /v1/git/claim/selections/{id} — complete the claim by binding one
	// option the callback already proved. Single-use; can_manage re-checked now.
	mux.HandleFunc("POST /v1/git/claim/selections/{id}", func(w http.ResponseWriter, r *http.Request) {
		installationID, err := optionalInstallationID(r.URL.Query().Get("installationId"))
		if err != nil || installationID == 0 {
			core.WriteErr(w, fmt.Errorf("%w: installationId is required", core.ErrBadRequest))
			return
		}
		conn, err := s.SelectClaim(r.Context(), r.URL.Query().Get("ownerId"), r.PathValue("id"), installationID)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, conn)
	})

	mux.HandleFunc("GET /v1/git/callback", func(w http.ResponseWriter, r *http.Request) {
		stateToken := r.URL.Query().Get("state")
		if !s.configured() {
			// Preserve the optional-feature contract: when GitHub or its store is
			// off, every git-connect verb is a real 503 (never a misleading UI
			// redirect that makes the API look available).
			core.WriteErr(w, core.ErrGitHubUnavailable)
			return
		}
		if r.URL.Query().Get("error") != "" {
			err := errors.New("github did not complete the app installation")
			s.writeCallbackFailure(w, r, "github_error", err)
			return
		}

		if stateToken == "" {
			s.writeCallbackFailure(w, r, callbackStateErrorCode(errConnectStateMissing), errConnectStateMissing)
			return
		}
		nonce, err := s.verifyConnectState(stateToken)
		if err != nil {
			if errors.Is(err, core.ErrGitHubUnavailable) {
				core.WriteErr(w, err)
				return
			}
			s.writeCallbackFailure(w, r, callbackStateErrorCode(err), err)
			return
		}

		// The authenticated bex subject, when the browser presented one. The gate
		// lets this exact route through anonymously (GitHub has no bex credential
		// to offer), but the session cookie IS Lax-scoped, so a signed-in user's
		// identity does arrive on this top-level navigation — and w1/m67 F3 now
		// requires it to match whoever started the flow.
		caller := ""
		if identity, ok := core.IdentityFrom(r.Context()); ok {
			caller = identity.Subject
		}
		raw := r.URL.Query().Get("installation_id")
		if raw == "" {
			// No installation id at all ⇒ the ADR078 §3a claim flow: GitHub's OAuth
			// authorize redirect carries only code + state, and the installation is
			// resolved server-side from the authorizing user's admin set. A PRESENT
			// but malformed id stays invalid_installation below — only true absence
			// selects the claim branch.
			if _, err := s.claimFromCallback(r.Context(), nonce, caller, r.URL.Query().Get("code")); err != nil {
				// Ambiguity is no longer a dead end (ADR078 §3a): the callback proved
				// several installations, so it hands the human the choice instead of
				// discarding the work. Not a failure — a deferred success.
				var pending *claimSelectionRequiredError
				if errors.As(err, &pending) {
					if location := s.callbackRedirectWith("git_claim_selection", pending.SelectionID); location != "" {
						redirectCallback(w, r, location)
						return
					}
					core.WriteJSON(w, http.StatusOK, map[string]string{
						"status":      "selection_required",
						"selectionId": pending.SelectionID,
					})
					return
				}
				s.writeCallbackFailure(w, r, callbackConnectErrorCode(err), err)
				return
			}
		} else {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id <= 0 {
				s.writeCallbackFailure(w, r, "invalid_installation", core.ErrBadRequest)
				return
			}
			if _, err := s.connectFromCallback(r.Context(), nonce, caller, id, r.URL.Query().Get("code")); err != nil {
				s.writeCallbackFailure(w, r, callbackConnectErrorCode(err), err)
				return
			}
		}
		if location := s.callbackRedirect(""); location != "" {
			redirectCallback(w, r, location)
			return
		}
		core.WriteJSON(w, http.StatusOK, map[string]string{"status": "connected"})
	})

	// GET /v1/git/connections — the workspace's full connection set (ADR078). The
	// multi-account surface the dashboard reads; the singular alias below stays for
	// compatibility.
	mux.HandleFunc("GET /v1/git/connections", func(w http.ResponseWriter, r *http.Request) {
		conns, err := s.ListConnections(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, conns)
	})

	// DELETE /v1/git/connections/{installationId} — disconnect one installation
	// (ADR078). Admin-only, scoped to the caller's workspace.
	mux.HandleFunc("DELETE /v1/git/connections/{installationId}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("installationId"), 10, 64)
		if err != nil || id <= 0 {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		if err := s.Disconnect(r.Context(), r.URL.Query().Get("ownerId"), id); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// GET/DELETE /v1/git/connection (singular) — deprecated compatibility aliases
	// over the workspace's sole connection (ADR078 §5). DELETE with several
	// connections is an ambiguous 409; use the per-installation route.
	mux.HandleFunc("GET /v1/git/connection", func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.GetConnection(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, conn)
	})

	mux.HandleFunc("DELETE /v1/git/connection", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Disconnect(r.Context(), r.URL.Query().Get("ownerId"), 0); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /v1/repos", func(w http.ResponseWriter, r *http.Request) {
		repos, err := s.ListRepos(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, repos)
	})
}

func callbackStateErrorCode(err error) string {
	switch {
	case errors.Is(err, errConnectStateMissing):
		return "missing_state"
	case errors.Is(err, errConnectStateExpired):
		return "expired_state"
	default:
		return "invalid_state"
	}
}

func callbackConnectErrorCode(err error) string {
	switch {
	// The claim sentinels wrap ErrBadRequest for their HTTP class, so they must
	// be matched before the generic case.
	case errors.Is(err, errNoClaimableInstallation):
		return "no_claimable_installation"
	case errors.Is(err, errAmbiguousClaim):
		return "ambiguous_installation"
	case errors.Is(err, core.ErrBadRequest):
		return "invalid_installation"
	default:
		return "connect_failed"
	}
}

// writeCallbackFailure keeps a browser out of a bare API error page. Only fixed,
// non-sensitive reason codes cross into the dashboard URL; GitHub/upstream error
// text is never reflected. A self-hoster without BEX_DASHBOARD_URL retains a
// clear JSON response with the domain error's normal status.
func (s *Service) writeCallbackFailure(w http.ResponseWriter, r *http.Request, code string, err error) {
	if location := s.callbackRedirect(code); location != "" {
		redirectCallback(w, r, location)
		return
	}
	if code == "missing_state" || code == "invalid_state" || code == "expired_state" || code == "github_error" {
		// The one error dialect (w9/m38), plus a `code` superset the dashboard's
		// callback page keys on to render a fixed, non-sensitive reason.
		core.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "message": err.Error(), "id": "bad_request", "code": code})
		return
	}
	core.WriteErr(w, err)
}

// redirectCallback prevents the credential-bearing callback URL from becoming
// the Referer of the dashboard page or any resource it loads.
func redirectCallback(w http.ResponseWriter, r *http.Request, location string) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, location, http.StatusFound)
}

func (s *Service) callbackRedirect(errorCode string) string {
	return s.callbackRedirectWith("git_error", errorCode)
}

// callbackRedirectWith builds the dashboard return URL carrying one bounded
// query parameter. Two are used: `git_error` for a fixed failure code, and
// `git_claim_selection` for a pending account choice (ADR078 §3a) — which is a
// SUCCESS continuation, not a failure, and deliberately does not reuse the error
// channel the dashboard renders as an alert.
func (s *Service) callbackRedirectWith(key, value string) string {
	if s.DashboardURL == "" {
		return ""
	}
	u, err := url.Parse(strings.TrimRight(s.DashboardURL, "/") + "/settings")
	if err != nil {
		return ""
	}
	if value != "" {
		q := u.Query()
		q.Set(key, value)
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// optionalInstallationID parses a GitHub installation id from a query value.
// Empty is 0 (unspecified), never an error; anything present must be a positive
// integer — a malformed id is a bad request rather than a silent "unspecified",
// which would quietly widen a claim the caller meant to narrow.
func optionalInstallationID(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("%w: installationId must be a positive integer", core.ErrBadRequest)
	}
	return v, nil
}
