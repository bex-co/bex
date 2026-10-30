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
	"fmt"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w2/056: a Hobby workspace's datastore count caps (25 Postgres / 25 Key Value)
// must bind before the dimensions datastores share with services.

// quotaCharge is one object as the ResourceQuota admission plugin charges it:
// its usage and, for a Pod, its PriorityClass (scopes only ever match Pods).
type quotaCharge struct {
	pod   bool
	class string
	usage corev1.ResourceList
}

// quotaRefusal replays quota admission over a namespace's quotas: every quota
// whose scope matches a charge adds its usage, and the first dimension past its
// hard cap names the refusing quota ("" when everything is admitted). It models
// the two shapes baseResourceQuotas emits: unscoped, and PriorityClass NotIn.
func quotaRefusal(t *testing.T, quotas []*corev1.ResourceQuota, charges []quotaCharge) string {
	t.Helper()
	for _, q := range quotas {
		used := corev1.ResourceList{}
		for _, c := range charges {
			if !quotaScopeMatches(t, q, c) {
				continue
			}
			for key, v := range c.usage {
				if _, capped := q.Spec.Hard[key]; !capped {
					continue
				}
				sum := used[key]
				sum.Add(v)
				used[key] = sum
			}
		}
		for key, v := range used {
			if hard := q.Spec.Hard[key]; v.Cmp(hard) > 0 {
				return fmt.Sprintf("%s: %s used %s > %s", q.Name, key, v.String(), hard.String())
			}
		}
	}
	return ""
}

func quotaScopeMatches(t *testing.T, q *corev1.ResourceQuota, c quotaCharge) bool {
	t.Helper()
	if q.Spec.ScopeSelector == nil {
		return true
	}
	for _, req := range q.Spec.ScopeSelector.MatchExpressions {
		if req.ScopeName != corev1.ResourceQuotaScopePriorityClass || req.Operator != corev1.ScopeSelectorOpNotIn {
			t.Fatalf("quota %s: unmodelled scope %+v", q.Name, req)
		}
		if !c.pod || slices.Contains(req.Values, c.class) {
			return false
		}
	}
	return true
}

// podCharge is one Pod. Its containers declare no ephemeral-storage, so each
// gets the LimitRange defaults (baseLimitRange), as on the API server.
func podCharge(class, cpu, memory string, containers int64) quotaCharge {
	lr := baseLimitRange("ns").Spec.Limits[0]
	ephReq, ephLim := lr.DefaultRequest[corev1.ResourceEphemeralStorage], lr.Default[corev1.ResourceEphemeralStorage]
	return quotaCharge{pod: true, class: class, usage: corev1.ResourceList{
		corev1.ResourcePods:                     resource.MustParse("1"),
		corev1.ResourceRequestsCPU:              resource.MustParse(cpu),
		corev1.ResourceLimitsCPU:                resource.MustParse(cpu),
		corev1.ResourceRequestsMemory:           resource.MustParse(memory),
		corev1.ResourceLimitsMemory:             resource.MustParse(memory),
		corev1.ResourceRequestsEphemeralStorage: *resource.NewQuantity(containers*ephReq.Value(), resource.BinarySI),
		corev1.ResourceLimitsEphemeralStorage:   *resource.NewQuantity(containers*ephLim.Value(), resource.BinarySI),
	}}
}

func pvcCharge(size string) quotaCharge {
	return quotaCharge{usage: corev1.ResourceList{
		corev1.ResourcePersistentVolumeClaims: resource.MustParse("1"),
		corev1.ResourceRequestsStorage:        resource.MustParse(size),
	}}
}

func countCharge(key corev1.ResourceName) quotaCharge {
	return quotaCharge{usage: corev1.ResourceList{key: resource.MustParse("1")}}
}

func repeatCharge(n int, c quotaCharge) []quotaCharge {
	out := make([]quotaCharge, n)
	for i := range out {
		out[i] = c
	}
	return out
}

// hobbyServicesAtBudget is a Hobby workspace whose services already use their
// budget: 25 Apps (the services cap) running 100m Pods, 2.5 of the 3 CPU, and
// four default 10 GB disks.
func hobbyServicesAtBudget() []quotaCharge {
	var charges []quotaCharge
	charges = append(charges, repeatCharge(25, countCharge(AppsQuotaCountKey))...)
	charges = append(charges, repeatCharge(25, podCharge("", "100m", "128Mi", 1))...)
	return append(charges, repeatCharge(4, pvcCharge("10Gi"))...)
}

// haPostgres is one `2c-4g` high-availability Postgres with backups and a
// pooler, as the operator builds it: the CR, two instance Pods (postgres +
// barman-cloud sidecar) under the datastore class, one 10 GiB data PVC per
// instance (no WAL volume), a PgBouncer Pod, and the Jobs a day can leave
// behind (3 exports, their 3 expiries, CNPG's join Job).
func haPostgres() []quotaCharge {
	charges := []quotaCharge{countCharge(DatabasesQuotaCountKey)}
	charges = append(charges, repeatCharge(2, podCharge(appv1alpha1.DatastorePriorityClassName, "2", "4Gi", 2))...)
	charges = append(charges, repeatCharge(2, pvcCharge("10Gi"))...)
	charges = append(charges, podCharge(appv1alpha1.DatastorePriorityClassName, "100m", "128Mi", 1))
	return append(charges, repeatCharge(7, countCharge("count/jobs.batch"))...)
}

// keyValue is one `standard` Key Value with backups: the CR, its Valkey Pod
// (valkey + exporter) and internal proxy under the datastore class, one 5 GiB
// PVC, and the 3 successful + 3 failed Jobs its backup CronJob keeps.
func keyValue() []quotaCharge {
	charges := []quotaCharge{countCharge(KeyValuesQuotaCountKey)}
	charges = append(charges, podCharge(appv1alpha1.DatastorePriorityClassName, "500m", "1Gi", 2))
	charges = append(charges, podCharge(appv1alpha1.DatastorePriorityClassName, "25m", "16Mi", 1))
	charges = append(charges, pvcCharge("5Gi"))
	return append(charges, repeatCharge(6, countCharge("count/jobs.batch"))...)
}

func hobbyQuotas(t *testing.T) []*corev1.ResourceQuota {
	t.Helper()
	r, _, _ := newTestNamespaceReconciler(t)
	return r.baseResourceQuotas("tea-hobby", Tenant{ID: "tea-hobby", Plan: PlanHobby})
}

// TestHobbyDatastoreCountCapsBindFirst is the w2/056 acceptance: a Hobby
// workspace whose services already use their budget still admits 25 HA
// Postgres and 25 Key Value with every Pod, PVC and Job they create, and the
// 26th of either is refused by its count cap, not by a shared dimension.
func TestHobbyDatastoreCountCapsBindFirst(t *testing.T) {
	quotas := hobbyQuotas(t)
	caps := QuotaCapsForPlan(PlanHobby)
	charges := hobbyServicesAtBudget()
	for range caps.Postgres {
		charges = append(charges, haPostgres()...)
	}
	for range caps.KeyValues {
		charges = append(charges, keyValue()...)
	}
	if refused := quotaRefusal(t, quotas, charges); refused != "" {
		t.Fatalf("Hobby refused its %d HA Postgres + %d Key Value before the count caps: %s", caps.Postgres, caps.KeyValues, refused)
	}
	for _, tc := range []struct {
		next []quotaCharge
		want corev1.ResourceName
	}{{haPostgres(), DatabasesQuotaCountKey}, {keyValue(), KeyValuesQuotaCountKey}} {
		refused := quotaRefusal(t, quotas, append(slices.Clone(charges), tc.next...))
		if !strings.Contains(refused, string(tc.want)) {
			t.Errorf("the next datastore was refused by %q, want its count cap %s", refused, tc.want)
		}
	}
}

// TestHobbyServicesStillBoundedBesideDatastores: making room for datastores
// must not let services through. Beside 25 HA Postgres, a 26th App, a Pod past
// the services' pod or CPU budget, and a disk past the storage ceiling are
// each still refused.
func TestHobbyServicesStillBoundedBesideDatastores(t *testing.T) {
	quotas := hobbyQuotas(t)
	var datastores []quotaCharge
	for range QuotaCapsForPlan(PlanHobby).Postgres {
		datastores = append(datastores, haPostgres()...)
	}
	for _, tc := range []struct {
		name     string
		services []quotaCharge
		want     string
	}{
		{"26th service", repeatCharge(26, countCharge(AppsQuotaCountKey)), string(AppsQuotaCountKey)},
		{"51st Pod", repeatCharge(51, podCharge("", "10m", "16Mi", 1)), core.TenantComputeQuotaName + ": pods"},
		{"CPU past the budget", repeatCharge(4, podCharge("", "1", "1Gi", 1)), core.TenantComputeQuotaName + ": requests.cpu"},
		{"disk past the storage ceiling", []quotaCharge{pvcCharge("10000Gi")}, core.TenantQuotaName + ": requests.storage"},
	} {
		refused := quotaRefusal(t, quotas, append(slices.Clone(datastores), tc.services...))
		if !strings.Contains(refused, tc.want) {
			t.Errorf("%s: refused by %q, want %s", tc.name, refused, tc.want)
		}
	}
}

// TestDatastoreQuotaRoomCoversLargestShape pins the derivation in
// quotaForPlan's comment: each plan's tenant-quota is its own budget plus the
// count caps times the largest datastore shape (an HA pair + 5 readers), and
// the pod count lives only in the scoped compute quota.
func TestDatastoreQuotaRoomCoversLargestShape(t *testing.T) {
	if got, want := postgresFootprint(), (datastoreFootprint{pvcs: 7, storageGi: 70, ephemeralRequestsGi: 20, ephemeralLimitsGi: 200, jobs: 8}); got != want {
		t.Errorf("Postgres footprint = %+v, want %+v", got, want)
	}
	if got, want := keyValueFootprint(), (datastoreFootprint{pvcs: 1, storageGi: 5, ephemeralRequestsGi: 15, ephemeralLimitsGi: 51, jobs: 7}); got != want {
		t.Errorf("Key Value footprint = %+v, want %+v", got, want)
	}
	for _, tc := range []struct {
		plan                                string
		pvcs, jobs                          int64
		storage, ephemeralReq, ephemeralLim string
	}{
		{PlanHobby, 208, 400, "1995Gi", "975Gi", "6475Gi"},
		{PlanPro, 400, 625, "6995Gi", "2923Gi", "10371Gi"},
	} {
		quota := quotaForPlan(tc.plan)
		for key, want := range map[corev1.ResourceName]resource.Quantity{
			corev1.ResourcePersistentVolumeClaims:   *resource.NewQuantity(tc.pvcs, resource.DecimalSI),
			"count/jobs.batch":                      *resource.NewQuantity(tc.jobs, resource.DecimalSI),
			corev1.ResourceRequestsStorage:          resource.MustParse(tc.storage),
			corev1.ResourceRequestsEphemeralStorage: resource.MustParse(tc.ephemeralReq),
			corev1.ResourceLimitsEphemeralStorage:   resource.MustParse(tc.ephemeralLim),
		} {
			if got := quota[key]; got.Cmp(want) != 0 {
				t.Errorf("plan %s: %s = %s, want %s", tc.plan, key, got.String(), want.String())
			}
		}
		compute, rest := splitComputeQuota(quota)
		if _, ok := rest[corev1.ResourcePods]; ok {
			t.Errorf("plan %s: pods is still in the unscoped tenant-quota, so datastore Pods use up the services' budget", tc.plan)
		}
		if _, ok := compute[corev1.ResourcePods]; !ok {
			t.Errorf("plan %s: the scoped compute quota has no pods cap, so services' Pod count is unbounded", tc.plan)
		}
	}
}
