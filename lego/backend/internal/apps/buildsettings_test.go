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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func dockerRepoApp(name string) *appv1alpha1.App {
	a := repoApp(name, "https://github.com/x/mono", "main")
	a.Spec.Runtime = "docker"
	a.Spec.Builder = "dockerfile"
	return a
}

func TestSetDockerfilePathSetsClearsAndRestarts(t *testing.T) {
	svc, cl := newService(nil, dockerRepoApp("web"))

	view, err := svc.SetDockerfilePath(context.Background(), "web", "  docker/Dockerfile.prod  ")
	if err != nil {
		t.Fatalf("SetDockerfilePath: %v", err)
	}
	if view.DockerfilePath != "docker/Dockerfile.prod" {
		t.Fatalf("view dockerfilePath = %q", view.DockerfilePath)
	}
	app := getApp(t, cl, "web")
	if app.Spec.DockerfilePath != "docker/Dockerfile.prod" || app.Spec.RestartedAt == "" {
		t.Fatalf("spec after set = %#v", app.Spec)
	}

	if _, err := svc.SetDockerfilePath(context.Background(), "web", ""); err != nil {
		t.Fatalf("clear Dockerfile path: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.DockerfilePath; got != "" {
		t.Fatalf("cleared dockerfilePath = %q", got)
	}
}

func TestSetDockerfilePathRejectsInvalidAndInapplicableServices(t *testing.T) {
	native := repoApp("native", "https://github.com/x/native", "main")
	native.Spec.Runtime = "node"
	native.Spec.Builder = "native"
	legacy := repoApp("legacy", "https://github.com/x/legacy", "main")
	legacy.Spec.Builder = "dockerfile"
	svc, cl := newService(nil, dockerRepoApp("docker"), native, legacy, sampleApp("image"))

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "docker", path: "../Dockerfile"},
		{name: "docker", path: "/Dockerfile"},
		{name: "native", path: "Dockerfile"},
		{name: "image", path: "Dockerfile"},
	} {
		if _, err := svc.SetDockerfilePath(context.Background(), tc.name, tc.path); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("SetDockerfilePath(%q, %q) = %v, want bad request", tc.name, tc.path, err)
		}
	}
	if got := getApp(t, cl, "docker").Spec.DockerfilePath; got != "" {
		t.Fatalf("rejected write changed spec.dockerfilePath to %q", got)
	}
	if _, err := svc.SetDockerfilePath(context.Background(), "legacy", "docker/Dockerfile"); err != nil {
		t.Fatalf("legacy Dockerfile builder rejected: %v", err)
	}
}

func TestSetCommandsClearsNativeStartCommand(t *testing.T) {
	native := repoApp("native", "https://github.com/x/native", "main")
	native.Spec.Runtime = "node"
	native.Spec.Builder = "native"
	native.Spec.StartCommand = "npm start"
	svc, cl := newService(nil, native)
	empty := "  "

	if _, err := svc.SetCommands(context.Background(), "native", nil, &empty); err != nil {
		t.Fatalf("clear native start command: %v", err)
	}
	if got := getApp(t, cl, "native").Spec.StartCommand; got != "" {
		t.Fatalf("cleared startCommand = %q", got)
	}
}

func TestBuildSettingSettersRejectUnauthorizedCaller(t *testing.T) {
	app := dockerRepoApp("other")
	app.Labels = map[string]string{core.LabelTenant: "tea-b", core.LabelServiceName: "other"}
	svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"}, app)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})
	command := "bin/forbidden"

	// Round-7 F8: the by-name denial reports absence — a non-member probing
	// names learns nothing about foreign workspaces' Apps.
	if _, err := svc.SetCommands(ctx, "other", nil, &command); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("SetCommands by non-member = %v, want not found", err)
	}
	if _, err := svc.SetDockerfilePath(ctx, "other", "docker/Dockerfile"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("SetDockerfilePath by non-member = %v, want not found", err)
	}
	got := getApp(t, cl, "other")
	if got.Spec.StartCommand != "" || got.Spec.DockerfilePath != "" {
		t.Fatalf("denied setters mutated App spec: %#v", got.Spec)
	}
}

func TestRESTPatchBuildSettings(t *testing.T) {
	svc, cl := newService(nil, dockerRepoApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	body := `{"serviceDetails":{"envSpecificDetails":{"dockerCommand":"bin/server","dockerfilePath":"docker/Dockerfile.prod"}}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH build settings => %d: %s", rec.Code, rec.Body)
	}
	app := getApp(t, cl, "web")
	if app.Spec.StartCommand != "bin/server" || app.Spec.DockerfilePath != "docker/Dockerfile.prod" {
		t.Fatalf("PATCH did not persist build settings: %#v", app.Spec)
	}
}

func TestGraphQLBuildSettingsRoundTrip(t *testing.T) {
	svc, _ := newService(nil, dockerRepoApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	run := func(query string) map[string]any {
		t.Helper()
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
		if len(res.Errors) > 0 {
			t.Fatalf("GraphQL %s: %v", query, res.Errors)
		}
		return res.Data.(map[string]any)
	}

	start := run(`mutation { setStartCommand(id:"web", command:"bin/server") { startCommand } }`)["setStartCommand"].(map[string]any)
	if start["startCommand"] != "bin/server" {
		t.Fatalf("setStartCommand = %#v", start)
	}
	path := run(`mutation { setDockerfilePath(id:"web", dockerfilePath:"docker/Dockerfile.prod") { dockerfilePath } }`)["setDockerfilePath"].(map[string]any)
	if path["dockerfilePath"] != "docker/Dockerfile.prod" {
		t.Fatalf("setDockerfilePath = %#v", path)
	}
	read := run(`{ server(id:"web") { startCommand dockerfilePath } }`)["server"].(map[string]any)
	if read["startCommand"] != "bin/server" || read["dockerfilePath"] != "docker/Dockerfile.prod" {
		t.Fatalf("server read = %#v", read)
	}
	clearedStart := run(`mutation { setStartCommand(id:"web", command:"") { startCommand } }`)["setStartCommand"].(map[string]any)
	clearedPath := run(`mutation { setDockerfilePath(id:"web", dockerfilePath:"") { dockerfilePath } }`)["setDockerfilePath"].(map[string]any)
	if clearedStart["startCommand"] != "" || clearedPath["dockerfilePath"] != "" {
		t.Fatalf("GraphQL clear results: start=%#v path=%#v", clearedStart, clearedPath)
	}
}

func TestMCPBuildSettingsRoundTrip(t *testing.T) {
	svc, _ := newService(nil, dockerRepoApp("web"))
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v isError=%v", name, err, res != nil && res.IsError)
		}
		data, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		return out
	}

	// One field at a time, as the retired set_start_command/set_dockerfile_path
	// tools did: each present argument must reach the same verb it used to.
	start := call("update_service", map[string]any{"serviceId": "web", "startCommand": "bin/server"})
	startDetails, _ := start["serviceDetails"].(map[string]any)
	startEnvSpecific, _ := startDetails["envSpecificDetails"].(map[string]any)
	if startEnvSpecific["dockerCommand"] != "bin/server" {
		t.Fatalf("update_service startCommand = %#v", start)
	}
	path := call("update_service", map[string]any{"serviceId": "web", "dockerfilePath": "docker/Dockerfile.prod"})
	details, _ := path["serviceDetails"].(map[string]any)
	envSpecific, _ := details["envSpecificDetails"].(map[string]any)
	if envSpecific["dockerfilePath"] != "docker/Dockerfile.prod" {
		t.Fatalf("update_service dockerfilePath = %#v", path)
	}
	// …and the fold's own property: the second call left startCommand alone,
	// because an omitted argument is not a write. Before w1/m71 this could not
	// regress, since each field had its own tool.
	if envSpecific["dockerCommand"] != "bin/server" {
		t.Fatalf("update_service dockerfilePath cleared the untouched startCommand: %#v", envSpecific)
	}
}

// TestSetBuildCommandOnStaticSite verifies that SetCommands sets (and clears)
// spec.BuildCommand for a static_site repo-backed App and surfaces it in the
// AppView. This is the same SetCommands verb native-runtime services use; the
// difference is that static sites pass a nil startCommand pointer.
func TestSetBuildCommandOnStaticSite(t *testing.T) {
	a := repoApp("site", "https://github.com/x/site", "main")
	a.Spec.Type = "static_site"
	a.Spec.PublishPath = "dist"
	svc, cl := newService(nil, a)

	cmd := "npm run build"
	view, err := svc.SetCommands(context.Background(), "site", &cmd, nil)
	if err != nil {
		t.Fatalf("SetCommands: %v", err)
	}
	if view.BuildCommand != "npm run build" {
		t.Fatalf("AppView.BuildCommand = %q, want %q", view.BuildCommand, "npm run build")
	}
	if got := getApp(t, cl, "site").Spec.BuildCommand; got != "npm run build" {
		t.Fatalf("spec.BuildCommand after set = %q", got)
	}

	// Clear via empty string — should revert to empty (runtime default).
	empty := ""
	if _, err := svc.SetCommands(context.Background(), "site", &empty, nil); err != nil {
		t.Fatalf("clear BuildCommand: %v", err)
	}
	if got := getApp(t, cl, "site").Spec.BuildCommand; got != "" {
		t.Fatalf("spec.BuildCommand after clear = %q", got)
	}
}

// TestGraphQLSetBuildCommandRoundTrip verifies the setBuildCommand GraphQL
// mutation persists and is readable via the server query (w7/m41).
func TestGraphQLSetBuildCommandRoundTrip(t *testing.T) {
	a := repoApp("site", "https://github.com/x/site", "main")
	a.Spec.Type = "static_site"
	a.Spec.PublishPath = "dist"
	svc, _ := newService(nil, a)
	schema := mustSchema(t, svc)
	run := func(query string) map[string]any {
		t.Helper()
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
		if len(res.Errors) > 0 {
			t.Fatalf("GraphQL %s: %v", query, res.Errors)
		}
		return res.Data.(map[string]any)
	}

	mut := run(`mutation { setBuildCommand(id:"site", command:"npm run build") { buildCommand } }`)["setBuildCommand"].(map[string]any)
	if mut["buildCommand"] != "npm run build" {
		t.Fatalf("setBuildCommand response = %#v", mut)
	}
	read := run(`{ server(id:"site") { buildCommand } }`)["server"].(map[string]any)
	if read["buildCommand"] != "npm run build" {
		t.Fatalf("server.buildCommand after set = %#v", read)
	}
	cleared := run(`mutation { setBuildCommand(id:"site", command:"") { buildCommand } }`)["setBuildCommand"].(map[string]any)
	if cleared["buildCommand"] != "" {
		t.Fatalf("setBuildCommand clear = %#v", cleared)
	}
}

// TestCreateServiceWithBuildFilterGraphQL verifies that createService accepts a
// buildFilter for a static_site and surfaces it on the resulting App (w7/m41).
func TestCreateServiceWithBuildFilterGraphQL(t *testing.T) {
	svc, cl := newService(nil)
	schema := mustSchema(t, svc)

	const q = `mutation {
		createService(
			name: "mysite"
			type: "static_site"
			repo: "https://github.com/x/site"
			branch: "main"
			publishPath: "dist"
			buildFilter: {paths: ["src/**"], ignoredPaths: ["**/*.test.ts"]}
		) { id }
	}`
	ctx := context.Background()
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: q})
	if len(res.Errors) > 0 {
		t.Fatalf("createService: %v", res.Errors)
	}

	app := getApp(t, cl, "mysite")
	bf := app.Spec.BuildFilter
	if bf == nil || len(bf.Paths) != 1 || bf.Paths[0] != "src/**" {
		t.Fatalf("spec.BuildFilter.Paths = %#v", bf)
	}
	if bf == nil || len(bf.IgnoredPaths) != 1 || bf.IgnoredPaths[0] != "**/*.test.ts" {
		t.Fatalf("spec.BuildFilter.IgnoredPaths = %#v", bf)
	}
}
