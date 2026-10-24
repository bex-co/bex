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
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A KeyValue in an isolated environment (labelNetworkIsolation on its CR, w8/m136)
// must leave the namespace's allow-same-namespace policy, which excludes every
// pod carrying that label. The label is a pod-template label for a newborn
// store, but an existing store must never restart because an environment's
// isolation flipped (a Valkey restart is downtime), so for it the template only
// adopts the desired value when the template changes for another reason (the
// lazy-adoption rule applyDeploymentSpec follows), and the running pod is
// relabelled in place by syncKeyValuePodIsolation.

// keyValueJobPeer selects the store's own backup and purge Jobs, which carry no
// isolation label and must still reach it.
func keyValueJobPeer(kv *appv1alpha1.KeyValue) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{labelKeyValue: kv.Name},
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      "app.bex.co/component",
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{keyValueBackupComponent, keyValueBackupPurgeComponent},
		}},
	}
}

// holdIsolationLabel sets the isolation label on sts's pod template to want,
// unless sts already existed, its template is otherwise unchanged from before,
// and the label differs: then the existing template value stays so the pods are
// not rolled.
func holdIsolationLabel(sts *appsv1.StatefulSet, before *corev1.PodTemplateSpec, want string) {
	labels := maps.Clone(sts.Spec.Template.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	set := func(v string) {
		if v == "" {
			delete(labels, labelNetworkIsolation)
		} else {
			labels[labelNetworkIsolation] = v
		}
		sts.Spec.Template.Labels = labels
	}
	set(want)
	if sts.CreationTimestamp.IsZero() || before.Labels[labelNetworkIsolation] == want {
		return
	}
	if templateEqualIgnoringIsolation(before, &sts.Spec.Template) {
		set(before.Labels[labelNetworkIsolation])
	}
}

func templateEqualIgnoringIsolation(a, b *corev1.PodTemplateSpec) bool {
	strip := func(t *corev1.PodTemplateSpec) *corev1.PodTemplateSpec {
		c := t.DeepCopy()
		delete(c.Labels, labelNetworkIsolation)
		return c
	}
	return equality.Semantic.DeepEqual(strip(a), strip(b))
}

// syncKeyValuePodIsolation relabels the store's running StatefulSet pods to the
// isolation the CR carries. Patching metadata does not restart a pod.
func (r *KeyValueReconciler) syncKeyValuePodIsolation(ctx context.Context, kv *appv1alpha1.KeyValue) error {
	want := kv.Labels[labelNetworkIsolation]
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(kv.Namespace), client.MatchingLabels{labelKeyValue: kv.Name}); err != nil {
		return err
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		owner := metav1.GetControllerOf(pod)
		if owner == nil || owner.Kind != "StatefulSet" || owner.Name != kv.Name || !pod.DeletionTimestamp.IsZero() {
			continue
		}
		if pod.Labels[labelNetworkIsolation] == want {
			continue
		}
		patch := client.MergeFrom(pod.DeepCopy())
		if want == "" {
			delete(pod.Labels, labelNetworkIsolation)
		} else {
			pod.Labels[labelNetworkIsolation] = want
		}
		if err := client.IgnoreNotFound(r.Patch(ctx, pod, patch)); err != nil {
			return err
		}
	}
	return nil
}
