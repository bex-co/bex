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
	c     chan struct{}
}

func (f *fakeBuildFanout) BuildSucceeded(_ context.Context, b BuiltSource) {
	f.mu.Lock()
	f.calls = append(f.calls, b.Namespace+"/"+b.Carrier+"@"+b.Image)
	f.mu.Unlock()
	f.c <- struct{}{}
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
			carrier = a.Namespace + "/" + a.Name + "@" + a.Status.Image
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
