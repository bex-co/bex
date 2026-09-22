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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// These are compatibility evidence for w1/m164's operator-only fix. Cancel
// closes its row synchronously; releaseGeneration=0 must not resurrect it.
// Legacy status without activeRevision keeps its existing convergence rules.
func TestPGRecordDeployReleaseGenerationCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name              string
		phase             appv1alpha1.AppPhase
		releaseGeneration int64
		cancel            bool
		want              string
	}{
		{name: "canceled never served", phase: appv1alpha1.PhaseCanceled, cancel: true, want: DeployCanceled},
		{name: "legacy running without release identity", phase: appv1alpha1.PhaseRunning, want: DeployLive},
		{name: "positive generation without active revision is not live", phase: appv1alpha1.PhaseRunning, releaseGeneration: 1, want: DeployUpdateInProgress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			st := openLifecyclePG(t)
			tenant, err := st.CreateTenant(ctx, tc.name, PlanHobby)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
			row, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Image: "img:crash", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
			if err != nil {
				t.Fatal(err)
			}
			open, ok, err := openDeployFor(ctx, st, row.ID)
			if err != nil || !ok {
				t.Fatalf("open deploy: ok=%v err=%v", ok, err)
			}
			mustTransition(t, st, open.ID, DeployUpdateInProgress, nil)
			open, err = st.GetDeploy(ctx, row.ID, open.ID)
			if err != nil {
				t.Fatal(err)
			}
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Generation: open.Generation},
				Status: appv1alpha1.AppStatus{
					Phase: tc.phase, Image: row.Image, ObservedGeneration: open.Generation,
					ReleaseGeneration: tc.releaseGeneration,
				},
			}
			if tc.cancel {
				// Mirror deploys.Service.Cancel's durable terminal write before the
				// operator settles the never-served App with releaseGeneration zero.
				won, err := st.CloseDeploy(ctx, open.ID, DeployCanceled, "")
				if err != nil || !won {
					t.Fatalf("cancel: won=%v err=%v", won, err)
				}
				if got := observedDeployStatus(open, app, false); got != "" {
					t.Fatalf("canceled status manufactures transition %q", got)
				}
			}
			rec := NewReconciler(nil, st)
			// Include a stale in-flight snapshot to exercise SQL terminal CAS even
			// after Cancel won, and the timeout path that would otherwise fail it.
			rec.recordDeploy(ctx, DesiredApp{App: row}, open, app)
			if tc.cancel {
				rec.DeployGateTimeout = -1
				rec.recordDeploy(ctx, DesiredApp{App: row}, open, app)
			}
			got, err := st.GetDeploy(ctx, row.ID, open.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tc.want {
				t.Fatalf("status=%q, want %q", got.Status, tc.want)
			}
			terminal := tc.want == DeployLive || tc.want == DeployCanceled
			if (got.FinishedAt != nil) != terminal {
				t.Fatalf("finishedAt=%v, terminal=%v", got.FinishedAt, terminal)
			}
			wantImage := ""
			if tc.want == DeployLive {
				wantImage = row.Image
			}
			if got.ResolvedImage != wantImage {
				t.Fatalf("rollback image=%q, want %q", got.ResolvedImage, wantImage)
			}
			if tc.cancel && (got.FailureReason != "" || got.CancelReason != "") {
				t.Fatalf("user cancel gained failure/supersede reason: %+v", got)
			}
		})
	}
}
