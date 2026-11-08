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

package store

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w2/m254 (ADR097): a linked service's row projects spec.buildSource — its
// carrier's CR, named by the carrier's bds- id — and a newly selected build
// reaches an existing CR through applyOwnedSpec, as any owned field does.
func TestProjectorCarriesTheBuildSourceLink(t *testing.T) {
	d := DesiredApp{App: App{TenantID: "tea-a", Name: "web", Type: "web_service", BuildSourceID: "bds-1", BuildSourceImage: "zot/tea-a/src@sha256:aaa"}}
	spec := projectSpec(d)
	if spec.BuildSource == nil || spec.BuildSource.Name != CRName("tea-a", "bds-1") || spec.BuildSource.Image != "zot/tea-a/src@sha256:aaa" {
		t.Fatalf("spec.buildSource = %+v", spec.BuildSource)
	}
	if plain := projectSpec(DesiredApp{App: App{TenantID: "tea-a", Name: "api", Image: "nginx"}}); plain.BuildSource != nil {
		t.Fatalf("an unlinked service projected %+v", plain.BuildSource)
	}

	live := spec
	d.BuildSourceImage = "zot/tea-a/src@sha256:bbb"
	if !applyOwnedSpec(&live, projectSpec(d)) || live.BuildSource.Image != "zot/tea-a/src@sha256:bbb" {
		t.Fatalf("selecting the next build did not reach the CR: %+v", live.BuildSource)
	}
	if applyOwnedSpec(&live, projectSpec(d)) {
		t.Error("an unchanged link reported a change")
	}
}

type fakeBuildFanout struct {
	mu    sync.Mutex
	calls []string
	err   error // what BuildSucceeded returns
	c     chan struct{}
}

func (f *fakeBuildFanout) BuildSucceeded(_ context.Context, b BuiltSource) error {
	f.mu.Lock()
	f.calls = append(f.calls, b.Namespace+"/"+b.Carrier+"@"+b.Image+"#"+b.Commit.Hash)
	err := f.err
	f.mu.Unlock()
	f.c <- struct{}{}
	return err
}

// A carrier's deploy closing live fans its build out exactly once; a
// service's own deploy never does (ADR097 §5).
func TestRecordDeployFansOutACarrierBuildOnce(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	fanout := &fakeBuildFanout{c: make(chan struct{}, 8)}
	rec.BuildFanout = fanout
	ten, _ := st.CreateTenant(ctx, "acme", "free")
	_, _ = st.CreateApp(ctx, App{TenantID: ten.ID, Name: "bds-1", Type: TypeBuildSource, Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	_, _ = st.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var list appv1alpha1.AppList
	if err := cl.List(ctx, &list); err != nil || len(list.Items) != 2 {
		t.Fatalf("apps = %d, %v; want 2", len(list.Items), err)
	}
	carrier := ""
	for i := range list.Items {
		a := &list.Items[i]
		a.Status.Phase = appv1alpha1.PhaseRunning
		if a.Spec.Type == appv1alpha1.TypeBuildSource {
			a.Status.Image = "zot/src@sha256:built"
			carrier = a.Namespace + "/" + a.Name + "@" + a.Status.Image + "#"
		}
		if err := cl.Status().Update(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := rec.ReconcileOnce(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
	}
	select {
	case <-fanout.c:
	case <-time.After(2 * time.Second):
		t.Fatal("BuildSucceeded was not called within 2s")
	}
	if err := rec.ReconcileOnce(ctx); err != nil { // a later pass over the closed row
		t.Fatalf("reconcile: %v", err)
	}
	select {
	case <-fanout.c:
		t.Fatal("BuildSucceeded fired twice for one build")
	case <-time.After(200 * time.Millisecond):
	}
	fanout.mu.Lock()
	defer fanout.mu.Unlock()
	if len(fanout.calls) != 1 || fanout.calls[0] != carrier {
		t.Fatalf("fan-out calls = %v, want exactly [%s]", fanout.calls, carrier)
	}
}

// w9/m180 t002: a fan-out that did not complete (a failed service, or a
// bex-api restart mid run, which leaves the claim without a completion) is
// re-run by the resync once its lease expires, exactly once, and a live lease
// keeps the resync away.
func TestResyncFinishesAnInterruptedFanoutOnce(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	fanout := &fakeBuildFanout{c: make(chan struct{}, 16), err: errors.New("service web: transient")}
	rec.BuildFanout = fanout
	rec.FanoutLease = time.Hour
	ten, _ := st.CreateTenant(ctx, "acme", "free")
	_, _ = st.CreateApp(ctx, App{TenantID: ten.ID, Name: "bds-1", Type: TypeBuildSource, Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var list appv1alpha1.AppList
	if err := cl.List(ctx, &list); err != nil || len(list.Items) != 1 {
		t.Fatalf("apps = %d, %v; want 1", len(list.Items), err)
	}
	carrier := &list.Items[0]
	carrier.Status.Phase = appv1alpha1.PhaseRunning
	carrier.Status.Image = "zot/src@sha256:built"
	if err := cl.Status().Update(ctx, carrier); err != nil {
		t.Fatal(err)
	}
	calls := func() int {
		rec.fanouts.Wait()
		fanout.mu.Lock()
		defer fanout.mu.Unlock()
		return len(fanout.calls)
	}
	for range 2 {
		if err := rec.ReconcileOnce(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
	}
	if n := calls(); n != 1 {
		t.Fatalf("the closing pass fanned out %d times, want 1", n)
	}

	// The failed run left the build open but its lease is live: no re-run.
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if n := calls(); n != 1 {
		t.Fatalf("the resync re-ran under a live lease: %d calls", n)
	}

	// The lease expires (as after a restart): the resync re-runs it once, the
	// run completes, and later passes leave it alone.
	fanout.mu.Lock()
	fanout.err = nil
	fanout.mu.Unlock()
	rec.FanoutLease = time.Nanosecond
	for range 3 {
		if err := rec.ReconcileOnce(ctx); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
		rec.fanouts.Wait()
	}
	fanout.mu.Lock()
	defer fanout.mu.Unlock()
	want := carrier.Namespace + "/" + carrier.Name + "@zot/src@sha256:built#"
	if len(fanout.calls) != 2 || fanout.calls[1] != want {
		t.Fatalf("fan-out calls = %v, want the failed run then exactly one re-run of %s", fanout.calls, want)
	}
}
