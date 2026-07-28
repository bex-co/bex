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
	"context"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RegisterREST wires the Render-compatible `/v1/sandboxes*` surface so
// `render ea sandbox create/list/stop` work unmodified (ADR042 D2,
// docs/render-artifacts/ea-sandbox.md, cli-compatibility-checklist rows
// 238-241). Exec (`ea sandbox exec`, the two-step run-token + SSE stream) is a
// follow-up. ownerId is Render's workspace binding (query param).
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Plan     Plan   `json:"plan"`
			Template string `json:"template"`
		}
		// A missing/empty body is fine — template+plan fall back to defaults.
		_ = core.DecodeJSON(r, &body)
		sb, err := s.Create(r.Context(), CreateRequest{
			OwnerID:  r.URL.Query().Get("ownerId"),
			Template: body.Template,
			Plan:     body.Plan,
		})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, sb)
	})
	mux.HandleFunc("GET /v1/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		ctx := core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId"))
		out, err := s.List(ctx)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("GET /v1/sandboxes/{id}", func(w http.ResponseWriter, r *http.Request) {
		ctx := core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId"))
		sb, err := s.Get(ctx, r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, sb)
	})
	// Render CLI `stop` → terminate.
	mux.HandleFunc("POST /v1/sandboxes/{id}/terminate", func(w http.ResponseWriter, r *http.Request) {
		s.lifecycleREST(w, r, s.Terminate)
	})
	// bex lifecycle extensions (Render's suspended/resuming statuses).
	mux.HandleFunc("POST /v1/sandboxes/{id}/pause", func(w http.ResponseWriter, r *http.Request) {
		s.lifecycleREST(w, r, s.Suspend)
	})
	mux.HandleFunc("POST /v1/sandboxes/{id}/resume", func(w http.ResponseWriter, r *http.Request) {
		s.lifecycleREST(w, r, s.Resume)
	})
}

func (s *Service) lifecycleREST(w http.ResponseWriter, r *http.Request, op func(ctx context.Context, id string) error) {
	ctx := core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId"))
	if err := op(ctx, r.PathValue("id")); err != nil {
		core.WriteErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
