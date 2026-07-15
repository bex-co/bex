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

// ipAllowEntry is components.schemas.cidrBlockAndDescription — Render's
// ipAllowList wire shape is a list of {cidrBlock, description} objects, not
// bare CIDR strings. bex's core KeyValueView/CreateKeyValueRequest (shared
// with GraphQL/MCP, which set []string directly in Go, never via JSON decode)
// stays []string; only the REST wire boundary translates, same as
// toRenderKeyValue below. The description bex doesn't store is dropped on
// input and left empty on output — never fabricated.
type ipAllowEntry struct {
	CidrBlock   string `json:"cidrBlock"`
	Description string `json:"description"`
}

func toIPAllowList(cidrs []string) []ipAllowEntry {
	out := make([]ipAllowEntry, 0, len(cidrs))
	for _, c := range cidrs {
		out = append(out, ipAllowEntry{CidrBlock: c})
	}
	return out
}

func fromIPAllowList(entries []ipAllowEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.CidrBlock)
	}
	return out
}

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
// ownerId/maxmemoryPolicy/persistenceMode/ipAllowList-of-strings (the shape
// GraphQL/MCP read directly off KeyValueView) silently zero-valued or failed
// to decode client-side because Render's real contract nests owner/options
// and uses {cidrBlock,description} allow-list entries. Region/version are
// deliberately omitted rather than faked — bex doesn't track either.
type renderKeyValue struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Plan          string              `json:"plan"`
	Status        string              `json:"status"`
	Suspended     string              `json:"suspended"`
	CreatedAt     string              `json:"createdAt,omitempty"`
	UpdatedAt     string              `json:"updatedAt,omitempty"`
	Owner         *keyValueOwner      `json:"owner,omitempty"`
	Region        string              `json:"region,omitempty"`
	DashboardURL  string              `json:"dashboardUrl,omitempty"`
	Version       string              `json:"version,omitempty"`
	Options       keyValueOptionsView `json:"options"`
	IPAllowList   []ipAllowEntry      `json:"ipAllowList,omitempty"`
	EnvironmentID string              `json:"environmentId,omitempty"`
	ExternalHost  string              `json:"externalHost,omitempty"`
	Public        bool                `json:"public"`
	ProjectID     string              `json:"projectId,omitempty"`
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
		IPAllowList:   toIPAllowList(kv.IPAllowList),
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
		after, limit := core.PageParams(q)
		page := core.Page(out, after, limit, func(kv KeyValueView) string { return kv.ID })
		core.WriteJSON(w, http.StatusOK, s.toKeyValueList(r.Context(), page)) // [{keyValue, cursor}, ...]
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		var wire struct {
			CreateKeyValueRequest
			IPAllowList []ipAllowEntry `json:"ipAllowList,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
			writeBadRequestBody(w, err)
			return
		}
		req := wire.CreateKeyValueRequest
		req.IPAllowList = fromIPAllowList(wire.IPAllowList)
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
	mux.HandleFunc("PATCH "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Plan   string `json:"plan"`
			DryRun bool   `json:"dryRun,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeBadRequestBody(w, err)
			return
		}
		dryRun := req.DryRun || r.URL.Query().Get("dryRun") == "true"
		if dryRun {
			kv, err := s.PreviewSetPlan(r.Context(), r.PathValue("id"), req.Plan)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, http.StatusOK, s.renderOneKeyValue(r.Context(), kv))
			return
		}
		kv, err := s.SetPlan(r.Context(), r.PathValue("id"), req.Plan)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, s.renderOneKeyValue(r.Context(), kv))
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
	// {"cidrs": [...]} stays bex-native-plain here since nothing Render-side
	// depends on this specific endpoint's shape.
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
