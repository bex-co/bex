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
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

// domainOverlapFixture is two workspaces: a and sameWorkspace in one, b in
// the other.
type domainOverlapFixture struct {
	a, sameWorkspace, b App
	suffix              string
}

func newDomainOverlapFixture(t *testing.T, s *PGStore) domainOverlapFixture {
	t.Helper()
	ctx := context.Background()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	app := func(tenantID, name string) App {
		t.Helper()
		a, err := s.CreateApp(ctx, App{TenantID: tenantID, Name: name, Image: "nginx:1", Branch: "main", Port: 3000, Replicas: 1, Tier: "starter"})
		if err != nil {
			t.Fatalf("CreateApp(%s): %v", name, err)
		}
		t.Cleanup(func() { _ = s.DeleteApp(context.Background(), a.ID) })
		return a
	}
	tenant := func(name string) Tenant {
		t.Helper()
		tn, err := s.CreateTenant(ctx, name+"-"+suffix, PlanHobby)
		if err != nil {
			t.Fatalf("CreateTenant(%s): %v", name, err)
		}
		return tn
	}
	ta, tb := tenant("overlap-a"), tenant("overlap-b")
	return domainOverlapFixture{a: app(ta.ID, "web"), sameWorkspace: app(ta.ID, "api"), b: app(tb.ID, "theirs"), suffix: suffix}
}

func addPendingClaim(t *testing.T, s *PGStore, appID, host string) Domain {
	t.Helper()
	d, _, err := s.AddDomainClaim(context.Background(), appID, host, "")
	if err != nil {
		t.Fatalf("AddDomainClaim(%s): %v", host, err)
	}
	return d
}

// waitForOverlapLockWaiters waits until want transactions wait on host's
// overlap lock.
func waitForOverlapLockWaiters(t *testing.T, s *PGStore, host string, want int) {
	t.Helper()
	ctx := context.Background()
	var key int64
	if err := s.Pool.QueryRow(ctx, `SELECT hashtextextended($1, 0)`, "domain-overlap:"+domainOverlapDomain(host)).Scan(&key); err != nil {
		t.Fatal(err)
	}
	// pg_locks shows a bigint advisory key as its high half in classid and
	// its low half in objid.
	high, low := int64(uint64(key)>>32), int64(uint32(uint64(key)))
	deadline := time.Now().Add(10 * time.Second)
	for {
		var waiting int
		if err := s.Pool.QueryRow(ctx,
			`SELECT count(*) FROM pg_locks
			 WHERE locktype = 'advisory' AND NOT granted AND objsubid = 1
			   AND database = (SELECT oid FROM pg_database WHERE datname = current_database())
			   AND classid::bigint = $1 AND objid::bigint = $2`, high, low).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d promotions wait on the overlap lock of %q, want %d", waiting, host, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// w4/m326: two bex-api replicas promote a wildcard of one workspace and a
// name under it of another at the same moment. Both promotions wait on the
// one overlap lock, and exactly one verifies; the other is ErrDomainCovered
// and stays pending.
func TestPGConcurrentOverlappingPromotionsVerifyOne(t *testing.T) {
	s := newReplayTestStore(t)
	ctx := context.Background()
	fx := newDomainOverlapFixture(t, s)
	base := "race-" + fx.suffix + ".example.com"
	pair := []Domain{
		addPendingClaim(t, s, fx.b.ID, "*."+base),
		addPendingClaim(t, s, fx.a.ID, "api."+base),
	}

	holder, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(context.Background()) }()
	if err := lockDomainOverlap(ctx, holder, pair[0].Host); err != nil {
		t.Fatal(err)
	}
	promoted := make([]bool, len(pair))
	errs := make([]error, len(pair))
	var wg sync.WaitGroup
	for i, claim := range pair {
		wg.Go(func() {
			_, promoted[i], errs[i] = s.PromoteDomainClaim(ctx, claim.AppID, claim.ID, time.Now())
		})
	}
	waitForOverlapLockWaiters(t, s, pair[0].Host, len(pair))
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()

	verified := 0
	for i, claim := range pair {
		row, err := s.GetDomainClaim(ctx, claim.AppID, claim.Host)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case errs[i] == nil && promoted[i] && row.ClaimState == "verified":
			verified++
		case errors.Is(errs[i], ErrDomainCovered) && row.ClaimState == "pending":
		default:
			t.Errorf("%s: promoted=%v err=%v state=%s", claim.Host, promoted[i], errs[i], row.ClaimState)
		}
	}
	if verified != 1 {
		t.Fatalf("%d of the overlapping pair verified, want exactly 1", verified)
	}
}

// The promotion refuses only another workspace's verified overlap: a
// wildcard above the host at any depth, or for a wildcard any name under its
// base (w4/m326).
func TestPGPromoteDomainClaimRefusesAnotherWorkspacesVerifiedOverlap(t *testing.T) {
	s := newReplayTestStore(t)
	ctx := context.Background()
	fx := newDomainOverlapFixture(t, s)
	for i, tc := range []struct {
		name         string
		held         App
		heldHost     string
		heldVerified bool
		promote      string
		wantCovered  bool
	}{
		{name: "a name under a verified foreign wildcard", held: fx.b, heldHost: "*.%s", heldVerified: true, promote: "api.%s", wantCovered: true},
		{name: "a name two labels under it", held: fx.b, heldHost: "*.%s", heldVerified: true, promote: "login.eu.%s", wantCovered: true},
		{name: "a wildcard over a verified foreign name", held: fx.b, heldHost: "a.b.%s", heldVerified: true, promote: "*.%s", wantCovered: true},
		{name: "a wildcard over a verified foreign wildcard", held: fx.b, heldHost: "*.eu.%s", heldVerified: true, promote: "*.%s", wantCovered: true},
		{name: "a wildcard under a verified foreign wildcard", held: fx.b, heldHost: "*.%s", heldVerified: true, promote: "*.eu.%s", wantCovered: true},
		{name: "a pending foreign wildcard", held: fx.b, heldHost: "*.%s", promote: "api.%s"},
		{name: "the same workspace's wildcard", held: fx.sameWorkspace, heldHost: "*.%s", heldVerified: true, promote: "api.%s"},
		{name: "the base a wildcard does not cover", held: fx.b, heldHost: "%s", heldVerified: true, promote: "*.%s"},
		{name: "a wildcard on a sibling domain", held: fx.b, heldHost: "*.other-%s", heldVerified: true, promote: "api.%s"},
	} {
		base := fmt.Sprintf("c%d-%s.example.com", i, fx.suffix)
		held := addPendingClaim(t, s, tc.held.ID, fmt.Sprintf(tc.heldHost, base))
		if tc.heldVerified {
			if _, _, err := s.PromoteDomainClaim(ctx, held.AppID, held.ID, time.Now()); err != nil {
				t.Fatalf("%s: promote the held claim: %v", tc.name, err)
			}
		}
		claim := addPendingClaim(t, s, fx.a.ID, fmt.Sprintf(tc.promote, base))
		_, promoted, err := s.PromoteDomainClaim(ctx, claim.AppID, claim.ID, time.Now())
		row, getErr := s.GetDomainClaim(ctx, claim.AppID, claim.Host)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if tc.wantCovered {
			if !errors.Is(err, ErrDomainCovered) || row.ClaimState != "pending" || row.VerificationAttempts != 0 {
				t.Errorf("%s: promote %s = promoted %v, %v, state %s attempts %d; want ErrDomainCovered and pending untouched",
					tc.name, claim.Host, promoted, err, row.ClaimState, row.VerificationAttempts)
			}
		} else if err != nil || !promoted || row.ClaimState != "verified" {
			t.Errorf("%s: promote %s = promoted %v, %v, state %s; want verified", tc.name, claim.Host, promoted, err, row.ClaimState)
		}
	}
}

// ForeignWildcardCovers finds another workspace's wildcard over a name at any
// depth, pending or verified, and never the name's own workspace's (w4/m326).
func TestPGForeignWildcardCovers(t *testing.T) {
	s := newReplayTestStore(t)
	ctx := context.Background()
	fx := newDomainOverlapFixture(t, s)
	base := "cover-" + fx.suffix + ".example.com"
	wildcard := addPendingClaim(t, s, fx.b.ID, "*."+base)
	ownWildcard := addPendingClaim(t, s, fx.sameWorkspace.ID, "*.own-"+base)
	check := func(state string) {
		t.Helper()
		for _, tc := range []struct {
			workspace, host string
			want            bool
		}{
			{fx.a.TenantID, "api." + base, true},
			{fx.a.TenantID, "login.eu." + base, true},
			{fx.b.TenantID, "api." + base, false},
			{fx.a.TenantID, base, false},
			{fx.a.TenantID, "api.own-" + base, false},
			{fx.b.TenantID, "api.own-" + base, true},
			{fx.a.TenantID, "api.other-" + base, false},
		} {
			got, err := s.ForeignWildcardCovers(ctx, tc.workspace, tc.host)
			if err != nil || got != tc.want {
				t.Errorf("%s wildcards: ForeignWildcardCovers(%s, %s) = %v, %v; want %v", state, tc.workspace, tc.host, got, err, tc.want)
			}
		}
	}
	check("pending")
	for _, claim := range []Domain{wildcard, ownWildcard} {
		if _, _, err := s.PromoteDomainClaim(ctx, claim.AppID, claim.ID, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	check("verified")
}
