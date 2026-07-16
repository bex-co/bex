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
	"fmt"
	"net/http"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
)

// keyValueOwner is components.schemas.owner's resource wire subset.
type keyValueOwner struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Type  string `json:"type"`
}

// keyValueOptionsView is components.schemas.keyValueOptions — Render nests
// maxmemoryPolicy/persistenceMode under "options", not top-level.
type keyValueOptionsView struct {
	MaxmemoryPolicy string `json:"maxmemoryPolicy,omitempty"`
	PersistenceMode string `json:"persistenceMode,omitempty"`
}

// renderKeyValue is the Render-shaped wire response for a single KeyValue —
// every REST handler below returns this, not a bare KeyValueView. Verified
// against the render-oss/cli generated KeyValueDetail type: bex's flat
// ownerId/maxmemoryPolicy/persistenceMode (the shape GraphQL/MCP read directly
// off KeyValueView) silently zero-valued or failed to decode client-side
// because Render's real contract nests owner/options. The allow list is
// core.IPAllowListEntry — already Render's {cidrBlock, description} shape.
// Region/dashboardUrl are populated via resourcemeta, matching postgres/apps.
// Version is the one field genuinely omitted — bex doesn't track it.
type renderKeyValue struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	Plan          string                  `json:"plan"`
	Status        string                  `json:"status"`
	Suspended     string                  `json:"suspended"`
	CreatedAt     string                  `json:"createdAt,omitempty"`
	UpdatedAt     string                  `json:"updatedAt,omitempty"`
	Owner         *keyValueOwner          `json:"owner,omitempty"`
	Region        string                  `json:"region,omitempty"`
	DashboardURL  string                  `json:"dashboardUrl,omitempty"`
	Version       string                  `json:"version,omitempty"`
	Options       keyValueOptionsView     `json:"options"`
	IPAllowList   []core.IPAllowListEntry `json:"ipAllowList,omitempty"`
	EnvironmentID string                  `json:"environmentId,omitempty"`
	ExternalHost  string                  `json:"externalHost,omitempty"`
	Public        bool                    `json:"public"`
	ProjectID     string                  `json:"projectId,omitempty"`
}

func toRenderKeyValue(kv KeyValueView) renderKeyValue {
	return renderKeyValue{
		ID:        kv.ID,
		Name:      kv.Name,
		Plan:      kv.Plan,
		Status:    kv.Status,
		Suspended: kv.Suspended,
		CreatedAt: kv.CreatedAt,
		UpdatedAt: kv.UpdatedAt,
		Version:   kv.Version,
		Options: keyValueOptionsView{
			MaxmemoryPolicy: kv.MaxmemoryPolicy,
			PersistenceMode: kv.PersistenceMode,
		},
		IPAllowList:   kv.IPAllowList,
		EnvironmentID: kv.EnvironmentID,
		ExternalHost:  kv.ExternalHost,
		Public:        kv.Public,
		ProjectID:     kv.ProjectID,
	}
}

func (s *Service) renderKeyValues(ctx context.Context, kvs []KeyValueView) []renderKeyValue {
	ownerIDs := make([]string, 0, len(kvs))
	for _, kv := range kvs {
		ownerIDs = append(ownerIDs, kv.OwnerID)
	}
	owners := resourcemeta.ResolveOwners(ctx, s.Owners, ownerIDs)
	out := make([]renderKeyValue, 0, len(kvs))
	for _, kv := range kvs {
		rendered := toRenderKeyValue(kv)
		rendered.Region = s.Metadata.PlatformRegion()
		rendered.DashboardURL = s.Metadata.DashboardURL("keyvalue", kv.ID)
		if owner, ok := owners[kv.OwnerID]; ok && owner.Available() {
			rendered.Owner = &keyValueOwner{ID: owner.ID, Name: owner.Name, Email: owner.Email, Type: owner.Type}
		}
		out = append(out, rendered)
	}
	return out
}

func (s *Service) renderOneKeyValue(ctx context.Context, kv KeyValueView) renderKeyValue {
	return s.renderKeyValues(ctx, []KeyValueView{kv})[0]
}

// keyValueWithCursor is components.schemas.keyValueWithCursor — the list-item
// envelope GET /v1/key-value returns as a bare JSON array (cursor is a
// SIBLING of the keyValue object, not a wrapper member; the same shape
// postgresWithCursor/serviceWithCursor use), verified against the
// render-oss/cli generated client.
type keyValueWithCursor struct {
	KeyValue renderKeyValue `json:"keyValue"`
	Cursor   string         `json:"cursor"`
}

func (s *Service) toKeyValueList(ctx context.Context, kvs []KeyValueView) []keyValueWithCursor {
	rendered := s.renderKeyValues(ctx, kvs)
	out := make([]keyValueWithCursor, 0, len(kvs))
	for i, kv := range kvs {
		// cursor is opaque in Render; the KeyValue name/id is a stable, valid cursor.
		out = append(out, keyValueWithCursor{KeyValue: rendered[i], Cursor: kv.ID})
	}
	return out
}

// writeBadRequestBody answers a JSON-decode failure with Render's error
// contract ({id,message} alongside bex's {error}), matching core.WriteErr —
// a hand-written {"error": "..."} literal here would swallow the real reason
// into the official CLI's generic "unknown error" (docs/cli-compatibility-
// checklist.md RC1) exactly as it does for every other bex-api error path.
func writeBadRequestBody(w http.ResponseWriter, err error) {
	core.WriteErr(w, fmt.Errorf("%w: bad request body: %s", core.ErrBadRequest, err))
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
		// Omission preserves the original complete-list behavior; requested pages
		// use stable id order so a full walk has no gaps or duplicates.
		after, limit := core.PageParams(q)
		page := core.StablePage(out, after, limit, q.Has("cursor") || q.Has("limit"), func(kv KeyValueView) string { return kv.ID })
		core.WriteJSON(w, http.StatusOK, s.toKeyValueList(r.Context(), page)) // [{keyValue, cursor}, ...]
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		// CreateKeyValueRequest.IPAllowList decodes Render's {cidrBlock,
		// description} objects directly (and tolerates bare CIDR strings, the
		// pre-m24 lenient shape) — no wire wrapper needed since w4/m24.
		var req CreateKeyValueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeBadRequestBody(w, err)
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
			core.WriteJSON(w, http.StatusOK, s.renderOneKeyValue(r.Context(), kv)) // dry-run: 200 (nothing created, w2/m29)
			return
		}
		core.WriteJSON(w, http.StatusCreated, s.renderOneKeyValue(r.Context(), kv)) // Render: create => 201
	})
	mux.HandleFunc("GET "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		kv, err := s.GetKeyValue(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, s.renderOneKeyValue(r.Context(), kv))
	})
	mux.HandleFunc("PATCH "+base+"/{id}", s.handleUpdateKeyValue)
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
			core.WriteJSON(w, http.StatusAccepted, s.renderOneKeyValue(r.Context(), kv))
		})
	}
	lifecycle("/suspend", s.Suspend)
	lifecycle("/resume", s.Resume)

	// IP allowlist (Render's Networking control) — same GET/PUT pair the
	// postgres feature exposes at /v1/postgres/{id}/ip-allow-list. Not called
	// by the official CLI's KeyValue commands directly (`keyvalues update
	// --ip-allow-list` goes through PATCH instead — that flow isn't wired up
	// on bex's side yet, a separate gap from this REST wire-shape fix), but
	// {"cidrs": [...]} stays bex-native-plain in the response since nothing
	// Render-side depends on this endpoint's shape; descriptions travel
	// through the Render-shaped create/get/list. The PUT body's array elements
	// decode as either bare CIDR strings or {cidrBlock, description} objects
	// (core.IPAllowListEntry's union), so a client that writes back entries
	// keeps their descriptions; a string-only full replace clears them.
	mux.HandleFunc("GET "+base+"/{id}/ip-allow-list", func(w http.ResponseWriter, r *http.Request) {
		list, err := s.GetIPAllowList(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, map[string][]string{"cidrs": core.AllowListCIDRs(list)})
	})
	mux.HandleFunc("PUT "+base+"/{id}/ip-allow-list", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CIDRs []core.IPAllowListEntry `json:"cidrs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeBadRequestBody(w, err)
			return
		}
		kv, err := s.SetIPAllowList(r.Context(), r.PathValue("id"), req.CIDRs)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, s.renderOneKeyValue(r.Context(), kv))
	})
}

// handleUpdateKeyValue is PATCH /v1/key-value/{id} — pulled out of RegisterREST
// to keep that function's complexity down (mirrors postgres.handleUpdatePostgres).
// Render's KeyValuePATCHInput is all-pointer: an omitted field means "leave
// unchanged", not "clear". The prior handler decoded plan as a plain string and
// called SetPlan unconditionally, so `keyvalues update <name> --name <new>`
// (which sends no plan) 400'd with "plan must be one of ..." — the same class of
// bug the identical Postgres handler fixed. Routing to the item by its opaque
// red- id is what makes the official CLI's rename land on the right store.
func (s *Service) handleUpdateKeyValue(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   *string `json:"name,omitempty"`
		Plan   *string `json:"plan,omitempty"`
		DryRun bool    `json:"dryRun,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequestBody(w, err)
		return
	}
	id := r.PathValue("id")
	patch := KeyValuePatch{Name: req.Name, Plan: req.Plan}
	dryRun := req.DryRun || r.URL.Query().Get("dryRun") == "true"
	if dryRun {
		kv, err := s.PreviewUpdateKeyValue(r.Context(), id, patch)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, s.renderOneKeyValue(r.Context(), kv))
		return
	}
	kv, err := s.UpdateKeyValue(r.Context(), id, patch)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, s.renderOneKeyValue(r.Context(), kv))
}
