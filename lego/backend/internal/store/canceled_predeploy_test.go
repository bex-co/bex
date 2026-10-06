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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// canceledStepFixture opens generation 7's deploy with its pre-deploy step
// running (or with none when !stepRan), then cancels it as deploys.Service.Cancel
// does: the release is stamped canceled and the row closes canceled. The App is
// what the operator then reports — the served release 6 restored, and release
// 7's step as pd.
func canceledStepFixture(t *testing.T, stepRan bool, pd *appv1alpha1.PreDeployStatus) (*Reconciler, *memStore, DesiredApp, Deploy, *appv1alpha1.App) {
	t.Helper()
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	tenant, err := st.CreateTenant(ctx, "canceled-step", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	row, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Image: "docker.io/traefik/whoami:latest", Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	open, err := st.CreateDeploy(ctx, row.ID, "deploy_hook", row.Image, 7, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.TransitionDeploy(ctx, open.ID, DeployPreDeployInProgress, "", "", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if stepRan {
		if _, err := st.SetDeployPreDeployStatus(ctx, open.ID, PreDeployRunning); err != nil {
			t.Fatal(err)
		}
	}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-canceled-step", Generation: 7}}
	if err := cl.Create(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := CancelRelease(ctx, cl, app, 7, ""); err != nil {
		t.Fatal(err)
	}
	if won, err := st.CloseDeploy(ctx, open.ID, DeployCanceled, ""); err != nil || !won {
		t.Fatalf("cancel = (%v, %v)", won, err)
	}
	app.Status = appv1alpha1.AppStatus{ReleaseGeneration: 6, ActiveRevision: "rev-6", PreDeploy: pd}
	return rec, st, DesiredApp{App: row}, open, app
}

// w5/085: canceling a release leaves its running pre-deploy step to finish. The
// close settles the step "canceled"; once the operator records how the step
// ended, the closed row reads that verdict, and the step's pre_deploy_ended
// fact, which the close could not record, joins the feed.
func TestCancelDuringARunningPreDeployRecordsHowTheStepEnded(t *testing.T) {
	ctx := context.Background()
	step := &appv1alpha1.PreDeployStatus{Job: "predeploy-web-gen-7", Generation: 7, Status: appv1alpha1.PreDeployRunning,
		StartedAt: "2026-10-06T09:00:00Z"}
	rec, st, d, open, app := canceledStepFixture(t, true, step)
	endedKey := preDeployEndedFact(open, time.Time{}, "").SourceKey

	rec.recordObservations(ctx, d, app, nil)
	if got := reloadDeploy(t, st, d.ID, open.ID); got.Status != DeployCanceled || got.PreDeployStatus != PreDeployCanceled {
		t.Fatalf("while the step runs = %s, step %q; want canceled, step canceled", got.Status, got.PreDeployStatus)
	}
	if _, ok := st.eventFacts[endedKey]; ok {
		t.Fatal("pre_deploy_ended recorded before the step ended")
	}

	step.Status, step.FinishedAt = appv1alpha1.PreDeploySucceeded, "2026-10-06T09:04:00Z"
	rec.recordObservations(ctx, d, app, nil)
	settled := reloadDeploy(t, st, d.ID, open.ID)
	if settled.Status != DeployCanceled || settled.PreDeployStatus != PreDeploySucceeded {
		t.Fatalf("after the step succeeded = %s, step %q; want canceled, step succeeded", settled.Status, settled.PreDeployStatus)
	}
	fact, ok := st.eventFacts[endedKey]
	if want := time.Date(2026, 10, 6, 9, 4, 0, 0, time.UTC); !ok || fact.Status != EventStatusSucceeded || !fact.At.Equal(want) || fact.DeployID != open.ID {
		t.Fatalf("pre_deploy_ended = %+v (recorded %v), want succeeded at %s", fact, ok, want)
	}

	rec.recordObservations(ctx, d, app, nil)
	if again := reloadDeploy(t, st, d.ID, open.ID); !again.UpdatedAt.Equal(settled.UpdatedAt) {
		t.Fatalf("a repeat pass rewrote the settled row: updated_at %s → %s", settled.UpdatedAt, again.UpdatedAt)
	}
}

// w5/085: the verdict is projected only for the release the cancel stamp
// names, only once the step has ended, and only onto a row whose step was
// running when it closed.
func TestCanceledStepVerdictIsScopedToTheCanceledRelease(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name     string
		stepRan  bool
		stamp    bool
		pd       appv1alpha1.PreDeployStatus
		wantStep string
	}{
		{"the step fails after the cancel", true, true,
			appv1alpha1.PreDeployStatus{Generation: 7, Status: appv1alpha1.PreDeployFailed}, PreDeployFailed},
		{"a newer release's verdict", true, true,
			appv1alpha1.PreDeployStatus{Generation: 8, Status: appv1alpha1.PreDeploySucceeded}, PreDeployCanceled},
		{"no cancel stamp", true, false,
			appv1alpha1.PreDeployStatus{Generation: 7, Status: appv1alpha1.PreDeploySucceeded}, PreDeployCanceled},
		{"a row whose step never ran", false, true,
			appv1alpha1.PreDeployStatus{Generation: 7, Status: appv1alpha1.PreDeploySucceeded}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pd := tc.pd
			pd.Job = "predeploy-web-gen-7"
			rec, st, d, open, app := canceledStepFixture(t, tc.stepRan, &pd)
			if !tc.stamp {
				delete(app.Annotations, appv1alpha1.AnnotationCanceledReleaseGeneration)
			}
			rec.recordObservations(ctx, d, app, nil)
			if got := reloadDeploy(t, st, d.ID, open.ID); got.Status != DeployCanceled || got.PreDeployStatus != tc.wantStep {
				t.Fatalf("closed row = %s, step %q; want canceled, step %q", got.Status, got.PreDeployStatus, tc.wantStep)
			}
			_, ended := st.eventFacts[preDeployEndedFact(open, time.Time{}, "").SourceKey]
			if wantEnded := tc.wantStep == PreDeployFailed; ended != wantEnded {
				t.Fatalf("pre_deploy_ended recorded = %v, want %v", ended, wantEnded)
			}
		})
	}
}
