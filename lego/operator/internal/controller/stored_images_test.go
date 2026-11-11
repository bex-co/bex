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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// stored_images_test.go is the operator's half of w8/m156 t003. The registry
// keeps a bounded history of an App's builds, and what it kept cannot be
// derived from outside it. The operator reads the repository and publishes the
// digests it holds (status.storedImages), which bex-api uses to refuse a
// rollback to a build that is gone.

// fakeTagRegistry serves a repository's tag list and each tag's digest to one
// user, and counts the listings.
type fakeTagRegistry struct {
	mu       sync.Mutex
	repo     string
	user     string
	pass     string
	tags     map[string]string // tag -> digest; "" answers 404, as a tag retention just removed
	failTag  string            // answers 503: the registry could not be asked
	listings int
}

func (f *fakeTagRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, p, ok := r.BasicAuth(); !ok || u != f.user || p != f.pass {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	base := "/v2/" + f.repo
	switch {
	case r.Method == http.MethodGet && r.URL.Path == base+"/tags/list":
		f.listings++
		names := make([]string, 0, len(f.tags))
		for tag := range f.tags {
			names = append(names, tag)
		}
		slices.Sort(names)
		_ = json.NewEncoder(w).Encode(map[string]any{"name": f.repo, "tags": names})
	case r.Method == http.MethodHead && strings.HasPrefix(r.URL.Path, base+"/manifests/"):
		tag := strings.TrimPrefix(r.URL.Path, base+"/manifests/")
		if tag == f.failTag {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		digest := f.tags[tag]
		if digest == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Docker-Content-Digest", digest)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func storedImagesFixture(t *testing.T, tags map[string]string) (*AppReconciler, *appv1alpha1.App, *fakeTagRegistry) {
	t.Helper()
	app := liveTagApp("")
	reg := &fakeTagRegistry{repo: appIdentity(app).Repo(), user: appIdentity(app).ZotUsername(), pass: "pw", tags: tags}
	srv := httptest.NewServer(reg)
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(srv.URL, "http://")
	app.Status.Image = host + "/" + reg.repo + ":gen-7@" + app.Status.ImageDigest.Digest

	auth := base64.StdEncoding.EncodeToString([]byte(reg.user + ":" + reg.pass))
	dockerConfig, err := json.Marshal(map[string]any{"auths": map[string]any{
		host: map[string]string{"username": reg.user, "password": reg.pass, "auth": auth},
	}})
	if err != nil {
		t.Fatal(err)
	}
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	pull := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: appIdentity(app).PullSecretName(), Namespace: app.Namespace},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{corev1.DockerConfigJsonKey: dockerConfig},
	}
	creds := &registry.Creds{Registry: host, Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(pull).Build()}
	return &AppReconciler{Registry: host, PerAppRegistry: creds}, app, reg
}

func digestOf(c string) string { return "sha256:" + strings.Repeat(c, 64) }

func TestRefreshStoredImagesPublishesWhatTheRegistryHolds(t *testing.T) {
	r, app, reg := storedImagesFixture(t, map[string]string{
		"gen-7":  digestOf("a"),
		"live-7": digestOf("a"), // the same manifest under its live tag
		"gen-6":  digestOf("b"),
		"gen-5":  "", // listed, then gone: retention removed it meanwhile
	})

	r.refreshStoredImages(context.Background(), app, false)
	want := []string{digestOf("a"), digestOf("b")}
	if app.Status.StoredImages == nil || !slices.Equal(app.Status.StoredImages.Digests, want) {
		t.Fatalf("storedImages = %+v, want %v", app.Status.StoredImages, want)
	}
	// The reading carries the generation it was read at: it says nothing
	// about a build made later.
	if got := app.Status.StoredImages.Generation; got != app.Status.ImageDigest.Generation {
		t.Errorf("storedImages.generation = %d, want the serving release's %d", got, app.Status.ImageDigest.Generation)
	}
	if !app.Status.StoredImages.HoldsImage(app.Status.Image) {
		t.Error("the serving image is not among the stored ones")
	}

	// A settled App is not read again inside the interval…
	published := app.Status.StoredImages
	r.refreshStoredImages(context.Background(), app, false)
	if reg.listings != 1 || app.Status.StoredImages != published {
		t.Fatalf("listings = %d, want 1: a settled App was read again", reg.listings)
	}
	// …but a new release reads at once, and an unchanged holding is not rewritten.
	r.refreshStoredImages(context.Background(), app, true)
	if reg.listings != 2 || app.Status.StoredImages != published {
		t.Fatalf("listings = %d, status rewritten = %v; want a second reading and the same status", reg.listings, app.Status.StoredImages != published)
	}
	// Once the interval has passed, retention's removals show up.
	reg.mu.Lock()
	delete(reg.tags, "gen-6")
	reg.mu.Unlock()
	r.storedImagesRead.Store(app.Namespace+"/"+app.Name, time.Now().Add(-time.Second))
	r.refreshStoredImages(context.Background(), app, false)
	if got := app.Status.StoredImages.Digests; !slices.Equal(got, []string{digestOf("a")}) {
		t.Fatalf("storedImages after retention = %v, want only the live build", got)
	}
}

// A reading that fails keeps the previous one: nil stays nil (unknown, which
// bex-api never refuses on), and a published list is not emptied.
func TestRefreshStoredImagesKeepsThePreviousReadingOnFailure(t *testing.T) {
	r, app, reg := storedImagesFixture(t, map[string]string{"gen-7": digestOf("a")})
	reg.pass = "rotated" // the credential the Secret holds is refused

	r.refreshStoredImages(context.Background(), app, true)
	if app.Status.StoredImages != nil {
		t.Fatalf("a failed first reading published %+v, want nothing", app.Status.StoredImages)
	}
	published := &appv1alpha1.StoredImagesStatus{Digests: []string{digestOf("a")}}
	app.Status.StoredImages = published
	r.refreshStoredImages(context.Background(), app, true)
	if app.Status.StoredImages != published {
		t.Fatalf("a failed reading replaced the published list with %+v", app.Status.StoredImages)
	}
}

// bex-api refuses on what is published, so a reading is published only when it
// is whole and plausible. A registry that could not be asked about one tag, or
// a listing without the build that is serving right now, keeps the previous
// reading: neither may read as "that build is gone".
func TestRefreshStoredImagesPublishesOnlyAWholePlausibleReading(t *testing.T) {
	for name, breakIt := range map[string]func(*fakeTagRegistry){
		"the registry fails for one tag":           func(f *fakeTagRegistry) { f.failTag = "gen-6" },
		"the listing lacks the serving build":      func(f *fakeTagRegistry) { delete(f.tags, "gen-7") },
		"the repository answers with no tags":      func(f *fakeTagRegistry) { f.tags = map[string]string{} },
		"the serving tag vanished during the read": func(f *fakeTagRegistry) { f.tags["gen-7"] = "" },
	} {
		r, app, reg := storedImagesFixture(t, map[string]string{"gen-7": digestOf("a"), "gen-6": digestOf("b")})
		r.refreshStoredImages(context.Background(), app, true)
		published := app.Status.StoredImages
		if published == nil {
			t.Fatalf("%s: the healthy first reading published nothing", name)
		}
		reg.mu.Lock()
		breakIt(reg)
		reg.mu.Unlock()
		r.refreshStoredImages(context.Background(), app, true)
		if app.Status.StoredImages != published {
			t.Errorf("%s: published %+v over the previous reading", name, app.Status.StoredImages)
		}
		// The failed reading is retried sooner than a good one is refreshed,
		// and not on the very next pass.
		before := reg.listings
		r.refreshStoredImages(context.Background(), app, false)
		if reg.listings != before {
			t.Errorf("%s: a failed reading was retried on the next pass", name)
		}
	}
}

// An image the registry was never asked to keep has no listing.
func TestRefreshStoredImagesLeavesExternalImagesAlone(t *testing.T) {
	r, app, reg := storedImagesFixture(t, map[string]string{"gen-7": digestOf("a")})
	app.Status.Image = "docker.io/traefik/whoami@" + digestOf("a")
	// A listing from when the service built its own images is dropped: it
	// describes a repository the service no longer runs from.
	app.Status.StoredImages = &appv1alpha1.StoredImagesStatus{Generation: 3, Digests: []string{digestOf("b")}}
	r.protectServingImage(context.Background(), app)
	if app.Status.StoredImages != nil || reg.listings != 0 {
		t.Fatalf("an external image kept or gained a listing: status %+v, listings %d", app.Status.StoredImages, reg.listings)
	}
	// Nor is anything read without the per-App credential.
	app.Status.Image = r.Registry + "/" + reg.repo + ":gen-7@" + digestOf("a")
	(&AppReconciler{Registry: r.Registry}).refreshStoredImages(context.Background(), app, true)
	if app.Status.StoredImages != nil {
		t.Fatalf("read without a per-App credential: %+v", app.Status.StoredImages)
	}
}

func TestStoredImagesHoldsOnlyPinnedReferences(t *testing.T) {
	s := &appv1alpha1.StoredImagesStatus{Digests: []string{digestOf("a")}}
	for image, want := range map[string]bool{
		"zot:5000/ws/web:gen-7@" + digestOf("a"): true,
		"zot:5000/ws/web@" + digestOf("a"):       true,
		"zot:5000/ws/web:gen-6@" + digestOf("b"): false,
		"zot:5000/ws/web:gen-7":                  false, // a tag names no build
		"":                                       false,
	} {
		if got := s.HoldsImage(image); got != want {
			t.Errorf("HoldsImage(%q) = %v, want %v", image, got, want)
		}
	}
	if (*appv1alpha1.StoredImagesStatus)(nil).HoldsImage("zot:5000/ws/web@" + digestOf("a")) {
		t.Error("a nil listing holds nothing; callers decide what unknown means")
	}
}
