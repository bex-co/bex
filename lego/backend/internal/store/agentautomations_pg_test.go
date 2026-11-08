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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func newAutomation(ws, name string, next *time.Time) AgentAutomation {
	return AgentAutomation{ID: ids.New(ids.AgentAutomation), WorkspaceID: ws, Name: name, Enabled: true,
		CreatedBy: "alice", Triggers: json.RawMessage(`[{"type":"schedule","cron":"*/5 * * * *"}]`),
		Template: json.RawMessage(`{"task":"t"}`), NextFireAt: next}
}

// everyMinute is a caller-supplied advance: run the due tick, schedule the
// next one a minute later.
func everyMinute(a AgentAutomation) (time.Time, *time.Time, int, error) {
	fire := *a.NextFireAt
	next := fire.Add(time.Minute)
	return fire, &next, 0, nil
}

func runRows(t *testing.T, pool *pgxpool.Pool, id string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM agent_automation_runs WHERE automation_id=$1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPGAgentAutomationsCRUD(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-automations")
	ctx := context.Background()
	next := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	created, err := s.CreateAgentAutomation(ctx, newAutomation(tenant.ID, "nightly", &next))
	if err != nil || created.Version != 1 || created.OverlapPolicy != "skip" || !created.Enabled ||
		created.NextFireAt == nil || !created.NextFireAt.Equal(next) || created.PausedReason != "" || created.LastSessionID != "" {
		t.Fatalf("create = %+v err=%v", created, err)
	}
	if _, err := s.CreateAgentAutomation(ctx, newAutomation(tenant.ID, "nightly", nil)); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate name = %v, want ErrConflict", err)
	}
	if _, err := s.CreateAgentAutomation(ctx, newAutomation(tenant.ID, "Bad--Name", nil)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad name = %v, want ErrInvalid", err)
	}
	got, err := s.GetAgentAutomation(ctx, created.ID)
	if err != nil || got.ID != created.ID || string(got.Template) != `{"task": "t"}` {
		t.Fatalf("get = %+v err=%v", got, err)
	}
	if _, err := s.GetAgentAutomation(ctx, ids.New(ids.AgentAutomation)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get missing = %v", err)
	}
	byName, err := s.GetAgentAutomationByName(ctx, tenant.ID, "nightly")
	if err != nil || byName.ID != created.ID {
		t.Fatalf("by name = %+v err=%v", byName, err)
	}
	if _, err := s.GetAgentAutomationByName(ctx, "elsewhere", "nightly"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("by name in another workspace = %v", err)
	}

	edit := created
	edit.Name, edit.Enabled, edit.NextFireAt = "weekly", false, nil
	edit.Template = json.RawMessage(`{"task":"u"}`)
	updated, err := s.UpdateAgentAutomation(ctx, edit, 1)
	if err != nil || updated.Version != 2 || updated.Name != "weekly" || updated.Enabled || updated.NextFireAt != nil {
		t.Fatalf("update = %+v err=%v", updated, err)
	}
	if _, err := s.UpdateAgentAutomation(ctx, edit, 1); !errors.Is(err, ErrAgentAutomationVersion) {
		t.Fatalf("stale version = %v", err)
	}
	ghost := newAutomation(tenant.ID, "ghost", nil)
	if _, err := s.UpdateAgentAutomation(ctx, ghost, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update missing = %v", err)
	}
	other, err := s.CreateAgentAutomation(ctx, newAutomation(tenant.ID, "other", nil))
	if err != nil {
		t.Fatal(err)
	}
	clash := other
	clash.Name = "weekly"
	if _, err := s.UpdateAgentAutomation(ctx, clash, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("rename onto a taken name = %v", err)
	}

	// Pause and resume.
	paused, err := s.SetAgentAutomationPaused(ctx, other.ID, true, "repeated_failures")
	if err != nil || paused.Enabled || paused.PausedReason != "repeated_failures" {
		t.Fatalf("pause = %+v err=%v", paused, err)
	}
	if _, err := s.SetAgentAutomationPaused(ctx, other.ID, true, "bogus"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bogus reason = %v", err)
	}
	if _, err := s.SetAgentAutomationPaused(ctx, ids.New(ids.AgentAutomation), false, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pause missing = %v", err)
	}

	// Keyset paging, newest first, with a creator filter.
	bob := newAutomation(tenant.ID, "bobs", nil)
	bob.CreatedBy = "bob"
	if _, err := s.CreateAgentAutomation(ctx, bob); err != nil {
		t.Fatal(err)
	}
	var seen []string
	cursor := ""
	for {
		page, err := s.ListAgentAutomations(ctx, tenant.ID, "", cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range page {
			seen = append(seen, a.Name)
		}
		if len(page) < 2 {
			break
		}
		cursor = page[len(page)-1].ID
	}
	if fmt.Sprint(seen) != "[bobs other weekly]" {
		t.Fatalf("paged list = %v", seen)
	}
	mine, err := s.ListAgentAutomations(ctx, tenant.ID, "bob", "", 10)
	if err != nil || len(mine) != 1 || mine[0].Name != "bobs" {
		t.Fatalf("filtered list = %+v err=%v", mine, err)
	}
	if page, err := s.ListAgentAutomations(ctx, tenant.ID, "", "aut-unknown", 10); err != nil || len(page) != 0 {
		t.Fatalf("unknown cursor = %+v err=%v", page, err)
	}

	if err := s.DeleteAgentAutomation(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAgentAutomation(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v", err)
	}
}

// ADR101 D3: a tick is claimed once, the schedule advances in the claiming
// transaction, and a duplicate (automation, fire_at) is rejected by the key.
func TestPGAgentAutomationClaimAndRuns(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "automation-claim")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	due := now.Add(-time.Minute)
	a, err := s.CreateAgentAutomation(ctx, newAutomation(tenant.ID, "claimme", &due))
	if err != nil {
		t.Fatal(err)
	}
	future := now.Add(time.Hour)
	if _, err := s.CreateAgentAutomation(ctx, newAutomation(tenant.ID, "later", &future)); err != nil {
		t.Fatal(err)
	}
	off := newAutomation(tenant.ID, "off", &due)
	off.Enabled = false
	if _, err := s.CreateAgentAutomation(ctx, off); err != nil {
		t.Fatal(err)
	}

	boom := errors.New("bad schedule")
	broken := newAutomation(tenant.ID, "broken", &due)
	broken, err = s.CreateAgentAutomation(ctx, broken)
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.ClaimDueAutomations(ctx, now, 10, time.Minute, func(x AgentAutomation) (time.Time, *time.Time, int, error) {
		if x.ID == broken.ID {
			return time.Time{}, nil, 0, boom
		}
		return everyMinute(x)
	})
	if err != nil || len(res.Runs) != 1 || len(res.Failures) != 1 || !errors.Is(res.Failures[0].Err, boom) || res.Failures[0].Automation.ID != broken.ID {
		t.Fatalf("claim = %+v err=%v", res, err)
	}
	run := res.Runs[0]
	if run.Run.AutomationID != a.ID || !run.Run.FireAt.Equal(due) || run.Run.Outcome != "claimed" ||
		run.Run.ClaimedUntil == nil || !run.Run.ClaimedUntil.Equal(now.Add(time.Minute)) ||
		run.Automation.NextFireAt == nil || !run.Automation.NextFireAt.Equal(due.Add(time.Minute)) ||
		run.Automation.LastFireAt == nil || !run.Automation.LastFireAt.Equal(due) {
		t.Fatalf("claimed run = %+v", run)
	}
	if got, _ := s.GetAgentAutomation(ctx, broken.ID); got.NextFireAt == nil || !got.NextFireAt.Equal(due) || got.LastFireAt != nil {
		t.Fatalf("a failed advance must leave the row alone: %+v", got)
	}

	// A finished schedule (next nil) disables the automation.
	done, err := s.CreateAgentAutomation(ctx, newAutomation(tenant.ID, "oneshot", &due))
	if err != nil {
		t.Fatal(err)
	}
	res, err = s.ClaimDueAutomations(ctx, now, 10, time.Minute, func(x AgentAutomation) (time.Time, *time.Time, int, error) {
		if x.ID != done.ID {
			return time.Time{}, nil, 0, boom
		}
		return *x.NextFireAt, nil, 3, nil
	})
	if err != nil || len(res.Runs) != 1 || res.Runs[0].Run.MissedTicks != 3 || res.Runs[0].Automation.Enabled || res.Runs[0].Automation.NextFireAt != nil {
		t.Fatalf("oneshot claim = %+v err=%v", res, err)
	}

	// The key: a direct second insert of the same tick is refused.
	_, err = pool.Exec(ctx, `INSERT INTO agent_automation_runs (automation_id, fire_at, outcome) VALUES ($1,$2,'claimed')`, a.ID, due)
	if sqlState(err) != "23505" {
		t.Fatalf("duplicate (automation_id, fire_at) = %v, want unique violation", err)
	}
	// ...and ClaimDueAutomations tolerates a pre-existing run row: it still
	// advances the schedule but returns nothing to run.
	again := now.Add(-30 * time.Second)
	if _, err := pool.Exec(ctx, `UPDATE agent_automations SET next_fire_at=$2 WHERE id=$1`, a.ID, again); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO agent_automation_runs (automation_id, fire_at, outcome) VALUES ($1,$2,'started')`, a.ID, again); err != nil {
		t.Fatal(err)
	}
	res, err = s.ClaimDueAutomations(ctx, now, 10, time.Minute, func(x AgentAutomation) (time.Time, *time.Time, int, error) {
		if x.ID != a.ID {
			return time.Time{}, nil, 0, boom
		}
		return everyMinute(x)
	})
	if err != nil || len(res.Runs) != 0 {
		t.Fatalf("claim over an existing run = %+v err=%v", res, err)
	}
	if got, _ := s.GetAgentAutomation(ctx, a.ID); got.NextFireAt == nil || !got.NextFireAt.Equal(again.Add(time.Minute)) {
		t.Fatalf("schedule must still advance: %+v", got)
	}
	var outcome string
	if err := pool.QueryRow(ctx, `SELECT outcome FROM agent_automation_runs WHERE automation_id=$1 AND fire_at=$2`, a.ID, again).Scan(&outcome); err != nil || outcome != "started" {
		t.Fatalf("existing run must be untouched: %q %v", outcome, err)
	}

	// Deleting the automation takes its runs along.
	if err := s.DeleteAgentAutomation(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if n := runRows(t, pool, a.ID); n != 0 {
		t.Fatalf("runs after delete = %d", n)
	}
}

// The multi-replica guarantee: a row another transaction holds is skipped,
// and once that transaction commits the row is no longer due.
func TestPGAgentAutomationClaimSkipsLockedRow(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "automation-skiplocked")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	due := now.Add(-time.Minute)
	a, err := s.CreateAgentAutomation(ctx, newAutomation(tenant.ID, "held", &due))
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	var mu sync.Mutex
	advance := func(x AgentAutomation) (time.Time, *time.Time, int, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return everyMinute(x)
	}

	// Replica A: claims inside an open transaction (the schedule advances but
	// is not yet committed).
	txA, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = txA.Rollback(ctx) }()
	held, err := scanAgentAutomation(txA.QueryRow(ctx,
		`SELECT `+agentAutomationColumns+` FROM agent_automations WHERE id=$1 FOR UPDATE`, a.ID))
	if err != nil {
		t.Fatal(err)
	}

	// Replica B, concurrently: must not block and must not see the row.
	resB, err := s.ClaimDueAutomations(ctx, now, 10, time.Minute, advance)
	if err != nil || len(resB.Runs) != 0 || len(resB.Failures) != 0 || calls != 0 {
		t.Fatalf("claim while locked = %+v calls=%d err=%v", resB, calls, err)
	}

	// A finishes its claim and commits.
	fire, nxt, _, _ := everyMinute(held)
	if _, err := txA.Exec(ctx, `INSERT INTO agent_automation_runs (automation_id, fire_at, outcome) VALUES ($1,$2,'claimed')`, a.ID, fire); err != nil {
		t.Fatal(err)
	}
	if _, err := txA.Exec(ctx, `UPDATE agent_automations SET next_fire_at=$2, last_fire_at=$3 WHERE id=$1`, a.ID, nxt, fire); err != nil {
		t.Fatal(err)
	}
	if err := txA.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// B's next tick: next_fire_at already advanced past now, nothing is due.
	resB, err = s.ClaimDueAutomations(ctx, now.Add(-30*time.Second), 10, time.Minute, advance)
	if err != nil || len(resB.Runs) != 0 || calls != 0 {
		t.Fatalf("claim after commit = %+v calls=%d err=%v", resB, calls, err)
	}
	if n := runRows(t, pool, a.ID); n != 1 {
		t.Fatalf("run rows = %d, want exactly 1", n)
	}
}

// A thousand rounds of 4 concurrent claimers: every (automation, fire_at)
// gets exactly one run row and is returned to exactly one claimer.
func TestPGAgentAutomationConcurrentClaims(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "automation-concurrent")
	ctx := context.Background()
	const automations, claimers, rounds = 12, 4, 1000
	base := time.Now().UTC().Truncate(time.Second).Add(-24 * time.Hour)
	for i := range automations {
		next := base.Add(time.Duration(i) * time.Second)
		if _, err := s.CreateAgentAutomation(ctx, newAutomation(tenant.ID, fmt.Sprintf("c-%d", i), &next)); err != nil {
			t.Fatal(err)
		}
	}
	now := base.Add(time.Hour) // everything is due, round after round, as it advances a minute at a time
	var mu sync.Mutex
	got := map[string]int{}
	var wg sync.WaitGroup
	for range claimers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds / claimers * 4 {
				res, err := s.ClaimDueAutomations(ctx, now, 5, time.Minute, everyMinute)
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				for _, r := range res.Runs {
					got[r.Run.AutomationID+"@"+r.Run.FireAt.Format(time.RFC3339Nano)]++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	for k, n := range got {
		if n != 1 {
			t.Fatalf("%s claimed %d times", k, n)
		}
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_automation_runs r JOIN agent_automations a ON a.id=r.automation_id WHERE a.workspace_id=$1`, tenant.ID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != len(got) || rows < automations*10 {
		t.Fatalf("run rows = %d, returned = %d", rows, len(got))
	}
}

func TestPGAgentAutomationStaleRunsAndSettle(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "automation-settle")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	due := now.Add(-time.Minute)
	a, err := s.CreateAgentAutomation(ctx, newAutomation(tenant.ID, "settle", &due))
	if err != nil {
		t.Fatal(err)
	}
	if res, err := s.ClaimDueAutomations(ctx, now, 10, time.Minute, everyMinute); err != nil || len(res.Runs) != 1 {
		t.Fatalf("claim = %+v err=%v", res, err)
	}

	// Within its lease nothing is stale; after it, exactly one caller gets it.
	if st, err := s.ClaimStaleRuns(ctx, now, time.Minute, 10); err != nil || len(st) != 0 {
		t.Fatalf("stale inside lease = %+v err=%v", st, err)
	}
	later := now.Add(2 * time.Minute)
	results := make([][]ClaimedRun, 4)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st, err := s.ClaimStaleRuns(ctx, later, time.Minute, 10)
			if err != nil {
				t.Error(err)
			}
			results[i] = st
		}()
	}
	wg.Wait()
	total := 0
	for _, r := range results {
		total += len(r)
	}
	if total != 1 {
		t.Fatalf("concurrent stale claims returned %d runs, want 1", total)
	}
	if st, err := s.ClaimStaleRuns(ctx, later, time.Minute, 10); err != nil || len(st) != 0 {
		t.Fatalf("second stale claim = %+v err=%v", st, err)
	}
	var until time.Time
	if err := pool.QueryRow(ctx, `SELECT claimed_until FROM agent_automation_runs WHERE automation_id=$1`, a.ID).Scan(&until); err != nil || !until.Equal(later.Add(time.Minute)) {
		t.Fatalf("lease extended to %v err=%v", until, err)
	}
	if st, err := s.ClaimStaleRuns(ctx, later.Add(2*time.Minute), time.Minute, 10); err != nil || len(st) != 1 || st[0].Automation.ID != a.ID {
		t.Fatalf("stale again after the extended lease = %+v err=%v", st, err)
	}

	// Settle counters over a run of ticks.
	tick := func(i int) time.Time { return due.Add(time.Duration(-i-1) * time.Hour) }
	for i := range 5 {
		if _, err := pool.Exec(ctx, `INSERT INTO agent_automation_runs (automation_id, fire_at, outcome, claimed_until) VALUES ($1,$2,'claimed',$3)`, a.ID, tick(i), now); err != nil {
			t.Fatal(err)
		}
	}
	steps := []struct {
		outcome, session string
		want             int
	}{
		{"failed_create", "", 1}, {"denied", "", 2}, {"skipped_overlap", "", 2}, {"started", "ags-1", 0}, {"failed_create", "", 1},
	}
	for i, st := range steps {
		n, err := s.SettleRun(ctx, a.ID, tick(i), st.outcome, st.session, "e")
		if err != nil || n != st.want {
			t.Fatalf("settle %d %s = %d err=%v, want %d", i, st.outcome, n, err, st.want)
		}
	}
	got, _ := s.GetAgentAutomation(ctx, a.ID)
	if got.LastSessionID != "ags-1" || got.ConsecutiveFailures != 1 {
		t.Fatalf("automation after settles = %+v", got)
	}
	if _, err := s.SettleRun(ctx, a.ID, tick(0), "started", "ags-2", ""); !errors.Is(err, ErrAgentAutomationRunSettled) {
		t.Fatalf("second settle = %v", err)
	}
	if _, err := s.SettleRun(ctx, a.ID, tick(99), "started", "ags-2", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("settle of a missing run = %v", err)
	}
	resumed, err := s.SetAgentAutomationPaused(ctx, a.ID, false, "")
	if err != nil || resumed.ConsecutiveFailures != 0 || !resumed.Enabled || resumed.PausedReason != "" {
		t.Fatalf("resume = %+v err=%v", resumed, err)
	}

	sid, found, err := s.LatestStartedSession(ctx, a.ID)
	if err != nil || !found || sid != "ags-1" {
		t.Fatalf("latest started = %q %v %v", sid, found, err)
	}
	if _, found, err := s.LatestStartedSession(ctx, ids.New(ids.AgentAutomation)); err != nil || found {
		t.Fatalf("latest started of nothing = %v %v", found, err)
	}

	// Newest-first paging over fire_at.
	var order []time.Time
	var cursor time.Time
	for {
		page, err := s.ListRuns(ctx, a.ID, cursor, 3)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range page {
			order = append(order, r.FireAt)
		}
		if len(page) < 3 {
			break
		}
		cursor = page[len(page)-1].FireAt
	}
	if len(order) != 6 { // the claimed tick plus the five inserted above
		t.Fatalf("runs listed = %d", len(order))
	}
	for i := 1; i < len(order); i++ {
		if !order[i].Before(order[i-1]) {
			t.Fatalf("runs not strictly newest first: %v", order)
		}
	}
}

// The 0209 migration applies, reverses and re-applies cleanly.
func TestAgentAutomationsMigrationRoundTrip(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	read := func(name string) string {
		raw, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exists := func(rel string) bool {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, rel).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	hasColumn := func() bool {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_name='agent_sessions' AND column_name='automation_id')`).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	exec := func(sql string) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	// Whatever state the database is in (migrated already), go down, up, down, up.
	exec(read("0209_agent_automations.down.sql"))
	if exists("agent_automations") || exists("agent_automation_runs") || hasColumn() {
		t.Fatal("down left objects behind")
	}
	exec(read("0209_agent_automations.up.sql"))
	if !exists("agent_automations") || !exists("agent_automation_runs") || !hasColumn() || !exists("agent_automations_due_idx") {
		t.Fatal("up did not create objects")
	}
	exec(read("0209_agent_automations.down.sql"))
	exec(read("0209_agent_automations.up.sql"))
	if !exists("agent_automations") || !hasColumn() {
		t.Fatal("second up failed")
	}
}

func TestPGAgentAutomationManualRun(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "automation-manual")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	next := now.Add(time.Hour)
	a, err := s.CreateAgentAutomation(ctx, newAutomation(tenant.ID, "manual", &next))
	if err != nil {
		t.Fatal(err)
	}
	run, inserted, err := s.InsertManualRun(ctx, a.ID, now, now, time.Minute)
	if err != nil || !inserted || run.Outcome != AutomationRunClaimed || !run.FireAt.Equal(now) ||
		run.ClaimedUntil == nil || !run.ClaimedUntil.Equal(now.Add(time.Minute)) {
		t.Fatalf("insert = %+v inserted=%v err=%v", run, inserted, err)
	}
	if _, inserted, err := s.InsertManualRun(ctx, a.ID, now, now, time.Minute); err != nil || inserted {
		t.Fatalf("same fire_at again inserted=%v err=%v, want DO NOTHING", inserted, err)
	}
	if _, _, err := s.InsertManualRun(ctx, ids.New(ids.AgentAutomation), now, now, time.Minute); err == nil {
		t.Fatal("a run for a missing automation was inserted")
	}
	after, err := s.GetAgentAutomation(ctx, a.ID)
	if err != nil || after.NextFireAt == nil || !after.NextFireAt.Equal(next) {
		t.Fatalf("a manual run moved the schedule: %+v err=%v", after, err)
	}
}

func TestPGAgentSessionAutomationID(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "automation-session")
	ctx := context.Background()
	next := time.Now().Add(time.Hour)
	a, err := s.CreateAgentAutomation(ctx, newAutomation(tenant.ID, "linked", &next))
	if err != nil {
		t.Fatal(err)
	}
	linked, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, AutomationID: a.ID, InitialPrompt: "x"})
	if err != nil || linked.AutomationID != a.ID {
		t.Fatalf("linked = %+v err=%v", linked, err)
	}
	plain, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "y"})
	if err != nil || plain.AutomationID != "" {
		t.Fatalf("plain = %+v err=%v", plain, err)
	}
	got, err := s.GetAgentSession(ctx, linked.ID)
	if err != nil || got.AutomationID != a.ID {
		t.Fatalf("get = %+v err=%v", got, err)
	}
	rows, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{AutomationID: a.ID, Limit: 10})
	if err != nil || len(rows) != 1 || rows[0].ID != linked.ID {
		t.Fatalf("filtered list = %+v err=%v", rows, err)
	}
	if rows, err = s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{Limit: 10}); err != nil || len(rows) != 2 {
		t.Fatalf("unfiltered list = %d err=%v", len(rows), err)
	}
}
