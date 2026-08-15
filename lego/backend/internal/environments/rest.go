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
	return core.Filter(environments, func(e EnvironmentView) bool {
		return (len(names) == 0 || slices.Contains(names, e.Name)) &&
			(len(owners) == 0 || slices.Contains(owners, e.OwnerID)) &&
			(len(ids) == 0 || slices.Contains(ids, e.ID)) &&
			(createdBefore.IsZero() || e.CreatedAt.Before(createdBefore)) &&
			(createdAfter.IsZero() || e.CreatedAt.After(createdAfter))
	}), nil
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

// RegisterREST mounts the environment CRUD endpoints.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/environments", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		q := r.URL.Query()
		projectIDs := core.QueryList(q, "projectId")
		if len(projectIDs) == 0 {
			return nil, core.ErrBadRequest
		}
		var environments []EnvironmentView
		for _, projectID := range projectIDs {
			es, err := s.List(r.Context(), projectID)
			if err != nil {
				return nil, err
			}
			environments = append(environments, es...)
		}
		environments, err := filterEnvironmentList(environments, q)
		if err != nil {
			return nil, err
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
		return out, nil
	}))

	// POST /v1/environments takes Render's full create body (w4/017): name +
	// projectId plus the optional ACL triple — protectedStatus,
	// networkIsolationEnabled, and ipAllowList as Render's
	// [{cidrBlock, description}] objects (both fields persist, w4/m24). The
	// ACL is validated BEFORE the create so a bad CIDR is a clean 400, never
	// an orphaned environment, then applied through the same SetACL verb the
	// bex-native /acl route uses.
	mux.HandleFunc("POST /v1/environments", core.HandleMapped(http.StatusCreated, func(r *http.Request) (EnvironmentView, error) {
		req, err := core.DecodeBody[CreateEnvironmentRequest](r)
		if err != nil {
			return EnvironmentView{}, err
		}
		return s.CreateWithACL(r.Context(), req)
	}, toRenderEnvironment))

	mux.HandleFunc("GET /v1/environments/{id}", core.HandleMapped(http.StatusOK, func(r *http.Request) (EnvironmentView, error) {
		return s.Get(r.Context(), r.PathValue("id"))
	}, toRenderEnvironment))

	// PATCH /v1/environments/{id} takes Render's full update body (w4/017):
	// every field optional, absent fields untouched — name renames; the ACL
	// fields merge into the current triple and flow through the core Update
	// verb (w4/m30), which owns the merge + pre-migration default exactly
	// once (environments/service.go) so REST/GraphQL/MCP cannot drift.
	// ipAllowList arrives as Render's [{cidrBlock, description}] objects (both
	// fields persist, w4/m24). Pointer fields distinguish "absent" from a zero
	// value — networkIsolationEnabled:false must be appliable.
	mux.HandleFunc("PATCH /v1/environments/{id}", core.HandleMapped(http.StatusOK, func(r *http.Request) (EnvironmentView, error) {
		req, err := core.DecodeBody[struct {
			Name                    *string                  `json:"name"`
			ProtectedStatus         *string                  `json:"protectedStatus"`
			NetworkIsolationEnabled *bool                    `json:"networkIsolationEnabled"`
			IPAllowList             *[]core.IPAllowListEntry `json:"ipAllowList"`
		}](r)
		if err != nil {
			return EnvironmentView{}, err
		}
		return s.Update(r.Context(), r.PathValue("id"), EnvironmentPatch{
			Name:                    req.Name,
			ProtectedStatus:         req.ProtectedStatus,
			NetworkIsolationEnabled: req.NetworkIsolationEnabled,
			IPAllowList:             req.IPAllowList,
		})
	}, toRenderEnvironment))

	mux.HandleFunc("DELETE /v1/environments/{id}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.Delete(r.Context(), r.PathValue("id"))
	}))

	// The four link routes replace an environment's full membership list for
	// one resource kind; an absent array clears it. Legacy service names remain
	// accepted on service-links during the stable-id transition; databaseIds
	// and keyValueIds are CR names (w6/m20, mirroring projects' database-links).
	mux.HandleFunc("PUT /v1/environments/{id}/service-links", core.HandleLinks[core.ServiceLinks](s.SetServices, toRenderEnvironment))
	mux.HandleFunc("PUT /v1/environments/{id}/database-links", core.HandleLinks[core.DatabaseLinks](s.SetDatabases, toRenderEnvironment))
	mux.HandleFunc("PUT /v1/environments/{id}/keyvalue-links", core.HandleLinks[core.KeyValueLinks](s.SetKeyValues, toRenderEnvironment))
	mux.HandleFunc("PUT /v1/environments/{id}/env-group-links", core.HandleLinks[core.EnvGroupLinks](s.SetEnvGroups, toRenderEnvironment))

	// PATCH /v1/environments/{id}/acl replaces the full protected-environment
	// ACL triple (w6/m19: protectedStatus/networkIsolationEnabled/ipAllowList
	// — Render parity, see SetACL's doc comment for why it's full-replace).
	// Body: {"protectedStatus": "protected"|"unprotected",
	// "networkIsolationEnabled": bool, "ipAllowList": [...]}, where entries
	// are Render-shaped {cidrBlock, description} objects.
	mux.HandleFunc("PATCH /v1/environments/{id}/acl", core.HandleMapped(http.StatusOK, func(r *http.Request) (EnvironmentView, error) {
		req, err := core.DecodeBody[struct {
			ProtectedStatus         string                  `json:"protectedStatus"`
			NetworkIsolationEnabled bool                    `json:"networkIsolationEnabled"`
			IPAllowList             []core.IPAllowListEntry `json:"ipAllowList"`
		}](r)
		if err != nil {
			return EnvironmentView{}, err
		}
		return s.SetACL(r.Context(), r.PathValue("id"), req.ProtectedStatus, req.NetworkIsolationEnabled, req.IPAllowList)
	}, toRenderEnvironment))
}
