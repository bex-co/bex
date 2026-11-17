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
