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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
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

// w32/056. The switch back without commands, through each surface: REST with
// exactly the body the pinned CLI sends for `services update <id> --repo …
// --branch main` (lego/cli/switched_image_switch_back_contract_test.go records
// it; the client refuses to send the commands to a service reading image),
// GraphQL setRepo and MCP update_service.
var w32056SwitchBack = map[string]w32052Surface{
	"REST (pinned CLI)": func(t *testing.T, svc *Service, name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "cli", "testdata", "switched-image-switch-back-request.json"))
		if err != nil {
			t.Fatal(err)
		}
		return w32052REST(string(raw))(t, svc, name)
	},
	"GraphQL": w32052GraphQL(`mutation { setRepo(id: "%s", repo: "` + m461Repo + `", branch: "main") { id } }`),
	"MCP":     w32052MCP(map[string]any{"repo": m461Repo, "branch": "main"}),
}

// w32056SwitchBackWith sends the switch back with both commands, as the
// dashboard's Update Source dialog does, on each surface.
var w32056SwitchBackWith = map[string]w32052Surface{
	"REST (pinned CLI)": w32052REST(`{"repo":"` + m461Repo + `","branch":"main",` +
		`"serviceDetails":{"envSpecificDetails":{"buildCommand":"pnpm i","startCommand":"node v2.js"}}}`),
	"GraphQL": w32052GraphQL(`mutation { setRepo(id: "%s", repo: "` + m461Repo + `", branch: "main", ` +
		`buildCommand: "pnpm i", startCommand: "node v2.js") { id } }`),
	"MCP": w32052MCP(map[string]any{"repo": m461Repo, "branch": "main", "buildCommand": "pnpm i", "startCommand": "node v2.js"}),
}

// assertSwitchedBack checks that name builds the repo natively again with
// want, and keeps nothing more for another switch back.
func assertSwitchedBack(t *testing.T, cl client.Client, name string, want m461Commands) {
	t.Helper()
	a := getApp(t, cl, name)
	if a.Spec.Repo != m461Repo || a.Spec.Image != "" || a.Spec.Runtime != "node" || m461CommandsOf(a.Spec) != want {
		t.Errorf("after the switch back: repo %q image %q runtime %q commands %+v, want the native repo with %+v",
			a.Spec.Repo, a.Spec.Image, a.Spec.Runtime, m461CommandsOf(a.Spec), want)
	}
	if kept := switchBackCommands(a); kept != (nativeCommands{}) {
		t.Errorf("the repo service still keeps %+v for a switch back", kept)
	}
}

// The w32/056 repro: a native service switched to an image (w8/m461 drops its
// commands from the spec, which the image runs) switches back to Git with
// `bex services update --repo … --branch …` alone, on every type that runs a
// start command and through every caller: the switch to the image kept the
// commands, and the switch back builds with them. Commands sent with the
// switch back win over the kept ones.
func TestSwitchingBackToANativeRepoRestoresItsCommands(t *testing.T) {
	types := []string{appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob}
	for surface, switchBack := range w32056SwitchBack {
		for _, svcType := range types {
			t.Run(surface+"/"+svcType, func(t *testing.T) {
				svc, cl := newService(nil, m461NativeApp("web", svcType))
				m461Switch(t, svc, "web", "")
				switched := getApp(t, cl, "web")
				if switched.Spec.Runtime != "node" || switched.Spec.Image != m461Image || m461CommandsOf(switched.Spec) != (m461Commands{}) {
					t.Fatalf("fixture after the image switch = runtime %q image %q commands %+v", switched.Spec.Runtime, switched.Spec.Image, m461CommandsOf(switched.Spec))
				}
				if kept := switchBackCommands(switched); kept != (nativeCommands{build: "yarn", start: "node app.js"}) {
					t.Fatalf("the image switch kept %+v, want the native commands it dropped", kept)
				}
				if got := switchBack(t, svc, "web"); got != "" {
					t.Fatalf("switch back with no commands = %q, want saved with the kept ones", got)
				}
				assertSwitchedBack(t, cl, "web", m461Commands{build: "yarn", start: "node app.js"})
			})
		}
	}
	for surface, switchBack := range w32056SwitchBackWith {
		t.Run(surface+"/with commands", func(t *testing.T) {
			svc, cl := newService(nil, m461NativeApp("web", appv1alpha1.TypeWebService))
			m461Switch(t, svc, "web", "")
			if got := switchBack(t, svc, "web"); got != "" {
				t.Fatalf("switch back with its commands = %q, want saved", got)
			}
			assertSwitchedBack(t, cl, "web", m461Commands{build: "pnpm i", start: "node v2.js"})
		})
	}
}

// What the switch keeps follows the image service: a change of image keeps
// it, and the native start command replaces a Docker Command saved on the image,
// which the image ran and the native build does not. A switch back that
// clears a kept command in the same update is refused as a clear is.
func TestKeptCommandsFollowTheImageService(t *testing.T) {
	t.Run("a change of image keeps them", func(t *testing.T) {
		svc, cl := newService(nil, m461NativeApp("web", appv1alpha1.TypeWebService))
		m461Switch(t, svc, "web", "")
		if got := w32052REST(`{"image":{"imagePath":"docker.io/library/nginx:1.27-alpine","ownerId":""}}`)(t, svc, "web"); got != "" {
			t.Fatalf("image change = %q", got)
		}
		if got := w32056SwitchBack["REST (pinned CLI)"](t, svc, "web"); got != "" {
			t.Fatalf("switch back = %q", got)
		}
		assertSwitchedBack(t, cl, "web", m461Commands{build: "yarn", start: "node app.js"})
	})
	t.Run("a Docker Command saved on the image gives way", func(t *testing.T) {
		svc, cl := newService(nil, m461NativeApp("web", appv1alpha1.TypeWebService))
		m461Switch(t, svc, "web", "")
		if got := w32052GraphQL(`mutation { setStartCommand(id: "%s", command: "/bin/echo-server --verbose") { id } }`)(t, svc, "web"); got != "" {
			t.Fatalf("Docker Command = %q", got)
		}
		if got := getApp(t, cl, "web").Spec.StartCommand; got != "/bin/echo-server --verbose" {
			t.Fatalf("the image's Docker Command = %q", got)
		}
		if got := w32056SwitchBack["MCP"](t, svc, "web"); got != "" {
			t.Fatalf("switch back = %q", got)
		}
		assertSwitchedBack(t, cl, "web", m461Commands{build: "yarn", start: "node app.js"})
	})
	t.Run("clearing one in the switch back is refused", func(t *testing.T) {
		svc, cl := newService(nil, m461NativeApp("web", appv1alpha1.TypeWebService))
		m461Switch(t, svc, "web", "")
		before := getApp(t, cl, "web")
		got := w32052REST(`{"repo":"`+m461Repo+`","branch":"main","serviceDetails":{"envSpecificDetails":{"buildCommand":""}}}`)(t, svc, "web")
		after := getApp(t, cl, "web")
		assertRefused(t, got, before.Spec, after.Spec)
		if switchBackCommands(after) != switchBackCommands(before) {
			t.Errorf("the refused switch back dropped the kept commands: %+v", switchBackCommands(after))
		}
	})
}

// The w32/052 refusal stays for a switch back with nothing kept, such as an
// image service switched before w32/056: switching back without commands is
// refused on every surface and leaves the image service as it was; sending
// them in the same update saves the switch, as on create.
func TestSwitchingBackWithNothingKeptNeedsItsCommands(t *testing.T) {
	for surface, refused := range w32056SwitchBack {
		t.Run(surface, func(t *testing.T) {
			svc, cl := newService(nil, m461NativeApp("web", appv1alpha1.TypeWebService))
			m461Switch(t, svc, "web", "")
			switched := getApp(t, cl, "web")
			forgetSwitchBackCommands(switched)
			if err := cl.Update(context.Background(), switched); err != nil {
				t.Fatal(err)
			}
			before := getApp(t, cl, "web").Spec
			assertRefused(t, refused(t, svc, "web"), before, getApp(t, cl, "web").Spec)

			if got := w32056SwitchBackWith[surface](t, svc, "web"); got != "" {
				t.Fatalf("switch back with its commands = %q, want saved", got)
			}
			assertSwitchedBack(t, cl, "web", m461Commands{build: "pnpm i", start: "node v2.js"})
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

// A link to a build source forgets what the switch to the image kept: the
// link brings its own build and clears the runtime the commands built with.
func TestLinkingABuildSourceForgetsKeptCommands(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	image, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{OwnerID: "tea-a", Name: "Img", Image: &BuildSourceImage{ImageURL: "nginx:1"}})
	if err != nil {
		t.Fatal(err)
	}
	web, err := svc.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: "web", Type: appv1alpha1.TypeWebService,
		Repo: "https://github.com/acme/web", Runtime: "node", BuildCommand: "npm ci", StartCommand: "node server.js"})
	if err != nil {
		t.Fatal(err)
	}
	img := m461Image
	if _, err := svc.SetSourceAndRegistryCredential(ctx, web.ID, sourcePatch{Image: &img}); err != nil {
		t.Fatal(err)
	}
	if kept := switchBackCommands(getTenantApp(t, svc.Client, "tea-a", "web")); kept != (nativeCommands{build: "npm ci", start: "node server.js"}) {
		t.Fatalf("the image switch kept %+v", kept)
	}
	if _, err := svc.LinkBuildSource(ctx, web.ID, image.ID, nil, false); err != nil {
		t.Fatal(err)
	}
	if kept := switchBackCommands(getTenantApp(t, svc.Client, "tea-a", "web")); kept != (nativeCommands{}) {
		t.Errorf("the linked service still keeps %+v for a switch back", kept)
	}
}

// A Blueprint sync that moves an image service back to a repo declares its own
// commands, so it forgets the ones the switch to the image kept; a sync that
// keeps the image keeps them.
func TestBlueprintSyncOffTheImageForgetsKeptCommands(t *testing.T) {
	native := "services:\n- {name: own-source, type: web, runtime: node, repo: " + switchRepo + ", branch: master, plan: free, buildCommand: 'true', startCommand: 'node server.js'}\n"
	image := "services:\n- {name: own-source, type: web, runtime: image, image: {url: '" + switchImage + "'}, plan: free, healthCheckPath: /healthz}\n"
	for _, tc := range []struct {
		name, manifest string
		want           nativeCommands
	}{
		{"to the repo", native, nativeCommands{}},
		{"on the image", image, nativeCommands{build: "yarn", start: "node app.js"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st, _, err := compileStack(DeployRequest{Manifest: tc.manifest})
			if err != nil {
				t.Fatal(err)
			}
			decl := st.services[0]
			want, err := specFromCreate(decl.req)
			if err != nil {
				t.Fatal(err)
			}
			existing := blueprintIntentApp(true)
			existing.Spec = imageBornSpec()
			existing.Spec.Runtime = "node"
			setSwitchBackCommands(existing, nativeCommands{build: "yarn", start: "node app.js"})
			cl := fakeClient(existing)
			svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, Store: &recordingStore{}}
			if _, err := svc.updateStackService(context.Background(), decl.req, decl.fields, want, existing); err != nil {
				t.Fatal(err)
			}
			if kept := switchBackCommands(getApp(t, cl, existing.Name)); kept != tc.want {
				t.Errorf("kept after the sync %+v, want %+v", kept, tc.want)
			}
		})
	}
}
