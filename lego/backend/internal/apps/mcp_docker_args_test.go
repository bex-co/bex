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
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp_docker_args_test.go covers upstream render-mcp-server bc94f8d (#154):
// create_web_service and create_cron_job take dockerCommand/dockerContext,
// which apply when runtime is docker (w1/118). They land on the same spec
// fields REST's envSpecificDetails.dockerCommand/dockerContext reach
// (TestRenderDockerDetailsMapToDockerfileBuild), so an agent written against
// Render's MCP gets the build REST would have produced.

// dockerArgsMCP calls a tool and reports its tool-level error text instead of
// failing the test, so refusals can be asserted.
func dockerArgsMCP(t *testing.T, svc *Service) func(string, map[string]any) (bool, string) {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "docker-args-test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "docker-args-test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return func(name string, args map[string]any) (bool, string) {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var text strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				text.WriteString(tc.Text)
			}
		}
		return res.IsError, text.String()
	}
}

func TestMCPCreateWebServiceDockerArgsReachSpec(t *testing.T) {
	svc, cl := newService(nil)
	call := dockerArgsMCP(t, svc)

	isErr, msg := call("create_web_service", map[string]any{
		"name":           "web",
		"repo":           "https://github.com/x/web",
		"runtime":        "docker",
		"dockerfilePath": "docker/Dockerfile.prod",
		"dockerContext":  "services/web",
		"dockerCommand":  "bin/server",
	})
	if isErr {
		t.Fatalf("create_web_service with Dockerfile args refused: %s", msg)
	}
	spec := getApp(t, cl, "web").Spec
	// Same assertion shape as the REST twin: dockerCommand stands in for the
	// start command, dockerContext is its own spec field.
	if spec.Builder != "dockerfile" || spec.Runtime != "docker" ||
		spec.DockerContext != "services/web" || spec.DockerfilePath != "docker/Dockerfile.prod" ||
		spec.StartCommand != "bin/server" || spec.RootDir != "" {
		t.Fatalf("spec = %+v", spec)
	}
}

func TestMCPCreateCronJobDockerArgsReachSpec(t *testing.T) {
	svc, cl := newService(nil)
	call := dockerArgsMCP(t, svc)

	isErr, msg := call("create_cron_job", map[string]any{
		"name":          "nightly",
		"schedule":      "0 0 * * *",
		"repo":          "https://github.com/x/jobs",
		"runtime":       "docker",
		"dockerContext": "jobs/nightly",
		"dockerCommand": "bin/report",
	})
	if isErr {
		t.Fatalf("create_cron_job with Dockerfile args refused: %s", msg)
	}
	spec := getApp(t, cl, "nightly").Spec
	// REST's docker cron bridge: the run command is the dockerCommand.
	if spec.Command != "bin/report" || spec.DockerContext != "jobs/nightly" || spec.Builder != "dockerfile" {
		t.Fatalf("spec = %+v", spec)
	}
}

// TestMCPCreateDockerArgsApplyOnlyToDockerRuntime pins upstream's "Applies
// when runtime is 'docker'" (and REST's identical rule): on a native runtime
// both args are inert, so a call that succeeds against Render succeeds here
// with the native commands untouched.
func TestMCPCreateDockerArgsApplyOnlyToDockerRuntime(t *testing.T) {
	web := createWebServiceArgs{
		Name: "w", Repo: "https://github.com/x/w", Runtime: "node",
		BuildCommand: "npm ci", StartCommand: "npm start",
		DockerCommand: "bin/server", DockerContext: "services/w",
	}.toCreateRequest()
	if web.StartCommand != "npm start" || web.DockerContext != "" {
		t.Errorf("create_web_service native runtime picked up docker args: %+v", web)
	}

	cron := createCronJobArgs{
		Name: "c", Schedule: "0 0 * * *", Repo: "https://github.com/x/c", Runtime: "python",
		BuildCommand: "pip install -r requirements.txt", StartCommand: "python job.py",
		DockerCommand: "bin/report", DockerContext: "jobs/c",
	}.toCreateRequest()
	if cron.StartCommand != "python job.py" || cron.Command != "" || cron.DockerContext != "" {
		t.Errorf("create_cron_job native runtime picked up docker args: %+v", cron)
	}
}

// TestMCPCreateDockerArgsRefusals covers the validation REST and the Blueprint
// compiler already apply: dockerContext must be a safe repo-relative path, and
// dockerCommand cannot be sent alongside another spelling of the same command
// (the Blueprint's "cannot set both dockerCommand and startCommand").
func TestMCPCreateDockerArgsRefusals(t *testing.T) {
	cases := []struct {
		name, tool string
		args       map[string]any
		want       string
	}{
		{"web dockerContext escapes the repo", "create_web_service", map[string]any{
			"name": "web", "repo": "https://github.com/x/web", "runtime": "docker", "dockerContext": "../outside",
		}, "dockerContext must be a relative path"},
		{"cron dockerContext escapes the repo", "create_cron_job", map[string]any{
			"name": "job", "schedule": "0 0 * * *", "repo": "https://github.com/x/job", "runtime": "docker", "dockerContext": "/abs",
		}, "dockerContext must be a relative path"},
		{"web dockerCommand with startCommand", "create_web_service", map[string]any{
			"name": "web", "repo": "https://github.com/x/web", "runtime": "docker", "dockerCommand": "bin/a", "startCommand": "bin/b",
		}, "cannot set both dockerCommand and startCommand"},
		{"cron dockerCommand with command", "create_cron_job", map[string]any{
			"name": "job", "schedule": "0 0 * * *", "repo": "https://github.com/x/job", "runtime": "docker", "dockerCommand": "bin/a", "command": "bin/b",
		}, "cannot set both dockerCommand and command"},
		{"cron dockerCommand with startCommand", "create_cron_job", map[string]any{
			"name": "job", "schedule": "0 0 * * *", "repo": "https://github.com/x/job", "runtime": "docker", "dockerCommand": "bin/a", "startCommand": "bin/b",
		}, "cannot set both dockerCommand and startCommand"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil)
			call := dockerArgsMCP(t, svc)
			isErr, msg := call(tc.tool, tc.args)
			if !isErr || !strings.Contains(msg, tc.want) {
				t.Fatalf("isError=%v msg=%q, want refusal containing %q", isErr, msg, tc.want)
			}
			if n := countApps(t, cl); n != 0 {
				t.Fatalf("a refused create wrote %d App(s)", n)
			}
		})
	}
}
