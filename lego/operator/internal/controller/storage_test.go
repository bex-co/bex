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

import "testing"

// TestGrowOnlyIntent is the truth table for the grow-only storage invariant
// the Database and KeyValue storage-intent paths share (w1/m80): current is
// the high-water allocation, effective only ever grows, and only a NEW
// explicit spec intent below the allocation is a shrink.
func TestGrowOnlyIntent(t *testing.T) {
	for _, tc := range []struct {
		name                               string
		allocated, observed, spec, desired int32
		sizes                              []int32
		wantCurrent, wantEffective         int32
		wantShrink                         bool
	}{
		{
			name:      "fresh create has no allocation and takes the desired size",
			allocated: 0, observed: 0, spec: 5, desired: 5,
			wantCurrent: 0, wantEffective: 5, wantShrink: false,
		},
		{
			name:      "steady state round-trips the allocated size",
			allocated: 10, observed: 10, spec: 10, desired: 10, sizes: []int32{10},
			wantCurrent: 10, wantEffective: 10, wantShrink: false,
		},
		{
			name:      "explicit grow raises the effective size",
			allocated: 10, observed: 10, spec: 20, desired: 20, sizes: []int32{10},
			wantCurrent: 10, wantEffective: 20, wantShrink: false,
		},
		{
			name:      "explicit shrink below the allocation is rejected",
			allocated: 20, observed: 20, spec: 10, desired: 10, sizes: []int32{20},
			wantCurrent: 20, wantEffective: 20, wantShrink: true,
		},
		{
			name: "stale spec after an automatic grow is NOT a shrink",
			// The autoscaler grew the volume past the spec; the spec was already
			// observed at its current value, so the grown size keeps
			// round-tripping without the user editing anything.
			allocated: 20, observed: 10, spec: 10, desired: 10, sizes: []int32{20},
			wantCurrent: 20, wantEffective: 20, wantShrink: false,
		},
		{
			name:      "unset spec never shrinks",
			allocated: 10, observed: 5, spec: 0, desired: 10, sizes: []int32{10},
			wantCurrent: 10, wantEffective: 10, wantShrink: false,
		},
		{
			name: "substrate high-water mark wins over the status record",
			// e.g. an adopted resource: the substrate shows more than status.
			allocated: 5, observed: 5, spec: 5, desired: 5, sizes: []int32{3, 12, 8},
			wantCurrent: 12, wantEffective: 12, wantShrink: false,
		},
		{
			name:      "no allocation yet means no shrink even below observed sizes",
			allocated: 0, observed: 0, spec: 5, desired: 5, sizes: []int32{20},
			wantCurrent: 20, wantEffective: 20, wantShrink: false,
		},
		{
			name:      "desired below current is raised to current (grow-only)",
			allocated: 15, observed: 15, spec: 15, desired: 10, sizes: []int32{15},
			wantCurrent: 15, wantEffective: 15, wantShrink: false,
		},
		{
			name:      "new intent equal to the allocation is not a shrink",
			allocated: 20, observed: 10, spec: 20, desired: 20, sizes: []int32{20},
			wantCurrent: 20, wantEffective: 20, wantShrink: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current, effective, shrink := growOnlyIntent(tc.allocated, tc.observed, tc.spec, tc.desired, tc.sizes...)
			if current != tc.wantCurrent || effective != tc.wantEffective || shrink != tc.wantShrink {
				t.Fatalf("growOnlyIntent(%d, %d, %d, %d, %v) = (%d, %d, %v), want (%d, %d, %v)",
					tc.allocated, tc.observed, tc.spec, tc.desired, tc.sizes,
					current, effective, shrink, tc.wantCurrent, tc.wantEffective, tc.wantShrink)
			}
		})
	}
}
