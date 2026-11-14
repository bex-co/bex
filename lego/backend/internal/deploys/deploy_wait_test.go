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
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// imageServiceRollingOut is an image-backed service whose first deploy went
// live and whose second, dep-flight, is rolling out.
func imageServiceRollingOut(t *testing.T) (deferringStore, *Service, string) {
	t.Helper()
	ds := newDeferringStore()
	ctx := context.Background()
	first, _ := ds.CreateDeploy(ctx, "srv-1", store.TriggerCreate, "web:v1", 1, store.CommitInfo{}, "")
	if won, err := ds.CloseDeploy(ctx, first.ID, store.DeployLive, "web:v1"); err != nil || !won {
		t.Fatalf("close first live: won=%v err=%v", won, err)
	}
	flight, _ := ds.CreateDeploy(ctx, "srv-1", store.TriggerAPI, "web:v2", 2, store.CommitInfo{}, "")
	ds.set(flight.ID, func(d *store.Deploy) { d.ID, d.Status = "dep-flight", store.DeployUpdateInProgress })
	app := sampleApp("web", "srv-1")
	app.Spec.Image = "web:v2"
	app.Generation = 2
	svc := &Service{Base: &core.Base{Client: generationBumpingClient(app), Namespace: "default", Clock: fixedNow}, Store: ds}
	return ds, svc, first.ID
}

func settle(t *testing.T, ds deferringStore, svc *Service, id string) {
	t.Helper()
	ds.set(id, func(d *store.Deploy) { d.Status = store.DeployLive })
	if err := (&DeferredDeployWorker{Svc: svc}).tick(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func rowOf(t *testing.T, ds deferringStore, id string) store.Deploy {
	t.Helper()
	d, err := ds.GetDeploy(context.Background(), "srv-1", id)
	if err != nil {
		t.Fatalf("deploy %s: %v", id, err)
	}
	return d
}

// w4/652: under Render's Wait a Rollback behind a deploy rolling out leaves it
// to finish, and runs, as the row the caller was answered with, once it has.
func TestRollbackWaitsForTheDeployInFlight(t *testing.T) {
	ds, svc, target := imageServiceRollingOut(t)
	before := getApp(t, svc.Client, "web")

	d, err := svc.Rollback(context.Background(), "web", target)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if d.Status != store.DeployQueued || d.Trigger != store.TriggerRollback || d.RollbackOf != target || d.Image != "web:v1" {
		t.Fatalf("rollback = %+v, want a queued rollback of %s showing web:v1", d, target)
	}
	if got := getApp(t, svc.Client, "web"); got.ResourceVersion != before.ResourceVersion {
		t.Fatal("a waiting rollback changed the App")
	}
	if err := (&DeferredDeployWorker{Svc: svc}).tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, svc.Client, "web"); got.ResourceVersion != before.ResourceVersion {
		t.Fatal("the rollback ran while the deploy was still rolling out")
	}

	settle(t, ds, svc, "dep-flight")
	got := getApp(t, svc.Client, "web")
	if got.Spec.ReleaseConfig == nil || got.Spec.ReleaseConfig.Image != "web:v1" || got.Spec.Image != "web:v2" {
		t.Fatalf("after the deploy settled: releaseConfig %+v image %q, want web:v1 selected over the saved web:v2", got.Spec.ReleaseConfig, got.Spec.Image)
	}
	if row := rowOf(t, ds, d.ID); row.Status != store.DeployCreated || row.Deferred || row.Generation == 0 || row.RollbackOf != target {
		t.Fatalf("rollback row = %+v, want %s itself promoted to a release", row, d.ID)
	}
	if flight := rowOf(t, ds, "dep-flight"); flight.Status != store.DeployLive {
		t.Fatalf("deploy that was in flight = %s, want live", flight.Status)
	}
}

// w4/652: an image-override deploy, from the API or the deploy hook's imgURL
// (which answers Render's 202), waits like any deploy; a second one replaces
// the first in the pending slot.
func TestImageOverrideDeployWaitsForTheDeployInFlight(t *testing.T) {
	ds, svc, _ := imageServiceRollingOut(t)
	before := getApp(t, svc.Client, "web")

	first, err := svc.Trigger(context.Background(), "web", TriggerParams{ImageURL: "web:v3"})
	if err != nil || first.Status != store.DeployQueued || first.Image != "web:v3" {
		t.Fatalf("imageUrl deploy = %+v (err %v), want queued showing web:v3", first, err)
	}
	svc.DeployHookLimiter = NewDeployHookRateLimiter(6000, 10)
	hook, err := svc.GetDeployHook(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	before = getApp(t, svc.Client, "web") // the hook token lives on the App
	w := httptest.NewRecorder()
	svc.DeployHookHandler().ServeHTTP(w, httptest.NewRequest(http.MethodPost,
		"/v1/deploy-hooks?key="+url.QueryEscape(deployHookTokenFromURL(t, hook.URL))+"&imgURL="+url.QueryEscape("web:v4"), nil))
	if w.Code != http.StatusAccepted {
		t.Fatalf("hook imgURL behind a deploy in progress: code=%d body=%s, want 202", w.Code, w.Body.String())
	}
	if got := getApp(t, svc.Client, "web"); got.ResourceVersion != before.ResourceVersion {
		t.Fatal("a waiting image-override deploy changed the App")
	}
	if row := rowOf(t, ds, first.ID); row.Status != store.DeployCanceled {
		t.Fatalf("first override = %s, want replaced in the pending slot by the hook's", row.Status)
	}

	settle(t, ds, svc, "dep-flight")
	got := getApp(t, svc.Client, "web")
	if got.Spec.ReleaseConfig == nil || got.Spec.ReleaseConfig.Image != "web:v4" {
		t.Fatalf("after the deploy settled: releaseConfig %+v, want the hook's web:v4", got.Spec.ReleaseConfig)
	}
	open := 0
	for _, d := range ds.byApp["srv-1"] {
		if store.IsOpenDeployStatus(d.Status) {
			open++
			if d.Trigger != store.TriggerDeployHook || d.Image != "web:v4" || d.Deferred {
				t.Fatalf("open deploy = %+v, want the hook's promoted web:v4 deploy", d)
			}
		}
	}
	if open != 1 {
		t.Fatalf("open deploys = %d, want 1", open)
	}
}

// w4/652: a selected release in flight is not one the operator pins, so a
// plain deploy behind it waits too; behind a plain deploy, a plain deploy
// still writes through and the operator's pin holds the one in flight.
func TestPlainDeployWaitsOnlyBehindASelectedRelease(t *testing.T) {
	ds, svc, target := imageServiceRollingOut(t)
	plain, err := svc.Trigger(context.Background(), "web", TriggerParams{})
	if err != nil || plain.Status != store.DeployQueued || rowOf(t, ds, plain.ID).Deferred {
		t.Fatalf("plain deploy behind a plain one = %+v (err %v), want a queued release written through", plain, err)
	}

	settle(t, ds, svc, "dep-flight")
	ds.set(plain.ID, func(d *store.Deploy) { d.Status = store.DeployLive })
	if _, err := svc.Rollback(context.Background(), "web", target); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	rolling := getApp(t, svc.Client, "web")
	if rolling.ActiveReleaseConfig() == nil {
		t.Fatal("nothing in flight: the rollback should have run at once")
	}
	behind, err := svc.Trigger(context.Background(), "web", TriggerParams{})
	if err != nil || behind.Status != store.DeployQueued || !rowOf(t, ds, behind.ID).Deferred {
		t.Fatalf("plain deploy behind the rollback = %+v (err %v), want a deferred deploy", behind, err)
	}
	if got := getApp(t, svc.Client, "web"); got.ResourceVersion != rolling.ResourceVersion {
		t.Fatal("a deploy behind a selected release in flight changed the App")
	}
	if restart, err := svc.Restart(context.Background(), "web"); err != nil || restart.Status != store.DeployQueued {
		t.Fatalf("Restart behind the rollback = %+v (err %v), want it to wait", restart, err)
	}
	if row := rowOf(t, ds, behind.ID); row.Status != store.DeployCanceled {
		t.Fatalf("plain deploy = %s, want replaced in the pending slot by the restart", row.Status)
	}
}
