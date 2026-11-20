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
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/052. `bex services update <native service> --build-command ''` (or
// `--start-command ''`, or `--repo` switching an image service back to Git)
// answered success, and every later deploy then failed at dispatch with
// "native runtime node requires buildCommand". Create refuses that state up
// front; an update now refuses it with create's own answer and leaves the
// service as it was.

const w32052Refusal = "native runtime node requires buildCommand and startCommand"

// w32052Surface makes one update through a caller of the shared verbs and
// answers its error text ("" on success).
type w32052Surface func(t *testing.T, svc *Service, name string) string

// w32052REST PATCHes body as the pinned CLI sends it, checked against
// Render's schema.
func w32052REST(body string) w32052Surface {
	return func(t *testing.T, svc *Service, name string) string {
		t.Helper()
		rec := m461Do(t, svc, http.MethodPatch, "/v1/services/"+name, body)
		if rec.Code == http.StatusOK {
			return ""
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("PATCH %s = %d, want 200 or 400: %s", body, rec.Code, rec.Body)
		}
		return rec.Body.String()
	}
}

// w32052GraphQL runs mutation (with %s for the service id) as the dashboard
// does.
func w32052GraphQL(mutation string) w32052Surface {
	return func(t *testing.T, svc *Service, name string) string {
		t.Helper()
		res := graphql.Do(graphql.Params{Schema: sourceSwapSchema(t, svc), Context: context.Background(),
			RequestString: fmt.Sprintf(mutation, name)})
		if len(res.Errors) == 0 {
			return ""
		}
		return res.Errors[0].Message
	}
}

// w32052MCP calls update_service with args beside the service id.
func w32052MCP(args map[string]any) w32052Surface {
	return func(t *testing.T, svc *Service, name string) string {
		t.Helper()
		srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
		svc.RegisterMCP(srv)
		ctx := context.Background()
		serverT, clientT := mcp.NewInMemoryTransports()
		if _, err := srv.Connect(ctx, serverT, nil); err != nil {
			t.Fatalf("server connect: %v", err)
		}
		cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
		if err != nil {
			t.Fatalf("client connect: %v", err)
		}
		defer cs.Close()
		withID := map[string]any{"serviceId": name}
		for k, v := range args {
			withID[k] = v
		}
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_service", Arguments: withID})
		if err != nil {
			t.Fatalf("update_service: %v", err)
		}
		if !res.IsError {
			return ""
		}
		var text strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				text.WriteString(tc.Text)
			}
		}
		return text.String()
	}
}

// assertRefused checks that got is create's refusal and that the refused
// update wrote nothing.
func assertRefused(t *testing.T, got string, before, after appv1alpha1.AppSpec) {
	t.Helper()
	if !strings.Contains(got, w32052Refusal) {
		t.Errorf("answer %q, want %q", got, w32052Refusal)
	}
	if m461CommandsOf(after) != m461CommandsOf(before) || after.Repo != before.Repo || after.Image != before.Image {
		t.Errorf("refused update changed the service: commands %+v repo %q image %q, was %+v %q %q",
			m461CommandsOf(after), after.Repo, after.Image, m461CommandsOf(before), before.Repo, before.Image)
	}
}

// The repro's steps 2 and 4: clearing either command of a native service, on
// every type that runs one and through every caller, is refused. A cron's
// blank start command drops only its command override (w8/m69), so it keeps
// the start command its build needs and saves.
func TestClearingANativeCommandIsRefused(t *testing.T) {
	clears := map[string]map[string]w32052Surface{
		"buildCommand": {
			"REST":    w32052REST(`{"serviceDetails":{"envSpecificDetails":{"buildCommand":""}}}`),
			"GraphQL": w32052GraphQL(`mutation { setBuildCommand(id: "%s", command: "") { id } }`),
			"MCP":     w32052MCP(map[string]any{"buildCommand": ""}),
		},
		"startCommand": {
			"REST":    w32052REST(`{"serviceDetails":{"envSpecificDetails":{"startCommand":"  "}}}`),
			"GraphQL": w32052GraphQL(`mutation { setStartCommand(id: "%s", command: "") { id } }`),
			"MCP":     w32052MCP(map[string]any{"startCommand": ""}),
		},
	}
	types := []string{appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob}
	for field, surfaces := range clears {
		for surface, clear := range surfaces {
			for _, svcType := range types {
				t.Run(field+"/"+surface+"/"+svcType, func(t *testing.T) {
					svc, cl := newService(nil, m461NativeApp("web", svcType))
					before := getApp(t, cl, "web").Spec
					got := clear(t, svc, "web")
					after := getApp(t, cl, "web").Spec
					if svcType == appv1alpha1.TypeCronJob && field == "startCommand" {
						if got != "" || after.StartCommand != before.StartCommand {
							t.Errorf("native cron blank start command = %q, start %q; want saved, start %q kept", got, after.StartCommand, before.StartCommand)
						}
						return
					}
					assertRefused(t, got, before, after)
				})
			}
		}
	}
}

// The repro's step 5: a native service switched to an image keeps its runtime
// for a switch back (w4/m202) but not the commands its build baked in
// (w8/m461). Switching back without them is refused on every surface and
// leaves the image service as it was; sending them in the same update saves
// the switch, as on create.
func TestSwitchingBackToANativeRepoNeedsItsCommands(t *testing.T) {
	refused := map[string]w32052Surface{
		"REST":    w32052REST(`{"repo":"` + m461Repo + `","branch":"main"}`),
		"GraphQL": w32052GraphQL(`mutation { setRepo(id: "%s", repo: "` + m461Repo + `", branch: "main") { id } }`),
		"MCP":     w32052MCP(map[string]any{"repo": m461Repo, "branch": "main"}),
	}
	saved := map[string]w32052Surface{
		"REST": w32052REST(`{"repo":"` + m461Repo + `","branch":"main",` +
			`"serviceDetails":{"envSpecificDetails":{"buildCommand":"yarn","startCommand":"node app.js"}}}`),
		"GraphQL": w32052GraphQL(`mutation { setRepo(id: "%s", repo: "` + m461Repo + `", branch: "main", ` +
			`buildCommand: "yarn", startCommand: "node app.js") { id } }`),
		"MCP": w32052MCP(map[string]any{"repo": m461Repo, "branch": "main", "buildCommand": "yarn", "startCommand": "node app.js"}),
	}
	for surface := range refused {
		t.Run(surface, func(t *testing.T) {
			svc, cl := newService(nil, m461NativeApp("web", appv1alpha1.TypeWebService))
			m461Switch(t, svc, "web", "")
			before := getApp(t, cl, "web").Spec
			if before.Runtime != "node" || before.Image != m461Image || before.BuildCommand != "" || before.StartCommand != "" {
				t.Fatalf("fixture after the image switch = runtime %q image %q commands %+v", before.Runtime, before.Image, m461CommandsOf(before))
			}
			assertRefused(t, refused[surface](t, svc, "web"), before, getApp(t, cl, "web").Spec)

			if got := saved[surface](t, svc, "web"); got != "" {
				t.Fatalf("switch back with its commands = %q, want saved", got)
			}
			spec := getApp(t, cl, "web").Spec
			if spec.Repo != m461Repo || spec.Image != "" || spec.Runtime != "node" ||
				m461CommandsOf(spec) != (m461Commands{build: "yarn", start: "node app.js"}) {
				t.Errorf("after the switch back: repo %q image %q runtime %q commands %+v, want the native repo with its commands",
					spec.Repo, spec.Image, spec.Runtime, m461CommandsOf(spec))
			}
		})
	}
}

// The regression checks' controls: what builds without a command still saves
// with none, and a service already missing one (saved before this check)
// still takes every other update, each command on its own included, one save
// at a time as the dashboard's Build & Deploy fields send them.
func TestNativeCommandRefusalLeavesOtherSavesAlone(t *testing.T) {
	t.Run("a Dockerfile service's blank start command", func(t *testing.T) {
		svc, cl := newService(nil, dockerRepoApp("web"))
		start := "bin/server"
		if _, err := svc.SetCommands(context.Background(), "web", nil, &start); err != nil {
			t.Fatal(err)
		}
		if got := w32052REST(`{"serviceDetails":{"envSpecificDetails":{"dockerCommand":""}}}`)(t, svc, "web"); got != "" {
			t.Fatalf("blank Docker Command = %q, want saved", got)
		}
		if got := getApp(t, cl, "web").Spec.StartCommand; got != "" {
			t.Errorf("Docker Command after the clear = %q", got)
		}
	})
	t.Run("an image service's switch back to a Dockerfile repo", func(t *testing.T) {
		svc, cl := newService(nil, sampleApp("web"))
		if got := w32052REST(`{"repo":"`+m461Repo+`","branch":"main"}`)(t, svc, "web"); got != "" {
			t.Fatalf("image to a Dockerfile repo = %q, want saved", got)
		}
		if got := getApp(t, cl, "web").Spec.Repo; got != m461Repo {
			t.Errorf("repo = %q", got)
		}
	})
	t.Run("a service already missing both commands", func(t *testing.T) {
		app := m461NativeApp("web", appv1alpha1.TypeWebService)
		app.Spec.BuildCommand, app.Spec.StartCommand = "", ""
		svc, cl := newService(nil, app)
		for _, update := range []struct{ name, body string }{
			{"rename", `{"name":"renamed"}`},
			{"plan", `{"serviceDetails":{"plan":"starter"}}`},
			{"build command alone", `{"serviceDetails":{"envSpecificDetails":{"buildCommand":"yarn"}}}`},
			{"start command alone", `{"serviceDetails":{"envSpecificDetails":{"startCommand":"node app.js"}}}`},
		} {
			if got := w32052REST(update.body)(t, svc, "web"); got != "" {
				t.Errorf("%s = %q, want saved", update.name, got)
			}
		}
		if got := m461CommandsOf(getApp(t, cl, "web").Spec); got != (m461Commands{build: "yarn", start: "node app.js"}) {
			t.Errorf("commands after the repair = %+v", got)
		}
	})
	t.Run("a service missing its build command clears its start command", func(t *testing.T) {
		app := m461NativeApp("web", appv1alpha1.TypeWebService)
		app.Spec.BuildCommand = ""
		svc, cl := newService(nil, app)
		before := getApp(t, cl, "web").Spec
		got := w32052GraphQL(`mutation { setStartCommand(id: "%s", command: "") { id } }`)(t, svc, "web")
		assertRefused(t, got, before, getApp(t, cl, "web").Spec)
	})
}
