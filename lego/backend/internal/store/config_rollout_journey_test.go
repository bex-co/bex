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
	"strconv"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestConfigChangeFailsTerminallyThenSelfHeals replays w6/m51's original live
// journey end to end against the reconciler, now that a Settings edit opens its
// own deploy row (qa-20260823-restart, 2026-08-23):
//
//  1. Edit the Start Command to something the app rejects. The service
//     genuinely rebuilds and redeploys — and the rollout never converges.
//  2. Edit it back to a valid value.
//
// Before this milestone step 1 recorded nothing at all: the Deploys tab stayed
// empty, `service.phase` read Building with no row to inspect, retry, or cancel,
// and step 2 produced no visible progress either — only an explicit Manual
// Deploy did. The two guarantees asserted here are the fix: the failed rollout
// reaches a TERMINAL status inside the deploy gate rather than hanging open, and
// the correction alone opens its own row that reaches live, with no manual
// deploy or restart in between.
func TestConfigChangeFailsTerminallyThenSelfHeals(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	// Any elapsed time trips the gates — deterministic without sleeping.
	rec.DeployGateTimeout, rec.BuildGateTimeout, rec.PreDeployGateTimeout = 0, 0, 0
	ten, _ := st.CreateTenant(ctx, "acme", "free")
	row, _ := st.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Repo: "https://github.com/bex-co/hello.git", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	// The service is created and its first deploy goes live. (The fake client
	// does not maintain metadata.generation, so the create's own release
	// generation is set here the way a real API server would.)
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	created := getApp(t, cl)
	created.Generation = FirstDeployGeneration
	if err := cl.Update(ctx, created); err != nil {
		t.Fatal(err)
	}
	setObservedRelease(t, cl, appv1alpha1.PhaseRunning, FirstDeployGeneration)
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Step 1 — the broken Start Command. bex-api patches the spec and, since the
	// patch moves release identity, opens a deploy row for the rollout it forces.
	broken := patchStartCommand(t, cl, "./app --qa-simple-test")
	brokenDeploy, err := st.CreateDeploy(ctx, row.ID, TriggerConfigChange, "", broken, CommitInfo{})
	if err != nil {
		t.Fatalf("open deploy for the broken edit: %v", err)
	}

	// The rollout never becomes healthy: the pods keep failing readiness, so the
	// App sits in Deploying for the whole gate window.
	setObservedRelease(t, cl, appv1alpha1.PhaseDeploying, broken)
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	got := deployByID(t, ctx, st, row.ID, brokenDeploy.ID)
	if !IsTerminalDeployStatus(got.Status) {
		t.Fatalf("broken rollout status = %q, want a TERMINAL status — the original bug was a rollout that never settled anywhere a user could see", got.Status)
	}
	if got.Status != DeployUpdateFailed {
		t.Fatalf("broken rollout status = %q, want update_failed", got.Status)
	}
	if got.FailureReason == "" {
		t.Error("a failed config rollout must explain itself, not close blank")
	}

	// Step 2 — the correction, and nothing else. No Manual Deploy, no Restart.
	fixed := patchStartCommand(t, cl, "./app")
	fixedDeploy, err := st.CreateDeploy(ctx, row.ID, TriggerConfigChange, "", fixed, CommitInfo{})
	if err != nil {
		t.Fatalf("open deploy for the corrected edit: %v", err)
	}
	if fixedDeploy.Status == DeployCanceled {
		t.Fatal("the correction must open a real deploy, not be swallowed as superseded")
	}

	setObservedRelease(t, cl, appv1alpha1.PhaseRunning, fixed)
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if got := deployByID(t, ctx, st, row.ID, fixedDeploy.ID); got.Status != DeployLive {
		t.Fatalf("corrected rollout status = %q, want live with no manual deploy in between", got.Status)
	}
}

// patchStartCommand applies the spec edit and the release-generation stamp
// bex-api's rollout tracker writes, returning the release generation the deploy
// row is filed under.
func patchStartCommand(t *testing.T, cl client.Client, command string) int64 {
	t.Helper()
	app := getApp(t, cl)
	generation := app.Generation + 1
	app.Spec.StartCommand = command
	app.Generation = generation
	if app.Annotations == nil {
		app.Annotations = map[string]string{}
	}
	app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = strconv.FormatInt(generation, 10)
	if err := cl.Update(context.Background(), app); err != nil {
		t.Fatalf("patch start command: %v", err)
	}
	return generation
}

// setObservedRelease reports the operator's view of the release the CR now asks
// for: the phase it reached, under that release's generation, with the Ready
// condition the phase mapping reads as current evidence.
func setObservedRelease(t *testing.T, cl client.Client, phase appv1alpha1.AppPhase, generation int64) {
	t.Helper()
	app := getApp(t, cl)
	app.Status.Phase = phase
	app.Status.ReleaseGeneration = generation
	app.Status.ObservedGeneration = generation
	reason, status := "Deploying", metav1.ConditionFalse
	if phase == appv1alpha1.PhaseRunning {
		app.Status.ActiveRevision = "rev-" + strconv.FormatInt(generation, 10)
		reason, status = "Running", metav1.ConditionTrue
	}
	app.Status.Conditions = []metav1.Condition{{
		Type: appv1alpha1.ConditionReady, Status: status, Reason: reason,
		ObservedGeneration: app.Generation, LastTransitionTime: metav1.Now(),
	}}
	if err := cl.Status().Update(context.Background(), app); err != nil {
		t.Fatalf("update status: %v", err)
	}
}

func deployByID(t *testing.T, ctx context.Context, st *memStore, appID, deployID string) Deploy {
	t.Helper()
	deploys, err := st.ListDeploys(ctx, appID, DeployFilter{})
	if err != nil {
		t.Fatalf("list deploys: %v", err)
	}
	for _, d := range deploys {
		if d.ID == deployID {
			return d
		}
	}
	t.Fatalf("deploy %s not found in %+v", deployID, deploys)
	return Deploy{}
}
