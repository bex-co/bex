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

package envgroups

import (
	"encoding/json"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the env-groups REST fragment (Render's /v1/env-groups): group CRUD,
// its env vars + secret files, and service link/unlink. Behavior lives in the
// Service, so GraphQL and MCP stay identical.

// RegisterREST mounts the env-groups endpoints. Store unconfigured => the Service
// returns core.ErrSecretsUnavailable => 503 on these routes only.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/env-groups", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.ListEnvGroups(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /v1/env-groups", func(w http.ResponseWriter, r *http.Request) {
		var req CreateEnvGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		g, err := s.CreateEnvGroup(r.Context(), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, g) // Render: create => 201
	})
	mux.HandleFunc("GET /v1/env-groups/{id}", func(w http.ResponseWriter, r *http.Request) {
		g, err := s.GetEnvGroup(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, g)
	})
	mux.HandleFunc("PATCH /v1/env-groups/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		g, err := s.RenameEnvGroup(r.Context(), r.PathValue("id"), req.Name)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, g)
	})
	mux.HandleFunc("DELETE /v1/env-groups/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.DeleteEnvGroup(r.Context(), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// Group env vars: replace-all plus Render's per-key reveal/upsert/delete.
	mux.HandleFunc("PUT /v1/env-groups/{id}/env-vars", func(w http.ResponseWriter, r *http.Request) {
		var in []EnvVarView
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		vars, err := s.SetEnvGroupVars(r.Context(), r.PathValue("id"), in)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, vars)
	})
	mux.HandleFunc("GET /v1/env-groups/{id}/env-vars/{key}", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.GetEnvGroupVar(r.Context(), r.PathValue("id"), r.PathValue("key"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("PUT /v1/env-groups/{id}/env-vars/{key}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		v, err := s.SetEnvGroupVar(r.Context(), r.PathValue("id"), r.PathValue("key"), req.Value)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("DELETE /v1/env-groups/{id}/env-vars/{key}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.DeleteEnvGroupVar(r.Context(), r.PathValue("id"), r.PathValue("key")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// Group secret files: upsert one, reveal one, delete one.
	mux.HandleFunc("PUT /v1/env-groups/{id}/secret-files/{name}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		f, err := s.SetEnvGroupFile(r.Context(), r.PathValue("id"), r.PathValue("name"), req.Content)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, f)
	})
	mux.HandleFunc("GET /v1/env-groups/{id}/secret-files/{name}", func(w http.ResponseWriter, r *http.Request) {
		f, err := s.GetEnvGroupFile(r.Context(), r.PathValue("id"), r.PathValue("name"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, f)
	})
	mux.HandleFunc("DELETE /v1/env-groups/{id}/secret-files/{name}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.DeleteEnvGroupFile(r.Context(), r.PathValue("id"), r.PathValue("name")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// Link / unlink a service to the group.
	mux.HandleFunc("POST /v1/env-groups/{id}/services/{serviceId}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.LinkService(r.Context(), r.PathValue("id"), r.PathValue("serviceId")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /v1/env-groups/{id}/services/{serviceId}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.UnlinkService(r.Context(), r.PathValue("id"), r.PathValue("serviceId")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
