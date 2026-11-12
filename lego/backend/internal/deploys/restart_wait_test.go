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

package deploys

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// deferringStore adds the Wait-policy capabilities PGStore has to fakeStore.
type deferringStore struct{ *fakeStore }

func (f deferringStore) CreateDeferredRestart(_ context.Context, appID, triggeredBy string) (store.Deploy, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	busy := false
	for _, d := range f.byApp[appID] {
		busy = busy || (store.IsOpenDeployStatus(d.Status) && !d.OverlapPending)
	}
	if !busy {
		return store.Deploy{}, false, nil
	}
	f.nextID++
	d := store.Deploy{ID: fmt.Sprintf("dep-%d", f.nextID), AppID: appID, Trigger: store.TriggerRestart, TriggeredBy: triggeredBy,
		Status: store.DeployQueued, OverlapPending: true, RestartDeferred: true, CreatedAt: time.Now()}
	f.byApp[appID] = append([]store.Deploy{d}, f.byApp[appID]...)
	return d, true, nil
}

func (f deferringStore) ListDeferredRestarts(_ context.Context, _ int) ([]store.DeferredRestart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.DeferredRestart
	for appID, rows := range f.byApp {
		for _, d := range rows {
			if !d.RestartDeferred || d.Status != store.DeployQueued {
				continue
			}
			ready := true
			for _, o := range f.byApp[appID] {
				if o.ID != d.ID && store.IsOpenDeployStatus(o.Status) && !o.OverlapPending {
					ready = false
				}
			}
			out = append(out, store.DeferredRestart{Deploy: d, Ready: ready})
		}
	}
	return out, nil
}

func (f deferringStore) CancelDeferredRestart(_ context.Context, id, reason string) (bool, error) {
	won := false
	f.set(id, func(d *store.Deploy) {
		if d.RestartDeferred && d.Status == store.DeployQueued {
			d.Status, d.CancelReason, d.OverlapPending, won = store.DeployCanceled, reason, false, true
		}
	})
	return won, nil
}

func (f deferringStore) set(id string, mutate func(*store.Deploy)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for appID, rows := range f.byApp {
		for i := range rows {
			if rows[i].ID == id {
				mutate(&f.byApp[appID][i])
			}
		}
	}
}

// CreateDeploy promotes the deferred row its context names, as PGStore does.
func (f deferringStore) CreateDeploy(ctx context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo, triggeredBy string, clearCache ...bool) (store.Deploy, error) {
	id := store.DeferredRestartID(ctx)
	if id == "" || trigger != store.TriggerRestart {
		return f.fakeStore.CreateDeploy(ctx, appID, trigger, image, generation, commit, triggeredBy, clearCache...)
	}
	var promoted store.Deploy
	f.set(id, func(d *store.Deploy) {
		d.Status, d.OverlapPending, d.RestartDeferred, d.Generation, d.Image, d.Commit = store.DeployCreated, false, false, generation, image, commit.Hash
		promoted = *d
	})
	if promoted.ID == "" {
		return store.Deploy{}, store.ErrDeferredRestartGone
	}
	return promoted, nil
}

// w5/m284: under Render's Wait policy a Restart during a deploy waits for it,
// leaves it running, and then restarts what serves once it settled — the new
// release, not the one serving when the restart was requested.
func TestRestartWaitsForTheDeployInFlight(t *testing.T) {
	ds := deferringStore{newFakeStore()}
	const newCommit = "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111"
	seedDeploys(ds.fakeStore,
		store.Deploy{ID: "dep-flight", Status: store.DeployBuildInProgress, Commit: newCommit, Generation: 3},
		store.Deploy{ID: "dep-live", Status: store.DeployLive, Commit: runningCommit, Generation: 2},
	)
	svc, cl := newService(ds, repoApp("web", "srv-1", "main"))
	svc.Commits = &branchResolver{}
	before := getApp(t, cl, "web")

	d, err := svc.Restart(context.Background(), "web")
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if d.Status != store.DeployQueued || d.Trigger != store.TriggerRestart {
		t.Fatalf("restart = %+v, want a queued restart", d)
	}
	after := getApp(t, cl, "web")
	if after.ResourceVersion != before.ResourceVersion {
		t.Fatalf("a waiting restart changed the App (resourceVersion %s→%s)", before.ResourceVersion, after.ResourceVersion)
	}
	if flight := ds.byApp["srv-1"][1]; flight.ID != "dep-flight" || flight.Status != store.DeployBuildInProgress {
		t.Fatalf("deploy in flight = %+v, want left running (Wait, not Override)", flight)
	}

	w := &DeferredRestartWorker{Svc: svc}
	if err := w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "web"); got.ResourceVersion != before.ResourceVersion {
		t.Fatal("the restart ran while the deploy was still in flight")
	}

	ds.set("dep-flight", func(d *store.Deploy) { d.Status = store.DeployLive })
	if err := w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	ran := getApp(t, cl, "web")
	if ran.ResourceVersion == before.ResourceVersion || ran.Spec.BuildCommit != newCommit {
		t.Fatalf("after the deploy settled: buildCommit %q; want the restart to rebuild the now-serving %s", ran.Spec.BuildCommit, newCommit)
	}
	if row := ds.byApp["srv-1"][0]; row.ID != d.ID || row.Status != store.DeployCreated || row.RestartDeferred {
		t.Fatalf("restart row = %+v, want %s itself promoted", row, d.ID)
	}
}

// A waiting restart whose service was suspended meanwhile is canceled with
// the reason, not left queued.
func TestDeferredRestartThatCannotRunIsCanceled(t *testing.T) {
	ds := deferringStore{newFakeStore()}
	seedDeploys(ds.fakeStore,
		store.Deploy{ID: "dep-flight", Status: store.DeployBuildInProgress, Generation: 3},
		store.Deploy{ID: "dep-live", Status: store.DeployLive, Commit: runningCommit, Generation: 2},
	)
	svc, cl := newService(ds, repoApp("web", "srv-1", "main"))
	d, err := svc.Restart(context.Background(), "web")
	if err != nil || d.Status != store.DeployQueued {
		t.Fatalf("Restart = %+v, %v", d, err)
	}
	a := getApp(t, cl, "web")
	a.Spec.Suspended = true
	if err := cl.Update(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	ds.set("dep-flight", func(d *store.Deploy) { d.Status = store.DeployLive })
	if err := (&DeferredRestartWorker{Svc: svc}).tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if row := ds.byApp["srv-1"][0]; row.Status != store.DeployCanceled || row.CancelReason == "" {
		t.Fatalf("restart row = %+v, want canceled with a reason", row)
	}
}

// One that waits past MaxWait is canceled.
func TestDeferredRestartGivesUpAfterMaxWait(t *testing.T) {
	ds := deferringStore{newFakeStore()}
	seedDeploys(ds.fakeStore,
		store.Deploy{ID: "dep-flight", Status: store.DeployBuildInProgress, Generation: 3},
		store.Deploy{ID: "dep-live", Status: store.DeployLive, Commit: runningCommit, Generation: 2},
	)
	svc, _ := newService(ds, repoApp("web", "srv-1", "main"))
	if _, err := svc.Restart(context.Background(), "web"); err != nil {
		t.Fatal(err)
	}
	w := &DeferredRestartWorker{Svc: svc, MaxWait: time.Hour, Now: func() time.Time { return time.Now().Add(2 * time.Hour) }}
	if err := w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	row := ds.byApp["srv-1"][0]
	if row.Status != store.DeployCanceled {
		t.Fatalf("restart row = %+v, want canceled after MaxWait", row)
	}
	assertCancelReason(t, row.CancelReason, "the deploy this restart waited for did not finish within 1h0m0s",
		appv1alpha1.DeployReasonRestartWaitTimedOut, appv1alpha1.DeployReasonParam{Name: "wait", Value: "1h0m0s"})
}

// One whose service was deleted meanwhile is canceled saying so.
func TestDeferredRestartOfADeletedServiceIsCanceled(t *testing.T) {
	ds := deferringStore{newFakeStore()}
	seedDeploys(ds.fakeStore,
		store.Deploy{ID: "dep-flight", Status: store.DeployBuildInProgress, Generation: 3},
		store.Deploy{ID: "dep-live", Status: store.DeployLive, Commit: runningCommit, Generation: 2},
	)
	svc, cl := newService(ds, repoApp("web", "srv-1", "main"))
	if _, err := svc.Restart(context.Background(), "web"); err != nil {
		t.Fatal(err)
	}
	if err := cl.Delete(context.Background(), getApp(t, cl, "web")); err != nil {
		t.Fatal(err)
	}
	ds.set("dep-flight", func(d *store.Deploy) { d.Status = store.DeployLive })
	if err := (&DeferredRestartWorker{Svc: svc}).tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	row := ds.byApp["srv-1"][0]
	if row.Status != store.DeployCanceled {
		t.Fatalf("restart row = %+v, want canceled", row)
	}
	assertCancelReason(t, row.CancelReason, "the service no longer exists", appv1alpha1.DeployReasonRestartServiceGone)
}

// w4/591: a deferred restart's cancelReason parses back to its template and
// values, so the dashboard can word it in its own language.
func assertCancelReason(t *testing.T, got, want, code string, params ...appv1alpha1.DeployReasonParam) {
	t.Helper()
	if got != want {
		t.Fatalf("cancel_reason = %q, want %q", got, want)
	}
	gotCode, gotParams, ok := appv1alpha1.ParseDeployReason(got)
	if !ok || gotCode != code || !slices.Equal(gotParams, params) {
		t.Fatalf("%q parses as %s %v (ok=%v), want %s %v", got, gotCode, gotParams, ok, code, params)
	}
}
