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
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// w4/m301: migration 0219 turns each host auto-match into the explicit
// binding it resolved to (newest credential per canonical host), and leaves
// explicit bindings, explicit clears, repo builds and unmatched images alone.
func TestRegistryCredentialExplicitBindingBackfill(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE SCHEMA migration_0219_registry;
 SET LOCAL search_path TO migration_0219_registry;
 CREATE TABLE apps (id text PRIMARY KEY, tenant_id text, image text, repo text, registry_credential_id text);
 CREATE TABLE registry_credentials (id text PRIMARY KEY, workspace_id text, host text, created_at timestamptz);
 INSERT INTO registry_credentials VALUES
   ('rgc-ghcr-old', 'tea-a', 'https://ghcr.io/', '2026-01-01'),
   ('rgc-ghcr-new', 'tea-a', 'ghcr.io', '2026-02-01'),
   ('rgc-hub', 'tea-a', 'index.docker.io', '2026-01-01'),
   ('rgc-other-ws', 'tea-b', 'quay.io', '2026-01-01');
 INSERT INTO apps VALUES
   ('ghcr', 'tea-a', 'ghcr.io/acme/private:1', '', NULL),
   ('hub-short', 'tea-a', 'nginx:1.27', '', NULL),
   ('hub-long', 'tea-a', 'docker.io/library/nginx:1.27', NULL, NULL),
   ('explicit', 'tea-a', 'ghcr.io/acme/private:1', '', 'rgc-ghcr-old'),
   ('cleared', 'tea-a', 'ghcr.io/acme/private:1', '', ''),
   ('repo', 'tea-a', '', 'https://github.com/acme/app', NULL),
   ('unmatched', 'tea-a', 'quay.io/acme/app:1', '', NULL),
   ('other-ws', 'tea-c', 'quay.io/acme/app:1', '', NULL);`)
	sql, err := migrationsFS.ReadFile("migrations/0219_registry_credential_explicit_binding.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	exec(string(sql))
	want := map[string]*string{
		"ghcr": new("rgc-ghcr-new"), "hub-short": new("rgc-hub"), "hub-long": new("rgc-hub"),
		"explicit": new("rgc-ghcr-old"), "cleared": new(""), "repo": nil, "unmatched": nil, "other-ws": nil,
	}
	for id, w := range want {
		var got *string
		if err := tx.QueryRow(ctx, `SELECT registry_credential_id FROM apps WHERE id = $1`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if (got == nil) != (w == nil) || (got != nil && *got != *w) {
			t.Errorf("%s: registry_credential_id = %v, want %v", id, deref(got), deref(w))
		}
	}
}

func deref(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}
