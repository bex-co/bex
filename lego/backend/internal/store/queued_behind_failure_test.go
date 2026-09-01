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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// This file is the bex-api half of w6/m100. The operator's half stops a deploy
// queued behind a build-failed sibling from being stranded (its release
// generation now advances past the failed one); these tests pin what this side
// must do with that advance:
//
//   - the queued row is finally PROMOTED. supersededDeployStatus's
//     OverlapPending branch was only ever exercised in its waiting state, on
//     the documented assumption that the operator would eventually adopt the
//     row's generation — the assumption the operator bug broke.
//   - the row whose build actually failed still closes build_failed WITH its
//     build error, not canceled. The operator adopts the pending generation
//     milliseconds after recording the failure, so on a 30s resync this pass
//     almost always observes the CR only afterwards; status.conditions[Build]
//     is what keeps the verdict readable that late.

const failedBuildMessage = "build failed: PodFailurePolicy: Container clone for pod " +
	"bex-build/bld-qa-rollback-test-gen-1 failed with exit code 90 matching FailJob rule at index 1"

// crAdoptedPendingAfterBuildFailure is the App CR as the operator leaves it one
// reconcile after gen-1's build failed with gen-2 already queued behind it:
// gen-1's verdict durably recorded, gen-2 adopted as the release and building.
func crAdoptedPendingAfterBuildFailure(failedGen, activeGen int64) *appv1alpha1.App {
	cr := buildingApp(activeGen, appv1alpha1.ReasonBuilding, "")
	cr.Status.Conditions = append(cr.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionBuild, Status: metav1.ConditionFalse,
		Reason: appv1alpha1.ReasonBuildFailedUserError, Message: failedBuildMessage,
		ObservedGeneration: failedGen,
	})
	return cr
}

// crSupersededWithoutBuildVerdict is the same CR with no durable verdict at all:
// a legacy operator, or a release genuinely superseded with nothing failed.
func crSupersededWithoutBuildVerdict(activeGen int64) *appv1alpha1.App {
	return buildingApp(activeGen, appv1alpha1.ReasonBuilding, "")
}

// The queued row waits while the operator is still on the older release, then
// reports the live build the moment that release becomes its own.
func TestQueuedRowIsPromotedOnceReleaseGenerationAdvances(t *testing.T) {
	queued := Deploy{Generation: 2, Status: DeployQueued, OverlapPending: true}

	building := buildingApp(2, appv1alpha1.ReasonBuilding, "")
	building.Status.ReleaseGeneration = 1

	// Still gen-1's build: the pending slot legitimately waits through it, and
	// its elapsed time is not evidence of an orphan (timedOut is true here).
	if got := observedDeployStatus(queued, building, true); got != "" {
		t.Errorf("queued row while the operator is still on gen-1 = %q, want no transition", got)
	}

	adopted := crAdoptedPendingAfterBuildFailure(1, 2)
	if got := observedDeployStatus(queued, adopted, false); got != DeployBuildInProgress {
		t.Errorf("queued row after the release advanced = %q, want %q", got, DeployBuildInProgress)
	}
}

// The failed row's own verdict outranks the generation comparison — otherwise
// the deploy that actually broke reports canceled and loses its build error.
func TestFailedRowClosesBuildFailedEvenAfterTheReleaseMovedOn(t *testing.T) {
	failed := Deploy{Generation: 1, Status: DeployBuildInProgress}
	adopted := crAdoptedPendingAfterBuildFailure(1, 2)

	if got := observedDeployStatus(failed, adopted, false); got != DeployBuildFailed {
		t.Errorf("row whose build failed = %q, want %q", got, DeployBuildFailed)
	}

	// No durable verdict for this row (a legacy operator, or a genuine
	// supersede where nothing failed): the missed-observation fallback stands.
	if got := observedDeployStatus(failed, crSupersededWithoutBuildVerdict(2), false); got != DeployCanceled {
		t.Errorf("genuinely superseded row = %q, want %q", got, DeployCanceled)
	}

	// A verdict for a DIFFERENT generation must never be borrowed.
	other := crAdoptedPendingAfterBuildFailure(7, 9)
	if got := observedDeployStatus(failed, other, false); got != DeployCanceled {
		t.Errorf("row with another generation's verdict = %q, want %q", got, DeployCanceled)
	}
}

// newQueuedBehindFailureFixture opens the live-shaped pair of rows: deploy 1
// executing against gen-1, deploy 2 queued behind it against gen-2. second is
// created by createSecond so the same fixture serves Manual Deploy and Rollback.
func newQueuedBehindFailureFixture(
	t *testing.T,
	createSecond func(m *memStore, appID string) (Deploy, error),
) (*Reconciler, *memStore, DesiredApp, Deploy, Deploy) {
	t.Helper()
	ctx := context.Background()
	// The default gate timeouts (35m build / 12m pre-deploy / 18m deploy) keep
	// every gate closed for rows this fixture opened microseconds ago, so no row
	// here is ever evaluated as timed out.
	rec, st, _ := newTestReconciler(t)
	tenant, err := st.CreateTenant(ctx, "acme", "free")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "web", Repo: "https://github.com/bex-co/hello",
	})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}

	// CreateApp opens the service's own first deploy (trigger=create, gen 1) —
	// exactly the deploy the live repro's wrong branch failed on.
	first := onlyDeploy(t, st, app.ID)
	if won, err := st.TransitionDeploy(ctx, first.ID, DeployBuildInProgress, "", "", "", nil); err != nil || !won {
		t.Fatalf("start first build: won=%v err=%v", won, err)
	}
	second, err := createSecond(st, app.ID)
	if err != nil {
		t.Fatalf("create second deploy: %v", err)
	}
	if second.Status != DeployQueued || !second.OverlapPending {
		t.Fatalf("second deploy = %s (overlapPending=%v), want queued in the pending slot",
			second.Status, second.OverlapPending)
	}

	return rec, st, DesiredApp{App: app}, first, second
}

func reloadDeploy(t *testing.T, st *memStore, appID, deployID string) Deploy {
	t.Helper()
	d, err := st.GetDeploy(context.Background(), appID, deployID)
	if err != nil {
		t.Fatalf("GetDeploy(%s): %v", deployID, err)
	}
	return d
}

// The milestone's live repro, end to end on this side: with the operator fixed,
// one observation pass closes the failed deploy build_failed (carrying the build
// error) and moves the deploy that was stranded at queued into its own build.
func TestQueuedDeployBehindFailedBuildReachesTerminalStates(t *testing.T) {
	rec, st, desired, first, second := newQueuedBehindFailureFixture(t,
		func(m *memStore, appID string) (Deploy, error) {
			return m.CreateDeploy(context.Background(), appID, TriggerAPI, "", 2, CommitInfo{})
		})

	cr := crAdoptedPendingAfterBuildFailure(first.Generation, second.Generation)
	rec.recordObservations(context.Background(), desired, cr, []Deploy{
		reloadDeploy(t, st, desired.ID, first.ID),
		reloadDeploy(t, st, desired.ID, second.ID),
	})

	failed := reloadDeploy(t, st, desired.ID, first.ID)
	if failed.Status != DeployBuildFailed {
		t.Errorf("first deploy = %q, want %q", failed.Status, DeployBuildFailed)
	}
	if failed.FailureReason != failedBuildMessage {
		t.Errorf("first deploy failure reason = %q, want the operator's build error", failed.FailureReason)
	}
	promoted := reloadDeploy(t, st, desired.ID, second.ID)
	if promoted.Status != DeployBuildInProgress {
		t.Errorf("queued deploy = %q, want %q (this is the row that used to sit at queued forever)",
			promoted.Status, DeployBuildInProgress)
	}
	if promoted.StartedAt == nil {
		t.Error("the promoted deploy must stamp started_at — the live bug left it empty indefinitely")
	}
	if promoted.OverlapPending {
		t.Error("a promoted row is no longer the pending slot")
	}
}

// Rollback shares triggerFetched's patchApp/stampReleaseGeneration shape, so it
// races a failing build identically (milestone Blast radius #2). Its own deploy
// row must reach a terminal state rather than the same stuck queued.
func TestRollbackQueuedBehindFailedBuildReachesTerminalState(t *testing.T) {
	rec, st, desired, first, rollback := newQueuedBehindFailureFixture(t,
		func(m *memStore, appID string) (Deploy, error) {
			return m.CreateRollbackDeploy(context.Background(), appID, "web:v1", "dep-previous", 2, CommitInfo{})
		})
	if rollback.Trigger != TriggerRollback {
		t.Fatalf("fixture trigger = %q, want rollback", rollback.Trigger)
	}

	cr := crAdoptedPendingAfterBuildFailure(first.Generation, rollback.Generation)
	rec.recordObservations(context.Background(), desired, cr, []Deploy{
		reloadDeploy(t, st, desired.ID, first.ID),
		reloadDeploy(t, st, desired.ID, rollback.ID),
	})

	if got := reloadDeploy(t, st, desired.ID, first.ID).Status; got != DeployBuildFailed {
		t.Errorf("first deploy = %q, want %q", got, DeployBuildFailed)
	}
	promoted := reloadDeploy(t, st, desired.ID, rollback.ID)
	if promoted.Status == DeployQueued {
		t.Fatalf("rollback deploy is still queued — the stuck-queue bug reproduces on the Rollback call site")
	}
	if promoted.Status != DeployBuildInProgress {
		t.Errorf("rollback deploy = %q, want %q", promoted.Status, DeployBuildInProgress)
	}
}

// Cancel must keep working on a row in the affected state: the durable build
// verdict is read only for rows still in a build phase, so a row a human (or
// the Cancel verb) has already closed is never repainted build_failed.
func TestRecordedBuildFailureNeverRepaintsAClosedRow(t *testing.T) {
	cr := crAdoptedPendingAfterBuildFailure(1, 2)
	for _, status := range []string{DeployCanceled, DeployLive, DeployUpdateFailed, DeployUpdateInProgress} {
		got, settled := supersededDeployStatus(Deploy{Generation: 1, Status: status}, cr, false)
		if settled && got == DeployBuildFailed {
			t.Errorf("row already past its build at %q was repainted %q", status, got)
		}
	}
}
