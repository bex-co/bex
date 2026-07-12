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
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
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
