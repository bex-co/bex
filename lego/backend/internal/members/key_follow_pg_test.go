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

package members

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// key_follow_pg_test.go is w9/m481 t003's fault half against REAL Postgres and
// REAL OpenFGA: a key whose re-seat failed during its creator's role change is
// left a queued intent, and the role reconciler converges it exactly as it
// converges a member.

type revokeFailer struct {
	RoleRevoker
	mu      sync.Mutex
	subject string
}

func (r *revokeFailer) set(subject string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subject = subject
}

func (r *revokeFailer) RevokeWorkspaceMember(ctx context.Context, tenantID, subject, relation string) error {
	r.mu.Lock()
	fail := r.subject
	r.mu.Unlock()
	if subject == fail {
		return errors.New("openfga: injected revoke failure")
	}
	return r.RoleRevoker.RevokeWorkspaceMember(ctx, tenantID, subject, relation)
}

func TestTheRoleReconcilerConvergesAKeyWhoseReseatFailed(t *testing.T) {
	dbURI, fgaURL := os.Getenv("BEX_TEST_DB_URI"), os.Getenv("BEX_TEST_OPENFGA_URL")
	if dbURI == "" || fgaURL == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI and BEX_TEST_OPENFGA_URL not both set")
	}
	ctx := context.Background()
	if err := store.Migrate(dbURI); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)
	revoker := &revokeFailer{RoleRevoker: roles}

	run := ids.New(ids.Workspace)[4:]
	creator, admin2, key := "creator-"+run, "admin2-"+run, "key-"+run
	ws, err := st.CreateWorkspace(ctx, "follow-"+run, store.PlanPro, creator)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddMember(ctx, admin2, ws.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{creator, admin2} {
		if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+s, "admin"); err != nil {
			t.Fatal(err)
		}
	}
	if role, err := st.BindClientForCreator(ctx, key, ws.ID, creator); err != nil || role != "admin" {
		t.Fatalf("bind: %q %v", role, err)
	}
	if err := roles.GrantWorkspaceRole(ctx, ws.ID, "user:"+key, "admin"); err != nil {
		t.Fatal(err)
	}
	held := func(subject, relation string) bool {
		t.Helper()
		ok, err := checker.(core.FreshChecker).CheckFresh(ctx, "user:"+subject, relation, core.WorkspaceObject(ws.ID))
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}

	s := &Service{Base: &core.Base{Authz: checker}, Store: st, Granter: roles, Revoker: revoker}
	revoker.set("user:" + key)
	actor := core.WithIdentity(ctx, core.Identity{Subject: admin2, Method: core.MethodSession})
	if _, err := s.ChangeRole(actor, ws.ID, creator, "developer"); err != nil {
		t.Fatalf("the creator's demotion must land even when their key's re-seat fails: %v", err)
	}
	if !held(creator, "developer") || held(creator, "admin") {
		t.Fatal("the creator's own tuples did not move")
	}
	if !held(key, "admin") {
		t.Fatal("the injected failure did not strand the key's admin tuple")
	}
	if pending, err := st.HasPendingRoleReconciliation(ctx, ws.ID, key); err != nil || !pending {
		t.Fatalf("no intent queued for the key: %v %v", pending, err)
	}

	// OpenFGA recovers; once the intent's backoff is due the reconciler
	// converges the key to exactly developer and acknowledges it.
	revoker.set("")
	if _, err := pool.Exec(ctx, `UPDATE membership_role_reconciliations SET next_attempt_at = now() WHERE tenant_id = $1`, ws.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.reconcilePendingRoles(ctx, 500); err != nil {
		t.Fatal(err)
	}
	if held(key, "admin") || !held(key, "developer") {
		t.Fatal("the reconciler did not converge the key to its creator's role")
	}
	if pending, _ := st.HasPendingRoleReconciliation(ctx, ws.ID, key); pending {
		t.Fatal("the converged key's intent is still queued")
	}
}
