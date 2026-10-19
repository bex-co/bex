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
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// guardFixture is one workspace with a project holding a production and a
// staging environment, against the real database.
type guardFixture struct {
	s             *PGStore
	pool          *pgxpool.Pool
	ten           Tenant
	proj          Project
	prod, staging Environment
	stamp         string
}

func newGuardFixture(t *testing.T) guardFixture {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := NewPGStore(pool)
	f := guardFixture{s: s, pool: pool, stamp: fmt.Sprintf("%d", time.Now().UnixNano())}
	if f.ten, err = s.CreateWorkspace(ctx, "guard-"+f.stamp, PlanHobby, "alice-"+f.stamp); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteTenant(context.Background(), f.ten.ID) })
	if f.proj, err = s.CreateProject(ctx, f.ten.ID, "stack-"+f.stamp); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if f.prod, err = s.CreateEnvironment(ctx, f.proj.ID, f.ten.ID, "production"); err != nil {
		t.Fatalf("create prod: %v", err)
	}
	if f.staging, err = s.CreateEnvironment(ctx, f.proj.ID, f.ten.ID, "staging"); err != nil {
		t.Fatalf("create staging: %v", err)
	}
	return f
}

func (f guardFixture) app(t *testing.T, name string) App {
	t.Helper()
	app, err := f.s.CreateApp(context.Background(), App{TenantID: f.ten.ID, Name: name + "-" + f.stamp, Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	return app
}

// placement is the app row's (project, environment), "" for NULL.
func (f guardFixture) placement(t *testing.T, appID string) (string, string) {
	t.Helper()
	var project, environment *string
	if err := f.pool.QueryRow(context.Background(), `SELECT project_id, environment_id FROM apps WHERE id = $1`, appID).Scan(&project, &environment); err != nil {
		t.Fatal(err)
	}
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return deref(project), deref(environment)
}

// TestPGPlacementGuardRefusalRollsBackTheWrite is w4/m232 against the real
// transactions: each placement writer hands its guard the environment a row
// leaves while the row is locked, and a refusal commits nothing.
func TestPGPlacementGuardRefusalRollsBackTheWrite(t *testing.T) {
	f := newGuardFixture(t)
	ctx := context.Background()
	s, pool, ten, proj, prod, staging := f.s, f.pool, f.ten, f.proj, f.prod, f.staging
	app := f.app(t, "web")
	place := func() *string {
		t.Helper()
		var env *string
		if err := pool.QueryRow(ctx, `SELECT environment_id FROM apps WHERE id = $1`, app.ID).Scan(&env); err != nil {
			t.Fatal(err)
		}
		return env
	}
	inProd := func(label string) {
		t.Helper()
		if env := place(); env == nil || *env != prod.ID {
			t.Fatalf("%s: environment = %v, want prod (the refused write must roll back)", label, env)
		}
	}
	if _, err := s.SetEnvironmentServices(ctx, prod.ID, proj.ID, ten.ID, []string{app.ID}, nil); err != nil {
		t.Fatalf("seed prod: %v", err)
	}

	refused := errors.New("refused")
	var left []string
	refuse := func(ids ...string) error {
		left = ids
		return refused
	}
	if _, err := s.SetEnvironmentServices(ctx, staging.ID, proj.ID, ten.ID, []string{app.ID}, refuse); !errors.Is(err, refused) {
		t.Fatalf("SetEnvironmentServices = %v, want the refusal", err)
	}
	if !slices.Equal(left, []string{prod.ID}) {
		t.Fatalf("guard saw %v leaving, want prod", left)
	}
	inProd("SetEnvironmentServices")

	left = nil
	if _, err := s.SetProjectServices(ctx, proj.ID, ten.ID, nil, refuse); !errors.Is(err, refused) {
		t.Fatalf("SetProjectServices = %v, want the refusal", err)
	}
	if !slices.Equal(left, []string{prod.ID}) {
		t.Fatalf("guard saw %v leaving, want prod", left)
	}
	inProd("SetProjectServices")

	left = nil
	if err := s.SetAppEnvironment(ctx, app.ID, proj.ID, staging.ID, refuse); !errors.Is(err, refused) || !slices.Equal(left, []string{prod.ID}) {
		t.Fatalf("SetAppEnvironment = %v (exits %v), want the refusal of prod", err, left)
	}
	inProd("SetAppEnvironment")

	// Admitted, the same write lands.
	if _, err := s.SetEnvironmentServices(ctx, staging.ID, proj.ID, ten.ID, []string{app.ID}, func(...string) error { return nil }); err != nil {
		t.Fatalf("admitted SetEnvironmentServices: %v", err)
	}
	if env := place(); env == nil || *env != staging.ID {
		t.Fatalf("admitted move left environment %v, want staging", env)
	}
}

// TestPGPlacementFunnelLocksItsRowsAgainstAConcurrentMove: a project
// membership write racing a transaction that moves one member into prod and a
// second service into the project. The funnel waits for the member's row and
// vets the environment it actually leaves (prod, not the staging an unlocked
// read saw), and the newcomer it never locked is left alone rather than pulled
// out of prod unvetted.
func TestPGPlacementFunnelLocksItsRowsAgainstAConcurrentMove(t *testing.T) {
	f := newGuardFixture(t)
	ctx := context.Background()
	member, newcomer := f.app(t, "member"), f.app(t, "newcomer")
	if _, err := f.s.SetEnvironmentServices(ctx, f.staging.ID, f.proj.ID, f.ten.ID, []string{member.ID}, nil); err != nil {
		t.Fatalf("seed staging: %v", err)
	}

	mover, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mover.Rollback(ctx) }()
	if _, err := mover.Exec(ctx, `UPDATE apps SET environment_id = $2 WHERE id = $1`, member.ID, f.prod.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := mover.Exec(ctx, `UPDATE apps SET project_id = $2, environment_id = $3 WHERE id = $1`, newcomer.ID, f.proj.ID, f.prod.ID); err != nil {
		t.Fatal(err)
	}

	var left []string
	done := make(chan error, 1)
	go func() {
		_, err := f.s.SetProjectServices(ctx, f.proj.ID, f.ten.ID, nil, func(ids ...string) error {
			left = ids
			return nil
		})
		done <- err
	}()
	// Commit the move only once the funnel is waiting on its row lock.
	deadline := time.Now().Add(10 * time.Second)
	for {
		var waiting int
		if err := f.pool.QueryRow(ctx,
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`,
		).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the funnel never waited on the moved row")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := mover.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("SetProjectServices: %v", err)
	}

	if !slices.Equal(left, []string{f.prod.ID}) {
		t.Fatalf("guard saw %v leaving, want prod (the member's committed environment)", left)
	}
	if project, environment := f.placement(t, member.ID); project != "" || environment != "" {
		t.Fatalf("member = (%q, %q), want out of the project", project, environment)
	}
	if project, environment := f.placement(t, newcomer.ID); project != f.proj.ID || environment != f.prod.ID {
		t.Fatalf("newcomer = (%q, %q), want untouched in prod", project, environment)
	}
}
