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

package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func oidcKeyStore(t *testing.T) (*store.PGStore, *pgxpool.Pool) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), `DELETE FROM oidc_signing_keys`); err != nil {
		t.Fatal(err)
	}
	return store.NewPGStore(pool), pool
}

// TestOIDCBootstrapInsertsExactlyOneKey is ADR116 D6's bootstrap: replicas
// starting together on an empty key set each try to insert their own key, and
// exactly one key is stored, so every replica signs with the same one.
func TestOIDCBootstrapInsertsExactlyOneKey(t *testing.T) {
	st, _ := oidcKeyStore(t)
	ctx := context.Background()
	const replicas = 16
	var wg sync.WaitGroup
	created := make([]bool, replicas)
	errs := make([]error, replicas)
	for i := range replicas {
		wg.Add(1)
		go func() {
			defer wg.Done()
			created[i], errs[i] = st.BootstrapOIDCSigningKey(ctx, fmt.Sprintf("kid-%d", i), []byte{byte(i)})
		}()
	}
	wg.Wait()
	winners := 0
	for i := range replicas {
		if errs[i] != nil {
			t.Fatalf("replica %d: %v", i, errs[i])
		}
		if created[i] {
			winners++
		}
	}
	keys, err := st.OIDCSigningKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if winners != 1 || len(keys) != 1 {
		t.Fatalf("%d replicas reported an insert and %d keys are stored, want 1 and 1", winners, len(keys))
	}
	if !keys[0].ActivatesAt.Equal(keys[0].CreatedAt) {
		t.Errorf("the bootstrap key activates at %v, created %v; want it signing at once", keys[0].ActivatesAt, keys[0].CreatedAt)
	}
	if again, err := st.BootstrapOIDCSigningKey(ctx, "kid-late", []byte{9}); err != nil || again {
		t.Fatalf("a bootstrap over a non-empty set = %v, %v; want no insert", again, err)
	}
}

// TestOIDCBootstrapWaitsForAWriterHoldingTheKeySet makes the race of the test
// above deterministic: while another replica's bootstrap holds the key-set
// lock and has inserted its key, this one waits, then sees that key and adds
// none.
func TestOIDCBootstrapWaitsForAWriterHoldingTheKeySet(t *testing.T) {
	st, pool := oidcKeyStore(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('oidc-signing-keys', 0))`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO oidc_signing_keys (kid, private_key, activates_at) VALUES ('other-replica', '\x00', now())`); err != nil {
		t.Fatal(err)
	}
	type result struct {
		created bool
		err     error
	}
	done := make(chan result, 1)
	go func() {
		created, err := st.BootstrapOIDCSigningKey(ctx, "this-replica", []byte{1})
		done <- result{created, err}
	}()
	select {
	case r := <-done:
		t.Fatalf("bootstrap finished (%+v) while another writer held the key set", r)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	r := <-done
	if r.err != nil || r.created {
		t.Fatalf("bootstrap after the other replica's insert = %+v, want no insert", r)
	}
	if keys, err := st.OIDCSigningKeys(ctx); err != nil || len(keys) != 1 || keys[0].KID != "other-replica" {
		t.Fatalf("key set = %+v, %v; want only the other replica's key", keys, err)
	}
}

// TestOIDCRotationIsOneAtATime: a rotation stores a key that signs after the
// lead (database time), a second one is refused while the first is pending,
// an emergency rotation replaces everything, and pruning deletes by kid.
func TestOIDCRotationIsOneAtATime(t *testing.T) {
	st, _ := oidcKeyStore(t)
	ctx := context.Background()
	if _, err := st.BootstrapOIDCSigningKey(ctx, "first", []byte{1}); err != nil {
		t.Fatal(err)
	}
	next, err := st.RotateOIDCSigningKey(ctx, "second", []byte{2}, time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	if lead := next.ActivatesAt.Sub(next.CreatedAt); lead != time.Hour {
		t.Errorf("the rotated-in key activates %v after it is published, want 1h", lead)
	}
	if _, err := st.RotateOIDCSigningKey(ctx, "third", []byte{3}, time.Hour, false); !errors.Is(err, store.ErrOIDCRotationPending) {
		t.Fatalf("a second pending rotation = %v, want ErrOIDCRotationPending", err)
	}
	if _, err := st.RotateOIDCSigningKey(ctx, "emergency", []byte{4}, 0, true); err != nil {
		t.Fatal(err)
	}
	keys, err := st.OIDCSigningKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].KID != "emergency" || string(keys[0].PrivateKey) != "\x04" {
		t.Fatalf("after an emergency rotation the set is %+v, want only the emergency key", keys)
	}
	if err := st.DeleteOIDCSigningKeys(ctx, []string{"emergency", "absent"}); err != nil {
		t.Fatal(err)
	}
	if keys, _ := st.OIDCSigningKeys(ctx); len(keys) != 0 {
		t.Fatalf("delete left %+v", keys)
	}
}
