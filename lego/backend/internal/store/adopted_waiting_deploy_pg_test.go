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
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// adoptedCtx is ctx carrying an App whose operator reports release generation
// adopted.
func adoptedCtx(ctx context.Context, generation int64) context.Context {
	return WithAdoptedRelease(ctx, &appv1alpha1.App{Status: appv1alpha1.AppStatus{ReleaseGeneration: generation}})
}

// w8/396: once the deploy ahead serves, the operator adopts the deploy waiting
// behind it and runs it to the end, but the reconciler frees its pending slot
// only on its next pass. A deploy opened in between, told the App's adopted
// release, waits behind it rather than canceling it; a waiting deploy the
// operator has not adopted is still replaced.
func TestPGAdoptedWaitingDeployIsNotReplaced(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	row, first := deferredRestartApp(t, st)
	if _, err := st.TransitionDeploy(ctx, first.ID, DeployLive, RanImage{}, "", "", "", nil); err != nil {
		t.Fatal(err)
	}
	ahead, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", first.Generation+1, CommitInfo{}, "")
	if err != nil || ahead.Status != DeployCreated {
		t.Fatalf("deploy ahead = %+v (err %v)", ahead, err)
	}
	waiting, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", first.Generation+2, CommitInfo{}, "")
	if err != nil || waiting.Status != DeployQueued || !waiting.OverlapPending {
		t.Fatalf("waiting deploy = %+v (err %v), want the pending slot", waiting, err)
	}
	if _, err := st.TransitionDeploy(ctx, ahead.ID, DeployLive, RanImage{}, "", "", "", nil); err != nil {
		t.Fatal(err)
	}
	stillWaiting := func(when string) {
		t.Helper()
		got, err := st.GetDeploy(ctx, row.ID, waiting.ID)
		if err != nil || got.Status != DeployQueued || got.OverlapPending || got.CancelReason != "" {
			t.Fatalf("%s: adopted deploy = %+v (err %v), want it executing (queued, out of the pending slot)", when, got, err)
		}
	}

	// A Restart (and any deploy that waits at the row level) waits behind it.
	restart, deferred, err := st.CreateDeferredDeploy(adoptedCtx(ctx, waiting.Generation), Deploy{AppID: row.ID, Trigger: TriggerRestart}, DeferredRequest{})
	if err != nil || !deferred {
		t.Fatalf("restart behind an adopted waiting deploy: deferred=%v err=%v, want it to wait", deferred, err)
	}
	stillWaiting("after a restart")

	// A deploy replaces only the Restart waiting behind it.
	next, err := st.CreateDeploy(adoptedCtx(ctx, waiting.Generation), row.ID, TriggerAPI, "", first.Generation+3, CommitInfo{}, "")
	if err != nil || next.Status != DeployQueued || !next.OverlapPending {
		t.Fatalf("deploy behind an adopted waiting deploy = %+v (err %v), want queued in the pending slot", next, err)
	}
	stillWaiting("after a deploy")
	if got, _ := st.GetDeploy(ctx, row.ID, restart.ID); got.Status != DeployCanceled || SupersedingDeployID(got.CancelReason) != next.ID {
		t.Fatalf("waiting restart = %s (%q), want replaced by %s", got.Status, got.CancelReason, next.ID)
	}

	// The control: a waiting deploy the operator has not adopted is replaced.
	last, err := st.CreateDeploy(adoptedCtx(ctx, waiting.Generation), row.ID, TriggerAPI, "", first.Generation+4, CommitInfo{}, "")
	if err != nil || last.Status != DeployQueued {
		t.Fatalf("newest deploy = %+v (err %v), want queued", last, err)
	}
	if got, _ := st.GetDeploy(ctx, row.ID, next.ID); got.Status != DeployCanceled || SupersedingDeployID(got.CancelReason) != last.ID {
		t.Fatalf("unadopted waiting deploy = %s (%q), want replaced by %s", got.Status, got.CancelReason, last.ID)
	}
	stillWaiting("after the control")
}

// w8/396: without the App's adopted release, or while the deploy ahead is
// still recorded executing, the pending slot is left to the reconciler: two
// executing rows would break the one-executing-deploy rule. w8/397: the wait
// for the reconciler to record the deploy ahead is bounded, never records its
// verdict itself, and then opens the deploy as before.
func TestPGWaitingDeployAdoptionNeedsAFreeSlot(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	st.DeployAheadRecordWait = 300 * time.Millisecond
	row, first := deferredRestartApp(t, st)
	waiting, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", first.Generation+1, CommitInfo{}, "")
	if err != nil || !waiting.OverlapPending {
		t.Fatalf("waiting deploy = %+v (err %v), want the pending slot", waiting, err)
	}
	start := time.Now()
	next, err := st.CreateDeploy(adoptedCtx(ctx, waiting.Generation), row.ID, TriggerAPI, "", first.Generation+2, CommitInfo{}, "")
	if err != nil || next.Status != DeployQueued {
		t.Fatalf("deploy while the first is still recorded executing = %+v (err %v), want queued", next, err)
	}
	if took := time.Since(start); took < st.DeployAheadRecordWait || took > 5*time.Second {
		t.Fatalf("the deploy opened after %s, want it to wait out DeployAheadRecordWait (%s) and no longer", took, st.DeployAheadRecordWait)
	}
	if got, _ := st.GetDeploy(ctx, row.ID, first.ID); got.Status != first.Status || got.OverlapPending {
		t.Fatalf("deploy recorded executing = %+v, want untouched", got)
	}

	if _, err := st.TransitionDeploy(ctx, first.ID, DeployLive, RanImage{}, "", "", "", nil); err != nil {
		t.Fatal(err)
	}
	last, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", first.Generation+3, CommitInfo{}, "")
	if err != nil || last.Status != DeployCreated {
		t.Fatalf("deploy with no adopted release named = %+v (err %v), want it to replace the waiting one and start", last, err)
	}
	if got, _ := st.GetDeploy(ctx, row.ID, next.ID); got.Status != DeployCanceled || SupersedingDeployID(got.CancelReason) != last.ID {
		t.Fatalf("waiting deploy = %s (%q), want replaced by %s", got.Status, got.CancelReason, last.ID)
	}
}

// aheadUnrecorded is w8/397's window: deploy A served and the operator at once
// adopted B, the deploy waiting behind it, but the reconciler has not recorded
// A's verdict yet, so A's row still reads executing and B still holds the
// pending slot. cr is the App as the operator reports it. A kick runs the
// reconciler's recording of A from cr, as the kicked pass does; kicked counts
// the kicks, and passes receives each pass once it has finished.
type aheadUnrecorded struct {
	st     *PGStore
	row    App
	a, b   Deploy
	cr     *appv1alpha1.App
	kicked *atomic.Int32
	passes chan struct{}
}

func newAheadUnrecorded(t *testing.T) aheadUnrecorded {
	t.Helper()
	ctx := context.Background()
	st := openLifecyclePG(t)
	row, a := deferredRestartApp(t, st)
	if _, err := st.TransitionDeploy(ctx, a.ID, DeployUpdateInProgress, RanImage{}, "", "", "", nil); err != nil {
		t.Fatal(err)
	}
	b, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "", a.Generation+1, CommitInfo{}, "")
	if err != nil || !b.OverlapPending {
		t.Fatalf("B = %+v (err %v), want it waiting in the pending slot behind A", b, err)
	}
	f := aheadUnrecorded{st: st, row: row, b: b, kicked: new(atomic.Int32), passes: make(chan struct{}, 8), cr: &appv1alpha1.App{
		Status: appv1alpha1.AppStatus{ReleaseGeneration: b.Generation, ActiveRevision: fmt.Sprintf("rev-%d", a.Generation)},
	}}
	f.cr.Name, f.cr.Namespace = row.Name, "default"
	if f.a, err = st.GetDeploy(ctx, row.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	rec := NewReconciler(fake.NewClientBuilder().WithScheme(scheme).Build(), st)
	st.KickReconciler = func() {
		f.kicked.Add(1)
		go func() {
			defer func() { f.passes <- struct{}{} }()
			if open, err := st.GetDeploy(ctx, row.ID, a.ID); err == nil {
				rec.recordDeploy(ctx, DesiredApp{App: row}, open, f.cr)
			}
		}()
	}
	return f
}

func (f aheadUnrecorded) get(t *testing.T, d Deploy) Deploy {
	t.Helper()
	got, err := f.st.GetDeploy(context.Background(), f.row.ID, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// recorded fails unless exactly one kicked pass recorded A live and B is the
// deploy in progress: queued, out of the pending slot, never canceled.
func (f aheadUnrecorded) recorded(t *testing.T, when string) {
	t.Helper()
	select {
	case <-f.passes:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s: no reconciler pass was kicked", when)
	}
	if n := f.kicked.Load(); n != 1 {
		t.Fatalf("%s: %d reconciler passes kicked, want one", when, n)
	}
	if got := f.get(t, f.a); got.Status != DeployLive {
		t.Fatalf("%s: A = %s, want the kicked pass to record it live", when, got.Status)
	}
	if got := f.get(t, f.b); got.Status != DeployQueued || got.OverlapPending || got.CancelReason != "" {
		t.Fatalf("%s: B = %+v, want it in progress (queued, out of the pending slot), never canceled", when, got)
	}
}

// w8/397: a deploy C that lands after the operator adopted B, but before the
// reconciler recorded A, used to close B canceled ("Superseded by" C) while the
// operator ran B to the end, then wait for its whole publish. C now waits for
// the kicked pass that records A, then waits behind B, which keeps running.
func TestPGDeployWaitsForThePassThatRecordsTheDeployAhead(t *testing.T) {
	f := newAheadUnrecorded(t)
	c, err := f.st.CreateDeploy(WithAdoptedRelease(context.Background(), f.cr), f.row.ID, TriggerAPI, "", f.b.Generation+1, CommitInfo{}, "")
	if err != nil || c.Status != DeployQueued || !c.OverlapPending {
		t.Fatalf("C = %+v (err %v), want it waiting in the pending slot behind B", c, err)
	}
	f.recorded(t, "after C")
}

// w8/397: a deploy that waits at the row level (a Restart, a Rollback, an
// environment "Save and deploy") waits for that pass the same way.
func TestPGDeferredDeployWaitsForThePassThatRecordsTheDeployAhead(t *testing.T) {
	f := newAheadUnrecorded(t)
	r, deferred, err := f.st.CreateDeferredDeploy(WithAdoptedRelease(context.Background(), f.cr), Deploy{AppID: f.row.ID, Trigger: TriggerRestart}, DeferredRequest{})
	if err != nil || !deferred || !r.OverlapPending {
		t.Fatalf("restart = %+v deferred=%v (err %v), want it waiting behind B", r, deferred, err)
	}
	f.recorded(t, "after the restart")
}

// w8/397: under Override the new deploy replaces the one in progress at once,
// so nothing waits for the pass: B is replaced and the new deploy starts.
func TestPGOverrideDeployDoesNotWaitForTheDeployAhead(t *testing.T) {
	f := newAheadUnrecorded(t)
	ctx := context.Background()
	if err := f.st.SetOverlappingDeployPolicy(ctx, f.row.TenantID, OverlapOverride); err != nil {
		t.Fatal(err)
	}
	c, err := f.st.CreateDeploy(WithAdoptedRelease(ctx, f.cr), f.row.ID, TriggerAPI, "", f.b.Generation+1, CommitInfo{}, "")
	if err != nil || c.Status != DeployCreated {
		t.Fatalf("C under Override = %+v (err %v), want it started", c, err)
	}
	if f.kicked.Load() != 0 {
		t.Fatal("a deploy under Override kicked the reconciler to wait for the deploy ahead")
	}
	if got := f.get(t, f.b); got.Status != DeployCanceled || SupersedingDeployID(got.CancelReason) != c.ID {
		t.Fatalf("B under Override = %s (%q), want replaced by %s", got.Status, got.CancelReason, c.ID)
	}
}
