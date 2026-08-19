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

import "k8s.io/apimachinery/pkg/api/resource"

const bytesPerGi = int64(1024 * 1024 * 1024)

func quantityGiCeil(q resource.Quantity) int32 {
	bytes := q.Value()
	if bytes <= 0 {
		return 0
	}
	return int32((bytes + bytesPerGi - 1) / bytesPerGi)
}

func storageQuantityGB(value string) int32 {
	q, err := resource.ParseQuantity(value)
	if err != nil {
		return 0
	}
	return quantityGiCeil(q)
}

// growOnlyStorage raises a requested volume size to a plan's floor — storage
// can grow past the plan, never shrink below it. Shared by the Database
// (resolvePlan) and KeyValue (resolveKVPlan) plan resolvers.
func growOnlyStorage(requested, floor int32) int32 {
	return max(requested, floor)
}

// growOnlyIntent is the grow-only storage arithmetic shared by the Database
// and KeyValue storage-intent paths, which differ only in where their observed
// substrate sizes come from (CNPG Cluster spec vs StatefulSet template + PVC).
// Pure so the invariant is directly unit-testable:
//
//   - currentGB is the high-water allocation: the status record
//     (allocatedGB) and every substrate-observed size, whichever is largest.
//   - effectiveGB is what the reconcile converges toward — max(desiredGB,
//     currentGB), so storage only ever grows.
//   - shrink reports a NEW spec intent (observedGB != specGB, on a resource
//     that has an allocation) explicitly below the current allocation, which
//     the caller must reject with its own per-resource condition writes. A
//     spec already observed at its current value is never a shrink even when
//     the allocation has grown past it — that is what lets an automatic disk
//     grow keep round-tripping without the user editing the spec. An unset
//     spec (0) never shrinks.
func growOnlyIntent(allocatedGB, observedGB, specGB, desiredGB int32, observedSizesGB ...int32) (currentGB, effectiveGB int32, shrink bool) {
	currentGB = allocatedGB
	for _, gb := range observedSizesGB {
		currentGB = max(currentGB, gb)
	}
	effectiveGB = max(desiredGB, currentGB)
	intentChanged := allocatedGB > 0 && observedGB != specGB
	shrink = intentChanged && specGB > 0 && currentGB > specGB
	return currentGB, effectiveGB, shrink
}
