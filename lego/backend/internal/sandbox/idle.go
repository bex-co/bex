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
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Idle suspension (w2/m278, ADR014 D4, ADR042 D5). Render suspends a sandbox
// after idleTimeoutSeconds with no command execution or file transfer in
// progress, and the next exec or file transfer resumes it. bex-api brokers
// every exec and file transfer, so it is the activity signal; OpenSandbox's
// pause/resume is the mechanism.
const (
	// Render's idleTimeoutSeconds contract (pinned OpenAPI create-sandbox):
	// omitted means 900 s; 0 disables; a positive value is 300 through 86400.
	defaultIdleTimeout = 900
	minIdleTimeout     = 300
	maxIdleTimeout     = 86400
	// metadataIdleTimeout stamps the resolved idle timeout at create. Missing
	// (a sandbox created before w2/m278, or an agent-session sandbox) reads as
	// 0: such sandboxes never idle-suspend, which is what they reported.
	metadataIdleTimeout = "bex.co/idle-timeout-seconds"

	// IdleSweepInterval is how often the idle sweeper looks for sandboxes past
	// their idle window. A sandbox suspends at most this long after it.
	IdleSweepInterval = 30 * time.Second
	// activityHeartbeat re-touches a sandbox while one of its exec streams or
	// file transfers is open, so an in-progress command is never idle. It is
	// well under the 300 s minimum idle timeout.
	activityHeartbeat = time.Minute
	// activityTouchTimeout bounds one heartbeat or closing touch.
	activityTouchTimeout = 10 * time.Second
	// suspendClaimGrace is how long an exec that finds a fresh sweeper claim
	// on a still-running sandbox waits for the claimed pause to show up. A
	// claim older than this belongs to a sweeper that died before pausing.
	suspendClaimGrace = 30 * time.Second
	// resumeRetryInterval spaces repeated resume calls on a sandbox that still
	// reads suspended (a lagging controller, or a ResumeFailed retry).
	resumeRetryInterval = 10 * time.Second
	defaultResumePoll   = time.Second

	// defaultResumeWait bounds how long a request blocks on a resuming
	// sandbox. It stays inside bex-api's 30 s HTTP write timeout, so GraphQL
	// and MCP callers get the named SANDBOX_RESUMING answer rather than a cut
	// connection. The REST exec and file routes extend their write deadline
	// and wait LongResumeWait instead (allowLongResume).
	defaultResumeWait = 20 * time.Second
	// LongResumeWait is the REST routes' bound: a rootfs-snapshot resume pulls
	// the snapshot image and schedules a new pod, which can take a minute or
	// more on a cold node.
	LongResumeWait = 3 * time.Minute
)

// ActivityStore is the durable activity record behind idle suspension.
// *store.PGStore implements it. nil (store off) disables idle suspension; a
// suspended sandbox is still resumed by the next exec.
type ActivityStore interface {
	TouchSandboxActivity(ctx context.Context, workspaceID, sandboxID string, at time.Time) (*time.Time, error)
	ClaimSandboxIdleSuspend(ctx context.Context, workspaceID, sandboxID string, since, cutoff, now, staleBefore time.Time) (bool, error)
	ClearSandboxSuspendClaim(ctx context.Context, workspaceID, sandboxID string) error
	PruneSandboxActivity(ctx context.Context, workspaceID string, live []string) error
}

// resolveIdleTimeout applies Render's idleTimeoutSeconds contract. requested
// nil is an omitted value (Render's 900 s default).
func resolveIdleTimeout(requested *int) (int, error) {
	if requested == nil {
		return defaultIdleTimeout, nil
	}
	idle := *requested
	if idle != 0 && (idle < minIdleTimeout || idle > maxIdleTimeout) {
		return 0, core.NewBadRequestError("SANDBOX_IDLE_TIMEOUT_INVALID",
			fmt.Sprintf("sandbox idleTimeoutSeconds must be 0 (disabled) or between %d and %d", minIdleTimeout, maxIdleTimeout),
			map[string]any{"field": "idleTimeoutSeconds", "minimumNonZero": minIdleTimeout, "max": maxIdleTimeout})
	}
	return idle, nil
}

// idleTimeoutFromMetadata reads bex.co/idle-timeout-seconds. Missing or
// malformed values mean 0 (never idle-suspend), never an invented window.
func idleTimeoutFromMetadata(v string) int {
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

type resumeWaitKey struct{}

// withResumeWait sets how long a resume-on-exec may block this request.
func withResumeWait(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, resumeWaitKey{}, d)
}

func resumeWait(ctx context.Context) time.Duration {
	if d, ok := ctx.Value(resumeWaitKey{}).(time.Duration); ok && d > 0 {
		return d
	}
	return defaultResumeWait
}

// allowLongResume lets a REST exec or file route wait LongResumeWait for a
// suspended sandbox to resume. It pushes the connection's write deadline out
// by that wait (net/http's WriteTimeout would otherwise cut the response at
// 30 s) and carries the longer bound on the request context.
func allowLongResume(w http.ResponseWriter, r *http.Request) *http.Request {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(LongResumeWait + 30*time.Second))
	return r.WithContext(withResumeWait(r.Context(), LongResumeWait))
}

// CodeSandboxResuming answers a request whose sandbox did not finish resuming
// within the request's wait. The resume keeps going; a retry finds it running.
const CodeSandboxResuming = "SANDBOX_RESUMING"

func sandboxResumingError(id string, waited time.Duration) error {
	return core.NewUnavailableError(CodeSandboxResuming,
		fmt.Sprintf("sandbox %s was suspended for inactivity and is still resuming after %ds; retry the command in a few seconds", id, int(waited.Seconds())),
		map[string]any{"id": id, "status": string(StatusResuming)})
}

func sandboxResumeFailedError(id string) error {
	return core.NewUnavailableError("SANDBOX_RESUME_FAILED",
		fmt.Sprintf("sandbox %s was suspended for inactivity and failed to resume; retry, or terminate it and create a new sandbox", id),
		map[string]any{"id": id})
}

// idleTracked reports whether a sandbox takes part in idle suspension.
// Agent-session sandboxes do not: the agent-sessions feature owns their
// lifecycle (idle grace teardown, ADR047), and their stamp is absent.
func idleTracked(raw osSandbox) bool {
	return raw.Metadata[metadataAgentSession] == ""
}

// touchActivity records one exec or file interaction. A store failure is
// logged and never refuses the interaction: activity is a billing
// optimization, not an authorization input.
func (s *Service) touchActivity(ctx context.Context, ws, sandboxID string) *time.Time {
	if s.Activity == nil {
		return nil
	}
	claimed, err := s.Activity.TouchSandboxActivity(ctx, ws, sandboxID, s.Now())
	if err != nil {
		log.Printf("sandbox activity: touch workspace=%s sandbox=%s: %v", ws, sandboxID, err)
		return nil
	}
	return claimed
}

func (s *Service) clearSuspendClaim(ctx context.Context, ws, sandboxID string) {
	if s.Activity == nil {
		return
	}
	if err := s.Activity.ClearSandboxSuspendClaim(ctx, ws, sandboxID); err != nil {
		log.Printf("sandbox activity: clear claim workspace=%s sandbox=%s: %v", ws, sandboxID, err)
	}
}

// holdActive keeps a sandbox active for as long as one exec stream or file
// transfer is open: Render counts a sandbox idle only when no command or
// transfer is in progress. The returned stop re-touches the sandbox, so the
// idle window runs from the end of the command, as on Render.
func (s *Service) holdActive(ctx context.Context, ws string, raw osSandbox) func() {
	if s.Activity == nil || !idleTracked(raw) {
		return func() {}
	}
	// Detached from the request so the closing touch lands after a client
	// hangs up, but bounded so a stalled store never pins the handler.
	base := context.WithoutCancel(ctx)
	touch := func() {
		ctx, cancel := context.WithTimeout(base, activityTouchTimeout)
		defer cancel()
		s.touchActivity(ctx, ws, raw.ID)
	}
	interval := s.ActivityHeartbeat
	if interval <= 0 {
		interval = activityHeartbeat
	}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				touch()
			}
		}
	})
	return sync.OnceFunc(func() {
		close(done)
		wg.Wait()
		touch()
	})
}

// ensureRunning is wake-on-exec. It records the interaction, then makes sure
// the sandbox is running before an exec or file transfer is served: a
// suspended sandbox is resumed and the request blocks (within its resume
// wait) until OpenSandbox reports it running. A sandbox the idle sweeper has
// just claimed is waited for and resumed too, so a command never lands in a
// sandbox that is being paused. Other states (creating, errored, terminated)
// pass through unchanged for the gateway to answer as before.
func (s *Service) ensureRunning(ctx context.Context, key, ws string, raw osSandbox) (osSandbox, error) {
	if !idleTracked(raw) {
		return raw, nil
	}
	claimed := s.touchActivity(ctx, ws, raw.ID)
	budget := resumeWait(ctx)
	deadline := s.Now().Add(budget)
	poll := s.ResumePoll
	if poll <= 0 {
		poll = defaultResumePoll
	}
	var lastResume time.Time
	resumed := false
	for {
		now := s.Now()
		switch mapOpenSandboxStatus(raw.Status.State) {
		case StatusRunning:
			if claimed == nil {
				return raw, nil
			}
			if now.Sub(*claimed) >= suspendClaimGrace {
				// The sweeper that claimed it never paused it.
				s.clearSuspendClaim(ctx, ws, raw.ID)
				return raw, nil
			}
			// A fresh claim: the sweeper's pause is about to land. Wait for it.
		case StatusSuspended:
			if lastResume.IsZero() || now.Sub(lastResume) >= resumeRetryInterval {
				if err := s.resumeForActivity(ctx, key, ws, raw); err != nil {
					return raw, err
				}
				lastResume, resumed, claimed = now, true, nil
			}
		case StatusSuspending, StatusResuming:
			// In flight; poll until it settles.
		case StatusErrored:
			if resumed {
				return raw, sandboxResumeFailedError(canonicalID(raw))
			}
			return raw, nil
		default:
			return raw, nil
		}
		if !now.Before(deadline) {
			return raw, sandboxResumingError(canonicalID(raw), budget)
		}
		if err := sleepCtx(ctx, poll); err != nil {
			return raw, err
		}
		next, err := s.Client.Get(ctx, key, raw.ID)
		if err != nil {
			if errors.Is(err, errOpenSandboxNotFound) {
				return raw, sandboxNotFound(canonicalID(raw))
			}
			return raw, err
		}
		raw = next
		if claimed != nil && mapOpenSandboxStatus(raw.Status.State) == StatusRunning {
			// Still running under a claim: re-read it, so a sweeper whose
			// pause failed (and released its claim) does not hold this
			// request for the whole grace.
			claimed = s.touchActivity(ctx, ws, raw.ID)
		}
	}
}

// resumeForActivity issues one resume for wake-on-exec. The billing gate is
// the one the explicit resume verb applies: a workspace that may not start
// compute is not resumed by a command either. A 409 means another request or
// replica is already resuming it. Like the explicit verb, it stamps the
// resume and restarts the runtime timeout (beforeResume, afterResume).
func (s *Service) resumeForActivity(ctx context.Context, key, ws string, raw osSandbox) error {
	if err := s.RequireBillingMutation(ctx, ws); err != nil {
		return err
	}
	if err := s.beforeResume(ctx, key, raw); err != nil {
		return err
	}
	switch err := s.Client.Resume(ctx, key, raw.ID); {
	case err == nil:
		// The runtime timeout restarts at the resume, as on Render.
		s.afterResume(ctx, key, raw)
	case errors.Is(err, errOpenSandboxConflict):
		// Another request or replica is resuming it and renews the expiry.
	case errors.Is(err, errOpenSandboxNotFound):
		return sandboxNotFound(canonicalID(raw))
	default:
		return err
	}
	s.clearSuspendClaim(ctx, ws, raw.ID)
	raw.Status.State = string(StatusResuming)
	s.Meter.Observe(ctx, raw)
	return nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// TenantKeyLister enumerates every workspace's OpenSandbox tenant key.
// *store.PGStore implements it (the meter uses the same listing).
type TenantKeyLister interface {
	ListSandboxTenantKeys(ctx context.Context) ([]store.SandboxTenantKey, error)
}

// IdleSuspender is the idle sweeper: on every tick it pauses each running
// sandbox whose last exec or file transfer is older than its idle timeout.
// Every bex-api replica runs one; the store claim makes exactly one of them
// pause a given sandbox.
type IdleSuspender struct {
	Service  *Service
	Tenants  TenantKeyLister
	Interval time.Duration
}

// Run sweeps until ctx ends.
func (w *IdleSuspender) Run(ctx context.Context) {
	if w == nil || w.Service == nil || !w.Service.enabled() || w.Service.Activity == nil || w.Tenants == nil {
		return
	}
	interval := w.Interval
	if interval <= 0 {
		interval = IdleSweepInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		w.Sweep(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Sweep runs one pass over every workspace and returns how many sandboxes it
// suspended.
func (w *IdleSuspender) Sweep(ctx context.Context) int {
	keys, err := w.Tenants.ListSandboxTenantKeys(ctx)
	if err != nil {
		log.Printf("sandbox idle: list tenant keys: %v", err)
		return 0
	}
	suspended := 0
	for _, key := range keys {
		if ctx.Err() != nil {
			break
		}
		suspended += w.sweepWorkspace(ctx, key)
	}
	return suspended
}

func (w *IdleSuspender) sweepWorkspace(ctx context.Context, key store.SandboxTenantKey) int {
	s := w.Service
	rows, err := s.Client.List(ctx, key.APIKey)
	if err != nil {
		log.Printf("sandbox idle: list workspace=%s: %v", key.WorkspaceID, err)
		return 0
	}
	suspended := 0
	live := make([]string, 0, len(rows))
	for _, raw := range rows {
		if raw.ID == "" {
			continue
		}
		live = append(live, raw.ID)
		if w.suspendIfIdle(ctx, key, raw) {
			suspended++
		}
	}
	// Only after a complete list: rows of sandboxes that are gone.
	if err := s.Activity.PruneSandboxActivity(ctx, key.WorkspaceID, live); err != nil {
		log.Printf("sandbox idle: prune workspace=%s: %v", key.WorkspaceID, err)
	}
	return suspended
}

func (w *IdleSuspender) suspendIfIdle(ctx context.Context, key store.SandboxTenantKey, raw osSandbox) bool {
	s := w.Service
	if !validOwnedSandbox(raw, key.WorkspaceID) || !idleTracked(raw) ||
		mapOpenSandboxStatus(raw.Status.State) != StatusRunning || raw.Created == nil {
		return false
	}
	idle := time.Duration(idleTimeoutFromMetadata(raw.Metadata[metadataIdleTimeout])) * time.Second
	now := s.Now()
	// Activity is never earlier than creation, so a sandbox younger than its
	// window cannot be idle yet; skip the store round trip.
	if idle <= 0 || now.Sub(*raw.Created) < idle {
		return false
	}
	claimed, err := s.Activity.ClaimSandboxIdleSuspend(ctx, key.WorkspaceID, raw.ID,
		*raw.Created, now.Add(-idle), now, now.Add(-suspendClaimGrace))
	if err != nil {
		log.Printf("sandbox idle: claim workspace=%s sandbox=%s: %v", key.WorkspaceID, raw.ID, err)
		return false
	}
	if !claimed {
		return false
	}
	if err := s.beforeSuspend(ctx, key.APIKey, raw); err != nil {
		s.clearSuspendClaim(ctx, key.WorkspaceID, raw.ID)
		log.Printf("sandbox idle: suspend workspace=%s sandbox=%s: %v", key.WorkspaceID, raw.ID, err)
		return false
	}
	if err := s.Client.Suspend(ctx, key.APIKey, raw.ID); err != nil {
		// A 409 means it is already pausing or resuming; leave the claim to
		// age out. Anything else failed outright, so release it now.
		if !errors.Is(err, errOpenSandboxConflict) {
			s.clearSuspendClaim(ctx, key.WorkspaceID, raw.ID)
			log.Printf("sandbox idle: suspend workspace=%s sandbox=%s: %v", key.WorkspaceID, raw.ID, err)
		}
		return false
	}
	// Close the running interval now: compute stops accruing at the moment
	// of suspension, not at the meter's next poll.
	raw.Status.State = string(StatusSuspending)
	s.Meter.Observe(ctx, raw)
	return true
}
