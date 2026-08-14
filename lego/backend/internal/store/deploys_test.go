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
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// These exercise the in-memory Store (always on, no BEX_TEST_DB_URI needed) —
// PGStore's equivalent behavior is covered against real Postgres by
// assertDeployLifecycle in store_pg_test.go.

// openDeployFor is the test-side equivalent of the old per-app OpenDeploy:
// ListOpenDeploys (the batched call the reconciler now uses once per pass,
// not once per app) filtered to one app, for tests that only care about one.
func openDeployFor(ctx context.Context, s Store, appID string) (Deploy, bool, error) {
	open, err := s.ListOpenDeploys(ctx)
	if err != nil {
		return Deploy{}, false, err
	}
	for _, d := range open {
		if d.AppID == appID {
			return d, true, nil
		}
	}
	return Deploy{}, false, nil
}

func TestCreateAppOpensFirstDeploy(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	app, err := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}

	deploys, err := s.ListDeploys(ctx, app.ID, DeployFilter{})
	if err != nil {
		t.Fatalf("list deploys: %v", err)
	}
	if len(deploys) != 1 || deploys[0].Trigger != "create" || deploys[0].Image != "img:1" || deploys[0].Status != DeployCreated {
		t.Fatalf("deploy #1 = %+v", deploys)
	}

	open, ok, err := openDeployFor(ctx, s, app.ID)
	if err != nil || !ok || open.ID != deploys[0].ID {
		t.Fatalf("open deploy = %+v ok=%v (err %v)", open, ok, err)
	}
}

func TestCloseDeployIsIdempotentAndListIsNewestFirst(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	app, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	first, _, err := openDeployFor(ctx, s, app.ID)
	if err != nil {
		t.Fatalf("open first deploy: %v", err)
	}

	if won, err := s.CloseDeploy(ctx, first.ID, DeployLive, "img:resolved"); err != nil || !won {
		t.Fatalf("close: won=%v err=%v", won, err)
	}
	// Re-closing an already-terminal deploy must not flip its status, and must
	// report it lost the race (already terminal).
	if won, err := s.CloseDeploy(ctx, first.ID, DeployUpdateFailed, ""); err != nil || won {
		t.Fatalf("re-close: won=%v err=%v, want won=false", won, err)
	}
	got, err := s.GetDeploy(ctx, app.ID, first.ID)
	if err != nil || got.Status != DeployLive || got.FinishedAt == nil || got.ResolvedImage != "img:resolved" {
		t.Fatalf("closed deploy = %+v (err %v), want status live with finished_at + resolved_image set", got, err)
	}
	if _, ok, err := openDeployFor(ctx, s, app.ID); err != nil || ok {
		t.Fatalf("open deploy after close: ok=%v (err %v), want none open", ok, err)
	}

	second, err := s.CreateDeploy(ctx, app.ID, "api", "img:2", 2, CommitInfo{})
	if err != nil {
		t.Fatalf("trigger deploy: %v", err)
	}
	deploys, err := s.ListDeploys(ctx, app.ID, DeployFilter{})
	if err != nil {
		t.Fatalf("list deploys: %v", err)
	}
	if len(deploys) != 2 || deploys[0].ID != second.ID || deploys[1].ID != first.ID {
		t.Fatalf("list not newest-first: %+v", deploys)
	}
}

func TestTransitionDeployAdvancesTimestampsOnlyOnRealStateChanges(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	app, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	d, _, _ := openDeployFor(ctx, s, app.ID)
	if d.Status != DeployCreated || d.UpdatedAt.IsZero() || !d.UpdatedAt.Equal(d.CreatedAt) {
		t.Fatalf("new deploy = %+v, want created with updated_at=created_at", d)
	}

	if changed, err := s.TransitionDeploy(ctx, d.ID, DeployUpdateInProgress, "", "", ""); err != nil || !changed {
		t.Fatalf("transition to update_in_progress: changed=%v err=%v", changed, err)
	}
	progress, _ := s.GetDeploy(ctx, app.ID, d.ID)
	if progress.StartedAt == nil || !progress.UpdatedAt.After(d.UpdatedAt) || progress.FinishedAt != nil {
		t.Fatalf("in-progress deploy = %+v, want started/updated and unfinished", progress)
	}

	if changed, err := s.TransitionDeploy(ctx, d.ID, DeployUpdateInProgress, "", "", ""); err != nil || changed {
		t.Fatalf("repeated transition: changed=%v err=%v, want no-op", changed, err)
	}
	repeated, _ := s.GetDeploy(ctx, app.ID, d.ID)
	if !repeated.UpdatedAt.Equal(progress.UpdatedAt) {
		t.Errorf("no-op changed updated_at: %s -> %s", progress.UpdatedAt, repeated.UpdatedAt)
	}
	if changed, err := s.TransitionDeploy(ctx, d.ID, DeployBuildInProgress, "", "", ""); err != nil || changed {
		t.Fatalf("regression: changed=%v err=%v, want rejected", changed, err)
	}
	regressed, _ := s.GetDeploy(ctx, app.ID, d.ID)
	if regressed.Status != DeployUpdateInProgress || !regressed.UpdatedAt.Equal(progress.UpdatedAt) {
		t.Errorf("rejected regression mutated deploy: %+v", regressed)
	}
}

func TestLiveTransitionDeactivatesPriorLiveDeploy(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	app, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	first, _, _ := openDeployFor(ctx, s, app.ID)
	if won, err := s.CloseDeploy(ctx, first.ID, DeployLive, "img:1"); err != nil || !won {
		t.Fatalf("first live: won=%v err=%v", won, err)
	}
	firstLive, _ := s.GetDeploy(ctx, app.ID, first.ID)

	second, err := s.CreateDeploy(ctx, app.ID, TriggerAPI, "img:2", 2, CommitInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if won, err := s.CloseDeploy(ctx, second.ID, DeployLive, "img:2"); err != nil || !won {
		t.Fatalf("second live: won=%v err=%v", won, err)
	}
	prior, _ := s.GetDeploy(ctx, app.ID, first.ID)
	current, _ := s.GetDeploy(ctx, app.ID, second.ID)
	if prior.Status != DeployDeactivated || current.Status != DeployLive {
		t.Fatalf("prior/current = %+v / %+v, want deactivated/live", prior, current)
	}
	if prior.FinishedAt == nil || firstLive.FinishedAt == nil || !prior.FinishedAt.Equal(*firstLive.FinishedAt) {
		t.Errorf("deactivation changed the prior live finished_at: before=%v after=%v", firstLive.FinishedAt, prior.FinishedAt)
	}
	if !prior.UpdatedAt.After(firstLive.UpdatedAt) {
		t.Errorf("deactivation did not advance updated_at: %s -> %s", firstLive.UpdatedAt, prior.UpdatedAt)
	}
}

func TestCreateDeployCancelsOlderOpenDeployNewestWins(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	app, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Repo: "https://example.com/repo.git", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	first, _, _ := openDeployFor(ctx, s, app.ID)
	if changed, err := s.TransitionDeploy(ctx, first.ID, DeployBuildInProgress, "", "", ""); err != nil || !changed {
		t.Fatalf("start first build: changed=%v err=%v", changed, err)
	}
	second, err := s.CreateDeploy(ctx, app.ID, TriggerAPI, "", 2, CommitInfo{})
	if err != nil {
		t.Fatal(err)
	}
	old, _ := s.GetDeploy(ctx, app.ID, first.ID)
	if old.Status != DeployCanceled || old.FinishedAt == nil {
		t.Fatalf("superseded deploy = %+v, want canceled terminal", old)
	}
	open, ok, _ := openDeployFor(ctx, s, app.ID)
	if !ok || open.ID != second.ID || open.Status != DeployCreated {
		t.Fatalf("newest open deploy = %+v ok=%v, want %s created", open, ok, second.ID)
	}
}

func TestDelayedLowerGenerationDeployCannotSupersedeNewerOpenDeploy(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	app, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	newer, err := s.CreateDeploy(ctx, app.ID, TriggerAPI, "img:3", 3, CommitInfo{})
	if err != nil || newer.Status != DeployCreated {
		t.Fatalf("newer deploy = %+v (err %v), want created", newer, err)
	}
	delayed, err := s.CreateDeploy(ctx, app.ID, TriggerAPI, "img:2", 2, CommitInfo{})
	if err != nil || delayed.Status != DeployCanceled || delayed.FinishedAt == nil {
		t.Fatalf("delayed deploy = %+v (err %v), want immediately canceled", delayed, err)
	}
	open, ok, err := openDeployFor(ctx, s, app.ID)
	if err != nil || !ok || open.ID != newer.ID || open.Generation != 3 {
		t.Fatalf("open deploy = %+v ok=%v (err %v), want higher-generation %s", open, ok, err, newer.ID)
	}
}

// TestCreateRollbackDeployRecordsProvenanceAndResolvedImage covers w2/m10's
// t001: unlike a normal trigger, a rollback deploy already knows the exact
// image it restores (it ran live before), so ResolvedImage is set immediately
// — Rollback never has to wait on the reconciler's write-back.
func TestCreateRollbackDeployRecordsProvenanceAndResolvedImage(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	app, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	first, _, _ := openDeployFor(ctx, s, app.ID)

	rb, err := s.CreateRollbackDeploy(ctx, app.ID, "img:1", first.ID, 2, CommitInfo{})
	if err != nil {
		t.Fatalf("create rollback deploy: %v", err)
	}
	if rb.Trigger != "rollback" || rb.Image != "img:1" || rb.ResolvedImage != "img:1" || rb.RollbackOf != first.ID {
		t.Fatalf("rollback deploy = %+v", rb)
	}
	got, err := s.GetDeploy(ctx, app.ID, rb.ID)
	if err != nil || got.RollbackOf != first.ID {
		t.Fatalf("get rollback deploy = %+v (err %v)", got, err)
	}
}

// seedDeploy writes a deploy row with explicit fields directly into the
// memStore, bypassing CreateDeploy's time.Now() stamp, so the filter tests
// below control the timeline exactly (no flaky same-instant timestamps).
func seedDeploy(s *memStore, id, appID, status string, createdAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deploys[id] = Deploy{ID: id, AppID: appID, Trigger: TriggerAPI, Status: status, CreatedAt: createdAt}
}

func deployIDs(deploys []Deploy) []string {
	out := make([]string, len(deploys))
	for i, d := range deploys {
		out[i] = d.ID
	}
	return out
}

func TestListDeploysFilters(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	// Five deploys a minute apart, plus two sharing one instant (the id
	// tiebreak's case), newest-first expected order:
	//   dep-g,dep-f (same instant, id desc) > dep-e > dep-d > dep-c > dep-b > dep-a
	seedDeploy(s, "dep-a", "srv-1", DeployLive, base)
	seedDeploy(s, "dep-b", "srv-1", DeployUpdateFailed, base.Add(1*time.Minute))
	seedDeploy(s, "dep-c", "srv-1", DeployLive, base.Add(2*time.Minute))
	seedDeploy(s, "dep-d", "srv-1", DeployCanceled, base.Add(3*time.Minute))
	seedDeploy(s, "dep-e", "srv-1", DeployUpdateInProgress, base.Add(4*time.Minute))
	seedDeploy(s, "dep-f", "srv-1", DeployLive, base.Add(5*time.Minute))
	seedDeploy(s, "dep-g", "srv-1", DeployLive, base.Add(5*time.Minute))
	// Another app's deploy must never leak in.
	seedDeploy(s, "dep-x", "srv-2", DeployLive, base.Add(6*time.Minute))

	assertIDs := func(name string, f DeployFilter, want ...string) {
		t.Helper()
		got, err := s.ListDeploys(ctx, "srv-1", f)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !slices.Equal(deployIDs(got), want) {
			t.Errorf("%s = %v, want %v", name, deployIDs(got), want)
		}
	}

	assertIDs("empty filter (full history, newest first, id tiebreak)", DeployFilter{},
		"dep-g", "dep-f", "dep-e", "dep-d", "dep-c", "dep-b", "dep-a")
	assertIDs("one status", DeployFilter{Statuses: []string{DeployLive}},
		"dep-g", "dep-f", "dep-c", "dep-a")
	assertIDs("two statuses", DeployFilter{Statuses: []string{DeployUpdateFailed, DeployCanceled}},
		"dep-d", "dep-b")
	assertIDs("unknown status matches nothing", DeployFilter{Statuses: []string{"warp_in_progress"}})
	// createdBefore/createdAfter are EXCLUSIVE: the row at the bound stays out.
	assertIDs("createdAfter", DeployFilter{CreatedAfter: base.Add(3 * time.Minute)},
		"dep-g", "dep-f", "dep-e")
	assertIDs("createdBefore", DeployFilter{CreatedBefore: base.Add(2 * time.Minute)},
		"dep-b", "dep-a")
	assertIDs("window", DeployFilter{CreatedAfter: base, CreatedBefore: base.Add(3 * time.Minute)},
		"dep-c", "dep-b")
	assertIDs("status + window + limit combined",
		DeployFilter{Statuses: []string{DeployLive}, CreatedBefore: base.Add(5 * time.Minute), Limit: 1},
		"dep-c")
	assertIDs("limit", DeployFilter{Limit: 3}, "dep-g", "dep-f", "dep-e")
	assertIDs("cursor resumes after its row", DeployFilter{Cursor: "dep-e"},
		"dep-d", "dep-c", "dep-b", "dep-a")
	assertIDs("cursor within a same-instant pair (id tiebreak)", DeployFilter{Cursor: "dep-g"},
		"dep-f", "dep-e", "dep-d", "dep-c", "dep-b", "dep-a")
	assertIDs("unknown cursor is an empty page", DeployFilter{Cursor: "dep-doesnotexist00000"})
}

// TestListDeploysCursorWalkIsGapAndDupFree pages the full history with a
// small limit and asserts the concatenated pages are exactly the unpaged
// list — no row skipped, none repeated, including across the same-instant
// pair the id tiebreak orders.
func TestListDeploysCursorWalkIsGapAndDupFree(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	for i := range 5 {
		seedDeploy(s, fmt.Sprintf("dep-%03d", i), "srv-1", DeployLive, base.Add(time.Duration(i)*time.Minute))
	}
	// Two same-instant rows, positioned so a limit-3 page boundary splits them.
	seedDeploy(s, "dep-t1", "srv-1", DeployLive, base.Add(10*time.Minute))
	seedDeploy(s, "dep-t2", "srv-1", DeployLive, base.Add(10*time.Minute))

	full, err := s.ListDeploys(ctx, "srv-1", DeployFilter{})
	if err != nil || len(full) != 7 {
		t.Fatalf("full list = %v (err %v), want 7 rows", deployIDs(full), err)
	}

	var walked []string
	cursor := ""
	for range 10 { // bounded so a paging bug can't loop forever
		page, err := s.ListDeploys(ctx, "srv-1", DeployFilter{Cursor: cursor, Limit: 3})
		if err != nil {
			t.Fatalf("page after %q: %v", cursor, err)
		}
		if len(page) == 0 {
			break
		}
		walked = append(walked, deployIDs(page)...)
		cursor = page[len(page)-1].ID
	}
	if !slices.Equal(walked, deployIDs(full)) {
		t.Errorf("cursor walk = %v, want the unpaged list %v", walked, deployIDs(full))
	}
}

func TestListDeploysLimitClampsToMaxPageLimit(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	for i := range core.MaxPageLimit + 20 {
		seedDeploy(s, fmt.Sprintf("dep-%04d", i), "srv-1", DeployLive, base.Add(time.Duration(i)*time.Second))
	}
	got, err := s.ListDeploys(ctx, "srv-1", DeployFilter{Limit: core.MaxPageLimit + 10})
	if err != nil || len(got) != core.MaxPageLimit {
		t.Errorf("limit above the cap: got %d rows (err %v), want %d", len(got), err, core.MaxPageLimit)
	}
	// An omitted limit is bounded too (codex-security round-6 #7): the old
	// "absent = full history" contract let any viewer of a long-lived service
	// materialize an unbounded history; callers page with the keyset cursor.
	if got, err := s.ListDeploys(ctx, "srv-1", DeployFilter{}); err != nil || len(got) != core.MaxPageLimit {
		t.Errorf("no limit must clamp to the cap: got %d rows (err %v), want %d", len(got), err, core.MaxPageLimit)
	}
}

func TestGetDeployIsScopedToItsApp(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	ten, _ := s.CreateTenant(ctx, "acme", "free")
	a1, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	a2, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "api", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	d1, _, err := openDeployFor(ctx, s, a1.ID)
	if err != nil {
		t.Fatalf("open deploy for a1: %v", err)
	}

	// A deploy id that belongs to a1 must not resolve when scoped to a2 — no
	// cross-app leak through the id alone.
	if _, err := s.GetDeploy(ctx, a2.ID, d1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get a1's deploy scoped to a2: want ErrNotFound, got %v", err)
	}
	if got, err := s.GetDeploy(ctx, a1.ID, d1.ID); err != nil || got.ID != d1.ID {
		t.Errorf("get a1's deploy scoped to a1: got %+v (err %v)", got, err)
	}
	if _, err := s.GetDeploy(ctx, a1.ID, "dep-doesnotexist00000"); !errors.Is(err, ErrNotFound) {
		t.Errorf("get unknown deploy: want ErrNotFound, got %v", err)
	}
}
