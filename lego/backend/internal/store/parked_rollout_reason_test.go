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

// w6/m147: a free service with no traffic auto-hibernated mid-rollout. Ready
// then read AutoHibernated, the open row's stall diagnosis was cleared, and the
// deploy closed at the gate with "the deploy did not become healthy within the
// health-gate window; check the service logs" although for minutes the row had
// named the crash (exit 127) or the failing TCP probe.
func TestParkedRolloutClosesWithItsLastStallDiagnosis(t *testing.T) {
	for _, tc := range []struct {
		name, reason, message string
	}{
		{"crash loop", "CrashLoopBackOff", "the container keeps crashing on start (last exit code 127)"},
		{"tcp probe", "HealthCheckFailing", "the container is running but its readiness health check has not succeeded, " +
			"so the rollout is waiting: a TCP connect to port 3000."},
	} {
		for _, park := range []string{appv1alpha1.ReasonAutoHibernated, appv1alpha1.ReasonSuspended} {
			t.Run(tc.name+"/"+park, func(t *testing.T) {
				ctx := context.Background()
				st := newMemStore()
				tenant, err := st.CreateTenant(ctx, "parked", PlanHobby)
				if err != nil {
					t.Fatal(err)
				}
				row, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Image: "docker.io/library/app:2", Tier: "free"})
				if err != nil {
					t.Fatal(err)
				}
				open, err := st.CreateDeploy(ctx, row.ID, "api", row.Image, 7, CommitInfo{}, "")
				if err != nil {
					t.Fatal(err)
				}
				app := &appv1alpha1.App{
					ObjectMeta: metav1.ObjectMeta{Generation: 7},
					Status: appv1alpha1.AppStatus{
						Phase: appv1alpha1.PhaseDeploying, Image: row.Image, ObservedGeneration: 7,
						ReleaseGeneration: 7, ActiveRevision: "rev-6",
						Conditions: []metav1.Condition{{
							Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
							Reason: tc.reason, Message: tc.message, ObservedGeneration: 7,
						}},
					},
				}
				rec := NewReconciler(nil, st)
				reread := func() Deploy {
					t.Helper()
					d, err := st.GetDeploy(ctx, row.ID, open.ID)
					if err != nil {
						t.Fatal(err)
					}
					return d
				}
				rec.recordDeploy(ctx, DesiredApp{App: row}, reread(), app)
				if got := reread().StallReason; got != tc.message {
					t.Fatalf("open row stall reason = %q, want the diagnosis", got)
				}

				// The park overwrites Ready; no rollout verdict follows.
				app.Status.Phase = appv1alpha1.PhaseHibernated
				app.Status.Conditions[0].Reason = park
				app.Status.Conditions[0].Message = "idle ≥900s on free tier; wakes on next request"
				rec.recordDeploy(ctx, DesiredApp{App: row}, reread(), app)
				if got := reread().StallReason; got != tc.message {
					t.Fatalf("park cleared the stall reason: %q", got)
				}

				rec.DeployGateTimeout = -1
				rec.recordDeploy(ctx, DesiredApp{App: row}, reread(), app)
				got := reread()
				if got.Status != DeployUpdateFailed || got.FailureReason != tc.message {
					t.Fatalf("closed %s with %q, want update_failed with the diagnosis", got.Status, got.FailureReason)
				}
			})
		}
	}
}

// The close-time order for update_failed: the operator's durable verdict, then
// a current diagnosis, then the row's last stall diagnosis, then the generic
// line.
func TestUpdateFailedCloseReasonOrder(t *testing.T) {
	const stall = "the container keeps crashing on start (last exit code 127)"
	parked := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Generation: 7},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseHibernated, Conditions: []metav1.Condition{{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
			Reason: appv1alpha1.ReasonAutoHibernated, ObservedGeneration: 7,
		}}},
	}
	open := Deploy{Generation: 7, StallReason: stall}
	for _, matches := range []bool{true, false} {
		if got, _ := deployCloseFailureReason(parked, open, DeployUpdateFailed, matches); got != stall {
			t.Errorf("matches=%v: reason = %q, want the row's stall diagnosis", matches, got)
		}
	}
	if got, _ := deployCloseFailureReason(parked, Deploy{Generation: 7}, DeployUpdateFailed, true); got != timedOutDeployReason(DeployUpdateFailed) {
		t.Errorf("no diagnosis anywhere: reason = %q, want the generic line", got)
	}

	verdict := parked.DeepCopy()
	verdict.Status.Conditions = append(verdict.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: "HealthCheckFailing",
		Message: "probe verdict", ObservedGeneration: 7,
	})
	if got, _ := deployCloseFailureReason(verdict, open, DeployUpdateFailed, true); got != "probe verdict" {
		t.Errorf("verdict present: reason = %q, want the durable verdict", got)
	}

	current := parked.DeepCopy()
	current.Status.Conditions[0].Reason = "ImagePullBackOff"
	current.Status.Conditions[0].Message = "pull failed"
	if got, code := deployCloseFailureReason(current, open, DeployUpdateFailed, true); got != "pull failed" || code != EventReasonImagePullBackoff {
		t.Errorf("current diagnosis: reason = %q code = %q", got, code)
	}
}
