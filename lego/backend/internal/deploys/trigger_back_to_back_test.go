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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// imageServiceLive is an image-backed service whose first deploy is live and
// adopted, with nothing in flight. Its clock ticks, so every release write
// changes the App.
func imageServiceLive(t *testing.T) (deferringStore, *Service, string) {
	t.Helper()
	ds := newDeferringStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", store.TriggerCreate, "web:v1", 1, store.CommitInfo{}, "")
	if won, err := ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "web:v1"); err != nil || !won {
		t.Fatalf("close first live: won=%v err=%v", won, err)
	}
	app := sampleApp("web", "srv-1")
	app.Status.ReleaseGeneration = 1
	var tick atomic.Int64
	clock := func() time.Time { return fixedNow().Add(time.Duration(tick.Add(1)) * time.Second) }
	svc := &Service{Base: &core.Base{Client: generationBumpingClient(app), Namespace: "default", Clock: clock}, Store: ds}
	return ds, svc, first.ID
}

// adopt records the operator adopting release generation gen, as its
// status.releaseGeneration does.
func adopt(t *testing.T, svc *Service, gen int64) {
	t.Helper()
	a := getApp(t, svc.Client, "web")
	a.Status.ReleaseGeneration = gen
	if err := svc.Client.Update(context.Background(), a); err != nil {
		t.Fatal(err)
	}
}

// triggerREST fires Render's POST /v1/services/{id}/deploys and returns the
// answer's code and, on a 201, the deploy it started.
func triggerREST(t *testing.T, do func(method, path string) *httptest.ResponseRecorder) (int, string) {
	t.Helper()
	rec := do("POST", "/v1/services/web/deploys")
	if rec.Code != http.StatusCreated {
		return rec.Code, ""
	}
	var started renderDeploy
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil || started.ID == "" {
		t.Fatalf("201 body %s: %v", rec.Body, err)
	}
	return rec.Code, started.ID
}

// queuedID is the deploy waiting in the latest-pending slot, as the pinned
// CLI finds it after a 202 (?status=queued&limit=1).
func queuedID(t *testing.T, do func(method, path string) *httptest.ResponseRecorder) string {
	t.Helper()
	queued := decodeList(t, do("GET", "/v1/services/web/deploys?status=queued&limit=1"))
	if len(queued) != 1 {
		t.Fatalf("queued deploys = %+v, want one", queued)
	}
	return queued[0].Deploy.ID
}

// w8/387: under Render's Wait two back-to-back triggers both run, in order.
// The first, answered 201, is the deploy in progress from the moment it is
// created; the second, answered 202, waits until it has finished. Before the
// fix the second was written to the App before the operator adopted the
// first, which closed "Superseded by …" and never ran.
func TestBackToBackTriggersBothRunUnderWait(t *testing.T) {
	ds, svc, _ := imageServiceLive(t)
	do := newRESTHarness(t, svc)

	code, first := triggerREST(t, do)
	if code != http.StatusCreated {
		t.Fatalf("first trigger = %d, want 201", code)
	}
	written := getApp(t, svc.Client, "web")
	if code, _ := triggerREST(t, do); code != http.StatusAccepted {
		t.Fatalf("second trigger before the first was adopted = %d, want 202", code)
	}
	second := queuedID(t, do)
	if got := getApp(t, svc.Client, "web"); got.ResourceVersion != written.ResourceVersion {
		t.Fatal("the waiting deploy changed the App: the first deploy's release is replaced")
	}
	if err := (&DeferredDeployWorker{Svc: svc}).tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if row := rowOf(t, ds, second); row.Status != store.DeployQueued || !row.Deferred {
		t.Fatalf("second deploy while the first is in flight = %+v, want it still waiting", row)
	}

	adopt(t, svc, rowOf(t, ds, first).Generation)
	settle(t, ds, svc, first)
	ran := rowOf(t, ds, second)
	if ran.Status != store.DeployCreated || ran.Deferred || ran.Generation <= rowOf(t, ds, first).Generation {
		t.Fatalf("second deploy once the first went live = %+v, want it running as a newer release", ran)
	}
	settle(t, ds, svc, second)
	if a, b := rowOf(t, ds, first), rowOf(t, ds, second); a.Status != store.DeployLive || b.Status != store.DeployLive {
		t.Fatalf("deploys = %s then %s, want live then live", a.Status, b.Status)
	}
}

// w8/387: a third trigger while the second waits replaces only the second;
// the first still runs to its own end and the third runs after it. The deploy
// hook, a Restart, a Rollback and a "Save, rebuild, and deploy" behind a
// deploy the operator has not adopted wait under the same rule.
func TestThirdTriggerReplacesOnlyTheWaitingDeploy(t *testing.T) {
	ds, svc, target := imageServiceLive(t)
	do := newRESTHarness(t, svc)
	ctx := context.Background()

	_, first := triggerREST(t, do)
	written := getApp(t, svc.Client, "web")
	for _, tc := range []struct {
		name string
		run  func() (DeployView, error)
	}{
		{"restart", func() (DeployView, error) { return svc.Restart(ctx, "web") }},
		{"rollback", func() (DeployView, error) { return svc.Rollback(ctx, "web", target) }},
		{"rebuild", func() (DeployView, error) { return svc.Rebuild(ctx, "web") }},
	} {
		d, err := tc.run()
		if err != nil || d.Status != store.DeployQueued || !rowOf(t, ds, d.ID).Deferred {
			t.Fatalf("%s behind a deploy not adopted yet = %+v (err %v), want it waiting", tc.name, d, err)
		}
	}
	rebuild := queuedID(t, do)
	if code, _ := triggerREST(t, do); code != http.StatusAccepted {
		t.Fatalf("second trigger = %d, want 202", code)
	}
	second := queuedID(t, do)
	if code, _ := triggerREST(t, do); code != http.StatusAccepted {
		t.Fatalf("third trigger = %d, want 202", code)
	}
	third := queuedID(t, do)
	if got := getApp(t, svc.Client, "web"); got.ResourceVersion != written.ResourceVersion {
		t.Fatal("a waiting deploy changed the App")
	}
	for id, want := range map[string]string{first: store.DeployCreated, rebuild: store.DeployCanceled, second: store.DeployCanceled, third: store.DeployQueued} {
		if got := rowOf(t, ds, id).Status; got != want {
			t.Fatalf("deploy %s = %s, want %s", id, got, want)
		}
	}

	adopt(t, svc, rowOf(t, ds, first).Generation)
	settle(t, ds, svc, first)
	if a, c := rowOf(t, ds, first), rowOf(t, ds, third); a.Status != store.DeployLive || c.Status != store.DeployCreated || c.Deferred {
		t.Fatalf("first = %s, third = %+v, want the first live and the third running", a.Status, c)
	}
}

// w8/387: "Save, rebuild, and deploy" that waited runs as a rebuild of the
// latest commit, not as an environment "Save and deploy" on the build serving.
func TestWaitingRebuildRunsAsARebuild(t *testing.T) {
	ds, svc, _ := imageServiceLive(t)
	do := newRESTHarness(t, svc)
	_, first := triggerREST(t, do)
	rebuild, err := svc.Rebuild(context.Background(), "web")
	if err != nil || rebuild.Status != store.DeployQueued {
		t.Fatalf("Rebuild behind a deploy not adopted yet = %+v (err %v), want it waiting", rebuild, err)
	}
	adopt(t, svc, rowOf(t, ds, first).Generation)
	settle(t, ds, svc, first)
	if row := rowOf(t, ds, rebuild.ID); row.Status != store.DeployCreated || row.Trigger != store.TriggerConfigChange || row.Generation == 0 {
		t.Fatalf("waiting rebuild once the deploy ahead went live = %+v (cancel reason %q), want it running", row, row.CancelReason)
	}
}
