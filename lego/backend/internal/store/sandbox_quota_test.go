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
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

var everyPlan = []string{PlanHobby, "", PlanPro, PlanScale, PlanEnterprise}

// A workspace's sandbox namespace holds its plan's concurrent sandboxes (100
// on Hobby, 1,000 on Pro and higher) of the largest sandbox pod on every key
// the quota charges, on top of the plan's own budget, which stays for the
// agent-session sandboxes the count leaves out. Before w7/076 it carried the
// plan's budget alone: 264Gi of memory limits held 66 `1c-2g` pods at the
// 4Gi their init container charges, so a create the plan admitted failed
// after the pod-ready wait.
func TestSandboxQuotaHoldsThePlansConcurrentSandboxes(t *testing.T) {
	for _, plan := range everyPlan {
		sandbox, budget := quotaForRegime(plan, RegimeSandbox), quotaForPlan(plan)
		n := int64(core.ConcurrentSandboxes(plan))
		for key, each := range SandboxPodQuotaMax() {
			total := sandbox[key]
			room := total.DeepCopy()
			room.Sub(budget[key])
			need := each.DeepCopy()
			need.Mul(n)
			if room.Cmp(need) < 0 {
				t.Errorf("plan %q: sandbox quota %s is %s, which leaves %s above the plan's budget for %d sandboxes needing %s",
					plan, key, total.String(), room.String(), n, need.String())
			}
		}
	}
}

// The room is the sandbox regime's alone: a hosting namespace keeps its plan's
// quota, key for key.
func TestHostingQuotaIsThePlansAlone(t *testing.T) {
	for _, plan := range everyPlan {
		if got, want := quotaForRegime(plan, RegimeHosting), quotaForPlan(plan); !equality.Semantic.DeepEqual(got, want) {
			t.Errorf("plan %q: hosting quota = %v, want the plan's %v", plan, got, want)
		}
	}
}

// A sandbox pod's quota charge is, key by key, the larger of its containers'
// sum and its init container's, and the init container (execd-installer)
// takes the sandbox LimitRange's defaults. Those defaults must stay within
// SandboxPodQuotaMax, or the init container would set a pod's charge above
// it (as its 4Gi memory default does for `1c-2g`).
func TestSandboxPodQuotaMaxCoversTheInitContainer(t *testing.T) {
	limits := limitRangeFor("ns", RegimeSandbox).Spec.Limits[0]
	max := SandboxPodQuotaMax()
	checks := []struct {
		quotaKey corev1.ResourceName
		defaults corev1.ResourceList
		name     corev1.ResourceName
	}{
		{corev1.ResourceLimitsCPU, limits.Default, corev1.ResourceCPU},
		{corev1.ResourceLimitsMemory, limits.Default, corev1.ResourceMemory},
		{corev1.ResourceLimitsEphemeralStorage, limits.Default, corev1.ResourceEphemeralStorage},
		{corev1.ResourceRequestsCPU, limits.DefaultRequest, corev1.ResourceCPU},
		{corev1.ResourceRequestsMemory, limits.DefaultRequest, corev1.ResourceMemory},
		{corev1.ResourceRequestsEphemeralStorage, limits.DefaultRequest, corev1.ResourceEphemeralStorage},
	}
	for _, c := range checks {
		got, ceiling := c.defaults[c.name], max[c.quotaKey]
		if got.Cmp(ceiling) > 0 {
			t.Errorf("sandbox LimitRange default %s %s exceeds SandboxPodQuotaMax %s %s", c.name, got.String(), c.quotaKey, ceiling.String())
		}
	}
}

// The reconciler writes the sandbox room into the workspace's sandbox
// namespace only, split as every quota is: CPU, memory and pods in the scoped
// compute quota, ephemeral storage in tenant-quota.
func TestReconcileGivesTheSandboxNamespaceItsRoom(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t)
	tn, err := store.CreateTenant(ctx, "acme", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		namespace string
		want      corev1.ResourceList
	}{
		{WorkspaceNamespace(tn.ID), quotaForPlan(PlanPro)},
		{SandboxNamespace(tn.ID), quotaForRegime(PlanPro, RegimeSandbox)},
	} {
		wantCompute, wantRest := splitComputeQuota(c.want)
		var compute, rest corev1.ResourceQuota
		if err := cl.Get(ctx, client.ObjectKey{Namespace: c.namespace, Name: core.TenantComputeQuotaName}, &compute); err != nil {
			t.Fatalf("%s: compute quota: %v", c.namespace, err)
		}
		if err := cl.Get(ctx, client.ObjectKey{Namespace: c.namespace, Name: core.TenantQuotaName}, &rest); err != nil {
			t.Fatalf("%s: tenant quota: %v", c.namespace, err)
		}
		if !equality.Semantic.DeepEqual(compute.Spec.Hard, wantCompute) {
			t.Errorf("%s: compute quota = %v, want %v", c.namespace, compute.Spec.Hard, wantCompute)
		}
		if !equality.Semantic.DeepEqual(rest.Spec.Hard, wantRest) {
			t.Errorf("%s: tenant quota = %v, want %v", c.namespace, rest.Spec.Hard, wantRest)
		}
	}
	// The figures a Pro workspace's sandbox namespace reaches: 1,000 `8c-16g`
	// pods on top of the plan's 116 CPU of limits and 500 pods.
	sandboxCPU, _ := SandboxComputeLimits(PlanPro)
	if got, want := sandboxCPU.Value(), int64(116+1000*8); got != want {
		t.Errorf("Pro sandbox CPU limits = %d, want %d", got, want)
	}
	pods := quotaForRegime(PlanPro, RegimeSandbox)[corev1.ResourcePods]
	if got, want := pods.Value(), int64(500+1000); got != want {
		t.Errorf("Pro sandbox pods = %d, want %d", got, want)
	}
}
