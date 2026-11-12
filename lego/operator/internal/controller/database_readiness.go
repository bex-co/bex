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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// cnpgProbeQuery is CNPG's readiness strategy that connects from the
	// instance manager, bounded only by the kubelet's probe timeout.
	cnpgProbeQuery = "query"
	// singleInstanceReadinessTimeoutSeconds is ten times the 3s pg_isready
	// allows: a CPU-saturated 100m instance needs seconds of CPU time to
	// start the probe's backend.
	singleInstanceReadinessTimeoutSeconds int64 = 30
)

// singleInstanceReadiness is the readiness probe of a Cluster with one
// instance (w8/298, ADR009 § Readiness under load). CNPG's default strategy
// runs pg_isready, whose own 3s connect timeout ignores the kubelet's, so load
// alone took the lone Pod off the -rw Service. Clusters with standbys keep
// CNPG's defaults: there, readiness picks which instances serve.
func singleInstanceReadiness() map[string]any {
	return map[string]any{
		"type":           cnpgProbeQuery,
		"timeoutSeconds": singleInstanceReadinessTimeoutSeconds,
	}
}

// holdReadinessTiming keeps a live Cluster's readiness timeout in a desired
// spec whose Pods are not otherwise rolling. The timeout is part of every
// instance Pod's spec, so CNPG applies a change to it by restarting the
// instances; the strategy, which the instance manager reads from the Cluster
// at each check, changes in place.
func holdReadinessTiming(live *unstructured.Unstructured, desired map[string]any) {
	if _, projected, _ := unstructured.NestedMap(desired, "probes", "readiness"); !projected {
		return
	}
	timeout, found, _ := unstructured.NestedFieldNoCopy(live.Object, "spec", "probes", "readiness", "timeoutSeconds")
	if !found {
		unstructured.RemoveNestedField(desired, "probes", "readiness", "timeoutSeconds")
		return
	}
	_ = unstructured.SetNestedField(desired, timeout, "probes", "readiness", "timeoutSeconds")
}
