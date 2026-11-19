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
	"cmp"
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A session whose sandbox the pool cannot place waits for a machine instead
// of failing, as Devin's sessions wait "for capacity" (ADR047 § w10/m497).

// sandboxWaitBound is how long a dispatch waits for room on the sandbox pool
// before its turn fails. It never exceeds the turn ceiling (sandboxWaitLimit).
// A var so tests give up fast.
var sandboxWaitBound = time.Hour

// sandboxWaitBackoff spaces a waiting dispatch's creates. Each create already
// waits up to the OpenSandbox server's 300 s with its pod in the scheduler's
// queue, so this is only the gap after the server gave the pod up. A var so
// tests wait fast.
var sandboxWaitBackoff = 15 * time.Second

// sandboxWaitWatch is how often the create in flight checks, once the session
// waits, that it still does. A var so tests check fast.
var sandboxWaitWatch = 10 * time.Second

// errWaitEnded ends a wait the session left: a cancel, a delete, an archive,
// a newer turn, or recovery abandoning the dispatch.
var errWaitEnded = errors.New("the session stopped waiting for a sandbox machine")

// archivedWhileWaiting is the failure of a session archived during its wait.
var archivedWhileWaiting = store.AgentSessionFailure{Reason: "The session was archived while it waited for a sandbox machine, so it did not start."}

// waitExpired is a pool-full start failure after the whole wait: it carries
// how long the session waited, for the failure's words.
type waitExpired struct {
	err    error
	waited time.Duration
}

func (e *waitExpired) Error() string { return e.err.Error() }
func (e *waitExpired) Unwrap() error { return e.err }

// waitedOf is how long a start failure waited for a machine; 0 for one that
// did not wait.
func waitedOf(err error) time.Duration {
	var expired *waitExpired
	if errors.As(err, &expired) {
		return expired.waited
	}
	return 0
}

// WaitingForSandboxCapacity is a session view's waitingFor while no sandbox
// machine has room for its turn.
const WaitingForSandboxCapacity = "sandbox_capacity"

// The descriptions the waiting and cause fields carry on every GraphQL type
// that has them: the session and an automation run's session.
const (
	waitingForDescription   = "What a working session waits for before its turn can start: sandbox_capacity while no sandbox machine has room for it; null otherwise."
	waitingSinceDescription = "When the session began waiting for what waitingFor names; null when it does not wait."
	failureCauseDescription = "A failed start's cause, beside failureReasonCode SANDBOX_START_FAILED only: pool_full, image_pull, runtime, quota or unknown."
)

// startState is what a view shows of a session's start: what it waits for and
// since when, and a failed start's cause. A wait shows only while the turn is
// still unbound and starting; a start failure from before causes were read
// has the cause unknown.
func startState(phase, sandboxID string, waitSince *time.Time, code, cause string) (waitingFor string, since *time.Time, failureCause string) {
	if waitSince != nil && sandboxID == "" && startingPhase(phase) {
		at := waitSince.UTC()
		waitingFor, since = WaitingForSandboxCapacity, &at
	}
	if code == sandbox.CodeSandboxStartFailed {
		failureCause = cmp.Or(cause, string(sandbox.StartCauseUnknown))
	}
	return waitingFor, since, failureCause
}

// sandboxWaitLimit is the wait's bound: an hour, never past the turn ceiling.
func (s *Service) sandboxWaitLimit() time.Duration { return min(sandboxWaitBound, s.turnTimeout()) }

// dispatchCeilingKey carries a background dispatch's ceiling timer (detach).
type dispatchCeilingKey struct{}

// restartDispatchCeiling gives the dispatch a full dispatchTimeout from now.
// Only a dispatch waiting for room on the pool calls it, once per create.
func restartDispatchCeiling(ctx context.Context) {
	if timer, ok := ctx.Value(dispatchCeilingKey{}).(*time.Timer); ok {
		timer.Reset(dispatchTimeout)
	}
}

// sandboxWait is one dispatch's wait for a machine. It starts at the first
// sign the pool has no room: the pod seen unschedulable while a create waits,
// or a create that ended pool-full.
type sandboxWait struct {
	s *Service
	d store.AgentDispatch

	mu    sync.Mutex
	since time.Time // zero until the dispatch waits
	ended bool      // the session left the wait
}

func (w *sandboxWait) waiting() (time.Time, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.since, !w.since.IsZero()
}

func (w *sandboxWait) left() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ended
}

// mark records the wait and keeps the dispatch alive for another create. It
// reports false when the session left the wait.
func (w *sandboxWait) mark(ctx context.Context) bool {
	now := time.Now()
	since, ok, err := w.s.Store.MarkAgentDispatchWaiting(ctx, w.d, now, now.Add(dispatchTimeout))
	if err != nil {
		// The keep-alive did not land; the next create tries again, and the
		// dispatch still has dispatchTimeout before recovery could take it.
		log.Printf("agent-session dispatch: record the wait for a sandbox machine failed (session=%s turn=%d): %v", w.d.SessionID, w.d.Turn, err)
		return !w.left()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !ok {
		w.ended = true
		return false
	}
	if w.since.IsZero() {
		w.since = since
		w.s.Metrics.waitBegan()
		log.Printf("agent-session dispatch: waiting for a sandbox machine (session=%s turn=%d)", w.d.SessionID, w.d.Turn)
	}
	restartDispatchCeiling(ctx)
	return true
}

// watch cancels the create in flight within sandboxWaitWatch of the session
// leaving the wait: a mark it refused, or a read once the dispatch waits.
func (w *sandboxWait) watch(ctx context.Context, cancel context.CancelFunc) {
	ticker := time.NewTicker(sandboxWaitWatch)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if w.left() {
			cancel()
			return
		}
		if _, waiting := w.waiting(); !waiting {
			continue
		}
		if !w.stillWaiting(ctx) {
			w.mu.Lock()
			w.ended = true
			w.mu.Unlock()
			cancel()
			return
		}
	}
}

// stillWaiting reads the session: this turn, unbound, starting and not
// archived. A failed read keeps the wait.
func (w *sandboxWait) stillWaiting(ctx context.Context) bool {
	row, err := w.s.Store.GetAgentSession(ctx, w.d.SessionID)
	if err != nil {
		return !errors.Is(err, store.ErrNotFound)
	}
	return row.Turns == w.d.Turn && row.SandboxID == "" && row.ArchivedAt == nil && startingPhase(row.Phase)
}

// startingPhase is a turn still starting: the only phases that wait.
func startingPhase(phase string) bool { return phase == PhaseCreating || phase == PhaseRedispatching }

// createWaitingForRoom creates the dispatch's sandbox. While the pool has no
// room for it, the dispatch waits, as the session's waitingFor says, and
// creates it again, until it is placed, the session leaves the wait or
// sandboxWaitLimit passes. Before each new create it ends any sandbox of this
// turn that survived the last one, so a wait never leaves two sandboxes for
// one turn. Every other start failure ends the dispatch at once.
//
// It returns how long the dispatch waited, from the wait's start to the start
// of its last create.
func (s *Service) createWaitingForRoom(ctx context.Context, record store.AgentSession, spec dispatchSpec, image sandbox.AgentImage, env map[string]string) (sb sandbox.Sandbox, waited time.Duration, err error) {
	w := &sandboxWait{s: s, d: store.AgentDispatch{SessionID: record.ID, WorkspaceID: record.WorkspaceID, Turn: spec.turn}}
	outcome := waitOutcomeEnded
	defer func() {
		if since, waiting := w.waiting(); waiting {
			s.Metrics.observeWait(outcome, time.Since(since))
			s.Metrics.waitOver()
		}
	}()
	for {
		attempt, cancel := context.WithCancel(ctx)
		watched := make(chan struct{})
		go func() {
			defer close(watched)
			w.watch(attempt, cancel)
		}()
		started := time.Now()
		if since, waiting := w.waiting(); waiting {
			waited = started.Sub(since)
		}
		sb, err = s.createSessionSandbox(attempt, record, spec, image, env, func() { w.mark(ctx) })
		cancel()
		<-watched
		if w.left() && ctx.Err() == nil {
			// The session left while this create ran: whatever it made goes.
			s.endAttempt(ctx, w.d)
			return sandbox.Sandbox{}, waited, errWaitEnded
		}
		if !sandbox.IsPoolFull(err) {
			if err == nil {
				outcome = waitOutcomeStarted
			}
			return sb, waited, err
		}
		s.Metrics.observeProvision(provisionWaiting, time.Since(started))
		if !w.mark(ctx) {
			s.endAttempt(ctx, w.d)
			return sandbox.Sandbox{}, waited, errWaitEnded
		}
		since, _ := w.waiting()
		if total := time.Since(since); total >= s.sandboxWaitLimit() {
			log.Printf("agent-session dispatch: no sandbox machine had room within %s (session=%s turn=%d)", total.Round(time.Second), record.ID, spec.turn)
			outcome = waitOutcomeGaveUp
			return sandbox.Sandbox{}, waited, &waitExpired{err: err, waited: total}
		}
		s.endAttempt(ctx, w.d)
		select {
		case <-ctx.Done():
			return sandbox.Sandbox{}, waited, ctx.Err()
		case <-time.After(sandboxWaitBackoff):
		}
		if !w.mark(ctx) {
			return sandbox.Sandbox{}, waited, errWaitEnded
		}
	}
}

// endAttempt ends any sandbox the dispatch's last create left for its turn.
// The OpenSandbox server deletes a pod it gave up on, so this normally finds
// nothing; a failure is logged and left to dispatch recovery's tombstone.
func (s *Service) endAttempt(ctx context.Context, d store.AgentDispatch) {
	if err := s.Sandbox.CleanupAgentDispatches(ctx, []store.AgentDispatch{d}); err != nil {
		log.Printf("agent-session dispatch: end the last create's sandbox failed (session=%s turn=%d): %v", d.SessionID, d.Turn, err)
	}
}
