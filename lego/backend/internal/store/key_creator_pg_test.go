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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// key_creator_pg_test.go is w9/m481's store half against REAL Postgres: an
// API key is seated at its creator's role when minted, follows that role in
// the same transaction as every change to it, and a key bound before the
// creator column existed takes its creator once at startup.
//
// Additive: subjects are unique per run, and it never truncates.

func keyCreatorStore(t *testing.T) *PGStore {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return NewPGStore(pool)
}

func boundRole(t *testing.T, st *PGStore, tenantID, subject string) (role, creator string) {
	t.Helper()
	var by *string
	if err := st.Pool.QueryRow(context.Background(),
		`SELECT role, created_by FROM tenant_members WHERE tenant_id = $1 AND subject = $2 AND kind = 'machine'`,
		tenantID, subject).Scan(&role, &by); err != nil {
		t.Fatalf("read %s's binding: %v", subject, err)
	}
	if by != nil {
		creator = *by
	}
	return role, creator
}

func queuedRole(t *testing.T, st *PGStore, tenantID, subject string) string {
	t.Helper()
	var role string
	err := st.Pool.QueryRow(context.Background(),
		`SELECT role FROM membership_role_reconciliations WHERE tenant_id = $1 AND subject = $2`,
		tenantID, subject).Scan(&role)
	if err != nil {
		return ""
	}
	return role
}

func TestBindClientForCreatorSeatsTheCreatorsRole(t *testing.T) {
	st := keyCreatorStore(t)
	ctx := context.Background()
	run := uniqueMachineRun()
	admin, dev := "admin-"+run, "dev-"+run
	ws, err := st.CreateWorkspace(ctx, "keyrole-"+run, PlanScale, admin)
	if err != nil {
		t.Fatal(err)
	}
	// Every member role mints a key, seated at that role (w9/m514).
	seated := map[string]string{admin: "admin", dev: "developer"}
	for _, role := range []string{"developer", "contributor", "viewer", "billing"} {
		subject := role + "-" + run
		if role == "developer" {
			subject = dev
		}
		if err := st.AddMember(ctx, subject, ws.ID, role); err != nil {
			t.Fatal(err)
		}
		seated[subject] = role
	}

	for creator, want := range seated {
		key := "key-" + creator
		got, err := st.BindClientForCreator(ctx, key, ws.ID, creator)
		if err != nil || got != want {
			t.Fatalf("BindClientForCreator(%s) = %q, %v, want %q", creator, got, err, want)
		}
		if role, by := boundRole(t, st, ws.ID, key); role != want || by != creator {
			t.Errorf("%s's key row = %s by %q, want %s by %s", creator, role, by, want, creator)
		}
	}

	// A stranger is not a member, and another key is not a person: neither
	// binds anything.
	for _, creator := range []string{"stranger-" + run, "key-" + admin} {
		key := "refused-key-" + creator
		if _, err := st.BindClientForCreator(ctx, key, ws.ID, creator); !errors.Is(err, ErrNotFound) {
			t.Errorf("BindClientForCreator(%s) = %v, want ErrNotFound", creator, err)
		}
		if member, _ := st.IsMember(ctx, key, ws.ID); member {
			t.Errorf("a refused mint for %s bound its key", creator)
		}
	}

	// A creator whose own role change has not converged yet cannot mint.
	if err := st.UpdateMemberRole(ctx, ws.ID, dev, "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.BindClientForCreator(ctx, "unsettled-key-"+run, ws.ID, dev); !errors.Is(err, ErrRoleUnsettled) {
		t.Fatalf("mint while the creator's role is queued = %v, want ErrRoleUnsettled", err)
	}
	if member, _ := st.IsMember(ctx, "unsettled-key-"+run, ws.ID); member {
		t.Error("a refused mint bound its key")
	}
}

func TestRoleChangeReseatsTheCreatorsKeysInTheSameTransaction(t *testing.T) {
	st := keyCreatorStore(t)
	ctx := context.Background()
	run := uniqueMachineRun()
	owner, admin, other := "owner-"+run, "admin-"+run, "other-"+run
	ws, err := st.CreateWorkspace(ctx, "follow-"+run, PlanPro, owner)
	if err != nil {
		t.Fatal(err)
	}
	elsewhere, err := st.CreateWorkspace(ctx, "elsewhere-"+run, PlanPro, admin)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []struct{ subject, role string }{{admin, "admin"}, {other, "admin"}} {
		if err := st.AddMember(ctx, m.subject, ws.ID, m.role); err != nil {
			t.Fatal(err)
		}
	}
	keyA1, keyA2, keyOther, keyElsewhere := "ka1-"+run, "ka2-"+run, "kother-"+run, "kelse-"+run
	for _, k := range []struct{ key, creator, ws string }{
		{keyA1, admin, ws.ID}, {keyA2, admin, ws.ID}, {keyOther, other, ws.ID}, {keyElsewhere, admin, elsewhere.ID},
	} {
		if _, err := st.BindClientForCreator(ctx, k.key, k.ws, k.creator); err != nil {
			t.Fatalf("bind %s: %v", k.key, err)
		}
	}

	for _, role := range []string{"developer", "admin", "viewer"} {
		if err := st.UpdateMemberRole(ctx, ws.ID, admin, role); err != nil {
			t.Fatalf("change role to %s: %v", role, err)
		}
		for _, key := range []string{keyA1, keyA2} {
			if got, _ := boundRole(t, st, ws.ID, key); got != role {
				t.Errorf("after the creator became %s, %s's row = %s", role, key, got)
			}
			if got := queuedRole(t, st, ws.ID, key); got != role {
				t.Errorf("after the creator became %s, %s's queued intent = %q", role, key, got)
			}
		}
		// Another member's key, and the creator's key in another workspace,
		// do not move.
		if got, _ := boundRole(t, st, ws.ID, keyOther); got != "admin" {
			t.Errorf("another member's key moved to %s", got)
		}
		if got, _ := boundRole(t, st, elsewhere.ID, keyElsewhere); got != "admin" {
			t.Errorf("the creator's key in another workspace moved to %s", got)
		}
	}
	keys, err := st.CreatorKeyBindings(ctx, ws.ID, admin)
	if err != nil || len(keys) != 2 {
		t.Fatalf("CreatorKeyBindings = %+v, %v, want the creator's two keys here", keys, err)
	}
	for _, k := range keys {
		if k.Role != "viewer" || k.Kind != MemberKindMachine {
			t.Errorf("CreatorKeyBindings row %+v, want a viewer machine row", k)
		}
	}
}

func TestRecordKeyCreatorsSeatsLegacyKeysOnce(t *testing.T) {
	st := keyCreatorStore(t)
	ctx := context.Background()
	run := uniqueMachineRun()
	admin, gone := "admin-"+run, "gone-"+run
	ws, err := st.CreateWorkspace(ctx, "legacy-"+run, PlanPro, admin)
	if err != nil {
		t.Fatal(err)
	}
	legacy, orphan, unstamped := "legacy-"+run, "orphan-"+run, "unstamped-"+run
	for _, key := range []string{legacy, orphan, unstamped} {
		if err := st.BindClient(ctx, key, ws.ID); err != nil {
			t.Fatal(err)
		}
	}

	seated, err := st.RecordKeyCreators(ctx, map[string]string{legacy: admin, orphan: gone, unstamped: ""})
	if err != nil || seated != 1 {
		t.Fatalf("RecordKeyCreators = %d, %v, want the one key whose creator is here", seated, err)
	}
	if role, by := boundRole(t, st, ws.ID, legacy); role != "admin" || by != admin {
		t.Errorf("legacy key = %s by %q, want admin by its creator", role, by)
	}
	if got := queuedRole(t, st, ws.ID, legacy); got != "admin" {
		t.Errorf("legacy key's queued intent = %q, want admin", got)
	}
	// A key whose creator left keeps its binding, now attributed, so the
	// request-time ceiling refuses it; a key with no stamp stays a Developer.
	if role, by := boundRole(t, st, ws.ID, orphan); role != "developer" || by != gone {
		t.Errorf("orphaned key = %s by %q, want developer by its departed creator", role, by)
	}
	if role, by := boundRole(t, st, ws.ID, unstamped); role != "developer" || by != "" {
		t.Errorf("unstamped key = %s by %q, want an unattributed developer", role, by)
	}
	// Idempotent: a second start changes nothing.
	if again, err := st.RecordKeyCreators(ctx, map[string]string{legacy: admin, orphan: gone}); err != nil || again != 0 {
		t.Errorf("second RecordKeyCreators = %d, %v, want 0", again, err)
	}
	// A person's row can never carry a creator.
	if _, err := st.Pool.Exec(ctx, `UPDATE tenant_members SET created_by = 'x' WHERE tenant_id = $1 AND subject = $2`, ws.ID, admin); err == nil {
		t.Error("a person's row took a created_by")
	}
}
