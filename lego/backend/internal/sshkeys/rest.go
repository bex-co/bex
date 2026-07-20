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

package sshkeys

import (
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/ssh-keys", func(w http.ResponseWriter, r *http.Request) {
		keys, err := s.List(r.Context())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, keys)
	})
	mux.HandleFunc("POST /v1/ssh-keys", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name      string `json:"name"`
			PublicKey string `json:"publicKey"`
		}
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		key, err := s.Create(r.Context(), req.Name, req.PublicKey)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, key)
	})
	mux.HandleFunc("DELETE /v1/ssh-keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Delete(r.Context(), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
