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

package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// buildSourceReconciler is a fake cluster holding app (and others), reconciled
// by the lifecycle fixture's reconciler.
func buildSourceReconciler(t *testing.T, app *appv1alpha1.App, others ...client.Object) (*AppReconciler, client.Client, types.NamespacedName) {
	t.Helper()
	scheme := wakeScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(append(others, app)...).WithStatusSubresource(&appv1alpha1.App{}).Build()
	return wakeReconciler(cl, scheme), cl, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
}

// carrierApp is the build source "src" in the fixture namespace, backed by
// image (or by a repo when image is empty).
func carrierApp(image string) *appv1alpha1.App {
	carrier := activeApp("tea-bsrc")
	carrier.Name = "src"
	carrier.Spec.Type = appv1alpha1.TypeBuildSource
	carrier.Spec.Tier, carrier.Spec.Expose = "", false
	carrier.Spec.Image = image
	if image == "" {
		carrier.Spec.Repo = "https://github.com/acme/mono"
	}
	return carrier
}

func workerLinkedTo(image string) *appv1alpha1.App {
	app := heldWorkerApp("tea-bsrc")
	app.Spec.Image = ""
	app.Spec.BuildSource = &appv1alpha1.BuildSourceLink{Name: "src", Image: image}
	return app
}

func deploymentImage(t *testing.T, cl client.Client, nn types.NamespacedName) string {
	t.Helper()
	var dep appsv1.Deployment
	if err := cl.Get(context.Background(), nn, &dep); err != nil {
		t.Fatalf("Deployment: %v", err)
	}
	return dep.Spec.Template.Spec.Containers[0].Image
}

// ADR097 §1: a carrier builds (here: names an image) and runs nothing — no
// Deployment, Service or Ingress — and its build is reported ready.
func TestBuildSourceCarrierRunsNothing(t *testing.T) {
	r, cl, nn := buildSourceReconciler(t, carrierApp("nginx:1"))
	reconcileTwice(t, r, nn)

	for _, obj := range []client.Object{&appsv1.Deployment{}, &corev1.Service{}, &networkingv1.Ingress{}} {
		if err := cl.Get(context.Background(), nn, obj); !apierrors.IsNotFound(err) {
			t.Errorf("carrier has a %T (err %v)", obj, err)
		}
	}
	live := liveApp(t, cl, nn)
	ready := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionReady)
	if live.Status.Image != "nginx:1" || ready == nil || ready.Reason != "Built" || live.Status.ActiveRevision == "" || live.Status.URL != "" {
		t.Fatalf("carrier status = %+v, want its image recorded, Ready=Built, an active revision and no URL", live.Status)
	}
}

// A linked service with no selected build waits instead of building its own.
func TestLinkedServiceWaitsForABuild(t *testing.T) {
	r, cl, nn := buildSourceReconciler(t, workerLinkedTo(""))
	reconcileTwice(t, r, nn)
	if err := cl.Get(context.Background(), nn, &appsv1.Deployment{}); !apierrors.IsNotFound(err) {
		t.Fatalf("a linked service with no build has a Deployment (err %v)", err)
	}
	ready := meta.FindStatusCondition(liveApp(t, cl, nn).Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Reason != "AwaitingBuildSource" {
		t.Fatalf("Ready = %+v, want AwaitingBuildSource", ready)
	}
}

// An image-backed source's image lives outside the platform registry: the
// linked service runs it as is.
func TestLinkedServiceRunsAnExternalImage(t *testing.T) {
	r, cl, nn := buildSourceReconciler(t, workerLinkedTo("docker.io/library/nginx:1.27"), carrierApp("docker.io/library/nginx:1.27"))
	reconcileTwice(t, r, nn)
	if got := deploymentImage(t, cl, nn); got != "docker.io/library/nginx:1.27" {
		t.Fatalf("Deployment image = %q", got)
	}
}

// ADR097 §4: a platform build is copied into the service's own repository
// under the same digest, and the service runs that copy; selecting another
// build is a new release.
func TestLinkedServiceDeploysACopyOfThePlatformBuild(t *testing.T) {
	var mu sync.Mutex
	manifests := map[string]string{
		appIdentity(carrierApp("")).Repo() + "@" + digestA: `{"config":{"digest":"` + digestCfg + `"},"layers":[]}`,
		appIdentity(carrierApp("")).Repo() + "@" + digestB: `{"config":{"digest":"` + digestCfg + `"},"layers":[]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		path := strings.TrimPrefix(req.URL.Path, "/v2/")
		switch {
		case strings.HasSuffix(path, "/blobs/uploads/"):
			w.WriteHeader(http.StatusCreated)
		case strings.Contains(path, "/manifests/") && req.Method == http.MethodGet:
			repo, ref, _ := strings.Cut(path, "/manifests/")
			body, ok := manifests[repo+"@"+ref]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = io.WriteString(w, body)
		case strings.Contains(path, "/manifests/") && req.Method == http.MethodPut:
			repo, ref, _ := strings.Cut(path, "/manifests/")
			body, _ := io.ReadAll(req.Body)
			manifests[repo+"@"+ref] = string(body)
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	carrier := carrierApp("")
	src := appIdentity(carrier).Repo()
	app := workerLinkedTo(host + "/" + src + ":gen-1@" + digestA)
	r, cl, nn := buildSourceReconciler(t, app, carrier)
	r.Registry = host
	reconcileTwice(t, r, nn)
	own := appIdentity(app).Repo()
	if got, want := deploymentImage(t, cl, nn), host+"/"+own+"@"+digestA; got != want {
		t.Fatalf("Deployment image = %q, want the copy %q", got, want)
	}
	mu.Lock()
	_, copied := manifests[own+"@"+digestA]
	_, tagged := manifests[own+"@"+linkedBuildTagPrefix+strings.Repeat("a", 12)]
	mu.Unlock()
	if !copied || !tagged {
		t.Fatalf("the build was not copied (%v) and tagged (%v) into %s", copied, tagged, own)
	}

	live := liveApp(t, cl, nn)
	live.Spec.BuildSource.Image = host + "/" + src + ":gen-2@" + digestB
	if err := cl.Update(context.Background(), &live); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)
	if got, want := deploymentImage(t, cl, nn), host+"/"+own+"@"+digestB; got != want {
		t.Fatalf("after selecting the next build: Deployment image = %q, want %q", got, want)
	}
}

var digestCfg = "sha256:" + strings.Repeat("c", 64)

// A link is checked against its carrier before anything is copied with the
// builder's credential: an image outside the carrier's repository (another
// tenant's, say) or a name that is not a build source is refused.
func TestLinkedServiceRefusesAnImageOutsideItsBuildSource(t *testing.T) {
	carrier := carrierApp("")
	for name, tc := range map[string]struct {
		image  string
		others []client.Object
	}{
		"another repository": {"zot.test:5000/tea-other/secret@" + digestA, []client.Object{carrier}},
		"not a build source": {"zot.test:5000/" + appIdentity(carrier).Repo() + "@" + digestA, []client.Object{heldWorkerApp("tea-bsrc")}},
		"external mismatch":  {"docker.io/library/redis:7", []client.Object{carrierApp("docker.io/library/nginx:1.27")}},
		"unpinned":           {"zot.test:5000/" + appIdentity(carrier).Repo() + ":gen-1", []client.Object{carrier}},
	} {
		t.Run(name, func(t *testing.T) {
			app := workerLinkedTo(tc.image)
			if name == "not a build source" {
				tc.others[0].SetName("src")
			}
			r, cl, nn := buildSourceReconciler(t, app, tc.others...)
			r.Registry = "zot.test:5000"
			reconcileTwice(t, r, nn)
			if err := cl.Get(context.Background(), nn, &appsv1.Deployment{}); !apierrors.IsNotFound(err) {
				t.Fatalf("a refused link has a Deployment (err %v)", err)
			}
			if live := liveApp(t, cl, nn); live.Status.Phase != appv1alpha1.PhaseFailed {
				t.Fatalf("phase = %s, want Failed", live.Status.Phase)
			}
		})
	}
}

// A canceled build on a carrier that already built keeps its previous build
// current and still runs nothing.
func TestBuildSourceCanceledBuildRunsNothing(t *testing.T) {
	r, cl, nn := buildSourceReconciler(t, carrierApp("nginx:1"))
	reconcileTwice(t, r, nn)
	// The fake client does not bump metadata.generation, so settle the
	// canceled release directly over the carrier that already built.
	live := liveApp(t, cl, nn)
	live.Spec.Image = "nginx:2"
	if _, err := r.settleCanceledRelease(context.Background(), &live, 0); err != nil {
		t.Fatal(err)
	}
	for _, obj := range []client.Object{&appsv1.Deployment{}, &corev1.Service{}} {
		if err := cl.Get(context.Background(), nn, obj); !apierrors.IsNotFound(err) {
			t.Errorf("canceled carrier has a %T (err %v)", obj, err)
		}
	}
	if got := liveApp(t, cl, nn).Status.Image; got != "nginx:1" {
		t.Fatalf("image after the canceled build = %q, want the previous build", got)
	}
}

// w2/m254 t016: a native git build source has no start command of its own
// (each linked service supplies one), and its build still dispatches instead
// of failing with "native runtime requires startCommand".
func TestNativeBuildSourceDispatchesWithoutAStartCommand(t *testing.T) {
	carrier := carrierApp("")
	carrier.Spec.Runtime, carrier.Spec.Builder = "node", "native"
	carrier.Spec.BuildCommand, carrier.Spec.StartCommand = "yarn", ""
	carrier.UID = "carrier-uid"
	r, cl, nn := buildSourceReconciler(t, carrier)
	reconcileTwice(t, r, nn)

	live := liveApp(t, cl, nn)
	for _, c := range live.Status.Conditions {
		if strings.Contains(c.Message, "requires startCommand") {
			t.Fatalf("carrier condition %s = %s: %s; a build source needs no start command", c.Type, c.Reason, c.Message)
		}
	}
	if live.Status.Phase == appv1alpha1.PhaseFailed {
		t.Fatalf("native carrier failed: %+v", live.Status.Conditions)
	}
}

// w2/m254 t017: a service linked to a native build source runs its start
// command as bash arguments to the image's entrypoint (native-init), and the
// recorded launch reads back the same way; otherwise it keeps the shell launch.
func TestServiceLinkedToANativeSourceStartsLikeANativeService(t *testing.T) {
	const start = "[[ -n $PORT ]] && npm start"
	app := projectionApp(func(a *appv1alpha1.App) {
		a.Spec = appv1alpha1.AppSpec{StartCommand: start, BuildSource: &appv1alpha1.BuildSourceLink{Name: "src", Image: "zot/src@sha256:a"}}
	})
	p := webParams()
	p.image, p.launch = "zot/web@sha256:a", launchNativeArgs
	c := appContainerOf(t, project(app, p))
	if want := []string{"/bin/bash", "-c", "[[ -n $$PORT ]] && npm start"}; len(c.Command) != 0 || !slices.Equal(c.Args, want) {
		t.Fatalf("container command %q args %q, want no command and args %q", c.Command, c.Args, want)
	}
	rec := m202Record(start, p.image, nil)
	rec.template.Spec.Containers[0].Args = c.Args
	if got := rec.launch(app); got != launchNativeArgs {
		t.Fatalf("recorded launch = %q, want %q", got, launchNativeArgs)
	}
	p.launch = launchUnknown
	if c := appContainerOf(t, project(app, p)); !slices.Equal(c.Command, shellCommand(start)) || len(c.Args) != 0 {
		t.Fatalf("shell launch: command %q args %q, want the shell override", c.Command, c.Args)
	}
}

// Native-args only for the carrier's current native build: a Docker source,
// an older digest, or a build made before the source switched to native keeps
// the shell launch, which runs on any image.
func TestLinkedToNativeSourceOnlyForTheCurrentNativeBuild(t *testing.T) {
	const current = "zot/src@sha256:new"
	native := carrierApp("")
	native.Spec.Runtime, native.Spec.Builder, native.Spec.BuildCommand = "node", "native", "yarn"
	native.Status.ArtifactImage, native.Status.ArtifactFingerprint = current, desiredAppReleaseIdentity(native.Spec).artifact
	switched := native.DeepCopy()
	switched.Name, switched.Status.ArtifactFingerprint = "switched", "built-as-docker"
	docker := carrierApp("")
	docker.Name, docker.Spec.Runtime = "dsrc", "docker"
	docker.Status.ArtifactImage, docker.Status.ArtifactFingerprint = current, desiredAppReleaseIdentity(docker.Spec).artifact
	linkedTo := func(name, image string) *appv1alpha1.App {
		a := workerLinkedTo(image)
		a.Spec.BuildSource.Name = name
		return a
	}
	r, _, _ := buildSourceReconciler(t, linkedTo("src", current), native, switched, docker)
	for name, tc := range map[string]struct {
		app  *appv1alpha1.App
		want bool
	}{
		"current native build":             {linkedTo("src", current), true},
		"an older digest":                  {linkedTo("src", "zot/src@sha256:old"), false},
		"built before switching to native": {linkedTo("switched", current), false},
		"a Docker source":                  {linkedTo("dsrc", current), false},
		"a missing source":                 {linkedTo("gone", current), false},
	} {
		if got := r.linkedToNativeSource(context.Background(), tc.app); got != tc.want {
			t.Errorf("%s: native-args = %v, want %v", name, got, tc.want)
		}
	}
}

// w2/m262 t004: a cron job linked to a native build source runs its command
// as bash arguments to the image's entrypoint, like a linked web service; any
// other launch keeps the shell Command.
func TestCronLinkedToANativeSourceRunsUnderItsEntrypoint(t *testing.T) {
	const command = "[[ -n $PORT ]] && node job.js"
	app := projectionApp(func(a *appv1alpha1.App) {
		a.Spec = appv1alpha1.AppSpec{Type: appv1alpha1.TypeCronJob, Command: command,
			BuildSource: &appv1alpha1.BuildSourceLink{Name: "src", Image: "zot/src@sha256:a"}}
	})
	c := (&AppReconciler{}).cronPodSpec(app, "zot/cron@sha256:a", 0, nil, launchNativeArgs, "").Spec.Containers[0]
	if want := []string{"/bin/bash", "-c", "[[ -n $$PORT ]] && node job.js"}; len(c.Command) != 0 || !slices.Equal(c.Args, want) {
		t.Fatalf("cron command %q args %q, want no command and args %q", c.Command, c.Args, want)
	}
	c = (&AppReconciler{}).cronPodSpec(app, "zot/cron@sha256:a", 0, nil, launchUnknown, "").Spec.Containers[0]
	if !slices.Equal(c.Command, shellCommand(command)) || len(c.Args) != 0 {
		t.Fatalf("shell launch: command %q args %q, want the shell Command", c.Command, c.Args)
	}
}

// w9/m180: a web service linked to an image-backed source gets the carrier's
// image-port mode, so its network is read from the source's image — the
// image the service deploys — not refused for lacking an own build (the
// production failure "image port discovery requires this service's built
// image repository").
func TestLinkedServiceResolvesTheSourceImageNetwork(t *testing.T) {
	const image = "docker.io/library/nginx:1.27"
	app := workerLinkedTo(image)
	app.Spec.Type = appv1alpha1.TypeWebService
	app.Spec.PortMode = appv1alpha1.PortModeWebImageV1
	r, _, _ := buildSourceReconciler(t, app, carrierApp(image))
	r.Registry = "registry.test"
	r.HTTPClient = imageConfigRegistry(t, `"8080/tcp":{}`)

	_, port, err := r.resolveImageNetwork(t.Context(), app, image)
	if err != nil {
		t.Fatalf("resolve the linked image's network: %v", err)
	}
	if port != 8080 {
		t.Fatalf("primary port = %d, want the source image's 8080", port)
	}

	// Only the linked image itself reads as the source's: anything else a
	// linked service might be handed stays refused.
	if _, _, err := r.resolveImageNetwork(t.Context(), app, "docker.io/library/redis:7"); err == nil {
		t.Fatal("a linked service resolved an image that is not its source's")
	}
}
