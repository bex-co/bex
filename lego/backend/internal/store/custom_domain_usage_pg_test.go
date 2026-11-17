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
	"slices"
	"testing"
	"time"
)

// w7/m429 (docs/ADR030 §16): a custom domain counts toward the plan's
// included count once verified, and the www/apex sibling bex adds for it does
// not count again. These are SQL filters, so they run against Postgres.
func TestPGCountedCustomDomains(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	suffix := time.Now().Format("150405.000000")
	tenant, err := s.CreateTenant(ctx, "domains-"+suffix, PlanHobby)
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	app, err := s.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Image: "nginx:1", Branch: "main", Port: 3000, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	host := func(label string) string {
		return label + ".m429-" + time.Now().Format("150405000000") + ".example.com"
	}
	base := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	claim := func(name, redirect string, verifiedAt *time.Time) {
		t.Helper()
		d, _, err := s.AddDomainClaim(ctx, app.ID, name, redirect)
		if err != nil {
			t.Fatalf("AddDomainClaim(%s): %v", name, err)
		}
		if verifiedAt != nil {
			if _, _, err := s.PromoteDomainClaim(ctx, app.ID, d.ID, *verifiedAt); err != nil {
				t.Fatalf("PromoteDomainClaim(%s): %v", name, err)
			}
		}
	}
	first, second, later := base.Add(-2*time.Hour), base.Add(-time.Hour), base.Add(time.Hour)
	apex := host("apex")
	claim(host("first"), "", &first)
	claim(apex, "", &second)
	claim("www."+apex, apex, &second) // the sibling bex added: not counted again
	claim(host("pending"), "", nil)   // unverified: never routes, never counted
	claim(host("later"), "", &later)  // verified after the hour asked about

	want := CustomDomainUsage{WorkspaceID: tenant.ID, Plan: PlanHobby, VerifiedAt: []time.Time{first, second}}
	got, err := s.CountedCustomDomains(ctx, base, tenant.ID)
	if err != nil {
		t.Fatalf("CountedCustomDomains: %v", err)
	}
	if len(got) != 1 || got[0].WorkspaceID != want.WorkspaceID || got[0].Plan != want.Plan || !slices.EqualFunc(got[0].VerifiedAt, want.VerifiedAt, time.Time.Equal) {
		t.Fatalf("counted = %+v, want %+v", got, want)
	}
	all, err := s.CountedCustomDomains(ctx, base, "")
	if err != nil {
		t.Fatalf("CountedCustomDomains(every workspace): %v", err)
	}
	i := slices.IndexFunc(all, func(u CustomDomainUsage) bool { return u.WorkspaceID == tenant.ID })
	if i < 0 || len(all[i].VerifiedAt) != 2 {
		t.Fatalf("every-workspace read = %+v, want this workspace's two domains", all)
	}
}

func TestPGUsageMeterCursorNeverMovesBack(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	kind := "test_cursor_" + time.Now().Format("150405.000000")
	if _, ok, err := s.UsageMeterCursor(ctx, kind); err != nil || ok {
		t.Fatalf("fresh cursor ok=%v err=%v, want none", ok, err)
	}
	newer := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	for _, w := range []time.Time{newer, newer.Add(-time.Hour)} {
		if err := s.AdvanceUsageMeterCursor(ctx, kind, w); err != nil {
			t.Fatalf("AdvanceUsageMeterCursor(%s): %v", w, err)
		}
	}
	got, ok, err := s.UsageMeterCursor(ctx, kind)
	if err != nil || !ok || !got.Equal(newer) {
		t.Fatalf("cursor = %s ok=%v err=%v, want %s", got, ok, err, newer)
	}
}
