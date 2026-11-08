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

package agentsessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// The scheduler against the real control-plane Postgres: the claim, the run
// table and the idempotent session create are the real ones; authorization is
// a thread-safe stand-in for OpenFGA behind the same core.Checker seam.

// schedFGA is a goroutine-safe core.Checker + WorkspaceResolver + TupleWriter
// with switchable outage and membership.
type schedFGA struct {
	mu      sync.Mutex
	roles   map[string]string // subject -> admin|developer|viewer, all in one workspace
	ws      string
	parents map[string]string
	down    bool
}

func (f *schedFGA) Check(_ context.Context, subject, relation, object string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return false, errors.New("openfga unreachable")
	}
	ws := ""
	switch {
	case strings.HasPrefix(object, "workspace:"):
		ws = strings.TrimPrefix(object, "workspace:")
	case strings.HasPrefix(object, "agent_session:"):
		ws = f.parents[strings.TrimPrefix(object, "agent_session:")]
	}
	if ws == "" || ws != f.ws {
		return false, nil
	}
	switch f.roles[strings.TrimPrefix(subject, "user:")] {
	case "admin":
		return true, nil
	case "developer":
		return relation != core.RelCanManage, nil
	case "viewer":
		return relation == core.RelCanView, nil
	}
	return false, nil
}

func (f *schedFGA) GrantAgentSessionWorkspace(_ context.Context, sessionID, workspaceID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.parents[sessionID] = workspaceID
	return nil
}

func (f *schedFGA) Tenant(_ context.Context, id core.Identity) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.roles[id.Subject]; ok {
		return f.ws, true
	}
	return "", false
}

func (f *schedFGA) IsMember(_ context.Context, id core.Identity, workspace string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.roles[id.Subject]
	return ok && workspace == f.ws, nil
}

func (f *schedFGA) setDown(down bool) {
	f.mu.Lock()
	f.down = down
	f.mu.Unlock()
}

func (f *schedFGA) remove(subject string) {
	f.mu.Lock()
	delete(f.roles, subject)
	f.mu.Unlock()
}

type schedClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *schedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *schedClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type schedEnv struct {
	t     *testing.T
	pool  *pgxpool.Pool
	pg    *store.PGStore
	ws    string
	svc   *Service
	fga   *schedFGA
	clock *schedClock
}

// newSchedEnv builds a Service over the real store whose sessions are accepted
// (row, tuple) but never provisioned: they stay in the working phase until a
// test finishes them, like a long agent run.
func newSchedEnv(t *testing.T, name string) *schedEnv {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	pg := store.NewPGStore(pool)
	tenant, err := pg.CreateTenant(ctx, fmt.Sprintf("%s-%d", name, time.Now().UnixNano()), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM agent_session_dispatches WHERE workspace_id=$1`, tenant.ID)
		_ = pg.DeleteTenant(ctx, tenant.ID)
	})
	// The scheduler claims every workspace's due automations; leftovers of other
	// tests in a reused database would take its batch, so retire them.
	if _, err := pool.Exec(ctx, `UPDATE agent_automations SET enabled=false WHERE workspace_id <> $1 AND enabled`, tenant.ID); err != nil {
		t.Fatal(err)
	}
	fga := &schedFGA{ws: tenant.ID, parents: map[string]string{},
		roles: map[string]string{"alice": "developer", "bob": "developer", "root": "admin", "eve": "viewer"}}
	clock := &schedClock{t: time.Now().UTC().Truncate(time.Minute)}
	svc := &Service{
		Base:  &core.Base{Authz: fga, Workspace: fga, Clock: clock.Now},
		Store: pg, Tuples: fga, Sandbox: &fakeLifecycle{}, TicketSecret: []byte("session-secret"), GatewayURL: "wss://ssh.bex.co/agent-sessions",
		ModelProxyURL:  "http://bex-ssh-gateway.bex-system.svc.cluster.local:8084",
		ModelKeys:      fixtureModelKeys(tenant.ID),
		Automations:    pg,
		dispatchRunner: func(func()) {},
	}
	return &schedEnv{t: t, pool: pool, pg: pg, ws: tenant.ID, svc: svc, fga: fga, clock: clock}
}

func (e *schedEnv) scheduler() *AutomationScheduler {
	return &AutomationScheduler{Service: e.svc, Now: e.clock.Now}
}

func (e *schedEnv) tick() {
	e.t.Helper()
	if err := e.scheduler().RunOnce(context.Background()); err != nil {
		e.t.Fatalf("tick: %v", err)
	}
}

func (e *schedEnv) template() AutomationTemplate {
	return AutomationTemplate{Task: "update the dependencies", Repo: "bex-co/example",
		AgentConfig: AutomationAgentConfig{Agent: "codex", Model: "gpt-5"}}
}

// automation inserts an every-5-minutes automation due at next.
func (e *schedEnv) automation(name, creator string, next time.Time) store.AgentAutomation {
	e.t.Helper()
	return e.automationWith(name, creator, next, e.template())
}

func (e *schedEnv) automationWith(name, creator string, next time.Time, tpl AutomationTemplate) store.AgentAutomation {
	e.t.Helper()
	triggers, _ := json.Marshal([]AutomationTrigger{{Type: "schedule",
		Schedule: &AutomationSchedule{Frequency: "custom", Cron: "*/5 * * * *", Timezone: "UTC"}}})
	template, _ := json.Marshal(tpl)
	row, err := e.pg.CreateAgentAutomation(context.Background(), store.AgentAutomation{
		ID: ids.New(ids.AgentAutomation), WorkspaceID: e.ws, Name: name, Enabled: true, CreatedBy: creator,
		Triggers: triggers, Template: template, NextFireAt: &next})
	if err != nil {
		e.t.Fatal(err)
	}
	return row
}

func (e *schedEnv) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *schedEnv) sessions(automationID string) int {
	return e.count(`SELECT count(*) FROM agent_sessions WHERE automation_id=$1`, automationID)
}

func (e *schedEnv) runs(automationID string, outcome string) int {
	return e.count(`SELECT count(*) FROM agent_automation_runs WHERE automation_id=$1 AND ($2 = '' OR outcome=$2)`, automationID, outcome)
}

func (e *schedEnv) finishSessions(automationID string) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), `UPDATE agent_sessions SET phase='completed' WHERE automation_id=$1`, automationID); err != nil {
		e.t.Fatal(err)
	}
}

func (e *schedEnv) reload(id string) store.AgentAutomation {
	e.t.Helper()
	row, err := e.pg.GetAgentAutomation(context.Background(), id)
	if err != nil {
		e.t.Fatal(err)
	}
	return row
}

func (e *schedEnv) latestRun(id string) store.AgentAutomationRun {
	e.t.Helper()
	rows, err := e.pg.ListRuns(context.Background(), id, time.Time{}, 1)
	if err != nil || len(rows) != 1 {
		e.t.Fatalf("latest run = %+v err=%v", rows, err)
	}
	return rows[0]
}

// (a) Four scheduler instances over ten due automations: every (automation,
// fire_at) is one run row and one session, however the replicas interleave.
func TestAutomationSchedulerConcurrentReplicas(t *testing.T) {
	e := newSchedEnv(t, "aut-concurrent")
	const automations, rounds, replicas = 10, 500, 4
	start := e.clock.Now()
	var rows []store.AgentAutomation
	for i := range automations {
		rows = append(rows, e.automation(fmt.Sprintf("job-%d", i), "alice", start.Add(-time.Minute)))
	}
	schedulers := make([]*AutomationScheduler, replicas)
	for i := range schedulers {
		schedulers[i] = e.scheduler()
	}
	for round := range rounds {
		e.clock.Advance(5 * time.Minute)
		var wg sync.WaitGroup
		for _, s := range schedulers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := s.RunOnce(context.Background()); err != nil {
					t.Errorf("round %d: %v", round, err)
				}
			}()
		}
		wg.Wait()
		// The previous fire's session finishes, so the next fire is not an overlap.
		if _, err := e.pool.Exec(context.Background(),
			`UPDATE agent_sessions SET phase='completed' WHERE workspace_id=$1 AND automation_id IS NOT NULL`, e.ws); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range rows {
		if n := e.runs(row.ID, ""); n != rounds {
			t.Fatalf("%s: %d run rows, want %d", row.Name, n, rounds)
		}
		if n := e.runs(row.ID, store.AutomationRunStarted); n != rounds {
			t.Fatalf("%s: %d started runs, want %d", row.Name, n, rounds)
		}
		if n := e.sessions(row.ID); n != rounds {
			t.Fatalf("%s: %d sessions, want %d", row.Name, n, rounds)
		}
		if n := e.count(`SELECT count(DISTINCT session_id) FROM agent_automation_runs WHERE automation_id=$1`, row.ID); n != rounds {
			t.Fatalf("%s: runs share sessions: %d distinct", row.Name, n)
		}
	}
	if n := e.count(`SELECT count(*) FROM agent_sessions WHERE workspace_id=$1`, e.ws); n != automations*rounds {
		t.Fatalf("%d sessions in the workspace, want %d", n, automations*rounds)
	}
}

// (b) The replica dies after the claim commits and before the create: the
// recovery pass, even two racing, starts exactly one session. A second
// variant dies after the create and before the settle: recovery finishes that
// fire with the session it made.
func TestAutomationSchedulerCrashRecovery(t *testing.T) {
	e := newSchedEnv(t, "aut-crash")
	ctx := context.Background()
	now := e.clock.Now()
	before := e.automation("before-create", "alice", now.Add(-time.Minute))
	after := e.automation("after-create", "alice", now.Add(-time.Minute))

	// The replica claims both, then dies.
	claimed, err := e.pg.ClaimDueAutomations(ctx, now, 20, automationLease, scheduleAdvance(now))
	if err != nil || len(claimed.Runs) != 2 {
		t.Fatalf("claim = %+v err=%v", claimed, err)
	}
	// ... the second one right after its session create.
	for _, c := range claimed.Runs {
		if c.Automation.ID != after.ID {
			continue
		}
		req := e.templateOf(c.Automation).createRequest(c.Automation, automationIdempotencyKey(c.Automation.ID, c.Run.FireAt))
		if _, err := e.svc.Create(core.WithIdentity(ctx, core.Identity{Subject: "alice", Method: "session"}), req); err != nil {
			t.Fatal(err)
		}
	}
	if e.sessions(before.ID) != 0 || e.sessions(after.ID) != 1 || e.runs(before.ID, store.AutomationRunClaimed) != 1 {
		t.Fatal("setup: unexpected state after the simulated crash")
	}

	// Inside the lease nothing is recovered.
	e.clock.Advance(30 * time.Second)
	e.tick()
	if e.sessions(before.ID) != 0 {
		t.Fatal("a run was recovered inside its lease")
	}

	// After the lease, three passes race for the same stale runs.
	e.clock.Advance(2 * time.Minute)
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.scheduler().RunOnce(ctx); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	for _, a := range []store.AgentAutomation{before, after} {
		if e.sessions(a.ID) != 1 {
			t.Fatalf("%s: %d sessions after recovery, want exactly 1", a.Name, e.sessions(a.ID))
		}
		if e.runs(a.ID, store.AutomationRunStarted) != 1 || e.runs(a.ID, store.AutomationRunClaimed) != 0 {
			t.Fatalf("%s: recovery left runs unsettled", a.Name)
		}
	}
	// The after-create run finished with the session the dead replica made.
	var sessionID string
	if err := e.pool.QueryRow(ctx, `SELECT id FROM agent_sessions WHERE automation_id=$1`, after.ID).Scan(&sessionID); err != nil {
		t.Fatal(err)
	}
	var runSession string
	if err := e.pool.QueryRow(ctx, `SELECT session_id FROM agent_automation_runs WHERE automation_id=$1 AND outcome='started'`, after.ID).Scan(&runSession); err != nil || runSession != sessionID {
		t.Fatalf("run session = %q (%v), want the crashed replica's %q", runSession, err, sessionID)
	}
}

func (e *schedEnv) templateOf(a store.AgentAutomation) AutomationTemplate {
	e.t.Helper()
	var tpl AutomationTemplate
	if err := json.Unmarshal(a.Template, &tpl); err != nil {
		e.t.Fatal(err)
	}
	return tpl
}

// (c) A five-hour outage on an every-5-minutes schedule fires once, the
// latest tick, and records the ticks it skipped.
func TestAutomationSchedulerOutageCatchUp(t *testing.T) {
	e := newSchedEnv(t, "aut-outage")
	now := e.clock.Now().Add(2 * time.Minute) // between ticks
	due := now.Truncate(5 * time.Minute).Add(-5 * time.Hour)
	a := e.automation("outage", "alice", due)
	e.clock.Advance(2 * time.Minute)
	e.tick()

	if e.runs(a.ID, "") != 1 || e.sessions(a.ID) != 1 {
		t.Fatalf("runs=%d sessions=%d, want one each", e.runs(a.ID, ""), e.sessions(a.ID))
	}
	run := e.latestRun(a.ID)
	latest := now.Truncate(5 * time.Minute)
	wantMissed := int(latest.Sub(due) / (5 * time.Minute))
	if wantMissed != 60 || !run.FireAt.Equal(latest) || run.MissedTicks != wantMissed || run.Outcome != store.AutomationRunStarted {
		t.Fatalf("run = %+v, want started at %s with %d missed ticks", run, latest, wantMissed)
	}
	row := e.reload(a.ID)
	if row.NextFireAt == nil || !row.NextFireAt.Equal(latest.Add(5*time.Minute)) {
		t.Fatalf("next_fire_at = %v, want %s", row.NextFireAt, latest.Add(5*time.Minute))
	}
	e.tick()
	if e.runs(a.ID, "") != 1 {
		t.Fatal("a second tick re-fired the caught-up automation")
	}
}

// (d) A still-working previous session skips the fire; a finished one lets
// the next fire start.
func TestAutomationSchedulerOverlap(t *testing.T) {
	e := newSchedEnv(t, "aut-overlap")
	a := e.automation("overlap", "alice", e.clock.Now().Add(-time.Minute))
	e.tick()
	if e.sessions(a.ID) != 1 || e.latestRun(a.ID).Outcome != store.AutomationRunStarted {
		t.Fatal("the first fire did not start a session")
	}
	e.clock.Advance(5 * time.Minute)
	e.tick()
	run := e.latestRun(a.ID)
	if run.Outcome != store.AutomationRunSkippedOverlap || e.sessions(a.ID) != 1 || run.SessionID != "" {
		t.Fatalf("overlapping fire = %+v, sessions=%d", run, e.sessions(a.ID))
	}
	if got := e.reload(a.ID); got.ConsecutiveFailures != 0 || !got.Enabled {
		t.Fatalf("a skip counted as a failure: %+v", got)
	}
	e.finishSessions(a.ID)
	e.clock.Advance(5 * time.Minute)
	e.tick()
	if run := e.latestRun(a.ID); run.Outcome != store.AutomationRunStarted || e.sessions(a.ID) != 2 {
		t.Fatalf("fire after the session finished = %+v, sessions=%d", run, e.sessions(a.ID))
	}
}

// (e) A creator who lost access is denied and the automation pauses; a
// transient authorization outage settles nothing and recovers.
func TestAutomationSchedulerAuthorization(t *testing.T) {
	e := newSchedEnv(t, "aut-authz")
	gone := e.automation("gone", "bob", e.clock.Now().Add(-time.Minute))
	e.fga.remove("bob")
	e.tick()
	run := e.latestRun(gone.ID)
	row := e.reload(gone.ID)
	if run.Outcome != store.AutomationRunDenied || e.sessions(gone.ID) != 0 || row.Enabled || row.PausedReason != AutomationPausedCreatorLostAccess {
		t.Fatalf("run=%+v sessions=%d automation=%+v", run, e.sessions(gone.ID), row)
	}
	e.clock.Advance(10 * time.Minute)
	e.tick()
	if e.runs(gone.ID, "") != 1 {
		t.Fatal("a paused automation fired again")
	}

	flaky := e.automation("flaky", "alice", e.clock.Now().Add(-time.Minute))
	e.fga.setDown(true)
	e.tick()
	run = e.latestRun(flaky.ID)
	row = e.reload(flaky.ID)
	if run.Outcome != store.AutomationRunClaimed || e.sessions(flaky.ID) != 0 || !row.Enabled || row.PausedReason != "" || row.ConsecutiveFailures != 0 {
		t.Fatalf("outage left run=%+v sessions=%d automation=%+v, want a claimed run and an untouched automation", run, e.sessions(flaky.ID), row)
	}
	e.fga.setDown(false)
	e.clock.Advance(2 * time.Minute) // past the lease
	e.tick()
	if run = e.latestRun(flaky.ID); run.Outcome != store.AutomationRunStarted || e.sessions(flaky.ID) != 1 {
		t.Fatalf("recovery after the outage = %+v, sessions=%d", run, e.sessions(flaky.ID))
	}
}

// (f) Three failed creates in a row pause the automation; the next tick does
// not fire it.
func TestAutomationSchedulerRepeatedFailuresPause(t *testing.T) {
	e := newSchedEnv(t, "aut-failures")
	tpl := e.template()
	tpl.Repo = "not a repository"
	a := e.automationWith("broken", "alice", e.clock.Now().Add(-time.Minute), tpl)
	for i := 1; i <= 3; i++ {
		e.tick()
		run := e.latestRun(a.ID)
		if run.Outcome != store.AutomationRunFailedCreate || run.Error == "" {
			t.Fatalf("fire %d = %+v, want failed_create with the real error", i, run)
		}
		row := e.reload(a.ID)
		if row.ConsecutiveFailures != i || row.Enabled != (i < 3) {
			t.Fatalf("after fire %d: %+v", i, row)
		}
		e.clock.Advance(5 * time.Minute)
	}
	if row := e.reload(a.ID); row.PausedReason != AutomationPausedRepeatedFailures {
		t.Fatalf("pause reason = %q", row.PausedReason)
	}
	e.tick()
	if e.runs(a.ID, "") != 3 || e.sessions(a.ID) != 0 {
		t.Fatalf("a fourth tick fired: runs=%d sessions=%d", e.runs(a.ID, ""), e.sessions(a.ID))
	}
}

// (g) A fire whose session already exists (the create replayed) is started,
// with that session.
func TestAutomationSchedulerIdempotentReplay(t *testing.T) {
	e := newSchedEnv(t, "aut-replay")
	a := e.automation("replay", "alice", e.clock.Now().Add(-time.Minute))
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "session"})
	claimed, err := e.pg.ClaimDueAutomations(ctx, e.clock.Now(), 20, automationLease, scheduleAdvance(e.clock.Now()))
	if err != nil || len(claimed.Runs) != 1 {
		t.Fatalf("claim = %+v err=%v", claimed, err)
	}
	c := claimed.Runs[0]
	key := automationIdempotencyKey(a.ID, c.Run.FireAt)
	first, err := e.svc.Create(ctx, e.templateOf(a).createRequest(c.Automation, key))
	if err != nil {
		t.Fatal(err)
	}
	// The create path itself replays too (a second replica past the pre-check).
	again, err := e.svc.Create(ctx, e.templateOf(a).createRequest(c.Automation, key))
	if err != nil || again.ID != first.ID || again.Created == nil || *again.Created {
		t.Fatalf("replay = %+v err=%v", again, err)
	}
	settled, err := e.svc.fireAutomationRun(context.Background(), c)
	if err != nil || settled.Outcome != store.AutomationRunStarted || settled.SessionID != first.ID {
		t.Fatalf("fire of a replayed key = %+v err=%v", settled, err)
	}
	if e.sessions(a.ID) != 1 {
		t.Fatalf("%d sessions, want 1", e.sessions(a.ID))
	}
}

// (h) Run now and the run list.
func TestAutomationRunNowAndRuns(t *testing.T) {
	e := newSchedEnv(t, "aut-runnow")
	ctx := func(who string) context.Context {
		return core.WithIdentity(context.Background(), core.Identity{Subject: who, Method: "session"})
	}
	next := e.clock.Now().Add(time.Hour)
	a := e.automation("manual", "alice", next)

	run, err := e.svc.RunAgentAutomationNow(ctx("alice"), e.ws, a.ID)
	if err != nil || run.Outcome != store.AutomationRunStarted || run.SessionID == "" || run.SettledAt == nil || run.AutomationID != a.ID {
		t.Fatalf("run now = %+v err=%v", run, err)
	}
	if got := e.reload(a.ID); got.NextFireAt == nil || !got.NextFireAt.Equal(next) {
		t.Fatalf("run now moved the schedule: %v", got.NextFireAt)
	}
	// The same instant is refused; a later one overlaps the working session.
	if _, err := e.svc.RunAgentAutomationNow(ctx("alice"), e.ws, a.ID); !isCode(err, CodeAutomationConflict) {
		t.Fatalf("same-instant run now = %v, want %s", err, CodeAutomationConflict)
	}
	e.clock.Advance(time.Millisecond)
	if run, err = e.svc.RunAgentAutomationNow(ctx("root"), e.ws, a.Name); err != nil || run.Outcome != store.AutomationRunSkippedOverlap {
		t.Fatalf("admin run now over a working session = %+v err=%v", run, err)
	}
	e.finishSessions(a.ID)
	e.clock.Advance(time.Millisecond)
	if run, err = e.svc.RunAgentAutomationNow(ctx("alice"), e.ws, a.ID); err != nil || run.Outcome != store.AutomationRunStarted {
		t.Fatalf("run now after finish = %+v err=%v", run, err)
	}
	if e.sessions(a.ID) != 2 {
		t.Fatalf("%d sessions, want 2", e.sessions(a.ID))
	}
	// Another developer may not; a viewer may not even list.
	e.clock.Advance(time.Millisecond)
	if _, err := e.svc.RunAgentAutomationNow(ctx("bob"), e.ws, a.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-creator developer run now = %v", err)
	}
	if _, err := e.svc.AgentAutomationRuns(ctx("eve"), e.ws, a.ID, "", 0); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("viewer runs = %v", err)
	}

	// Two more runs, then page through all four, newest first.
	for range 2 {
		e.finishSessions(a.ID)
		e.clock.Advance(time.Second)
		if _, err := e.svc.RunAgentAutomationNow(ctx("alice"), e.ws, a.ID); err != nil {
			t.Fatal(err)
		}
	}
	var seen []AgentAutomationRun
	cursor := ""
	for pages := 0; ; pages++ {
		list, err := e.svc.AgentAutomationRuns(ctx("bob"), e.ws, a.Name, cursor, 2)
		if err != nil || list.AutomationID != a.ID {
			t.Fatalf("runs page = %+v err=%v", list, err)
		}
		seen = append(seen, list.Runs...)
		if list.NextCursor == "" {
			break
		}
		if pages > 5 || len(list.Runs) != 2 {
			t.Fatalf("paging did not terminate: %+v", list)
		}
		cursor = list.NextCursor
	}
	if len(seen) != 5 {
		t.Fatalf("paged %d runs, want 5", len(seen))
	}
	for i := 1; i < len(seen); i++ {
		if !seen[i].FireAt.Before(seen[i-1].FireAt) {
			t.Fatalf("runs not newest first: %v then %v", seen[i-1].FireAt, seen[i].FireAt)
		}
	}
	if _, err := e.svc.AgentAutomationRuns(ctx("alice"), e.ws, a.ID, "yesterday", 0); !isCode(err, CodeAutomationInvalid) || !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("bad cursor = %v, want the named 400", err)
	}

	// An automation paused for a reason must be resumed first.
	paused := e.automation("paused", "alice", next)
	if _, err := e.pg.SetAgentAutomationPaused(context.Background(), paused.ID, true, AutomationPausedRepeatedFailures); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.RunAgentAutomationNow(ctx("alice"), e.ws, paused.ID); !isCode(err, CodeAutomationPaused) {
		t.Fatalf("run now on a paused automation = %v, want %s", err, CodeAutomationPaused)
	}
	// ... while one the creator merely turned off still runs.
	off := e.automation("turned-off", "alice", next)
	if _, err := e.pg.SetAgentAutomationPaused(context.Background(), off.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if run, err := e.svc.RunAgentAutomationNow(ctx("alice"), e.ws, off.ID); err != nil || run.Outcome != store.AutomationRunStarted {
		t.Fatalf("run now on a manually paused automation = %+v err=%v", run, err)
	}
}

// (i) The automation id lands on the session row and its view.
func TestAutomationSessionCarriesAutomationID(t *testing.T) {
	e := newSchedEnv(t, "aut-link")
	a := e.automation("linked", "alice", e.clock.Now().Add(-time.Minute))
	e.tick()
	run := e.latestRun(a.ID)
	record, err := e.pg.GetAgentSession(context.Background(), run.SessionID)
	if err != nil || record.AutomationID != a.ID {
		t.Fatalf("row = %+v err=%v", record, err)
	}
	view, err := e.svc.Get(core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "session"}), run.SessionID)
	if err != nil || view.AutomationID != a.ID {
		t.Fatalf("view = %+v err=%v", view, err)
	}
	if raw, _ := json.Marshal(view); !strings.Contains(string(raw), `"automationId":"`+a.ID+`"`) {
		t.Fatalf("view JSON lacks automationId: %s", raw)
	}
	if raw, _ := json.Marshal(CreateRequest{AutomationID: a.ID}); strings.Contains(string(raw), "utomation") {
		t.Fatalf("a client can set automationId through CreateRequest: %s", raw)
	}
	rows, err := e.pg.ListAgentSessions(context.Background(), e.ws, store.AgentSessionListQuery{AutomationID: a.ID, Limit: 10})
	if err != nil || len(rows) != 1 {
		t.Fatalf("list by automation = %d err=%v", len(rows), err)
	}
}

// An automation whose stored triggers no longer decode is paused, not retried
// every tick; the Run loop itself fires a due automation and stops with ctx.
func TestAutomationSchedulerUnschedulableAndRunLoop(t *testing.T) {
	e := newSchedEnv(t, "aut-loop")
	bad := e.automation("corrupt", "alice", e.clock.Now().Add(-time.Minute))
	if _, err := e.pool.Exec(context.Background(), `UPDATE agent_automations SET triggers='[]' WHERE id=$1`, bad.ID); err != nil {
		t.Fatal(err)
	}
	good := e.automation("fine", "alice", e.clock.Now().Add(-time.Minute))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s := e.scheduler()
	s.Interval = 20 * time.Millisecond
	go func() { s.Run(ctx); close(done) }()
	deadline := time.Now().Add(10 * time.Second)
	for e.sessions(good.ID) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop with its context")
	}
	if e.sessions(good.ID) != 1 {
		t.Fatalf("the loop started %d sessions", e.sessions(good.ID))
	}
	if row := e.reload(bad.ID); row.Enabled || row.PausedReason != AutomationPausedTemplateInvalid || e.runs(bad.ID, "") != 0 {
		t.Fatalf("corrupt automation = %+v runs=%d", row, e.runs(bad.ID, ""))
	}
}
