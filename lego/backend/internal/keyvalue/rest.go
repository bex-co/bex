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

package keyvalue

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// keyValueWithCursor is components.schemas.keyValueWithCursor — the list-item
// envelope GET /v1/key-value returns as a bare JSON array (cursor is a
// SIBLING of the keyValue object, not a wrapper member; the same shape
// postgresWithCursor/serviceWithCursor use), verified against the
// render-oss/cli generated client.
type keyValueWithCursor struct {
	KeyValue KeyValueView `json:"keyValue"`
	Cursor   string       `json:"cursor"`
}

func toKeyValueList(kvs []KeyValueView) []keyValueWithCursor {
	out := make([]keyValueWithCursor, 0, len(kvs))
	for _, kv := range kvs {
		// cursor is opaque in Render; the KeyValue name/id is a stable, valid cursor.
		out = append(out, keyValueWithCursor{KeyValue: kv, Cursor: kv.ID})
	}
	return out
}

// RegisterREST adds the managed key-value endpoints, Render-shaped
// (/v1/key-value), mirroring the postgres feature's /v1/postgres surface.
// delete => 204, create => 201 (Render conventions).
func (s *Service) RegisterREST(mux *http.ServeMux) {
	base := "/v1/key-value"

	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		out, err := s.ListKeyValues(r.Context(), q.Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		// name filters by exact name, OR'd across repeated ?name= values (Render's
		// documented "Filter by name" — the official CLI resolves a bare
		// name/id argument to a key-value id by calling this with ?name=, and
		// requires it to narrow to exactly one match).
		if names := q["name"]; len(names) > 0 {
			filtered := make([]KeyValueView, 0, len(out))
			for _, kv := range out {
				if slices.Contains(names, kv.Name) {
					filtered = append(filtered, kv)
				}
			}
			out = filtered
		}
		// Render's cursor-pagination envelope — a bare array breaks the official
		// CLI's list decode (ListKeyValueResponse.JSON200 is *[]KeyValueWithCursor).
		after, limit := core.PageParams(q)
		page := core.Page(out, after, limit, func(kv KeyValueView) string { return kv.ID })
		core.WriteJSON(w, http.StatusOK, toKeyValueList(page)) // [{keyValue, cursor}, ...]
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		var req CreateKeyValueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
			return
		}
		if !req.DryRun && r.URL.Query().Get("dryRun") == "true" {
			req.DryRun = true
		}
		kv, err := s.CreateKeyValue(r.Context(), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		if req.DryRun {
			core.WriteJSON(w, http.StatusOK, kv) // dry-run: 200 (nothing created, w2/m29)
			return
		}
		core.WriteJSON(w, http.StatusCreated, kv) // Render: create => 201
	})
	mux.HandleFunc("GET "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		kv, err := s.GetKeyValue(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, kv)
	})
	mux.HandleFunc("PATCH "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Plan   string `json:"plan"`
			DryRun bool   `json:"dryRun,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
			return
		}
		dryRun := req.DryRun || r.URL.Query().Get("dryRun") == "true"
		if dryRun {
			kv, err := s.PreviewSetPlan(r.Context(), r.PathValue("id"), req.Plan)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, kv)
			return
		}
		kv, err := s.SetPlan(r.Context(), r.PathValue("id"), req.Plan)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, kv)
	})
	mux.HandleFunc("DELETE "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.DeleteKeyValue(r.Context(), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent) // Render: delete => 204
	})
	mux.HandleFunc("GET "+base+"/{id}/connection-info", func(w http.ResponseWriter, r *http.Request) {
		info, err := s.KeyValueConnectionInfo(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, info)
	})

	// Lifecycle verbs return the updated object with 202 Accepted (the operator
	// converges asynchronously) — matching the App suspend/resume surface.
	lifecycle := func(path string, verb func(context.Context, string) (KeyValueView, error)) {
		mux.HandleFunc("POST "+base+"/{id}"+path, func(w http.ResponseWriter, r *http.Request) {
			kv, err := verb(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusAccepted, kv)
		})
	}
	lifecycle("/suspend", s.Suspend)
	lifecycle("/resume", s.Resume)

	// IP allowlist (Render's Networking control) — same GET/PUT pair the
	// postgres feature exposes at /v1/postgres/{id}/ip-allow-list.
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
		kv, err := s.SetIPAllowList(r.Context(), r.PathValue("id"), req.CIDRs)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, kv)
	})
}
