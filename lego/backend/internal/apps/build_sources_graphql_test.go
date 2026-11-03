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

package apps

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The GraphQL build source verbs reach the same service verbs as REST: a
// create, read, list, rename and delete round trip, with REST's refusals.
func TestBuildSourceGraphQLRoundTrip(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	do := func(q string) map[string]any {
		t.Helper()
		res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: q})
		if res.HasErrors() {
			t.Fatalf("%s: %v", q, res.Errors)
		}
		return res.Data.(map[string]any)
	}

	created := do(`mutation { createBuildSource(ownerId:"tea-a", name:"Mono", git:{repoUrl:"https://github.com/acme/mono", branch:"main", runtime:"node", buildCommand:"npm ci"}) {
		id name ownerId git { repoUrl branch runtime registryCredentialId } image { imageUrl } serviceLinks { id } } }`)["createBuildSource"].(map[string]any)
	id, _ := created["id"].(string)
	git, _ := created["git"].(map[string]any)
	if !strings.HasPrefix(id, "bds-") || created["name"] != "Mono" || git["runtime"] != "node" || git["registryCredentialId"] != nil || created["image"] != nil {
		t.Fatalf("created = %+v", created)
	}
	if got := do(`{ buildSource(id:"` + id + `") { id name } }`)["buildSource"].(map[string]any); got["id"] != id {
		t.Fatalf("buildSource = %+v", got)
	}
	if list := do(`{ buildSources(ownerId:"tea-a") { id cursor } }`)["buildSources"].([]any); len(list) != 1 || list[0].(map[string]any)["cursor"] != id {
		t.Fatalf("buildSources = %+v", list)
	}
	if list := do(`{ buildSources(ownerId:"tea-a", name:["Other"]) { id } }`)["buildSources"].([]any); len(list) != 0 {
		t.Fatalf("buildSources filtered by another name = %+v", list)
	}
	if list := do(`{ buildSources(ownerId:"tea-a", cursor:"` + id + `") { id } }`)["buildSources"].([]any); len(list) != 0 {
		t.Fatalf("buildSources after the last cursor = %+v", list)
	}
	if res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ buildSources(ownerId:"tea-a", createdAfter:"yesterday") { id } }`}); !res.HasErrors() {
		t.Fatal("a malformed createdAfter was accepted; REST refuses it")
	}
	if got := do(`mutation { updateBuildSource(id:"` + id + `", name:"Monorepo") { name } }`)["updateBuildSource"].(map[string]any); got["name"] != "Monorepo" {
		t.Fatalf("updateBuildSource = %+v", got)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `mutation { createBuildSource(ownerId:"tea-a", name:"Both", git:{repoUrl:"https://github.com/acme/x"}, image:{imageUrl:"nginx:1"}) { id } }`})
	if !res.HasErrors() || !strings.Contains(res.Errors[0].Message, "exactly one of git or image") {
		t.Fatalf("git and image together = %+v, want REST's refusal", res.Errors)
	}
	if ok := do(`mutation { deleteBuildSource(id:"` + id + `") }`)["deleteBuildSource"]; ok != true {
		t.Fatalf("deleteBuildSource = %v", ok)
	}
}

// The MCP build source tools reach the same verbs.
func TestBuildSourceMCPTools(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	server := mcp.NewServer(&mcp.Implementation{Name: "bds-test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	a, b := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, a, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "bds-test", Version: "0"}, nil).Connect(context.Background(), b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	call := func(tool string, args map[string]any) (map[string]any, bool) {
		t.Helper()
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		encoded, _ := json.Marshal(result.StructuredContent)
		_ = json.Unmarshal(encoded, &out)
		return out, result.IsError
	}

	created, failed := call("create_build_source", map[string]any{"ownerId": "tea-a", "name": "Mono",
		"git": map[string]any{"repoUrl": "https://github.com/acme/mono", "branch": "main", "runtime": "node", "buildCommand": "npm ci"}})
	id, _ := created["id"].(string)
	if failed || !strings.HasPrefix(id, "bds-") {
		t.Fatalf("create_build_source = %+v (error %v)", created, failed)
	}
	if got, failed := call("get_build_source", map[string]any{"buildSourceId": id}); failed || got["name"] != "Mono" {
		t.Fatalf("get_build_source = %+v (error %v)", got, failed)
	}
	if got, failed := call("list_build_sources", map[string]any{"ownerId": "tea-a"}); failed || len(got["buildSources"].([]any)) != 1 {
		t.Fatalf("list_build_sources = %+v (error %v)", got, failed)
	}
	if _, failed := call("create_build_source", map[string]any{"ownerId": "tea-a", "name": "NoBuild",
		"git": map[string]any{"repoUrl": "https://github.com/acme/mono", "runtime": "node"}}); !failed {
		t.Error("a native build source without a buildCommand was created; every build would fail")
	}
	if _, failed := call("create_build_source", map[string]any{"ownerId": "tea-a", "name": "Bad",
		"git": map[string]any{"repoUrl": "https://github.com/acme/mono", "runtime": "cobol"}}); !failed {
		t.Error("create_build_source with a runtime outside Render's enum succeeded")
	}
	if _, failed := call("get_build_source", map[string]any{"buildSourceId": "bds-c185th5c2rvvnhbfiltg"}); !failed {
		t.Error("get_build_source of an unknown source succeeded")
	}
	if got, failed := call("delete_build_source", map[string]any{"buildSourceId": id}); failed || got["deleted"] != true {
		t.Fatalf("delete_build_source = %+v (error %v)", got, failed)
	}
}

// GraphQL linkBuildSource is REST's PATCH buildSourceId: a native source
// needs a start command, and the service then reports its buildSourceId.
func TestBuildSourceGraphQLLink(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	source, err := svc.CreateBuildSource(ctx, monorepoSource)
	if err != nil {
		t.Fatal(err)
	}
	web, err := svc.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: "web", Type: appv1alpha1.TypeWebService, Image: "nginx:1"})
	if err != nil {
		t.Fatal(err)
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	link := func(args string) *graphql.Result {
		return graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `mutation { linkBuildSource(serviceId:"` + web.ID + `", buildSourceId:"` + source.ID + `"` + args + `) { buildSourceId repo imagePath } }`})
	}
	if res := link(""); !res.HasErrors() || !strings.Contains(res.Errors[0].Message, "startCommand") {
		t.Fatalf("a native source without a start command = %+v, want REST's refusal", res.Errors)
	}
	res := link(`, startCommand:"node server.js"`)
	if res.HasErrors() {
		t.Fatal(res.Errors)
	}
	got := res.Data.(map[string]any)["linkBuildSource"].(map[string]any)
	if got["buildSourceId"] != source.ID || got["imagePath"] != "" && got["imagePath"] != nil {
		t.Fatalf("linked service = %+v, want buildSourceId %s and no image of its own", got, source.ID)
	}
}
