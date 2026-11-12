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
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// noticeFixture is one automation in a fresh workspace with helpers to settle
// runs the way the scheduler does.
type noticeFixture struct {
	t    *testing.T
	s    *PGStore
	pool *pgxpool.Pool
	ws   string
	a    AgentAutomation
	base time.Time
	n    int
}

func newNoticeFixture(t *testing.T, name string) *noticeFixture {
	t.Helper()
	s, pool, tenant := agentSessionPGStore(t, name)
	a, err := s.CreateAgentAutomation(context.Background(), newAutomation(tenant.ID, "notify", nil))
	if err != nil {
		t.Fatal(err)
	}
	return &noticeFixture{t: t, s: s, pool: pool, ws: tenant.ID, a: a, base: time.Now().UTC().Truncate(time.Microsecond)}
}

// settle books one run with outcome (and a session for started) and returns
// the session id.
func (f *noticeFixture) settle(outcome, sessionPhase string) string {
	f.t.Helper()
	ctx := context.Background()
	at := f.base.Add(time.Duration(f.n) * time.Minute)
	f.n++
	if _, err := f.pool.Exec(ctx, `INSERT INTO agent_automation_runs (automation_id, fire_at, outcome, claimed_until) VALUES ($1,$2,'claimed',$3)`,
		f.a.ID, at, f.base); err != nil {
		f.t.Fatal(err)
	}
	sessionID := ""
	if outcome == AutomationRunStarted {
		session, err := f.s.CreateAgentSession(ctx, AgentSession{WorkspaceID: f.ws, AutomationID: f.a.ID, InitialPrompt: "x"})
		if err != nil {
			f.t.Fatal(err)
		}
		sessionID = session.ID
		f.phase(sessionID, sessionPhase)
	}
	if _, err := f.s.SettleRun(ctx, f.a.ID, at, outcome, sessionID, "boom"); err != nil {
		f.t.Fatal(err)
	}
	return sessionID
}

func (f *noticeFixture) phase(sessionID, phase string) {
	f.t.Helper()
	if phase == "" {
		return
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE agent_sessions SET phase=$2, failure_reason=CASE WHEN $2='failed' THEN 'the agent crashed' ELSE failure_reason END WHERE id=$1`,
		sessionID, phase); err != nil {
		f.t.Fatal(err)
	}
}

// claim returns this fixture's automation's claimed notices (other tests'
// leftovers in a shared database are claimed too, and ignored).
func (f *noticeFixture) claim() []AutomationRunNotice {
	f.t.Helper()
	got, err := f.s.ClaimAutomationRunNotices(context.Background(), time.Now(), 200)
	if err != nil {
		f.t.Fatal(err)
	}
	var mine []AutomationRunNotice
	for _, n := range got {
		if n.Automation.ID == f.a.ID {
			mine = append(mine, n)
		}
	}
	return mine
}

func TestPGAutomationRunNoticesFollowRunOutcomes(t *testing.T) {
	f := newNoticeFixture(t, "automation-run-notices")
	if f.a.NotifyEmail != AutomationNotifyOnFailure || f.a.NotifyFailureStreak != 0 {
		t.Fatalf("new automation = %q streak %d, want on_failure and 0", f.a.NotifyEmail, f.a.NotifyFailureStreak)
	}

	// A started run whose session is still working is not claimed until it ends.
	working := f.settle(AutomationRunStarted, "running")
	if got := f.claim(); len(got) != 0 {
		t.Fatalf("working session claimed: %+v", got)
	}
	f.phase(working, "completed")
	got := f.claim()
	if len(got) != 1 || got[0].Result != AutomationRunResultSucceeded || got[0].Run.SessionID != working || got[0].Automation.NotifyFailureStreak != 0 {
		t.Fatalf("completed session notice = %+v", got)
	}
	if again := f.claim(); len(again) != 0 {
		t.Fatalf("a run was claimed twice: %+v", again)
	}

	// Failures count up the streak, in order; cancels leave it, and an overlap
	// skip is never claimed at all.
	f.settle(AutomationRunSkippedOverlap, "")
	steps := []struct {
		outcome, phase, result string
		streak                 int
	}{
		{AutomationRunFailedCreate, "", AutomationRunResultFailed, 1},
		{AutomationRunDenied, "", AutomationRunResultFailed, 2},
		{AutomationRunStarted, "canceled", AutomationRunResultCanceled, 2},
		{AutomationRunStarted, "failed", AutomationRunResultFailed, 3},
		{AutomationRunStarted, "completed", AutomationRunResultSucceeded, 0},
	}
	for _, step := range steps {
		f.settle(step.outcome, step.phase)
	}
	got = f.claim()
	if len(got) != len(steps) {
		t.Fatalf("claimed %d notices, want %d: %+v", len(got), len(steps), got)
	}
	for i, step := range steps {
		n := got[i]
		if n.Run.Outcome != step.outcome || n.Result != step.result || n.Automation.NotifyFailureStreak != step.streak {
			t.Fatalf("step %d: notice = outcome %q result %q streak %d, want %q %q %d",
				i, n.Run.Outcome, n.Result, n.Automation.NotifyFailureStreak, step.outcome, step.result, step.streak)
		}
	}
	if got[3].SessionFailureReason != "the agent crashed" || got[0].Run.Error != "boom" || got[3].EndedAt.IsZero() {
		t.Fatalf("failure details = %q / %q / ended %v", got[3].SessionFailureReason, got[0].Run.Error, got[3].EndedAt)
	}

	// A started run whose session row is gone is neither success nor failure.
	f.settle(AutomationRunFailedCreate, "")
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE agent_automation_runs SET outcome='started', session_id='ags-gone' WHERE automation_id=$1 AND outcome='failed_create' AND notified_at IS NULL`, f.a.ID); err != nil {
		t.Fatal(err)
	}
	if got = f.claim(); len(got) != 1 || got[0].Result != AutomationRunResultCanceled || got[0].Automation.NotifyFailureStreak != 0 {
		t.Fatalf("missing session notice = %+v", got)
	}
}

// Concurrent claimers split the pending runs: every run is claimed by exactly
// one of them, and the streak counts every failure once.
func TestPGAutomationRunNoticesConcurrentClaims(t *testing.T) {
	f := newNoticeFixture(t, "automation-run-notices-concurrent")
	const runs = 30
	for range runs {
		f.settle(AutomationRunFailedCreate, "")
	}
	var mu sync.Mutex
	seen := map[time.Time]int{}
	var wg sync.WaitGroup
	// FOR UPDATE ... SKIP LOCKED answers an empty claim while another claimer
	// holds the automation's row, so an empty claim is not "nothing left": a
	// claimer keeps trying until every run is claimed or the deadline passes.
	deadline := time.Now().Add(10 * time.Second)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				got, err := f.s.ClaimAutomationRunNotices(context.Background(), time.Now(), 5)
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				for _, n := range got {
					if n.Automation.ID == f.a.ID {
						seen[n.Run.FireAt]++
					}
				}
				done := len(seen) >= runs
				mu.Unlock()
				if done || time.Now().After(deadline) {
					return
				}
				if len(got) == 0 {
					time.Sleep(5 * time.Millisecond)
				}
			}
		}()
	}
	wg.Wait()
	if len(seen) != runs {
		t.Fatalf("claimed %d distinct runs, want %d", len(seen), runs)
	}
	for at, n := range seen {
		if n != 1 {
			t.Fatalf("run %s claimed %d times", at, n)
		}
	}
	row, err := f.s.GetAgentAutomation(context.Background(), f.a.ID)
	if err != nil || row.NotifyFailureStreak != runs {
		t.Fatalf("streak = %d err=%v, want %d", row.NotifyFailureStreak, err, runs)
	}
}

func TestPGAutomationPauseNotices(t *testing.T) {
	f := newNoticeFixture(t, "automation-pause-notices")
	ctx := context.Background()
	claimMine := func() []AgentAutomation {
		t.Helper()
		got, err := f.s.ClaimAutomationPauseNotices(ctx, time.Now(), 200)
		if err != nil {
			t.Fatal(err)
		}
		var mine []AgentAutomation
		for _, a := range got {
			if a.ID == f.a.ID {
				mine = append(mine, a)
			}
		}
		return mine
	}

	// A manual pause carries no reason: it is not a platform pause.
	manual, err := f.s.SetAgentAutomationPaused(ctx, f.a.ID, true, "")
	if err != nil || manual.PausedAt != nil {
		t.Fatalf("manual pause = %+v err=%v", manual, err)
	}
	if got := claimMine(); len(got) != 0 {
		t.Fatalf("manual pause notified: %+v", got)
	}

	paused, err := f.s.SetAgentAutomationPaused(ctx, f.a.ID, true, AutomationPausedRepeatedFailures)
	if err != nil || paused.PausedAt == nil {
		t.Fatalf("platform pause = %+v err=%v", paused, err)
	}
	listed, err := f.s.ListPausedAutomationsForPush(ctx, paused.PausedAt.Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range listed {
		found = found || a.ID == f.a.ID
	}
	if !found {
		t.Fatalf("paused automation not listed for push: %+v", listed)
	}
	got := claimMine()
	if len(got) != 1 || got[0].PausedReason != AutomationPausedRepeatedFailures || got[0].PauseNotifiedAt == nil || got[0].Version != paused.Version {
		t.Fatalf("pause notice = %+v (a notice must not bump the version %d)", got, paused.Version)
	}
	if again := claimMine(); len(again) != 0 {
		t.Fatalf("a pause was notified twice: %+v", again)
	}

	// Pausing an already paused automation keeps its pause instant.
	repeat, err := f.s.SetAgentAutomationPaused(ctx, f.a.ID, true, AutomationPausedRepeatedFailures)
	if err != nil || repeat.PausedAt == nil || !repeat.PausedAt.Equal(*paused.PausedAt) {
		t.Fatalf("repeated pause moved paused_at: %+v err=%v", repeat, err)
	}
	if again := claimMine(); len(again) != 0 {
		t.Fatalf("a repeated pause notified again: %+v", again)
	}

	// Resume clears the pause and the email failure streak; a new pause is a new notice.
	if _, err := f.pool.Exec(ctx, `UPDATE agent_automations SET notify_failure_streak=4 WHERE id=$1`, f.a.ID); err != nil {
		t.Fatal(err)
	}
	resumed, err := f.s.SetAgentAutomationPaused(ctx, f.a.ID, false, "")
	if err != nil || resumed.PausedAt != nil || resumed.NotifyFailureStreak != 0 {
		t.Fatalf("resume = %+v err=%v", resumed, err)
	}
	if _, err := f.s.SetAgentAutomationPaused(ctx, f.a.ID, true, AutomationPausedCreatorLostAccess); err != nil {
		t.Fatal(err)
	}
	if got := claimMine(); len(got) != 1 || got[0].PausedReason != AutomationPausedCreatorLostAccess {
		t.Fatalf("second pause notice = %+v", got)
	}
}

func TestPGAutomationNotifyEmailIsStored(t *testing.T) {
	f := newNoticeFixture(t, "automation-notify-email")
	ctx := context.Background()
	row := f.a
	row.NotifyEmail = AutomationNotifyOnSuccess
	updated, err := f.s.UpdateAgentAutomation(ctx, row, row.Version)
	if err != nil || updated.NotifyEmail != AutomationNotifyOnSuccess {
		t.Fatalf("update = %+v err=%v", updated, err)
	}
	updated.NotifyEmail = "sometimes"
	if _, err := f.s.UpdateAgentAutomation(ctx, updated, updated.Version); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown setting = %v, want ErrInvalid", err)
	}
	created := newAutomation(f.ws, "never-mail", nil)
	created.NotifyEmail = AutomationNotifyNever
	if got, err := f.s.CreateAgentAutomation(ctx, created); err != nil || got.NotifyEmail != AutomationNotifyNever {
		t.Fatalf("create with never = %+v err=%v", got, err)
	}
}

// The migration's backfill: runs settled before it are not mailed, except a
// started run whose session is still working.
func TestAgentAutomationNotificationsMigrationRoundTrip(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	f := newNoticeFixture(t, "automation-notify-migration")
	finished := f.settle(AutomationRunStarted, "completed")
	working := f.settle(AutomationRunStarted, "running")
	f.settle(AutomationRunFailedCreate, "")
	ctx := context.Background()
	read := func(name string) string {
		raw, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(sql string) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	exec(read("0236_agent_automation_notifications.down.sql"))
	exec(read("0236_agent_automation_notifications.up.sql"))
	pending := map[string]bool{}
	rows, err := tx.Query(ctx, `SELECT coalesce(session_id, outcome) FROM agent_automation_runs WHERE automation_id=$1 AND notified_at IS NULL`, f.a.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatal(err)
		}
		pending[key] = true
	}
	rows.Close()
	if len(pending) != 1 || !pending[working] || pending[finished] {
		t.Fatalf("after the backfill, pending runs = %v, want only the working session %s", pending, working)
	}
	var notify string
	if err := tx.QueryRow(ctx, `SELECT notify_email FROM agent_automations WHERE id=$1`, f.a.ID).Scan(&notify); err != nil || notify != AutomationNotifyOnFailure {
		t.Fatalf("backfilled notify_email = %q err=%v", notify, err)
	}
	exec(read("0236_agent_automation_notifications.down.sql"))
	exec(read("0236_agent_automation_notifications.up.sql"))
}
