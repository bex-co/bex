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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// TestResourcesForTierReadsTheSharedCatalog pins resourcesForTier's contract
// with lego/types/tiers now that it no longer carries its own ladder: a known
// tier yields requests == limits (Guaranteed QoS) from the catalog, empty or
// unknown tiers stay best-effort (no resource constraints at all — not
// zeroed requests, which would make the pod BestEffort-evictable the same way
// but for the wrong reason).
func TestResourcesForTierReadsTheSharedCatalog(t *testing.T) {
	got := resourcesForTier("standard")
	want := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}
	if !got.Requests.Cpu().Equal(*want.Requests.Cpu()) || !got.Requests.Memory().Equal(*want.Requests.Memory()) {
		t.Errorf("standard requests = %+v, want %+v", got.Requests, want.Requests)
	}
	if !got.Limits.Cpu().Equal(*want.Limits.Cpu()) || !got.Limits.Memory().Equal(*want.Limits.Memory()) {
		t.Errorf("standard limits = %+v, want %+v", got.Limits, want.Limits)
	}
	if got.Requests.Cpu().Cmp(*got.Limits.Cpu()) != 0 || got.Requests.Memory().Cmp(*got.Limits.Memory()) != 0 {
		t.Error("requests must equal limits (Guaranteed QoS)")
	}
}

func TestResourcesForTierEmptyOrUnknownIsBestEffort(t *testing.T) {
	for _, tier := range []string{"", "gold-plated"} {
		got := resourcesForTier(tier)
		if got.Requests != nil || got.Limits != nil {
			t.Errorf("resourcesForTier(%q) = %+v, want a zero-value ResourceRequirements (best-effort)", tier, got)
		}
	}
}
