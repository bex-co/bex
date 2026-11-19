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
	"strconv"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/044: bex-api's container policy migration saves render-v1 on a service
// that runs nothing for its next deploy, the way Render's Update Service saves
// a setting: the pending-source marker and no release stamp. A first release
// whose rollout failed and stopped, or that was canceled, stays stopped in its
// own phase through that write, and the next deploy runs under the policy.
func TestSavedPolicyKeepsAStoppedFirstReleaseStopped(t *testing.T) {
	for _, tc := range []struct {
		name  string
		phase appv1alpha1.AppPhase
		stop  func(*testing.T, *AppReconciler, client.Client, types.NamespacedName)
	}{
		{"failed rollout", appv1alpha1.PhaseFailed, stopFailedFirstRelease},
		{"canceled", appv1alpha1.PhaseCanceled, func(t *testing.T, r *AppReconciler, cl client.Client, nn types.NamespacedName) {
			reconcileTwice(t, r, nn)
			updateApp(t, cl, nn, func(a *appv1alpha1.App) {
				a.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = "1"
			})
			reconcileTwice(t, r, nn)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := firstReleaseApp(appv1alpha1.TypeWebService, "starter")
			// bex-api's create stamps the first release, as on production.
			app.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "1"
			r, cl, nn := lifecycleFixture(t, app)
			tc.stop(t, r, cl, nn)
			setDeploymentStatus(t, cl, nn, statusDrained)
			reconcileTwice(t, r, nn)
			stopped := func(when string) {
				t.Helper()
				if got := deploymentReplicas(t, cl, nn); got != 0 {
					t.Fatalf("%s: replicas = %d, want 0", when, got)
				}
				if live := liveApp(t, cl, nn); live.Status.Phase != tc.phase || live.Status.ActiveRevision != "" {
					t.Fatalf("%s: phase %q activeRevision %q, want %q with nothing served", when, live.Status.Phase, live.Status.ActiveRevision, tc.phase)
				}
			}
			stopped("setup")
			template := deploymentTemplate(t, cl, nn)

			updateApp(t, cl, nn, func(a *appv1alpha1.App) {
				a.Generation++
				a.Spec.ContainerPolicy = appv1alpha1.ContainerPolicyRenderV1
				a.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] = strconv.FormatInt(a.Generation, 10)
			})
			for range 3 {
				reconcileOnce(t, r, nn)
			}
			stopped("after the saved policy")
			if got := deploymentTemplate(t, cl, nn); !equality.Semantic.DeepEqual(template, got) {
				t.Fatalf("the saved policy changed the stopped release's pod template:\nbefore %+v\nafter  %+v", template, got)
			}

			// The next deploy runs the service, under the saved policy.
			deployImageAt(t, cl, nn, "nginx:2", liveApp(t, cl, nn).Generation+1)
			dep := liveDeployment(t, cl, nn)
			dep.Generation = dep.Status.ObservedGeneration + 1
			if err := cl.Update(context.Background(), &dep); err != nil {
				t.Fatal(err)
			}
			reconcileOnce(t, r, nn)
			if got := deploymentReplicas(t, cl, nn); got != 1 {
				t.Fatalf("replicas = %d after the next deploy, want 1", got)
			}
			if got := deploymentTemplate(t, cl, nn).Labels[execution.LabelContainerPolicy]; got != appv1alpha1.ContainerPolicyRenderV1 {
				t.Fatalf("the next deploy's pod template runs container policy %q, want render-v1", got)
			}
		})
	}
}
