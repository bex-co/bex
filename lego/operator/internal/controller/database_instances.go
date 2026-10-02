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

	"github.com/bex-co/bex/lego/types/tiers"
)

// databaseInstanceCount keeps named readers separate from the HA standby.
// Legacy declarations outside admission policy cannot add capacity, but their
// existing Cluster capacity remains until the author removes the declarations.
func databaseInstanceCount(p clusterParams) int64 {
	base := max(int64(1), int64(p.plan.Instances))
	if p.highAvailability {
		base = max(base, 2)
	}
	if p.readReplicas == 0 {
		return base
	}
	if p.readReplicas <= p.plan.MaxReadReplicas() &&
		p.storageGB >= tiers.PostgresReadReplicaMinStorageGB &&
		p.storageGB <= tiers.Postgres.MaxStorageGB(p.plan.ID) {
		return base + int64(p.readReplicas)
	}
	return max(base, p.currentInstances)
}

// Include both intent and observation: a scale-up or restart must not erase an
// existing legacy reader merely because CNPG has not reported it ready yet.
func cnpgInstanceCount(cluster *unstructured.Unstructured) int64 {
	declared, _, _ := unstructured.NestedInt64(cluster.Object, "spec", "instances")
	return max(declared, cnpgObservedInstanceCount(cluster))
}

func cnpgObservedInstanceCount(cluster *unstructured.Unstructured) int64 {
	observed, _, _ := unstructured.NestedInt64(cluster.Object, "status", "instances")
	ready, _, _ := unstructured.NestedInt64(cluster.Object, "status", "readyInstances")
	return max(observed, ready)
}
