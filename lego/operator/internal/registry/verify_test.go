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

package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// fakeZot mimics the zot behaviors that matter here: it authenticates against
// a fixed in-memory user set loaded "at startup" (ignoring later credential
// writes until "restarted" via loadUser), and can simulate an authenticated
// user whose per-repo ACL entry is missing (403).
type fakeZot struct {
	mu        sync.Mutex
	users     map[string]string
	aclDenied bool
}

//nolint:unparam // every test loads the fixture's app-hello user; the signature mirrors the htpasswd shape
func (z *fakeZot) loadUser(user, pass string) {
	z.mu.Lock()
	defer z.mu.Unlock()
	if z.users == nil {
		z.users = map[string]string{}
	}
	z.users[user] = pass
}

func (z *fakeZot) denyACL(denied bool) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.aclDenied = denied
}

func (z *fakeZot) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		z.mu.Lock()
		want, known := z.users[user]
		denied := z.aclDenied
		z.mu.Unlock()
		if !ok || !known || want != pass {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if denied {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		// Authenticated + authorized but the repo has never been pushed —
		// the state the probe sees before the first build completes.
		w.WriteHeader(http.StatusNotFound)
	})
}

// newVerifyFixture builds a Creds wired to a fake zot server and a fake
// cluster holding the App's pull Secret, one labeled zot pod, and the zot
// htpasswd/config Secrets (so EnsureCreds/RotateCreds work too).
func newVerifyFixture(t *testing.T, zot *fakeZot, grace time.Duration) (*Creds, client.Client) {
	t.Helper()
	srv := httptest.NewServer(zot.handler())
	t.Cleanup(srv.Close)
	registryHost := strings.TrimPrefix(srv.URL, "http://")

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	creds := &Creds{
		ZotNamespace:    "reg",
		HTPasswdName:    "zot-htpasswd",
		ConfigName:      "zot-config",
		Registry:        registryHost,
		ActivationGrace: grace,
	}
	pullSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: PullSecretName("hello"), Namespace: "default"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: creds.dockerConfig(ZotUsername("hello"), "pw123"),
		},
	}
	htpasswd := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: "reg"},
		Data:       map[string][]byte{htpasswdKey: []byte("bex-builder:$2y$10$fakehash\n")},
	}
	zotPod := newZotPod()
	creds.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(pullSecret, htpasswd, zotPod).Build()
	return creds, creds.Client
}

func newZotPod() *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "zot-0", Namespace: "reg",
		Labels: map[string]string{ZotPodLabelKey: ZotPodLabelValue},
	}}
}

func zotPodExists(t *testing.T, cl client.Client) bool {
	t.Helper()
	var pod corev1.Pod
	err := cl.Get(context.Background(), client.ObjectKey{Namespace: "reg", Name: "zot-0"}, &pod)
	return err == nil
}

func TestEnsureActiveAcceptedCredentialMemoizes(t *testing.T) {
	zot := &fakeZot{}
	zot.loadUser("app-hello", "pw123")
	c, cl := newVerifyFixture(t, zot, time.Hour)

	active, err := c.EnsureActive(context.Background(), "hello", "default")
	if err != nil {
		t.Fatalf("EnsureActive: %v", err)
	}
	if !active {
		t.Fatal("credential loaded in zot must report active")
	}
	if !zotPodExists(t, cl) {
		t.Fatal("no bounce expected for an accepted credential")
	}

	// Memoized: even if zot "forgets" the user (which a real restart cannot
	// cause — it re-reads the current Secrets), EnsureActive stays true with
	// no probe until rotation/revocation invalidates it.
	zot.mu.Lock()
	zot.users = nil
	zot.mu.Unlock()
	if active, err := c.EnsureActive(context.Background(), "hello", "default"); err != nil || !active {
		t.Fatalf("memoized credential: active=%v err=%v; want active with no probe", active, err)
	}
}

func TestEnsureActiveRejectedWithinGraceDoesNotBounce(t *testing.T) {
	for name, setup := range map[string]func(*fakeZot){
		"user not loaded (401)": func(*fakeZot) {},
		"acl not loaded (403)":  func(z *fakeZot) { z.loadUser("app-hello", "pw123"); z.denyACL(true) },
	} {
		t.Run(name, func(t *testing.T) {
			zot := &fakeZot{}
			setup(zot)
			c, cl := newVerifyFixture(t, zot, time.Hour)

			active, err := c.EnsureActive(context.Background(), "hello", "default")
			if err != nil {
				t.Fatalf("EnsureActive: %v", err)
			}
			if active {
				t.Fatal("rejected credential must not report active")
			}
			if !zotPodExists(t, cl) {
				t.Fatal("must not bounce inside the activation grace window")
			}
		})
	}
}

func TestEnsureActiveRejectedPastGraceBouncesOnceUnderCooldown(t *testing.T) {
	zot := &fakeZot{}
	c, cl := newVerifyFixture(t, zot, time.Nanosecond)

	// First rejection starts the grace clock; no bounce yet.
	if active, err := c.EnsureActive(context.Background(), "hello", "default"); err != nil || active {
		t.Fatalf("first probe: active=%v err=%v; want pending", active, err)
	}
	if !zotPodExists(t, cl) {
		t.Fatal("first rejected probe must not bounce")
	}

	// Past the (1ns) grace: bounce — the zot pod is deleted.
	time.Sleep(2 * time.Millisecond)
	if active, err := c.EnsureActive(context.Background(), "hello", "default"); err != nil || active {
		t.Fatalf("second probe: active=%v err=%v; want pending", active, err)
	}
	if zotPodExists(t, cl) {
		t.Fatal("rejected past grace must bounce the zot pod")
	}

	// Recreate the pod (as the StatefulSet would); still rejected, but the
	// default 2m cooldown forbids a second bounce.
	if err := cl.Create(context.Background(), newZotPod()); err != nil {
		t.Fatal(err)
	}
	if active, err := c.EnsureActive(context.Background(), "hello", "default"); err != nil || active {
		t.Fatalf("third probe: active=%v err=%v; want pending", active, err)
	}
	if !zotPodExists(t, cl) {
		t.Fatal("bounce must be rate-limited by the cooldown")
	}

	// Once zot "restarts" with the credential loaded, activation completes and
	// the per-App rejection state clears.
	zot.loadUser("app-hello", "pw123")
	if active, err := c.EnsureActive(context.Background(), "hello", "default"); err != nil || !active {
		t.Fatalf("post-restart probe: active=%v err=%v; want active", active, err)
	}
	c.mu.Lock()
	stillAnchored := !c.apps["hello"].anchor.IsZero()
	c.mu.Unlock()
	if stillAnchored {
		t.Fatal("accepted credential must clear the rejection anchor")
	}
}

func TestEnsureActiveGraceAnchorsToCredentialWrite(t *testing.T) {
	zot := &fakeZot{}
	c, cl := newVerifyFixture(t, zot, time.Nanosecond)

	// EnsureCreds writes the htpasswd entry (fixture htpasswd lacks app-hello)
	// and stamps the write moment — the grace anchor.
	if err := c.EnsureCreds(context.Background(), "hello", "default"); err != nil {
		t.Fatalf("EnsureCreds: %v", err)
	}
	time.Sleep(2 * time.Millisecond) // "the build runs" — grace elapses before any probe

	// The very first rejected probe may bounce: no dead wait re-anchored to it.
	if active, err := c.EnsureActive(context.Background(), "hello", "default"); err != nil || active {
		t.Fatalf("probe: active=%v err=%v; want pending", active, err)
	}
	if zotPodExists(t, cl) {
		t.Fatal("write-anchored grace already elapsed — the first rejected probe must bounce")
	}
}

func TestRotateCredsInvalidatesActivation(t *testing.T) {
	zot := &fakeZot{}
	zot.loadUser("app-hello", "pw123")
	c, _ := newVerifyFixture(t, zot, time.Hour)

	if active, err := c.EnsureActive(context.Background(), "hello", "default"); err != nil || !active {
		t.Fatalf("initial activation: active=%v err=%v", active, err)
	}

	// Rotation rewrites the password; zot still holds the old hash, so the
	// memoized acceptance must be dropped and the next probe must reject.
	if err := c.RotateCreds(context.Background(), "hello", "default"); err != nil {
		t.Fatalf("RotateCreds: %v", err)
	}
	if active, err := c.EnsureActive(context.Background(), "hello", "default"); err != nil || active {
		t.Fatalf("post-rotation: active=%v err=%v; want pending (zot has the old hash)", active, err)
	}
}

func TestEnsureActiveUnreachableRegistryErrsWithoutBounce(t *testing.T) {
	zot := &fakeZot{}
	c, cl := newVerifyFixture(t, zot, time.Nanosecond)
	// Point the probe at a closed port; the pull Secret still matches c.Registry
	// for password extraction, so rewrite Registry only after fixture setup is
	// not possible — instead shut the server down by replacing the client with
	// one that always fails.
	c.HTTPClient = &http.Client{Transport: failingTransport{}}

	time.Sleep(2 * time.Millisecond)
	if _, err := c.EnsureActive(context.Background(), "hello", "default"); err == nil {
		t.Fatal("unreachable registry must surface an error, not a stale verdict")
	}
	if !zotPodExists(t, cl) {
		t.Fatal("an unreachable registry must never trigger a bounce")
	}
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, http.ErrServerClosed
}

func TestEnsureActiveMissingPullSecretErrs(t *testing.T) {
	zot := &fakeZot{}
	c, _ := newVerifyFixture(t, zot, time.Hour)
	if _, err := c.EnsureActive(context.Background(), "ghost", "default"); err == nil {
		t.Fatal("missing pull secret must error")
	}
}
