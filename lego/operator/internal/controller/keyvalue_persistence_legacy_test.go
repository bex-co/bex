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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPersistenceLegacyPendingTemplateUsesServingSnapshot(t *testing.T) {
	for _, target := range []string{"snapshot", "journal-snapshot"} {
		t.Run(target, func(t *testing.T) {
			ctx := context.Background()
			kv, sts, p, c := persistenceLiveFixture(t)
			kv.Spec.PersistenceMode = target
			sts.Spec.Template.Annotations = nil
			sts.Spec.Template.Spec.Containers[0].Args = []string{"--appendonly", "yes"}
			sts.Status.UpdateRevision = "journal-requested"
			sts.Status.CurrentRevision = "snapshot-revision"
			// The old controller already authored journal flags, but the owned serving
			// process still runs Snapshot. Neither its template nor its revision is authority.
			r := &KeyValueReconciler{Client: c}
			intent := legacyPersistenceTestIntent(kv)
			prepared := false
			_, stop, err := r.prepareKeyValuePersistenceWith(ctx, kv, sts, &corev1.Secret{}, &intent, func(_ context.Context, actual *corev1.Pod, _ *corev1.Secret) (bool, error) {
				prepared = true
				if actual.UID != p.UID || !reflect.DeepEqual(actual.Spec.Containers, p.Spec.Containers) {
					t.Fatal("prepared a different process")
				}
				return true, nil
			})
			if err != nil || stop {
				t.Fatalf("stop=%v err=%v", stop, err)
			}
			if intent.persistenceSource != "snapshot" {
				t.Fatalf("bootstrap source=%q, want actual snapshot", intent.persistenceSource)
			}
			if prepared != (target == "journal-snapshot") {
				t.Fatalf("live handoff called=%v target=%s", prepared, target)
			}
			plan, storage := resolveKVPlan(kv.Spec)
			desired := keyValueIntentFor(kv, plan, storage, "")
			desired.persistenceSource = intent.persistenceSource
			desired.persistenceToken = intent.persistenceToken
			applyKeyValueStatefulSet(sts, kv, desired)
			if sts.Spec.Template.Annotations[keyValuePersistenceSourceAnnotation] != "snapshot" {
				t.Fatal("template lost actual bootstrap source")
			}
		})
	}
}

func TestPersistenceLegacyUnknownSourceDefersMutation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*corev1.Pod)
		missing bool
	}{
		{name: "missing serving pod", missing: true},
		{name: "foreign serving pod", mutate: func(p *corev1.Pod) { p.OwnerReferences[0].UID = "foreign-owner" }},
		{name: "unready serving pod", mutate: func(p *corev1.Pod) { p.Status.Conditions[0].Status = corev1.ConditionFalse }},
		{name: "unknown serving container", mutate: func(p *corev1.Pod) { p.Spec.Containers = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			kv, sts, p, c := persistenceLiveFixture(t)
			sts.Spec.Template.Annotations = nil
			if tc.missing {
				if err := c.Delete(ctx, p); err != nil {
					t.Fatal(err)
				}
			} else {
				tc.mutate(p)
				if err := c.Update(ctx, p); err != nil {
					t.Fatal(err)
				}
				tc.mutate(p)
				if err := c.Status().Update(ctx, p); err != nil {
					t.Fatal(err)
				}
			}
			before := sts.DeepCopy()
			r := &KeyValueReconciler{Client: c}
			intent := legacyPersistenceTestIntent(kv)
			result, stop, err := r.prepareKeyValuePersistenceWith(ctx, kv, sts, &corev1.Secret{}, &intent, func(context.Context, *corev1.Pod, *corev1.Secret) (bool, error) {
				t.Fatal("unknown source reached journal preparation")
				return false, nil
			})
			if err != nil || !stop || result.RequeueAfter == 0 {
				t.Fatalf("result=%+v stop=%v err=%v", result, stop, err)
			}
			if !reflect.DeepEqual(sts.Spec, before.Spec) || intent.persistenceSource != "" {
				t.Fatal("unknown source mutated or fabricated persistence authority")
			}
		})
	}
}

func TestPersistenceUnchangedLegacyStoreDoesNotRoll(t *testing.T) {
	for _, mode := range []string{"journal-snapshot", "snapshot", "off"} {
		t.Run(mode, func(t *testing.T) {
			kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "fixture", Namespace: "test"}, Spec: appv1alpha1.KeyValueSpec{PersistenceMode: mode}}
			plan, storage := resolveKVPlan(kv.Spec)
			intent := keyValueIntentFor(kv, plan, storage, "")
			intent.authSecretName = "fixture-auth"
			intent.skipPersistenceInit = true // Build the real pre-migration shape, including Off without --dir.
			sts := &appsv1.StatefulSet{}
			applyKeyValueStatefulSet(sts, kv, intent)
			sts.UID = "existing-sts"
			sts.CreationTimestamp = metav1.Now()
			delete(sts.Spec.Template.Annotations, keyValuePersistenceSourceAnnotation)
			delete(sts.Spec.Template.Annotations, keyValuePersistenceTokenAnnotation)
			sts.Spec.Template.Spec.InitContainers = nil
			sts.Generation = 1
			sts.Status = appsv1.StatefulSetStatus{ObservedGeneration: 1, CurrentRevision: "current", UpdateRevision: "current", CurrentReplicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
			before := sts.Spec.Template.DeepCopy()
			intent.persistenceSource = mode
			intent.skipPersistenceInit = legacyKeyValueWorkloadUnchanged(sts, kv, intent)
			if !intent.skipPersistenceInit {
				t.Fatal("unchanged legacy workload was not detected")
			}
			applyKeyValueStatefulSet(sts, kv, intent)
			if !reflect.DeepEqual(&sts.Spec.Template, before) {
				t.Fatal("operator upgrade changed unchanged legacy pod template")
			}
			// A later real policy change installs the migration in that same rollout.
			kv.Spec.MaxmemoryPolicy = "noeviction"
			intent.skipPersistenceInit = legacyKeyValueWorkloadUnchanged(sts, kv, intent)
			applyKeyValueStatefulSet(sts, kv, intent)
			if len(sts.Spec.Template.Spec.InitContainers) != 1 || sts.Spec.Template.Annotations[keyValuePersistenceSourceAnnotation] != mode {
				t.Fatal("real workload change did not install handoff with frozen authority")
			}
		})
	}
}

func TestPersistenceNewStorageBootstrapDoesNotTrustRetainedPVC(t *testing.T) {
	for _, retained := range []bool{false, true} {
		t.Run(map[bool]string{false: "new volume", true: "retained volume"}[retained], func(t *testing.T) {
			ctx := context.Background()
			kv, existing, p, c := persistenceLiveFixture(t)
			if err := c.Delete(ctx, existing); err != nil {
				t.Fatal(err)
			}
			if err := c.Delete(ctx, p); err != nil {
				t.Fatal(err)
			}
			if retained {
				if err := c.Create(ctx, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: keyValuePVCName(kv.Name), Namespace: kv.Namespace}}); err != nil {
					t.Fatal(err)
				}
			}
			sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}}
			r := &KeyValueReconciler{Client: c}
			intent := legacyPersistenceTestIntent(kv)
			_, stop, err := r.prepareKeyValuePersistenceWith(ctx, kv, sts, &corev1.Secret{}, &intent, func(context.Context, *corev1.Pod, *corev1.Secret) (bool, error) {
				t.Fatal("new STS reached live engine")
				return false, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if retained {
				if !stop || intent.persistenceSource != "" {
					t.Fatal("retained unknown volume treated as new")
				}
			} else if stop || intent.persistenceSource != "new:journal-snapshot" {
				t.Fatalf("new storage: stop=%v source=%q", stop, intent.persistenceSource)
			}
		})
	}
}

func TestPersistencePendingLegacyRolloutCannotSkipBootstrap(t *testing.T) {
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "fixture", Namespace: "test"}, Spec: appv1alpha1.KeyValueSpec{PersistenceMode: "journal-snapshot"}}
	plan, storage := resolveKVPlan(kv.Spec)
	intent := keyValueIntentFor(kv, plan, storage, "")
	sts := &appsv1.StatefulSet{}
	applyKeyValueStatefulSet(sts, kv, intent)
	sts.UID = "existing-sts"
	sts.CreationTimestamp = metav1.Now()
	sts.Generation = 2
	sts.Spec.Template.Spec.InitContainers = nil
	delete(sts.Spec.Template.Annotations, keyValuePersistenceSourceAnnotation)
	sts.Status = appsv1.StatefulSetStatus{ObservedGeneration: 2, CurrentRevision: "serving-snapshot", UpdateRevision: "requested-journal"}
	if legacyKeyValueWorkloadUnchanged(sts, kv, intent) {
		t.Fatal("pending journal template bypassed serving-source bootstrap")
	}
	sts.Status.CurrentRevision = sts.Status.UpdateRevision
	sts.Status.ObservedGeneration = 1
	if legacyKeyValueWorkloadUnchanged(sts, kv, intent) {
		t.Fatal("unobserved template bypassed serving-source bootstrap")
	}
}

func legacyPersistenceTestIntent(kv *appv1alpha1.KeyValue) keyValueIntent {
	plan, storage := resolveKVPlan(kv.Spec)
	return keyValueIntentFor(kv, plan, storage, "")
}

func TestPersistenceLegacySuspendedWithoutPodRequiresKnownSource(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged remains suspended", true: "mode change cannot infer source"}[changed], func(t *testing.T) {
			ctx := context.Background()
			kv, _, _, c := persistenceLiveFixture(t)
			kv.Spec.PersistenceMode = "snapshot"
			kv.Spec.Suspended = true
			if err := c.Update(ctx, kv); err != nil {
				t.Fatal(err)
			}
			plan, storage := resolveKVPlan(kv.Spec)
			intent := keyValueIntentFor(kv, plan, storage, "")
			intent.skipPersistenceInit = true
			sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}}
			applyKeyValueStatefulSet(sts, kv, intent)
			sts.UID = "suspended-legacy"
			sts.CreationTimestamp = metav1.Now()
			sts.Generation = 2
			sts.Status = appsv1.StatefulSetStatus{ObservedGeneration: 2, CurrentRevision: "converged", UpdateRevision: "converged"}
			// The fake fixture pod must be absent. Zero replicas make revision convergence
			// possible even when no process has ever served those desired startup flags.
			existing := &corev1.Pod{}
			if err := c.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: kv.Name + "-0"}, existing); err != nil {
				t.Fatal(err)
			}
			if err := c.Delete(ctx, existing); err != nil {
				t.Fatal(err)
			}
			if changed {
				kv.Spec.PersistenceMode = "journal-snapshot"
				kv.Generation++
				if err := c.Update(ctx, kv); err != nil {
					t.Fatal(err)
				}
			}
			intent = keyValueIntentFor(kv, plan, storage, "")
			before := sts.DeepCopy()
			r := &KeyValueReconciler{Client: c}
			_, stop, err := r.prepareKeyValuePersistenceWith(ctx, kv, sts, &corev1.Secret{}, &intent, func(context.Context, *corev1.Pod, *corev1.Secret) (bool, error) {
				t.Fatal("suspended no-pod store reached engine")
				return false, nil
			})
			if err != nil || stop != changed {
				t.Fatalf("changed=%v stop=%v err=%v", changed, stop, err)
			}
			if !reflect.DeepEqual(sts.Spec, before.Spec) || intent.persistenceSource != "" {
				t.Fatal("inferred persistence source from zero replicas")
			}
			if changed {
				condition := meta.FindStatusCondition(kv.Status.Conditions, appv1alpha1.ConditionReady)
				if condition == nil || condition.Reason != "PersistenceSourceUnknown" || condition.Status != metav1.ConditionFalse {
					t.Fatalf("unknown source diagnostic=%+v", condition)
				}
			} else if !intent.skipPersistenceInit {
				t.Fatal("unchanged suspended legacy store was scheduled for migration")
			}
		})
	}
}
