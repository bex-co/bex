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

package staticserver

import "testing"

// cgroupLimitBytes mirrors the static-server pod's memory limit in
// config/staticserver/deployment.yaml (2 GiB). Keep in sync; the deployment
// comment references this arithmetic.
const cgroupLimitBytes int64 = 2 << 30 // 2 GiB

// TestAggregateBudgetInvariant pins the codex-security round-18 budget: the
// three independent in-memory ceilings — the object cache, the live-body
// lease budget, and the fetch gate's worst-case in-flight reservation — must
// sum to at most half of the pod's 2 GiB cgroup limit, leaving the other half
// for slice-capacity growth, io.ReadAll transients, the Go runtime/GC, the S3
// SDK, and the resolver. Bump any one knob past the invariant and this test
// fails instead of the single replica OOMing in production.
func TestAggregateBudgetInvariant(t *testing.T) {
	if got, want := defaultMaxInflightBytes, int64(defaultMaxConcurrentFetches)*maxOriginObjectBytes; got != want {
		t.Fatalf("fetch-gate byte budget %d must equal concurrency %d x max object %d = %d",
			got, defaultMaxConcurrentFetches, maxOriginObjectBytes, want)
	}
	aggregate := int64(DefaultCacheBytes) + defaultMaxLiveBodyBytes + defaultMaxInflightBytes
	maxAggregate := cgroupLimitBytes / 2
	if aggregate > maxAggregate {
		t.Fatalf("aggregate memory budget %d MiB (cache %d + live-body %d + fetch %d) exceeds %d MiB = 50%% of the %d MiB pod limit",
			aggregate>>20, DefaultCacheBytes>>20, defaultMaxLiveBodyBytes>>20, defaultMaxInflightBytes>>20,
			maxAggregate>>20, cgroupLimitBytes>>20)
	}
	t.Logf("aggregate budget %d MiB of %d MiB allowed (pod limit %d MiB)",
		aggregate>>20, maxAggregate>>20, cgroupLimitBytes>>20)
}
