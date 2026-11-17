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

package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// onboarding_fault_e2e_test.go pins w32/m446's OpenFGA side of onboarding
// through the production checker: a Check that fails on a dropped keep-alive
// connection is resent and sends no write, and a Check that errors outright is
// followed by a grant the real OpenFGA accepts for an admin who already has
// the tuple. Before w21/099 that grant was refused as a duplicate, and the
// person got 503 tenant onboarding unavailable.

// erroringCheck is the production granter with every Check failing, as when
// OpenFGA cannot answer a Check but still takes the write that follows.
type erroringCheck struct{ store.MembershipGranter }

func (erroringCheck) Check(context.Context, string, string, string) (bool, error) {
	return false, errFGAReset
}

func TestOnboardingServesAnAdminWhoseCheckErrorsE2E(t *testing.T) {
	dbURI, fgaURL := os.Getenv("BEX_TEST_DB_URI"), os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
	}
	ctx := context.Background()
	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	granter := checker.(store.MembershipGranter)
	full := ids.New(ids.Workspace)
	person := "onboard-fault-" + full[len(full)-8:]

	tid, err := NewTenantService(st, granter).EnsureTenant(ctx, person, "", false)
	if err != nil {
		t.Fatalf("first onboarding: %v", err)
	}
	// A new service, so neither cache can serve the person: the grant path must.
	got, err := NewTenantService(st, erroringCheck{granter}).EnsureTenant(ctx, person, "", false)
	if err != nil || got != tid {
		t.Fatalf("existing admin with an erroring Check = %q %v, want %q served", got, err, tid)
	}
	if ok, err := checker.(core.FreshChecker).CheckFresh(ctx, "user:"+person, "admin", "workspace:"+tid); err != nil || !ok {
		t.Fatalf("admin tuple after the repeated grant: %v %v, want still held", ok, err)
	}
}

// t007 step 1: a Check that the transport resends after a reset, and that then
// says admin, serves the person with no write to OpenFGA.
func TestOnboardingSendsNoWriteWhenAResetCheckIsResent(t *testing.T) {
	var checks, writes atomic.Int32
	fga := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stores":
			_, _ = fmt.Fprint(w, `{"stores":[{"id":"store-1","name":"bex"}]}`)
		case "/stores/store-1/check":
			if checks.Add(1) == 2 {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Errorf("hijack: %v", err)
					return
				}
				_ = conn.(*net.TCPConn).SetLinger(0)
				_ = conn.Close()
				return
			}
			_, _ = fmt.Fprint(w, `{"allowed":true}`)
		case "/stores/store-1/write":
			writes.Add(1)
			_, _ = fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fga.Close)
	st := newFakeTenantStore()
	tenant, err := st.CreateTenantWithMember(context.Background(), faultPerson, store.PlanHobby)
	if err != nil {
		t.Fatal(err)
	}

	// Two services with their own checkers, so neither Check is answered from
	// a cache; they share the pooled connection, as one replica's do.
	for i := range 2 {
		granter := authz.NewOpenFGAChecker(fga.URL, "").(store.MembershipGranter)
		got, err := NewTenantService(st, granter).EnsureTenant(context.Background(), faultPerson, "", false)
		if err != nil || got != tenant.ID {
			t.Fatalf("onboarding %d = %q %v, want %q", i+1, got, err, tenant.ID)
		}
	}
	if got := checks.Load(); got != 3 {
		t.Errorf("checks = %d, want 3 (one reset, resent once)", got)
	}
	if got := writes.Load(); got != 0 {
		t.Errorf("writes = %d, want none for an admin the Check confirmed", got)
	}
}
