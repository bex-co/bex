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

package core

import "github.com/bex-co/bex/lego/types/tiers"

// PaidPlan reports whether an already-catalog-validated plan is non-free.
// Every tier family has the same explicit/default free id in tiers.yaml; the
// feature owning the family validates/normalizes before calling this helper.
func PaidPlan(plan string) bool {
	return plan != "" && plan != "free"
}

// DefaultPaidComputeTier is the cheapest paid rung of the compute ladder — the
// catalog is ordered smallest-first, so the first non-free id is the tier a
// plan-less service falls to (starter, Render's 0.5c-512mb). The catalog-default
// fallback is unreachable with the reviewed tiers.yaml, which always carries
// paid rungs.
func DefaultPaidComputeTier() string {
	for _, tierID := range tiers.Compute.IDs() {
		if PaidPlan(tierID) {
			return tierID
		}
	}
	return tiers.Compute.Default().ID
}
