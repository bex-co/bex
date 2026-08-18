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

package envgroups

import (
	"net/http"
	"net/url"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func envGroupListFilter(q url.Values) (EnvGroupListFilter, error) {
	filter := EnvGroupListFilter{
		Names: core.QueryList(q, "name"), OwnerIDs: core.QueryList(q, "ownerId"),
		EnvironmentIDs: core.QueryList(q, "environmentId"),
	}
	var err error
	if filter.CreatedBefore, err = core.QueryTime(q, "createdBefore"); err != nil {
		return EnvGroupListFilter{}, err
	}
	if filter.CreatedAfter, err = core.QueryTime(q, "createdAfter"); err != nil {
		return EnvGroupListFilter{}, err
	}
	if filter.UpdatedBefore, err = core.QueryTime(q, "updatedBefore"); err != nil {
		return EnvGroupListFilter{}, err
	}
	if filter.UpdatedAfter, err = core.QueryTime(q, "updatedAfter"); err != nil {
		return EnvGroupListFilter{}, err
	}
	return filter, nil
}

// envGroupWithCursor is Render's list envelope. Render's endpoint OpenAPI
// currently misdeclares the live response as []envGroupMeta, but its official
// pagination contract and live API return the cursor beside the resource.
type envGroupWithCursor struct {
	EnvGroup EnvGroupView `json:"envGroup"`
	Cursor   string       `json:"cursor"`
}

func envGroupList(groups []EnvGroupView) []envGroupWithCursor {
	out := make([]envGroupWithCursor, 0, len(groups))
	for _, group := range groups {
		out = append(out, envGroupWithCursor{EnvGroup: group, Cursor: group.ID})
	}
	return out
}

// rest.go is the env-groups REST fragment (Render's /v1/env-groups): group CRUD,
// its env vars + secret files, and service link/unlink. Behavior lives in the
// Service, so GraphQL and MCP stay identical.

// RegisterREST mounts the env-groups endpoints. Store unconfigured => the Service
// returns core.ErrSecretsUnavailable => 503 on these routes only.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/env-groups", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		q := r.URL.Query()
		filter, err := envGroupListFilter(q)
		if err != nil {
			return nil, err
		}
		out, err := s.ListEnvGroupsFiltered(r.Context(), filter)
		if err != nil {
			return nil, err
		}
		after, limit := core.PageParams(q)
		return envGroupList(pageEnvGroups(out, after, limit, true)), nil
	}))
	mux.HandleFunc("POST /v1/env-groups", core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[CreateEnvGroupRequest](r)
		if err != nil {
			return nil, err
		}
		return s.CreateEnvGroup(r.Context(), req) // Render: create => 201
	}))
	mux.HandleFunc("GET /v1/env-groups/{id}", core.HandleByID(s.GetEnvGroup))
	mux.HandleFunc("PATCH /v1/env-groups/{id}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[struct {
			Name string `json:"name"`
		}](r)
		if err != nil {
			return nil, err
		}
		return s.RenameEnvGroup(r.Context(), r.PathValue("id"), req.Name)
	}))
	mux.HandleFunc("DELETE /v1/env-groups/{id}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteEnvGroup(r.Context(), r.PathValue("id"))
	}))
	mux.HandleFunc("POST /v1/env-groups/{id}/clone", core.HandleJSON(http.StatusCreated, func(r *http.Request) (any, error) {
		request, err := core.DecodeBody[CloneEnvGroupRequest](r)
		if err != nil {
			return nil, err
		}
		return s.CloneEnvGroup(r.Context(), r.PathValue("id"), request)
	}))
	mux.HandleFunc("PATCH /v1/env-groups/{id}/contents", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		patch, err := core.DecodeBody[EnvironmentPatch](r)
		if err != nil {
			return nil, err
		}
		return s.PatchEnvironment(r.Context(), r.PathValue("id"), patch)
	}))
	mux.HandleFunc("PATCH /v1/env-groups/{id}/environment", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		request, err := core.DecodeBody[struct {
			EnvironmentID string `json:"environmentId"`
		}](r)
		if err != nil {
			return nil, err
		}
		return s.MoveEnvGroup(r.Context(), r.PathValue("id"), request.EnvironmentID)
	}))

	// Group env vars: replace-all plus Render's per-key reveal/upsert/delete.
	mux.HandleFunc("PUT /v1/env-groups/{id}/env-vars", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		in, err := core.DecodeBody[[]EnvVarView](r)
		if err != nil {
			return nil, err
		}
		return s.SetEnvGroupVars(r.Context(), r.PathValue("id"), in)
	}))
	mux.HandleFunc("GET /v1/env-groups/{id}/env-vars/{key}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.GetEnvGroupVar(r.Context(), r.PathValue("id"), r.PathValue("key"))
	}))
	mux.HandleFunc("PUT /v1/env-groups/{id}/env-vars/{key}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[struct {
			Value         *string `json:"value"`
			GenerateValue bool    `json:"generateValue"`
		}](r)
		if err != nil {
			return nil, err
		}
		value := ""
		if req.Value != nil {
			value = *req.Value
		}
		return s.SetEnvGroupVarInput(r.Context(), r.PathValue("id"), EnvVarView{
			Key: r.PathValue("key"), Value: value, ValueSet: req.Value != nil, GenerateValue: req.GenerateValue,
		})
	}))
	mux.HandleFunc("DELETE /v1/env-groups/{id}/env-vars/{key}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteEnvGroupVar(r.Context(), r.PathValue("id"), r.PathValue("key"))
	}))

	// Group secret files: upsert one, reveal one, delete one.
	mux.HandleFunc("PUT /v1/env-groups/{id}/secret-files/{name}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		req, err := core.DecodeBody[struct {
			Content string `json:"content"`
		}](r)
		if err != nil {
			return nil, err
		}
		return s.SetEnvGroupFile(r.Context(), r.PathValue("id"), r.PathValue("name"), req.Content)
	}))
	mux.HandleFunc("GET /v1/env-groups/{id}/secret-files/{name}", core.HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		return s.GetEnvGroupFile(r.Context(), r.PathValue("id"), r.PathValue("name"))
	}))
	mux.HandleFunc("DELETE /v1/env-groups/{id}/secret-files/{name}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteEnvGroupFile(r.Context(), r.PathValue("id"), r.PathValue("name"))
	}))

	// Link / unlink a service to the group.
	mux.HandleFunc("POST /v1/env-groups/{id}/services/{serviceId}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.LinkService(r.Context(), r.PathValue("id"), r.PathValue("serviceId"))
	}))
	mux.HandleFunc("DELETE /v1/env-groups/{id}/services/{serviceId}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.UnlinkService(r.Context(), r.PathValue("id"), r.PathValue("serviceId"))
	}))
}
