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
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestImageRefusedBeforeDispatchKeepsThePriorReleaseServing is w4/533: a
// deploy of a missing tag over a serving image service failed before any pod
// was dispatched, yet the service read Failed and bex-api recorded a phantom
// server_failed. Over a served release the phase and Ready keep describing the
// release that serves, and the Rollout verdict carries the failure to the
// deploy row; with nothing served it stays an ordinary failure.
func TestImageRefusedBeforeDispatchKeepsThePriorReleaseServing(t *testing.T) {
	ctx := context.Background()
	refused := errors.New("image metadata request returned status 404")
	setup := func(t *testing.T, served bool) (*AppReconciler, *appv1alpha1.App) {
		t.Helper()
		one := int32(1)
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", Generation: 2,
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "2"}},
			Spec:   appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Image: "docker.io/traefik/whoami:qa-no-such-tag", Replicas: 1},
			Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, ReleaseGeneration: 2},
		}
		if served {
			app.Status.ActiveRevision = "rev-1"
		}
		dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a"}, Spec: appsv1.DeploymentSpec{Replicas: &one}}
		scheme := wakeScheme()
		cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(app).WithObjects(app, dep).Build()
		return wakeReconciler(cl, scheme), app
	}

	t.Run("over a served release", func(t *testing.T) {
		r, app := setup(t, true)
		if _, err := r.failBeforeDispatch(ctx, app, "ImageMetadataUnavailable", refused, "the latest image could not be read"); !errors.Is(err, refused) {
			t.Fatalf("err = %v, want the refusal for a retry", err)
		}
		got := &appv1alpha1.App{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(app), got); err != nil {
			t.Fatal(err)
		}
		ready := meta.FindStatusCondition(got.Status.Conditions, appv1alpha1.ConditionReady)
		rollout := meta.FindStatusCondition(got.Status.Conditions, appv1alpha1.ConditionRollout)
		if got.Status.Phase != appv1alpha1.PhaseRunning || ready == nil || ready.Status != metav1.ConditionTrue ||
			ready.Reason != reasonPriorReleaseServing {
			t.Fatalf("phase %q ready %+v, want Running and the prior release serving", got.Status.Phase, ready)
		}
		if rollout == nil || rollout.Status != metav1.ConditionFalse || rollout.ObservedGeneration != 2 || rollout.Message != refused.Error() {
			t.Fatalf("rollout verdict = %+v, want the refusal for release 2", rollout)
		}
	})

	t.Run("with nothing served", func(t *testing.T) {
		r, app := setup(t, false)
		_, _ = r.failBeforeDispatch(ctx, app, "ImageMetadataUnavailable", refused, "the latest image could not be read")
		got := &appv1alpha1.App{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(app), got); err != nil {
			t.Fatal(err)
		}
		if got.Status.Phase != appv1alpha1.PhaseFailed {
			t.Fatalf("phase = %q, want Failed for a first release", got.Status.Phase)
		}
	})
}
