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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/operator/internal/identity"
)

// projectedZot models zot v2.1.22 behind a kubelet Secret mount (w4/m347).
// zot re-reads htpasswd and the accessControl block whenever the mounted files
// change, so it serves whatever the kubelet last projected, with no restart.
// The kubelet projects a Secret write when the pod object changes (a nudge);
// its minute-long periodic sync is outside a unit test's horizon, so here a
// write the operator never follows with a nudge is never seen. Authorization
// follows the per-repo ACL, so it also answers the isolation question: one
// App's credential on another App's repository.
type projectedZot struct {
	mu                 sync.Mutex
	htpasswd, document []byte
}

// project copies the operator's two Secrets into the "mounted files".
func (z *projectedZot) project(ctx context.Context, cl client.Client) error {
	var htpasswd, config corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "reg", Name: "zot-htpasswd"}, &htpasswd); err != nil {
		return err
	}
	if err := client.IgnoreNotFound(cl.Get(ctx, client.ObjectKey{Namespace: "reg", Name: "zot-config"}, &config)); err != nil {
		return err
	}
	z.mu.Lock()
	defer z.mu.Unlock()
	z.htpasswd, z.document = htpasswd.Data[htpasswdKey], config.Data[zotConfigKey]
	return nil
}

func (z *projectedZot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, pass, ok := r.BasicAuth()
	repo, found := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, "/v2/"), "/tags/list")
	z.mu.Lock()
	htpasswd, document := z.htpasswd, z.document
	z.mu.Unlock()
	hash, known := htpasswdUserHash(htpasswd, user)
	if !ok || !found || !known || bcrypt.CompareHashAndPassword(hash, []byte(pass)) != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var doc map[string]any
	_ = json.Unmarshal(document, &doc)
	if zotRepoHasUser(doc, repo, user) {
		w.WriteHeader(http.StatusNotFound) // authorized; nothing pushed yet
		return
	}
	w.WriteHeader(http.StatusForbidden)
}

// countZotPodOps wraps base so pod patches (nudges) and deletes (restarts) are
// counted; afterNudge, when set, runs after each successful pod patch.
func countZotPodOps(base client.Client, afterNudge func(context.Context, client.WithWatch) error) (cl client.Client, nudges, restarts *atomic.Int32) {
	nudges, restarts = &atomic.Int32{}, &atomic.Int32{}
	cl = interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, cw client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if err := cw.Patch(ctx, obj, patch, opts...); err != nil {
				return err
			}
			if _, ok := obj.(*corev1.Pod); ok {
				nudges.Add(1)
				if afterNudge != nil {
					return afterNudge(ctx, cw)
				}
			}
			return nil
		},
		Delete: func(ctx context.Context, cw client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if _, ok := obj.(*corev1.Pod); ok {
				restarts.Add(1)
			}
			return cw.Delete(ctx, obj, opts...)
		},
	})
	return cl, nudges, restarts
}

// newReloadFixture is newVerifyFixture against projectedZot, counting the
// nudges (zot pod patches, each of which projects the Secrets) and restarts
// (zot pod deletes) the operator issues.
func newReloadFixture(t *testing.T) (c *Creds, cl client.Client, nudges, restarts *atomic.Int32) {
	t.Helper()
	c, base := newVerifyFixture(t, &fakeZot{}, 0)
	zot := &projectedZot{}
	cl, nudges, restarts = countZotPodOps(base, func(ctx context.Context, cw client.WithWatch) error {
		return zot.project(ctx, cw)
	})
	c.Client = cl
	// The storage block is not under test: mark it loaded so its one-time
	// restart (w8/m156) does not count as an activation restart.
	c.storageActive = true
	if err := zot.project(context.Background(), cl); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(zot)
	t.Cleanup(srv.Close)
	c.Registry = strings.TrimPrefix(srv.URL, "http://")
	return c, cl, nudges, restarts
}

func zotPodNudge(t *testing.T, cl client.Client) string {
	t.Helper()
	var pod corev1.Pod
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "reg", Name: "zot-0"}, &pod); err != nil {
		t.Fatalf("zot pod: %v", err)
	}
	return pod.Annotations[ZotReloadAnnotation]
}

// A new App's credential is live on the first probe: the write nudges zot to
// reload, nothing waits on a grace or cooldown, and the registry keeps running.
func TestNewCredentialActiveOnFirstProbeWithoutRestart(t *testing.T) {
	c, cl, nudges, restarts := newReloadFixture(t)
	ctx := context.Background()

	if err := c.EnsureCreds(ctx, "fresh", "default"); err != nil {
		t.Fatal(err)
	}
	if zotPodNudge(t, cl) == "" {
		t.Fatal("writing a credential must nudge the zot pod to reload")
	}
	active, err := c.EnsureActive(ctx, "fresh", "default")
	if err != nil || !active {
		t.Fatalf("first probe after the write: active=%v err=%v, want active", active, err)
	}
	if restarts.Load() != 0 || !zotPodExists(t, cl) {
		t.Fatal("activating a new credential restarted zot")
	}
	if nudges.Load() != 1 {
		t.Fatalf("nudges = %d, want exactly one for one write", nudges.Load())
	}
}

// Six services created together each clear the gate on their first probe,
// none waiting on a restart cooldown.
func TestConcurrentNewAppsActivateWithoutCooldown(t *testing.T) {
	c, cl, _, restarts := newReloadFixture(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := range 6 {
		wg.Go(func() {
			name := fmt.Sprintf("svc%d", i)
			for {
				// Both zot Secrets are shared: a lost race surfaces as a
				// conflict requeue, as it does in the reconciler.
				err := c.EnsureCreds(ctx, name, "default")
				if err == nil {
					break
				}
				if !errors.Is(err, ErrConflictRequeue) {
					errs <- err
					return
				}
			}
			if active, err := c.EnsureActive(ctx, name, "default"); err != nil || !active {
				errs <- fmt.Errorf("%s first probe: active=%v err=%v", name, active, err)
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if restarts.Load() != 0 || !zotPodExists(t, cl) {
		t.Fatal("concurrent new Apps restarted zot")
	}
}

// Isolation holds through hot reload: App A's credential is refused on App B's
// repository, and a revoked credential is refused, with no restart either way.
func TestIsolationAndRevocationWithoutRestart(t *testing.T) {
	c, cl, _, restarts := newReloadFixture(t)
	ctx := context.Background()
	for _, name := range []string{"alpha", "beta"} {
		if err := c.EnsureCreds(ctx, name, "default"); err != nil {
			t.Fatal(err)
		}
	}
	alphaPass, err := c.readPasswordFor(ctx, identity.ForApp("alpha", ""), "default")
	if err != nil {
		t.Fatal(err)
	}
	status := func(repo string) int {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, "http://"+c.Registry+"/v2/"+repo+"/tags/list", nil)
		req.SetBasicAuth(ZotUsername("alpha"), alphaPass)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	if got := status(identity.ForApp("alpha", "").Repo()); got != http.StatusNotFound {
		t.Fatalf("alpha on its own repository = %d, want 404 (authorized, empty)", got)
	}
	if got := status(identity.ForApp("beta", "").Repo()); got != http.StatusForbidden && got != http.StatusUnauthorized {
		t.Fatalf("alpha on beta's repository = %d, want 401/403", got)
	}

	before := zotPodNudge(t, cl)
	time.Sleep(time.Millisecond) // the nudge stamp has nanosecond resolution
	if err := c.RevokeCreds(ctx, "alpha"); err != nil {
		t.Fatal(err)
	}
	if got := status(identity.ForApp("alpha", "").Repo()); got != http.StatusUnauthorized {
		t.Fatalf("revoked alpha on its own repository = %d, want 401", got)
	}
	if zotPodNudge(t, cl) == before {
		t.Fatal("revocation must nudge zot to reload")
	}
	if restarts.Load() != 0 || !zotPodExists(t, cl) {
		t.Fatal("revocation restarted zot")
	}
}

// The restart survives only as a last resort: a credential zot keeps rejecting
// is re-nudged (rate-limited) inside the grace, and only past it is zot
// restarted, counted on the metric the alert reads.
func TestRestartOnlyWhenHotReloadFails(t *testing.T) {
	zot := &fakeZot{} // never learns the credential: hot reload is broken
	c, base := newVerifyFixture(t, zot, time.Hour)
	var nudges, restarts *atomic.Int32
	c.Client, nudges, restarts = countZotPodOps(base, nil)
	ctx := context.Background()
	restartsBefore := testutil.ToFloat64(activationRestartsTotal)

	for range 3 {
		if active, err := c.EnsureActive(ctx, "hello", "default"); err != nil || active {
			t.Fatalf("rejected credential: active=%v err=%v", active, err)
		}
	}
	if nudges.Load() != 1 {
		t.Fatalf("nudges inside one nudge interval = %d, want 1", nudges.Load())
	}
	if restarts.Load() != 0 || testutil.ToFloat64(activationRestartsTotal) != restartsBefore {
		t.Fatal("zot was restarted inside the activation grace")
	}

	c.ActivationGrace = time.Nanosecond
	if active, err := c.EnsureActive(ctx, "hello", "default"); err != nil || active {
		t.Fatalf("past grace: active=%v err=%v", active, err)
	}
	if restarts.Load() != 1 {
		t.Fatal("a credential still rejected past the grace must fall back to restarting zot")
	}
	if got := testutil.ToFloat64(activationRestartsTotal) - restartsBefore; got != 1 {
		t.Fatalf("bex_registry_activation_restarts_total rose by %v, want 1", got)
	}
}

// A failed nudge (no pod, or a missing RBAC grant during a rollout) never fails
// the credential write: the kubelet's periodic sync still delivers it.
func TestNudgeFailureDoesNotFailTheWrite(t *testing.T) {
	c, cl, _, _ := newReloadFixture(t)
	ctx := context.Background()
	if err := cl.Delete(ctx, newZotPod()); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureCreds(ctx, "fresh", "default"); err != nil {
		t.Fatalf("EnsureCreds with no zot pod to nudge: %v", err)
	}
}
