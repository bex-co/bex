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

package secrets

import (
	"encoding/json"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the env-vars REST fragment — Render public-API compatible, all five
// of Render's env-var endpoints: list, retrieve-one, replace-all, upsert-one,
// delete-one. Served under Render's noun /v1/services and bex's /v1/apps alias.
// Behavior lives in the Service, so GraphQL and MCP stay identical.

// envVarWithCursor is Render's env-vars list-item envelope
// ({envVar:{key,value}, cursor}); GET/PUT return an array of these.
type envVarWithCursor struct {
	EnvVar EnvVarView `json:"envVar"`
	Cursor string     `json:"cursor"`
}

// toEnvVarList wraps key-sorted env vars in Render's cursor envelope; the key is
// a stable, opaque-enough cursor (as the App name is for services).
func toEnvVarList(vars []EnvVarView) []envVarWithCursor {
	out := make([]envVarWithCursor, 0, len(vars))
	for _, v := range vars {
		out = append(out, envVarWithCursor{EnvVar: v, Cursor: v.Key})
	}
	return out
}

// RegisterREST adds the Render-shaped env-vars endpoints. Store unconfigured =>
// the Service returns core.ErrSecretsUnavailable => 503 on these routes only.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	for _, base := range []string{"/v1/services", "/v1/apps"} {
		mux.HandleFunc("GET "+base+"/{id}/env-vars", func(w http.ResponseWriter, r *http.Request) {
			vars, err := s.ListEnvVars(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, toEnvVarList(vars))
		})
		mux.HandleFunc("PUT "+base+"/{id}/env-vars", func(w http.ResponseWriter, r *http.Request) {
			var in []EnvVarView
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				core.WriteErr(w, core.ErrBadRequest)
				return
			}
			vars, err := s.SetEnvVars(r.Context(), r.PathValue("id"), in)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, toEnvVarList(vars))
		})
		mux.HandleFunc("GET "+base+"/{id}/env-vars/{key}", func(w http.ResponseWriter, r *http.Request) {
			v, err := s.GetEnvVar(r.Context(), r.PathValue("id"), r.PathValue("key"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, v) // Render: bare {key,value}, no cursor envelope
		})
		mux.HandleFunc("PUT "+base+"/{id}/env-vars/{key}", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Value string `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				core.WriteErr(w, core.ErrBadRequest)
				return
			}
			v, err := s.SetEnvVar(r.Context(), r.PathValue("id"), r.PathValue("key"), req.Value)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, v)
		})
		mux.HandleFunc("DELETE "+base+"/{id}/env-vars/{key}", func(w http.ResponseWriter, r *http.Request) {
			if err := s.DeleteEnvVar(r.Context(), r.PathValue("id"), r.PathValue("key")); err != nil {
				core.WriteErr(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent) // Render: delete => 204
		})
	}
}
