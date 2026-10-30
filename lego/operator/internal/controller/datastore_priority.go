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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Datastore Pods run under appv1alpha1.DatastorePriorityClassName so the
// workspace's compute ResourceQuota, scoped `PriorityClass NotIn` that class,
// does not charge them (w2/m247). A Pod's class is immutable, so a running
// datastore only gains it when its Pods are recreated. Stamping the class on
// every existing store at once would restart every tenant database and Key
// Value in the fleet on the operator rollout that ships it, a single-instance
// outage each. Instead an existing workload ADOPTS the class only when its Pods
// are about to be replaced anyway; until then it is charged exactly as before,
// which is no regression. New workloads always carry it.

// holdDatastorePriority stamps the datastore class on a Valkey-family pod
// template, unless the workload already exists without it and the class is the
// only change that would roll its Pod. The isolation label is ignored in that
// comparison because it is relabelled in place and never rolls a Pod
// (holdIsolationLabel), so an isolation toggle cannot smuggle a restart in.
func holdDatastorePriority(tmpl *corev1.PodTemplateSpec, existed bool, before *corev1.PodTemplateSpec) {
	tmpl.Spec.PriorityClassName = appv1alpha1.DatastorePriorityClassName
	if !existed || before.Spec.PriorityClassName == tmpl.Spec.PriorityClassName {
		return
	}
	held := tmpl.DeepCopy()
	held.Spec.PriorityClassName = before.Spec.PriorityClassName
	if templateEqualIgnoringIsolation(before, held) {
		tmpl.Spec.PriorityClassName = before.Spec.PriorityClassName
	}
}

// clusterAdoptsDatastorePriority reports whether a CNPG Cluster's desired spec
// should carry the datastore class. It does when the Cluster is new or already
// carries it, and otherwise only when CNPG is about to replace its instance
// Pods for another reason:
//
//   - the plan's resources change (CNPG rolls every instance);
//   - the image changes (minor update or major upgrade);
//   - an instance is added (HA standby or read replica). The new Pod must not
//     be charged to the compute quota: that refusal is what this class exists
//     to remove. CNPG then rolls the existing instances once, primary last;
//   - a restart is requested (the Render restart verb);
//   - the Cluster is, or is leaving, hibernation: it has no Pods to roll.
func clusterAdoptsDatastorePriority(live *unstructured.Unstructured, desired map[string]any, suspended bool, restartedAt string) bool {
	if live.GetResourceVersion() == "" || suspended {
		return true
	}
	if class, _, _ := unstructured.NestedString(live.Object, "spec", "priorityClassName"); class == appv1alpha1.DatastorePriorityClassName {
		return true
	}
	annotations := live.GetAnnotations()
	if annotations[hibernationAnnotation] == "on" || (restartedAt != "" && restartedAt != annotations[restartAnnotation]) {
		return true
	}
	liveImage, _, _ := unstructured.NestedString(live.Object, "spec", "imageName")
	if image, _ := desired["imageName"].(string); image != "" && image != liveImage {
		return true
	}
	liveInstances, _, _ := unstructured.NestedInt64(live.Object, "spec", "instances")
	if instances, _ := desired["instances"].(int64); instances > liveInstances {
		return true
	}
	liveResources, _, _ := unstructured.NestedMap(live.Object, "spec", "resources")
	desiredResources, _ := desired["resources"].(map[string]any)
	return !sameResourceQuantities(liveResources, desiredResources)
}

// sameResourceQuantities compares two CNPG resources stanzas by quantity, so
// "1" and "1000m" are the same plan and only a real resize counts as a roll.
func sameResourceQuantities(a, b map[string]any) bool {
	for _, kind := range []string{"requests", "limits"} {
		for _, name := range []string{"cpu", "memory"} {
			x, _, _ := unstructured.NestedFieldNoCopy(a, kind, name)
			y, _, _ := unstructured.NestedFieldNoCopy(b, kind, name)
			if !sameQuantity(x, y) {
				return false
			}
		}
	}
	return true
}

func sameQuantity(x, y any) bool {
	if x == nil || y == nil {
		return x == nil && y == nil
	}
	qx, errX := resource.ParseQuantity(fmt.Sprint(x))
	qy, errY := resource.ParseQuantity(fmt.Sprint(y))
	if errX != nil || errY != nil {
		return fmt.Sprint(x) == fmt.Sprint(y)
	}
	return qx.Cmp(qy) == 0
}

// poolerTemplate is the CNPG Pooler pod-template override carrying only the
// datastore class. CNPG merges it with its own PgBouncer container by name,
// so the empty container list (required by the CRD schema) changes nothing
// else about the Pod.
func poolerTemplate() map[string]any {
	return map[string]any{"spec": map[string]any{
		"containers":        []any{},
		"priorityClassName": appv1alpha1.DatastorePriorityClassName,
	}}
}

// poolerAdoptsDatastorePriority reports whether a Pooler's desired spec should
// carry the class: when it is new, already does, or its Cluster carries it. A
// Pooler's own spec never changes after create, so the Cluster adopting the
// class (a resize, upgrade, restart or added instance, which already
// reconnects every client) is the event an existing PgBouncer rolls with.
func poolerAdoptsDatastorePriority(live, cluster *unstructured.Unstructured) bool {
	if live.GetResourceVersion() == "" {
		return true
	}
	own, _, _ := unstructured.NestedString(live.Object, "spec", "template", "spec", "priorityClassName")
	clusters, _, _ := unstructured.NestedString(cluster.Object, "spec", "priorityClassName")
	return own == appv1alpha1.DatastorePriorityClassName || clusters == appv1alpha1.DatastorePriorityClassName
}
