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

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestFailMarkerSurvivesStatusWriteConflict pins w2/m82 t007: the durable
// Failed marker (phase + Ready condition) must LAND even when a concurrent
// writer — the bex-api control-plane projector rewrites App CRs on resync —
// bumps the resourceVersion between the operator's read and its status write.
// Before the fix, setNotReadyCondition did `_ = sw.Update(...)`, the 409 was
// swallowed, and every gate keyed on the marker (buildOutcomeAlreadyRecorded,
// terminalBuildFailureRecorded) silently never engaged: prod showed 8
// consecutive lost retries after one r.fail (2026-08-20).
func TestFailMarkerSurvivesStatusWriteConflict(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(appv1alpha1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a", UID: "uid-web", Generation: 3},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).WithInterceptorFuncs(interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, cl client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			if subResourceName != "status" {
				return cl.SubResource(subResourceName).Update(ctx, obj, opts...)
			}
			// First attempt: simulate a lost race by rejecting the stale object.
			live := obj.DeepCopyObject().(client.Object)
			if err := cl.Get(ctx, client.ObjectKeyFromObject(obj), live); err != nil {
				return err
			}
			if live.GetResourceVersion() != obj.GetResourceVersion() {
				return apierrors.NewConflict(schema.GroupResource{Group: appv1alpha1.SchemeGroupVersion.Group, Resource: "apps"}, obj.GetName(), errors.New("stale write"))
			}
			return cl.SubResource("status").Update(ctx, obj, opts...)
		},
	}).Build()

	// Simulate the projector racing: bump the stored object's resourceVersion
	// with a spec annotation AFTER the reconciler fetched `app` (it was fetched
	// into the fake's store above), so the first status write is stale.
	racing := &appv1alpha1.App{}
	g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(app), racing)).To(Succeed())
	racing.Annotations = map[string]string{"projector-resync": "yes"}
	g.Expect(cl.Update(ctx, racing)).To(Succeed())

	r := &AppReconciler{Client: cl, Scheme: scheme}
	_, _ = r.fail(ctx, app, appv1alpha1.ReasonBuildFailedUserError, errors.New("build failed: exit 90"))

	stored := &appv1alpha1.App{}
	g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(app), stored)).To(Succeed())
	g.Expect(stored.Status.Phase).To(Equal(appv1alpha1.PhaseFailed),
		"a conflicted status write must not lose the Failed phase")
	cond := findReadyCondition(stored)
	g.Expect(cond).NotTo(BeNil(), "the Ready condition must land")
	g.Expect(cond.Reason).To(Equal(appv1alpha1.ReasonBuildFailedUserError))
	g.Expect(cond.ObservedGeneration).To(Equal(int64(3)),
		"the marker must carry the generation the gates key on")
	g.Expect(stored.Status.ObservedGeneration).To(BeZero(),
		"fail's contract: ObservedGeneration stays unstamped on failure")
	g.Expect(stored.Annotations).To(HaveKey("projector-resync"),
		"the retry must not clobber the projector's concurrent spec write")
}

func findReadyCondition(a *appv1alpha1.App) *metav1.Condition {
	for i := range a.Status.Conditions {
		if a.Status.Conditions[i].Type == appv1alpha1.ConditionReady {
			return &a.Status.Conditions[i]
		}
	}
	return nil
}
