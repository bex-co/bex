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
//
// The datastores' maintenance Pods carry it too (w2/057): Postgres logical
// export and export-expiry Jobs (controlled by their Database), the Key Value
// backup CronJob's template (controlled by its KeyValue), and the two
// delete-time purge Jobs (`purge-db-`/`purge-kv-`, unowned by design and
// identified by their parent-UID label). Otherwise a workspace whose services
// fill the compute quota could have a backup or export refused. Each is
// stamped unconditionally: a Job is created once and never updated, and a
// CronJob template change restarts nothing. CNPG's own bootstrap Jobs
// (initdb, join, pgbasebackup, recovery) inherit the Cluster's
// spec.priorityClassName (CNPG 1.30.1 pkg/specs/jobs.go CreatePrimaryJob).

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

// carriesDatastorePriority reports whether a live CNPG Cluster already runs
// under the datastore class. One that does not adopts it only when its Pods
// roll anyway (clusterPodsRoll); an added instance counts because the new Pod
// must not be charged to the compute quota, the refusal the class exists to
// remove.
func carriesDatastorePriority(live *unstructured.Unstructured) bool {
	class, _, _ := unstructured.NestedString(live.Object, "spec", "priorityClassName")
	return class == appv1alpha1.DatastorePriorityClassName
}

// clusterPodsRoll reports whether a change to a CNPG Cluster's instance Pod
// spec costs no restart of its own: the Cluster is new, or CNPG is about to
// replace its instance Pods for another reason:
//
//   - the plan's resources change (CNPG rolls every instance);
//   - the image changes (minor update or major upgrade);
//   - an instance is added (HA standby or read replica). CNPG then rolls the
//     existing instances once, primary last;
//   - a restart is requested (the Render restart verb). CNPG 1.30.1 deletes
//     and recreates the Pod for it, never restarts in place, so the new spec
//     lands with that one restart;
//   - the Cluster is, or is leaving, hibernation: it has no Pods to roll.
func clusterPodsRoll(live *unstructured.Unstructured, desired map[string]any, suspended bool, restartedAt string) bool {
	if live.GetResourceVersion() == "" || suspended {
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

// poolerTemplate is the CNPG Pooler pod-template override: the datastore
// class and the PgBouncer container's resources. CNPG merges the container
// into its own by name, so nothing else about the Pod changes.
//
// The resources are sized for Render's 30,000 clients (w6/m458), measured on
// PgBouncer 1.25.2 with TLS clients: 23 MiB of cgroup memory idle, 143 MiB
// with 3,000 clients connected (about 42 KB each), so about 1.2 GiB at 30,000.
// The limit leaves room above that; the request holds about 5,000 clients and
// is what a quiet pooler reserves on its node. PgBouncer is single-threaded,
// so it never uses more than one of its two cores; the container's file limit
// (524,288 on the tenant nodes) already exceeds the clients.
func poolerTemplate() map[string]any {
	return map[string]any{"spec": map[string]any{
		"containers": []any{map[string]any{
			"name": "pgbouncer",
			"resources": map[string]any{
				"requests": map[string]any{string(corev1.ResourceCPU): "100m", string(corev1.ResourceMemory): "256Mi"},
				"limits":   map[string]any{string(corev1.ResourceCPU): "2", string(corev1.ResourceMemory): "2Gi"},
			},
		}},
		"priorityClassName": appv1alpha1.DatastorePriorityClassName,
	}}
}

// poolerAdoptsDatastorePriority reports whether a Pooler's desired spec should
// carry the template: when it is new, already carries the class, or its
// Cluster carries it. PgBouncer's settings follow the plan by reload, never
// through the pod template, so the Cluster adopting the
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
