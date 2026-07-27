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

package billing

import (
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RegisterREST mounts the workspace-admin customer-billing onboarding API.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/billing", func(w http.ResponseWriter, r *http.Request) {
		status, err := s.Status(r.Context(), r.PathValue("workspaceId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, status)
	})
	mux.HandleFunc("POST /v1/workspaces/{workspaceId}/billing/checkout-session", func(w http.ResponseWriter, r *http.Request) {
		var req CheckoutRequest
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		session, err := s.Checkout(r.Context(), r.PathValue("workspaceId"), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, session)
	})
	mux.HandleFunc("POST /v1/workspaces/{workspaceId}/billing/portal-session", func(w http.ResponseWriter, r *http.Request) {
		var req PortalRequest
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		session, err := s.Portal(r.Context(), r.PathValue("workspaceId"), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, session)
	})
}
