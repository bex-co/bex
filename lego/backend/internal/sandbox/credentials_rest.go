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

package sandbox

import (
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// registerCredentialREST mounts all 7 Render public API routes for sandbox
// group credentials (w2/m532, docs/ADR122-sandbox-credential-brokering.md).
func (s *Service) registerCredentialREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/sandbox-groups/{sandboxGroupId}/credentials", func(w http.ResponseWriter, r *http.Request) {
		creds, err := s.ListGroupCredentials(r.Context(), r.PathValue("sandboxGroupId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, creds)
	})
	mux.HandleFunc("POST /v1/sandbox-groups/{sandboxGroupId}/credentials", func(w http.ResponseWriter, r *http.Request) {
		var req CreateGroupCredentialRequest
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		cred, err := s.CreateGroupCredential(r.Context(), r.PathValue("sandboxGroupId"), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, cred)
	})
	mux.HandleFunc("GET /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}", func(w http.ResponseWriter, r *http.Request) {
		cred, err := s.GetGroupCredential(r.Context(), r.PathValue("sandboxGroupId"), r.PathValue("credentialId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, cred)
	})
	mux.HandleFunc("PATCH /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}", func(w http.ResponseWriter, r *http.Request) {
		var req UpdateGroupCredentialRequest
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		cred, err := s.UpdateGroupCredential(r.Context(), r.PathValue("sandboxGroupId"), r.PathValue("credentialId"), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, cred)
	})
	mux.HandleFunc("DELETE /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.DeleteGroupCredential(r.Context(), r.PathValue("sandboxGroupId"), r.PathValue("credentialId")); err != nil {
			core.WriteErr(w, err)
			return
		}
		// Render's delete-sandbox-group-credential returns 202 Accepted with an empty body.
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}/versions", func(w http.ResponseWriter, r *http.Request) {
		versions, err := s.ListGroupCredentialVersions(r.Context(), r.PathValue("sandboxGroupId"), r.PathValue("credentialId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, versions)
	})
	mux.HandleFunc("POST /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}/restore", func(w http.ResponseWriter, r *http.Request) {
		var req RestoreGroupCredentialRequest
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		cred, err := s.RestoreGroupCredential(r.Context(), r.PathValue("sandboxGroupId"), r.PathValue("credentialId"), req.VersionID)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, cred)
	})
}
