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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestPersistenceLegacyAuthoritySurvivesSuspendedEdits(t *testing.T) {
	for _, source := range []string{"", "journal-snapshot", "snapshot", "off"} {
		t.Run(source, func(t *testing.T) {
			kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "fixture", Namespace: "default"}, Spec: appv1alpha1.KeyValueSpec{PersistenceMode: source, Suspended: true}}
			plan, storage := resolveKVPlan(kv.Spec)
			sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.Now()}}
			sts.Spec.Template.Spec.Containers = []corev1.Container{{Name: "valkey", Args: valkeyArgs(kv.Spec, plan)}}
			for _, target := range []string{"off", "snapshot", "journal-snapshot", "snapshot"} {
				kv.Spec.PersistenceMode = target
				intent := keyValueIntentFor(kv, plan, storage, "")
				applyKeyValueStatefulSet(sts, kv, intent)
				if got := sts.Spec.Template.Annotations[keyValuePersistenceSourceAnnotation]; got != keyValuePersistenceMode(source) {
					t.Fatalf("after %s: source=%s want %s", target, got, keyValuePersistenceMode(source))
				}
				if *sts.Spec.Replicas != 0 {
					t.Fatal("suspended edit woke the server")
				}
				before := sts.DeepCopy()
				applyKeyValueStatefulSet(sts, kv, intent)
				if !reflect.DeepEqual(sts, before) {
					t.Fatal("redundant projection mutated the workload")
				}
			}
		})
	}
}

func TestPersistenceTemplateRetainsConsumedToken(t *testing.T) {
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "fixture", Namespace: "default"}, Spec: appv1alpha1.KeyValueSpec{PersistenceMode: "journal-snapshot"}}
	plan, storage := resolveKVPlan(kv.Spec)
	sts := &appsv1.StatefulSet{}
	intent := keyValueIntentFor(kv, plan, storage, "")
	intent.persistenceToken = "process-generation"
	applyKeyValueStatefulSet(sts, kv, intent)
	intent.persistenceToken = ""
	kv.Spec.PersistenceMode = "snapshot"
	applyKeyValueStatefulSet(sts, kv, intent)
	if sts.Spec.Template.Annotations[keyValuePersistenceTokenAnnotation] != "process-generation" {
		t.Fatal("lost consumed handoff token")
	}
	for _, env := range sts.Spec.Template.Spec.InitContainers[0].Env {
		if env.Name == "PERSISTENCE_TOKEN" && env.Value == "process-generation" {
			return
		}
	}
	t.Fatal("initializer lost consumed token across reversal")
}

func TestKeyValuePersistenceDirectCRWriteCannotPublishReadyBeforeHandoff(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "handoff", Namespace: "default", Generation: 1}, Spec: appv1alpha1.KeyValueSpec{Plan: "free", PersistenceMode: "snapshot"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).WithStatusSubresource(&appv1alpha1.KeyValue{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme}
	ctx := context.Background()
	key := types.NamespacedName{Name: kv.Name, Namespace: kv.Namespace}
	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key}); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, key, kv); err != nil {
		t.Fatal(err)
	}
	kv.Spec.PersistenceMode = "journal-snapshot"
	kv.Generation++
	if err := cl.Update(ctx, kv); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key}); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, key, kv); err != nil {
		t.Fatal(err)
	}
	if kv.Status.Phase == appv1alpha1.KVPhaseReady {
		t.Fatal("reported Ready before handoff pod started")
	}
	sts := &appsv1.StatefulSet{}
	if err := cl.Get(ctx, key, sts); err != nil {
		t.Fatal(err)
	}
	if len(sts.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatal("direct writer bypassed persistence gate")
	}
	if got := sts.Spec.Template.Annotations[keyValuePersistenceSourceAnnotation]; got != "new:snapshot" {
		t.Fatalf("lost source mode: %s", got)
	}
}
