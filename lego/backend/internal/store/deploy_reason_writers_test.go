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
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/591: every failure and cancel reason bex-api writes itself parses back to
// its template and values, so the dashboard can word it in its own language,
// while every API keeps the English it has always returned.
func TestBexAPIDeployReasonsParseAsTheirTemplates(t *testing.T) {
	ctx := context.Background()
	rec, st, _ := newTestReconciler(t)
	ten, _ := st.CreateTenant(ctx, "acme", "free")
	row, _ := st.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "nginx", Port: 80, Replicas: 1, Tier: "free"})
	newer, err := st.CreateDeploy(ctx, row.ID, TriggerAPI, "nginx", 4, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	open := Deploy{AppID: row.ID, Generation: 3}
	canceledAlong := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: "4"},
	}}
	canceledUnknown := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: "9"},
	}}
	queued := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
		Status: appv1alpha1.AppStatus{Conditions: []metav1.Condition{{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: appv1alpha1.ReasonBuildQueued,
			Message: "waiting for a build slot: 4 of 4 builds are running", ObservedGeneration: 2,
		}}},
	}
	neverStarted, _, _ := currentFailureReason(queued)
	deploy := func(id string) []appv1alpha1.DeployReasonParam {
		return []appv1alpha1.DeployReasonParam{{Name: "deploy", Value: id}}
	}

	for _, tc := range []struct {
		got, want, code string
		params          []appv1alpha1.DeployReasonParam
	}{
		{neverStarted, "the build never started: waiting for a build slot: 4 of 4 builds are running", appv1alpha1.DeployReasonBuildNeverStarted,
			[]appv1alpha1.DeployReasonParam{{Name: "cause", Value: "waiting for a build slot: 4 of 4 builds are running"}}},
		{abandonedDeployReason(DeployBuildFailed), "the build did not finish within the build window, and this service's App record could not be read to explain why",
			appv1alpha1.DeployReasonBuildAbandoned, nil},
		{abandonedDeployReason(DeployUpdateFailed), "the deploy did not finish within its window, and this service's App record could not be read to explain why",
			appv1alpha1.DeployReasonDeployAbandoned, nil},
		{suspendCancelReason, "Canceled: the service was suspended", appv1alpha1.DeployReasonCanceledSuspended, nil},
		{rec.supersededCancelReason(ctx, open, nil), "Superseded by a newer release", appv1alpha1.DeployReasonSupersededByNewer, nil},
		{supersededByReason("dep-abc123"), "Superseded by dep-abc123", appv1alpha1.DeployReasonSupersededBy, deploy("dep-abc123")},
		{restartCanceledReason("dep-abc123"), "Canceled by restart dep-abc123, which kept the running release: what this deploy was applying was not deployed",
			appv1alpha1.DeployReasonCanceledByRestart, deploy("dep-abc123")},
		{rec.canceledAlongReason(ctx, open, canceledAlong), "Canceled along with " + newer.ID, appv1alpha1.DeployReasonCanceledAlongWith, deploy(newer.ID)},
		{rec.canceledAlongReason(ctx, open, canceledUnknown), "Canceled along with a newer deploy", appv1alpha1.DeployReasonCanceledAlongNewer, nil},
	} {
		if tc.got != tc.want {
			t.Errorf("reason = %q, want %q", tc.got, tc.want)
		}
		code, params, ok := appv1alpha1.ParseDeployReason(tc.got)
		if !ok || code != tc.code || !slices.Equal(params, tc.params) {
			t.Errorf("%q parses as %s %v (ok=%v), want %s %v", tc.got, code, params, ok, tc.code, tc.params)
		}
	}
}
