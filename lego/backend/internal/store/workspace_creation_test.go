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

import "testing"

// TestTenantBillingEmailAtFinalize pins the Hobby birth rule across the full
// plan catalog: a workspace finalized on Hobby stores no billing email — its
// billing email is its owner's account email — while a paid finalize keeps
// the attempt's address. updateTenantPlanTx clears it the same way on every
// move to Hobby; without this, an upgrade unmasks the creation-time address
// as one the workspace set.
func TestTenantBillingEmailAtFinalize(t *testing.T) {
	for _, tc := range []struct {
		plan    string
		attempt string
		want    string
	}{
		{PlanHobby, "owner@example.com", ""},
		{PlanPro, "finance@example.com", "finance@example.com"},
		{PlanScale, "finance@example.com", "finance@example.com"},
		{PlanEnterprise, "finance@example.com", "finance@example.com"},
		{PlanPro, "", ""},
		{PlanEnterprise, "", ""},
	} {
		if got := tenantBillingEmailAtFinalize(tc.plan, tc.attempt); got != tc.want {
			t.Errorf("tenantBillingEmailAtFinalize(%q, %q) = %q, want %q", tc.plan, tc.attempt, got, tc.want)
		}
	}
}
