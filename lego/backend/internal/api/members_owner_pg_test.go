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
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// members_owner_pg_test.go proves, against REAL Postgres, the workspace
// creator's removal and demotion (w5/m372, after w5/m101) over the REAL chain:
// members.Service over PGStore, then the onboarding path
// (tenantService.EnsureTenant) the auth gate runs on every session request
// (internal/api/auth.go).
//
// Onboarding resolves the personal workspace by tenants.owner_identity_id and
// re-grants workspace admin on every cache miss. w5/m101 refused the creator's
// removal because, with the binding left in place, a removal either never
// stuck (admin resurrected) or stranded the account. The store now ends the
// binding in the same transaction as the membership, so neither can happen:
// the removed creator's next session lands in a fresh personal workspace.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	  go test ./internal/api -run 'TestRemovingTheCreator|TestDemotingTheCreator|TestConcurrentRemovals' -v
//
// Like the other internal/api real-Postgres tests this is ADDITIVE: it never
// truncates (packages run concurrently against the same throwaway database),
// so every subject and workspace name is unique to the run.
func TestRemovingTheCreatorNeitherRestoresAdminNorStrandsTheAccount(t *testing.T) {
	ctx, pool, st := ownerGuardPG(t)
	run := uniqueRunSuffix()
	owner, second := "owner-"+run, "admin-"+run

	tenant, err := st.CreateTenantWithMember(ctx, owner, store.PlanPro)
	if err != nil {
		t.Fatalf("mint personal workspace: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, 'admin')`,
		tenant.ID, second); err != nil {
		t.Fatalf("seed second admin: %v", err)
	}

	granter := &recordingGranter{}
	ms := &members.Service{Base: &core.Base{}, Store: st, Granter: granter, Revoker: granter}

	// 1. Another admin removes the creator, as Render allows (w5/m372): the row
	//    and the onboarding binding end in one transaction.
	if err := ms.Remove(core.WithIdentity(ctx, core.Identity{Subject: second, Method: "session"}), tenant.ID, owner); err != nil {
		t.Fatalf("remove the creator: %v", err)
	}
	if _, err := st.GetTenantMember(ctx, tenant.ID, owner); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("creator's row after removal: %v, want gone", err)
	}
	if b, err := st.TenantOwnerSubject(ctx, tenant.ID); err != nil || b != "" {
		t.Fatalf("owner binding after removal = %q (%v), want none", b, err)
	}

	// 2. The creator's next sessions land in one fresh personal workspace and
	//    are never re-granted admin on the old one (the resurrection m101
	//    refused removal to avoid). Each session uses a fresh tenant service,
	//    so every request reads the store rather than a cached answer.
	var landed string
	for i := range 3 {
		got, err := NewTenantService(st, granter).EnsureTenant(ctx, owner, "", false)
		if err != nil {
			t.Fatalf("session %d: EnsureTenant for the removed creator: %v", i+1, err)
		}
		if got == tenant.ID {
			t.Fatalf("session %d resolved the removed creator back to %q", i+1, tenant.ID)
		}
		if landed != "" && got != landed {
			t.Fatalf("session %d landed in %q, session 1 in %q: one personal workspace, not one per session", i+1, got, landed)
		}
		landed = got
	}
	if granter.granted(tenant.ID, "user:"+owner) {
		t.Error("onboarding re-granted the removed creator admin on the old workspace")
	}
	if _, err := st.GetTenantMember(ctx, tenant.ID, owner); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("creator's row after three sessions: %v, want still gone", err)
	}
	if !granter.granted(landed, "user:"+owner) {
		t.Errorf("the creator has no admin on their new personal workspace %q", landed)
	}
}

// Demoting the creator ends the binding too, so onboarding cannot restore
// their admin tuple; the store does it under the same lock as the role write,
// whether or not the members service ran first.
func TestDemotingTheCreatorInTheStoreEndsTheBinding(t *testing.T) {
	ctx, pool, st := ownerGuardPG(t)
	run := uniqueRunSuffix()
	owner, second := "owner-"+run, "admin-"+run

	tenant, err := st.CreateTenantWithMember(ctx, owner, store.PlanPro)
	if err != nil {
		t.Fatalf("mint personal workspace: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, 'admin')`,
		tenant.ID, second); err != nil {
		t.Fatalf("seed second admin: %v", err)
	}
	if err := st.UpdateMemberRole(ctx, tenant.ID, owner, "viewer"); err != nil {
		t.Fatalf("demote the creator: %v", err)
	}
	if b, err := st.TenantOwnerSubject(ctx, tenant.ID); err != nil || b != "" {
		t.Fatalf("owner binding after demotion = %q (%v), want none", b, err)
	}
	if m, err := st.GetTenantMember(ctx, tenant.ID, owner); err != nil || m.Role != "viewer" {
		t.Fatalf("creator after demotion = %+v (%v), want a viewer", m, err)
	}
}

// TestConcurrentRemovalsCannotStripTheLastAdmin drives the in-transaction
// last-admin rule under the membership advisory lock: two admins removing each
// other at the same instant, in a workspace whose creator is one of them.
// Whatever interleaving wins, exactly one admin remains, and if the creator was
// the one removed the binding went with them.
func TestConcurrentRemovalsCannotStripTheLastAdmin(t *testing.T) {
	ctx, pool, st := ownerGuardPG(t)
	run := uniqueRunSuffix()
	owner, second := "owner-"+run, "admin-"+run

	tenant, err := st.CreateTenantWithMember(ctx, owner, store.PlanPro)
	if err != nil {
		t.Fatalf("mint personal workspace: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, 'admin')`,
		tenant.ID, second); err != nil {
		t.Fatalf("seed second admin: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = st.RemoveMember(ctx, tenant.ID, owner) }()
	go func() { defer wg.Done(); _ = st.RemoveMember(ctx, tenant.ID, second) }()
	wg.Wait()

	admins, err := st.CountTenantAdmins(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("count admins: %v", err)
	}
	if admins != 1 {
		t.Fatalf("workspace left with %d admins, want exactly 1", admins)
	}
	_, ownerErr := st.GetTenantMember(ctx, tenant.ID, owner)
	b, err := st.TenantOwnerSubject(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("owner binding: %v", err)
	}
	if errors.Is(ownerErr, store.ErrNotFound) && b != "" {
		t.Fatalf("the creator was removed but the binding still names %q", b)
	}
}

func ownerGuardPG(t *testing.T) (context.Context, *pgxpool.Pool, *store.PGStore) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return ctx, pool, store.NewPGStore(pool)
}

// uniqueRunSuffix keeps this additive test's subjects distinct per run (the
// trailing xid chars are the counter/pid, so two runs in the same second still
// differ) — the same convention multiworkspace_e2e_test.go uses.
func uniqueRunSuffix() string {
	full := ids.New(ids.Workspace)
	return full[len(full)-8:]
}

// recordingGranter is the membership granter as a tuple set — enough to observe
// whether onboarding re-asserted the owner's admin grant.
type recordingGranter struct {
	mu     sync.Mutex
	tuples map[string]bool
}

func (g *recordingGranter) put(tenantID, subject, relation string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.tuples == nil {
		g.tuples = map[string]bool{}
	}
	g.tuples[relation+":"+tenantID+":"+subject] = true
}

func (g *recordingGranter) granted(tenantID, subject string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.tuples["admin:"+tenantID+":"+subject]
}

func (g *recordingGranter) GrantWorkspaceAdmin(_ context.Context, tenantID, subject string) error {
	g.put(tenantID, subject, "admin")
	return nil
}

func (g *recordingGranter) GrantWorkspaceMember(_ context.Context, tenantID, subject string) error {
	g.put(tenantID, subject, "developer")
	return nil
}

func (g *recordingGranter) GrantWorkspaceRole(_ context.Context, tenantID, subject, relation string) error {
	g.put(tenantID, subject, relation)
	return nil
}

func (g *recordingGranter) RevokeWorkspaceMember(_ context.Context, tenantID, subject, relation string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.tuples, relation+":"+tenantID+":"+subject)
	return nil
}
