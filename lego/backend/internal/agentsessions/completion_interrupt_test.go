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
	"slices"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ADR047 D12: a turn a person stopped reports `interrupted` in the driver's
// status file, and the Completer settles it as a completed turn, flagged,
// provided bex-api recorded the request to stop that turn.

func interruptedStatus(git statusGit) string {
	raw, _ := json.Marshal(statusReport{State: driverStateInterrupted, Git: git})
	return string(raw)
}

// requestStop records, as Service.Interrupt does before it writes the sentinel,
// that a person asked to stop the turn.
func requestStop(t *testing.T, st *fakeStore, id string, turn int) {
	t.Helper()
	if err := st.RequestAgentSessionTurnInterrupt(context.Background(), id, turn); err != nil {
		t.Fatal(err)
	}
}

func TestCompleterSettlesAnInterruptedTurnAsCompleted(t *testing.T) {
	c, st, lc, id := completerFixture(interruptedStatus(statusGit{RemoteHead: remoteHead, Undelivered: []string{dirtyReason}}), nil)
	c.Metrics = NewCompletionMetrics(prometheus.NewRegistry())
	lc.transcriptLog = `{"at":"t","type":"ui-message","part":{"type":"text","text":"half an answer"}}`
	recordAgentPR(st, id)
	requestStop(t, st, id, 1)
	c.Reconcile(context.Background())

	row := st.rows[id]
	if row.Phase != PhaseCompleted || row.FailureReason != "" || row.FailureReasonCode != "" {
		t.Fatalf("an interrupted turn settled as %+v, want completed with no failure", row)
	}
	if !steerable(row.Phase, false) {
		t.Fatalf("the session is not steerable after the stop: %+v", row)
	}
	if !st.turns[id][1].Interrupted {
		t.Fatalf("the turn is not flagged interrupted: %+v", st.turns[id][1])
	}
	// What the agent had pushed is recorded; what it left behind is named.
	if row.HeadSHA != remoteHead || row.DeliveryNote != dirtyReason {
		t.Fatalf("head %q note %q", row.HeadSHA, row.DeliveryNote)
	}
	// The reply streamed so far is harvested before the sandbox goes, as for any turn.
	if lc.readTranscript != 1 || !lc.readTranscriptBeforeCancel || lc.canceled != 1 {
		t.Fatalf("readTranscript=%d before teardown=%v canceled=%d", lc.readTranscript, lc.readTranscriptBeforeCancel, lc.canceled)
	}
	parts, err := st.AgentSessionTranscript(context.Background(), id, -1, 1<<30, 0)
	if err != nil || len(parts) != 1 {
		t.Fatalf("the partial reply is not in the transcript: %v %v", parts, err)
	}
	if got := testutil.ToFloat64(c.Metrics.turnOutcomes.WithLabelValues(string(turnOutcomeInterrupted))); got != 1 {
		t.Fatalf("interrupted outcomes = %v, want 1", got)
	}
	if got := testutil.ToFloat64(c.Metrics.turnOutcomes.WithLabelValues(string(turnOutcomeFailed))); got != 0 {
		t.Fatalf("a stop counted as a failure")
	}
	// The next pass finds the session settled and does nothing more.
	c.Reconcile(context.Background())
	if lc.readTranscript != 1 {
		t.Fatalf("a settled turn was read again: %d", lc.readTranscript)
	}
}

// A turn that owed a structured result is not failed for lacking one: it was
// stopped before it could submit it.
func TestAnInterruptedTurnIsNotFailedForMissingItsRequiredResult(t *testing.T) {
	c, st, id := structuredCompleter(t, true, true, false)
	c.Sandbox.(*fakeLifecycle).status = interruptedStatus(statusGit{})
	requestStop(t, st, id, 1)
	c.Reconcile(context.Background())
	row := st.rows[id]
	if row.Phase != PhaseCompleted || row.FailureReasonCode != "" {
		t.Fatalf("row = %+v, want completed", row)
	}
	// The same turn that ended by itself is still failed.
	c, st, id = structuredCompleter(t, true, true, false)
	c.Reconcile(context.Background())
	if row := st.rows[id]; row.Phase != PhaseFailed || row.FailureReasonCode != FailureCodeStructuredOutputMissing {
		t.Fatalf("a finished turn without its result = %+v", row)
	}
}

// The stop, the Completer's settle and the dispatcher compose: the next queued
// message runs once the interrupted turn is settled.
func TestAnInterruptedTurnReleasesTheQueue(t *testing.T) {
	e := newQueueEnv(t)
	e.send(t, "alice", "now, please")
	if _, err := e.svc.Interrupt(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	// While the stop is only requested the queue waits.
	ctx := context.Background()
	if err := e.dispatcher().RunOnce(ctx); err != nil || len(e.prompts(t, e.id)) != 1 {
		t.Fatalf("the queue ran before the turn settled: %v %v", err, e.prompts(t, e.id))
	}
	e.lc.status = interruptedStatus(statusGit{})
	c := &Completer{Store: e.pg, Sandbox: e.lc}
	c.Reconcile(ctx)
	turns, err := e.pg.AgentSessionTurns(ctx, e.id)
	if err != nil || len(turns) != 1 || !turns[0].Interrupted || turns[0].InterruptRequestedAt == nil {
		t.Fatalf("turns = %+v, %v; want the turn flagged interrupted, with its request recorded", turns, err)
	}
	if got := e.phase(t, e.id); got != PhaseCompleted {
		t.Fatalf("phase after the stop = %s, want completed", got)
	}
	if err := e.dispatcher().RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.prompts(t, e.id); len(got) != 2 || got[1] != "now, please" {
		t.Fatalf("turns = %v; the queued message did not run after the stop", got)
	}
	// What the person streamed and what stopped stay readable on the transcript.
	page, err := e.svc.Transcript(caller("alice"), e.id, -1, 0)
	if err != nil || len(page.Turns) != 2 || !page.Turns[0].Interrupted || page.Turns[1].Interrupted {
		t.Fatalf("transcript turns = %+v, %v", page.Turns, err)
	}
}

// The status file is written by the sandbox, which is untrusted: an agent that
// writes `interrupted` itself, with no stop request from a person, would end
// its turn without the structured result it owes. The Completer believes the
// status only for a turn bex-api recorded a request for; any other takes the
// path of a finished turn, where that check applies.
func TestAnInterruptedStatusNobodyRequestedIsNotBelieved(t *testing.T) {
	c, st, id := structuredCompleter(t, true, true, false)
	c.Sandbox.(*fakeLifecycle).status = interruptedStatus(statusGit{})
	c.Reconcile(context.Background())
	row := st.rows[id]
	if row.Phase != PhaseFailed || row.FailureReasonCode != FailureCodeStructuredOutputMissing {
		t.Fatalf("a forged interrupted status skipped the structured-result check: %+v", row)
	}
	if st.turns[id][1].Interrupted {
		t.Fatalf("an unrequested status flagged the turn interrupted")
	}
}

// Without a contract to skip, an unrequested `interrupted` is a finished turn:
// completed, not flagged, counted as completed and not as a person's stop.
func TestAnUnrequestedInterruptedStatusSettlesAsAnOrdinaryTurn(t *testing.T) {
	c, st, _, id := completerFixture(interruptedStatus(statusGit{RemoteHead: remoteHead}), nil)
	c.Metrics = NewCompletionMetrics(prometheus.NewRegistry())
	recordAgentPR(st, id)
	c.Reconcile(context.Background())
	if row := st.rows[id]; row.Phase != PhaseCompleted || row.FailureReason != "" {
		t.Fatalf("row = %+v, want completed", row)
	}
	if st.turns[id][1].Interrupted {
		t.Fatalf("an unrequested status flagged the turn interrupted")
	}
	if got := testutil.ToFloat64(c.Metrics.turnOutcomes.WithLabelValues(string(turnOutcomeInterrupted))); got != 0 {
		t.Fatalf("interrupted outcomes = %v, want 0", got)
	}
	if got := testutil.ToFloat64(c.Metrics.turnOutcomes.WithLabelValues(string(turnOutcomeCompleted))); got != 1 {
		t.Fatalf("completed outcomes = %v, want 1", got)
	}
}

// A request is for one turn. The stop recorded for turn 1 does not make the
// sandbox's `interrupted` believable for turn 2, whose own contract then
// applies: the agent cannot reuse an earlier stop to skip its result.
func TestAStopRequestedForOneTurnDoesNotCoverTheNext(t *testing.T) {
	ctx := context.Background()
	c, st, lc, id := completerFixture(interruptedStatus(statusGit{}), nil)
	requestStop(t, st, id, 1)
	// Turn 1 ran, was stopped and settled; turn 2 starts and owes a result.
	row := st.rows[id]
	row.Phase = PhaseCompleted
	st.rows[id] = row
	if _, err := st.BeginAgentSessionTurn(ctx, id, "next", DeliveryRedispatch, PhaseRedispatching, "redispatching", store.AgentAttachmentBinding{}, store.AgentTurnOutput{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := st.RecordAgentSessionDispatch(ctx, id, "sandbox-2", PhaseRunning, "running", "", 2); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(reportSchema)
	st.setTurnOutput(id, 2, store.AgentTurnOutput{Schema: raw, Required: true})
	lc.status = interruptedStatus(statusGit{})

	c.Reconcile(ctx)
	got := st.rows[id]
	if got.Phase != PhaseFailed || got.FailureReasonCode != FailureCodeStructuredOutputMissing {
		t.Fatalf("turn 2's unrequested interrupted status = %+v, want failed STRUCTURED_OUTPUT_MISSING", got)
	}
	if st.turns[id][2].Interrupted || st.turns[id][2].InterruptRequestedAt != nil {
		t.Fatalf("turn 2 carries turn 1's stop: %+v", st.turns[id][2])
	}
	if st.turns[id][1].InterruptRequestedAt == nil {
		t.Fatalf("turn 1 lost its request")
	}
}

// stopAtSentinel reads the turn's recorded stop request at the moment the
// sentinel is written.
type stopAtSentinel struct {
	SandboxLifecycle
	pg   *store.PGStore
	seen []bool
}

func (w *stopAtSentinel) InterruptSessionTurn(ctx context.Context, workspaceID, sessionID, sandboxID string, turn int) error {
	turns, err := w.pg.AgentSessionTurns(ctx, sessionID)
	if err != nil {
		return err
	}
	w.seen = append(w.seen, turn >= 1 && turn <= len(turns) && turns[turn-1].InterruptRequestedAt != nil)
	return w.SandboxLifecycle.InterruptSessionTurn(ctx, workspaceID, sessionID, sandboxID, turn)
}

// Stop turn records its request before the sentinel exists, so the sandbox
// cannot write `interrupted` first; asking again keeps the first request; and
// the request is its turn's, not the next one's.
func TestInterruptRecordsItsRequestBeforeTheSentinelAndForItsTurnOnly(t *testing.T) {
	e := newQueueEnv(t)
	ctx := context.Background()
	watch := &stopAtSentinel{SandboxLifecycle: e.lc, pg: e.pg}
	e.svc.Sandbox = watch
	e.svc.Stops = &StopSignal{}
	e.send(t, "alice", "next")

	if _, err := e.svc.Interrupt(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if len(watch.seen) != 1 || !watch.seen[0] {
		t.Fatalf("the request was recorded before the sentinel: %v, want [true]", watch.seen)
	}
	// The Completer is told, so it settles the turn on its short tick.
	if got := e.svc.Stops.Pending(time.Now()); len(got) != 1 || got[0] != e.id {
		t.Fatalf("the stop was not signalled to the Completer: %v", got)
	}
	turns, err := e.pg.AgentSessionTurns(ctx, e.id)
	if err != nil || len(turns) != 1 || turns[0].InterruptRequestedAt == nil || turns[0].Interrupted {
		t.Fatalf("turns = %+v, %v; want a request recorded and the turn not yet flagged", turns, err)
	}
	first := *turns[0].InterruptRequestedAt
	if _, err := e.svc.Interrupt(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if again, _ := e.pg.AgentSessionTurns(ctx, e.id); !again[0].InterruptRequestedAt.Equal(first) {
		t.Fatalf("asking again moved the request: %v then %v", first, again[0].InterruptRequestedAt)
	}

	// The turn settles stopped; the queued message runs as turn 2, which no one asked to stop.
	e.lc.status = interruptedStatus(statusGit{})
	(&Completer{Store: e.pg, Sandbox: e.lc}).Reconcile(ctx)
	if err := e.dispatcher().RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if turns, err := e.pg.AgentSessionTurns(ctx, e.id); err != nil || len(turns) != 2 || turns[1].InterruptRequestedAt != nil {
		t.Fatalf("turns = %+v, %v; turn 2 must carry no request", turns, err)
	}
	// So the sandbox's own `interrupted` for turn 2 settles it as an ordinary turn.
	(&Completer{Store: e.pg, Sandbox: e.lc}).Reconcile(ctx)
	turns, err = e.pg.AgentSessionTurns(ctx, e.id)
	if err != nil || !turns[0].Interrupted || turns[1].Interrupted {
		t.Fatalf("turns = %+v, %v; want only turn 1 flagged interrupted", turns, err)
	}
	if got := e.phase(t, e.id); got != PhaseCompleted {
		t.Fatalf("phase = %s, want completed", got)
	}
}

// The signal Service.Interrupt hands the Completer is stamped by the service's
// own clock, the one the Completer reads Pending with: under an injected clock
// the request is still being watched, not already past its window.
func TestInterruptNotesTheStopOnTheServicesOwnClock(t *testing.T) {
	e := newQueueEnv(t)
	ahead := time.Now().Add(24 * time.Hour)
	e.svc.Clock = func() time.Time { return ahead }
	e.svc.Stops = &StopSignal{}
	if _, err := e.svc.Interrupt(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if got := e.svc.Stops.Pending(e.svc.Now()); len(got) != 1 || got[0] != e.id {
		t.Fatalf("pending on the service's clock = %v, want the stopped session: the signal was stamped by another clock", got)
	}
}

// A person's Stop turn is settled on the short tick, not the 15 s one: the
// Completer reads a session Service.Interrupt noted within StopInterval, while
// the ordinary tick is far away (w22/m413 t001).
func TestAStopRequestedSessionIsSettledOnTheShortTick(t *testing.T) {
	c, st, lc, id := completerFixture(interruptedStatus(statusGit{RemoteHead: remoteHead}), nil)
	c.Metrics = NewCompletionMetrics(prometheus.NewRegistry())
	recordAgentPR(st, id)
	requestStop(t, st, id, 1)
	c.Stops = &StopSignal{}
	c.Stops.Note(id, time.Now())
	c.Interval = time.Hour
	c.StopInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	// The ordinary tick is an hour away: only the short tick can settle it.
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done

	if got := st.rows[id].Phase; got != PhaseCompleted || !st.turns[id][1].Interrupted {
		t.Fatalf("the stop was not settled on the short tick: phase %s, turn %+v", got, st.turns[id][1])
	}
	if lc.readTranscript != 1 {
		t.Fatalf("transcript read %d times, want once", lc.readTranscript)
	}
	if pending := c.Stops.Pending(time.Now()); len(pending) != 0 {
		t.Fatalf("a settled session is still watched: %v", pending)
	}
}

// getFailing makes the next reads of one session fail with err.
type getFailing struct {
	Store
	err      error
	failures int
}

func (g *getFailing) GetAgentSession(ctx context.Context, id string) (store.AgentSession, error) {
	if g.failures > 0 {
		g.failures--
		return store.AgentSession{}, g.err
	}
	return g.Store.GetAgentSession(ctx, id)
}

// The short look at a stopped session ends when the session is gone or has
// left its live phases, and not when one read of it fails: a transient database
// error keeps the request, so the next short tick settles the turn instead of
// leaving it to the 15 s tick (the watch window bounds the retries).
func TestTheShortTickKeepsAStopRequestWhenTheSessionReadFailsTransiently(t *testing.T) {
	for name, tc := range map[string]struct {
		err         error
		stillWatchd bool
	}{
		"a transient error":        {errors.New("connection reset by peer"), true},
		"the session is not found": {store.ErrNotFound, false},
	} {
		t.Run(name, func(t *testing.T) {
			c, st, _, id := completerFixture(interruptedStatus(statusGit{RemoteHead: remoteHead}), nil)
			c.Metrics = NewCompletionMetrics(prometheus.NewRegistry())
			recordAgentPR(st, id)
			requestStop(t, st, id, 1)
			c.Stops = &StopSignal{}
			c.Stops.Note(id, time.Now())
			c.Store = &getFailing{Store: st, err: tc.err, failures: 1}

			c.reconcileStopped(context.Background())
			if got := c.Stops.Pending(time.Now()); (len(got) == 1) != tc.stillWatchd {
				t.Fatalf("after a failed read (%v) the request is watched: %v, want %v", tc.err, got, tc.stillWatchd)
			}
			if !tc.stillWatchd {
				return
			}
			// The next short tick reads the session again and settles the turn.
			c.reconcileStopped(context.Background())
			if st.rows[id].Phase != PhaseCompleted || !st.turns[id][1].Interrupted {
				t.Fatalf("the retried look did not settle the stop: phase %s, turn %+v", st.rows[id].Phase, st.turns[id][1])
			}
			if got := c.Stops.Pending(time.Now()); len(got) != 0 {
				t.Fatalf("a settled session is still watched: %v", got)
			}
		})
	}
}

// settledReadsFail fails every read that finds the session out of its live
// phases while down is set: the read after a settle that could not be made.
type settledReadsFail struct {
	Store
	down bool
}

func (g *settledReadsFail) GetAgentSession(ctx context.Context, id string) (store.AgentSession, error) {
	record, err := g.Store.GetAgentSession(ctx, id)
	if err == nil && g.down && !slices.Contains(activePhases, record.Phase) {
		return store.AgentSession{}, errors.New("connection reset by peer")
	}
	return record, err
}

// A failed read after the settle keeps the request too, until a read can say
// the session has left its live phases.
func TestTheShortTickKeepsAStopRequestWhenTheReadAfterTheSettleFails(t *testing.T) {
	c, st, _, id := completerFixture(interruptedStatus(statusGit{RemoteHead: remoteHead}), nil)
	c.Metrics = NewCompletionMetrics(prometheus.NewRegistry())
	recordAgentPR(st, id)
	requestStop(t, st, id, 1)
	c.Stops = &StopSignal{}
	c.Stops.Note(id, time.Now())
	flaky := &settledReadsFail{Store: st, down: true}
	c.Store = flaky

	c.reconcileStopped(context.Background())
	if st.rows[id].Phase != PhaseCompleted {
		t.Fatalf("the stop was not settled: phase %s", st.rows[id].Phase)
	}
	if got := c.Stops.Pending(time.Now()); len(got) != 1 {
		t.Fatalf("pending after a failed read = %v, want the request kept", got)
	}
	// The database comes back: the next look finds the session settled and lets it go.
	flaky.down = false
	c.reconcileStopped(context.Background())
	if got := c.Stops.Pending(time.Now()); len(got) != 0 {
		t.Fatalf("a settled session is still watched: %v", got)
	}
}

// Without a noted stop the short tick reads nothing: a running turn is still
// watched by the ordinary tick alone.
func TestTheShortTickReadsNothingForASessionNobodyStopped(t *testing.T) {
	c, _, lc, _ := completerFixture(interruptedStatus(statusGit{}), nil)
	c.Stops = &StopSignal{}
	c.reconcileStopped(context.Background())
	if lc.readTranscript != 0 {
		t.Fatalf("the short tick read a session nobody stopped: transcript %d", lc.readTranscript)
	}
}

// A request that nobody settles stops being watched after its window, so the
// set cannot grow without bound.
func TestAStopRequestIsForgottenAfterItsWindow(t *testing.T) {
	s := &StopSignal{}
	now := time.Now()
	s.Note("agt-old", now.Add(-stopWatchWindow-time.Second))
	s.Note("agt-new", now)
	got := s.Pending(now)
	if len(got) != 1 || got[0] != "agt-new" {
		t.Fatalf("pending = %v, want only the recent request", got)
	}
	var none *StopSignal
	none.Note("x", now)
	none.Done("x")
	if none.Pending(now) != nil {
		t.Fatal("a nil StopSignal reported something")
	}
}
