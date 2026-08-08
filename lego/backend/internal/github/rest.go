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
// short-lived signed state credential instead of a dashboard cookie; API callers
// may keep using their ordinary Bearer/session credential. In both cases the
// installation_id is validated against GitHub before anything is recorded
// (docs/ADR026-github-integration.md).
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/git/connect", func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.StartConnect(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, conn)
	})

	mux.HandleFunc("GET /v1/git/callback", func(w http.ResponseWriter, r *http.Request) {
		stateToken := r.URL.Query().Get("state")
		_, authenticated := core.IdentityFrom(r.Context())
		browserCallback := stateToken != "" || !authenticated
		if !s.configured() {
			// Preserve the optional-feature contract: when GitHub or its store is
			// off, every git-connect verb is a real 503 (never a misleading UI
			// redirect that makes the API look available).
			core.WriteErr(w, core.ErrGitHubUnavailable)
			return
		}
		if r.URL.Query().Get("error") != "" {
			err := errors.New("github did not complete the app installation")
			if browserCallback {
				s.writeCallbackFailure(w, r, "github_error", err)
			} else {
				core.WriteErr(w, core.ErrBadRequest)
			}
			return
		}

		workspaceID := ""
		if stateToken != "" {
			var err error
			workspaceID, err = s.verifyConnectState(stateToken)
			if err != nil {
				if errors.Is(err, core.ErrGitHubUnavailable) {
					core.WriteErr(w, err)
					return
				}
				s.writeCallbackFailure(w, r, callbackStateErrorCode(err), err)
				return
			}
		} else if !authenticated {
			s.writeCallbackFailure(w, r, callbackStateErrorCode(errConnectStateMissing), errConnectStateMissing)
			return
		}

		raw := r.URL.Query().Get("installation_id")
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			if browserCallback {
				s.writeCallbackFailure(w, r, "invalid_installation", core.ErrBadRequest)
			} else {
				core.WriteErr(w, core.ErrBadRequest)
			}
			return
		}
		if workspaceID != "" {
			// Browser install callback: enforce the installation-admin proof (F2)
			// using the OAuth `code` GitHub appends when user authorization is
			// enabled, then record against the state-authenticated workspace.
			_, err = s.connectFromCallback(r.Context(), workspaceID, id, r.URL.Query().Get("code"))
		} else {
			// Backward-compatible API/agent path: auth.go attached the caller
			// Identity, so the ordinary authorization-gated Connect still applies.
			_, err = s.Connect(r.Context(), id)
		}
		if err != nil {
			if browserCallback {
				s.writeCallbackFailure(w, r, callbackConnectErrorCode(err), err)
			} else {
				core.WriteErr(w, err)
			}
			return
		}
		if browserCallback {
			if location := s.callbackRedirect(""); location != "" {
				redirectCallback(w, r, location)
				return
			}
		}
		core.WriteJSON(w, http.StatusOK, map[string]string{"status": "connected"})
	})

	mux.HandleFunc("GET /v1/git/connection", func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.GetConnection(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, conn)
	})

	mux.HandleFunc("DELETE /v1/git/connection", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Disconnect(r.Context(), r.URL.Query().Get("ownerId")); err != nil {
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
	if s.DashboardURL == "" {
		return ""
	}
	u, err := url.Parse(strings.TrimRight(s.DashboardURL, "/") + "/settings")
	if err != nil {
		return ""
	}
	if errorCode != "" {
		q := u.Query()
		q.Set("git_error", errorCode)
		u.RawQuery = q.Encode()
	}
	return u.String()
}
