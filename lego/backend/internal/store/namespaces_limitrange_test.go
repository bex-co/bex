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

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// The sandbox regime's LimitRange admits Render's 40 GB writable layer, the
// hosting regime keeps its 32Gi ladder, and a Hobby workspace's
// ephemeral-storage quota still holds its plan's 100 active sandboxes at that
// size (w22/008, w7/069).
func TestSandboxRegimeLimitRangeAdmitsTheSandboxWritableLayer(t *testing.T) {
	max := func(regime string) resource.Quantity {
		return limitRangeFor("ns", regime).Spec.Limits[0].Max[corev1.ResourceEphemeralStorage]
	}
	if got, want := max(RegimeSandbox), resource.MustParse("40Gi"); got.Cmp(want) != 0 {
		t.Errorf("sandbox LimitRange ephemeral max = %s, want %s", got.String(), want.String())
	}
	if got, want := max(RegimeHosting), resource.MustParse("32Gi"); got.Cmp(want) != 0 {
		t.Errorf("hosting LimitRange ephemeral max = %s, want %s", got.String(), want.String())
	}
	sandbox := limitRangeFor("ns", RegimeSandbox).Spec.Limits[0]
	if got, want := sandbox.Default[corev1.ResourceEphemeralStorage], resource.MustParse("10Gi"); got.Cmp(want) != 0 {
		t.Errorf("sandbox LimitRange ephemeral default = %s, want the unchanged %s", got.String(), want.String())
	}
	// The Hobby cap is the plan table's, not a copy of it.
	quota := quotaForPlan(PlanHobby)[corev1.ResourceLimitsEphemeralStorage]
	each := resource.MustParse(SandboxEphemeralMax)
	holds := quota.Value() / each.Value()
	if want := core.ConcurrentSandboxes(PlanHobby); holds < int64(want) {
		t.Errorf("Hobby ephemeral-storage limit quota %s holds %d sandboxes of %s, below the plan's cap of %d",
			quota.String(), holds, SandboxEphemeralMax, want)
	}
}

// A container in a sandbox namespace that names no CPU limit (the substrate's
// execd-installer init container) gets the smallest plan's CPU, so it never
// raises a sandbox pod's CPU quota above its plan; hosting keeps 2 (w22/020).
func TestSandboxRegimeLimitRangeDefaultsCPUToTheSmallestPlan(t *testing.T) {
	item := func(regime string) corev1.LimitRangeItem { return limitRangeFor("ns", regime).Spec.Limits[0] }
	sandbox, hosting := item(RegimeSandbox), item(RegimeHosting)
	if got, want := sandbox.Default[corev1.ResourceCPU], resource.MustParse("500m"); got.Cmp(want) != 0 {
		t.Errorf("sandbox LimitRange default CPU limit = %s, want %s", got.String(), want.String())
	}
	if got, want := hosting.Default[corev1.ResourceCPU], resource.MustParse("2"); got.Cmp(want) != 0 {
		t.Errorf("hosting LimitRange default CPU limit = %s, want the unchanged %s", got.String(), want.String())
	}
	// The API server refuses a LimitRange whose default request exceeds its
	// default limit, and nothing else about the sandbox defaults moved.
	request := sandbox.DefaultRequest[corev1.ResourceCPU]
	limit := sandbox.Default[corev1.ResourceCPU]
	if request.Cmp(limit) > 0 {
		t.Errorf("sandbox LimitRange default CPU request %s exceeds its default limit %s", request.String(), limit.String())
	}
	for _, name := range []corev1.ResourceName{corev1.ResourceMemory, corev1.ResourceEphemeralStorage} {
		got, want := sandbox.Default[name], hosting.Default[name]
		if got.Cmp(want) != 0 {
			t.Errorf("sandbox LimitRange default %s = %s, want hosting's %s", name, got.String(), want.String())
		}
	}
}
