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

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type servicePlacementReader struct {
	rows  map[string]store.AppPlacement
	calls [][]string
	err   error
}

func (r *servicePlacementReader) GetAppPlacements(_ context.Context, ids []string) (map[string]store.AppPlacement, error) {
	r.calls = append(r.calls, slices.Clone(ids))
	if r.err != nil {
		return nil, r.err
	}
	out := map[string]store.AppPlacement{}
	for _, id := range ids {
		if row, ok := r.rows[id]; ok {
			out[id] = row
		}
	}
	return out, nil
}

func TestServicePlacementAcrossComposedSurfaces(t *testing.T) {
	ws := newFakeWSStore()
	owner := mustCreate(t, ws, "placement", store.PlanHobby, "client-1")
	projectA, projectB := id.New(id.Project), id.New(id.Project)
	envA, envB := id.New(id.Environment), id.New(id.Environment)
	reader := &servicePlacementReader{rows: map[string]store.AppPlacement{}}
	var objects []client.Object
	var services []*appv1alpha1.App
	for i := range 2 {
		app := ownedApp(id.New(id.Service), owner.ID)
		app.Namespace = owner.ID
		app.Labels[store.LabelManagedBy] = store.ManagedByValue
		app.Labels[store.LabelAppID] = app.Name
		if i == 0 {
			app.Labels[core.LabelProject] = projectA
			app.Labels[core.LabelEnvironment] = envA
		}
		objects = append(objects, app)
		services = append(services, app)
	}
	base := serverBase(t, ws)
	base.Client = fakeClient(objects...)
	h, srv := serverWith(t, base, Deps{WorkspaceStore: ws, AppPlacements: reader})
	cs := mcpSessionAs(t, srv, "client-1")
	for _, placement := range []struct{ name, project, environment string }{
		{"moved", projectB, envB}, {"environment-cleared", projectB, ""}, {"project-cleared", "", ""},
	} {
		t.Run(placement.name, func(t *testing.T) {
			for _, app := range services {
				reader.rows[app.Name] = store.AppPlacement{TenantID: owner.ID, ProjectID: placement.project, EnvironmentID: placement.environment}
			}
			assertPlacement := func(source string, object map[string]any) {
				t.Helper()
				project, _ := object["projectId"].(string)
				environment, _ := object["environmentId"].(string)
				if placement.environment == "" {
					if strings.HasPrefix(source, "REST") {
						if _, exists := object["environmentId"]; exists {
							t.Fatalf("REST cleared environmentId must be omitted: %#v", object)
						}
					}
					if strings.HasPrefix(source, "GraphQL") {
						if value, exists := object["environmentId"]; !exists || value != nil {
							t.Fatalf("GraphQL cleared environmentId must be null: %#v", object)
						}
					}
				}
				if project != placement.project || environment != placement.environment {
					t.Fatalf("%s project=%q environment=%q, want %q %q", source, project, environment, placement.project, placement.environment)
				}
			}
			response := do(t, h, http.MethodGet, "/v1/services/"+services[0].Name, testToken, "")
			if response.Code != http.StatusOK {
				t.Fatalf("REST get: %d %s", response.Code, response.Body.String())
			}
			assertPlacement("REST get", decodeObject(t, response.Body.Bytes()))
			reader.calls = nil
			response = do(t, h, http.MethodGet, "/v1/services?ownerId="+owner.ID, testToken, "")
			var listed []map[string]any
			if response.Code != http.StatusOK {
				t.Fatalf("REST list: %d %s", response.Code, response.Body.String())
			}
			if err := json.Unmarshal(response.Body.Bytes(), &listed); err != nil || len(listed) != 2 {
				t.Fatalf("REST list: %s err=%v", response.Body.String(), err)
			}
			for _, item := range listed {
				assertPlacement("REST list", item["service"].(map[string]any))
			}
			if len(reader.calls) != 1 || len(reader.calls[0]) != 2 {
				t.Fatalf("list should read one batch of 2, got %v", reader.calls)
			}
			data := gql(t, h, fmt.Sprintf(`{service(id:%q){id projectId environmentId}}`, services[0].Name))
			assertPlacement("GraphQL get", data["service"].(map[string]any))
			reader.calls = nil
			data = gql(t, h, fmt.Sprintf(`{services(ownerId:%q){id projectId environmentId}}`, owner.ID))
			gqlList := data["services"].([]any)
			if len(gqlList) != 2 {
				t.Fatalf("GraphQL list: %v", gqlList)
			}
			for _, item := range gqlList {
				assertPlacement("GraphQL list", item.(map[string]any))
			}
			if len(reader.calls) != 1 || len(reader.calls[0]) != 2 {
				t.Fatalf("GraphQL batch: %v", reader.calls)
			}
			assertPlacement("MCP get", callTool[map[string]any](t, cs, "get_service", map[string]any{"serviceId": services[0].Name}))
			reader.calls = nil
			mcpList := callTool[map[string][]map[string]any](t, cs, "list_services", map[string]any{"workspaceId": owner.ID})["services"]
			if len(mcpList) != 2 {
				t.Fatalf("MCP list: %v", mcpList)
			}
			for _, item := range mcpList {
				assertPlacement("MCP list", item)
			}
			if len(reader.calls) != 1 || len(reader.calls[0]) != 2 {
				t.Fatalf("MCP batch: %v", reader.calls)
			}
			for _, env := range []string{envA, envB} {
				response = do(t, h, http.MethodGet, "/v1/services?ownerId="+owner.ID+"&environmentId="+env, testToken, "")
				var filtered []map[string]any
				if response.Code != http.StatusOK {
					t.Fatalf("filtered list: %d %s", response.Code, response.Body.String())
				}
				if err := json.Unmarshal(response.Body.Bytes(), &filtered); err != nil {
					t.Fatal(err)
				}
				want := 0
				if env == placement.environment {
					want = 2
				}
				if len(filtered) != want {
					t.Fatalf("environment %q: got %d services, want %d", env, len(filtered), want)
				}
			}
			for _, app := range services {
				var actual appv1alpha1.App
				if err := base.Client.Get(t.Context(), client.ObjectKeyFromObject(app), &actual); err != nil {
					t.Fatal(err)
				}
				if actual.Labels[core.LabelProject] != app.Labels[core.LabelProject] || actual.Labels[core.LabelEnvironment] != app.Labels[core.LabelEnvironment] {
					t.Fatal("read mutated projected CR")
				}
			}
		})
	}
}

func TestServicePlacementMissingForeignAndUnavailable(t *testing.T) {
	for _, scenario := range []string{"missing", "foreign", "unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			ws := newFakeWSStore()
			owner := mustCreate(t, ws, "placement", store.PlanHobby, "client-1")
			app := ownedApp(id.New(id.Service), owner.ID)
			app.Namespace = owner.ID
			app.Labels[store.LabelManagedBy] = store.ManagedByValue
			app.Labels[store.LabelAppID] = app.Name
			reader := &servicePlacementReader{rows: map[string]store.AppPlacement{}}
			if scenario == "foreign" {
				reader.rows[app.Name] = store.AppPlacement{TenantID: id.New(id.Workspace), ProjectID: id.New(id.Project), EnvironmentID: id.New(id.Environment)}
			}
			if scenario == "unavailable" {
				reader.err = errors.New("placement lookup unavailable")
			}
			base := serverBase(t, ws)
			base.Client = fakeClient(app)
			h, srv := serverWith(t, base, Deps{WorkspaceStore: ws, AppPlacements: reader})
			response := do(t, h, http.MethodGet, "/v1/services/"+app.Name, testToken, "")
			if scenario == "unavailable" {
				if response.Code < 500 {
					t.Fatalf("store error swallowed: %d %s", response.Code, response.Body.String())
				}
			} else if scenario == "foreign" {
				if response.Code != http.StatusForbidden {
					t.Fatalf("foreign get: %d", response.Code)
				}
			} else if response.Code != http.StatusNotFound {
				t.Fatalf("missing/foreign get: %d %s", response.Code, response.Body.String())
			}
			response = do(t, h, http.MethodGet, "/v1/services?ownerId="+owner.ID, testToken, "")
			if scenario == "unavailable" {
				if response.Code < 500 {
					t.Fatalf("list store error swallowed: %d", response.Code)
				}
			} else if scenario == "foreign" {
				if response.Code != http.StatusForbidden {
					t.Fatalf("foreign list: %d", response.Code)
				}
			} else {
				var list []any
				if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil || response.Code != http.StatusOK || len(list) != 0 {
					t.Fatalf("missing/foreign list: %d %s err=%v", response.Code, response.Body.String(), err)
				}
			}
			for _, query := range []string{fmt.Sprintf(`{service(id:%q){id projectId environmentId}}`, app.Name), fmt.Sprintf(`{services(ownerId:%q){id projectId environmentId}}`, owner.ID)} {
				payload, _ := json.Marshal(map[string]any{"query": query})
				response = do(t, h, http.MethodPost, "/graphql", testToken, string(payload))
				body := decodeObject(t, response.Body.Bytes())
				isList := strings.HasPrefix(query, "{services(")
				errs, _ := body["errors"].([]any)
				if scenario != "missing" || !isList {
					if len(errs) == 0 {
						t.Fatalf("GraphQL swallowed placement failure: %s", response.Body.String())
					}
				} else {
					data := body["data"].(map[string]any)
					if len(errs) != 0 || len(data["services"].([]any)) != 0 {
						t.Fatalf("GraphQL missing list: %s", response.Body.String())
					}
				}
			}
			cs := mcpSessionAs(t, srv, "client-1")
			result, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "get_service", Arguments: map[string]any{"serviceId": app.Name}})
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError {
				t.Fatalf("MCP get leaked placement: %#v", result)
			}
			result, err = cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "list_services", Arguments: map[string]any{"workspaceId": owner.ID}})
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "missing" {
				if !result.IsError {
					t.Fatal("MCP list swallowed store error")
				}
			} else {
				encoded, _ := json.Marshal(result.StructuredContent)
				var body map[string][]any
				if err := json.Unmarshal(encoded, &body); err != nil || result.IsError || len(body["services"]) != 0 {
					t.Fatalf("MCP missing list: %s err=%v", encoded, err)
				}
			}
		})
	}
}

func TestServicePlacementLegacyFallbackAndAuthorization(t *testing.T) {
	for _, scenario := range []string{"storeless", "unmanaged", "denied"} {
		t.Run(scenario, func(t *testing.T) {
			ws := newFakeWSStore()
			owner := mustCreate(t, ws, "placement", store.PlanHobby, "client-1")
			app := ownedApp(id.New(id.Service), owner.ID)
			app.Namespace = owner.ID
			project, environment := id.New(id.Project), id.New(id.Environment)
			app.Labels[core.LabelProject] = project
			app.Labels[core.LabelEnvironment] = environment
			app.Labels[store.LabelAppID] = app.Name
			if scenario != "unmanaged" {
				app.Labels[store.LabelManagedBy] = store.ManagedByValue
			}
			reader := &servicePlacementReader{err: errors.New("placement must not be queried")}
			base := serverBase(t, ws)
			base.Client = fakeClient(app)
			deps := Deps{WorkspaceStore: ws}
			if scenario != "storeless" {
				deps.AppPlacements = reader
			}
			if scenario == "denied" {
				base.Authz = &fakeChecker{allow: false}
			}
			h, srv := serverWith(t, base, deps)
			for _, path := range []string{"/v1/services/" + app.Name, "/v1/services?ownerId=" + owner.ID} {
				response := do(t, h, http.MethodGet, path, testToken, "")
				if scenario == "denied" {
					if response.Code != http.StatusForbidden {
						t.Fatalf("denied %s: %d", path, response.Code)
					}
				} else if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), project) || !strings.Contains(response.Body.String(), environment) {
					t.Fatalf("fallback %s: %d %s", path, response.Code, response.Body.String())
				}
			}
			query := fmt.Sprintf(`{service(id:%q){projectId environmentId} services(ownerId:%q){projectId environmentId}}`, app.Name, owner.ID)
			payload, _ := json.Marshal(map[string]any{"query": query})
			response := do(t, h, http.MethodPost, "/graphql", testToken, string(payload))
			body := decodeObject(t, response.Body.Bytes())
			errs, _ := body["errors"].([]any)
			if scenario == "denied" {
				if len(errs) == 0 {
					t.Fatal("GraphQL denied request succeeded")
				}
			} else if len(errs) != 0 || !strings.Contains(response.Body.String(), project) || !strings.Contains(response.Body.String(), environment) {
				t.Fatalf("GraphQL fallback: %s", response.Body.String())
			}
			cs := mcpSessionAs(t, srv, "client-1")
			for _, tool := range []string{"get_service", "list_services"} {
				args := map[string]any{"workspaceId": owner.ID}
				if tool == "get_service" {
					args = map[string]any{"serviceId": app.Name}
				}
				result, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: tool, Arguments: args})
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "denied" {
					if !result.IsError {
						t.Fatalf("MCP %s denied request succeeded", tool)
					}
				} else {
					encoded, _ := json.Marshal(result.StructuredContent)
					if result.IsError || !strings.Contains(string(encoded), project) || !strings.Contains(string(encoded), environment) {
						t.Fatalf("MCP %s fallback: %s", tool, encoded)
					}
				}
			}
			if len(reader.calls) != 0 {
				t.Fatalf("%s queried placements: %v", scenario, reader.calls)
			}
		})
	}
}
