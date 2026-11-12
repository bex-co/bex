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
	"os"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestValkeyRuntimeMaxmemoryPolicy drives the operator's real CONFIG SET on
// both pinned Valkey images, started with the pod's own command
// (BEX_TEST_VALKEY=1, w4/m330): the platform user moves the budget and the
// policy in the running process (same run_id) and a Persistence Off key
// survives; setting what already runs is a no-op; clearing the policy returns
// the server to its defaults; the tenant still may not CONFIG SET (w4/191).
func TestValkeyRuntimeMaxmemoryPolicy(t *testing.T) {
	if os.Getenv("BEX_TEST_VALKEY") != "1" {
		t.Skip("set BEX_TEST_VALKEY=1 to run pinned Valkey Docker maxmemory tests")
	}
	for _, major := range []string{"7", "8"} {
		t.Run(major, func(t *testing.T) {
			e := startTLSSwitchEngine(t, major)
			if err := e.tenant.Set(e.ctx, "kept", "yes", 0).Err(); err != nil {
				t.Fatal(err)
			}
			birth := e.runID()
			plan, _ := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "free"})
			expect := func(step, maxmemory, policy string) {
				t.Helper()
				got, err := e.platform.ConfigGet(e.ctx, "maxmemory*").Result()
				if err != nil {
					t.Fatal(err)
				}
				if got["maxmemory"] != maxmemory || got["maxmemory-policy"] != policy {
					t.Fatalf("%s: maxmemory %q policy %q, want %q %q", step, got["maxmemory"], got["maxmemory-policy"], maxmemory, policy)
				}
				if v, err := e.tenant.Get(e.ctx, "kept").Result(); err != nil || v != "yes" {
					t.Fatalf("%s lost the key: %q, %v", step, v, err)
				}
				if e.runID() != birth {
					t.Fatalf("%s restarted the process", step)
				}
			}
			expect("started without a policy", "0", "noeviction")

			for _, policy := range []string{"allkeys-lru", "volatile-ttl", "volatile-ttl"} {
				maxmemory, want := valkeyMaxmemoryConfig(appv1alpha1.KeyValueSpec{MaxmemoryPolicy: policy}, plan)
				if err := setValkeyMaxmemory(e.ctx, e.platform, maxmemory, want); err != nil {
					t.Fatalf("set %s: %v", policy, err)
				}
				expect("set "+policy, maxmemory, want)
			}
			maxmemory, policy := valkeyMaxmemoryConfig(appv1alpha1.KeyValueSpec{}, plan)
			if err := setValkeyMaxmemory(e.ctx, e.platform, maxmemory, policy); err != nil {
				t.Fatalf("clear the policy: %v", err)
			}
			expect("cleared", "0", "noeviction")

			if err := e.tenant.ConfigSet(e.ctx, "maxmemory-policy", "allkeys-lru").Err(); err == nil {
				t.Fatal("the tenant's default user ran CONFIG SET")
			}
		})
	}
}
