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
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m461. `bex services update <native Node service> --image <ref>` kept
// runtime node and the native `yarn` / `node app.js` on the App. Every read
// said node, the operator ran `node app.js` as the image's Docker Command (the
// deploy failed: the startup program "node" does not exist in this image), and
// the pinned client's `create --from` refused the clone. A switch to a prebuilt
// image now reads as an image service and drops the commands the build baked
// into its own image; the runtime, builder and root directory stay for a
// switch back (w4/m202).

const (
	m461Image = "docker.io/jmalloc/echo-server:v0.3.7"
	m461Repo  = "https://github.com/render-examples/express-hello-world"
)

// m461NativeApp is the sweep's fixture: a native Node service of svcType.
func m461NativeApp(name, svcType string) *appv1alpha1.App {
	app := gitServiceApp(name, svcType)
	app.Spec.Repo, app.Spec.RootDir, app.Spec.BuildFilter = m461Repo, "", nil
	app.Spec.BuildCommand, app.Spec.StartCommand = "yarn", "node app.js"
	app.Spec.Tier, app.Spec.Replicas = "free", 1
	return app
}

// m461Do sends method path body to svc's REST mux, checked against Render's
// schema.
func m461Do(t *testing.T, svc *Service, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

// m461REST is m461Do with the answer decoded.
func m461REST(t *testing.T, svc *Service, method, path, body string) (int, map[string]any) {
	t.Helper()
	rec := m461Do(t, svc, method, path, body)
	var out map[string]any
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: decode %v: %s", method, path, err, rec.Body.String())
		}
	}
	return rec.Code, out
}

// m461Switch PATCHes the service to the image as the pinned CLI sends it
// (pkg/service/update.go), with extra serviceDetails when given.
func m461Switch(t *testing.T, svc *Service, name, serviceDetails string) {
	t.Helper()
	body := `{"image":{"imagePath":"` + m461Image + `","ownerId":""}` + serviceDetails + `}`
	if code, _ := m461REST(t, svc, http.MethodPatch, "/v1/services/"+name, body); code != http.StatusOK {
		t.Fatalf("PATCH %s to the image = %d", body, code)
	}
}

// serviceDetailsOf is a REST read's serviceDetails and its envSpecificDetails.
func serviceDetailsOf(read map[string]any) (details, envSpecific map[string]any) {
	details, _ = read["serviceDetails"].(map[string]any)
	envSpecific, _ = details["envSpecificDetails"].(map[string]any)
	return details, envSpecific
}

// The definition of done's REST and GraphQL reads, and the switch back.
func TestNativeToImageSwitchReadsAsAnImageService(t *testing.T) {
	svc, cl := newService(nil, m461NativeApp("web", appv1alpha1.TypeWebService))
	m461Switch(t, svc, "web", "")

	_, read := m461REST(t, svc, http.MethodGet, "/v1/services/web", "")
	details, envSpecific := serviceDetailsOf(read)
	if details["runtime"] != "image" || details["env"] != "image" || read["imagePath"] != m461Image {
		t.Errorf("read runtime %v env %v imagePath %v, want image/image/%s", details["runtime"], details["env"], read["imagePath"], m461Image)
	}
	// Render's image shape (w9/m473): no build or start command, and no
	// Docker Command.
	if want := renderImageDetails("", ""); !reflect.DeepEqual(envSpecific, want) {
		t.Errorf("envSpecificDetails = %v, want %v", envSpecific, want)
	}
	res := graphql.Do(graphql.Params{Schema: sourceSwapSchema(t, svc), Context: context.Background(),
		RequestString: `{ service(id: "web") { runtime startCommand buildCommand } }`})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	if got := res.Data.(map[string]any)["service"]; !reflect.DeepEqual(got, map[string]any{"runtime": "image", "startCommand": "", "buildCommand": ""}) {
		t.Errorf("GraphQL service = %v, want runtime image and no commands", got)
	}
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()
	if mcpDetails, mcpEnvSpecific := serviceDetailsOf(call("get_service", map[string]any{"serviceId": "web"})); mcpDetails["runtime"] != "image" || !reflect.DeepEqual(mcpEnvSpecific, renderImageDetails("", "")) {
		t.Errorf("MCP get_service runtime %v envSpecificDetails %v, want image with no commands", mcpDetails["runtime"], mcpEnvSpecific)
	}
	spec := getApp(t, cl, "web").Spec
	if spec.Runtime != "node" || spec.StartCommand != "" || spec.BuildCommand != "" {
		t.Errorf("spec runtime %q start %q build %q, want node kept for a switch back and no commands", spec.Runtime, spec.StartCommand, spec.BuildCommand)
	}

	// Back on the repo, the retained runtime reads again.
	if code, _ := m461REST(t, svc, http.MethodPatch, "/v1/services/web", `{"repo":"`+m461Repo+`","branch":"main"}`); code != http.StatusOK {
		t.Fatalf("switch back = %d", code)
	}
	_, read = m461REST(t, svc, http.MethodGet, "/v1/services/web", "")
	if details, _ := serviceDetailsOf(read); details["runtime"] != "node" || details["env"] != "node" {
		t.Errorf("after the switch back runtime %v env %v, want node", details["runtime"], details["env"])
	}
}

// A command saved with the switch is the image's Docker Command, in either
// spelling the PATCH carries it (w4/m202), and a later build command is
// refused as on any image service (w4/418).
func TestNativeToImageSwitchKeepsACommandSavedWithIt(t *testing.T) {
	for _, spelling := range []string{"startCommand", "dockerCommand"} {
		t.Run(spelling, func(t *testing.T) {
			svc, cl := newService(nil, m461NativeApp("web", appv1alpha1.TypeWebService))
			m461Switch(t, svc, "web", `,"serviceDetails":{"envSpecificDetails":{"`+spelling+`":"/bin/echo-server --verbose"}}`)
			if spec := getApp(t, cl, "web").Spec; spec.StartCommand != "/bin/echo-server --verbose" || spec.BuildCommand != "" {
				t.Errorf("spec start %q build %q, want the saved command and no build command", spec.StartCommand, spec.BuildCommand)
			}
			_, read := m461REST(t, svc, http.MethodGet, "/v1/services/web", "")
			if _, envSpecific := serviceDetailsOf(read); envSpecific["dockerCommand"] != "/bin/echo-server --verbose" {
				t.Errorf("read envSpecificDetails = %v, want the Docker Command", envSpecific)
			}
		})
	}

	svc, cl := newService(nil, m461NativeApp("web", appv1alpha1.TypeWebService))
	m461Switch(t, svc, "web", "")
	code, _ := m461REST(t, svc, http.MethodPatch, "/v1/services/web", `{"serviceDetails":{"envSpecificDetails":{"buildCommand":"yarn"}}}`)
	if code != http.StatusBadRequest {
		t.Errorf("a build command on the switched service = %d, want 400", code)
	}
	if got := getApp(t, cl, "web").Spec.BuildCommand; got != "" {
		t.Errorf("refused build command saved as %q", got)
	}
}

// m461Surfaces switch a service to the image through each caller of the one
// source writer: REST PATCH and MCP update_service share the patch table, and
// GraphQL setImage is the dashboard's Settings → Source → Existing Image save.
var m461Surfaces = map[string]func(t *testing.T, svc *Service, name string){
	"REST": func(t *testing.T, svc *Service, name string) { m461Switch(t, svc, name, "") },
	"GraphQL": func(t *testing.T, svc *Service, name string) {
		mustMutate(t, sourceSwapSchema(t, svc), `mutation { setImage(id: "`+name+`", image: "`+m461Image+`") { runtime } }`)
	},
	"MCP": func(t *testing.T, svc *Service, name string) {
		call, cleanup := appsMCPClient(t, svc)
		defer cleanup()
		call("update_service", map[string]any{"serviceId": name, "image": m461Image})
	},
}

// m461Commands are the commands an App holds: build, start and a cron's own.
type m461Commands struct{ build, start, command string }

func m461CommandsOf(spec appv1alpha1.AppSpec) m461Commands {
	return m461Commands{spec.BuildCommand, spec.StartCommand, spec.Command}
}

// m461Link links a service to build source "carrier", running command: a
// cron keeps a linked command in spec.command, any other type in
// spec.startCommand (LinkBuildSource).
func m461Link(command string) func(*appv1alpha1.AppSpec) {
	return func(s *appv1alpha1.AppSpec) {
		s.Repo, s.Branch, s.Runtime, s.BuildCommand, s.StartCommand = "", "", "", "", command
		s.BuildSource = &appv1alpha1.BuildSourceLink{Name: "carrier", Image: "zot.local:5000/carrier@sha256:abc"}
		if s.Type == appv1alpha1.TypeCronJob {
			s.StartCommand, s.Command = "", command
		}
	}
}

// Every source kind a service can leave for an image, on every type that can
// run one, through every caller: what was already a Docker Command (a
// Dockerfile build's, an image's, a cron's explicit override) carries over;
// what the build baked into its own image (native, buildpack, a build
// source's) and the build command do not.
func TestSourceSwitchToImageAcrossCallersTypesAndSources(t *testing.T) {
	types := []string{appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob}
	sources := []struct {
		name     string
		cronOnly bool
		shape    func(spec *appv1alpha1.AppSpec)
		want     func(before m461Commands) m461Commands
		// carrierRuntime is the linked build source's runtime, when linked.
		carrierRuntime string
	}{
		{"native", false, func(*appv1alpha1.AppSpec) {},
			func(m461Commands) m461Commands { return m461Commands{} }, ""},
		{"buildpack", false, func(s *appv1alpha1.AppSpec) { s.Runtime, s.Builder, s.BuildCommand = "", buildBuildpack, "" },
			func(m461Commands) m461Commands { return m461Commands{} }, ""},
		{"Dockerfile", false, func(s *appv1alpha1.AppSpec) {
			s.Runtime, s.BuildCommand = "docker", ""
			if s.Type == appv1alpha1.TypeCronJob {
				s.StartCommand, s.Command = "", "./report"
			} else {
				s.StartCommand = "./server"
			}
		}, func(before m461Commands) m461Commands { return before }, ""},
		{"native cron override", true, func(s *appv1alpha1.AppSpec) { s.Command = "echo override" },
			func(m461Commands) m461Commands { return m461Commands{command: "echo override"} }, ""},
		{"native build source link", false, m461Link("node report.js"),
			func(m461Commands) m461Commands { return m461Commands{} }, "node"},
		{"Docker build source link", false, m461Link("./server"),
			func(before m461Commands) m461Commands { return before }, "docker"},
		{"image", false, func(s *appv1alpha1.AppSpec) {
			s.Repo, s.Image, s.Runtime, s.BuildCommand = "", "docker.io/library/nginx:1.27-alpine", "image", ""
			if s.Type == appv1alpha1.TypeCronJob {
				s.StartCommand, s.Command = "", "echo image"
			} else {
				s.StartCommand = "nginx -g 'daemon off;'"
			}
		}, func(before m461Commands) m461Commands { return before }, ""},
	}
	for surface, switchToImage := range m461Surfaces {
		for _, source := range sources {
			for _, svcType := range types {
				if source.cronOnly && svcType != appv1alpha1.TypeCronJob {
					continue
				}
				t.Run(surface+"/"+source.name+"/"+svcType, func(t *testing.T) {
					app := m461NativeApp("svc", svcType)
					source.shape(&app.Spec)
					before := app.Spec
					carrier := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "carrier", Namespace: "default"},
						Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeBuildSource, Repo: m461Repo, Runtime: source.carrierRuntime}}
					svc, cl := newService(nil, app, carrier)
					switchToImage(t, svc, "svc")

					spec := getApp(t, cl, "svc").Spec
					if want := source.want(m461CommandsOf(before)); m461CommandsOf(spec) != want {
						t.Errorf("commands %+v, want %+v", m461CommandsOf(spec), want)
					}
					if spec.Image != m461Image || spec.Repo != "" || spec.BuildSource != nil || spec.Runtime != before.Runtime {
						t.Errorf("spec image %q repo %q link %v runtime %q, want the image alone with runtime %q retained",
							spec.Image, spec.Repo, spec.BuildSource, spec.Runtime, before.Runtime)
					}
					if v, err := svc.Get(context.Background(), "svc"); err != nil || v.Runtime != "image" {
						t.Errorf("read runtime %q (%v), want image", v.Runtime, err)
					}
				})
			}
		}
	}
}

// switchedImageCloneSource is the switched service's read the pinned client
// clones from, shared with lego/cli/switched_image_clone_contract_test.go.
// Regenerate with BEX_UPDATE_GOLDEN=1 when the read shape intentionally moves.
var switchedImageCloneSource = filepath.Join("..", "..", "..", "cli", "testdata", "switched-image-clone-source.json")

// The read a `create --from` of the switched service starts from: the pinned
// client refused the clone of the old read (runtime node beside imagePath, with
// the native commands) before sending anything.
func TestSwitchedImageReadIsTheCloneContract(t *testing.T) {
	svc, _ := newService(nil, m461NativeApp("qa-switched-image", appv1alpha1.TypeWebService))
	svc.Clock = func() time.Time { return time.Date(2026, 11, 17, 6, 57, 0, 0, time.UTC) }
	m461Switch(t, svc, "qa-switched-image", "")
	rec := m461Do(t, svc, http.MethodGet, "/v1/services/qa-switched-image", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d: %s", rec.Code, rec.Body.String())
	}
	body := withoutClock(t, rec.Body.Bytes())
	matchCloneGolden(t, switchedImageCloneSource, body)
}

// m461Identity are the read fields that name a service or date its history,
// which a switched service and an image-created one cannot share.
var m461Identity = []string{"id", "name", "slug", "immutableName", "createdAt", "updatedAt", "dashboardUrl"}

// m461WithoutIdentity is read without its identity and history.
func m461WithoutIdentity(read map[string]any) map[string]any {
	for _, field := range m461Identity {
		delete(read, field)
	}
	if details, ok := read["serviceDetails"].(map[string]any); ok {
		delete(details, "url")
		delete(details, "internalAddress")
	}
	return read
}

// m461ComparableRead is name's REST read without its identity and history.
func m461ComparableRead(t *testing.T, svc *Service, name string) map[string]any {
	t.Helper()
	_, read := m461REST(t, svc, http.MethodGet, "/v1/services/"+name, "")
	return m461WithoutIdentity(read)
}

// m504Serve stands in for the operator: name's release serves on port. A
// switched service keeps its configured-port mode, so the port its process
// binds is found by bound-port detection (status.detectedPort: w8/177 for a
// web service, w8/m504 for a private one); an image-created one serves its
// image network. port 0 is a native release, which binds PORT.
func m504Serve(t *testing.T, cl client.Client, name string, port int32) {
	t.Helper()
	app := getApp(t, cl, name)
	app.Status.Phase, app.Status.ActiveRevision = appv1alpha1.PhaseRunning, "rev-1"
	if app.Spec.HasImagePorts() {
		network := &appv1alpha1.ImageNetworkStatus{
			Image: m461Image + "@sha256:0", Revision: "rev-1", PrimaryPort: port, Ports: []int32{port}, ConfiguredPort: new(int32(0)),
		}
		app.Status.ImageNetwork, app.Status.ServingNetwork = network, network.DeepCopy()
	} else {
		app.Status.DetectedPort = port
	}
	if err := cl.Update(context.Background(), app); err != nil {
		t.Fatalf("serve %s: %v", name, err)
	}
}

// m504Ports is a read's serviceDetails port and openPorts.
func m504Ports(read map[string]any) (port any, openPorts any) {
	details, _ := read["serviceDetails"].(map[string]any)
	return details["port"], details["openPorts"]
}

// A switched service reads exactly as one created on the same image, apart
// from its identity and history (w8/m461 t004), port and openPorts included
// (w8/m504). Before the switched source is deployed, the native release still
// serves PORT 10000, and the read says so, as Render's openPorts reports the
// running instance. Once the deploy has found the port the image binds, the
// reads match on REST, MCP and GraphQL, and in the dashboard's Settings port
// row (configuredPort, App.SavedPort).
func TestSwitchedServiceReadsAsAnImageCreatedOne(t *testing.T) {
	for _, tc := range []struct {
		svcType, plan string
		port          int32
	}{
		{"web_service", "free", 80},
		// Private services are paid-only, as on Render; redis binds 6379.
		{"private_service", "starter", 6379},
	} {
		t.Run(tc.svcType, func(t *testing.T) {
			svc, cl := newService(nil)
			for _, create := range []string{
				`{"name":"switched","type":"` + tc.svcType + `","repo":"` + m461Repo + `","branch":"main","autoDeploy":"no","serviceDetails":{"runtime":"node","plan":"` + tc.plan + `",` +
					`"envSpecificDetails":{"buildCommand":"yarn","startCommand":"node app.js"}}}`,
				`{"name":"born","type":"` + tc.svcType + `","image":{"imagePath":"` + m461Image + `","ownerId":""},"serviceDetails":{"runtime":"image","plan":"` + tc.plan + `"}}`,
			} {
				if rec := m461Do(t, svc, http.MethodPost, "/v1/services", create); rec.Code != http.StatusCreated {
					t.Fatalf("POST = %d: %s", rec.Code, rec.Body.String())
				}
			}
			m504Serve(t, cl, "switched", 0)
			m504Serve(t, cl, "born", tc.port)
			m461Switch(t, svc, "switched", "")

			wantPorts := func(port int32) []any {
				return []any{map[string]any{"port": float64(port), "protocol": "TCP"}}
			}
			if port, open := m504Ports(m461ComparableRead(t, svc, "switched")); port != float64(10000) || !reflect.DeepEqual(open, wantPorts(10000)) {
				t.Errorf("before the deploy: port %v openPorts %v, want the native release's 10000", port, open)
			}

			m504Serve(t, cl, "switched", tc.port)
			switched, born := m461ComparableRead(t, svc, "switched"), m461ComparableRead(t, svc, "born")
			if port, open := m504Ports(switched); port != float64(tc.port) || !reflect.DeepEqual(open, wantPorts(tc.port)) {
				t.Errorf("after the deploy: port %v openPorts %v, want the image's %d", port, open, tc.port)
			}
			if !reflect.DeepEqual(switched, born) {
				a, _ := json.Marshal(switched)
				b, _ := json.Marshal(born)
				t.Errorf("REST: the switched read differs from an image-created one\nswitched: %s\n    born: %s", a, b)
			}

			call, cleanup := appsMCPClient(t, svc)
			defer cleanup()
			mcpSwitched := m461WithoutIdentity(call("get_service", map[string]any{"serviceId": "switched"}))
			if mcpBorn := m461WithoutIdentity(call("get_service", map[string]any{"serviceId": "born"})); !reflect.DeepEqual(mcpSwitched, mcpBorn) {
				t.Errorf("MCP get_service: the switched read differs from an image-created one\nswitched: %v\n    born: %v", mcpSwitched, mcpBorn)
			}

			gql := func(name string) any {
				res := graphql.Do(graphql.Params{Schema: sourceSwapSchema(t, svc), Context: context.Background(),
					RequestString: `{ service(id: "` + name + `") { port configuredPort openPorts { port protocol } } }`})
				if len(res.Errors) > 0 {
					t.Fatal(res.Errors)
				}
				return res.Data.(map[string]any)["service"]
			}
			want := map[string]any{"port": int(tc.port), "configuredPort": int(tc.port),
				"openPorts": []any{map[string]any{"port": int(tc.port), "protocol": "TCP"}}}
			if got := gql("switched"); !reflect.DeepEqual(got, want) {
				t.Errorf("GraphQL switched = %v, want %v", got, want)
			}
			if got := gql("born"); !reflect.DeepEqual(got, want) {
				t.Errorf("GraphQL born = %v, want %v", got, want)
			}
		})
	}
}
