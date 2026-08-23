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
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bex-co/bex/lego/types/tiers"
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

// TestQuotaBoundsEphemeralStorage (round-14 #2): the per-workspace aggregate
// quota must carry the requests/limits.ephemeral-storage dimensions — the
// writable-layer/log disk axis the PVC dimensions cannot see. Present and
// positive on every plan, free strictly below paid, and the LimitRange
// defaults tierless containers to a bounded value too.
func TestQuotaBoundsEphemeralStorage(t *testing.T) {
	for _, plan := range []string{PlanHobby, "free", "", "starter"} {
		quota := quotaForPlan(plan)
		for _, key := range []corev1.ResourceName{corev1.ResourceRequestsEphemeralStorage, corev1.ResourceLimitsEphemeralStorage} {
			got, ok := quota[key]
			if !ok {
				t.Errorf("plan %q: quota has no %s dimension — tenant writable-layer disk is aggregate-uncapped", plan, key)
				continue
			}
			if got.Value() <= 0 {
				t.Errorf("plan %q: %s = %s, want a positive bound", plan, key, got.String())
			}
		}
	}
	free := quotaForPlan(PlanHobby)[corev1.ResourceRequestsEphemeralStorage]
	paid := quotaForPlan("starter")[corev1.ResourceRequestsEphemeralStorage]
	if free.Cmp(paid) >= 0 {
		t.Errorf("free ephemeral-storage cap %s is not below the paid cap %s", free.String(), paid.String())
	}
	lr := baseLimitRange("ns")
	item := lr.Spec.Limits[0]
	for _, list := range []corev1.ResourceList{item.DefaultRequest, item.Default, item.Max} {
		if q, ok := list[corev1.ResourceEphemeralStorage]; !ok || q.Value() <= 0 {
			t.Errorf("LimitRange tierless-container defaults/max missing a positive ephemeral-storage bound: %+v", list)
		}
	}
	// The LimitRange max must admit the compute ladder's largest tier or every
	// pro-ultra container would be rejected at admission.
	maxQ := item.Max[corev1.ResourceEphemeralStorage]
	top, ok := tiers.Compute.ByID("pro-ultra")
	if !ok {
		t.Fatal("compute catalog has no pro-ultra rung")
	}
	tierQ := resource.MustParse(top.EphemeralStorage)
	if maxQ.Cmp(tierQ) < 0 {
		t.Errorf("LimitRange max ephemeral-storage %s < largest tier bound %s — top-tier containers would be refused", maxQ.String(), tierQ.String())
	}
}

// TestQuotaCarriesRollingUpdateSurgeHeadroom pins the compute-axis surge
// headroom: App Deployments use the default RollingUpdate strategy (maxSurge
// 25% → +1 pod at replicas:1, maxUnavailable 25% → 0), so a quota without
// slack rejects the surge ReplicaSet and hangs the deploy at the health gate
// (observed live on a Hobby workspace at requests.cpu 1910m/2 whose +500m
// starter surge was refused). Each plan must admit its steady-state budget
// plus one surge pod of the largest tier it can reasonably run, on requests
// AND limits (Guaranteed QoS charges both).
func TestQuotaCarriesRollingUpdateSurgeHeadroom(t *testing.T) {
	surgeOf := func(tierID string) (cpu, mem resource.Quantity) {
		tier, ok := tiers.Compute.ByID(tierID)
		if !ok {
			t.Fatalf("compute catalog has no %s rung", tierID)
		}
		return resource.MustParse(tier.CPU), resource.MustParse(tier.Memory)
	}
	// Steady-state budget each plan's quota is derived from, plus the surge
	// tier it must absorb on top (hobby: standard; paid: pro-ultra).
	for _, tc := range []struct {
		plans                       []string
		baseCPU, baseMem, surgeTier string
	}{
		{[]string{PlanHobby, "free", ""}, "2", "4Gi", "standard"},
		{[]string{"starter", PlanPro, PlanScale}, "50", "100Gi", "pro-ultra"},
	} {
		surgeCPU, surgeMem := surgeOf(tc.surgeTier)
		wantReqCPU := resource.MustParse(tc.baseCPU)
		wantReqCPU.Add(surgeCPU)
		wantReqMem := resource.MustParse(tc.baseMem)
		wantReqMem.Add(surgeMem)
		for _, plan := range tc.plans {
			quota := quotaForPlan(plan)
			if got := quota[corev1.ResourceRequestsCPU]; got.Cmp(wantReqCPU) != 0 {
				t.Errorf("plan %q requests.cpu = %s, want %s (base %s + 1 %s surge)", plan, got.String(), wantReqCPU.String(), tc.baseCPU, tc.surgeTier)
			}
			if got := quota[corev1.ResourceRequestsMemory]; got.Cmp(wantReqMem) != 0 {
				t.Errorf("plan %q requests.memory = %s, want %s (base %s + 1 %s surge)", plan, got.String(), wantReqMem.String(), tc.baseMem, tc.surgeTier)
			}
			// Limits stay at 2× requests, so the surge fits there too.
			if got := quota[corev1.ResourceLimitsCPU]; got.Cmp(wantReqCPU) < 0 {
				t.Errorf("plan %q limits.cpu = %s < requests.cpu %s — a Guaranteed-QoS surge would be refused", plan, got.String(), wantReqCPU.String())
			}
			if got := quota[corev1.ResourceLimitsMemory]; got.Cmp(wantReqMem) < 0 {
				t.Errorf("plan %q limits.memory = %s < requests.memory %s — a Guaranteed-QoS surge would be refused", plan, got.String(), wantReqMem.String())
			}
		}
	}
}
