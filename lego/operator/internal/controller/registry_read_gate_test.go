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
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// w21/m394 t014 (R25-04): a tenant's registry that accepts a connection and
// never answers holds the reconcile worker that reads it for the whole 5 s
// read budget, every attempt, from every App of the tenant. With 2 workers, 14
// such Apps kept both busy and every other tenant's reconcile queued behind
// them.

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func newTestGate() (*registryReadGate, *fakeClock) {
	clock := &fakeClock{now: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}
	g := newRegistryReadGate()
	g.now = clock.Now
	return g, clock
}

func TestRegistryReadGateCapsReadsPerHostAndPerWorkspace(t *testing.T) {
	g, _ := newTestGate()
	releaseA, _, ok := g.acquire("tarpit.example", "ws-1")
	if !ok {
		t.Fatal("first read refused")
	}
	if _, wait, ok := g.acquire("tarpit.example", "ws-2"); ok || wait <= 0 {
		t.Fatalf("a second read of one host = ok %v wait %v, want busy with a retry delay", ok, wait)
	}
	if _, _, ok := g.acquire("other.example", "ws-1"); ok {
		t.Fatal("a second read by one workspace was admitted")
	}
	releaseB, _, ok := g.acquire("other.example", "ws-2")
	if !ok {
		t.Fatal("an unrelated host and workspace was refused")
	}
	releaseA(false)
	releaseB(false)
	if _, _, ok := g.acquire("tarpit.example", "ws-1"); !ok {
		t.Fatal("a released host stayed busy")
	}
}

func TestRegistryReadGateBreaksAFailingHostWithBackoff(t *testing.T) {
	g, clock := newTestGate()
	fail := func(host string) {
		t.Helper()
		release, _, ok := g.acquire(host, "ws-"+host)
		if !ok {
			t.Fatalf("read of %s refused", host)
		}
		release(true)
	}
	fail("tarpit.example")
	if _, wait, ok := g.acquire("tarpit.example", "ws-1"); ok || wait != 30*time.Second {
		t.Fatalf("after one failure: ok %v wait %v, want refused for 30s", ok, wait)
	}
	// The healthy registries are untouched by it.
	for _, host := range []string{"registry-1.docker.io", "ghcr.io", "registry.bex"} {
		release, _, ok := g.acquire(host, "ws-"+host)
		if !ok {
			t.Fatalf("%s refused while another host's breaker is open", host)
		}
		release(false)
	}
	clock.Advance(31 * time.Second)
	fail("tarpit.example")
	if _, wait, ok := g.acquire("tarpit.example", "ws-1"); ok || wait != 60*time.Second {
		t.Fatalf("after two failures: ok %v wait %v, want refused for 60s", ok, wait)
	}
	clock.Advance(61 * time.Second)
	for range 8 {
		fail("tarpit.example")
		clock.Advance(imageRegistryRetryMax + time.Second) // past the window, inside the memory
	}
	fail("tarpit.example")
	if _, wait, _ := g.acquire("tarpit.example", "ws-1"); wait != imageRegistryRetryMax {
		t.Fatalf("breaker window = %v, want it capped at %v", wait, imageRegistryRetryMax)
	}
	// An answer, even a refusal, closes it.
	clock.Advance(imageRegistryRetryMax + time.Second)
	release, _, ok := g.acquire("tarpit.example", "ws-1")
	if !ok {
		t.Fatal("the breaker did not close after its window")
	}
	release(false)
	if release, _, ok := g.acquire("tarpit.example", "ws-1"); !ok {
		t.Fatal("a host that answered is still broken")
	} else {
		release(false)
	}
}

func TestRegistryReadGateForgetsAHostThatStayedQuiet(t *testing.T) {
	g, clock := newTestGate()
	fail := func() {
		t.Helper()
		release, _, ok := g.acquire("tarpit.example", "ws-1")
		if !ok {
			t.Fatal("read refused")
		}
		release(true)
	}
	fail()
	clock.Advance(31 * time.Second)
	fail()
	clock.Advance(61 * time.Second)
	fail() // three failures in a row: a 120 s window
	// Quiet for longer than the window plus the memory: the next failure is a first one.
	clock.Advance(120*time.Second + registryHostMemory + time.Second)
	fail()
	if _, wait, ok := g.acquire("tarpit.example", "ws-1"); ok || wait != imageRegistryRetry {
		t.Fatalf("after a long quiet: ok %v wait %v, want a fresh %v window", ok, wait, imageRegistryRetry)
	}
}

func TestRegistryReadGateForgetsIdleHosts(t *testing.T) {
	g, clock := newTestGate()
	for i := range 2000 {
		release, _, ok := g.acquire(fmt.Sprintf("tarpit-%d.example", i), fmt.Sprintf("ws-%d", i))
		if !ok {
			t.Fatal("refused")
		}
		release(true)
		clock.Advance(time.Second)
	}
	clock.Advance(time.Hour)
	release, _, _ := g.acquire("fresh.example", "ws-fresh")
	release(false)
	if n := len(g.hosts); n > 600 {
		t.Fatalf("the gate remembers %d hosts, want old failures forgotten", n)
	}
}

func TestPerAppRegistryBackoffDoublesAndResets(t *testing.T) {
	g, _ := newTestGate()
	got := make([]time.Duration, 0, 7)
	for range 7 {
		got = append(got, registryRetryDelay(g.appFailed("ns/app")))
	}
	want := []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second, 240 * time.Second,
		imageRegistryRetryMax, imageRegistryRetryMax, imageRegistryRetryMax}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("delays = %v, want %v", got, want)
		}
	}
	g.appSucceeded("ns/app")
	if d := registryRetryDelay(g.appFailed("ns/app")); d != imageRegistryRetry {
		t.Fatalf("after a success the delay is %v, want %v", d, imageRegistryRetry)
	}
	if other := registryRetryDelay(g.appFailed("ns/other")); other != imageRegistryRetry {
		t.Fatalf("another App's first failure = %v", other)
	}
}

// tarpitTransport answers fast.example at once from a real-shaped registry and
// holds every other host's read until release (or the request's own context).
type tarpitTransport struct {
	fast     *http.Client
	release  chan struct{}
	inflight atomic.Int32 // concurrent reads inside the tarpit
	peak     atomic.Int32
	entered  atomic.Int32
}

func (tp *tarpitTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host == "fast.example" {
		return tp.fast.Transport.RoundTrip(r)
	}
	now := tp.inflight.Add(1)
	defer tp.inflight.Add(-1)
	tp.entered.Add(1)
	for {
		peak := tp.peak.Load()
		if now <= peak || tp.peak.CompareAndSwap(peak, now) {
			break
		}
	}
	select {
	case <-tp.release:
	case <-r.Context().Done():
	}
	return nil, errors.New("tarpit: no answer")
}

func tarpitApp(workspace, name, image string) *appv1alpha1.App {
	app := webImageApp(1)
	app.Namespace, app.Name, app.Spec.Image = workspace, name, image
	app.UID = types.UID("uid-" + name)
	return app
}

// With two workers, a tenant whose registry never answers must leave a worker
// for everyone else: each call returns at once instead of waiting in the
// tarpit, and an unrelated tenant's read goes through while it is held.
func TestTarpitRegistryCannotHoldEveryWorker(t *testing.T) {
	tp := &tarpitTransport{fast: imageConfigRegistry(t, `"80/tcp":{}`), release: make(chan struct{})}
	r := &AppReconciler{HTTPClient: &http.Client{Transport: tp}, MaxConcurrentReconciles: 2}

	var wg sync.WaitGroup
	var busy atomic.Int32
	for i := range 10 {
		app := tarpitApp("ws-attacker", fmt.Sprintf("t%d", i), fmt.Sprintf("tarpit-%d.example/img:1", i%2))
		wg.Go(func() {
			_, _, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image)
			if _, turnedAway := errors.AsType[*registryBusyError](err); turnedAway {
				busy.Add(1)
			}
		})
	}
	NewWithT(t).Eventually(func() bool { return tp.entered.Load() > 0 }, 5*time.Second, 2*time.Millisecond).
		Should(BeTrue(), "no read reached the tarpit")
	// Let every other call finish: before the change each one waits inside.
	time.Sleep(300 * time.Millisecond)
	if peak := tp.peak.Load(); peak > 1 {
		t.Fatalf("%d reads sat in the tarpit at once, want at most 1 for one workspace", peak)
	}

	other := tarpitApp("ws-other", "healthy", "fast.example/img:1")
	start := time.Now()
	if _, port, err := r.resolveImageNetwork(t.Context(), other, other.Spec.Image); err != nil || port == 0 {
		t.Fatalf("an unrelated tenant's read while the tarpit is held = port %d, %v", port, err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("the unrelated read took %v", time.Since(start))
	}
	close(tp.release)
	wg.Wait()
	if got := busy.Load(); got < 8 {
		t.Fatalf("%d of 10 attacker reads came back as an immediate busy answer, want at least 8", got)
	}
}

// A tenant that holds several hosts is still one workspace: its second host
// does not get a second concurrent read.
func TestOneWorkspaceCannotReadSeveralTarpitsAtOnce(t *testing.T) {
	tp := &tarpitTransport{fast: imageConfigRegistry(t, `"80/tcp":{}`), release: make(chan struct{})}
	r := &AppReconciler{HTTPClient: &http.Client{Transport: tp}, MaxConcurrentReconciles: 2}
	var wg sync.WaitGroup
	for i := range 6 {
		app := tarpitApp("ws-attacker", fmt.Sprintf("m%d", i), fmt.Sprintf("many-%d.example/img:1", i))
		wg.Go(func() { _, _, _ = r.resolveImageNetwork(t.Context(), app, app.Spec.Image) })
	}
	time.Sleep(300 * time.Millisecond)
	if peak := tp.peak.Load(); peak > 1 {
		t.Fatalf("one workspace had %d reads inside tarpits at once, want 1", peak)
	}
	close(tp.release)
	wg.Wait()
}

// The platform's own reads and Docker Hub are not another tenant's problem: a
// broken host's breaker never touches them.
func TestFailingTenantHostDoesNotDelayDockerHubOrGHCR(t *testing.T) {
	tp := &tarpitTransport{fast: imageConfigRegistry(t, `"80/tcp":{}`), release: make(chan struct{})}
	close(tp.release) // the tarpit answers (with an error) at once, so its host trips the breaker
	r := &AppReconciler{HTTPClient: &http.Client{Transport: tp}, MaxConcurrentReconciles: 2}
	bad := tarpitApp("ws-a", "bad", "tarpit.example/img:1")
	if _, _, err := r.resolveImageNetwork(t.Context(), bad, bad.Spec.Image); err == nil {
		t.Fatal("a failing host resolved")
	}
	before := tp.entered.Load()
	again := tarpitApp("ws-b", "bad2", "tarpit.example/img:1")
	_, _, err := r.resolveImageNetwork(t.Context(), again, again.Spec.Image)
	var busy *registryBusyError
	if !errors.As(err, &busy) || busy.retryAfter <= 0 {
		t.Fatalf("a failing host's next read = %v, want a busy answer with a retry delay", err)
	}
	if tp.entered.Load() != before {
		t.Fatal("the open breaker still let a read reach the failing host")
	}
	for _, host := range []string{"fast.example"} {
		app := tarpitApp("ws-c", "ok-"+host, host+"/img:1")
		if _, _, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image); err != nil {
			t.Fatalf("%s read after a failing host = %v", host, err)
		}
	}
}

// imageRegistryWait backs a failing App off, doubling, and a busy answer is a
// short retry that does not count as a failure.
func TestImageRegistryWaitBacksOffPerApp(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := webImageApp(1)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).WithStatusSubresource(app).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme}
	transient := func() error {
		return registry.NewTransientError(errors.New("registry timeout"), 0)
	}
	got := make([]time.Duration, 0, 5)
	for range 5 {
		res, waiting := r.imageRegistryWait(t.Context(), app, transient())
		if !waiting {
			t.Fatal("a transient failure did not wait")
		}
		got = append(got, res.RequeueAfter)
	}
	want := []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second, 240 * time.Second, imageRegistryRetryMax}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("requeues = %v, want %v", got, want)
		}
	}
	// A busy answer retries soon and leaves the failure count alone.
	res, waiting := r.imageRegistryWait(t.Context(), app, &registryBusyError{retryAfter: 5 * time.Second})
	if !waiting || res.RequeueAfter < 5*time.Second || res.RequeueAfter > 10*time.Second {
		t.Fatalf("busy = %v waiting %v, want about 5s", res.RequeueAfter, waiting)
	}
	if res, _ := r.imageRegistryWait(t.Context(), app, transient()); res.RequeueAfter != imageRegistryRetryMax {
		t.Fatalf("a busy answer reset the failure count: next requeue %v", res.RequeueAfter)
	}
	ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || app.Status.Phase != appv1alpha1.PhaseDeploying {
		t.Fatalf("phase %q ready %+v, want Deploying with a waiting reason", app.Status.Phase, ready)
	}
	// A read that answered, successfully or with a refusal, starts the App over.
	for name, err := range map[string]error{"success": nil, "refusal": &registry.RefusedError{Status: http.StatusNotFound}} {
		if res, waiting := r.imageRegistryWait(t.Context(), app, err); waiting {
			t.Fatalf("%s waited %v", name, res.RequeueAfter)
		}
		if res, _ := r.imageRegistryWait(t.Context(), app, transient()); res.RequeueAfter != imageRegistryRetry {
			t.Fatalf("after a %s the next requeue = %v, want %v", name, res.RequeueAfter, imageRegistryRetry)
		}
	}
}

// A read the controller itself cancelled (a shutdown, a requeue) is not the
// registry's failure: it must not open the host's breaker.
func TestACancelledReadDoesNotOpenTheBreaker(t *testing.T) {
	tp := &tarpitTransport{fast: imageConfigRegistry(t, `"80/tcp":{}`), release: make(chan struct{})}
	r := &AppReconciler{HTTPClient: &http.Client{Transport: tp}}
	ctx, cancel := context.WithCancel(t.Context())
	app := tarpitApp("ws-a", "cancelled", "tarpit.example/img:1")
	done := make(chan error, 1)
	go func() {
		_, _, err := r.resolveImageNetwork(ctx, app, app.Spec.Image)
		done <- err
	}()
	NewWithT(t).Eventually(func() bool { return tp.entered.Load() > 0 }, 5*time.Second, 2*time.Millisecond).
		Should(BeTrue(), "the read never reached the tarpit")
	cancel()
	if err := <-done; err == nil {
		t.Fatal("a cancelled read resolved")
	}
	if release, wait, ok := r.registryReads().acquire("tarpit.example", "ws-b"); !ok {
		t.Fatalf("the host is closed for %v after a read this process cancelled", wait)
	} else {
		release(false)
	}
}
