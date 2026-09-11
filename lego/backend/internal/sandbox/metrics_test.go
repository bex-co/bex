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

package sandbox

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestCreateOutcomeSeparatesRefusalsFromSubstrateFailures is the whole point of
// the classifier: the alert built on these labels must not fire because a
// client sent bad requests, and must not be diluted by them either.
func TestCreateOutcomeSeparatesRefusalsFromSubstrateFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"success", nil, outcomeSucceeded},
		{"capacity refusal", sandboxCapacityError(), outcomeCapacity},
		{"wrapped capacity refusal", fmt.Errorf("create: %w", sandboxCapacityError()), outcomeCapacity},
		{"runtime not configured", core.ErrSandboxesUnavailable, outcomeUnavailable},
		{"unknown template", fmt.Errorf("%w: unknown template %q", core.ErrBadRequest, "nope"), outcomeRejected},
		{"denied", core.ErrForbidden, outcomeRejected},
		{"card required", core.ErrPaymentRequired, outcomeRejected},
		{"billing enforced", core.ErrBillingEnforced, outcomeRejected},
		{"unexplained runtime error", errors.New("connection reset by peer"), outcomeFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := createOutcome(tc.err); got != tc.want {
				t.Errorf("createOutcome(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestObserveCreateCountsEveryAttemptOnce(t *testing.T) {
	m := NewMetrics(prometheus.NewRegistry())
	m.observeCreate(nil, 2*time.Second)
	m.observeCreate(errors.New("boom"), time.Second)
	m.observeCreate(core.ErrForbidden, time.Second)

	for outcome, want := range map[string]float64{
		outcomeSucceeded: 1, outcomeFailed: 1, outcomeRejected: 1, outcomeCapacity: 0,
	} {
		if got := testutil.ToFloat64(m.creates.WithLabelValues(outcome)); got != want {
			t.Errorf("creates{outcome=%q} = %v, want %v", outcome, got, want)
		}
	}
}

// TestObserveCreateTimesOnlyRealAttempts keeps instant refusals out of the
// latency histogram: a burst of 400s would otherwise drag p95 down and mask a
// genuinely slow substrate.
func TestObserveCreateTimesOnlyRealAttempts(t *testing.T) {
	m := NewMetrics(prometheus.NewRegistry())
	m.observeCreate(core.ErrForbidden, time.Second)
	m.observeCreate(core.ErrSandboxesUnavailable, time.Second)
	if got := testutil.CollectAndCount(m.createTime); got != 0 {
		t.Fatalf("timed %d refusals, want none in the latency histogram", got)
	}

	m.observeCreate(nil, 3*time.Second)
	m.observeCreate(sandboxCapacityError(), 300*time.Second)
	if got := testutil.CollectAndCount(m.createTime); got != 2 {
		t.Fatalf("timed %d real attempts, want 2 (success and capacity)", got)
	}
}

func TestNilMetricsIsANoOp(t *testing.T) {
	var m *Metrics
	m.observeCreate(errors.New("boom"), time.Second)
	m.observeTerminate(nil)
}

// TestMetricLabelsAreAClosedSet guards the ADR088 label contract: every value
// these series can carry must come from the constant set, never from anything a
// tenant chooses.
func TestMetricLabelsAreAClosedSet(t *testing.T) {
	closed := map[string]bool{
		outcomeSucceeded: true, outcomeFailed: true, outcomeCapacity: true,
		outcomeRejected: true, outcomeUnavailable: true,
	}
	for _, err := range []error{
		nil,
		errors.New("anything at all"),
		fmt.Errorf("%w: unknown template %q", core.ErrBadRequest, "tenant-chosen-name"),
		sandboxCapacityError(),
		core.ErrSandboxesUnavailable,
	} {
		if outcome := createOutcome(err); !closed[outcome] {
			t.Errorf("createOutcome(%v) = %q, which is outside the closed label set", err, outcome)
		}
	}
}

// TestAgentSessionCreatesStayOutOfTheSandboxSurfaceSignal pins the scoping
// decision. Agent sessions share this runtime but enter through their own
// dispatch, which calls createResolved directly and reports through
// bex_agent_session_provision_seconds. If they were counted here too, a single
// substrate incident would fire two alerts and neither series would answer
// "is the /v1/sandboxes API healthy" — the question ADR088 had no signal for.
func TestAgentSessionCreatesStayOutOfTheSandboxSurfaceSignal(t *testing.T) {
	svc := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"os-1","status":{"state":"Creating"}}`))
	})
	svc.Metrics = NewMetrics(prometheus.NewRegistry())

	// The public verb reports.
	if _, err := svc.Create(callerCtx(), CreateRequest{Template: "node", Plan: PlanStandard}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := testutil.ToFloat64(svc.Metrics.creates.WithLabelValues(outcomeSucceeded)); got != 1 {
		t.Fatalf("public create counted %v times, want 1", got)
	}

	// The shared inner path the agent-session dispatch uses does not.
	if _, err := svc.createResolved(callerCtx(), "tea-a", "node", svc.Templates["node"],
		PlanStandard, "", 0, &NetworkPolicy{Default: NetworkPolicyDenyAll}, nil, nil); err != nil {
		t.Fatalf("createResolved: %v", err)
	}
	if got := testutil.ToFloat64(svc.Metrics.creates.WithLabelValues(outcomeSucceeded)); got != 1 {
		t.Fatalf("sandbox-surface counter = %v after an agent-path create, want it unchanged at 1", got)
	}
}
