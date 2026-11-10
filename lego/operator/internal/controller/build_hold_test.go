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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// heldApp is an App waiting as BuildQueued for its current release (generation
// gen), with a hold that began at since.
func heldApp(name, workspace string, gen int64, cause string, since time.Time) *appv1alpha1.App {
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: workspace, UID: types.UID("uid-" + name),
		Labels: map[string]string{labelWorkspace: workspace},
	}}
	app.Status.ReleaseGeneration = gen
	app.Status.Phase = appv1alpha1.PhaseBuilding
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reasonBuildQueued, Message: "waiting",
	})
	app.Status.BuildHold = &appv1alpha1.BuildHoldStatus{Generation: gen, Since: metav1.NewTime(since), Cause: cause}
	return app
}

// holdObservations is bex_build_hold_seconds' sample count for one cause. The
// series is absent until its first observation, which reads as zero.
func holdObservations(cause string) uint64 {
	count, _ := histogramSamples("bex_build_hold_seconds", "cause", cause)
	return count
}

func TestHoldBuildKeepsTheFirstMomentOfAGenerationsWait(t *testing.T) {
	start := time.Now().Add(-5 * time.Minute)
	app := &appv1alpha1.App{}
	app.Status.ReleaseGeneration = 3

	if !holdBuild(app, appv1alpha1.BuildHoldDatastore, start) {
		t.Fatal("the first hold must change status")
	}
	if holdBuild(app, appv1alpha1.BuildHoldDatastore, time.Now()) {
		t.Error("re-stamping the same hold must not report a change (no status write per requeue)")
	}
	if !holdBuild(app, appv1alpha1.BuildHoldAdmission, time.Now()) {
		t.Error("a new cause is a change")
	}
	if got := app.Status.BuildHold.Since.Time; !got.Equal(start) {
		t.Errorf("a later pass or a new cause moved the start to %v; the wait began at %v", got, start)
	}

	// A newer release is a new deploy and a new wait.
	app.Status.ReleaseGeneration = 4
	later := time.Now()
	if !holdBuild(app, appv1alpha1.BuildHoldAdmission, later) {
		t.Fatal("a new generation's hold must change status")
	}
	if hold := app.Status.BuildHold; hold.Generation != 4 || !hold.Since.Time.Equal(later) {
		t.Errorf("hold = %+v, want generation 4 starting at %v", hold, later)
	}
}

func TestEndBuildHoldObservesOnlyTheCurrentGenerationsWait(t *testing.T) {
	app := heldApp("ends", "tea-end", 2, appv1alpha1.BuildHoldAdmission, time.Now().Add(-90*time.Second))
	before := holdObservations(appv1alpha1.BuildHoldAdmission)
	if !endBuildHold(app, time.Now()) {
		t.Fatal("ending a hold must change status")
	}
	if app.Status.BuildHold != nil {
		t.Error("the hold must be cleared once the build exists")
	}
	if got := holdObservations(appv1alpha1.BuildHoldAdmission); got != before+1 {
		t.Errorf("admission hold observations = %d, want %d", got, before+1)
	}
	if endBuildHold(app, time.Now()) {
		t.Error("no hold, no change")
	}

	// A hold left by an earlier generation is cleared without being observed:
	// it does not describe the build that now exists.
	stale := heldApp("stale", "tea-end", 2, appv1alpha1.BuildHoldDatastore, time.Now().Add(-time.Hour))
	stale.Status.ReleaseGeneration = 3
	before = holdObservations(appv1alpha1.BuildHoldDatastore)
	endBuildHold(stale, time.Now())
	if got := holdObservations(appv1alpha1.BuildHoldDatastore); got != before {
		t.Errorf("a stale generation's hold was observed (%d → %d)", before, got)
	}
}

func TestHeldBuildIgnoresHoldsThatNoLongerDescribeAWaitingDeploy(t *testing.T) {
	since := time.Now().Add(-time.Minute)
	if heldBuild(heldApp("live", "tea-h", 1, appv1alpha1.BuildHoldAdmission, since)) == nil {
		t.Fatal("a BuildQueued App with its current release's hold is held")
	}
	cases := map[string]func(*appv1alpha1.App){
		"a newer release":    func(a *appv1alpha1.App) { a.Status.ReleaseGeneration = 2 },
		"no longer building": func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseFailed },
		"building, not queued": func(a *appv1alpha1.App) {
			meta.SetStatusCondition(&a.Status.Conditions, metav1.Condition{
				Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reasonBuilding,
			})
		},
		"being deleted": func(a *appv1alpha1.App) { now := metav1.Now(); a.DeletionTimestamp = &now },
		"no hold":       func(a *appv1alpha1.App) { a.Status.BuildHold = nil },
	}
	for name, mutate := range cases {
		app := heldApp("x", "tea-h", 1, appv1alpha1.BuildHoldAdmission, since)
		mutate(app)
		if heldBuild(app) != nil {
			t.Errorf("%s: still counted as held", name)
		}
	}
}

func TestFairOrderRanksTheQuieterWorkspaceThenTheLongerWait(t *testing.T) {
	now := time.Now()
	byWorkspace := map[string]int{"tea-busy": 2, "tea-quiet": 0}
	busyOld := heldEntry{workspace: "tea-busy", since: now.Add(-10 * time.Minute), key: "tea-busy/a"}
	quietNew := heldEntry{workspace: "tea-quiet", since: now, key: "tea-quiet/b"}
	if !quietNew.ranksBefore(busyOld, byWorkspace) || busyOld.ranksBefore(quietNew, byWorkspace) {
		t.Error("a workspace running fewer builds must go first, however long the other has waited")
	}
	older := heldEntry{workspace: "tea-x", since: now.Add(-time.Minute), key: "z"}
	newer := heldEntry{workspace: "tea-y", since: now, key: "a"}
	if !older.ranksBefore(newer, byWorkspace) || newer.ranksBefore(older, byWorkspace) {
		t.Error("between equally busy workspaces, the longer wait must go first")
	}
	tieA := heldEntry{workspace: "tea-x", since: now, key: "a"}
	tieB := heldEntry{workspace: "tea-y", since: now, key: "b"}
	if !tieA.ranksBefore(tieB, byWorkspace) || tieB.ranksBefore(tieA, byWorkspace) {
		t.Error("a full tie must still be a total order, so two racers agree")
	}
}

func TestFairOrderAheadCountsOnlyOtherAdmissionHolds(t *testing.T) {
	now := time.Now()
	self := heldApp("self", "tea-self", 1, appv1alpha1.BuildHoldAdmission, now.Add(-time.Minute))
	ahead := heldApp("ahead", "tea-other", 1, appv1alpha1.BuildHoldAdmission, now.Add(-2*time.Minute))
	behind := heldApp("behind", "tea-other", 1, appv1alpha1.BuildHoldAdmission, now)
	datastore := heldApp("datastore", "tea-other", 1, appv1alpha1.BuildHoldDatastore, now.Add(-time.Hour))
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(self, ahead, behind, datastore).Build()
	r := &AppReconciler{Client: cl}

	got, err := r.fairOrderAhead(context.Background(), self, map[string]int{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Errorf("ahead = %d, want 1: only the older admission hold ranks first; a datastore wait holds no slot", got)
	}

	// A newcomer without a hold ranks as arriving now: behind every earlier hold.
	newcomer := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "new", Namespace: "tea-new", UID: "uid-new",
		Labels: map[string]string{labelWorkspace: "tea-new"}}}
	got, err = r.fairOrderAhead(context.Background(), newcomer, map[string]int{}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Errorf("newcomer ahead = %d, want 3 (self, ahead, behind are all held before it)", got)
	}
}

func TestCountHeldBuildsByCauseAndOldest(t *testing.T) {
	now := time.Now().Truncate(time.Second) // status round-trips at second precision
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(
		heldApp("a", "tea-c", 1, appv1alpha1.BuildHoldAdmission, now.Add(-3*time.Minute)),
		heldApp("b", "tea-c", 1, appv1alpha1.BuildHoldAdmission, now.Add(-time.Minute)),
		heldApp("c", "tea-d", 1, appv1alpha1.BuildHoldDatastore, now.Add(-7*time.Minute)),
	).Build()

	census, err := countHeldBuilds(context.Background(), cl, now)
	if err != nil {
		t.Fatal(err)
	}
	if census.held[appv1alpha1.BuildHoldAdmission] != 2 || census.held[appv1alpha1.BuildHoldDatastore] != 1 {
		t.Errorf("held = %v, want admission 2, datastore 1", census.held)
	}
	if census.oldest != 7*time.Minute {
		t.Errorf("oldest = %v, want 7m: a datastore wait is a held deploy too", census.oldest)
	}

	publishHoldCensus(census)
	if got := testutil.ToFloat64(buildsHeld.WithLabelValues(appv1alpha1.BuildHoldAdmission)); got != 2 {
		t.Errorf("bex_builds_held{cause=admission} = %v, want 2", got)
	}
	publishHoldCensus(holdCensus{held: map[string]int{}})
	if got := testutil.ToFloat64(buildsHeld.WithLabelValues(appv1alpha1.BuildHoldAdmission)); got != 0 {
		t.Errorf("an emptied cause must read 0, got %v", got)
	}
	if got := testutil.ToFloat64(buildHoldOldestSeconds); got != 0 {
		t.Errorf("bex_build_hold_oldest_seconds = %v, want 0 when nothing is held", got)
	}
}
