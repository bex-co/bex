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

// databaseInstanceCount is the primary Cluster's instances: the plan's, and a
// standby with HA. Read replicas are clusters of their own
// (database_replicas.go), so the HA standby is the only standby CNPG can
// promote and no replica URL ever reads it (w6/m467).
func databaseInstanceCount(plan tiers.PostgresTier, highAvailability bool) int64 {
	base := max(int64(1), int64(plan.Instances))
	if highAvailability {
		base = max(base, 2)
	}
	return base
}

func cnpgObservedInstanceCount(cluster *unstructured.Unstructured) int64 {
	observed, _, _ := unstructured.NestedInt64(cluster.Object, "status", "instances")
	ready, _, _ := unstructured.NestedInt64(cluster.Object, "status", "readyInstances")
	return max(observed, ready)
}
