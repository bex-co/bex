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

// TestQuotaCapsForPlan pins the per-workspace object-count ceilings
// (ADR043 D3, w3/m34) against the full plan catalog — the numbers
// quotaForPlan (namespaces.go) stamps onto every workspace's ResourceQuota
// and ResourceLimits (workspaces.Service) shows as the "3/5 services" display
// cap. Hobby is asserted by its real stored plan name ("hobby", what
// NormalizePlan actually persists), not just the legacy "free"/"" aliases —
// this is the exact case that regressed silently before QuotaCapsForPlan
// existed (quotaForPlan used to match only "", "free", so every real Hobby
// workspace fell through to the generous paid ceiling).
func TestQuotaCapsForPlan(t *testing.T) {
	for _, tc := range []struct {
		plan string
		want QuotaCaps
	}{
		{PlanHobby, QuotaCaps{Services: 25, Postgres: 1, KeyValues: 1}},
		{"", QuotaCaps{Services: 25, Postgres: 1, KeyValues: 1}},
		{"free", QuotaCaps{Services: 25, Postgres: 1, KeyValues: 1}},
		{PlanPro, QuotaCaps{Services: 100, Postgres: 25, KeyValues: 25}},
		{PlanScale, QuotaCaps{Services: 100, Postgres: 25, KeyValues: 25}},
		{PlanEnterprise, QuotaCaps{Services: 100, Postgres: 25, KeyValues: 25}},
	} {
		if got := QuotaCapsForPlan(tc.plan); got != tc.want {
			t.Errorf("QuotaCapsForPlan(%q) = %+v, want %+v", tc.plan, got, tc.want)
		}
	}
}

// TestQuotaCapsForPlanServicesMatchesPlanLimits guards against the two
// independent "what is Hobby's service cap" catalogs (LimitsFor's workspace
// capability catalog and QuotaCapsForPlan's ResourceQuota ceiling) drifting
// apart — QuotaCapsForPlan derives Hobby's Services value from
// LimitsFor(PlanHobby).MaxServices rather than a second hardcoded literal.
func TestQuotaCapsForPlanServicesMatchesPlanLimits(t *testing.T) {
	if got, want := QuotaCapsForPlan(PlanHobby).Services, int64(LimitsFor(PlanHobby).MaxServices); got != want {
		t.Errorf("QuotaCapsForPlan(hobby).Services = %d, want it to match LimitsFor(hobby).MaxServices = %d", got, want)
	}
}
