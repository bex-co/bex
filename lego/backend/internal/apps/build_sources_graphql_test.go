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
	"errors"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
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

	created, failed := call("create_build_source", map[string]any{"name": "Mono",
		"git": map[string]any{"repoUrl": "https://github.com/acme/mono", "branch": "main", "runtime": "node", "buildCommand": "npm ci"}})
	id, _ := created["id"].(string)
	if failed || !strings.HasPrefix(id, "bds-") {
		t.Fatalf("create_build_source = %+v (error %v)", created, failed)
	}
	// The tool takes no ownerId: MCP names the workspace with workspaceId,
	// which the gateway moves into the context (w9/m180 production check).
	if _, failed := call("create_build_source", map[string]any{"ownerId": "tea-a", "name": "Owned",
		"git": map[string]any{"repoUrl": "https://github.com/acme/mono", "branch": "main", "runtime": "node", "buildCommand": "npm ci"}}); !failed {
		t.Error("create_build_source accepted ownerId, which the MCP gateway refuses")
	}
	if got, failed := call("get_build_source", map[string]any{"buildSourceId": id}); failed || got["name"] != "Mono" {
		t.Fatalf("get_build_source = %+v (error %v)", got, failed)
	}
	if got, failed := call("list_build_sources", map[string]any{"ownerId": "tea-a"}); failed || len(got["buildSources"].([]any)) != 1 {
		t.Fatalf("list_build_sources = %+v (error %v)", got, failed)
	}
	if _, failed := call("create_build_source", map[string]any{"name": "NoBuild",
		"git": map[string]any{"repoUrl": "https://github.com/acme/mono", "runtime": "node"}}); !failed {
		t.Error("a native build source without a buildCommand was created; every build would fail")
	}
	if _, failed := call("create_build_source", map[string]any{"name": "Bad",
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
	if res := link(`, dryRun:true`); !res.HasErrors() || !strings.Contains(res.Errors[0].Message, "startCommand") {
		t.Fatalf("a dryRun link still refuses bad input: got %+v", res.Errors)
	}
	if res := link(`, startCommand:"node server.js", dryRun:true`); res.HasErrors() || res.Data.(map[string]any)["linkBuildSource"].(map[string]any)["buildSourceId"] != nil {
		t.Fatalf("dryRun link = %+v %v, want the service unchanged", res.Data, res.Errors)
	}
	if got := getTenantApp(t, svc.Client, "tea-a", "web"); got.Spec.BuildSource != nil {
		t.Fatalf("a dryRun link wrote the link: %+v", got.Spec.BuildSource)
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

// TestBuildSourceGraphQLCreateService covers w2/m612 t001: GraphQL
// createService(buildSourceId, ...) enforces Render's mutual exclusion,
// git-source region match, native/Docker/image command rules, and static-site
// refusal identically to REST POST /v1/services.
func TestBuildSourceGraphQLCreateService(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	svc.Metadata = resourcemeta.Config{Region: "fsn1"} // default Render region = frankfurt

	nativeSrc, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{
		OwnerID: "tea-a", Name: "NativeMono",
		Git: &BuildSourceGit{RepoURL: "https://github.com/acme/mono", Branch: "main", Runtime: "node", BuildCommand: "npm ci", Region: "oregon"},
	})
	if err != nil {
		t.Fatal(err)
	}
	dockerSrc, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{
		OwnerID: "tea-a", Name: "DockerMono",
		Git: &BuildSourceGit{RepoURL: "https://github.com/acme/docker", Branch: "main", Runtime: "docker", DockerfilePath: "deploy/Dockerfile", DockerContext: "deploy"},
	})
	if err != nil {
		t.Fatal(err)
	}
	imageSrc, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{
		OwnerID: "tea-a", Name: "ImgSrc",
		Image: &BuildSourceImage{ImageURL: "nginx:1.27"},
	})
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
	exec := func(args string) *graphql.Result {
		t.Helper()
		return graphql.Do(graphql.Params{
			Schema:  schema,
			Context: ctx,
			RequestString: `mutation { createService(` + args + `) {
				id name type buildSourceId runtime buildCommand startCommand command dockerfilePath dockerContext repo imagePath region
			} }`,
		})
	}
	mustCreate := func(args string) map[string]any {
		t.Helper()
		res := exec(args)
		if res.HasErrors() {
			t.Fatalf("createService(%s): %v", args, res.Errors)
		}
		return res.Data.(map[string]any)["createService"].(map[string]any)
	}

	// 1. Native git Build Source across web_service, private_service, background_worker, cron_job.
	web := mustCreate(`ownerId:"tea-a", name:"gql-web", type:"web_service", region:"oregon", buildSourceId:"` + nativeSrc.ID + `", startCommand:"node web.js"`)
	if web["buildSourceId"] != nativeSrc.ID || web["runtime"] != "node" || web["buildCommand"] != "npm ci" || web["startCommand"] != "node web.js" || web["repo"] != "" {
		t.Fatalf("web = %+v", web)
	}
	pserv := mustCreate(`ownerId:"tea-a", name:"gql-pserv", type:"private_service", plan:"starter", region:"oregon", buildSourceId:"` + nativeSrc.ID + `", startCommand:"node internal.js"`)
	if pserv["buildSourceId"] != nativeSrc.ID || pserv["runtime"] != "node" || pserv["type"] != "private_service" {
		t.Fatalf("pserv = %+v", pserv)
	}
	worker := mustCreate(`ownerId:"tea-a", name:"gql-worker", type:"background_worker", plan:"starter", region:"oregon", buildSourceId:"` + nativeSrc.ID + `", startCommand:"node worker.js"`)
	if worker["buildSourceId"] != nativeSrc.ID || worker["runtime"] != "node" || worker["type"] != "background_worker" {
		t.Fatalf("worker = %+v", worker)
	}
	cron := mustCreate(`ownerId:"tea-a", name:"gql-cron", type:"cron_job", plan:"starter", schedule:"*/5 * * * *", region:"oregon", buildSourceId:"` + nativeSrc.ID + `", startCommand:"node job.js"`)
	if cron["buildSourceId"] != nativeSrc.ID || cron["runtime"] != "node" || cron["command"] != "node job.js" {
		t.Fatalf("cron = %+v", cron)
	}

	// 2. Docker git Build Source with and without dockerCommand.
	dweb := mustCreate(`ownerId:"tea-a", name:"gql-dweb", type:"web_service", buildSourceId:"` + dockerSrc.ID + `", dockerCommand:"./serve --port 8080"`)
	if dweb["buildSourceId"] != dockerSrc.ID || dweb["runtime"] != "docker" || dweb["dockerfilePath"] != "deploy/Dockerfile" || dweb["dockerContext"] != "deploy" || dweb["startCommand"] != "./serve --port 8080" {
		t.Fatalf("dweb = %+v", dweb)
	}
	dwebDefault := mustCreate(`ownerId:"tea-a", name:"gql-dweb-def", type:"web_service", buildSourceId:"` + dockerSrc.ID + `"`)
	if dwebDefault["buildSourceId"] != dockerSrc.ID || dwebDefault["runtime"] != "docker" || dwebDefault["startCommand"] != "" {
		t.Fatalf("dwebDefault = %+v", dwebDefault)
	}

	// 3. Image Build Source (region-independent) with optional dockerCommand and cron.
	iweb := mustCreate(`ownerId:"tea-a", name:"gql-iweb", type:"web_service", region:"singapore", buildSourceId:"` + imageSrc.ID + `", dockerCommand:"httpd -f -p 10000"`)
	if iweb["buildSourceId"] != imageSrc.ID || iweb["runtime"] != "image" || iweb["startCommand"] != "httpd -f -p 10000" {
		t.Fatalf("iweb = %+v", iweb)
	}
	icron := mustCreate(`ownerId:"tea-a", name:"gql-icron", type:"cron_job", plan:"starter", schedule:"0 * * * *", region:"singapore", buildSourceId:"` + imageSrc.ID + `", dockerCommand:"date"`)
	if icron["buildSourceId"] != imageSrc.ID || icron["runtime"] != "image" || icron["command"] != "date" {
		t.Fatalf("icron = %+v", icron)
	}

	// 4. Refusals: mutual exclusion, region mismatch, command rules, static_site.
	refusalCases := []struct {
		name     string
		args     string
		wantCode string
		wantSub  string
	}{
		{
			name:    "mutual exclusion repo",
			args:    `ownerId:"tea-a", name:"bad-repo", region:"oregon", buildSourceId:"` + nativeSrc.ID + `", repo:"https://github.com/acme/x", startCommand:"node x.js"`,
			wantSub: "repo cannot be set with buildSourceId",
		},
		{
			name:    "mutual exclusion image",
			args:    `ownerId:"tea-a", name:"bad-img", buildSourceId:"` + imageSrc.ID + `", image:"nginx:1"`,
			wantSub: "image cannot be set with buildSourceId",
		},
		{
			name:    "mutual exclusion runtime",
			args:    `ownerId:"tea-a", name:"bad-rt", region:"oregon", buildSourceId:"` + nativeSrc.ID + `", runtime:"node", startCommand:"node x.js"`,
			wantSub: "runtime cannot be set with buildSourceId",
		},
		{
			name:    "mutual exclusion branch",
			args:    `ownerId:"tea-a", name:"bad-br", region:"oregon", buildSourceId:"` + nativeSrc.ID + `", branch:"main", startCommand:"node x.js"`,
			wantSub: "branch cannot be set with buildSourceId",
		},
		{
			name:     "region mismatch explicit",
			args:     `ownerId:"tea-a", name:"bad-reg", region:"frankfurt", buildSourceId:"` + nativeSrc.ID + `", startCommand:"node x.js"`,
			wantCode: BuildSourceRegionMismatchCode,
		},
		{
			name:     "region mismatch default",
			args:     `ownerId:"tea-a", name:"bad-reg-def", buildSourceId:"` + nativeSrc.ID + `", startCommand:"node x.js"`,
			wantCode: BuildSourceRegionMismatchCode,
		},
		{
			name:     "native missing startCommand",
			args:     `ownerId:"tea-a", name:"bad-nostart", region:"oregon", buildSourceId:"` + nativeSrc.ID + `"`,
			wantCode: "BUILD_SOURCE_COMMAND_INCOMPATIBLE",
			wantSub:  "needs a startCommand",
		},
		{
			name:     "native forbids dockerCommand",
			args:     `ownerId:"tea-a", name:"bad-nativedocker", region:"oregon", buildSourceId:"` + nativeSrc.ID + `", dockerCommand:"node x.js"`,
			wantCode: "BUILD_SOURCE_COMMAND_INCOMPATIBLE",
			wantSub:  "takes startCommand, not dockerCommand",
		},
		{
			name:     "docker forbids startCommand",
			args:     `ownerId:"tea-a", name:"bad-dockerstart", buildSourceId:"` + dockerSrc.ID + `", startCommand:"./serve"`,
			wantCode: "BUILD_SOURCE_COMMAND_INCOMPATIBLE",
			wantSub:  "takes no startCommand",
		},
		{
			name:     "image forbids startCommand",
			args:     `ownerId:"tea-a", name:"bad-imgstart", buildSourceId:"` + imageSrc.ID + `", startCommand:"./serve"`,
			wantCode: "BUILD_SOURCE_COMMAND_INCOMPATIBLE",
			wantSub:  "takes no startCommand",
		},
		{
			name:    "both startCommand and dockerCommand refused",
			args:    `ownerId:"tea-a", name:"bad-bothcmd", buildSourceId:"` + dockerSrc.ID + `", startCommand:"a", dockerCommand:"b"`,
			wantSub: "cannot set both dockerCommand and startCommand",
		},
		{
			name:    "static_site refused",
			args:    `ownerId:"tea-a", name:"bad-static", type:"static_site", publishPath:"dist", buildSourceId:"` + dockerSrc.ID + `"`,
			wantSub: "a static_site cannot build from a build source",
		},
	}
	for _, tc := range refusalCases {
		t.Run(tc.name, func(t *testing.T) {
			res := exec(tc.args)
			if !res.HasErrors() {
				t.Fatalf("expected error, got data %+v", res.Data)
			}
			err := res.Errors[0]
			if tc.wantSub != "" && !strings.Contains(err.Message, tc.wantSub) {
				t.Errorf("error = %q, want substring %q", err.Message, tc.wantSub)
			}
			if tc.wantCode != "" {
				gotCode, _ := err.Extensions["code"].(string)
				if gotCode != tc.wantCode {
					var coded *core.CodedError
					if !errors.As(err.OriginalError(), &coded) || coded.Code != tc.wantCode {
						t.Errorf("error extensions.code = %q (orig %T: %v), want %s", gotCode, err.OriginalError(), err.OriginalError(), tc.wantCode)
					}
				}
			}
		})
	}
}

// TestBuildSourceMCPCreateTools covers w2/m612 t002: MCP create_web_service
// and create_cron_job accept buildSourceId (with runtime omitted), enforce
// Render's mutual exclusion, region match, and command rules identically to REST.
func TestBuildSourceMCPCreateTools(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	svc.Metadata = resourcemeta.Config{Region: "fsn1"}

	nativeSrc, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{
		OwnerID: "tea-a", Name: "NativeMono",
		Git: &BuildSourceGit{RepoURL: "https://github.com/acme/mono", Branch: "main", Runtime: "node", BuildCommand: "npm ci", Region: "oregon"},
	})
	if err != nil {
		t.Fatal(err)
	}
	dockerSrc, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{
		OwnerID: "tea-a", Name: "DockerMono",
		Git: &BuildSourceGit{RepoURL: "https://github.com/acme/docker", Branch: "main", Runtime: "docker", DockerfilePath: "Dockerfile"},
	})
	if err != nil {
		t.Fatal(err)
	}
	imageSrc, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{
		OwnerID: "tea-a", Name: "ImgSrc",
		Image: &BuildSourceImage{ImageURL: "nginx:1.27"},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "bds-create-test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	a, b := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, a, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "bds-create-test", Version: "0"}, nil).Connect(context.Background(), b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })

	call := func(tool string, args map[string]any) (map[string]any, string, bool) {
		t.Helper()
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		if result.StructuredContent != nil {
			encoded, _ := json.Marshal(result.StructuredContent)
			_ = json.Unmarshal(encoded, &out)
		}
		var text string
		for _, c := range result.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				text += tc.Text
			}
		}
		return out, text, result.IsError
	}

	// 1. Valid creates: native web/worker, docker web, image web, native cron, image cron.
	for _, tc := range []struct {
		name        string
		tool        string
		args        map[string]any
		wantSource  string
		wantRuntime string
	}{
		{
			name:        "native web",
			tool:        "create_web_service",
			args:        map[string]any{"name": "mcp-web", "region": "oregon", "buildSourceId": nativeSrc.ID, "startCommand": "node api.js"},
			wantSource:  nativeSrc.ID,
			wantRuntime: "node",
		},
		{
			name:        "docker web with dockerCommand",
			tool:        "create_web_service",
			args:        map[string]any{"name": "mcp-dweb", "buildSourceId": dockerSrc.ID, "dockerCommand": "./serve"},
			wantSource:  dockerSrc.ID,
			wantRuntime: "docker",
		},
		{
			name:        "image worker no command",
			tool:        "create_web_service",
			args:        map[string]any{"name": "mcp-iworker", "type": "background_worker", "plan": "starter", "region": "singapore", "buildSourceId": imageSrc.ID},
			wantSource:  imageSrc.ID,
			wantRuntime: "image",
		},
		{
			name:        "native cron with startCommand",
			tool:        "create_cron_job",
			args:        map[string]any{"name": "mcp-ncron", "schedule": "*/5 * * * *", "plan": "starter", "region": "oregon", "buildSourceId": nativeSrc.ID, "startCommand": "node job.js"},
			wantSource:  nativeSrc.ID,
			wantRuntime: "node",
		},
		{
			name:        "image cron with dockerCommand",
			tool:        "create_cron_job",
			args:        map[string]any{"name": "mcp-icron", "schedule": "0 * * * *", "plan": "starter", "region": "singapore", "buildSourceId": imageSrc.ID, "dockerCommand": "date"},
			wantSource:  imageSrc.ID,
			wantRuntime: "image",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, text, isErr := call(tc.tool, tc.args)
			if isErr {
				t.Fatalf("%s failed: %s", tc.tool, text)
			}
			s, _ := out["service"].(map[string]any)
			details, _ := s["serviceDetails"].(map[string]any)
			if s["buildSourceId"] != tc.wantSource || details["runtime"] != tc.wantRuntime {
				t.Fatalf("service = %+v, want buildSourceId=%s runtime=%s", s, tc.wantSource, tc.wantRuntime)
			}
		})
	}

	// 2. Refusals across both tools.
	for _, tc := range []struct {
		name    string
		tool    string
		args    map[string]any
		wantSub string
	}{
		{
			name:    "web mutual exclusion repo",
			tool:    "create_web_service",
			args:    map[string]any{"name": "bad-w1", "region": "oregon", "buildSourceId": nativeSrc.ID, "repo": "https://github.com/acme/x", "startCommand": "node x.js"},
			wantSub: "repo cannot be set with buildSourceId",
		},
		{
			name:    "web mutual exclusion runtime",
			tool:    "create_web_service",
			args:    map[string]any{"name": "bad-w2", "region": "oregon", "buildSourceId": nativeSrc.ID, "runtime": "node", "startCommand": "node x.js"},
			wantSub: "runtime cannot be set with buildSourceId",
		},
		{
			name:    "web region mismatch",
			tool:    "create_web_service",
			args:    map[string]any{"name": "bad-w3", "region": "frankfurt", "buildSourceId": nativeSrc.ID, "startCommand": "node x.js"},
			wantSub: BuildSourceRegionMismatchCode,
		},
		{
			name:    "web native missing startCommand",
			tool:    "create_web_service",
			args:    map[string]any{"name": "bad-w4", "region": "oregon", "buildSourceId": nativeSrc.ID},
			wantSub: "BUILD_SOURCE_COMMAND_INCOMPATIBLE",
		},
		{
			name:    "web native forbids dockerCommand",
			tool:    "create_web_service",
			args:    map[string]any{"name": "bad-w5", "region": "oregon", "buildSourceId": nativeSrc.ID, "dockerCommand": "node x.js"},
			wantSub: "BUILD_SOURCE_COMMAND_INCOMPATIBLE",
		},
		{
			name:    "web image forbids startCommand",
			tool:    "create_web_service",
			args:    map[string]any{"name": "bad-w6", "buildSourceId": imageSrc.ID, "startCommand": "httpd"},
			wantSub: "BUILD_SOURCE_COMMAND_INCOMPATIBLE",
		},
		{
			name:    "cron region mismatch",
			tool:    "create_cron_job",
			args:    map[string]any{"name": "bad-c1", "schedule": "*/5 * * * *", "plan": "starter", "region": "frankfurt", "buildSourceId": nativeSrc.ID, "startCommand": "node job.js"},
			wantSub: BuildSourceRegionMismatchCode,
		},
		{
			name:    "cron docker forbids startCommand",
			tool:    "create_cron_job",
			args:    map[string]any{"name": "bad-c2", "schedule": "*/5 * * * *", "plan": "starter", "buildSourceId": dockerSrc.ID, "startCommand": "./job"},
			wantSub: "BUILD_SOURCE_COMMAND_INCOMPATIBLE",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, text, isErr := call(tc.tool, tc.args)
			if !isErr {
				t.Fatalf("%s succeeded, want error containing %q", tc.tool, tc.wantSub)
			}
			if !strings.Contains(text, tc.wantSub) {
				t.Errorf("%s error = %q, want substring %q", tc.tool, text, tc.wantSub)
			}
		})
	}
}
