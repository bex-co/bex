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

// The workspace notification policy (w3/m271): no row until first written, the
// first partial write fills Render's default for the field it does not name, a
// later partial write keeps the other field, the CHECK refuses a level outside
// none|failure|all, and the row goes with its workspace.
func TestNotificationOwnerPolicyDefaultsWriteAndReadBack(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	tenant, err := st.CreateTenant(ctx, "notification-owner-policy", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })

	if _, err := st.GetNotificationOwnerPolicy(ctx, tenant.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unwritten policy error = %v, want ErrNotFound", err)
	}
	all := "all"
	got, err := st.UpsertNotificationOwnerPolicy(ctx, tenant.ID, NotificationOwnerPolicyPatch{NotificationsToSend: &all})
	if err != nil {
		t.Fatal(err)
	}
	if !got.EmailEnabled || got.NotificationsToSend != all || got.UpdatedAt.IsZero() {
		t.Fatalf("first partial write = %+v, want email on (Render's default) and level all", got)
	}
	off := false
	if got, err = st.UpsertNotificationOwnerPolicy(ctx, tenant.ID, NotificationOwnerPolicyPatch{EmailEnabled: &off}); err != nil {
		t.Fatal(err)
	}
	if got.EmailEnabled || got.NotificationsToSend != all {
		t.Fatalf("second partial write = %+v, lost the level", got)
	}
	read, err := st.GetNotificationOwnerPolicy(ctx, tenant.ID)
	if err != nil || read.EmailEnabled || read.NotificationsToSend != all || read.WorkspaceID != tenant.ID {
		t.Fatalf("read back = %+v err=%v", read, err)
	}

	for _, bad := range []string{"default", "everything", ""} {
		if _, err := st.UpsertNotificationOwnerPolicy(ctx, tenant.ID, NotificationOwnerPolicyPatch{NotificationsToSend: &bad}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("level %q error = %v, want ErrInvalid", bad, err)
		}
	}
	if read, _ = st.GetNotificationOwnerPolicy(ctx, tenant.ID); read.NotificationsToSend != all {
		t.Fatalf("a refused write changed the level to %q", read.NotificationsToSend)
	}
	if _, err := st.UpsertNotificationOwnerPolicy(ctx, "tea-00000000000000000000", NotificationOwnerPolicyPatch{NotificationsToSend: &all}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown workspace error = %v, want ErrNotFound", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetNotificationOwnerPolicy(ctx, tenant.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("policy survived its workspace: %v", err)
	}
}
