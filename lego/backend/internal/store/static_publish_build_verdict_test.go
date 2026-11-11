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
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w21/049: a static site's publish (clone-and-publish, or the extract from a
// built image) runs with the row in update_in_progress, and a missing publish
// directory is a user-source failure the operator records as the Build verdict
// (w4/493). Over a release that already served, that verdict leaves the phase
// Running, which the row's own mapping reads as "not live yet": the row sat in
// update_in_progress for the 18-minute health gate and closed update_failed
// with a sentence about "service logs" that a static site does not have. These
// tests pin the bex-api half the operator tests (which assert only what the
// operator records) never covered.

const missingDirectory = `the publish directory "examples/hello-node/nope" does not exist in the repository at "6eadb86"`

// republishedStaticSite is the operator's status after a replacement publish
// failed over a release that served: the phase is back to Running over the
// prior release, and the Build condition holds the verdict for the release the
// row was opened for.
func republishedStaticSite(gen int64, msg string) *appv1alpha1.App {
	app := servingAppWithFailedBuild(gen, msg)
	app.Spec.Type = appv1alpha1.TypeStaticSite
	return app
}

func TestStaticPublishBuildVerdictClosesTheRowOverAServedRelease(t *testing.T) {
	const gen = int64(3)
	app := republishedStaticSite(gen, missingDirectory)

	// Both exits that report a user-source error share the verdict and the path:
	// the clone exit (a missing directory in the repository) and the extract exit
	// (a missing directory in the built image).
	for _, msg := range []string{missingDirectory, `the publish directory "dist" does not exist in the built image`} {
		app.Status.Conditions[1].Message = msg
		open := Deploy{Generation: gen, Status: DeployUpdateInProgress}
		if got := observedDeployStatus(open, app, false); got != DeployBuildFailed {
			t.Errorf("static row update_in_progress + Build verdict (%q) => %q, want %q before the health gate", msg, got, DeployBuildFailed)
		}
		if got, _ := deployCloseFailureReason(app, open, DeployBuildFailed, true); got != msg {
			t.Errorf("close reason = %q, want the Build condition's sentence %q", got, msg)
		}
		if !CanTransitionDeploy(DeployUpdateInProgress, DeployBuildFailed) {
			t.Fatal("the row cannot move update_in_progress -> build_failed")
		}
	}
}

// The verdict is the row's own only when it is attributed to the row's release
// generation, and only a static site publishes in update_in_progress.
func TestStaticPublishBuildVerdictStaysNarrow(t *testing.T) {
	const gen = int64(3)
	open := Deploy{Generation: gen, Status: DeployUpdateInProgress}

	stale := republishedStaticSite(gen, "an earlier release's failure")
	stale.Status.Conditions[1].ObservedGeneration = gen - 1
	if got := observedDeployStatus(open, stale, false); got == DeployBuildFailed {
		t.Error("a Build verdict for an earlier release closed a newer static deploy")
	}

	web := republishedStaticSite(gen, missingDirectory)
	web.Spec.Type = appv1alpha1.TypeWebService
	if got := observedDeployStatus(open, web, false); got == DeployBuildFailed {
		t.Error("a web service's rollout row was closed build_failed from a Build verdict")
	}

	// A first publish has nothing served and takes the PhaseFailed branch: it
	// still closes build_failed, as before.
	first := republishedStaticSite(gen, missingDirectory)
	first.Status.Phase = appv1alpha1.PhaseFailed
	first.Status.ActiveRevision = ""
	first.Status.Conditions[0] = metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: appv1alpha1.ReasonBuildFailedUserError, Message: missingDirectory, ObservedGeneration: gen,
	}
	if got := observedDeployStatus(Deploy{Generation: gen, Status: DeployUpdateInProgress}, first, false); got != DeployBuildFailed {
		t.Errorf("first publish => %q, want build_failed", got)
	}

	// While no verdict exists the row keeps waiting, within the gate.
	waiting := republishedStaticSite(gen, "")
	waiting.Status.Conditions = waiting.Status.Conditions[:1]
	waiting.Status.Phase = appv1alpha1.PhaseDeploying
	if got := observedDeployStatus(open, waiting, false); got != DeployUpdateInProgress {
		t.Errorf("publishing with no verdict => %q, want update_in_progress", got)
	}
}

// A platform fault in the publish step is not a user-source verdict: it keeps
// its Rollout diagnosis and update_failed.
func TestStaticPublishPlatformFaultStaysUpdateFailed(t *testing.T) {
	const gen = int64(3)
	app := republishedStaticSite(gen, "")
	app.Status.Conditions = []metav1.Condition{
		{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: appv1alpha1.ReasonPriorReleaseServing, ObservedGeneration: gen},
		{Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: "PublishFailed", Message: "publish: upload to the store failed", ObservedGeneration: gen},
	}
	if got := observedDeployStatus(Deploy{Generation: gen, Status: DeployUpdateInProgress}, app, false); got != DeployUpdateFailed {
		t.Errorf("a platform publish fault => %q, want update_failed", got)
	}
	if strings.Contains(missingDirectory, "health-gate") {
		t.Fatal("fixture sentence must not be the generic gate reason")
	}
}

// The end to end of the regression: the operator's status goes in through the
// reconciler and the row and its build_ended event come out, for the clone exit
// and the extract exit alike, within one observation pass (no health gate).
func TestStaticPublishBuildVerdictClosesRowAndEventsThroughTheReconciler(t *testing.T) {
	const gen = int64(3)
	for name, msg := range map[string]string{
		"clone exit":   missingDirectory,
		"extract exit": `the publish directory "dist" does not exist in the built image`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			m := newMemStore()
			started := time.Now().Add(-time.Minute)
			m.deploys["dep-1"] = Deploy{
				ID: "dep-1", AppID: "srv-1", Generation: gen, Status: DeployUpdateInProgress,
				CreatedAt: started, StartedAt: &started,
			}
			r := &Reconciler{Store: m, DeployGateTimeout: 18 * time.Minute}
			app := republishedStaticSite(gen, msg)
			desired := DesiredApp{App: App{Repo: "https://github.com/bex-co/bex", Type: appv1alpha1.TypeStaticSite}}

			r.recordDeploy(ctx, desired, m.deploys["dep-1"], app)

			got := m.deploys["dep-1"]
			if got.Status != DeployBuildFailed || got.FailureReason != msg {
				t.Fatalf("row = %q / %q, want build_failed with %q", got.Status, got.FailureReason, msg)
			}
			if fact, ok := m.eventFacts["deploy:dep-1:build_ended"]; !ok || fact.Status != EventStatusFailed {
				t.Fatalf("build_ended = %+v (present %v), want failed", fact, ok)
			}
		})
	}
}
