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

package store

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// datastore_quota_test.go guards w7/m77/t006 — the quota half of the ADR043 D8
// defect. w3/m34 deleted the app-code BEX_MAX_POSTGRES / BEX_MAX_KEYVALUES caps
// on the premise that the per-namespace ResourceQuota replaced them. That
// premise only held for Services: the datastore CRs were created in the shared
// apps namespace, so these dimensions counted nothing and Postgres/Key Value
// creation was uncapped in practice (.pm/w3/010.md). D8 moved the CRs into the
// namespace being charged, which is what makes the dimensions real.

// TestQuotaChargesEveryKindThatLandsInTheNamespace pins the invariant that made
// the gap possible: a cap is only a cap if the object it counts actually lands
// where the quota applies. All three hosting kinds now do.
func TestQuotaChargesEveryKindThatLandsInTheNamespace(t *testing.T) {
	for _, plan := range []string{PlanHobby, "free", "", "starter"} {
		quota := quotaForPlan(plan)
		caps := QuotaCapsForPlan(plan)
		for _, tc := range []struct {
			key  corev1.ResourceName
			want int64
			kind string
		}{
			{AppsQuotaCountKey, caps.Services, "App"},
			{DatabasesQuotaCountKey, caps.Postgres, "Database"},
			{KeyValuesQuotaCountKey, caps.KeyValues, "KeyValue"},
		} {
			got, ok := quota[tc.key]
			if !ok {
				t.Errorf("plan %q: quota has no %s dimension, so %s creation is uncapped", plan, tc.key, tc.kind)
				continue
			}
			if got.Value() != tc.want {
				t.Errorf("plan %q: %s = %d, want the plan cap %d", plan, tc.key, got.Value(), tc.want)
			}
		}
	}
}

// TestFreePlanDatastoreCapsAreTighterThanPaid keeps the dimensions from being
// present but meaningless. A cap equal across every plan would satisfy the test
// above while capping nothing in practice, which is close to the state this
// milestone is fixing.
func TestFreePlanDatastoreCapsAreTighterThanPaid(t *testing.T) {
	free, paid := QuotaCapsForPlan(PlanHobby), QuotaCapsForPlan("starter")
	if free.Postgres >= paid.Postgres {
		t.Errorf("free Postgres cap %d is not below the paid cap %d", free.Postgres, paid.Postgres)
	}
	if free.KeyValues >= paid.KeyValues {
		t.Errorf("free Key Value cap %d is not below the paid cap %d", free.KeyValues, paid.KeyValues)
	}
}
