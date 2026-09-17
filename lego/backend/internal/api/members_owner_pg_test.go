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
)

// members_owner_pg_test.go is w5/m101's regression proof against REAL Postgres,
// driving the REAL chain the defect ran through: members.Service.Remove over
// PGStore, then the onboarding path (tenantService.EnsureTenant) the auth gate
// runs on every session request (internal/api/auth.go).
//
// The defect: onboarding resolves the personal workspace by
// tenants.owner_identity_id and re-grants workspace admin on every cache miss,
// so an owner removed through the members surface either (a) has their admin
// tuple resurrected on the next request — the removal never sticks — or (b),
// if that were "fixed" by honoring the removal, is left permanently
// workspace-less, because TenantForOwner keeps finding the workspace and
// CreateTenantWithMember is never reached. Refusing the removal is the only
// answer that is correct in both directions.
//
//	BEX_TEST_DB_URI=postgres://…/postgres?sslmode=disable \
//	  go test ./internal/api -run TestOwnerRemoval -v
//
// Like the other internal/api real-Postgres tests this is ADDITIVE: it never
// truncates (packages run concurrently against the same throwaway database),
// so every subject and workspace name is unique to the run.
func TestOwnerRemovalIsRefusedAndCannotResurrectAdmin(t *testing.T) {
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

	// 1. The refusal, from the surface the Team page drives. The last-admin rule
	//    cannot explain it — there are two admins.
	err = ms.Remove(core.WithIdentity(ctx, core.Identity{Subject: second, Method: "session"}), tenant.ID, owner)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != members.ErrorOwnerCannotBeRemoved {
		t.Fatalf("remove owner: %v, want %s", err, members.ErrorOwnerCannotBeRemoved)
	}
	if _, err := st.GetTenantMember(ctx, tenant.ID, owner); err != nil {
		t.Fatalf("owner membership row gone after a refused removal: %v", err)
	}

	// 2. The store backstop: the same refusal when the service gate is bypassed
	//    entirely (a second adapter, or a future caller reaching PGStore direct).
	if err := st.RemoveMember(ctx, tenant.ID, owner); !errors.Is(err, store.ErrOwnerMember) {
		t.Fatalf("direct store removal: %v, want ErrOwnerMember", err)
	}
	// And demotion, the same resurrection through the other verb.
	if err := st.UpdateMemberRole(ctx, tenant.ID, owner, "viewer"); !errors.Is(err, store.ErrOwnerRole) {
		t.Fatalf("direct store demotion: %v, want ErrOwnerRole", err)
	}

	// 3. The resurrection path is closed because the row never left: onboarding
	//    resolves the owner back to this same workspace and re-asserts admin,
	//    and that grant now agrees with the membership row instead of
	//    contradicting it.
	ten := NewTenantService(st, granter)
	got, err := ten.EnsureTenant(ctx, owner, "", false)
	if err != nil {
		t.Fatalf("EnsureTenant for the owner: %v", err)
	}
	if got != tenant.ID {
		t.Fatalf("EnsureTenant = %q, want the owned workspace %q", got, tenant.ID)
	}
	if !granter.granted(tenant.ID, "user:"+owner) {
		t.Error("onboarding did not re-assert the owner's admin tuple")
	}
	m, err := st.GetTenantMember(ctx, tenant.ID, owner)
	if err != nil || m.Role != "admin" {
		t.Fatalf("row↔tuple disagree: row=%+v err=%v, tuple=admin", m, err)
	}
}

// TestAccountOffboardingStillRemovesTheOwner keeps ADR086's exemption honest:
// account deletion legitimately removes an owner because it clears the owner
// binding in the SAME transaction, leaving no workspace whose owner is a
// deleted account.
func TestAccountOffboardingStillRemovesTheOwner(t *testing.T) {
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

	if err := st.RemoveAccountMember(ctx, tenant.ID, owner); err != nil {
		t.Fatalf("offboard owner: %v", err)
	}
	if _, err := st.GetTenantMember(ctx, tenant.ID, owner); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("offboarded owner still a member: %v", err)
	}
	if binding, err := st.TenantOwnerSubject(ctx, tenant.ID); err != nil || binding != "" {
		t.Fatalf("owner binding = %q err=%v, want cleared", binding, err)
	}
}

// TestConcurrentRemovalsCannotStripTheOwnerOrTheLastAdmin drives both
// in-transaction rules at once under the membership advisory lock: two admins
// removing each other at the same instant, in a workspace whose owner is one of
// them. Whatever interleaving wins, the owner's row survives and at least one
// admin remains.
func TestConcurrentRemovalsCannotStripTheOwnerOrTheLastAdmin(t *testing.T) {
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

	if _, err := st.GetTenantMember(ctx, tenant.ID, owner); err != nil {
		t.Fatalf("owner lost under concurrent removals: %v", err)
	}
	admins, err := st.CountTenantAdmins(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("count admins: %v", err)
	}
	if admins < 1 {
		t.Fatalf("workspace left with %d admins", admins)
	}
}

func ownerGuardPG(t *testing.T) (context.Context, *pgxpool.Pool, *store.PGStore) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
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
