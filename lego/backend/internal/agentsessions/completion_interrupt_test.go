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
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// ADR047 D12: a turn a person stopped reports `interrupted` in the driver's
// status file, and the Completer settles it as a completed turn, flagged.

func interruptedStatus(git statusGit) string {
	raw, _ := json.Marshal(statusReport{State: driverStateInterrupted, Git: git})
	return string(raw)
}

func TestCompleterSettlesAnInterruptedTurnAsCompleted(t *testing.T) {
	c, st, lc, id := completerFixture(interruptedStatus(statusGit{RemoteHead: remoteHead, Undelivered: []string{dirtyReason}}), nil)
	c.Metrics = NewCompletionMetrics(prometheus.NewRegistry())
	lc.transcriptLog = `{"at":"t","type":"ui-message","part":{"type":"text","text":"half an answer"}}`
	recordAgentPR(st, id)
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
	if err != nil || len(turns) != 1 || !turns[0].Interrupted {
		t.Fatalf("turns = %+v, %v", turns, err)
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
