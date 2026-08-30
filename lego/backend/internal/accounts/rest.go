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

package accounts

import (
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/users/deletion-preview", func(w http.ResponseWriter, r *http.Request) {
		preview, err := s.Preview(r.Context())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, preview)
	})
	mux.HandleFunc("DELETE /v1/users", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Confirmation string `json:"confirmation"`
		}
		if err := core.DecodeJSON(r, &request); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		result, err := s.Request(r.Context(), request.Confirmation)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusAccepted, result)
	})
}
