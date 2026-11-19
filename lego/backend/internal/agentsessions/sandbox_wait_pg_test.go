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
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A full sandbox pool makes a session wait for a machine and start when one
// frees, instead of failing its turn after the first create (ADR047 §
// w10/m497). These run against the real store, with the sandbox fake.

func startFailure(cause sandbox.StartCause) error {
	return core.NewUnavailableError(sandbox.CodeSandboxStartFailed, "the sandbox did not start", map[string]any{"cause": string(cause)})
}

// waitEnv is a session store on Postgres whose waits are fast.
func waitEnv(t *testing.T) lineageEnv {
	t.Helper()
	e := newLineageEnv(t)
	prevBackoff, prevWatch, prevBound := sandboxWaitBackoff, sandboxWaitWatch, sandboxWaitBound
	sandboxWaitBackoff, sandboxWaitWatch = time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { sandboxWaitBackoff, sandboxWaitWatch, sandboxWaitBound = prevBackoff, prevWatch, prevBound })
	e.svc.Metrics = NewCompletionMetrics(prometheus.NewRegistry())
	return e
}

func (e lineageEnv) row(t *testing.T, id string) store.AgentSession {
	t.Helper()
	row, err := e.pg.GetAgentSession(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

// dispatchNextCheck is when recovery would next look at the session's turn
// dispatch; ok is false when it has none.
func (e lineageEnv) dispatchNextCheck(t *testing.T, id string, turn int) (time.Time, bool) {
	t.Helper()
	var next time.Time
	err := e.pg.Pool.QueryRow(context.Background(), `SELECT next_check_at FROM agent_session_dispatches WHERE session_id=$1 AND turn=$2`, id, turn).Scan(&next)
	if err != nil {
		return time.Time{}, false
	}
	return next, true
}

func TestAFullPoolMakesTheSessionWaitThenStart(t *testing.T) {
	e := waitEnv(t)
	e.lc.createQueue = []error{startFailure(sandbox.StartCausePoolFull), startFailure(sandbox.StartCausePoolFull)}
	var during []View
	e.lc.onCreate = func(spec sandbox.AgentSessionSandbox) {
		if e.lc.createAttempts < 2 {
			return
		}
		// Every create after the first runs while the session waits: every
		// surface's view says so, and the dispatch is kept alive.
		row := e.row(t, spec.SessionID)
		view, err := viewOf(row)
		if err != nil {
			t.Fatal(err)
		}
		during = append(during, view)
		if got := testutil.ToFloat64(e.svc.Metrics.sandboxWaiting.WithLabelValues("pool_full")); got != 1 {
			t.Errorf("waiting gauge = %v while the session waits, want 1", got)
		}
		if next, ok := e.dispatchNextCheck(t, spec.SessionID, row.Turns); !ok || time.Until(next) < dispatchTimeout-time.Minute {
			t.Errorf("a waiting dispatch is due at %v (found %v); it must stay a full dispatchTimeout ahead", next, ok)
		}
	}
	created := e.create(t, "")
	if len(during) != 2 {
		t.Fatalf("the session was read waiting %d times, want 2 (two creates after the first)", len(during))
	}
	for _, view := range during {
		if view.WaitingFor != WaitingForSandboxCapacity || view.WaitingSince == nil || view.State != StateWorking || view.Phase != PhaseCreating {
			t.Fatalf("while waiting the view is waitingFor=%q since=%v state=%s phase=%s, want sandbox_capacity, a time, working, creating",
				view.WaitingFor, view.WaitingSince, view.State, view.Phase)
		}
	}
	row := e.row(t, created.ID)
	if row.Phase != PhaseRunning || row.FailureReason != "" || row.SandboxWaitSince != nil {
		t.Fatalf("after the wait the session is %s (failure %q, waiting since %v), want running and no longer waiting", row.Phase, row.FailureReason, row.SandboxWaitSince)
	}
	if view, _ := viewOf(row); view.WaitingFor != "" || view.WaitingSince != nil {
		t.Fatalf("a running session still reads waiting: %+v", view)
	}
	if e.lc.created != 1 || e.lc.createAttempts != 3 {
		t.Fatalf("created %d sandboxes in %d creates, want 1 in 3", e.lc.created, e.lc.createAttempts)
	}
	// Before each new create the turn's sandboxes were ended, so a wait
	// never leaves two for one turn.
	if e.lc.cleanups < 2 {
		t.Fatalf("the turn's sandboxes were ended %d times between creates, want one before each of the 2 retries", e.lc.cleanups)
	}
	if got := testutil.ToFloat64(e.svc.Metrics.sandboxWaiting.WithLabelValues("pool_full")); got != 0 {
		t.Fatalf("waiting gauge = %v after the session started, want 0", got)
	}
	if got := testutil.CollectAndCount(e.svc.Metrics.sandboxWait); got != 1 {
		t.Fatalf("wait histogram series = %d, want 1 (started)", got)
	}
	// Each create that ended for want of room is a provision of its own.
	if got := histogramCount(t, e.svc.Metrics.provisionLatency.WithLabelValues(string(provisionWaiting)).(prometheus.Histogram)); got != 2 {
		t.Fatalf("provisions that ended waiting = %d, want 2", got)
	}
}

func TestACancelDuringTheWaitEndsItWithNoSandbox(t *testing.T) {
	e := waitEnv(t)
	e.lc.createQueue = []error{startFailure(sandbox.StartCausePoolFull), startFailure(sandbox.StartCausePoolFull), nil}
	e.lc.onCreate = func(spec sandbox.AgentSessionSandbox) {
		if e.lc.createAttempts == 2 {
			if _, err := e.svc.Cancel(caller("alice"), spec.SessionID); err != nil {
				t.Errorf("cancel while waiting: %v", err)
			}
		}
	}
	created := e.create(t, "")
	row := e.row(t, created.ID)
	if row.Phase != PhaseCanceled {
		t.Fatalf("a session canceled while it waited is %s, want canceled", row.Phase)
	}
	if e.lc.created != 0 || e.lc.createAttempts != 2 {
		t.Fatalf("created %d sandboxes in %d creates, want none in 2: the wait must end at the next check", e.lc.created, e.lc.createAttempts)
	}
	if view, _ := viewOf(row); view.WaitingFor != "" {
		t.Fatalf("a canceled session reads waiting: %+v", view)
	}
}

// A cancel while a create is in flight ends that create too: the watcher on
// the session row cancels its context.
func TestACancelEndsTheCreateInFlight(t *testing.T) {
	e := waitEnv(t)
	e.lc.createQueue = []error{startFailure(sandbox.StartCausePoolFull)}
	blocked, canceled := make(chan struct{}), make(chan struct{})
	e.svc.Sandbox = &blockingCreate{fakeLifecycle: e.lc, after: 1, entered: blocked}
	go func() {
		defer close(canceled)
		<-blocked
		row, err := e.pg.ListAgentSessions(context.Background(), e.ws, store.AgentSessionListQuery{Limit: 1})
		if err != nil || len(row) == 0 {
			t.Errorf("list: %v", err)
			return
		}
		if _, err := e.svc.Cancel(caller("alice"), row[0].ID); err != nil {
			t.Errorf("cancel: %v", err)
		}
	}()
	done := make(chan View)
	go func() { done <- e.create(t, "") }()
	select {
	case created := <-done:
		<-canceled // Cancel's own second write settles the phase.
		if got := e.row(t, created.ID).Phase; got != PhaseCanceled {
			t.Fatalf("phase = %s, want canceled", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the create in flight was not ended by the cancel")
	}
}

// blockingCreate answers its first `after` creates from the fake and blocks
// every later one until its context ends.
type blockingCreate struct {
	*fakeLifecycle
	after   int
	entered chan struct{}
}

func (b *blockingCreate) CreateAgentSessionSandbox(ctx context.Context, spec sandbox.AgentSessionSandbox) (sandbox.Sandbox, error) {
	if b.after > 0 {
		b.after--
		return b.fakeLifecycle.CreateAgentSessionSandbox(ctx, spec)
	}
	close(b.entered)
	<-ctx.Done()
	return sandbox.Sandbox{}, ctx.Err()
}

func TestARecoveryTickDuringTheWaitDoesNotAbandonIt(t *testing.T) {
	e := waitEnv(t)
	e.lc.createQueue = []error{startFailure(sandbox.StartCausePoolFull), startFailure(sandbox.StartCausePoolFull)}
	c := &Completer{Store: e.pg, Sandbox: e.lc, Metrics: NewCompletionMetrics(prometheus.NewRegistry())}
	e.lc.onCreate = func(spec sandbox.AgentSessionSandbox) {
		ctx := context.Background()
		switch e.lc.createAttempts {
		case 1:
			// The turn was accepted more than dispatchTimeout ago, as it is
			// after a few creates' worth of waiting: without a keep-alive,
			// recovery would abandon it now.
			if _, err := e.pg.Pool.Exec(ctx, `UPDATE agent_session_dispatches SET next_check_at = now() - interval '1 minute' WHERE session_id=$1`, spec.SessionID); err != nil {
				t.Fatal(err)
			}
			if _, err := e.pg.Pool.Exec(ctx, `UPDATE agent_sessions SET updated_at = now() - interval '20 minutes' WHERE id=$1`, spec.SessionID); err != nil {
				t.Fatal(err)
			}
		case 2:
			c.recoverDispatches(ctx)
			if row := e.row(t, spec.SessionID); row.Phase != PhaseCreating || row.FailureReason != "" {
				t.Errorf("a recovery tick during the wait left the session %s (%q), want it still creating", row.Phase, row.FailureReason)
			}
		}
	}
	created := e.create(t, "")
	if got := e.row(t, created.ID).Phase; got != PhaseRunning {
		t.Fatalf("phase = %s, want running once a machine freed", got)
	}
}

func TestTheBoundPassingFailsWithThePoolFullCause(t *testing.T) {
	e := waitEnv(t)
	sandboxWaitBound = 300 * time.Millisecond
	e.lc.createErr = startFailure(sandbox.StartCausePoolFull)
	e.lc.onCreate = func(sandbox.AgentSessionSandbox) { time.Sleep(100 * time.Millisecond) }
	created := e.create(t, "")
	row := e.row(t, created.ID)
	if row.Phase != PhaseFailed || row.FailureReasonCode != sandbox.CodeSandboxStartFailed || row.FailureCause != string(sandbox.StartCausePoolFull) {
		t.Fatalf("after the bound the session is %s %s/%s, want failed SANDBOX_START_FAILED/pool_full", row.Phase, row.FailureReasonCode, row.FailureCause)
	}
	if !strings.HasPrefix(row.FailureReason, "No sandbox machine had room for this session for ") || row.SandboxWaitSince != nil {
		t.Fatalf("failure reason = %q (waiting since %v), want how long it waited and the wait cleared", row.FailureReason, row.SandboxWaitSince)
	}
	if e.lc.createAttempts < 2 {
		t.Fatalf("creates = %d, want the dispatch to have waited and created again", e.lc.createAttempts)
	}
	view, _ := viewOf(row)
	if view.FailureCause != "pool_full" || view.WaitingFor != "" {
		t.Fatalf("view failureCause=%q waitingFor=%q, want pool_full and no wait", view.FailureCause, view.WaitingFor)
	}
}

func TestAnImagePullFailureFailsAtOnce(t *testing.T) {
	e := waitEnv(t)
	e.lc.createQueue = []error{startFailure(sandbox.StartCauseImagePull), nil}
	created := e.create(t, "")
	row := e.row(t, created.ID)
	if e.lc.createAttempts != 1 {
		t.Fatalf("creates = %d, want 1: only a full pool waits", e.lc.createAttempts)
	}
	if row.Phase != PhaseFailed || row.FailureCause != string(sandbox.StartCauseImagePull) ||
		row.FailureReason != sandbox.StartFailureWords(sandbox.StartCauseImagePull, 0) || row.SandboxWaitSince != nil {
		t.Fatalf("session = %s %q cause %q since %v, want failed with the image-pull words", row.Phase, row.FailureReason, row.FailureCause, row.SandboxWaitSince)
	}
}

func TestAnArchiveDuringTheWaitEndsIt(t *testing.T) {
	e := waitEnv(t)
	e.lc.createQueue = []error{startFailure(sandbox.StartCausePoolFull), startFailure(sandbox.StartCausePoolFull), nil}
	e.lc.onCreate = func(spec sandbox.AgentSessionSandbox) {
		if e.lc.createAttempts == 2 {
			if _, err := e.svc.Archive(caller("alice"), spec.SessionID); err != nil {
				t.Errorf("archive while waiting: %v", err)
			}
		}
	}
	created := e.create(t, "")
	row := e.row(t, created.ID)
	if row.Phase != PhaseFailed || row.FailureReason != archivedWhileWaiting.Reason || e.lc.created != 0 {
		t.Fatalf("an archived waiting session is %s %q with %d sandboxes, want failed with the archived reason and none", row.Phase, row.FailureReason, e.lc.created)
	}
}

// A message to a finished session waits the same way: a redispatch is a
// dispatch.
func TestARedispatchWaitsToo(t *testing.T) {
	e := waitEnv(t)
	created := e.create(t, "")
	if _, _, err := e.pg.FinalizeAgentSession(context.Background(), created.ID, PhaseCompleted, "", "", ""); err != nil {
		t.Fatal(err)
	}
	e.lc.createQueue = []error{startFailure(sandbox.StartCausePoolFull), nil}
	if _, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "and then"}); err != nil {
		t.Fatal(err)
	}
	row := e.row(t, created.ID)
	if row.Phase != PhaseRunning || row.Turns != 2 {
		t.Fatalf("after the wait the redispatch is %s on turn %d, want running on 2", row.Phase, row.Turns)
	}
}

// A session that left before its wait began (archived while the first create
// waits) ends that create too: the pool-full mark it refused is enough, with
// no wait read to wait for.
func TestASessionThatLeftBeforeTheWaitEndsTheFirstCreate(t *testing.T) {
	e := waitEnv(t)
	e.svc.Sandbox = &archiveThenBlock{fakeLifecycle: e.lc, e: e, t: t}
	done := make(chan View)
	go func() { done <- e.create(t, "") }()
	select {
	case created := <-done:
		row := e.row(t, created.ID)
		if row.Phase != PhaseFailed || row.FailureReason != archivedWhileWaiting.Reason {
			t.Fatalf("session = %s %q, want failed with the archived reason", row.Phase, row.FailureReason)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the first create was not ended when its pool-full mark was refused")
	}
}

// archiveThenBlock archives the session, reports its pod waiting for room, and
// blocks the create until its context ends.
type archiveThenBlock struct {
	*fakeLifecycle
	e lineageEnv
	t *testing.T
}

func (a *archiveThenBlock) CreateAgentSessionSandbox(ctx context.Context, spec sandbox.AgentSessionSandbox) (sandbox.Sandbox, error) {
	if _, err := a.e.svc.Archive(caller("alice"), spec.SessionID); err != nil {
		a.t.Errorf("archive: %v", err)
	}
	spec.PoolFull()
	<-ctx.Done()
	return sandbox.Sandbox{}, ctx.Err()
}
