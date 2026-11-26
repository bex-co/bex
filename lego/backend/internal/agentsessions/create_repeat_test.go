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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

// create_repeat_test.go is w3/m405. Draining the node of the one OpenSandbox
// lifecycle server on 2026-11-12 refused bex-api's connections for about two
// minutes, and a session that started then failed its turn with `sandbox
// create failed`. A create the runtime did not serve is now repeated within a
// bound; every other failure still ends the turn at once with its reason.

// runtimeDown is a create the sandbox runtime did not serve, as the sandbox
// lifecycle wraps it (sandbox.IsTransientCreateFailure; which answers are this
// is sandbox's TestWhichCreateFailuresTheRuntimeDidNotServe).
var runtimeDown = fmt.Errorf("opensandbox create: %w", core.NewUnavailableError(sandbox.CodeSandboxRuntimeUnavailable, "the sandbox runtime is not answering right now; retry in a few seconds", nil))

// fastRepeats makes the repeats' waits a millisecond each.
func fastRepeats(t *testing.T) {
	t.Helper()
	backoff, bound := createRepeatBackoff, createRepeatBound
	createRepeatBackoff = make([]time.Duration, len(backoff))
	for i := range createRepeatBackoff {
		createRepeatBackoff[i] = time.Millisecond
	}
	t.Cleanup(func() { createRepeatBackoff, createRepeatBound = backoff, bound })
}

// provisionSamples is how many provisioning samples outcome has.
func provisionSamples(t *testing.T, m *CompletionMetrics, outcome provisionOutcome) uint64 {
	t.Helper()
	var sample dto.Metric
	if err := m.provisionLatency.WithLabelValues(string(outcome)).(prometheus.Metric).Write(&sample); err != nil {
		t.Fatal(err)
	}
	return sample.GetHistogram().GetSampleCount()
}

func TestACreateTheRuntimeDidNotServeIsRepeatedUntilTheSessionRuns(t *testing.T) {
	fastRepeats(t)
	svc, st, _, lc := fixture()
	svc.Metrics = NewCompletionMetrics(prometheus.NewRegistry())
	lc.createQueue = []error{runtimeDown, runtimeDown, nil}
	var repeats []bool
	lc.onCreate = func(spec sandbox.AgentSessionSandbox) { repeats = append(repeats, spec.Repeat) }
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[created.ID]
	if row.Phase != PhaseRunning || row.FailureReason != "" || row.SandboxID == "" {
		t.Fatalf("after two creates the runtime did not serve the session is %s (failure %q, sandbox %q), want running", row.Phase, row.FailureReason, row.SandboxID)
	}
	// The first create is a plain one; each one after a create the runtime
	// did not serve is a repeat, which answers any sandbox that create made
	// instead of making a second one.
	if lc.createAttempts != 3 || lc.created != 1 || len(repeats) != 3 || repeats[0] || !repeats[1] || !repeats[2] {
		t.Fatalf("%d creates made %d sandboxes with repeats %v, want 3 making 1, the last two repeats", lc.createAttempts, lc.created, repeats)
	}
	if got := testutil.ToFloat64(svc.Metrics.createRepeats); got != 2 {
		t.Fatalf("repeat counter = %v, want 2", got)
	}
	// The dispatch is one provisioning sample, whatever its creates.
	if running, failed := provisionSamples(t, svc.Metrics, provisionRunning), provisionSamples(t, svc.Metrics, provisionFailed); running != 1 || failed != 0 {
		t.Fatalf("provisioning samples running %d failed %d, want one running", running, failed)
	}
}

// A redispatched turn repeats its creates the same way.
func TestARedispatchRepeatsACreateTheRuntimeDidNotServe(t *testing.T) {
	fastRepeats(t)
	svc, st, _, lc := fixture()
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[created.ID]
	row.Phase = PhaseCompleted
	st.rows[created.ID] = row
	lc.createQueue = []error{runtimeDown, nil}
	attempts := lc.createAttempts
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "keep going"}); err != nil {
		t.Fatal(err)
	}
	if row := st.rows[created.ID]; row.Phase != PhaseRunning || row.FailureReason != "" || lc.createAttempts-attempts != 2 {
		t.Fatalf("the redispatch is %s (failure %q) after %d creates, want running after 2", row.Phase, row.FailureReason, lc.createAttempts-attempts)
	}
}

// Every other failure is an answer about the sandbox: one create, and the
// turn fails with the reason and code it always had.
func TestAFailureThatIsNotTheRuntimesEndsTheTurnAtOnce(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		reason     string
		code       string
		causeWords bool
	}{
		{"another server error", errors.New("opensandbox create: status 500: boom"), "sandbox create failed", "", false},
		{"a refusal", fmt.Errorf("create: %w", core.NewBadRequestError("SANDBOX::INVALID_METADATA_LABEL", "bad label", nil)), "sandbox create failed", "", false},
		{"the capacity limit", fmt.Errorf("create: %w", core.NewConflictError(sandbox.CodeSandboxCapacityLimit, "the workspace is at its sandbox limit", nil)), sandbox.CapacityFailureReason, sandbox.CodeSandboxCapacityLimit, false},
		{"a start failure", fmt.Errorf("create: %w", core.NewUnavailableError(sandbox.CodeSandboxStartFailed, "the sandbox did not start", map[string]any{"cause": "image_pull"})), "", sandbox.CodeSandboxStartFailed, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fastRepeats(t)
			svc, st, _, lc := fixture()
			lc.createErr = tc.err
			created, err := svc.Create(caller("alice"), createInput())
			if err != nil {
				t.Fatal(err)
			}
			row := st.rows[created.ID]
			reason := tc.reason
			if tc.causeWords {
				reason = sandbox.StartFailureWords(sandbox.StartCauseImagePull, 0)
			}
			if lc.createAttempts != 1 || row.Phase != PhaseFailed || row.FailureReason != reason || row.FailureReasonCode != tc.code {
				t.Fatalf("%d creates, then %s %q code %q; want 1, then failed %q code %q", lc.createAttempts, row.Phase, row.FailureReason, row.FailureReasonCode, reason, tc.code)
			}
		})
	}
}

// A runtime that never serves the create fails the turn once the repeats are
// spent, with today's reason and no code, and the turn's sandboxes are ended.
func TestARuntimeThatNeverServesFailsTheTurnAfterTheRepeats(t *testing.T) {
	fastRepeats(t)
	svc, st, _, lc := fixture()
	svc.Metrics = NewCompletionMetrics(prometheus.NewRegistry())
	lc.createErr = runtimeDown
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[created.ID]
	if lc.createAttempts != len(createRepeatBackoff)+1 || row.Phase != PhaseFailed || row.FailureReason != "sandbox create failed" || row.FailureReasonCode != "" {
		t.Fatalf("%d creates, then %s %q code %q; want %d, then failed \"sandbox create failed\" with no code", lc.createAttempts, row.Phase, row.FailureReason, row.FailureReasonCode, len(createRepeatBackoff)+1)
	}
	if lc.cleanups == 0 {
		t.Fatal("the failed turn's sandboxes were not ended")
	}
	if got := testutil.ToFloat64(svc.Metrics.createRepeats); got != float64(len(createRepeatBackoff)) {
		t.Fatalf("repeat counter = %v, want %d", got, len(createRepeatBackoff))
	}
	if running, failed := provisionSamples(t, svc.Metrics, provisionRunning), provisionSamples(t, svc.Metrics, provisionFailed); running != 0 || failed != 1 {
		t.Fatalf("provisioning samples running %d failed %d, want one failed", running, failed)
	}
}

// A session that leaves while a repeat waits is not created again: whatever
// the failed create made is ended, and the session keeps its own state.
func TestASessionCanceledBeforeTheRepeatIsNotCreatedAgain(t *testing.T) {
	fastRepeats(t)
	svc, st, _, lc := fixture()
	lc.createErr = runtimeDown
	lc.onCreate = func(spec sandbox.AgentSessionSandbox) {
		st.rowMu.Lock()
		defer st.rowMu.Unlock()
		row := st.rows[spec.SessionID]
		row.Phase, row.Status = PhaseCanceled, PhaseCanceled
		st.rows[spec.SessionID] = row
	}
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if row := st.rows[created.ID]; lc.createAttempts != 1 || row.Phase != PhaseCanceled || lc.cleanups == 0 {
		t.Fatalf("%d creates, then %s with %d cleanups; want 1, the session still canceled and the turn's sandboxes ended", lc.createAttempts, row.Phase, lc.cleanups)
	}
}

// The repeats stop at whichever bound comes first: every wait taken, or a
// repeat that would start more than createRepeatBound after the first failure.
func TestTheRepeatsAreBoundedInCountAndTime(t *testing.T) {
	start := time.Date(2026, 11, 12, 9, 35, 11, 0, time.UTC)
	var r createRepeats
	now, total := start, time.Duration(0)
	for i, want := range createRepeatBackoff {
		delay, ok := r.next(now)
		if !ok || delay != want {
			t.Fatalf("repeat %d = %s %v, want %s", i+1, delay, ok, want)
		}
		total += delay
		now = now.Add(delay)
	}
	if _, ok := r.next(now); ok {
		t.Fatal("a repeat past the last wait")
	}
	// The waits cover the two minutes the runtime refused connections on
	// 2026-11-12 and stay inside both bounds.
	if total < 2*time.Minute || total > createRepeatBound || createRepeatBound >= dispatchTimeout/2 {
		t.Fatalf("the waits add up to %s with a %s bound, want over two minutes, inside the bound and well inside the %s dispatch", total, createRepeatBound, dispatchTimeout)
	}
	// Slow failures reach the time bound first.
	var slow createRepeats
	if _, ok := slow.next(start); !ok {
		t.Fatal("no first repeat")
	}
	if _, ok := slow.next(start.Add(createRepeatBound)); ok {
		t.Fatal("a repeat that starts past the bound")
	}
}
