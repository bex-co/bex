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
	"errors"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// buildingApp is an App CR parked in PhaseBuilding with reason at the current
// generation — the exact CR shape the three w6/m95 production repros presented.
func buildingApp(gen int64, reason, message string) *appv1alpha1.App {
	app := &appv1alpha1.App{}
	app.Generation = gen
	app.Status.Phase = appv1alpha1.PhaseBuilding
	app.Status.ReleaseGeneration = gen
	app.Status.Conditions = []metav1.Condition{{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: reason,
		Message: message, ObservedGeneration: gen,
	}}
	return app
}

// TestBuildInProgressRequiresProofOfARunningBuild is the w6/m95 regression: the
// deploy row's build_in_progress is a claim that a BuildKit pod is compiling
// right now — the build-log stream promises to follow it, and the dashboard
// spins on it. Before this fix the control plane granted that status to EVERY
// PhaseBuilding reason except the one it happened to recognise, so
// RegistryCredsPending — which the operator writes BEFORE the build Job exists
// at all, while zot has not yet re-read a fresh App's push credential — reported
// a live build with no Job, no pod, and a log subscribe that answered
// "no running build is available to follow" in under 20ms.
func TestBuildInProgressRequiresProofOfARunningBuild(t *testing.T) {
	const gen = int64(3)
	open := Deploy{Generation: gen, Status: DeployCreated}

	for _, tc := range []struct {
		reason string
		want   string
		why    string
	}{
		{appv1alpha1.ReasonBuilding, DeployBuildInProgress, "a placed, running build pod is the one thing that licenses build_in_progress"},
		{appv1alpha1.ReasonBuildQueued, DeployQueued, "dispatched but nothing placed"},
		{appv1alpha1.ReasonRegistryCredsPending, DeployQueued, "parked before the build Job is even created"},
		{"SomeReasonAFutureOperatorInvents", DeployQueued, "an unrecognised wait must fail toward queued, never toward a claim of running infrastructure"},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			got := observedDeployStatus(open, buildingApp(gen, tc.reason, "m"), false)
			if got != tc.want {
				t.Errorf("reason %q => %q, want %q — %s", tc.reason, got, tc.want, tc.why)
			}
		})
	}
}

// TestFirstDeployWithNoBackingBuildReachesATerminalStateWithinTheGateTimeout is
// the milestone's definition of done, end to end through a full reconcile pass:
// a fresh repo-backed service whose App CR sits in PhaseBuilding with no build
// behind it must not stay open forever. Its row reports the honest queued while
// it waits, and once it is past BuildGateTimeout it closes build_failed with
// updatedAt advanced — not byte-identical to startedAt across polls minutes
// apart, which is how the production repros were identified.
func TestFirstDeployWithNoBackingBuildReachesATerminalStateWithinTheGateTimeout(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	ten, _ := st.CreateTenant(ctx, "acme", "free")
	row, _ := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Repo: "https://github.com/bex-co/bex.git",
		Branch: "main", Port: 80, Replicas: 1, Tier: "free",
	})
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// The operator parks the App before dispatching any build: zot has not yet
	// accepted this brand-new App's push credential.
	const waitMsg = "Waiting for the registry to accept this app's build credential"
	app := getApp(t, cl)
	app.Status = buildingApp(app.Generation, appv1alpha1.ReasonRegistryCredsPending, waitMsg).Status
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	open := onlyDeploy(t, st, row.ID)
	if open.Status != DeployQueued {
		t.Fatalf("deploy status = %q, want %q — no build Job exists yet, so nothing is in progress", open.Status, DeployQueued)
	}
	if open.StartedAt != nil {
		t.Error("started_at was stamped for a build that has not started; the log stream narrates \"Building from ...\" off it")
	}

	// Now let it sit past the build gate, exactly as a frozen row would.
	backdateDeploy(t, st, open.ID, 40*time.Minute)
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	closed := onlyDeploy(t, st, row.ID)
	if closed.Status != DeployBuildFailed {
		t.Fatalf("deploy status after the gate timeout = %q, want %q", closed.Status, DeployBuildFailed)
	}
	if closed.FinishedAt == nil {
		t.Error("a terminal deploy must carry finished_at")
	}
	if !closed.UpdatedAt.After(open.UpdatedAt) {
		t.Errorf("updated_at did not advance (%v); a frozen row is the symptom this milestone closes", closed.UpdatedAt)
	}
	// The failure the tenant reads must name what actually happened. The generic
	// health-gate line points at service logs that do not exist, because the
	// service never built.
	if !strings.Contains(closed.FailureReason, waitMsg) {
		t.Errorf("failure_reason = %q, want the operator's own explanation of what the build was waiting on", closed.FailureReason)
	}
	if strings.Contains(closed.FailureReason, "health-gate window") {
		t.Errorf("failure_reason = %q, want a build-specific line — no health gate ran", closed.FailureReason)
	}
}

// TestBuildGateTimeoutFailsARowWhoseBuildPodVanished covers the other half of
// the same invariant: the operator DID report a running build (so the row is
// legitimately build_in_progress), and then the backing Job/Pod went away —
// deleted out of band, TTL-reaped, or never rescheduled — leaving the App CR
// asserting PhaseBuilding with nothing underneath it. The row must still reach
// a terminal state on the build gate.
func TestBuildGateTimeoutFailsARowWhoseBuildPodVanished(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	ten, _ := st.CreateTenant(ctx, "acme", "free")
	row, _ := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Repo: "https://github.com/bex-co/bex.git",
		Branch: "main", Port: 80, Replicas: 1, Tier: "free",
	})
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	app := getApp(t, cl)
	app.Status = buildingApp(app.Generation, appv1alpha1.ReasonBuilding, "Building image from https://github.com/bex-co/bex.git").Status
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	open := onlyDeploy(t, st, row.ID)
	if open.Status != DeployBuildInProgress {
		t.Fatalf("deploy status = %q, want %q", open.Status, DeployBuildInProgress)
	}

	// The pod is gone; the CR still says Building and nothing ever contradicts it.
	backdateDeploy(t, st, open.ID, 40*time.Minute)
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	closed := onlyDeploy(t, st, row.ID)
	if closed.Status != DeployBuildFailed {
		t.Fatalf("deploy status after the gate timeout = %q, want %q", closed.Status, DeployBuildFailed)
	}
	if !strings.Contains(closed.FailureReason, "build window") {
		t.Errorf("failure_reason = %q, want the build budget named", closed.FailureReason)
	}
}

// TestHealthyBuildIsNotFailedAtTheGateBoundary is the control case, checked as
// hard as the failing one: nothing added here may cut short a build that is
// genuinely running. A build one second short of the boundary keeps going, and
// crossing a legal phase edge restarts the clock rather than inheriting the
// elapsed build time (the 2026-08-11 misattribution).
func TestHealthyBuildIsNotFailedAtTheGateBoundary(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	ten, _ := st.CreateTenant(ctx, "acme", "free")
	row, _ := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Repo: "https://github.com/bex-co/bex.git",
		Branch: "main", Port: 80, Replicas: 1, Tier: "free",
	})
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	app := getApp(t, cl)
	app.Status = buildingApp(app.Generation, appv1alpha1.ReasonBuilding, "Building image").Status
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// One second inside the build budget: still building.
	backdateDeploy(t, st, onlyDeploy(t, st, row.ID).ID, defaultBuildGateTimeout-time.Second)
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := onlyDeploy(t, st, row.ID).Status; got != DeployBuildInProgress {
		t.Fatalf("deploy status just inside the build gate = %q, want %q", got, DeployBuildInProgress)
	}

	// The build finishes and the rollout starts. The rollout gets its own budget
	// from this transition — a 34-minute build must not spend the rollout's.
	app = getApp(t, cl)
	app.Status.Phase = appv1alpha1.PhaseDeploying
	app.Status.Conditions[0].Reason = "Deploying"
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := onlyDeploy(t, st, row.ID).Status; got != DeployUpdateInProgress {
		t.Fatalf("deploy status = %q, want %q", got, DeployUpdateInProgress)
	}

	// And it goes live, with no timeout having fired anywhere along the way.
	app = getApp(t, cl)
	app.Status.Phase = appv1alpha1.PhaseRunning
	app.Status.ObservedGeneration = app.Generation
	app.Status.ActiveRevision = "rev-1"
	app.Status.Image = "zot.invalid/web:gen-1"
	app.Status.Conditions[0].Status = metav1.ConditionTrue
	app.Status.Conditions[0].Reason = "Deployed"
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	live := onlyDeploy(t, st, row.ID)
	if live.Status != DeployLive {
		t.Fatalf("deploy status = %q, want %q", live.Status, DeployLive)
	}
	if live.FailureReason != "" {
		t.Errorf("a successful deploy carries failure_reason %q", live.FailureReason)
	}
}

// TestDeployIsSettledWhenTheAppCRCannotBeObserved closes the structural hole
// behind "forever" as opposed to "35 minutes": every path to a terminal deploy
// status runs inside recordDeploy, which a reconcile pass reaches only after the
// App CR is found, owned and converged. A pass that skips those steps does not
// merely fail to advance the row — it never evaluates the gate timeout at all,
// so a permanently skipped App has a permanently open deploy.
//
// Both cases here are skips that do NOT clear on their own. (The third skip, an
// App CR missing from the List, self-heals: the same pass recreates it and the
// next one observes the recreated CR, which the timeout tail then closes.)
func TestDeployIsSettledWhenTheAppCRCannotBeObserved(t *testing.T) {
	for _, tc := range []struct {
		name   string
		break_ func(t *testing.T, cl *interceptClient, app *appv1alpha1.App)
	}{{
		name: "App CR belongs to another control plane",
		break_: func(t *testing.T, cl *interceptClient, app *appv1alpha1.App) {
			app.Labels[ControlPlaneLabel] = "some-other-control-plane"
			if err := cl.Update(context.Background(), app); err != nil {
				t.Fatal(err)
			}
		},
	}, {
		name: "the App CR's spec update keeps failing",
		break_: func(t *testing.T, cl *interceptClient, app *appv1alpha1.App) {
			// Drive a spec change every pass, and reject every Update.
			app.Spec.Port = 9999
			if err := cl.Client.Update(context.Background(), app); err != nil {
				t.Fatal(err)
			}
			cl.updateErr = errors.New("admission webhook rejected the update")
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			rec, st, cl := newInterceptingReconciler(t)
			ten, _ := st.CreateTenant(ctx, "acme", "free")
			row, _ := st.CreateApp(ctx, App{
				TenantID: ten.ID, Name: "web", Repo: "https://github.com/bex-co/bex.git",
				Branch: "main", Port: 80, Replicas: 1, Tier: "free",
			})
			if err := rec.ReconcileOnce(ctx); err != nil {
				t.Fatalf("reconcile: %v", err)
			}
			app := getApp(t, cl)
			app.Status = buildingApp(app.Generation, appv1alpha1.ReasonBuilding, "Building image").Status
			if err := cl.Status().Update(ctx, app); err != nil {
				t.Fatal(err)
			}
			if err := rec.ReconcileOnce(ctx); err != nil {
				t.Fatalf("reconcile: %v", err)
			}
			open := onlyDeploy(t, st, row.ID)
			if open.Status != DeployBuildInProgress {
				t.Fatalf("deploy status = %q, want %q", open.Status, DeployBuildInProgress)
			}

			tc.break_(t, cl, getApp(t, cl))

			// Still inside the build budget: nothing is closed on a transient.
			_ = rec.ReconcileOnce(ctx)
			if got := onlyDeploy(t, st, row.ID).Status; got != DeployBuildInProgress {
				t.Fatalf("deploy closed while still inside its budget => %q; a brief skip must cost nothing", got)
			}

			notifier := newFakeDeployNotifier()
			rec.DeployNotifier = notifier
			backdateDeploy(t, st, open.ID, 40*time.Minute)
			_ = rec.ReconcileOnce(ctx)
			closed := onlyDeploy(t, st, row.ID)
			if closed.Status != DeployBuildFailed {
				t.Fatalf("deploy status = %q, want %q — an App the pass cannot observe still owes its row a terminal state", closed.Status, DeployBuildFailed)
			}
			if closed.FinishedAt == nil {
				t.Error("a terminal deploy must carry finished_at")
			}
			if !strings.Contains(closed.FailureReason, "could not be read") {
				t.Errorf("failure_reason = %q, want it to say the App record could not be read rather than imply an observed failure", closed.FailureReason)
			}
			// A failure nobody is told about is barely better than a row that
			// never closes: this close notifies on the same terms as every other.
			notifier.awaitCall(t)
			sent := notifier.snapshot()
			if len(sent) != 1 || sent[0].DeployID != closed.ID || sent[0].Status != DeployBuildFailed {
				t.Fatalf("notifications = %+v, want one build_failed for %s", sent, closed.ID)
			}
			if sent[0].FailureReason != closed.FailureReason {
				t.Errorf("notified reason %q, want the row's own %q", sent[0].FailureReason, closed.FailureReason)
			}
		})
	}
}

// interceptClient fails Update on demand so a permanently un-convergeable App CR
// can be tested without a real admission webhook.
type interceptClient struct {
	client.Client
	updateErr error
}

func (c *interceptClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if c.updateErr != nil {
		if _, ok := obj.(*appv1alpha1.App); ok {
			return c.updateErr
		}
	}
	return c.Client.Update(ctx, obj, opts...)
}

func newInterceptingReconciler(t *testing.T) (*Reconciler, *memStore, *interceptClient) {
	t.Helper()
	rec, st, cl := newTestReconciler(t)
	wrapped := &interceptClient{Client: cl}
	rec.Client = wrapped
	return rec, st, wrapped
}

func onlyDeploy(t *testing.T, st *memStore, appID string) Deploy {
	t.Helper()
	ds, err := st.ListDeploys(context.Background(), appID, DeployFilter{})
	if err != nil {
		t.Fatalf("list deploys: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("deploys = %+v, want exactly one", ds)
	}
	return ds[0]
}

// backdateDeploy moves a row's clock back so a gate timeout can be exercised
// without a test that sleeps for 35 minutes. deployTimedOut measures from
// updated_at (created_at only when updated_at is unset), so both move.
func backdateDeploy(t *testing.T, st *memStore, id string, by time.Duration) {
	t.Helper()
	st.mu.Lock()
	defer st.mu.Unlock()
	d, ok := st.deploys[id]
	if !ok {
		t.Fatalf("deploy %s not found", id)
	}
	d.CreatedAt = d.CreatedAt.Add(-by)
	d.UpdatedAt = d.UpdatedAt.Add(-by)
	if d.StartedAt != nil {
		started := d.StartedAt.Add(-by)
		d.StartedAt = &started
	}
	st.deploys[id] = d
}
