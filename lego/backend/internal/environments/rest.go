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
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func filterEnvironmentList(environments []EnvironmentView, q url.Values) ([]EnvironmentView, error) {
	for _, key := range []string{"updatedBefore", "updatedAfter"} {
		if q.Has(key) {
			return nil, fmt.Errorf("%w: %s is unsupported because environments do not expose updatedAt", core.ErrBadRequest, key)
		}
	}
	createdBefore, err := core.QueryTime(q, "createdBefore")
	if err != nil {
		return nil, err
	}
	createdAfter, err := core.QueryTime(q, "createdAfter")
	if err != nil {
		return nil, err
	}
	names := core.QueryList(q, "name")
	owners := core.QueryList(q, "ownerId")
	ids := core.QueryList(q, "environmentId")
	out := make([]EnvironmentView, 0, len(environments))
	for _, environment := range environments {
		switch {
		case len(names) > 0 && !slices.Contains(names, environment.Name):
			continue
		case len(owners) > 0 && !slices.Contains(owners, environment.OwnerID):
			continue
		case len(ids) > 0 && !slices.Contains(ids, environment.ID):
			continue
		case !createdBefore.IsZero() && !environment.CreatedAt.Before(createdBefore):
			continue
		case !createdAfter.IsZero() && !environment.CreatedAt.After(createdAfter):
			continue
		}
		out = append(out, environment)
	}
	return out, nil
}

// rest.go is the environments REST fragment, mirroring internal/projects/
// rest.go's shape exactly: GET/POST /v1/environments, GET/PATCH/DELETE
// /v1/environments/{id}, PUT /v1/environments/{id}/service-links. Scoped by
// projectId (the project's own workspace scoping already gates authorization,
// so no separate ownerId param is needed here).

type renderEnvironment struct {
	ID                      string                  `json:"id"`
	ProjectID               string                  `json:"projectId"`
	Name                    string                  `json:"name"`
	ServiceIDs              []string                `json:"serviceIds"`
	DatabaseIDs             []string                `json:"databaseIds"`
	KeyValueIDs             []string                `json:"keyValueIds"`
	DatabasesIDs            []string                `json:"databasesIds"`
	RedisIDs                []string                `json:"redisIds"`
	EnvGroupIDs             []string                `json:"envGroupIds"`
	ProtectedStatus         string                  `json:"protectedStatus"`
	NetworkIsolationEnabled bool                    `json:"networkIsolationEnabled"`
	IPAllowList             []core.IPAllowListEntry `json:"ipAllowList"`
}

type environmentWithCursor struct {
	Environment renderEnvironment `json:"environment"`
	Cursor      string            `json:"cursor"`
}

func toRenderEnvironment(e EnvironmentView) renderEnvironment {
	allowList := e.IPAllowList
	if allowList == nil {
		allowList = []core.IPAllowListEntry{}
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
		IPAllowList:             allowList,
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
		core.WriteErrStatus(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	core.WriteErr(w, err)
}

// RegisterREST mounts the environment CRUD endpoints.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/environments", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		projectIDs := core.QueryList(q, "projectId")
		if len(projectIDs) == 0 {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		var environments []EnvironmentView
		for _, projectID := range projectIDs {
			es, err := s.List(r.Context(), projectID)
			if err != nil {
				writeErr(w, err)
				return
			}
			environments = append(environments, es...)
		}
		environments, err := filterEnvironmentList(environments, q)
		if err != nil {
			writeErr(w, err)
			return
		}
		out := make([]environmentWithCursor, 0, len(environments))
		for _, e := range environments {
			out = append(out, environmentWithCursor{Environment: toRenderEnvironment(e), Cursor: e.ID})
		}
		// Render's list schema is [{environment, cursor}], not a flat array.
		// The official CLI unwraps the environment member and otherwise decodes
		// every flat object as an all-zero Environment.
		after, limit := core.PageParams(q)
		out = core.Page(out, after, limit, func(e environmentWithCursor) string { return e.Cursor })
		if out == nil {
			out = []environmentWithCursor{}
		}
		core.WriteJSON(w, http.StatusOK, out)
	})

	// POST /v1/environments takes Render's full create body (w4/017): name +
	// projectId plus the optional ACL triple — protectedStatus,
	// networkIsolationEnabled, and ipAllowList as Render's
	// [{cidrBlock, description}] objects (both fields persist, w4/m24). The
	// ACL is validated BEFORE the create so a bad CIDR is a clean 400, never
	// an orphaned environment, then applied through the same SetACL verb the
	// bex-native /acl route uses.
	mux.HandleFunc("POST /v1/environments", func(w http.ResponseWriter, r *http.Request) {
		var req CreateEnvironmentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		e, err := s.CreateWithACL(r.Context(), req)
		if err != nil {
			writeErr(w, err)
			return
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
	// fields merge into the current triple and flow through the core Update
	// verb (w4/m30), which owns the merge + pre-migration default exactly
	// once (environments/service.go) so REST/GraphQL/MCP cannot drift.
	// ipAllowList arrives as Render's [{cidrBlock, description}] objects (both
	// fields persist, w4/m24). Pointer fields distinguish "absent" from a zero
	// value — networkIsolationEnabled:false must be appliable.
	mux.HandleFunc("PATCH /v1/environments/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name                    *string                  `json:"name"`
			ProtectedStatus         *string                  `json:"protectedStatus"`
			NetworkIsolationEnabled *bool                    `json:"networkIsolationEnabled"`
			IPAllowList             *[]core.IPAllowListEntry `json:"ipAllowList"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		e, err := s.Update(r.Context(), r.PathValue("id"), EnvironmentPatch{
			Name:                    req.Name,
			ProtectedStatus:         req.ProtectedStatus,
			NetworkIsolationEnabled: req.NetworkIsolationEnabled,
			IPAllowList:             req.IPAllowList,
		})
		if err != nil {
			writeErr(w, err)
			return
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
	// services in an environment. Body: {"serviceIds": ["srv-...", "srv-..."]};
	// legacy service names remain accepted during the stable-id transition.
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
	// "networkIsolationEnabled": bool, "ipAllowList": [...]}, where entries
	// are {cidrBlock, description} objects or bare CIDR strings (the
	// bex-native pre-m24 spelling — both decode, strings with an empty
	// description).
	mux.HandleFunc("PATCH /v1/environments/{id}/acl", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ProtectedStatus         string                  `json:"protectedStatus"`
			NetworkIsolationEnabled bool                    `json:"networkIsolationEnabled"`
			IPAllowList             []core.IPAllowListEntry `json:"ipAllowList"`
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
