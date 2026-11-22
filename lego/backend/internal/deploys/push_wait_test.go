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
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	pushedA = "aaaa2222aaaa2222aaaa2222aaaa2222aaaa2222"
	pushedB = "bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222"
)

// repoServiceLive is imageServiceLive for a Git-backed service: its first
// deploy is live and adopted, and nothing is in flight.
func repoServiceLive(t *testing.T) (deferringStore, *Service) {
	t.Helper()
	ds := newDeferringStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", store.TriggerCreate, "", 1, store.CommitInfo{Hash: runningCommit}, "")
	if won, err := ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "zot/web@sha256:1"); err != nil || !won {
		t.Fatalf("close first live: won=%v err=%v", won, err)
	}
	app := repoApp("web", "srv-1", "main")
	app.Generation = 1
	app.Status.ReleaseGeneration = 1
	var tick atomic.Int64
	clock := func() time.Time { return fixedNow().Add(time.Duration(tick.Add(1)) * time.Second) }
	svc := &Service{Base: &core.Base{Client: generationBumpingClient(app), Namespace: "default", Clock: clock}, Store: ds}
	svc.Commits = &branchResolver{}
	return ds, svc
}

// w8/390: a git push right after a trigger answered 201, before the operator
// adopted it, leaves that deploy running to live and then runs with its own
// commit. A second push while the first push's deploy waits replaces only
// that deploy. Before the fix the push wrote its release to the App at once,
// and the trigger's deploy closed "Superseded by …" without running.
func TestPushBehindAnUnadoptedDeployWaitsAndRunsItsCommit(t *testing.T) {
	ds, svc := repoServiceLive(t)
	do := newRESTHarness(t, svc)
	ctx := context.Background()

	code, first := triggerREST(t, do)
	if code != http.StatusCreated {
		t.Fatalf("trigger = %d, want 201", code)
	}
	written := getApp(t, svc.Client, "web")
	if deferred, err := svc.DeferNewCommit(ctx, getApp(t, svc.Client, "web"), store.CommitInfo{Hash: pushedA}); err != nil || !deferred {
		t.Fatalf("push behind a deploy not adopted yet: deferred=%v err=%v, want it waiting", deferred, err)
	}
	pushA := queuedID(t, do)
	if deferred, err := svc.DeferNewCommit(ctx, getApp(t, svc.Client, "web"), store.CommitInfo{Hash: pushedB}); err != nil || !deferred {
		t.Fatalf("second push: deferred=%v err=%v, want it waiting", deferred, err)
	}
	pushB := queuedID(t, do)
	if got := getApp(t, svc.Client, "web"); got.ResourceVersion != written.ResourceVersion {
		t.Fatal("a waiting push changed the App: the trigger's release is replaced")
	}
	for id, want := range map[string]string{first: store.DeployCreated, pushA: store.DeployCanceled, pushB: store.DeployQueued} {
		if got := rowOf(t, ds, id); got.Status != want {
			t.Fatalf("deploy %s = %s (%q), want %s", id, got.Status, got.CancelReason, want)
		}
	}
	if row := rowOf(t, ds, pushB); row.Trigger != store.TriggerNewCommit {
		t.Fatalf("waiting push trigger = %q, want %q", row.Trigger, store.TriggerNewCommit)
	}

	adopt(t, svc, rowOf(t, ds, first).Generation)
	settle(t, ds, svc, first)
	ran := rowOf(t, ds, pushB)
	if ran.Status != store.DeployCreated || ran.Deferred || ran.Generation <= rowOf(t, ds, first).Generation || ran.Commit != pushedB {
		t.Fatalf("push once the trigger went live = %+v, want it running %s as a newer release", ran, pushedB)
	}
	if got := getApp(t, svc.Client, "web").Spec.BuildCommit; got != pushedB {
		t.Fatalf("buildCommit = %q, want the pushed %s", got, pushedB)
	}
	settle(t, ds, svc, pushB)
	if a, b := rowOf(t, ds, first), rowOf(t, ds, pushB); a.Status != store.DeployLive || b.Status != store.DeployLive {
		t.Fatalf("deploys = %s then %s, want live then live", a.Status, b.Status)
	}
}

// w8/390: behind a deploy the operator has adopted, a push writes through, as
// before: the operator holds it behind its pin.
func TestPushBehindAnAdoptedDeployWritesThrough(t *testing.T) {
	ds, svc := repoServiceLive(t)
	do := newRESTHarness(t, svc)
	_, first := triggerREST(t, do)
	adopt(t, svc, rowOf(t, ds, first).Generation)
	if deferred, err := svc.DeferNewCommit(context.Background(), getApp(t, svc.Client, "web"), store.CommitInfo{Hash: pushedA}); err != nil || deferred {
		t.Fatalf("push behind an adopted deploy: deferred=%v err=%v, want it written through", deferred, err)
	}
}

// w8/390: a build-source fan-out behind a deploy the operator has not
// adopted waits, then ships the build it named once that deploy has ended. A
// fan-out that completes a service's first deploy, which waits for this very
// build, ships at once.
func TestFanoutBehindAnUnadoptedDeployShipsItsBuildOnceThatEnds(t *testing.T) {
	const tenant, built = "tea-a", "zot/src@sha256:new"
	cr := func(name string) string { return store.CRName(tenant, name) }
	ctx := context.Background()
	ds := newDeferringStore()
	ds.mintIDs = true
	ds.links = []store.App{
		{ID: "srv-1", TenantID: tenant, Name: "web", BuildSourceID: "bds-1", BuildSourceImage: "zot/src@sha256:old"},
		{ID: "srv-6", TenantID: tenant, Name: "first", BuildSourceID: "bds-1"},
	}
	web := linkedService(cr("web"), "srv-1", "bds-1", "zot/src@sha256:old", true)
	web.Generation, web.Status.ReleaseGeneration = 1, 1
	firstDeploy := linkedService(cr("first"), "srv-6", "bds-1", "", false)
	firstDeploy.Generation = 1
	svc := &Service{Base: &core.Base{Client: generationBumpingClient(builtCarrier("bds-1", "zot/src@sha256:old"), web, firstDeploy), Namespace: "default", Clock: time.Now}, Store: ds}
	live, _ := ds.CreateDeploy(ctx, "srv-1", store.TriggerCreate, "", 1, store.CommitInfo{}, "")
	ds.set(live.ID, func(d *store.Deploy) { d.Status = store.DeployLive })
	ahead, _ := ds.CreateDeploy(ctx, "srv-1", store.TriggerAPI, "", 2, store.CommitInfo{}, "")
	// The first service's creation deploy is open, and not adopted yet.
	if _, err := ds.CreateDeploy(ctx, "srv-6", store.TriggerCreate, "", 1, store.CommitInfo{}, ""); err != nil {
		t.Fatal(err)
	}
	carrierDeploy, _ := ds.CreateDeploy(ctx, "bds-1", store.TriggerAPI, "", 1, store.CommitInfo{Hash: "abc1234"}, "")
	ds.set(carrierDeploy.ID, func(d *store.Deploy) { d.Status, d.ResolvedImage = store.DeployLive, built })
	before := getApp(t, svc.Client, cr("web"))

	if err := svc.BuildSucceeded(ctx, store.BuiltSource{TenantID: tenant, CarrierID: "bds-1", DeployID: carrierDeploy.ID, Commit: carrierDeploy.CommitInfo(), Namespace: "default", Carrier: "bds-1", Image: built}); err != nil {
		t.Fatalf("BuildSucceeded: %v", err)
	}
	if got := getApp(t, svc.Client, cr("web")); got.ResourceVersion != before.ResourceVersion || ds.builds["srv-1"] != "" {
		t.Fatalf("the fan-out behind a deploy not adopted yet wrote build %q to the service", ds.builds["srv-1"])
	}
	waiting := ds.byApp["srv-1"][0]
	if waiting.Status != store.DeployQueued || !waiting.Deferred || ds.requests[waiting.ID].BuildID != BuildID(carrierDeploy.ID) {
		t.Fatalf("fan-out deploy = %+v (request %+v), want it waiting on build %s", waiting, ds.requests[waiting.ID], BuildID(carrierDeploy.ID))
	}
	if ds.builds["srv-6"] != built || ds.byApp["srv-6"][0].Deferred {
		t.Fatalf("first deploy's build = %q (deploys %+v), want the fan-out to ship %s at once", ds.builds["srv-6"], ds.byApp["srv-6"], built)
	}

	ds.set(ahead.ID, func(d *store.Deploy) { d.Status = store.DeployLive })
	if err := (&DeferredDeployWorker{Svc: svc}).tick(ctx); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, svc.Client, cr("web")).Spec.BuildSource; got == nil || got.Image != built || ds.builds["srv-1"] != built {
		t.Fatalf("after the deploy ahead ended: buildSource %+v, row build %q; want %s", got, ds.builds["srv-1"], built)
	}
	if row := rowOf(t, ds, waiting.ID); row.Status != store.DeployCreated || row.Deferred {
		t.Fatalf("fan-out deploy once the deploy ahead ended = %+v, want it running", row)
	}
}
