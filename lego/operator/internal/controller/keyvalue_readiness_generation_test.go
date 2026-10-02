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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestKeyValueReadinessDoesNotAcknowledgeCachedOldGeneration(t *testing.T) {
	for _, observedGeneration := range []int64{1, 2} {
		scheme := runtime.NewScheme()
		if err := appv1alpha1.AddToScheme(scheme); err != nil {
			t.Fatal(err)
		}
		if err := appsv1.AddToScheme(scheme); err != nil {
			t.Fatal(err)
		}
		if err := corev1.AddToScheme(scheme); err != nil {
			t.Fatal(err)
		}
		kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default", Generation: 2}}
		observed := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace, Generation: observedGeneration},
			Status:     appsv1.StatefulSetStatus{ObservedGeneration: observedGeneration, CurrentRevision: "ready", UpdateRevision: "ready", CurrentReplicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1},
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "cache-0", Namespace: kv.Namespace, Labels: map[string]string{labelKeyValue: kv.Name, appsv1.StatefulSetRevisionLabel: "ready"}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
		}
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv, observed, pod).WithStatusSubresource(kv).Build()
		r := &KeyValueReconciler{Client: cl, Scheme: scheme}
		written := observed.DeepCopy()
		written.Generation = 2
		if _, err := r.updateKeyValueReadiness(t.Context(), kv, written, 1, "credential"); err != nil {
			t.Fatal(err)
		}
		if err := cl.Get(t.Context(), client.ObjectKeyFromObject(kv), kv); err != nil {
			t.Fatal(err)
		}
		want := appv1alpha1.KVPhaseProvisioning
		if observedGeneration == 2 {
			want = appv1alpha1.KVPhaseReady
		}
		if kv.Status.Phase != want {
			t.Fatalf("observed generation %d: phase %s, want %s", observedGeneration, kv.Status.Phase, want)
		}
	}
}
