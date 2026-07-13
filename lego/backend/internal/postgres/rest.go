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

package postgres

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RegisterREST adds the managed-Postgres endpoints, Render-shaped (/v1/postgres)
// plus a bex-native /v1/databases alias.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	// verb maps a lifecycle action to a handler with a Render-accurate status code
	// (suspend/resume 202, restart 200 — same as services).
	verb := func(status int, fn func(context.Context, string) (PostgresView, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			pg, err := fn(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, status, pg)
		}
	}
	for _, base := range []string{"/v1/postgres", "/v1/databases"} {
		mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
			out, err := s.ListPostgres(r.Context(), r.URL.Query().Get("ownerId"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
			var req CreatePostgresRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
				return
			}
			pg, err := s.CreatePostgres(r.Context(), req)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusCreated, pg) // Render: create => 201
		})
		mux.HandleFunc("GET "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
			pg, err := s.GetPostgres(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, pg)
		})
		mux.HandleFunc("DELETE "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
			if err := s.DeletePostgres(r.Context(), r.PathValue("id")); err != nil {
				core.WriteErr(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent) // Render: delete => 204
		})
		mux.HandleFunc("GET "+base+"/{id}/connection-info", func(w http.ResponseWriter, r *http.Request) {
			info, err := s.PostgresConnectionInfo(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, info)
		})

		// --- lifecycle (Render: POST /postgres/{id}/{suspend,resume,restart,failover}) ---
		mux.HandleFunc("POST "+base+"/{id}/suspend", verb(http.StatusAccepted, s.Suspend))
		mux.HandleFunc("POST "+base+"/{id}/resume", verb(http.StatusAccepted, s.Resume))
		mux.HandleFunc("POST "+base+"/{id}/restart", verb(http.StatusOK, s.Restart))
		mux.HandleFunc("POST "+base+"/{id}/failover", func(w http.ResponseWriter, r *http.Request) {
			// Render: 202 Accepted, no response body.
			if err := s.Failover(r.Context(), r.PathValue("id")); err != nil {
				core.WriteErr(w, err)
				return
			}
			w.WriteHeader(http.StatusAccepted)
		})

		// --- recovery / exports (Render: recovery-info, recover, exports) ---
		recoveryInfo := func(w http.ResponseWriter, r *http.Request) {
			info, err := s.RecoveryInfo(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, info)
		}
		mux.HandleFunc("GET "+base+"/{id}/recovery-info", recoveryInfo)
		mux.HandleFunc("POST "+base+"/{id}/recovery-info", recoveryInfo) // Render uses POST; bex accepts both
		mux.HandleFunc("POST "+base+"/{id}/recover", func(w http.ResponseWriter, r *http.Request) {
			var req RecoverRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
				return
			}
			pg, err := s.Recover(r.Context(), r.PathValue("id"), req)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusCreated, pg) // a new instance => 201
		})
		mux.HandleFunc("GET "+base+"/{id}/exports", func(w http.ResponseWriter, r *http.Request) {
			out, err := s.ListExports(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("POST "+base+"/{id}/exports", func(w http.ResponseWriter, r *http.Request) {
			exp, err := s.CreateExport(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusCreated, exp)
		})

		// --- access: IP allowlist + users ---
		mux.HandleFunc("GET "+base+"/{id}/ip-allow-list", func(w http.ResponseWriter, r *http.Request) {
			list, err := s.GetIPAllowList(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, map[string][]string{"cidrs": list})
		})
		mux.HandleFunc("PUT "+base+"/{id}/ip-allow-list", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				CIDRs []string `json:"cidrs"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
				return
			}
			pg, err := s.SetIPAllowList(r.Context(), r.PathValue("id"), req.CIDRs)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, pg)
		})
		mux.HandleFunc("GET "+base+"/{id}/users", func(w http.ResponseWriter, r *http.Request) {
			users, err := s.ListUsers(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, users)
		})
		mux.HandleFunc("POST "+base+"/{id}/users", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
				return
			}
			res, err := s.CreateUser(r.Context(), r.PathValue("id"), req.Name)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusCreated, res)
		})
		mux.HandleFunc("DELETE "+base+"/{id}/users/{user}", func(w http.ResponseWriter, r *http.Request) {
			if err := s.DeleteUser(r.Context(), r.PathValue("id"), r.PathValue("user")); err != nil {
				core.WriteErr(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		// --- observability: processes / top-queries / sizes / table-scans / parameter-overrides ---
		mux.HandleFunc("GET "+base+"/{id}/processes", func(w http.ResponseWriter, r *http.Request) {
			out, err := s.Processes(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("GET "+base+"/{id}/top-queries", func(w http.ResponseWriter, r *http.Request) {
			out, err := s.TopQueries(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("GET "+base+"/{id}/sizes", func(w http.ResponseWriter, r *http.Request) {
			out, err := s.Sizes(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("GET "+base+"/{id}/table-scans", func(w http.ResponseWriter, r *http.Request) {
			out, err := s.TableScans(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("GET "+base+"/{id}/parameter-overrides", func(w http.ResponseWriter, r *http.Request) {
			out, err := s.ParameterOverrides(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, out)
		})
		mux.HandleFunc("PUT "+base+"/{id}/parameter-overrides", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Parameters map[string]string `json:"parameters"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
				return
			}
			pg, err := s.SetParameterOverrides(r.Context(), r.PathValue("id"), req.Parameters)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, pg)
		})
	}
}
