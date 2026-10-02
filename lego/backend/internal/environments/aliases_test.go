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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	aliasStoredFirst  = "env-c185th5c2rvvnhbfilt0"
	aliasPublicFirst  = "evm-c185th5c2rvvnhbfilt0"
	aliasStoredSecond = "env-c185th5c2rvvnhbfilt1"
	aliasPublicSecond = "evm-c185th5c2rvvnhbfilt1"
	aliasStoredOther  = "env-c185th5c2rvvnhbfilt2"
	aliasPublicOther  = "evm-c185th5c2rvvnhbfilt2"
)

func environmentAliasFixture() (*Service, *fakeStore) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "project"})
	st.addProject(store.Project{ID: "prj-other", TenantID: "tea-other", Name: "other"})
	for i, row := range []store.Environment{
		{ID: aliasStoredFirst, ProjectID: "prj-1", TenantID: "tea-a", Name: "staging"},
		{ID: aliasStoredSecond, ProjectID: "prj-1", TenantID: "tea-a", Name: aliasPublicFirst},
		{ID: aliasStoredOther, ProjectID: "prj-other", TenantID: "tea-other", Name: "foreign"},
	} {
		row.CreatedAt = time.Unix(int64(i+1), 0)
		row.ProtectedStatus = core.ProtectedStatusUnprotected
		st.envs[row.ID] = row
	}
	svc, _ := newServiceWithClient(st)
	return svc, st
}

func TestEnvironmentAliasesREST(t *testing.T) {
	for _, input := range []string{aliasStoredFirst, aliasPublicFirst} {
		t.Run(input, func(t *testing.T) {
			svc, st := environmentAliasFixture()
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			response := doREST(t, mux, http.MethodGet, "/v1/environments/"+input, "")
			var found renderEnvironment
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &found) != nil || found.ID != aliasPublicFirst || found.ProjectID != "prj-1" {
				t.Fatalf("Get alias = %d %s", response.Code, response.Body.String())
			}

			for _, tc := range []struct {
				query string
				want  []string
			}{
				{"", []string{aliasPublicFirst, aliasPublicSecond}},
				{"&environmentId=" + input, []string{aliasPublicFirst}},
				{"&cursor=" + input + "&limit=1", []string{aliasPublicSecond}},
				{"&cursor=evm-c185th5c2rvvnhbfilt9", nil},
				{"&name=staging", []string{aliasPublicFirst}},
				{"&name=" + aliasPublicFirst, []string{aliasPublicSecond}},
				{"&name=" + aliasStoredFirst, nil},
			} {
				response := doREST(t, mux, http.MethodGet, "/v1/environments?projectId=prj-1"+tc.query, "")
				var page []environmentWithCursor
				if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &page) != nil {
					t.Fatalf("list %s = %d %s", tc.query, response.Code, response.Body.String())
				}
				var got []string
				for _, item := range page {
					if item.Cursor != item.Environment.ID {
						t.Fatalf("cursor %q differs from returned id %q", item.Cursor, item.Environment.ID)
					}
					got = append(got, item.Environment.ID)
				}
				if !slices.Equal(got, tc.want) {
					t.Fatalf("list %s IDs = %v, want %v", tc.query, got, tc.want)
				}
			}
			response = doREST(t, mux, http.MethodPatch, "/v1/environments/"+input, `{"name":"renamed","networkIsolationEnabled":true}`)
			var updated renderEnvironment
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &updated) != nil || updated.ID != aliasPublicFirst || updated.Name != "renamed" || !updated.NetworkIsolationEnabled {
				t.Fatalf("update alias = %d %s", response.Code, response.Body.String())
			}
			if row := st.envs[aliasStoredFirst]; row.ID != aliasStoredFirst || row.Name != "renamed" || !row.NetworkIsolationEnabled {
				t.Fatalf("update changed identity or missed durable fields: %+v", row)
			}
			response = doREST(t, mux, http.MethodDelete, "/v1/environments/"+input, "")
			if response.Code != http.StatusNoContent {
				t.Fatalf("delete alias = %d %s", response.Code, response.Body.String())
			}
			if _, remains := st.envs[aliasStoredFirst]; remains {
				t.Fatal("delete left the stored environment")
			}
		})
	}
}

func TestEnvironmentAliasesGraphQL(t *testing.T) {
	for _, input := range []string{aliasStoredFirst, aliasPublicFirst} {
		t.Run(input, func(t *testing.T) {
			svc, st := environmentAliasFixture()
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
				Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
			})
			if err != nil {
				t.Fatal(err)
			}
			query := func(query string) map[string]any {
				t.Helper()
				result := graphql.Do(graphql.Params{Schema: schema, Context: ctxAs("user-a"), RequestString: query})
				if len(result.Errors) != 0 {
					t.Fatalf("GraphQL %s: %v", query, result.Errors)
				}
				return result.Data.(map[string]any)
			}
			data := query(fmt.Sprintf(`{ environment(id:%q) { id projectId } environments(projectId:"prj-1") { id } workspaceEnvironments(ownerId:"tea-a") { id } }`, input))
			if found := data["environment"].(map[string]any); found["id"] != aliasPublicFirst || found["projectId"] != "prj-1" {
				t.Fatalf("Get alias = %v", found)
			}
			for _, list := range []string{"environments", "workspaceEnvironments"} {
				var got []string
				for _, item := range data[list].([]any) {
					got = append(got, item.(map[string]any)["id"].(string))
				}
				slices.Sort(got)
				if !slices.Equal(got, []string{aliasPublicFirst, aliasPublicSecond}) {
					t.Fatalf("%s IDs = %v", list, got)
				}
			}
			data = query(fmt.Sprintf(`{ environments(projectId:"prj-1", cursor:%q, limit:1) { id } workspaceEnvironments(ownerId:"tea-a", cursor:%q, limit:1) { id } }`, input, input))
			for _, list := range []string{"environments", "workspaceEnvironments"} {
				page := data[list].([]any)
				if len(page) != 1 || page[0].(map[string]any)["id"] != aliasPublicSecond {
					t.Fatalf("%s with cursor %s = %v", list, input, page)
				}
			}
			data = query(fmt.Sprintf(`mutation { renameEnvironment(id:%q, name:"renamed") { id name } updateEnvironment(id:%q, networkIsolationEnabled:true) { id networkIsolationEnabled } }`, input, input))
			if renamed := data["renameEnvironment"].(map[string]any); renamed["id"] != aliasPublicFirst || renamed["name"] != "renamed" {
				t.Fatalf("rename alias = %v", renamed)
			}
			if updated := data["updateEnvironment"].(map[string]any); updated["id"] != aliasPublicFirst || updated["networkIsolationEnabled"] != true {
				t.Fatalf("update alias = %v", updated)
			}
			data = query(fmt.Sprintf(`mutation { deleteEnvironment(id:%q) }`, input))
			if data["deleteEnvironment"] != aliasPublicFirst {
				t.Fatalf("delete returned %v, want canonical ID", data)
			}
			if _, remains := st.envs[aliasStoredFirst]; remains {
				t.Fatal("delete left the stored environment")
			}
		})
	}
}

func TestEnvironmentAliasesMCP(t *testing.T) {
	for _, input := range []string{aliasStoredFirst, aliasPublicFirst} {
		t.Run(input, func(t *testing.T) {
			svc, st := environmentAliasFixture()
			// In-memory MCP does not carry the client's request identity; the
			// authorization matrix below exercises the same service with authz on.
			svc.Authz = nil
			client := newMCPClient(t, t.Context(), svc)
			call := func(name string, args map[string]any) map[string]any {
				t.Helper()
				result, err := client.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
				if err != nil || result.IsError {
					t.Fatalf("%s: result=%+v err=%v", name, result, err)
				}
				encoded, err := json.Marshal(result.StructuredContent)
				if err != nil {
					t.Fatal(err)
				}
				var out map[string]any
				if err := json.Unmarshal(encoded, &out); err != nil {
					t.Fatal(err)
				}
				return out
			}
			found := call("get_environment", map[string]any{"id": input})
			if found["id"] != aliasPublicFirst || found["projectId"] != "prj-1" {
				t.Fatalf("Get alias = %v", found)
			}
			page := call("list_environments", map[string]any{"projectId": "prj-1", "limit": 1})["environments"].([]any)
			if len(page) != 1 || page[0].(map[string]any)["id"] != aliasPublicFirst {
				t.Fatalf("list IDs = %v", page)
			}
			page = call("list_environments", map[string]any{"projectId": "prj-1", "cursor": input, "limit": 1})["environments"].([]any)
			if len(page) != 1 || page[0].(map[string]any)["id"] != aliasPublicSecond {
				t.Fatalf("list with cursor %s = %v", input, page)
			}
			updated := call("update_environment", map[string]any{"id": input, "name": "renamed", "networkIsolationEnabled": true})
			if updated["id"] != aliasPublicFirst || updated["name"] != "renamed" || updated["networkIsolationEnabled"] != true {
				t.Fatalf("update alias = %v", updated)
			}
			deleted := call("delete_environment", map[string]any{"id": input})
			if deleted["id"] != aliasPublicFirst {
				t.Fatalf("delete returned %v, want canonical ID", deleted)
			}
			if _, remains := st.envs[aliasStoredFirst]; remains {
				t.Fatal("delete left the stored environment")
			}
		})
	}
}

func TestEnvironmentAliasesKeepAuthorizationAndLookupNeighbors(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  error
	}{
		{aliasStoredFirst, nil}, {aliasPublicFirst, nil},
		{aliasStoredOther, core.ErrForbidden}, {aliasPublicOther, core.ErrForbidden},
		{"env-c185th5c2rvvnhbfilt9", core.ErrNotFound}, {"evm-c185th5c2rvvnhbfilt9", core.ErrNotFound},
		{"evm-c185th5c2rvvnhbfilt", core.ErrNotFound}, {"evm-c185th5c2rvvnhbfilt00", core.ErrNotFound},
		{"EVM-c185th5c2rvvnhbfilt0", core.ErrNotFound}, {"evg-c185th5c2rvvnhbfilt0", core.ErrNotFound},
		{"staging", core.ErrNotFound},
	} {
		t.Run(tc.input, func(t *testing.T) {
			svc, st := environmentAliasFixture()
			svc.Authz = denyObjectChecker(core.WorkspaceObject("tea-other"))
			view, err := svc.Get(ctxAs("user-a"), tc.input)
			if !errors.Is(err, tc.want) {
				t.Fatalf("authenticated lookup = %v, want %v", err, tc.want)
			}
			if tc.want == nil && (view.ID != aliasPublicFirst || view.ProjectID != "prj-1") {
				t.Fatalf("alias selected a different identity: %+v", view)
			}
			// A missing identity is denied before lookup, whether the alias
			// names an existing, foreign, malformed, or absent environment.
			if _, err := svc.Get(context.Background(), tc.input); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("anonymous lookup = %v, want authorization refusal", err)
			}
			if _, err := svc.Rename(context.Background(), tc.input, "anonymous"); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("anonymous rename = %v", err)
			}
			if err := svc.Delete(context.Background(), tc.input); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("anonymous delete = %v", err)
			}
			if len(st.envs) != 3 || st.envs[aliasStoredFirst].Name != "staging" {
				t.Fatal("authorization refusal mutated stored environments")
			}
			assignment, err := NewCreateResolver(svc).ResolveForCreate(ctxAs("user-a"), tc.input, "tea-a")
			if !errors.Is(err, tc.want) {
				t.Fatalf("create placement lookup = %v, want %v", err, tc.want)
			}
			if tc.want == nil && (assignment.ID != aliasStoredFirst || assignment.ProjectID != "prj-1") {
				t.Fatalf("create placement lost stored identity: %+v", assignment)
			}
		})
	}
}
