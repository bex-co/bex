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

// Package store is the bex control plane: the Postgres-backed source of
// truth for the product's business entities (tenants, apps, domains + their
// mappings to Ory and Metronome) and the minimal API over them. It projects `apps` rows into App CRs
// (app.bex.co/v1alpha1) for the operator to execute — policy/intent lives
// here, mechanism stays in the operator (docs/ADR003-control-plane.md).
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies all pending schema migrations to the database at uri
// (postgres:// or postgresql://). The schema ships embedded in the binary so
// a deploy always carries the schema it expects; a no-op when already current.
func Migrate(uri string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("controlplane: load embedded migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, pgx5URL(uri))
	if err != nil {
		return fmt.Errorf("controlplane: open database for migrate: %w", err)
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("controlplane: migrate: %w", err)
	}
	return nil
}

// pgx5URL rewrites a postgres:///postgresql:// URI to the pgx5:// scheme that
// golang-migrate's pgx/v5 driver registers under.
func pgx5URL(uri string) string {
	if rest, ok := strings.CutPrefix(uri, "postgresql://"); ok {
		return "pgx5://" + rest
	}
	if rest, ok := strings.CutPrefix(uri, "postgres://"); ok {
		return "pgx5://" + rest
	}
	return uri
}

// CheckOwnership verifies that every table in the public schema is owned by the
// role currently connected (CURRENT_USER). Returns an error listing any
// mis-owned tables. Call this at startup after Migrate so a hand-run migration
// executed as a superuser (which creates tables owned by that superuser rather
// than the application role) is caught before the first request.
//
// Incident (2026-07-12): tenant_invites was owned by postgres instead of bex,
// causing "permission denied for table tenant_invites" on every invite-redemption
// call. This check would have surfaced that at startup with a clear error.
func CheckOwnership(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		SELECT tablename, tableowner
		FROM pg_tables
		WHERE schemaname = 'public'
		  AND tableowner != current_user
		ORDER BY tablename
	`)
	if err != nil {
		return fmt.Errorf("controlplane: ownership check: %w", err)
	}
	defer rows.Close()
	var misowned []string
	for rows.Next() {
		var table, owner string
		if err := rows.Scan(&table, &owner); err != nil {
			return fmt.Errorf("controlplane: ownership check scan: %w", err)
		}
		misowned = append(misowned, fmt.Sprintf("%s (owner: %s)", table, owner))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("controlplane: ownership check rows: %w", err)
	}
	return ownershipError(misowned)
}

// ownershipError formats the error returned by CheckOwnership. Extracted so
// tests can exercise the error path without a live database.
func ownershipError(misowned []string) error {
	if len(misowned) == 0 {
		return nil
	}
	return fmt.Errorf("controlplane: table ownership mismatch — these public tables are not owned by the application role: %s — fix by running migration 0013 as a superuser or: ALTER TABLE <table> OWNER TO <app-role>", strings.Join(misowned, ", "))
}
