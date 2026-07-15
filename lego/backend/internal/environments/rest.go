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

package environments

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the environments REST fragment, mirroring internal/projects/
// rest.go's shape exactly: GET/POST /v1/environments, GET/PATCH/DELETE
// /v1/environments/{id}, PUT /v1/environments/{id}/service-links. Scoped by
// projectId (the project's own workspace scoping already gates authorization,
// so no separate ownerId param is needed here).

type renderCIDR struct {
	CIDRBlock   string `json:"cidrBlock"`
	Description string `json:"description"`
}

// cidrBlocks strips Render's {cidrBlock, description} objects down to the
// CIDR strings the neutral core carries (w4/017). bex stores only the CIDR —
// description is accepted on input and discarded, the same conscious
// divergence apps/postgres/keyvalue document (ADR018).
func cidrBlocks(in []renderCIDR) []string {
	out := make([]string, len(in))
	for i := range in {
		out[i] = in[i].CIDRBlock
	}
	return out
}

type renderEnvironment struct {
	ID                      string       `json:"id"`
	ProjectID               string       `json:"projectId"`
	Name                    string       `json:"name"`
	ServiceIDs              []string     `json:"serviceIds"`
	DatabaseIDs             []string     `json:"databaseIds"`
	KeyValueIDs             []string     `json:"keyValueIds"`
	DatabasesIDs            []string     `json:"databasesIds"`
	RedisIDs                []string     `json:"redisIds"`
	EnvGroupIDs             []string     `json:"envGroupIds"`
	ProtectedStatus         string       `json:"protectedStatus"`
	NetworkIsolationEnabled bool         `json:"networkIsolationEnabled"`
	IPAllowList             []renderCIDR `json:"ipAllowList"`
}

type environmentWithCursor struct {
	Environment renderEnvironment `json:"environment"`
	Cursor      string            `json:"cursor"`
}

func toRenderEnvironment(e EnvironmentView) renderEnvironment {
	cidrs := make([]renderCIDR, len(e.IPAllowList))
	for i := range e.IPAllowList {
		cidrs[i] = renderCIDR{CIDRBlock: e.IPAllowList[i]}
	}
	empty := func(in []string) []string {
		if in == nil {
			return []string{}
		}
		return in
	}
	return renderEnvironment{
		ID:                      e.ID,
		ProjectID:               e.ProjectID,
		Name:                    e.Name,
		ServiceIDs:              empty(e.ServiceIDs),
		DatabaseIDs:             empty(e.DatabaseIDs),
		KeyValueIDs:             empty(e.KeyValueIDs),
		DatabasesIDs:            empty(e.DatabaseIDs),
		RedisIDs:                empty(e.KeyValueIDs),
		EnvGroupIDs:             empty(e.EnvGroupIDs),
		ProtectedStatus:         e.ProtectedStatus,
		NetworkIsolationEnabled: e.NetworkIsolationEnabled,
		IPAllowList:             cidrs,
	}
}

// writeErr maps ErrEnvironmentsUnavailable to 503 before falling back to
// core.WriteErr: that shared mapper only recognizes error sentinels declared
// in package core (a leaf core cannot import a feature package to add this
// one directly, matching every other "…Unavailable" convention in the
// codebase — e.g. core.ErrWorkspacesUnavailable, core.ErrEventsUnavailable),
// so a package-local sentinel like this one would otherwise fall through to
// core.WriteErr's default 500.
func writeErr(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrEnvironmentsUnavailable) {
		core.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error(), "message": err.Error(), "id": "unavailable"})
		return
	}
	core.WriteErr(w, err)
}

// RegisterREST mounts the environment CRUD endpoints.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/environments", func(w http.ResponseWriter, r *http.Request) {
		var projectIDs []string
		for _, raw := range r.URL.Query()["projectId"] {
			for _, id := range strings.Split(raw, ",") {
				if id = strings.TrimSpace(id); id != "" {
					projectIDs = append(projectIDs, id)
				}
			}
		}
		if len(projectIDs) == 0 {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		var out []environmentWithCursor
		for _, projectID := range projectIDs {
			es, err := s.List(r.Context(), projectID)
			if err != nil {
				writeErr(w, err)
				return
			}
			for _, e := range es {
				out = append(out, environmentWithCursor{Environment: toRenderEnvironment(e), Cursor: e.ID})
			}
		}
		// Render's list schema is [{environment, cursor}], not a flat array.
		// The official CLI unwraps the environment member and otherwise decodes
		// every flat object as an all-zero Environment.
		after, limit := core.PageParams(r.URL.Query())
		out = core.Page(out, after, limit, func(e environmentWithCursor) string { return e.Cursor })
		if out == nil {
			out = []environmentWithCursor{}
		}
		core.WriteJSON(w, http.StatusOK, out)
	})

	// POST /v1/environments takes Render's full create body (w4/017): name +
	// projectId plus the optional ACL triple — protectedStatus,
	// networkIsolationEnabled, and ipAllowList as Render's
	// [{cidrBlock, description}] objects (description accepted and discarded;
	// bex stores only the CIDR). The ACL is validated BEFORE the create so a
	// bad CIDR is a clean 400, never an orphaned environment, then applied
	// through the same SetACL verb the bex-native /acl route uses.
	mux.HandleFunc("POST /v1/environments", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name                    string       `json:"name"`
			ProjectID               string       `json:"projectId"`
			ProtectedStatus         string       `json:"protectedStatus"`
			NetworkIsolationEnabled bool         `json:"networkIsolationEnabled"`
			IPAllowList             []renderCIDR `json:"ipAllowList"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" || req.ProjectID == "" {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		// A fresh environment defaults to unprotected/unisolated/open (the
		// store's own defaults), so the SetACL round-trip only runs when the
		// caller named an ACL field.
		hasACL := req.ProtectedStatus != "" || req.NetworkIsolationEnabled || len(req.IPAllowList) > 0
		if req.ProtectedStatus == "" {
			req.ProtectedStatus = ProtectedStatusUnprotected
		}
		if hasACL {
			if err := validateACL(req.ProtectedStatus, cidrBlocks(req.IPAllowList)); err != nil {
				writeErr(w, err)
				return
			}
		}
		e, err := s.Create(r.Context(), req.ProjectID, strings.TrimSpace(req.Name))
		if err != nil {
			writeErr(w, err)
			return
		}
		if hasACL {
			if e, err = s.SetACL(r.Context(), e.ID, req.ProtectedStatus, req.NetworkIsolationEnabled, cidrBlocks(req.IPAllowList)); err != nil {
				writeErr(w, err)
				return
			}
		}
		core.WriteJSON(w, http.StatusCreated, toRenderEnvironment(e))
	})

	mux.HandleFunc("GET /v1/environments/{id}", func(w http.ResponseWriter, r *http.Request) {
		e, err := s.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderEnvironment(e))
	})

	// PATCH /v1/environments/{id} takes Render's full update body (w4/017):
	// every field optional, absent fields untouched — name renames; the ACL
	// fields merge into the current triple (SetACL is full-replace by design,
	// so the handler reads the current values first) and flow through the same
	// SetACL verb the bex-native /acl route uses. ipAllowList arrives as
	// Render's [{cidrBlock, description}] objects (description accepted and
	// discarded). Pointer fields distinguish "absent" from a zero value —
	// networkIsolationEnabled:false must be appliable.
	mux.HandleFunc("PATCH /v1/environments/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name                    *string       `json:"name"`
			ProtectedStatus         *string       `json:"protectedStatus"`
			NetworkIsolationEnabled *bool         `json:"networkIsolationEnabled"`
			IPAllowList             *[]renderCIDR `json:"ipAllowList"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		hasACL := req.ProtectedStatus != nil || req.NetworkIsolationEnabled != nil || req.IPAllowList != nil
		if req.Name == nil && !hasACL {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		var e EnvironmentView
		var err error
		if req.Name != nil {
			if e, err = s.Rename(r.Context(), r.PathValue("id"), strings.TrimSpace(*req.Name)); err != nil {
				writeErr(w, err)
				return
			}
		}
		if hasACL {
			cur, err := s.Get(r.Context(), r.PathValue("id"))
			if err != nil {
				writeErr(w, err)
				return
			}
			status, isolated, cidrs := cur.ProtectedStatus, cur.NetworkIsolationEnabled, cur.IPAllowList
			if status == "" { // pre-ACL-migration rows surface as empty
				status = ProtectedStatusUnprotected
			}
			if req.ProtectedStatus != nil {
				status = *req.ProtectedStatus
			}
			if req.NetworkIsolationEnabled != nil {
				isolated = *req.NetworkIsolationEnabled
			}
			if req.IPAllowList != nil {
				cidrs = cidrBlocks(*req.IPAllowList)
			}
			if e, err = s.SetACL(r.Context(), r.PathValue("id"), status, isolated, cidrs); err != nil {
				writeErr(w, err)
				return
			}
		}
		core.WriteJSON(w, http.StatusOK, toRenderEnvironment(e))
	})

	mux.HandleFunc("DELETE /v1/environments/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Delete(r.Context(), r.PathValue("id")); err != nil {
			writeErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// PUT /v1/environments/{id}/service-links replaces the full list of
	// services in an environment. Body: {"serviceIds": ["name1", "name2"]}
	// where serviceIds are App CR names (same as projects' service-links).
	mux.HandleFunc("PUT /v1/environments/{id}/service-links", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ServiceIDs []string `json:"serviceIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		if req.ServiceIDs == nil {
			req.ServiceIDs = []string{}
		}
		e, err := s.SetServices(r.Context(), r.PathValue("id"), req.ServiceIDs)
		if err != nil {
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderEnvironment(e))
	})

	// PUT /v1/environments/{id}/database-links replaces the full list of
	// managed Postgres databases in an environment. Body:
	// {"databaseIds": ["name1", "name2"]} where databaseIds are Database CR
	// names (w6/m20 extension, mirroring projects' database-links).
	mux.HandleFunc("PUT /v1/environments/{id}/database-links", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			DatabaseIDs []string `json:"databaseIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		if req.DatabaseIDs == nil {
			req.DatabaseIDs = []string{}
		}
		e, err := s.SetDatabases(r.Context(), r.PathValue("id"), req.DatabaseIDs)
		if err != nil {
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderEnvironment(e))
	})

	// PUT /v1/environments/{id}/keyvalue-links is database-links' KeyValue-CR
	// counterpart. Body: {"keyValueIds": ["name1", "name2"]}.
	mux.HandleFunc("PUT /v1/environments/{id}/keyvalue-links", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			KeyValueIDs []string `json:"keyValueIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		if req.KeyValueIDs == nil {
			req.KeyValueIDs = []string{}
		}
		e, err := s.SetKeyValues(r.Context(), r.PathValue("id"), req.KeyValueIDs)
		if err != nil {
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderEnvironment(e))
	})

	// PUT /v1/environments/{id}/env-group-links replaces the full list of
	// environment groups assigned to the Environment. Body:
	// {"envGroupIds": ["evg-...", ...]}.
	mux.HandleFunc("PUT /v1/environments/{id}/env-group-links", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			EnvGroupIDs []string `json:"envGroupIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		if req.EnvGroupIDs == nil {
			req.EnvGroupIDs = []string{}
		}
		e, err := s.SetEnvGroups(r.Context(), r.PathValue("id"), req.EnvGroupIDs)
		if err != nil {
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderEnvironment(e))
	})

	// PATCH /v1/environments/{id}/acl replaces the full protected-environment
	// ACL triple (w6/m19: protectedStatus/networkIsolationEnabled/ipAllowList
	// — Render parity, see SetACL's doc comment for why it's full-replace).
	// Body: {"protectedStatus": "protected"|"unprotected",
	// "networkIsolationEnabled": bool, "ipAllowList": ["1.2.3.0/24", ...]}.
	mux.HandleFunc("PATCH /v1/environments/{id}/acl", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ProtectedStatus         string   `json:"protectedStatus"`
			NetworkIsolationEnabled bool     `json:"networkIsolationEnabled"`
			IPAllowList             []string `json:"ipAllowList"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		e, err := s.SetACL(r.Context(), r.PathValue("id"), req.ProtectedStatus, req.NetworkIsolationEnabled, req.IPAllowList)
		if err != nil {
			writeErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderEnvironment(e))
	})
}
