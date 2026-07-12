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
	"net/http"
	"strconv"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RegisterREST mounts the GitHub-connect surface. `GET /v1/repos` and the
// connection verbs are bex extensions (Render exposes repos only via its private
// dashboard API); naming follows Render's kebab-case noun style. The callback is
// GitHub's post-install "Setup URL" redirect target — it carries the dashboard
// session (no bearer), and its installation_id is validated server-side before
// anything is recorded (docs/github-integration.md).
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/git/connect", func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.StartConnect(r.Context())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, conn)
	})

	mux.HandleFunc("GET /v1/git/callback", func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.Query().Get("installation_id")
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		if _, err := s.Connect(r.Context(), id); err != nil {
			core.WriteErr(w, err)
			return
		}
		if s.DashboardURL != "" {
			http.Redirect(w, r, strings.TrimRight(s.DashboardURL, "/")+"/settings", http.StatusFound)
			return
		}
		core.WriteJSON(w, http.StatusOK, map[string]string{"status": "connected"})
	})

	mux.HandleFunc("GET /v1/git/connection", func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.GetConnection(r.Context())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, conn)
	})

	mux.HandleFunc("DELETE /v1/git/connection", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Disconnect(r.Context()); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /v1/repos", func(w http.ResponseWriter, r *http.Request) {
		repos, err := s.ListRepos(r.Context())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, repos)
	})
}
