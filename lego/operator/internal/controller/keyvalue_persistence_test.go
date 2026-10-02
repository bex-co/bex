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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestKeyValuePersistencePendingReversalRetainsLegacySource(t *testing.T) {
	for _, source := range []string{"", "journal-snapshot", "snapshot", "off"} {
		t.Run(source, func(t *testing.T) {
			plan, _ := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "free"})
			kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "persistence", Namespace: "default", Generation: 1}, Spec: appv1alpha1.KeyValueSpec{Plan: "free", PersistenceMode: source}}
			// The reconciler freezes a source established from the owned serving pod.
			sts := &appsv1.StatefulSet{}
			sts.Spec.Template.Annotations = map[string]string{keyValuePersistenceSourceAnnotation: normalizedKeyValuePersistence(source)}
			sts.Spec.Template.Spec.Containers = []corev1.Container{{Name: "valkey", Args: valkeyArgs(kv.Spec, plan)}}
			for _, desired := range []string{"snapshot", "journal-snapshot", "off", source} {
				kv.Generation++
				kv.Spec.PersistenceMode = desired
				// A suspended edit must retain the handoff so resume cannot bypass it.
				kv.Spec.Suspended = desired == "journal-snapshot"
				intent := keyValueIntentFor(kv, plan, 1, "")
				applyKeyValueStatefulSet(sts, kv, intent)
				if got := sts.Spec.Template.Annotations[keyValuePersistenceSourceAnnotation]; got != normalizedKeyValuePersistence(source) {
					t.Fatalf("source changed across pending reversal: %q", got)
				}
				if *sts.Spec.Replicas != intent.replicas {
					t.Fatal("suspension replicas changed")
				}
				if len(sts.Spec.Template.Spec.InitContainers) != 1 {
					t.Fatal("handoff missing")
				}
				init := sts.Spec.Template.Spec.InitContainers[0]
				env := map[string]string{}
				for _, variable := range init.Env {
					env[variable.Name] = variable.Value
					if variable.ValueFrom != nil {
						t.Fatal("offline converter must not mount credentials")
					}
				}
				if env["SOURCE_MODE"] != normalizedKeyValuePersistence(source) || env["TARGET_MODE"] != normalizedKeyValuePersistence(desired) {
					t.Fatalf("handoff modes=%v", env)
				}
				if init.Image != sts.Spec.Template.Spec.Containers[0].Image || init.SecurityContext.RunAsUser == nil || *init.SecurityContext.RunAsUser != valkeyRunAsUser {
					t.Fatal("handoff must use runtime engine and nonroot identity")
				}
			}
		})
	}
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

func TestKeyValuePersistenceBootstrapUsesOwnedRunningPod(t *testing.T) {
	for _, tc := range []struct {
		name      string
		pod       bool
		foreign   bool
		pending   bool
		retained  bool
		existing  bool
		want      string
		wantError bool
	}{
		{name: "new storage", want: "new:journal-snapshot"},
		{name: "retained storage", retained: true, want: "unknown"},
		{name: "existing workload without pod", existing: true, want: "unknown"},
		{name: "actual snapshot pod overrides journal template", pod: true, existing: true, want: "snapshot"},
		{name: "foreign pod cannot establish source", pod: true, foreign: true, existing: true, want: "unknown"},
		{name: "pending rollout cannot establish source", pod: true, pending: true, existing: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = appv1alpha1.AddToScheme(scheme)
			kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "bootstrap", Namespace: "default"}, Spec: appv1alpha1.KeyValueSpec{PersistenceMode: "journal-snapshot"}}
			sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}}
			if tc.existing {
				sts.UID = "owned-sts"
				sts.ResourceVersion = "1"
			}
			sts.Spec.Template.Spec.Containers = []corev1.Container{{Name: "valkey", Args: []string{"--appendonly", "yes"}}}
			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tc.pod {
				owner := sts.UID
				if tc.foreign {
					owner = "foreign-sts"
				}
				pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-0", Namespace: kv.Namespace, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "StatefulSet", Name: sts.Name, UID: owner, Controller: new(true)}}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "valkey", Args: []string{"--appendonly", "no"}}}}, Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "valkey", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}
				builder = builder.WithObjects(pod)
			}
			if tc.retained {
				builder = builder.WithObjects(&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: keyValuePVCName(kv.Name), Namespace: kv.Namespace}})
			}
			if tc.pending {
				sts.Status.CurrentRevision = "snapshot-revision"
				sts.Status.UpdateRevision = "journal-revision"
			}
			// A separate uncached reader proves stale informer contents cannot choose
			// the template's journal flags instead of the actual pod's snapshot flags.
			r := &KeyValueReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build(), APIReader: builder.Build()}
			got, err := r.seedKeyValuePersistence(context.Background(), kv, sts)
			if (err != nil) != tc.wantError {
				t.Fatalf("bootstrap error=%v, wantError=%v", err, tc.wantError)
			}
			if !tc.wantError && got != tc.want {
				t.Fatalf("bootstrap=%q want %q", got, tc.want)
			}
		})
	}
}

func TestKeyValuePersistenceLeavesUnchangedLegacyWorkloadAlone(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "legacy", Namespace: "default", UID: "kv-legacy"}, Spec: appv1alpha1.KeyValueSpec{PersistenceMode: "snapshot"}}
	plan, _ := resolveKVPlan(kv.Spec)
	intent := keyValueIntentFor(kv, plan, 1, "")
	sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace, UID: "legacy-sts"}}
	applyKeyValueStatefulSet(sts, kv, intent)
	delete(sts.Spec.Template.Annotations, keyValuePersistenceSourceAnnotation)
	sts.Spec.Template.Spec.InitContainers = nil
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme}
	if err := r.reconcileKeyValueWorkload(context.Background(), kv, sts, intent); err != nil {
		t.Fatal(err)
	}
	if len(sts.Spec.Template.Spec.InitContainers) != 0 {
		t.Fatal("operator upgrade restarted an unchanged legacy store")
	}
	// A real mode edit requires initialization; absent authoritative pod and
	// retained workload identity must not silently become new storage.
	kv.Spec.PersistenceMode = "journal-snapshot"
	if err := r.reconcileKeyValueWorkload(context.Background(), kv, sts, intent); err != nil {
		t.Fatal(err)
	}
	if len(sts.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatal("real mode edit bypassed initialization")
	}
	if sts.Spec.Template.Annotations[keyValuePersistenceSourceAnnotation] != "unknown" {
		t.Fatal("missing legacy source was guessed from desired mode")
	}
}
