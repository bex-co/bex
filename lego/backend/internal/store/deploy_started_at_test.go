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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestTransitionDeployFailureSkipStartedAt (w6/m123, memStore half): a
// queued → build_failed skip must not stamp started_at from the transition's
// own clock — a 68-second build used to record a one-microsecond duration
// that way. With the operator's recorded window as evidence, the real start is
// stamped; without it, started_at stays honestly null (like canceled). The
// in-progress path — correct today — keeps stamping the clock.
func TestTransitionDeployFailureSkipStartedAt(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	app, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	// Terminal skip without evidence: null start, real finish. CreateApp
	// auto-opens the app's first deploy row (status created); take that one.
	d1, ok, err := openDeployFor(ctx, s, app.ID)
	if err != nil || !ok {
		t.Fatalf("open deploy: ok=%v err=%v", ok, err)
	}
	mustTransition(t, s, d1.ID, DeployQueued, nil)
	mustTransition(t, s, d1.ID, DeployBuildFailed, nil)
	closed, _ := s.GetDeploy(ctx, app.ID, d1.ID)
	if closed.StartedAt != nil || closed.FinishedAt == nil {
		t.Fatalf("skip close: started=%v finished=%v, want null start and a finish", closed.StartedAt, closed.FinishedAt)
	}

	// Terminal skip with the operator's recorded window: the real start.
	start := time.Date(2026, 8, 27, 19, 56, 10, 0, time.UTC)
	d2, _ := s.CreateDeploy(ctx, app.ID, "web", "img:2", 2, CommitInfo{})
	mustTransition(t, s, d2.ID, DeployQueued, nil)
	mustTransition(t, s, d2.ID, DeployBuildFailed, &start)
	evidenced, _ := s.GetDeploy(ctx, app.ID, d2.ID)
	if evidenced.StartedAt == nil || !evidenced.StartedAt.Equal(start) {
		t.Fatalf("evidenced close: started=%v, want %v", evidenced.StartedAt, start)
	}
	if evidenced.FinishedAt == nil || evidenced.FinishedAt.Before(*evidenced.StartedAt) {
		t.Fatalf("evidenced close: finished=%v not after started=%v", evidenced.FinishedAt, evidenced.StartedAt)
	}

	// Canceled while queued keeps its null start (correct today, regression).
	d3, _ := s.CreateDeploy(ctx, app.ID, "web", "img:3", 3, CommitInfo{})
	mustTransition(t, s, d3.ID, DeployQueued, nil)
	mustTransition(t, s, d3.ID, DeployCanceled, nil)
	canceled, _ := s.GetDeploy(ctx, app.ID, d3.ID)
	if canceled.StartedAt != nil {
		t.Fatalf("canceled deploy grew a start: %v", canceled.StartedAt)
	}

	// The dispatch-observing path still stamps the clock (correct today).
	d4, _ := s.CreateDeploy(ctx, app.ID, "web", "img:4", 4, CommitInfo{})
	mustTransition(t, s, d4.ID, DeployBuildInProgress, nil)
	dispatched, _ := s.GetDeploy(ctx, app.ID, d4.ID)
	if dispatched.StartedAt == nil {
		t.Fatal("in-progress transition no longer stamps started_at")
	}
}

func mustTransition(t *testing.T, s Store, id, status string, startedAt *time.Time) {
	t.Helper()
	won, err := s.TransitionDeploy(context.Background(), id, status, "", "", "", startedAt)
	if err != nil || !won {
		t.Fatalf("transition %s -> %s: won=%v err=%v", id, status, won, err)
	}
}

// TestPGTransitionDeployFailureSkipStartedAt is the same contract against the
// real SQL — the CASE that used to read COALESCE(started_at, clock_timestamp())
// for every executing status, build_failed included.
func TestPGTransitionDeployFailureSkipStartedAt(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)

	ten, err := s.CreateTenantWithMember(ctx, "m123-started-at", PlanHobby)
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	app, err := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatalf("app: %v", err)
	}

	// Without evidence: the failure skip leaves started_at NULL. CreateApp
	// auto-opens the app's first deploy row (status created); take that one.
	d1, ok, err := openDeployFor(ctx, s, app.ID)
	if err != nil || !ok {
		t.Fatalf("open deploy: ok=%v err=%v", ok, err)
	}
	mustTransition(t, s, d1.ID, DeployQueued, nil)
	mustTransition(t, s, d1.ID, DeployBuildFailed, nil)
	closed, err := s.GetDeploy(ctx, app.ID, d1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.StartedAt != nil || closed.FinishedAt == nil {
		t.Fatalf("skip close: started=%v finished=%v, want NULL start and a finish", closed.StartedAt, closed.FinishedAt)
	}

	// With evidence: the recorded window's start, exactly.
	start := time.Date(2026, 8, 27, 19, 56, 10, 0, time.UTC)
	d2, err := s.CreateDeploy(ctx, app.ID, "web", "img:2", 2, CommitInfo{})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	mustTransition(t, s, d2.ID, DeployQueued, nil)
	mustTransition(t, s, d2.ID, DeployBuildFailed, &start)
	evidenced, err := s.GetDeploy(ctx, app.ID, d2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if evidenced.StartedAt == nil || !evidenced.StartedAt.Equal(start) {
		t.Fatalf("evidenced close: started=%v, want %v", evidenced.StartedAt, start)
	}
	if !evidenced.FinishedAt.After(*evidenced.StartedAt) {
		t.Fatalf("duration collapsed again: started=%v finished=%v", evidenced.StartedAt, evidenced.FinishedAt)
	}

	// An in-progress transition still stamps, and a later failure close must
	// not overwrite it with the (older) evidence.
	d3, err := s.CreateDeploy(ctx, app.ID, "web", "img:3", 3, CommitInfo{})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	mustTransition(t, s, d3.ID, DeployBuildInProgress, nil)
	inProgress, _ := s.GetDeploy(ctx, app.ID, d3.ID)
	if inProgress.StartedAt == nil {
		t.Fatal("in-progress transition no longer stamps started_at")
	}
	mustTransition(t, s, d3.ID, DeployBuildFailed, &start)
	kept, _ := s.GetDeploy(ctx, app.ID, d3.ID)
	if kept.StartedAt == nil || !kept.StartedAt.Equal(*inProgress.StartedAt) {
		t.Fatalf("failure close moved an already-stamped start: %v -> %v", inProgress.StartedAt, kept.StartedAt)
	}
}
