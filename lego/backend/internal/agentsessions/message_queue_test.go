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
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The dispatcher's verdict on each refusal a steer can answer, against an
// in-memory queue of one message: it is dropped (with its reason) only when no
// later attempt could start it, and kept, with the reason it is blocked, when
// a person can fix the cause.

// memQueue is a one-message queue with the store's lease semantics.
type memQueue struct {
	mu        sync.Mutex
	message   store.QueuedAgentMessage
	leased    bool
	delivered bool
	blocked   string
	dropped   string
	claims    int
	counts    int
}

func newMemQueue() *memQueue {
	return &memQueue{message: store.QueuedAgentMessage{ID: "aqm-1", SessionID: "ags-1", Author: "alice", Prompt: "run the linter", CreatedAt: time.Now()}}
}

func (q *memQueue) pending() bool { return !q.delivered && q.dropped == "" }

func (q *memQueue) ClaimQueuedAgentMessages(_ context.Context, _ time.Time, _ time.Duration, _ int, _ []string, _ time.Duration) ([]store.QueuedMessageClaim, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.claims++
	if !q.pending() || q.leased {
		return nil, nil
	}
	q.leased = true
	return []store.QueuedMessageClaim{{Session: store.AgentSession{ID: "ags-1", WorkspaceID: "tea-a"}, Message: q.message}}, nil
}

func (q *memQueue) ReleaseQueuedAgentMessage(_ context.Context, _, blockedReason string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.leased, q.blocked = false, blockedReason
	return nil
}

func (q *memQueue) DropQueuedAgentMessage(_ context.Context, _, reason string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.leased, q.dropped = false, reason
	return nil
}

func (q *memQueue) CountAllQueuedAgentMessages(context.Context) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.counts++
	if q.pending() {
		return 1, nil
	}
	return 0, nil
}

// refusingSessions answers every steer with err until it is cleared; a steer
// that succeeds records the message delivered, as the turn's transaction does.
type refusingSessions struct {
	queue   *memQueue
	err     error
	started []string
}

func (s *refusingSessions) Steer(_ context.Context, req SteerRequest) (View, error) {
	if s.err != nil {
		return View{}, s.err
	}
	s.started = append(s.started, req.Prompt)
	s.queue.mu.Lock()
	s.queue.delivered = true
	s.queue.mu.Unlock()
	return View{Turns: 2}, nil
}

func TestDispatcherDropsOnlyRefusalsNoLaterAttemptCanFix(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"the author lost can_create", core.ErrForbidden},
		{"a forbidden that is not the two-factor code", core.NewForbiddenError("SOMETHING_ELSE", "no", nil)},
		{"the session is gone", core.NewNotFoundError("AGENT_SESSION_NOT_FOUND", "agent session not found", nil)},
		{"the session is archived", errArchived(PhaseCompleted)},
		{"the session is canceled", errNotSteerable(PhaseCanceled)},
		{"the prompt is refused", core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "prompt is required", nil)},
		{"the prompt history is full", core.NewConflictError(codePromptQuota, "agent session prompt history has reached its byte limit", nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := newMemQueue()
			sessions := &refusingSessions{queue: q, err: tc.err}
			d := &MessageQueue{Sessions: sessions, Store: q, Metrics: NewQueueMetrics(prometheus.NewRegistry())}
			if err := d.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if q.dropped == "" || q.dropped != truncateReason(tc.err.Error()) || q.blocked != "" {
				t.Fatalf("dropped %q blocked %q, want the message dropped with %q", q.dropped, q.blocked, tc.err.Error())
			}
			if got := testutil.ToFloat64(d.Metrics.dropped); got != 1 {
				t.Fatalf("dropped counter = %v, want 1", got)
			}
			// Nothing is left to retry: a later pass starts nothing.
			sessions.err = nil
			if err := d.RunOnce(context.Background()); err != nil || len(sessions.started) != 0 {
				t.Fatalf("a dropped message ran later: %v %v", sessions.started, err)
			}
		})
	}
}

func TestDispatcherKeepsAMessageItCouldNotStartAndRetriesIt(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		says string
	}{
		{"no model key for the agent", core.NewBadRequestError(ModelKeyRequiredCode, "the codex agent needs this workspace's own model API key, which is not set", nil), "model API key"},
		{"the agent is not available", core.NewBadRequestError(AgentUnavailableCode, "the muse agent is not available in this workspace", nil), "not available"},
		{"the author has no second factor", core.NewWorkspaceTwoFactorRequiredError("tea-a"), "two-factor"},
		{"a skill it names is gone", errSkillNotFound("lint"), "no workspace skill"},
		{"a secret it names is gone", errSecretNotFound("API_TOKEN"), "no workspace agent secret"},
		// The limits a workspace's spend and capacity set: the message waits, it
		// is never dropped, and says which one holds it.
		{"the live-sandbox limit", core.NewConflictError(codeLiveLimit, "this workspace already has 5 live agent-session sandboxes", nil), "live agent-session sandboxes"},
		{"the trust tier's concurrent-session limit", core.NewConflictError(TrustLimitCode, "this workspace can run 2 agent sessions on the platform model at once", nil), "platform model"},
		{"the muse route's capacity", core.NewConflictError(MetaRouteCapacityCode, "the muse agent is at its platform capacity", nil), "platform capacity"},
		{"no payment method", fmt.Errorf("%w: add a card", core.ErrPaymentRequired), "add a card"},
		{"the billing lifecycle", fmt.Errorf("%w: past due", core.ErrBillingEnforced), "past due"},
		{"a transient failure", errors.New("postgres down"), "postgres down"},
		{"a turn that started since the claim", errTurnInFlight(PhaseRunning), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := newMemQueue()
			sessions := &refusingSessions{queue: q, err: tc.err}
			d := &MessageQueue{Sessions: sessions, Store: q, Metrics: NewQueueMetrics(prometheus.NewRegistry())}
			ctx := context.Background()
			if err := d.RunOnce(ctx); err != nil {
				t.Fatal(err)
			}
			if q.dropped != "" || !strings.Contains(q.blocked, tc.says) || !q.pending() || q.leased {
				t.Fatalf("dropped %q blocked %q: the message must stay queued and say why (%q)", q.dropped, q.blocked, tc.says)
			}
			if got := testutil.ToFloat64(d.Metrics.dropped); got != 0 {
				t.Fatalf("a fixable refusal counted as dropped: %v", got)
			}
			// It is retried on every pass while the cause stands...
			if err := d.RunOnce(ctx); err != nil || len(sessions.started) != 0 || q.claims != 2 {
				t.Fatalf("the blocked message was not retried: claims %d started %v %v", q.claims, sessions.started, err)
			}
			// ...and delivered, exactly once, on the first pass after it clears.
			sessions.err = nil
			for range 3 {
				if err := d.RunOnce(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if !slices.Equal(sessions.started, []string{"run the linter"}) {
				t.Fatalf("started %v, want the message once", sessions.started)
			}
		})
	}
}

// A pass that finds nothing queued does not open the claim transaction: the
// cheap count sets the depth gauge and the pass ends.
func TestDispatcherSkipsTheClaimWhenNothingIsQueued(t *testing.T) {
	q := newMemQueue()
	q.delivered = true
	d := &MessageQueue{Sessions: &refusingSessions{queue: q}, Store: q, Metrics: NewQueueMetrics(prometheus.NewRegistry())}
	if err := d.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if q.claims != 0 || q.counts != 1 {
		t.Fatalf("an empty pass claimed %d times and counted %d, want 0 and 1", q.claims, q.counts)
	}
	if got := testutil.ToFloat64(d.Metrics.depth); got != 0 {
		t.Fatalf("depth = %v, want 0", got)
	}
	// With a message waiting the pass claims, delivers and recounts.
	q = newMemQueue()
	sessions := &refusingSessions{queue: q}
	d = &MessageQueue{Sessions: sessions, Store: q, Metrics: NewQueueMetrics(prometheus.NewRegistry())}
	if err := d.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if q.claims != 1 || q.counts != 2 || len(sessions.started) != 1 {
		t.Fatalf("a pass with work claimed %d, counted %d, started %v; want 1, 2, one turn", q.claims, q.counts, sessions.started)
	}
	if got := testutil.ToFloat64(d.Metrics.depth); got != 0 {
		t.Fatalf("depth after delivery = %v, want 0", got)
	}
}

// The metric label of a blocked dispatch is one of a few fixed values, never
// the error's text.
func TestBlockedReasonLabelsAreBounded(t *testing.T) {
	for want, err := range map[string]error{
		blockedLiveLimit:     core.NewConflictError(codeLiveLimit, "x", nil),
		blockedTrustLimit:    core.NewConflictError(TrustLimitCode, "x", nil),
		blockedRouteCapacity: core.NewConflictError(MetaRouteCapacityCode, "x", nil),
		blockedBilling:       fmt.Errorf("%w: x", core.ErrPaymentRequired),
		blockedOther:         core.NewBadRequestError(ModelKeyRequiredCode, "workspace-specific text", nil),
	} {
		if got := blockedReasonLabel(err); got != want {
			t.Errorf("%v labelled %q, want %q", err, got, want)
		}
	}
}

func TestBlockedReasonsAreBoundedOnARuneBoundary(t *testing.T) {
	long := strings.Repeat("é", maxBlockedReason*2)
	if got := truncateReason(long); len([]rune(got)) != maxBlockedReason || !strings.HasSuffix(got, "…") {
		t.Fatalf("truncateReason kept %d runes", len([]rune(got)))
	}
}
