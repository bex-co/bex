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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// w21/m394 t013 (R25-10): an unverified claim is reclaimed 48 hours after it was
// made, but only the row's own created_at counted, so deleting the claim and
// adding it again started the 48 hours over, and the victim stayed blocked at
// "first claim + 50 h". The Blueprint and service-create paths did not reclaim
// an expired claim at all.

type claimExpiryFixture struct {
	s      *PGStore
	pool   *pgxpool.Pool
	tenant string
	apps   map[string]string // label -> app id
}

func newClaimExpiryFixture(t *testing.T, labels ...string) *claimExpiryFixture {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE domain_claim_history`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	tenant, err := s.CreateTenantWithMember(ctx, "identity-claim-expiry", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	f := &claimExpiryFixture{s: s, pool: pool, tenant: tenant.ID, apps: map[string]string{}}
	for _, label := range labels {
		f.apps[label] = f.newApp(t, label)
	}
	return f
}

func (f *claimExpiryFixture) newApp(t *testing.T, name string) string {
	t.Helper()
	app, err := f.s.CreateApp(context.Background(), App{
		TenantID: f.tenant, Name: name, Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	return app.ID
}

// age moves a host's claim back in time by d, as if it had been made earlier.
func (f *claimExpiryFixture) age(t *testing.T, host string, d time.Duration) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE domains SET created_at = created_at - $2::interval WHERE host = $1`, host, d.String()); err != nil {
		t.Fatal(err)
	}
}

func (f *claimExpiryFixture) createdAt(t *testing.T, host string) time.Time {
	t.Helper()
	var at time.Time
	if err := f.pool.QueryRow(context.Background(), `SELECT created_at FROM domains WHERE host = $1`, host).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func TestAddDomainClaimDeleteReaddKeepsExpiry(t *testing.T) {
	ctx := context.Background()
	for _, remove := range []string{"DeleteDomain", "RemoveDomain", "DeleteApp"} {
		t.Run(remove, func(t *testing.T) {
			f := newClaimExpiryFixture(t, "attacker", "victim")
			host := "squat-" + remove + ".example.com"
			if _, _, err := f.s.AddDomainClaim(ctx, f.apps["attacker"], host, ""); err != nil {
				t.Fatal(err)
			}
			f.age(t, host, 47*time.Hour)

			readder := f.apps["attacker"]
			switch remove {
			case "DeleteDomain":
				if err := f.s.DeleteDomain(ctx, f.apps["attacker"], host); err != nil {
					t.Fatal(err)
				}
			case "RemoveDomain":
				if err := f.s.RemoveDomain(ctx, f.apps["attacker"], host); err != nil {
					t.Fatal(err)
				}
			case "DeleteApp": // the cascade deletes the row too; a new service re-adds it
				if err := f.s.DeleteApp(ctx, f.apps["attacker"]); err != nil {
					t.Fatal(err)
				}
				readder = f.newApp(t, "attacker-again")
			}
			if _, created, err := f.s.AddDomainClaim(ctx, readder, host, ""); err != nil || !created {
				t.Fatalf("re-add = created %v, %v", created, err)
			}
			// 47 h after the first claim the re-added row is 1 h from expiry,
			// not 48 h: three more hours and the victim reclaims the host.
			f.age(t, host, 3*time.Hour)
			if _, _, err := f.s.AddDomainClaim(ctx, f.apps["victim"], host, ""); err != nil {
				t.Fatalf("victim's claim 50 h after the first = %v, want it to reclaim the squatted host", err)
			}
		})
	}
}

// A legitimate owner who deletes and re-adds their own claim is not locked out,
// and the re-added claim still protects the host from other apps inside the
// window.
func TestOwnerCanDeleteAndReaddAClaimInsideTheWindow(t *testing.T) {
	ctx := context.Background()
	f := newClaimExpiryFixture(t, "owner", "other")
	const host = "typo-fix.example.com"
	first, _, err := f.s.AddDomainClaim(ctx, f.apps["owner"], host, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.DeleteDomain(ctx, f.apps["owner"], host); err != nil {
		t.Fatal(err)
	}
	again, created, err := f.s.AddDomainClaim(ctx, f.apps["owner"], host, "")
	if err != nil || !created {
		t.Fatalf("owner's re-add = created %v, %v", created, err)
	}
	if !again.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("re-added claim created_at = %v, want the first claim's %v", again.CreatedAt, first.CreatedAt)
	}
	if _, _, err := f.s.AddDomainClaim(ctx, f.apps["other"], host, ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("another app's claim inside the window = %v, want ErrConflict", err)
	}
}

// The first claim time carries over a gap of under 24 hours, while it is still
// inside its own 48; past either bound the next claim starts afresh.
func TestClaimHistoryIsBounded(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name      string
		gap       string // how long ago the delete was recorded
		firstAge  time.Duration
		wantFresh bool
	}{
		{"recent delete inherits", "1 hour", 10 * time.Hour, false},
		{"delete over 24 hours ago starts afresh", "25 hours", 10 * time.Hour, true},
		{"first claim over 48 hours ago starts afresh", "1 hour", 49 * time.Hour, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newClaimExpiryFixture(t, "owner")
			const host = "bounded.example.com"
			if _, _, err := f.s.AddDomainClaim(ctx, f.apps["owner"], host, ""); err != nil {
				t.Fatal(err)
			}
			if err := f.s.DeleteDomain(ctx, f.apps["owner"], host); err != nil {
				t.Fatal(err)
			}
			// Age the recorded claim: it began firstAge ago and was deleted gap ago.
			if _, err := f.pool.Exec(ctx,
				`UPDATE domain_claim_history SET since = now() - $2::interval, recorded_at = now() - $3::interval WHERE host = $1`,
				host, tc.firstAge.String(), tc.gap); err != nil {
				t.Fatal(err)
			}
			if _, _, err := f.s.AddDomainClaim(ctx, f.apps["owner"], host, ""); err != nil {
				t.Fatal(err)
			}
			age := time.Since(f.createdAt(t, host))
			if tc.wantFresh && age > time.Minute {
				t.Fatalf("claim is %v old, want a fresh one", age)
			}
			if !tc.wantFresh && age < tc.firstAge-time.Minute {
				t.Fatalf("claim is %v old, want the first claim's %v", age, tc.firstAge)
			}
			var n int
			if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM domain_claim_history WHERE host = $1`, host).Scan(&n); err != nil || n != 0 {
				t.Fatalf("history rows after the re-add = %d, %v, want 0 (consumed)", n, err)
			}
		})
	}
}

// A verified claim is never reclaimed and leaves no history.
func TestVerifiedClaimLeavesNoHistory(t *testing.T) {
	ctx := context.Background()
	f := newClaimExpiryFixture(t, "owner")
	if err := f.s.AddDomain(ctx, f.apps["owner"], "verified.example.com", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.s.RemoveDomain(ctx, f.apps["owner"], "verified.example.com"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM domain_claim_history`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("history rows after deleting a verified claim = %d, %v, want 0", n, err)
	}
}

// The Blueprint update path reclaimed nothing: another app's pending claim
// older than 48 hours answered ErrConflict for ever.
func TestReplaceDomainClaimsReclaimsExpiredPendingSquat(t *testing.T) {
	ctx := context.Background()
	f := newClaimExpiryFixture(t, "squatter", "victim")
	const host = "blueprint-squat.example.com"
	if _, _, err := f.s.AddDomainClaim(ctx, f.apps["squatter"], host, ""); err != nil {
		t.Fatal(err)
	}
	decl := []DomainDeclaration{{Host: host, Primary: true}}

	f.age(t, host, 1*time.Hour)
	if _, err := f.s.ReplaceDomainClaims(ctx, f.apps["victim"], decl); !errors.Is(err, ErrConflict) {
		t.Fatalf("victim's replace against a fresh squat = %v, want ErrConflict", err)
	}
	f.age(t, host, 48*time.Hour)
	rows, err := f.s.ReplaceDomainClaims(ctx, f.apps["victim"], decl)
	if err != nil {
		t.Fatalf("victim's replace against a 49 h-old pending squat = %v, want it to reclaim", err)
	}
	if len(rows) != 1 || rows[0].AppID != f.apps["victim"] || rows[0].ClaimState != "pending" {
		t.Fatalf("rows = %+v, want the victim's pending claim", rows)
	}
}

// A verified claim is never reclaimed by a replace, however old.
func TestReplaceDomainClaimsKeepsAVerifiedClaim(t *testing.T) {
	ctx := context.Background()
	f := newClaimExpiryFixture(t, "owner", "other")
	const host = "verified-old.example.com"
	if err := f.s.AddDomain(ctx, f.apps["owner"], host, ""); err != nil {
		t.Fatal(err)
	}
	f.age(t, host, 100*time.Hour)
	if _, err := f.s.ReplaceDomainClaims(ctx, f.apps["other"], []DomainDeclaration{{Host: host, Primary: true}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("replace against a verified claim = %v, want ErrConflict", err)
	}
}

// The service-create plan refuses a host another service claims, but not one
// whose pending claim has expired: the create's own replace reclaims it.
func TestDomainHostsClaimedIgnoresExpiredPendingClaims(t *testing.T) {
	ctx := context.Background()
	f := newClaimExpiryFixture(t, "a")
	for _, host := range []string{"fresh.example.com", "stale.example.com"} {
		if _, _, err := f.s.AddDomainClaim(ctx, f.apps["a"], host, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.s.AddDomain(ctx, f.apps["a"], "verified-stale.example.com", ""); err != nil {
		t.Fatal(err)
	}
	f.age(t, "stale.example.com", 49*time.Hour)
	f.age(t, "verified-stale.example.com", 49*time.Hour)

	got, err := f.s.DomainHostsClaimed(ctx, []string{"fresh.example.com", "stale.example.com", "verified-stale.example.com", "free.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"fresh.example.com", "verified-stale.example.com"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("claimed = %v, want %v", got, want)
	}
}
