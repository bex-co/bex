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
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w9/106: Render's Terraform provider sends protectedStatus and
// networkIsolationEnabled for every environment it creates or updates,
// "unprotected" and false unless configured. Stating those defaults is not an
// access change, so a developer (every API key) may; restricting access, or
// lifting a restriction, stays an administrator's (round-5 finding 12).

// developerSurface drives one verb as a developer on one surface and reports
// the outcome: nil, core.ErrForbidden, or core.ErrBadRequest.
type developerSurface func(t *testing.T, svc *Service, args map[string]any) error

// restOutcome maps a REST status onto the sentinel the other surfaces return.
func restOutcome(t *testing.T, rec interface {
	Result() *http.Response
}, body string) error {
	t.Helper()
	switch code := rec.Result().StatusCode; code {
	case http.StatusOK, http.StatusCreated:
		return nil
	case http.StatusForbidden:
		return core.ErrForbidden
	case http.StatusBadRequest:
		return core.ErrBadRequest
	default:
		t.Fatalf("REST status %d: %s", code, body)
		return nil
	}
}

// toolOutcome maps an MCP tool result onto the same sentinels.
func toolOutcome(t *testing.T, res *mcp.CallToolResult, err error) error {
	t.Helper()
	if err != nil {
		t.Fatalf("MCP transport: %v", err)
	}
	if !res.IsError {
		return nil
	}
	raw, _ := json.Marshal(res.Content)
	switch {
	case strings.Contains(string(raw), core.ErrForbidden.Error()):
		return core.ErrForbidden
	case strings.Contains(string(raw), core.ErrBadRequest.Error()):
		return core.ErrBadRequest
	}
	t.Fatalf("MCP tool error: %s", raw)
	return nil
}

func developerService(t *testing.T) (*Service, *fakeStore, *http.ServeMux) {
	t.Helper()
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, _ := newServiceWithClient(st)
	svc.Authz = developerChecker{}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return svc, st, mux
}

func restBody(t *testing.T, args map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

var allowAll = []any{map[string]any{"cidrBlock": "0.0.0.0/0", "description": "everywhere"}}
var office = []any{map[string]any{"cidrBlock": "10.0.0.0/8", "description": "office"}}

func TestDeveloperCreatesAnEnvironmentThatStatesTheDefaults(t *testing.T) {
	surfaces := map[string]developerSurface{
		"REST": func(t *testing.T, svc *Service, args map[string]any) error {
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := doREST(t, mux, http.MethodPost, "/v1/environments", restBody(t, args))
			return restOutcome(t, rec, rec.Body.String())
		},
		"GraphQL": func(t *testing.T, svc *Service, args map[string]any) error {
			_, err := svc.GraphQLMutation()["createEnvironment"].Resolve(graphql.ResolveParams{Context: ctxAs("user-a"), Args: args})
			return err
		},
		"MCP": func(t *testing.T, svc *Service, args map[string]any) error {
			ctx := ctxAs("user-a")
			res, err := newMCPClient(t, ctx, svc).CallTool(ctx, &mcp.CallToolParams{Name: "create_environment", Arguments: args})
			return toolOutcome(t, res, err)
		},
	}
	cases := []struct {
		name string
		acl  map[string]any
		want error
	}{
		{"the provider's defaults", map[string]any{"protectedStatus": "unprotected", "networkIsolationEnabled": false}, nil},
		{"an allow-all list", map[string]any{"protectedStatus": "unprotected", "networkIsolationEnabled": false, "ipAllowList": allowAll}, nil},
		{"protected", map[string]any{"protectedStatus": "protected", "networkIsolationEnabled": false}, core.ErrForbidden},
		{"isolated", map[string]any{"protectedStatus": "unprotected", "networkIsolationEnabled": true}, core.ErrForbidden},
		{"a narrowing list", map[string]any{"ipAllowList": office}, core.ErrForbidden},
		{"deny-all", map[string]any{"ipAllowList": []any{}}, core.ErrForbidden},
	}
	for surface, create := range surfaces {
		for _, tc := range cases {
			t.Run(surface+"/"+tc.name, func(t *testing.T) {
				svc, st, _ := developerService(t)
				args := maps.Clone(tc.acl)
				args["name"], args["projectId"] = "prod", "prj-1"
				err := create(t, svc, args)
				if !errors.Is(err, tc.want) {
					t.Fatalf("create = %v, want %v", err, tc.want)
				}
				envs, _ := st.ListEnvironments(ctxAs("user-a"), "prj-1")
				if tc.want != nil {
					if len(envs) != 0 {
						t.Fatalf("a refused create left %+v", envs)
					}
					return
				}
				if len(envs) != 1 || envs[0].ProtectedStatus != ProtectedStatusUnprotected || envs[0].NetworkIsolationEnabled || aclBearing(envs[0]) {
					t.Fatalf("created %+v, want one unprotected, open environment", envs)
				}
			})
		}
	}
}

func TestDeveloperPatchesOnlyAnUnrestrictedEnvironmentWithUnrestrictedSettings(t *testing.T) {
	surfaces := map[string]func(t *testing.T, svc *Service, id string, args map[string]any) error{
		"REST": func(t *testing.T, svc *Service, id string, args map[string]any) error {
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := doREST(t, mux, http.MethodPatch, "/v1/environments/"+id, restBody(t, args))
			return restOutcome(t, rec, rec.Body.String())
		},
		"GraphQL": func(t *testing.T, svc *Service, id string, args map[string]any) error {
			args = maps.Clone(args)
			if list, ok := args["ipAllowList"]; ok { // GraphQL's object form
				delete(args, "ipAllowList")
				args["ipAllowListEntries"] = list
			}
			args["id"] = id
			_, err := svc.GraphQLMutation()["updateEnvironment"].Resolve(graphql.ResolveParams{Context: ctxAs("user-a"), Args: args})
			return err
		},
		"MCP": func(t *testing.T, svc *Service, id string, args map[string]any) error {
			args = maps.Clone(args)
			args["id"] = id
			ctx := ctxAs("user-a")
			res, err := newMCPClient(t, ctx, svc).CallTool(ctx, &mcp.CallToolParams{Name: "update_environment", Arguments: args})
			return toolOutcome(t, res, err)
		},
	}
	unrestricted := func(e *store.Environment) {}
	protected := func(e *store.Environment) { e.ProtectedStatus = ProtectedStatusProtected }
	narrowed := func(e *store.Environment) { e.IPAllowList = []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}} }
	// The provider's update: the name plus both settings, every time.
	providerUpdate := func(status string) map[string]any {
		return map[string]any{"name": "renamed", "protectedStatus": status, "networkIsolationEnabled": false}
	}
	cases := []struct {
		name  string
		start func(*store.Environment)
		patch map[string]any
		want  error
	}{
		{"the provider's update of a default environment", unrestricted, providerUpdate("unprotected"), nil},
		{"the provider resetting a removed allow list", unrestricted, map[string]any{"ipAllowList": allowAll}, nil},
		{"protecting", unrestricted, map[string]any{"name": "renamed", "protectedStatus": "protected"}, core.ErrForbidden},
		{"isolating", unrestricted, map[string]any{"name": "renamed", "networkIsolationEnabled": true}, core.ErrForbidden},
		{"narrowing the allow list", unrestricted, map[string]any{"name": "renamed", "ipAllowList": office}, core.ErrForbidden},
		{"unprotecting", protected, map[string]any{"name": "renamed", "protectedStatus": "unprotected"}, core.ErrForbidden},
		{"the provider's update of a protected environment", protected, providerUpdate("protected"), core.ErrForbidden},
		{"lifting a narrowed allow list", narrowed, map[string]any{"name": "renamed", "ipAllowList": allowAll}, core.ErrForbidden},
		{"an unknown status", unrestricted, map[string]any{"name": "renamed", "protectedStatus": "bogus"}, core.ErrBadRequest},
	}
	for surface, patch := range surfaces {
		for _, tc := range cases {
			t.Run(surface+"/"+tc.name, func(t *testing.T) {
				svc, st, _ := developerService(t)
				e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
				if err != nil {
					t.Fatal(err)
				}
				before, err := st.GetEnvironment(ctxAs("user-a"), e.ID)
				if err != nil {
					t.Fatal(err)
				}
				tc.start(&before)
				st.envs[before.ID] = before
				err = patch(t, svc, e.ID, tc.patch)
				if !errors.Is(err, tc.want) {
					t.Fatalf("patch = %v, want %v", err, tc.want)
				}
				after := st.envs[before.ID]
				if tc.want != nil {
					// A refused patch changes nothing, the name included.
					if fmt.Sprint(after) != fmt.Sprint(before) {
						t.Fatalf("a refused patch changed %+v to %+v", before, after)
					}
					return
				}
				if after.ProtectedStatus != ProtectedStatusUnprotected || after.NetworkIsolationEnabled || aclBearing(after) {
					t.Fatalf("patched %+v, want it unprotected and open", after)
				}
				if name, ok := tc.patch["name"]; ok && after.Name != name {
					t.Fatalf("name = %q, want %q", after.Name, name)
				}
				if _, ok := tc.patch["ipAllowList"]; ok && !slices.Equal(after.IPAllowList, []core.IPAllowListEntry{{CIDRBlock: "0.0.0.0/0", Description: "everywhere"}}) {
					t.Fatalf("ipAllowList = %+v, want the provider's allow-all rule", after.IPAllowList)
				}
			})
		}
	}
}
