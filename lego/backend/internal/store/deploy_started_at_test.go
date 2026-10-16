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

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestTransitionDeployFailureSkipStartedAt (w6/m123, memStore half): a
// queued → build_failed skip must not stamp started_at from the transition's
// own clock — a 68-second build used to record a one-microsecond duration
// that way. With the operator's recorded window as evidence, the real start is
// stamped; without it, started_at stays honestly null (like canceled). The
// in-progress path — correct today — keeps stamping the clock.
func TestTransitionDeployFailureSkipStartedAt(t *testing.T) {
	ctx := context.Background()
	s, appID := startedAtMemApp(t)

	// Terminal skip without evidence: null start, real finish. CreateApp
	// auto-opens the app's first deploy row (status created); take that one.
	d1, ok, err := openDeployFor(ctx, s, appID)
	if err != nil || !ok {
		t.Fatalf("open deploy: ok=%v err=%v", ok, err)
	}
	mustTransition(t, s, d1.ID, DeployQueued, nil)
	mustTransition(t, s, d1.ID, DeployBuildFailed, nil)
	closed, _ := s.GetDeploy(ctx, appID, d1.ID)
	if closed.StartedAt != nil || closed.FinishedAt == nil {
		t.Fatalf("skip close: started=%v finished=%v, want null start and a finish", closed.StartedAt, closed.FinishedAt)
	}

	// Terminal skip with the operator's recorded window: the real start.
	start := time.Date(2026, 8, 27, 19, 56, 10, 0, time.UTC)
	d2, _ := s.CreateDeploy(ctx, appID, "web", "img:2", 2, CommitInfo{}, "")
	mustTransition(t, s, d2.ID, DeployQueued, nil)
	mustTransition(t, s, d2.ID, DeployBuildFailed, &start)
	evidenced, _ := s.GetDeploy(ctx, appID, d2.ID)
	if evidenced.StartedAt == nil || !evidenced.StartedAt.Equal(start) {
		t.Fatalf("evidenced close: started=%v, want %v", evidenced.StartedAt, start)
	}
	if evidenced.FinishedAt == nil || evidenced.FinishedAt.Before(*evidenced.StartedAt) {
		t.Fatalf("evidenced close: finished=%v not after started=%v", evidenced.FinishedAt, evidenced.StartedAt)
	}

	// Canceled while queued keeps its null start (correct today, regression).
	d3, _ := s.CreateDeploy(ctx, appID, "web", "img:3", 3, CommitInfo{}, "")
	mustTransition(t, s, d3.ID, DeployQueued, nil)
	mustTransition(t, s, d3.ID, DeployCanceled, nil)
	canceled, _ := s.GetDeploy(ctx, appID, d3.ID)
	if canceled.StartedAt != nil {
		t.Fatalf("canceled deploy grew a start: %v", canceled.StartedAt)
	}

	// The dispatch-observing path still stamps the clock (correct today).
	d4, _ := s.CreateDeploy(ctx, appID, "web", "img:4", 4, CommitInfo{}, "")
	mustTransition(t, s, d4.ID, DeployBuildInProgress, nil)
	dispatched, _ := s.GetDeploy(ctx, appID, d4.ID)
	if dispatched.StartedAt == nil {
		t.Fatal("in-progress transition no longer stamps started_at")
	}
}

func mustTransition(t *testing.T, s Store, id, status string, startedAt *time.Time) {
	t.Helper()
	won, err := s.TransitionDeploy(context.Background(), id, status, "", "", "", "", startedAt)
	if err != nil || !won {
		t.Fatalf("transition %s -> %s: won=%v err=%v", id, status, won, err)
	}
}

// startedAtMemApp is a memStore with one free image app, whose first deploy
// row CreateApp has already opened.
func startedAtMemApp(t *testing.T) (*memStore, string) {
	t.Helper()
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	app, err := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	return s, app.ID
}

// startedAtPGApp is startedAtMemApp against a freshly truncated real
// database; the pool is returned for direct SQL reads. Requires
// BEX_TEST_DB_URI.
func startedAtPGApp(t *testing.T, tenant string) (*pgxpool.Pool, *PGStore, string) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	ten, err := s.CreateTenantWithMember(ctx, tenant, PlanHobby)
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	app, err := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	return pool, s, app.ID
}

func mustGetDeploy(t *testing.T, s Store, appID, id string) Deploy {
	t.Helper()
	d, err := s.GetDeploy(context.Background(), appID, id)
	if err != nil {
		t.Fatalf("get deploy %s: %v", id, err)
	}
	return d
}

// TestPGTransitionDeployFailureSkipStartedAt is the same contract against the
// real SQL — the CASE that used to read COALESCE(started_at, clock_timestamp())
// for every executing status, build_failed included.
func TestPGTransitionDeployFailureSkipStartedAt(t *testing.T) {
	ctx := context.Background()
	_, s, appID := startedAtPGApp(t, "m123-started-at")

	// Without evidence: the failure skip leaves started_at NULL. CreateApp
	// auto-opens the app's first deploy row (status created); take that one.
	d1, ok, err := openDeployFor(ctx, s, appID)
	if err != nil || !ok {
		t.Fatalf("open deploy: ok=%v err=%v", ok, err)
	}
	mustTransition(t, s, d1.ID, DeployQueued, nil)
	mustTransition(t, s, d1.ID, DeployBuildFailed, nil)
	closed, err := s.GetDeploy(ctx, appID, d1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.StartedAt != nil || closed.FinishedAt == nil {
		t.Fatalf("skip close: started=%v finished=%v, want NULL start and a finish", closed.StartedAt, closed.FinishedAt)
	}

	// With evidence: the recorded window's start, exactly.
	start := time.Date(2026, 8, 27, 19, 56, 10, 0, time.UTC)
	d2, err := s.CreateDeploy(ctx, appID, "web", "img:2", 2, CommitInfo{}, "")
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	mustTransition(t, s, d2.ID, DeployQueued, nil)
	mustTransition(t, s, d2.ID, DeployBuildFailed, &start)
	evidenced, err := s.GetDeploy(ctx, appID, d2.ID)
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
	d3, err := s.CreateDeploy(ctx, appID, "web", "img:3", 3, CommitInfo{}, "")
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	mustTransition(t, s, d3.ID, DeployBuildInProgress, nil)
	inProgress, _ := s.GetDeploy(ctx, appID, d3.ID)
	if inProgress.StartedAt == nil {
		t.Fatal("in-progress transition no longer stamps started_at")
	}
	mustTransition(t, s, d3.ID, DeployBuildFailed, &start)
	kept, _ := s.GetDeploy(ctx, appID, d3.ID)
	if kept.StartedAt == nil || !kept.StartedAt.Equal(*inProgress.StartedAt) {
		t.Fatalf("failure close moved an already-stamped start: %v -> %v", inProgress.StartedAt, kept.StartedAt)
	}
}

// assertSuccessfulSkipStartedAt is the w4/m156 contract both stores share: a
// deploy first observed already live was never seen executing, so the live
// observation is not its start. Without owned evidence started_at stays
// unknown (the image create/restart repro: 0s "durations" whose start banner
// trailed the app's own startup log); with the deploy's own recorded build
// window it is that start; and a start an in-progress observation already
// recorded is never moved — not by live, not by later evidence, not by the
// deactivation a newer live deploy applies. Returns the deploys whose
// analytics duration the PG half checks, with their expected start.
func assertSuccessfulSkipStartedAt(t *testing.T, s Store, appID string) map[string]*time.Time {
	t.Helper()
	ctx := context.Background()
	get := func(id string) Deploy { t.Helper(); return mustGetDeploy(t, s, appID, id) }

	// created → live with no evidence: unknown start, real finish.
	first, ok, err := openDeployFor(ctx, s, appID)
	if err != nil || !ok {
		t.Fatalf("open deploy: ok=%v err=%v", ok, err)
	}
	mustTransition(t, s, first.ID, DeployLive, nil)
	created := get(first.ID)
	if created.StartedAt != nil || created.FinishedAt == nil {
		t.Fatalf("created→live: started=%v finished=%v, want unknown start and a finish", created.StartedAt, created.FinishedAt)
	}

	// queued → live with no evidence: unknown too, and the deactivation it
	// applies to the first deploy invents nothing for that one either.
	restart, err := s.CreateDeploy(ctx, appID, TriggerAPI, "img:2", 2, CommitInfo{}, "")
	if err != nil {
		t.Fatalf("restart deploy: %v", err)
	}
	mustTransition(t, s, restart.ID, DeployQueued, nil)
	mustTransition(t, s, restart.ID, DeployLive, nil)
	if got := get(restart.ID); got.StartedAt != nil || got.FinishedAt == nil {
		t.Fatalf("queued→live: started=%v finished=%v, want unknown start and a finish", got.StartedAt, got.FinishedAt)
	}
	deactivated := get(first.ID)
	if deactivated.Status != DeployDeactivated || deactivated.StartedAt != nil ||
		!deactivated.FinishedAt.Equal(*created.FinishedAt) {
		t.Fatalf("deactivated first deploy = %+v, want unknown start and its live finish kept", deactivated)
	}

	// queued → live with the deploy's own recorded build window: that start.
	window := time.Date(2026, 10, 3, 3, 33, 54, 0, time.UTC)
	built, err := s.CreateDeploy(ctx, appID, TriggerAPI, "", 3, CommitInfo{}, "")
	if err != nil {
		t.Fatalf("built deploy: %v", err)
	}
	mustTransition(t, s, built.ID, DeployQueued, nil)
	mustTransition(t, s, built.ID, DeployLive, &window)
	if got := get(built.ID); got.StartedAt == nil || !got.StartedAt.Equal(window) {
		t.Fatalf("evidenced live: started=%v, want %v", got.StartedAt, window)
	}

	// An observed in-progress start (the native restart control: distinct
	// start and finish) survives live, evidence, and deactivation unchanged.
	observed, err := s.CreateDeploy(ctx, appID, TriggerAPI, "img:4", 4, CommitInfo{}, "")
	if err != nil {
		t.Fatalf("observed deploy: %v", err)
	}
	mustTransition(t, s, observed.ID, DeployUpdateInProgress, nil)
	stamped := get(observed.ID).StartedAt
	if stamped == nil {
		t.Fatal("in-progress transition no longer stamps started_at")
	}
	mustTransition(t, s, observed.ID, DeployLive, &window)
	live := get(observed.ID)
	if live.StartedAt == nil || !live.StartedAt.Equal(*stamped) || live.FinishedAt.Before(*stamped) {
		t.Fatalf("observed live: started=%v finished=%v, want start %v kept", live.StartedAt, live.FinishedAt, stamped)
	}
	next, err := s.CreateDeploy(ctx, appID, TriggerAPI, "img:5", 5, CommitInfo{}, "")
	if err != nil {
		t.Fatalf("next deploy: %v", err)
	}
	mustTransition(t, s, next.ID, DeployLive, nil)
	if got := get(observed.ID); got.Status != DeployDeactivated || !got.StartedAt.Equal(*stamped) ||
		!got.FinishedAt.Equal(*live.FinishedAt) {
		t.Fatalf("deactivated observed deploy = %+v, want start %v and finish %v kept", got, stamped, live.FinishedAt)
	}
	return map[string]*time.Time{first.ID: nil, restart.ID: nil, built.ID: &window, observed.ID: stamped}
}

func TestTransitionDeploySuccessfulSkipStartedAt(t *testing.T) {
	s, appID := startedAtMemApp(t)
	assertSuccessfulSkipStartedAt(t, s, appID)
}

// TestPGTransitionDeploySuccessfulSkipStartedAt runs the same contract against
// the real SQL, then reads the composed product-analytics trigger: a success
// whose start is unknown records an unknown (NULL) duration, never zero, while
// the evidenced deploys keep a real one.
func TestPGTransitionDeploySuccessfulSkipStartedAt(t *testing.T) {
	ctx := context.Background()
	pool, s, appID := startedAtPGApp(t, "m156-started-at")
	for id, start := range assertSuccessfulSkipStartedAt(t, s, appID) {
		var duration *float64
		if err := pool.QueryRow(ctx,
			`SELECT duration_ms::float8 FROM product_activity_events WHERE source_key = $1`,
			"deploy_finished:"+id).Scan(&duration); err != nil {
			t.Fatalf("analytics row for %s: %v", id, err)
		}
		if start == nil && duration != nil {
			t.Errorf("deploy %s with an unknown start recorded duration %v ms, want NULL", id, *duration)
		}
		if start != nil && duration == nil {
			t.Errorf("deploy %s with an evidenced start recorded no duration", id)
		}
	}
}

// assertFailedPreDeployStartedAt is the store half of w4/316: a created →
// pre_deploy_failed close carrying the failed step's recorded start stamps
// exactly that start, one without evidence stays unknown, and a start an
// in-progress observation already recorded is never moved by the evidence.
func assertFailedPreDeployStartedAt(t *testing.T, s Store, appID string) {
	t.Helper()
	ctx := context.Background()
	get := func(id string) Deploy { t.Helper(); return mustGetDeploy(t, s, appID, id) }
	start := time.Date(2026, 10, 15, 21, 12, 42, 0, time.UTC)

	fast, ok, err := openDeployFor(ctx, s, appID)
	if err != nil || !ok {
		t.Fatalf("open deploy: ok=%v err=%v", ok, err)
	}
	mustTransition(t, s, fast.ID, DeployPreDeployFailed, &start)
	if got := get(fast.ID); got.StartedAt == nil || !got.StartedAt.Equal(start) || got.FinishedAt == nil {
		t.Fatalf("evidenced fast failure: started=%v finished=%v, want %v and a finish", got.StartedAt, got.FinishedAt, start)
	}

	bare, err := s.CreateDeploy(ctx, appID, TriggerAPI, "img:2", 2, CommitInfo{}, "")
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	mustTransition(t, s, bare.ID, DeployPreDeployFailed, nil)
	if got := get(bare.ID); got.StartedAt != nil || got.FinishedAt == nil {
		t.Fatalf("unevidenced fast failure: started=%v finished=%v, want unknown start and a finish", got.StartedAt, got.FinishedAt)
	}

	slow, err := s.CreateDeploy(ctx, appID, TriggerAPI, "img:3", 3, CommitInfo{}, "")
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	mustTransition(t, s, slow.ID, DeployPreDeployInProgress, nil)
	stamped := get(slow.ID).StartedAt
	if stamped == nil {
		t.Fatal("pre_deploy_in_progress no longer stamps started_at")
	}
	mustTransition(t, s, slow.ID, DeployPreDeployFailed, &start)
	if got := get(slow.ID); got.StartedAt == nil || !got.StartedAt.Equal(*stamped) {
		t.Fatalf("slow failure moved its observed start: %v -> %v", stamped, got.StartedAt)
	}
}

func TestTransitionDeployFailedPreDeployStartedAt(t *testing.T) {
	s, appID := startedAtMemApp(t)
	assertFailedPreDeployStartedAt(t, s, appID)
}

func TestPGTransitionDeployFailedPreDeployStartedAt(t *testing.T) {
	_, s, appID := startedAtPGApp(t, "w4-316-started-at")
	assertFailedPreDeployStartedAt(t, s, appID)
}
