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
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// openPGStore is the real control-plane store (BEX_TEST_DB_URI), migrated.
func openPGStore(t *testing.T) *store.PGStore {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return store.NewPGStore(pool)
}

// adoptedWaiting is a source-built service whose deploy A served while B
// waited behind it, and whose operator adopted B at once: the reconciler has
// not yet freed B's pending slot. With aheadRecorded false it has not recorded
// A live either (w8/397), so A's row still reads executing.
type adoptedWaiting struct {
	st      *store.PGStore
	svc     *Service
	cl      client.Client
	appID   string
	a, b    store.Deploy
	trigger func(label string, want int) store.Deploy
	get     func(d store.Deploy) store.Deploy
}

func newAdoptedWaiting(t *testing.T, aheadRecorded bool) adoptedWaiting {
	t.Helper()
	ctx := context.Background()
	st := openPGStore(t)
	stamp := fmt.Sprintf("w8-396-%d", time.Now().UnixNano())
	ten, err := st.CreateWorkspace(ctx, stamp, store.PlanHobby, "owner-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ten.ID) })
	repo := "https://example.com/acme/site.git"
	row, err := st.CreateApp(ctx, store.App{TenantID: ten.ID, Name: "site", Repo: repo, Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	history, err := st.ListDeploys(ctx, row.ID, store.DeployFilter{})
	if err != nil || len(history) != 1 {
		t.Fatalf("CreateApp history = %+v (err %v), want its first deploy", history, err)
	}
	transition := func(d store.Deploy, status string) {
		t.Helper()
		if _, err := st.TransitionDeploy(ctx, d.ID, status, store.RanImage{}, "", "", "", nil); err != nil {
			t.Fatal(err)
		}
	}
	transition(history[0], store.DeployLive)
	app := sampleApp("web", row.ID)
	app.Spec.Image, app.Spec.Repo, app.Spec.Branch = "", repo, "main"
	app.Generation = history[0].Generation
	app.Status.ReleaseGeneration = history[0].Generation
	cl := generationBumpingClient(app)
	f := adoptedWaiting{st: st, appID: row.ID, cl: cl, svc: &Service{Base: &core.Base{Client: cl, Namespace: "default", Clock: time.Now}, Store: st}}
	do := newRESTHarness(t, f.svc)
	// adopt is the operator adopting release generation g.
	adopt := func(g int64) {
		t.Helper()
		a := getApp(t, cl, "web")
		a.Status.ReleaseGeneration = g
		if err := cl.Update(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{history[0].ID: true}
	f.trigger = func(label string, want int) store.Deploy {
		t.Helper()
		if rec := do("POST", "/v1/services/web/deploys"); rec.Code != want {
			t.Fatalf("trigger %s: code=%d body=%s, want %d", label, rec.Code, rec.Body.String(), want)
		}
		return f.newest(t, seen, label)
	}
	f.get = func(d store.Deploy) store.Deploy {
		t.Helper()
		got, err := st.GetDeploy(ctx, row.ID, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	f.a = f.trigger("A", http.StatusCreated)
	adopt(f.a.Generation)
	transition(f.a, store.DeployBuildInProgress)
	f.b = f.trigger("B", http.StatusAccepted)
	if !f.b.OverlapPending {
		t.Fatalf("B = %+v, want it waiting in the pending slot behind A", f.b)
	}
	// A serves; the operator adopts B before the reconciler's next pass.
	if aheadRecorded {
		transition(f.a, store.DeployLive)
	} else {
		transition(f.a, store.DeployUpdateInProgress)
	}
	adopt(f.b.Generation)
	return f
}

// newest is the deploy row of f's app that seen does not hold yet.
func (f adoptedWaiting) newest(t *testing.T, seen map[string]bool, label string) store.Deploy {
	t.Helper()
	rows, err := f.st.ListDeploys(context.Background(), f.appID, store.DeployFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range rows {
		if !seen[d.ID] {
			seen[d.ID] = true
			return d
		}
	}
	t.Fatalf("%s opened no deploy", label)
	return store.Deploy{}
}

// inProgress fails unless B is the deploy in progress: queued, out of the
// pending slot, never canceled.
func (f adoptedWaiting) inProgress(t *testing.T, when string) {
	t.Helper()
	if got := f.get(f.b); got.Status != store.DeployQueued || got.OverlapPending || got.CancelReason != "" {
		t.Fatalf("B %s = %+v, want it in progress (queued, out of the pending slot), never canceled", when, got)
	}
}

// w8/396: a trigger C before the reconciler's next pass frees B's pending
// slot used to cancel B ("Superseded by" C, Render's newDeploy) and start as
// 201 with nothing to wait behind, then sat queued for B's whole publish.
// Under Render's Wait, B is the deploy in progress, so C answers 202 and
// waits behind it. A waiting deploy the operator has not adopted is still
// replaced (D replaces C).
func TestRESTTriggerWaitsBehindAWaitingDeployTheOperatorAdopted(t *testing.T) {
	f := newAdoptedWaiting(t, true)
	c := f.trigger("C", http.StatusAccepted)
	f.inProgress(t, "after C")
	if c.Status != store.DeployQueued || !c.OverlapPending {
		t.Fatalf("C = %+v, want it waiting in the pending slot behind B", c)
	}

	d := f.trigger("D", http.StatusAccepted)
	if got := f.get(c); got.Status != store.DeployCanceled || store.SupersedingDeployID(got.CancelReason) != d.ID {
		t.Fatalf("C after D = %s (%q), want replaced by %s", got.Status, got.CancelReason, d.ID)
	}
	f.inProgress(t, "after D")
}

// w8/396: a Restart in the same window waits behind B too, at the row level
// with no App change (w5/m284): written to the App, its selection would
// override B in the operator.
func TestRestartWaitsBehindAWaitingDeployTheOperatorAdopted(t *testing.T) {
	f := newAdoptedWaiting(t, true)
	before := getApp(t, f.cl, "web")
	r, err := f.svc.Restart(context.Background(), "web")
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if row := f.get(store.Deploy{ID: r.ID}); row.Status != store.DeployQueued || row.Trigger != store.TriggerRestart || !row.Deferred {
		t.Fatalf("restart = %+v, want a queued deferred restart waiting behind B", row)
	}
	if got := getApp(t, f.cl, "web"); got.ResourceVersion != before.ResourceVersion {
		t.Fatal("a restart waiting behind B changed the App")
	}
	f.inProgress(t, "after the restart")
}

// w8/397: a trigger C after the operator adopted B, but before the reconciler
// recorded A live, used to close B canceled ("Superseded by" C) while the
// operator ran B to the end, and C then waited for B's whole publish. C now
// kicks the reconciler, waits for the pass that records A, and answers 202
// behind B, which keeps running. The pass here records A as the reconciler
// would (TestPGDeployWaitsForThePassThatRecordsTheDeployAhead runs the
// reconciler's own recording).
func TestRESTTriggerWaitsForThePassThatRecordsTheDeployAhead(t *testing.T) {
	f := newAdoptedWaiting(t, false)
	passes := make(chan error, 4)
	f.st.KickReconciler = func() {
		go func() {
			_, err := f.st.TransitionDeploy(context.Background(), f.a.ID, store.DeployLive, store.RanImage{}, "", "", "", nil)
			passes <- err
		}()
	}
	c := f.trigger("C", http.StatusAccepted)
	select {
	case err := <-passes:
		if err != nil {
			t.Fatalf("the kicked pass: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("C kicked no reconciler pass")
	}
	if got := f.get(f.a); got.Status != store.DeployLive {
		t.Fatalf("A = %s, want live", got.Status)
	}
	f.inProgress(t, "after C")
	if c.Status != store.DeployQueued || !c.OverlapPending {
		t.Fatalf("C = %+v, want it waiting in the pending slot behind B", c)
	}
}
