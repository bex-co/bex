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
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestKeyValueInternalProxyCutoverPreservesServerAndRouting(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-test", Namespace: "tea-test", UID: "test"}, Spec: appv1alpha1.KeyValueSpec{PersistenceMode: "off"}}
	server := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}, Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "valkey", Image: "existing", Args: []string{"--save", "", "--appendonly", "no"}}}}}}}
	original := server.DeepCopy()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(server).WithStatusSubresource(&appsv1.Deployment{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, InternalProxyImage: "bex@sha256:fixture"}
	intent := keyValueIntent{authSecretName: "auth", credentialRevision: "revision", tlsListener: true, public: true, labels: map[string]string{labelKeyValue: kv.Name}}
	ready, err := r.reconcileKeyValueInternalProxy(ctx, kv, intent)
	if err != nil || ready {
		t.Fatalf("new front ready=%v err=%v", ready, err)
	}
	if err := r.reconcileKeyValueService(ctx, kv, intent); err != nil {
		t.Fatal(err)
	}
	service := &corev1.Service{}
	key := client.ObjectKeyFromObject(kv)
	if err := cl.Get(ctx, key, service); err != nil {
		t.Fatal(err)
	}
	if service.Spec.Selector[labelKeyValue] != kv.Name {
		t.Fatal("cut over before proxy readiness")
	}
	backend := &corev1.Service{}
	if err := cl.Get(ctx, client.ObjectKey{Name: keyValueInternalBackendName(kv), Namespace: kv.Namespace}, backend); err != nil {
		t.Fatal(err)
	}
	if backend.Spec.Selector[labelKeyValue] != kv.Name || backend.Spec.Selector[labelKeyValueInternalProxy] != "" {
		t.Fatalf("backend can route to proxy: %v", backend.Spec.Selector)
	}
	dep := &appsv1.Deployment{}
	depKey := client.ObjectKey{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}
	if err := cl.Get(ctx, depKey, dep); err != nil {
		t.Fatal(err)
	}
	if dep.Spec.Template.Spec.Containers[0].ReadinessProbe.Exec.Command[1] != "--check" {
		t.Fatal("front cutover must use authenticated readiness")
	}
	dep.Status = appsv1.DeploymentStatus{ObservedGeneration: dep.Generation, Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
	if err := cl.Status().Update(ctx, dep); err != nil {
		t.Fatal(err)
	}
	ready, err = r.reconcileKeyValueInternalProxy(ctx, kv, intent)
	if err != nil || !ready {
		t.Fatalf("ready front=%v err=%v", ready, err)
	}
	intent.internalProxy = ready
	if err := r.reconcileKeyValueService(ctx, kv, intent); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, key, service); err != nil {
		t.Fatal(err)
	}
	if service.Spec.Selector[labelKeyValueInternalProxy] != kv.Name || len(service.Spec.Ports) != 2 {
		t.Fatalf("front selector/ports=%v/%v", service.Spec.Selector, service.Spec.Ports)
	}
	// Even if status lags or the front is rolling, routing must never oscillate
	// back to the credential-only backend after a cutover.
	intent.internalProxy = false
	if err := r.reconcileKeyValueService(ctx, kv, intent); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, key, service); err != nil {
		t.Fatal(err)
	}
	if service.Spec.Selector[labelKeyValueInternalProxy] != kv.Name {
		t.Fatal("cutover reverted while readiness lagged")
	}
	if err := cl.Get(ctx, key, server); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(server.Spec, original.Spec) {
		t.Fatal("migration changed Off server template and would discard data")
	}
}

func TestKeyValueInternalProxyRejectsMixedRolloutReadiness(t *testing.T) {
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Generation: 2}, Spec: appsv1.DeploymentSpec{Replicas: new(int32(1))}, Status: appsv1.DeploymentStatus{ObservedGeneration: 2, Replicas: 2, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}}
	if keyValueInternalDeploymentReady(dep) {
		t.Fatal("old available replica hid unready replacement")
	}
	dep.Status.Replicas = 1
	if !keyValueInternalDeploymentReady(dep) {
		t.Fatal("fully ready current generation was refused")
	}
	dep.Status.ObservedGeneration = 1
	if keyValueInternalDeploymentReady(dep) {
		t.Fatal("stale rollout acknowledged")
	}
}
