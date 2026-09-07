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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- Cancel ------------------------------------------------------------------

func TestCancelClosesOpenDeployAndIsIdempotentConflict(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	got, err := svc.Cancel(context.Background(), "web", first.ID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got.Status != store.DeployCanceled || got.FinishedAt == nil {
		t.Fatalf("canceled deploy = %+v, want status canceled with finished_at set", got)
	}
	stored, err := ds.GetDeploy(context.Background(), "srv-1", first.ID)
	if err != nil || stored.Status != store.DeployCanceled {
		t.Fatalf("stored deploy = %+v (err %v), want canceled", stored, err)
	}

	// Already terminal: a second Cancel is Render's 409, never a silent no-op.
	if _, err := svc.Cancel(context.Background(), "web", first.ID); !errors.Is(err, core.ErrConflict) {
		t.Errorf("re-cancel: want core.ErrConflict, got %v", err)
	}
}

func TestCancelRefusesAlreadyLiveDeploy(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	if won, err := ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "web:v1"); err != nil || !won {
		t.Fatalf("close: won=%v err=%v", won, err)
	}
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	if _, err := svc.Cancel(context.Background(), "web", first.ID); !errors.Is(err, core.ErrConflict) {
		t.Errorf("cancel of a live deploy: want core.ErrConflict, got %v", err)
	}
}

func TestCancelUnknownDeployIsNotFound(t *testing.T) {
	ds := newFakeStore()
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	if _, err := svc.Cancel(context.Background(), "web", "dep-nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("cancel unknown deploy: want core.ErrNotFound, got %v", err)
	}
}

// TestCancelDeletesInFlightBuildJob covers the repo-backed path (t002): a
// build Job named per the operator's own convention (buildJobName mirrors
// lego/operator/internal/build.JobName) is deleted when Cancel runs against
// a repo-backed App with an open deploy.
func TestCancelDeletesInFlightBuildJob(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "", 3, store.CommitInfo{})
	app := sampleApp("web", "srv-1")
	app.Spec.Image = ""
	app.Spec.Repo = "https://example.invalid/acme/web.git"
	app.Generation = 5 // deliberately different from the deploy's own Generation (3) — Cancel must use the row's, not this
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: buildJobName("web", 3), Namespace: "default"}}
	svc, cl := newService(ds, app, job)

	if _, err := svc.Cancel(context.Background(), "web", first.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	var gone batchv1.Job
	err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: buildJobName("web", 3)}, &gone)
	if !apierrors.IsNotFound(err) {
		t.Errorf("build job after cancel: want deleted (NotFound), got %v", err)
	}
	got := getApp(t, cl, "web")
	if marker := got.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration]; marker != "3" {
		t.Errorf("canceled release marker = %q, want deploy generation 3", marker)
	}
}

func TestCancelDeletesInFlightKpackImage(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "", 3, store.CommitInfo{})
	app := sampleApp("web", "srv-1")
	app.Spec.Image = ""
	app.Spec.Repo = "https://example.invalid/acme/web.git"
	image := &unstructured.Unstructured{}
	image.SetGroupVersionKind(schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: "Image"})
	image.SetName(buildJobName("web", 3))
	image.SetNamespace("default")
	svc, cl := newService(ds, app, image)

	if _, err := svc.Cancel(context.Background(), "web", first.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	gone := &unstructured.Unstructured{}
	gone.SetGroupVersionKind(image.GroupVersionKind())
	err := cl.Get(context.Background(), client.ObjectKeyFromObject(image), gone)
	if !apierrors.IsNotFound(err) {
		t.Errorf("kpack Image after cancel: want deleted (NotFound), got %v", err)
	}
	got := getApp(t, cl, "web")
	if marker := got.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration]; marker != "3" {
		t.Errorf("canceled release marker = %q, want 3", marker)
	}
}

// TestCancelImageBackedAppStampsCanceledReleaseNoJob covers the image-backed
// case (w6/m104): there is no build Job to delete, but Cancel must STILL stamp
// AnnotationCanceledReleaseGeneration with the deploy row's own generation. That
// stamp is the only signal that makes the level-triggered operator settle the
// App CR (settleCanceledRelease) instead of converging the canceled image
// forever with no terminal phase. Before m104 the stamp sat inside
// `if a.Spec.Repo != ""`, so an image-backed cancel closed the deploy row but
// never touched the App, leaving the service stuck Deploying — the regression
// this asserts against. It also proves the w6/m128 fix stays scoped to
// repo-backed deploys: an image-backed deploy has no build phase, so Cancel
// must not manufacture a build_started/build_ended pair for one.
func TestCancelImageBackedAppStampsCanceledReleaseNoJob(t *testing.T) {
	ds := newFakeStore()
	// Deploy generation (4) is deliberately distinct from the App's current
	// Generation (sampleApp uses 1): Cancel must stamp the row's own, per m52.
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 4, store.CommitInfo{})
	svc, cl := newService(ds, sampleApp("web", "srv-1")) // Repo == "", Image set — no Job ever existed

	got, err := svc.Cancel(context.Background(), "web", first.ID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got.Status != store.DeployCanceled {
		t.Errorf("canceled deploy = %+v", got)
	}
	app := getApp(t, cl, "web")
	if marker := app.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration]; marker != "4" {
		t.Errorf("canceled release marker = %q, want deploy generation 4 — an image-backed cancel must stamp it too", marker)
	}
	if len(ds.facts) != 0 {
		t.Errorf("image-backed cancel emitted build lifecycle facts %+v, want none", ds.facts)
	}
}

// TestCancelMidBuildEmitsBuildEndedCanceled is the w6/m128 bug: a deploy
// canceled while still build_in_progress used to close with three events —
// deploy_started, build_started, deploy_ended(canceled) — and no build_ended,
// leaving the build lifecycle unclosed in the feed forever (the reconciler
// never gets another pass over a row Cancel just made terminal). It must now
// carry a build_ended fact with a canceled outcome, per buildEndedStatus's
// first branch.
func TestCancelMidBuildEmitsBuildEndedCanceled(t *testing.T) {
	ds := newFakeStore()
	started := time.Date(2026, 8, 27, 23, 55, 6, 0, time.UTC)
	// Image "" — a repo build's row is born without one (CreateDeploy stamps the
	// service's spec.image); a row born WITH one skips the build entirely (w6/061).
	ds.byApp["srv-1"] = []store.Deploy{{
		ID: "dep-1", AppID: "srv-1", Generation: 2,
		Status: store.DeployBuildInProgress, CreatedAt: started, StartedAt: &started,
	}}
	app := sampleApp("web", "srv-1")
	app.Spec.Image = ""
	app.Spec.Repo = "https://example.invalid/acme/web.git"
	svc, _ := newService(ds, app)

	got, err := svc.Cancel(context.Background(), "web", "dep-1")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got.Status != store.DeployCanceled {
		t.Fatalf("canceled deploy = %+v", got)
	}

	var buildStarted, buildEnded *store.ServiceEventFact
	for i := range ds.facts {
		switch ds.facts[i].Type {
		case store.EventFactBuildStarted:
			buildStarted = &ds.facts[i]
		case store.EventFactBuildEnded:
			buildEnded = &ds.facts[i]
		}
	}
	if buildStarted == nil || !buildStarted.At.Equal(started) {
		t.Errorf("build_started = %+v, want one at %s", buildStarted, started)
	}
	if buildEnded == nil || buildEnded.Status != store.EventStatusCanceled {
		t.Fatalf("build_ended = %+v, want status canceled", buildEnded)
	}
	if buildEnded.SourceKey != "deploy:dep-1:build_ended" {
		t.Errorf("build_ended source key = %q, want deploy:dep-1:build_ended", buildEnded.SourceKey)
	}

	// A retried Cancel (client timeout, at-least-once webhook redelivery, …) or
	// a reconciler pass racing the same close must not double the pair: the
	// source_key stays "deploy:dep-1:build_started"/"build_ended" regardless of
	// how many times it is derived, and InsertServiceEventFact is ON CONFLICT
	// (source_key) DO NOTHING (event_facts.go), so re-deriving from the same
	// pre-cancel snapshot and re-inserting is a no-op.
	for _, fact := range store.CanceledBuildLifecycleFacts(store.Deploy{
		ID: "dep-1", AppID: "srv-1", Generation: 2,
		Status: store.DeployBuildInProgress, CreatedAt: started, StartedAt: &started,
	}) {
		if _, err := ds.InsertServiceEventFact(context.Background(), fact); err != nil {
			t.Fatalf("re-insert %s: %v", fact.SourceKey, err)
		}
	}
	if len(ds.facts) != 2 {
		t.Fatalf("facts after a re-derived re-insert = %+v, want still exactly build_started + build_ended", ds.facts)
	}
}

// TestCancelQueuedEmitsNoBuildFacts is the adjacent class the fix must not
// disturb: a deploy canceled before its build ever dispatched has nothing to
// report, so it must keep emitting neither build_started nor build_ended
// (buildStartedAt withholds both while the deploy never left the queue).
func TestCancelQueuedEmitsNoBuildFacts(t *testing.T) {
	ds := newFakeStore()
	ds.byApp["srv-1"] = []store.Deploy{{
		ID: "dep-1", AppID: "srv-1", Generation: 2,
		Status: store.DeployQueued, OverlapPending: true, CreatedAt: time.Now(),
	}}
	app := sampleApp("web", "srv-1")
	app.Spec.Image = ""
	app.Spec.Repo = "https://example.invalid/acme/web.git"
	svc, _ := newService(ds, app)

	if _, err := svc.Cancel(context.Background(), "web", "dep-1"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if len(ds.facts) != 0 {
		t.Errorf("canceled-while-queued facts = %+v, want none", ds.facts)
	}
}

// TestCancelRollbackDeployEmitsNoBuildFacts is w6/061 on the Cancel verb's own
// buildLifecycleFacts derivation: a rollback deploy on a repo-backed service is
// born with the restored image and runs no build, so canceling it mid-rollout
// must emit neither build_started nor build_ended — before the per-deploy
// guard, the a.Spec.Repo gate here manufactured a build_started +
// build_ended(succeeded) pair for a build that never existed.
func TestCancelRollbackDeployEmitsNoBuildFacts(t *testing.T) {
	ds := newFakeStore()
	started := time.Date(2026, 8, 27, 12, 28, 57, 0, time.UTC)
	ds.byApp["srv-1"] = []store.Deploy{{
		ID: "dep-rb", AppID: "srv-1", Trigger: "rollback", Generation: 3,
		Image: "web:gen-1@sha256:c0dd", ResolvedImage: "web:gen-1@sha256:c0dd", RollbackOf: "dep-old",
		Status: store.DeployUpdateInProgress, CreatedAt: started, StartedAt: &started,
	}}
	app := sampleApp("web", "srv-1")
	app.Spec.Image = ""
	app.Spec.Repo = "https://example.invalid/acme/web.git"
	svc, _ := newService(ds, app)

	got, err := svc.Cancel(context.Background(), "web", "dep-rb")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got.Status != store.DeployCanceled {
		t.Fatalf("canceled deploy = %+v", got)
	}
	if len(ds.facts) != 0 {
		t.Errorf("canceled rollback deploy emitted build facts %+v, want none — no build ever ran", ds.facts)
	}
}

// TestCancelAfterBuildFinishedEmitsBuildEndedSucceeded is the other adjacent
// class the fix must not disturb: a deploy canceled once its build already
// finished (now sitting in a later phase, e.g. update_in_progress) reports its
// build as having succeeded, per buildEndedStatus's second branch — the build
// itself was never canceled, only the rollout that followed it.
func TestCancelAfterBuildFinishedEmitsBuildEndedSucceeded(t *testing.T) {
	ds := newFakeStore()
	started := time.Date(2026, 8, 27, 23, 52, 24, 0, time.UTC)
	ds.byApp["srv-1"] = []store.Deploy{{
		ID: "dep-1", AppID: "srv-1", Generation: 2,
		Status: store.DeployUpdateInProgress, CreatedAt: started, StartedAt: &started,
	}}
	app := sampleApp("web", "srv-1")
	app.Spec.Image = ""
	app.Spec.Repo = "https://example.invalid/acme/web.git"
	svc, _ := newService(ds, app)

	if _, err := svc.Cancel(context.Background(), "web", "dep-1"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	var buildEnded *store.ServiceEventFact
	for i := range ds.facts {
		if ds.facts[i].Type == store.EventFactBuildEnded {
			buildEnded = &ds.facts[i]
		}
	}
	if buildEnded == nil || buildEnded.Status != store.EventStatusSucceeded {
		t.Fatalf("build_ended after build finished = %+v, want status succeeded", buildEnded)
	}
}

// --- Rollback ----------------------------------------------------------------

func TestRollbackRestoresPreviousLiveImage(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	if won, err := ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "web:v1"); err != nil || !won {
		t.Fatalf("close first live: won=%v err=%v", won, err)
	}
	bad, _ := ds.CreateDeploy(context.Background(), "srv-1", "api", "web:bad", 2, store.CommitInfo{})
	if won, err := ds.CloseDeploy(context.Background(), bad.ID, store.DeployUpdateFailed, ""); err != nil || !won {
		t.Fatalf("close bad failed: won=%v err=%v", won, err)
	}
	app := sampleApp("web", "srv-1")
	app.Spec.Image = "web:bad"
	svc, cl := newService(ds, app)

	rolled, err := svc.Rollback(context.Background(), "web", first.ID)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolled.Trigger != "rollback" || rolled.Image != "web:v1" || rolled.RollbackOf != first.ID {
		t.Fatalf("rolled-back deploy = %+v", rolled)
	}
	if ds.setImage["srv-1"] != "web:v1" {
		t.Errorf("row-first write: SetAppImage(srv-1, ...) = %q, want web:v1", ds.setImage["srv-1"])
	}
	got := getApp(t, cl, "web")
	if got.Spec.Image != "web:v1" {
		t.Errorf("app spec.image after rollback = %q, want web:v1", got.Spec.Image)
	}
	if got.Spec.RestartedAt == "" {
		t.Error("Rollback must bump spec.restartedAt so the CR converges immediately")
	}
	if annotation := got.Annotations[appv1alpha1.AnnotationReleaseGeneration]; annotation != "2" {
		t.Errorf("release-generation annotation = %q, want 2", annotation)
	}

	list, err := svc.List(context.Background(), "web", ListFilter{})
	if err != nil || len(list) != 3 || list[0].ID != rolled.ID {
		t.Fatalf("List after rollback (want newest-first, 3 entries) = %+v (err %v)", list, err)
	}
}

// w4/051: rolling back to the CURRENT live deploy whose image is already the
// running image is a no-op that would only restart the service and mint a
// redundant deploy — every surface must refuse it with a 409.
func TestRollbackRefusesCurrentLiveNoOp(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	if won, err := ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "web:v1"); err != nil || !won {
		t.Fatalf("close first live: won=%v err=%v", won, err)
	}
	// sampleApp's spec.image is "web:v1" == the live deploy's image: the no-op.
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	if _, err := svc.Rollback(context.Background(), "web", first.ID); !errors.Is(err, core.ErrConflict) {
		t.Errorf("rollback to the current live deploy (same image): want core.ErrConflict, got %v", err)
	}
}

// The complement of the no-op guard: after a failed deploy drifts spec.image off
// the still-live last-good deploy, rolling back to that live deploy restores the
// good image and IS allowed — the guard keys on the image, not merely the status
// (w4/051; mirrors TestRollbackRestoresPreviousLiveImage at the reject arm).
func TestRollbackToLiveDeployRecoversDriftedImage(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	if won, err := ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "web:v1"); err != nil || !won {
		t.Fatalf("close first live: won=%v err=%v", won, err)
	}
	// A later deploy failed and left spec.image drifted to its bad image while
	// `first` stayed live (a failed deploy never deactivates the prior).
	app := sampleApp("web", "srv-1")
	app.Spec.Image = "web:bad"
	svc, _ := newService(ds, app)

	rolled, err := svc.Rollback(context.Background(), "web", first.ID)
	if err != nil {
		t.Fatalf("rollback to still-live last-good deploy (drifted image): %v", err)
	}
	if rolled.Image != "web:v1" || rolled.RollbackOf != first.ID {
		t.Errorf("recovery rollback = %+v, want image web:v1 rolling back first", rolled)
	}
}

func TestRollbackRefusesNonLiveTarget(t *testing.T) {
	ds := newFakeStore()
	stillOpen, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	if _, err := svc.Rollback(context.Background(), "web", stillOpen.ID); !errors.Is(err, core.ErrConflict) {
		t.Errorf("rollback to a never-live deploy: want core.ErrConflict, got %v", err)
	}
}

func TestRollbackUnknownDeployIsNotFound(t *testing.T) {
	ds := newFakeStore()
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	if _, err := svc.Rollback(context.Background(), "web", "dep-nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("rollback unknown deploy: want core.ErrNotFound, got %v", err)
	}
}

func TestRollbackRefusesSuspendedService(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	_, _ = ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "web:v1")
	app := sampleApp("web", "srv-1")
	app.Spec.Suspended = true
	svc, _ := newService(ds, app)

	if _, err := svc.Rollback(context.Background(), "web", first.ID); !errors.Is(err, core.ErrConflict) {
		t.Errorf("rollback of suspended service: want core.ErrConflict, got %v", err)
	}
}

func TestRollbackRequiresStoreManagedApp(t *testing.T) {
	ds := newFakeStore()
	svc, _ := newService(ds, sampleApp("manual", ""))

	if _, err := svc.Rollback(context.Background(), "manual", "dep-1"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("hand-applied rollback: want core.ErrBadRequest, got %v", err)
	}
}

// --- Store-off 503 ------------------------------------------------------------

func TestCancelRollbackUnavailableWithoutStore(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web", "srv-1"))
	ctx := context.Background()

	if _, err := svc.Cancel(ctx, "web", "dep-1"); !errors.Is(err, core.ErrDeploysUnavailable) {
		t.Errorf("Cancel: want ErrDeploysUnavailable, got %v", err)
	}
	if _, err := svc.Rollback(ctx, "web", "dep-1"); !errors.Is(err, core.ErrDeploysUnavailable) {
		t.Errorf("Rollback: want ErrDeploysUnavailable, got %v", err)
	}
}

// --- REST fragment -------------------------------------------------------------

func TestRESTCancelAndRollback(t *testing.T) {
	ds := newFakeStore()
	open, _ := ds.CreateDeploy(context.Background(), "srv-1", "api", "web:v2", 2, store.CommitInfo{})
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/deploys/"+open.ID+"/cancel", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel: code=%d body=%s", rec.Code, rec.Body)
	}
	var canceled renderDeploy
	if err := json.Unmarshal(rec.Body.Bytes(), &canceled); err != nil || canceled.Status != store.DeployCanceled {
		t.Fatalf("cancel body = %s (err %v)", rec.Body, err)
	}

	// Cancel again: past the cancelable window, Render's 409.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/deploys/"+open.ID+"/cancel", nil))
	if rec.Code != http.StatusConflict {
		t.Errorf("re-cancel: code=%d, want 409", rec.Code)
	}

	// Rollback restores a PREVIOUS (deactivated) deploy — not the current live
	// one, which would be a no-op restart (w4/051). Take `old` live, then have a
	// newer deploy go live so `old` is deactivated, and roll back to `old`.
	old, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v0", 1, store.CommitInfo{})
	if won, err := ds.CloseDeploy(context.Background(), old.ID, store.DeployLive, "web:v0"); err != nil || !won {
		t.Fatalf("close old live: won=%v err=%v", won, err)
	}
	live, _ := ds.CreateDeploy(context.Background(), "srv-1", "api", "web:v1", 3, store.CommitInfo{})
	if won, err := ds.CloseDeploy(context.Background(), live.ID, store.DeployLive, "web:v1"); err != nil || !won {
		t.Fatalf("close live (supersedes old): won=%v err=%v", won, err)
	}

	rec = httptest.NewRecorder()
	body := strings.NewReader(`{"deployId":"` + old.ID + `"}`)
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/rollback", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("rollback: code=%d body=%s", rec.Code, rec.Body)
	}
	var rolled renderDeploy
	if err := json.Unmarshal(rec.Body.Bytes(), &rolled); err != nil || rolled.Trigger != "rollback" || rolled.RollbackOf != old.ID {
		t.Fatalf("rollback body = %s (err %v)", rec.Body, err)
	}
}

func TestREST503CancelRollbackWithoutStore(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web", "srv-1"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/deploys/dep-1/cancel", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("cancel: code=%d, want 503", rec.Code)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/rollback", strings.NewReader(`{"deployId":"dep-1"}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("rollback: code=%d, want 503", rec.Code)
	}
}

// --- MCP parity ---------------------------------------------------------------

// TestMCPRegistersCancelAndRollback covers three-adapter parity for w2/m10:
// cancel_deploy/rollback_deploy are registered alongside list_deploys/get_deploy.
func TestMCPRegistersCancelAndRollback(t *testing.T) {
	ds := newFakeStore()
	first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{})
	_, _ = ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "web:v1")
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	have := map[string]bool{}
	for _, tl := range tools.Tools {
		have[tl.Name] = true
	}
	for _, want := range []string{"list_deploys", "get_deploy", "cancel_deploy", "rollback_deploy"} {
		if !have[want] {
			t.Errorf("tool %q not registered", want)
		}
	}

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "cancel_deploy", Arguments: map[string]any{"serviceId": "web", "deployId": first.ID}})
	if err != nil {
		t.Fatalf("cancel_deploy transport error: %v", err)
	}
	// first is already live (closed above) — past the cancelable window, a tool error.
	if !res.IsError {
		t.Errorf("cancel_deploy on a live deploy: want a tool error, got %+v", res)
	}

	second, _ := ds.CreateDeploy(context.Background(), "srv-1", "api", "web:v2", 2, store.CommitInfo{})
	_, _ = ds.CloseDeploy(context.Background(), second.ID, store.DeployLive, "web:v2")
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "rollback_deploy", Arguments: map[string]any{"serviceId": "web", "deployId": first.ID}})
	if err != nil || res.IsError {
		t.Fatalf("rollback_deploy: %v isErr=%v", err, res.IsError)
	}
	var got renderDeploy
	if err := decodeStructured(res.StructuredContent, &got); err != nil {
		t.Fatalf("decode rollback_deploy result: %v", err)
	}
	if got.Trigger != "rollback" || got.RollbackOf != first.ID {
		t.Errorf("rollback_deploy result = %+v", got)
	}
}
