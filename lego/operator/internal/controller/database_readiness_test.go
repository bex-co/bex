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
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/298: the readiness timeout is in every instance Pod's spec, so an
// existing Cluster takes it only when its Pods roll anyway; the strategy
// changes in place at once.
func TestReconcileClusterHoldsTheReadinessTimeoutUntilTheClusterRolls(t *testing.T) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-a", Namespace: "tea-a", UID: "db-uid"},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "free"},
	}
	cl, reconcile := clusterReconcileHarness(t, db)
	plan, gb := resolvePlan(db.Spec)
	params := clusterParams{plan: plan, storageGB: gb, namespace: db.Namespace}
	readiness := func(u *unstructured.Unstructured) map[string]any {
		got, _, _ := unstructured.NestedMap(u.Object, "spec", "probes", "readiness")
		return got
	}

	if got := readiness(reconcile(params)); got["type"] != cnpgProbeQuery || got["timeoutSeconds"] != singleInstanceReadinessTimeoutSeconds {
		t.Fatalf("a new Cluster's readiness = %v", got)
	}

	// A Cluster created before w8/298: neither its spec nor its projection
	// has probes.
	live := reconcile(params)
	unstructured.RemoveNestedField(live.Object, "spec", "probes")
	var projected map[string]any
	if err := json.Unmarshal([]byte(live.GetAnnotations()[annotationProjectedSpec]), &projected); err != nil {
		t.Fatal(err)
	}
	delete(projected, "probes")
	encoded, err := json.Marshal(projected)
	if err != nil {
		t.Fatal(err)
	}
	annotations := live.GetAnnotations()
	annotations[annotationProjectedSpec] = string(encoded)
	live.SetAnnotations(annotations)
	if err := cl.Update(context.Background(), live); err != nil {
		t.Fatal(err)
	}
	got := readiness(reconcile(params))
	if _, has := got["timeoutSeconds"]; has {
		t.Fatalf("an unchanged Cluster was given the timeout (restarting its instance): %v", got)
	}
	if got["type"] != cnpgProbeQuery {
		t.Errorf("the strategy, which restarts nothing, was held: %v", got)
	}

	// A restart rolls the instance anyway; the timeout rides along and stays.
	db.Spec.RestartedAt = "2026-11-12T23:00:00Z"
	for range 2 {
		if got := readiness(reconcile(params)); got["timeoutSeconds"] != singleInstanceReadinessTimeoutSeconds {
			t.Fatalf("a restarting Cluster's readiness = %v", got)
		}
	}

	// A standby rolls the instances too, and the Cluster returns to CNPG's
	// defaults.
	params.highAvailability = true
	if got := readiness(reconcile(params)); got != nil {
		t.Errorf("a Cluster gaining a standby keeps readiness %v, want CNPG's defaults", got)
	}
}
