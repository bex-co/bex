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
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
)

func TestFailureOnlyMigrationPreservesCustomizedRows(t *testing.T) {
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
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	ten, err := s.CreateWorkspace(ctx, "notify-migration", PlanPro, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertNotificationSettings(ctx, ten.ID, "legacy-default", true, true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertNotificationSettings(ctx, ten.ID, "custom", true, false, true); err != nil {
		t.Fatal(err)
	}
	sql, err := migrationsFS.ReadFile("migrations/0032_notification_failure_only.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	legacy, _ := s.GetNotificationSettings(ctx, ten.ID, "legacy-default")
	custom, _ := s.GetNotificationSettings(ctx, ten.ID, "custom")
	if legacy.DeployStarted || legacy.DeploySucceeded || !legacy.DeployFailed {
		t.Fatalf("legacy = %+v", legacy)
	}
	if !custom.DeployStarted || custom.DeploySucceeded || !custom.DeployFailed {
		t.Fatalf("custom changed: %+v", custom)
	}
}
