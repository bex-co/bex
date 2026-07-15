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
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func TestPGStoreSSHKeysAndSessionAudit(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	st := NewPGStore(pool)

	keyID := ids.New(ids.SSHKey)
	duplicateID := ids.New(ids.SSHKey)
	subject := "ssh-pg-test-" + strings.TrimPrefix(keyID, "ssk-")
	fingerprint := "SHA256:" + strings.TrimPrefix(keyID, "ssk-")
	oldSessionID := ids.New(ids.SSHSession)
	currentSessionID := ids.New(ids.SSHSession)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ssh_sessions WHERE id = ANY($1)`, []string{oldSessionID, currentSessionID})
		_, _ = pool.Exec(context.Background(), `DELETE FROM ssh_keys WHERE id = ANY($1)`, []string{keyID, duplicateID})
		pool.Close()
	})

	created, err := st.CreateSSHKey(ctx, SSHKey{
		ID: keyID, Subject: subject, Name: "workstation",
		PublicKey: "ssh-ed25519 AAAATEST", Fingerprint: fingerprint,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != keyID || created.Subject != subject || created.CreatedAt.IsZero() {
		t.Fatalf("created SSH key = %+v", created)
	}
	if _, err := st.CreateSSHKey(ctx, SSHKey{
		ID: duplicateID, Subject: "another-subject", Name: "duplicate",
		PublicKey: "ssh-ed25519 AAAATEST", Fingerprint: fingerprint,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate fingerprint = %v, want ErrConflict", err)
	}
	keys, err := st.ListSSHKeys(ctx, subject)
	if err != nil || len(keys) != 1 || keys[0].ID != keyID {
		t.Fatalf("identity key list = %+v, %v", keys, err)
	}
	lookup, err := st.SSHKeyByFingerprint(ctx, fingerprint)
	if err != nil || lookup.Subject != subject {
		t.Fatalf("fingerprint lookup = %+v, %v", lookup, err)
	}
	if err := st.DeleteSSHKey(ctx, "foreign-subject", keyID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign key delete = %v, want ErrNotFound", err)
	}

	now := time.Now().UTC()
	for _, session := range []SSHSessionAudit{
		{
			ID: oldSessionID, Subject: subject,
			WorkspaceID: ids.New(ids.Workspace), ServiceID: ids.New(ids.Service),
			InstanceID: "srv-test-pod01", RemoteAddress: "127.0.0.1:22001",
			StartedAt: now.Add(-48 * time.Hour),
		},
		{
			ID: currentSessionID, Subject: subject,
			WorkspaceID: ids.New(ids.Workspace), ServiceID: ids.New(ids.Service),
			InstanceID: "srv-test-pod02", RemoteAddress: "127.0.0.1:22002",
			StartedAt: now,
		},
	} {
		if err := st.StartSSHSession(ctx, session); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.EndSSHSession(ctx, currentSessionID, "completed", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var result string
	var endedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT result, ended_at FROM ssh_sessions WHERE id = $1`, currentSessionID).Scan(&result, &endedAt); err != nil {
		t.Fatal(err)
	}
	if result != "completed" || endedAt == nil {
		t.Fatalf("ended SSH session = result %q, ended_at %v", result, endedAt)
	}
	if _, err := st.PurgeSSHSessions(ctx, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	var oldCount, currentCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ssh_sessions WHERE id = $1`, oldSessionID).Scan(&oldCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ssh_sessions WHERE id = $1`, currentSessionID).Scan(&currentCount); err != nil {
		t.Fatal(err)
	}
	if oldCount != 0 || currentCount != 1 {
		t.Fatalf("retained SSH sessions old/current = %d/%d, want 0/1", oldCount, currentCount)
	}
	if err := st.DeleteSSHKey(ctx, subject, keyID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SSHKeyByFingerprint(ctx, fingerprint); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted fingerprint lookup = %v, want ErrNotFound", err)
	}
}
