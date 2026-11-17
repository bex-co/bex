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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/670: the operator stamps a first release Deploying a pass before it
// creates the pre-deploy Job and records the step Running. That pass read
// update_in_progress, and the lifecycle then refused pre_deploy_in_progress, so
// the row said "rolling out" through the whole migration and ran on the
// 18-minute deploy gate instead of the pre-deploy gate.

// dispatchedFirstRelease is an image web service's first release (generation
// gen) at dispatch: Deploying, no pre-deploy step recorded yet.
func dispatchedFirstRelease(gen int64) *appv1alpha1.App {
	app := &appv1alpha1.App{}
	app.Generation = gen
	app.Spec.Type = appv1alpha1.TypeWebService
	app.Spec.PreDeployCommand = "sleep 120"
	app.Status.Phase = appv1alpha1.PhaseDeploying
	app.Status.ReleaseGeneration = gen
	app.Status.Conditions = []metav1.Condition{{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: "Deploying", ObservedGeneration: gen,
	}}
	return app
}

func TestFirstReleaseReadsPreDeployInProgressFromDispatch(t *testing.T) {
	const gen = 1
	for _, from := range []string{DeployCreated, DeployQueued, DeployBuildInProgress, DeployPreDeployInProgress} {
		open := Deploy{Generation: gen, Status: from}
		got := observedDeployStatus(open, dispatchedFirstRelease(gen), false)
		if got != DeployPreDeployInProgress {
			t.Errorf("from %s: status = %q, want %q before the pre-deploy Job exists", from, got, DeployPreDeployInProgress)
		}
		if from != got && !CanTransitionDeploy(from, got) {
			t.Errorf("from %s: the lifecycle refuses %q", from, got)
		}
		if got := observedDeployStatus(open, dispatchedFirstRelease(gen), true); got != DeployPreDeployFailed {
			t.Errorf("from %s, timed out: status = %q, want %q", from, got, DeployPreDeployFailed)
		}
	}
}

func TestAwaitingPreDeployCases(t *testing.T) {
	const gen = 2
	cases := []struct {
		name   string
		status string
		edit   func(*appv1alpha1.App)
		want   string
	}{
		{"a stale step from an earlier release does not count", DeployCreated, func(a *appv1alpha1.App) {
			a.Status.PreDeploy = &appv1alpha1.PreDeployStatus{Job: "pre-web-gen-1", Generation: gen - 1, Status: appv1alpha1.PreDeploySucceeded}
		}, DeployPreDeployInProgress},
		{"the step passed: the rollout has begun", DeployPreDeployInProgress, func(a *appv1alpha1.App) {
			a.Status.PreDeploy = &appv1alpha1.PreDeployStatus{Job: "pre-web-gen-2", Generation: gen, Status: appv1alpha1.PreDeploySucceeded}
		}, DeployUpdateInProgress},
		{"no pre-deploy command goes straight to the rollout", DeployCreated, func(a *appv1alpha1.App) {
			a.Spec.PreDeployCommand = "  "
		}, DeployUpdateInProgress},
		// w33/m435: the hook runs after the release is live, as Render runs
		// it, so a release that owes only a hook awaits no pre-deploy step.
		{"an owed initial deploy hook is no pre-deploy step", DeployCreated, func(a *appv1alpha1.App) {
			a.Spec.PreDeployCommand = ""
			a.Spec.InitialDeployHook = "./seed"
		}, DeployUpdateInProgress},
		{"beside a hook, the first release still awaits its own pre-deploy command", DeployCreated, func(a *appv1alpha1.App) {
			a.Spec.InitialDeployHook = "./seed"
		}, DeployPreDeployInProgress},
		{"a running hook holds no later deploy in pre-deploy", DeployCreated, func(a *appv1alpha1.App) {
			a.Spec.PreDeployCommand = ""
			a.Spec.InitialDeployHook = "./seed"
			a.Status.InitialDeployHook = &appv1alpha1.PreDeployStatus{Job: "predeploy-web-gen-1-hook", Generation: gen - 1, Status: appv1alpha1.PreDeployRunning}
		}, DeployUpdateInProgress},
		{"a background worker runs the step too", DeployCreated, func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeBackgroundWorker
		}, DeployPreDeployInProgress},
		{"a static site never runs one", DeployCreated, func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeStaticSite
		}, DeployUpdateInProgress},
		{"a row already rolling out never regresses", DeployUpdateInProgress, func(*appv1alpha1.App) {}, DeployUpdateInProgress},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := dispatchedFirstRelease(gen)
			tc.edit(app)
			if got := observedDeployStatus(Deploy{Generation: gen, Status: tc.status}, app, false); got != tc.want {
				t.Fatalf("status = %q, want %q", got, tc.want)
			}
		})
	}
}

// The row's own phase picks its timer: once it reads pre_deploy_in_progress it
// is judged by the pre-deploy gate, not the 18-minute deploy gate.
func TestFirstReleasePreDeployWindowUsesThePreDeployGate(t *testing.T) {
	rec, _, _ := newTestReconciler(t)
	app := dispatchedFirstRelease(1)
	open := Deploy{Generation: 1, Status: DeployCreated}
	open.Status = observedDeployStatus(open, app, false)
	open.UpdatedAt = time.Now().Add(-(rec.DeployGateTimeout + time.Minute))
	if rec.deployTimedOut(DesiredApp{}, open) {
		t.Fatalf("a %s row past the deploy gate (%v) timed out; the pre-deploy gate is %v",
			open.Status, rec.DeployGateTimeout, rec.PreDeployGateTimeout)
	}
}
