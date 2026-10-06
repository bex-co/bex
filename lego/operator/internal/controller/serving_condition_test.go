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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServingRevisionIgnoresFailingReplacement(t *testing.T) {
	for _, tc := range []struct {
		name, active string
		ready        bool
		want         metav1.ConditionStatus
	}{
		{"healthy prior release", "rev-old", true, metav1.ConditionTrue},
		{"crashed serving release", "rev-old", false, metav1.ConditionFalse},
		{"first deploy", "", true, metav1.ConditionUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Generation: 2}, Status: appv1alpha1.AppStatus{ActiveRevision: tc.active}}
			dep := progressDeadlineDep("web")
			old := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "old", Namespace: "default", Labels: map[string]string{"app": "web", labelRevision: "rev-old"}}}
			if tc.ready {
				old.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			}
			replacement := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "new", Namespace: "default", Labels: map[string]string{"app": "web", labelRevision: "rev-1"}}, Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}}}}}
			r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).WithObjects(old, replacement).Build()}
			r.observeServingRevision(context.Background(), app, dep, 1)
			c := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionServing)
			if c == nil || c.Status != tc.want || c.ObservedGeneration != 2 {
				t.Fatalf("serving = %+v, want %s", c, tc.want)
			}
			// A new observation must not retain a healthy condition after the pods vanish.
			if err := r.Delete(context.Background(), old); err != nil {
				t.Fatal(err)
			}
			r.observeServingRevision(context.Background(), app, dep, 1)
			if meta.IsStatusConditionTrue(app.Status.Conditions, appv1alpha1.ConditionServing) {
				t.Fatal("retained stale healthy signal")
			}
		})
	}
}
