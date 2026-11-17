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

package sandbox

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A sandbox pod's CPU quota is the larger of its container's limit (the
// plan's) and its init container's (the sandbox namespace's default limit).
// The meter bills the CPU the pod used, so a plan smaller than that default
// could be billed above its own size (w22/020).
func TestNoSandboxPlanIsSmallerThanTheNamespaceDefaultCPULimit(t *testing.T) {
	floor := resource.MustParse(store.SandboxDefaultCPULimit)
	for plan, size := range planResources {
		cpu, err := resource.ParseQuantity(size.CPU)
		if err != nil {
			t.Errorf("plan %s: CPU %q does not parse: %v", plan, size.CPU, err)
			continue
		}
		if cpu.Cmp(floor) < 0 {
			t.Errorf("plan %s has %s CPU, below the sandbox namespace's default limit %s: its pod's CPU quota, and its bill, would be the default's",
				plan, cpu.String(), floor.String())
		}
	}
}

// Every plan's pod fits the per-sandbox room the sandbox namespace's quota is
// sized with (store.SandboxPodQuotaMax): its container's limits are the plan's
// CPU and memory and the template's 40Gi writable layer, and its requests are
// what Create asks for (requestFor, ephemeralRequestFor). The init container's
// side is pinned in the store package. Without this, a larger plan would fill
// the quota before the plan's concurrent-sandbox cap and fail after the
// pod-ready wait (w7/076).
func TestEverySandboxPlanFitsTheQuotaRoomPerSandbox(t *testing.T) {
	max := store.SandboxPodQuotaMax()
	for plan, size := range planResources {
		reqCPU, reqMem := requestFor(size.CPU, size.Memory)
		charge := map[corev1.ResourceName]string{
			corev1.ResourceLimitsCPU:                size.CPU,
			corev1.ResourceLimitsMemory:             size.Memory,
			corev1.ResourceLimitsEphemeralStorage:   store.SandboxEphemeralMax,
			corev1.ResourceRequestsCPU:              reqCPU,
			corev1.ResourceRequestsMemory:           reqMem,
			corev1.ResourceRequestsEphemeralStorage: ephemeralRequestFor(store.SandboxEphemeralMax),
		}
		for key, value := range charge {
			got, ceiling := resource.MustParse(value), max[key]
			if got.Cmp(ceiling) > 0 {
				t.Errorf("plan %s charges %s %s, above the quota's room per sandbox %s", plan, key, got.String(), ceiling.String())
			}
		}
	}
}
