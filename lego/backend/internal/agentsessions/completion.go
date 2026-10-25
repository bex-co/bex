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
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// statusReport is the driver's machine-readable status file
// (lego/agent-image/driver). Only these fields are consumed; the driver may add
// more without breaking the reader. The file is written by the untrusted
// sandbox, so nothing in it is believed beyond describing that sandbox's own
// working tree: the PR the session finalizes with is the one bex-api itself
// recorded when the agent opened it (ADR090).
type statusReport struct {
	State string    `json:"state"` // one of the driverState* constants
	Error string    `json:"error"`
	Git   statusGit `json:"git"`
}

// statusGit is the driver's post-turn view of the session's Git checkout.
// Undelivered is the driver's plain-language list of reasons the turn's work is
// not in a reviewable PR (empty when delivered) — the single source of the
// delivery note (ADR090).
type statusGit struct {
	Dirty           bool     `json:"dirty"`
	UnpushedCommits int      `json:"unpushedCommits"`
	RemoteHead      string   `json:"remoteHead"`
	Undelivered     []string `json:"undelivered"`
}

// Completer is the trusted background loop that finalizes fire-and-forget
// sessions (ADR047 D4). Each tick it reads every running session's driver
// status file through the gateway exec boundary and, on a successful turn,
// records the outcome — the PR the agent opened itself through the gateway PR
// verb (ADR090), or a readable note on what it left undelivered — and tears the
// sandbox down. It never calls GitHub. A failed turn or a lost sandbox surfaces
// as a failed session with a named reason — never a hang. It performs NO tenant
// authorization: it acts only on durable sessions the platform owns.
type Completer struct {
	Store    Store
	Sandbox  SandboxLifecycle
	Interval time.Duration
	Now      func() time.Time
	// MaxTranscriptBytes overrides the per-session cumulative transcript quota
	// (store.MaxAgentSessionTranscriptBytes); zero => the platform default.
	// Tests set it small to exercise the quota without 64 MiB payloads.
	MaxTranscriptBytes int64

	// SSHGraceTTL bounds how long an OPEN editor SSH session pins a finished
	// session's sandbox (ADR054 D6): an ssh_sessions row older than this is
	// treated as leaked (a crashed gateway replica) and ignored. It equals the
	// gateway's SSH session cap; 0 => the 4h default (BEX_SSH_SESSION_TIMEOUT's).
	SSHGraceTTL time.Duration

	// IdleTTL is the idle grace (w2/m67, w10/m15): a finished session's sandbox
	// is kept alive until it has been idle this long, where idle = now − max(last
	// turn end, last SSH disconnect) and a currently-open editor SSH session pins
	// idle at zero regardless. Past it the sandbox is terminated; the durable
	// transcript stays and the next message continues the conversation in a
	// fresh sandbox (ADR047 D3). 0 ⇒ no extra grace (reap as soon as no editor is
	// connected — byte-identical to ADR054 D6). Set from
	// BEX_AGENT_SANDBOX_IDLE_TTL (main.go default 2h); the zero value here keeps
	// tests on the pre-m67 behavior so the ADR054 D6 suite is unchanged.
	IdleTTL time.Duration

	// StatusReadFailureTTL bounds a continuous ambiguous gateway/status-read
	// failure before the session is terminalized honestly. A coded missing/dead
	// target converges immediately. Zero uses two minutes; successful reads reset
	// the streak, so a one-off gateway rollout remains transient.
	StatusReadFailureTTL time.Duration
	Metrics              *CompletionMetrics
	// SandboxMetrics reports inventory gauges and durable teardown failures.
	// nil is a working no-op.
	SandboxMetrics *sandbox.Metrics
	// InventoryInterval bounds how often reconcileSandboxInventory walks every
	// tenant's OpenSandbox list. 0 ⇒ every Completer tick (tests); production
	// sets ~1m so the 15s finalize/reap loop stays cheap.
	InventoryInterval time.Duration
	lastInventory     time.Time
	statusFailuresMu  sync.Mutex
	statusFailures    map[string]time.Time
	idleMu            sync.Mutex
	idle              map[string]idleSandbox
}

// idleSandbox is what the Completer remembers about one finished turn whose
// sandbox is waiting out its idle grace, so the reaper does not repeat work
// every tick: the turn's transcript is harvested once when the turn finishes
// (and once more right before teardown, to catch a late flush), and each
// deferral reason is logged once rather than per tick. Process-local: after a
// restart the first tick harvests again, which the idempotent merge absorbs.
type idleSandbox struct {
	sandboxID   string
	turn        int
	harvested   bool   // the turn's transcript was read and stored
	deferReason string // last deferral reason logged for this turn
}

const defaultStatusReadFailureTTL = 2 * time.Minute

func (c *Completer) statusReadFailureTTL() time.Duration {
	if c.StatusReadFailureTTL > 0 {
		return c.StatusReadFailureTTL
	}
	return defaultStatusReadFailureTTL
}

func (c *Completer) noteStatusFailure(id string) bool {
	c.statusFailuresMu.Lock()
	defer c.statusFailuresMu.Unlock()
	if c.statusFailures == nil {
		c.statusFailures = map[string]time.Time{}
	}
	since, ok := c.statusFailures[id]
	if !ok {
		c.statusFailures[id] = c.now()
		return false
	}
	return c.now().Sub(since) >= c.statusReadFailureTTL()
}

func (c *Completer) clearStatusFailure(id string) {
	c.statusFailuresMu.Lock()
	delete(c.statusFailures, id)
	c.statusFailuresMu.Unlock()
}

func (c *Completer) pruneStatusFailures(rows []store.AgentSession) {
	active := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		active[row.ID] = struct{}{}
	}
	c.statusFailuresMu.Lock()
	for id := range c.statusFailures {
		if _, ok := active[id]; !ok {
			delete(c.statusFailures, id)
		}
	}
	c.statusFailuresMu.Unlock()
}

// withIdle applies fn to the remembered state of record's finished turn under
// the lock, starting fresh when the session moved to another sandbox or turn.
func (c *Completer) withIdle(record store.AgentSession, fn func(*idleSandbox)) {
	c.idleMu.Lock()
	defer c.idleMu.Unlock()
	if c.idle == nil {
		c.idle = map[string]idleSandbox{}
	}
	entry, ok := c.idle[record.ID]
	if !ok || entry.sandboxID != record.SandboxID || entry.turn != record.Turns {
		entry = idleSandbox{sandboxID: record.SandboxID, turn: record.Turns}
	}
	fn(&entry)
	c.idle[record.ID] = entry
}

// logDeferral logs a deferred teardown only when its reason changes, so a
// sandbox waiting out a 2h grace logs once, not every tick.
func (c *Completer) logDeferral(record store.AgentSession, reason string) {
	changed := false
	c.withIdle(record, func(e *idleSandbox) {
		changed = e.deferReason != reason
		e.deferReason = reason
	})
	if changed {
		log.Printf("agent-session completer: deferring teardown, %s (session=%s)", reason, record.ID)
	}
}

// pruneIdle drops remembered sandboxes no longer awaiting a reap (terminated,
// steered, or deleted). One that is only backing off simply harvests
// once more on its next attempt.
func (c *Completer) pruneIdle(rows []store.AgentSession) {
	waiting := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		waiting[row.ID] = struct{}{}
	}
	c.idleMu.Lock()
	for id := range c.idle {
		if _, ok := waiting[id]; !ok {
			delete(c.idle, id)
		}
	}
	c.idleMu.Unlock()
}

func (c *Completer) idleTTL() time.Duration {
	if c.IdleTTL > 0 {
		return c.IdleTTL
	}
	return 0
}

// transcriptQuota is the per-session cumulative transcript byte cap the
// harvest shares with the gateway's live tee (w1/m65 F10).
func (c *Completer) transcriptQuota() int64 {
	if c.MaxTranscriptBytes > 0 {
		return c.MaxTranscriptBytes
	}
	return store.MaxAgentSessionTranscriptBytes
}

// defaultSSHGraceTTL mirrors the gateway's default BEX_SSH_SESSION_TIMEOUT so a
// deferred sandbox can outlive one full-length editor session but no longer.
const defaultSSHGraceTTL = 4 * time.Hour

func (c *Completer) sshGraceTTL() time.Duration {
	if c.SSHGraceTTL > 0 {
		return c.SSHGraceTTL
	}
	return defaultSSHGraceTTL
}

func (c *Completer) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Completer) enabled() bool {
	return c != nil && c.Store != nil && c.Sandbox != nil
}

// Run drives the loop until ctx is canceled. It is a no-op (returns immediately)
// when any dependency is unwired, keeping store-off/sandbox-off builds unchanged.
func (c *Completer) Run(ctx context.Context) {
	if !c.enabled() {
		return
	}
	interval := c.Interval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.Reconcile(ctx)
		}
	}
}

// activePhases are the non-terminal phases the Completer watches: a live turn
// (running) plus the brief dispatch phases, so a session orphaned mid-dispatch
// (e.g. a crash before the running transition) is still reconciled.
var activePhases = []string{PhaseRunning, PhaseCreating, PhaseResuming, PhaseRedispatching}

// Reconcile finalizes every session currently in a running-turn phase. A single
// session's failure is isolated; it never blocks the others.
func (c *Completer) Reconcile(ctx context.Context) {
	if !c.enabled() {
		return
	}
	rows, err := c.Store.ListAgentSessionsByPhases(ctx, activePhases)
	if err != nil {
		return
	}
	c.pruneStatusFailures(rows)
	for _, row := range rows {
		c.finalize(ctx, row)
	}
	c.reapIdleSandboxes(ctx)
	c.reapPreviousSandboxes(ctx)
	c.reconcileSandboxInventory(ctx)
	c.recoverDispatches(ctx)
}

// reapIdleSandboxes tears down the sandboxes of already-finished sessions once
// they pass the idle grace (ADR054 D6, w10/m15). A terminal
// session drops out of activePhases, so its sandbox would otherwise linger
// forever; each tick this re-evaluates terminal sessions still holding a
// sandbox id whose sandbox_reap_after is due — no now-relative updated_at
// window, so a failing teardown is retried indefinitely with backoff (w5/m99).
func (c *Completer) reapIdleSandboxes(ctx context.Context) {
	rows, err := c.Store.ListTerminalAgentSessionsWithSandbox(ctx, c.now())
	if err != nil {
		return
	}
	c.pruneIdle(rows)
	for _, row := range rows {
		c.teardown(ctx, row)
	}
}

// reapPreviousSandboxes retries durable previous-sandbox teardown intents left
// by steer/redispatch (w5/m99 t004). Success clears the intent; failure defers
// with backoff and is counted.
func (c *Completer) reapPreviousSandboxes(ctx context.Context) {
	due, err := c.Store.ListPreviousSandboxTeardownsDue(ctx, c.now())
	if err != nil {
		log.Printf("agent-session completer: list previous-sandbox teardowns failed: %v", err)
		return
	}
	groups := make(map[string][]store.AgentDispatch)
	for _, d := range due {
		groups[d.WorkspaceID] = append(groups[d.WorkspaceID], d)
	}
	for workspace, dispatches := range groups {
		if err := ctx.Err(); err != nil {
			return
		}
		if err := c.Sandbox.CleanupAgentDispatches(ctx, dispatches); err != nil {
			log.Printf("agent-session completer: previous-sandbox teardown failed (workspace=%s): %v", workspace, err)
			c.SandboxMetrics.ObserveTeardownFailure(sandbox.TeardownReasonPreviousSandbox)
			for _, d := range dispatches {
				_ = c.Store.DeferPreviousSandboxTeardown(ctx, d.SessionID, d.Turn, c.now().Add(time.Minute))
			}
			continue
		}
		for _, d := range dispatches {
			if err := c.Store.ClearPreviousSandboxTeardown(ctx, d.SessionID, d.Turn); err != nil {
				log.Printf("agent-session completer: clear previous-sandbox intent failed (session=%s): %v", d.SessionID, err)
			}
		}
	}
}

// reconcileSandboxInventory walks every workspace with a sandbox tenant key and
// terminates sandboxes whose session is terminal/absent or past the lifetime
// bound — including the production shape where steer blanked sandbox_id.
func (c *Completer) reconcileSandboxInventory(ctx context.Context) {
	if c.InventoryInterval > 0 {
		now := c.now()
		if !c.lastInventory.IsZero() && now.Sub(c.lastInventory) < c.InventoryInterval {
			return
		}
		c.lastInventory = now
	}
	keys, err := c.Store.ListSandboxTenantKeys(ctx)
	if err != nil {
		log.Printf("agent-session completer: list sandbox tenant keys failed: %v", err)
		return
	}
	total := sandbox.InventoryCounts{OldestAge: map[string]float64{}}
	lookup := func(ctx context.Context, sessionID string) (sandbox.SessionClaim, error) {
		row, err := c.Store.GetAgentSession(ctx, sessionID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return sandbox.SessionClaim{}, nil
			}
			return sandbox.SessionClaim{}, err
		}
		return sandbox.SessionClaim{Phase: row.Phase, SandboxID: row.SandboxID, Found: true}, nil
	}
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return
		}
		counts, err := c.Sandbox.ReconcileWorkspaceInventory(ctx, key.WorkspaceID, key.APIKey, c.now(), lookup)
		if err != nil {
			log.Printf("agent-session completer: sandbox inventory reconcile failed (workspace=%s): %v", key.WorkspaceID, err)
			c.SandboxMetrics.ObserveTeardownFailure(sandbox.TeardownReasonInventory)
			continue
		}
		total.Merge(counts)
	}
	c.SandboxMetrics.SetInventory(total)
}

func (c *Completer) finalize(ctx context.Context, record store.AgentSession) {
	if record.SandboxID == "" {
		return
	}
	raw, err := c.Sandbox.ReadSessionStatus(ctx, record.WorkspaceID, record.ID, record.SandboxID)
	if err != nil {
		// A lost sandbox can never finish its turn; anything else is transient and
		// retried on the next tick. Log the transient case: a persistent read error
		// silently strands a session in running forever (the exact failure mode the
		// w3/m43 live E2E hit), so this loop must never be unobservable.
		if errors.Is(err, core.ErrNotFound) {
			c.Metrics.read(statusReadTerminal)
			c.clearStatusFailure(record.ID)
			c.failLostSandbox(ctx, record, "sandbox terminated before completion", convergenceTargetTerminated)
		} else {
			c.Metrics.read(statusReadTransientError)
			log.Printf("agent-session completer: read status failed (session=%s sandbox=%s): %v", record.ID, record.SandboxID, err)
			if c.noteStatusFailure(record.ID) {
				c.failLostSandbox(ctx, record, "sandbox status remained unavailable beyond the retry window", convergenceStatusUnavailable)
			}
		}
		return
	}
	c.Metrics.read(statusReadOK)
	c.clearStatusFailure(record.ID)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return // status file not written yet — the turn is still starting
	}
	var report statusReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		log.Printf("agent-session completer: torn/invalid status read (session=%s len=%d): %v", record.ID, len(raw), err)
		return // a partial/torn read; try again next tick
	}
	switch report.State {
	case driverStateSucceeded:
		c.complete(ctx, record, report)
	case driverStateFailed:
		reason := strings.TrimSpace(report.Error)
		if reason == "" {
			reason = "agent turn failed"
		}
		c.fail(ctx, record, reason)
	}
}

// complete finalizes a successful turn from what the agent itself delivered
// (ADR090). The PR is the one bex-api recorded when the agent opened it through
// the gateway PR verb; the driver's report only contributes the remote head and
// its reasons the work is undelivered. Undelivered work never
// fails the turn — the agent answered, and the work stays in the sandbox until
// it is closed after the idle TTL — it is named in the delivery note instead.
func (c *Completer) complete(ctx context.Context, record store.AgentSession, report statusReport) {
	headSHA := ""
	if core.ValidCommitSHA(report.Git.RemoteHead) {
		headSHA = report.Git.RemoteHead
	}
	note := deliveryNote(report.Git)
	final, fact, err := c.Store.FinalizeAgentSession(ctx, record.ID, PhaseCompleted, headSHA, note, "")
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("agent-session completer: finalize failed (session=%s): %v", record.ID, err)
		}
		return // retry next tick; the sandbox stays until the row is finalized
	}
	c.Metrics.observeTerminalTurn(turnOutcomeCompleted, fact)
	log.Printf("agent-session completer: completed session=%s pr=%s head=%s dirty=%t unpushed=%d undelivered=%t", record.ID, final.PRURL, final.HeadSHA, report.Git.Dirty, report.Git.UnpushedCommits, note != "")
	// Tear down against the FINALIZED row so the idle clock (record.UpdatedAt) is
	// the turn-end time, not the stale pre-finalize timestamp; SandboxID survives
	// finalize, so it is still present here.
	c.teardown(ctx, final)
}

// Bounds on the driver-supplied delivery reasons: the sandbox is untrusted, so
// its text is capped and stripped of control characters before it is stored
// and shown to the user.
const (
	maxDeliveryReasons     = 5
	maxDeliveryReasonRunes = 300
)

// deliveryNote joins the driver's undelivered reasons into the note a completed
// turn records. "" when the driver reported the work delivered.
func deliveryNote(git statusGit) string {
	reasons := make([]string, 0, min(len(git.Undelivered), maxDeliveryReasons))
	for _, raw := range git.Undelivered {
		if len(reasons) == maxDeliveryReasons {
			break
		}
		reason := strings.TrimSpace(strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, raw))
		if r := []rune(reason); len(r) > maxDeliveryReasonRunes {
			reason = strings.TrimSpace(string(r[:maxDeliveryReasonRunes])) + "…"
		}
		if reason != "" {
			reasons = append(reasons, reason)
		}
	}
	return strings.Join(reasons, " ")
}

func (c *Completer) fail(ctx context.Context, record store.AgentSession, reason string) {
	final, fact, ok := c.finalizeFailure(ctx, record, reason)
	if !ok {
		return
	}
	c.Metrics.observeTerminalTurn(turnOutcomeFailed, fact)
	log.Printf("agent-session completer: failed session=%s reason=%q", record.ID, reason)
	c.teardown(ctx, final)
}

// failLostSandbox is the dead-target backstop. Unlike an ordinary driver-
// reported failure, a terminal Pod cannot support editor reuse or transcript
// harvest, so it records the incomplete turn and reclaims the exact sandbox
// immediately instead of entering the idle grace.
func (c *Completer) failLostSandbox(ctx context.Context, record store.AgentSession, reason string, metricReason terminalConvergenceReason) {
	c.clearStatusFailure(record.ID)
	final, fact, ok := c.finalizeFailure(ctx, record, reason)
	if !ok {
		return
	}
	c.Metrics.converged(metricReason)
	c.Metrics.observeTerminalTurn(turnOutcomeLost, fact)
	c.markTurnTranscript(ctx, final, false, true, reason)
	log.Printf("agent-session completer: terminal fallback converged session=%s reason=%q", record.ID, reason)
	c.terminate(ctx, final)
}

func (c *Completer) finalizeFailure(ctx context.Context, record store.AgentSession, reason string) (store.AgentSession, store.TerminalTurnFact, bool) {
	final, fact, err := c.Store.FinalizeAgentSession(ctx, record.ID, PhaseFailed, "", "", reason)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("agent-session completer: finalize failed-state write errored (session=%s reason=%q): %v", record.ID, reason, err)
		}
		return store.AgentSession{}, store.TerminalTurnFact{}, false
	}
	return final, fact, true
}

// maxTranscriptParts bounds how many parts one turn's harvest persists — a
// defense against a pathological driver log, far above any real turn.
const maxTranscriptParts = 5000

// teardown captures the turn's conversation transcript (ADR051), then terminates
// the finished session's sandbox and removes its egress policy (idempotent)
// once the idle grace below has passed.
//
// Capture MUST happen before the sandbox is destroyed: a fire-and-forget turn
// runs headless with no browser attached, so nothing has teed its conversation
// yet, and the sandbox (with the driver's log) dies on teardown. Without it every
// completed session replays empty ("No conversation yet."). It is best-effort —
// a failure is logged but never blocks teardown, so a session is never stranded.
// The log is read once when the turn finishes, so the conversation is durable
// for the whole grace, and once more just before terminate for a late flush —
// never on the per-tick re-checks in between.
func (c *Completer) teardown(ctx context.Context, record store.AgentSession) {
	if record.SandboxID == "" {
		return
	}
	harvested := false
	c.withIdle(record, func(e *idleSandbox) { harvested = e.harvested })
	if !harvested {
		ok := c.captureTranscript(ctx, record)
		c.withIdle(record, func(e *idleSandbox) { e.harvested = ok })
	}
	// Idle grace (w2/m67, w10/m15), generalizing ADR054 D6. The sandbox is
	// kept alive while an editor SSH session is connected (never kill a live edit)
	// AND for IdleTTL past the last interaction, where idle is measured from the
	// later of this session's turn-end (record.UpdatedAt, stamped by the finalize
	// that produced this row) and the last editor SSH disconnect. The transcript
	// is already captured above, delivery already happened, and the session is
	// already finalized — only the physical teardown waits. The reaper retries
	// each tick; an explicit Cancel (its own teardown path) is not gated by this,
	// so a user asking to kill the session always wins.
	hasOpen, lastEnded, err := c.Store.AgentSessionSSHActivity(ctx, record.ID, c.now().Add(-c.sshGraceTTL()))
	if err != nil {
		log.Printf("agent-session completer: ssh-activity check failed (session=%s): %v", record.ID, err)
		return // transient store error: retry next tick rather than tear down blind
	}
	if hasOpen {
		c.logDeferral(record, "editor SSH open")
		return
	}
	// idleSince is the most recent interaction: the turn end (this finalized row's
	// updated_at) or a later SSH disconnect. IdleTTL=0 reduces this to "reap as
	// soon as no editor is connected" — byte-identical to ADR054 D6. Two cases
	// skip the grace entirely and reclaim at this very tick (the still-open-editor
	// pin above still applies to both — never kill a live edit):
	//   - an ARCHIVED session (ADR065 D1): archive is an explicit disinterest signal.
	//   - a FAILED session (w5/m80 t004): the idle grace exists to let a user
	//     reopen a SUCCESSFUL result in an editor; a failed turn has no such result,
	//     so holding its sandbox for the full IdleTTL only pins plan quota (a bad-key
	//     workspace could wedge itself under the ~2-sandbox hobby cap). A user who
	//     still wants to debug it keeps it alive with an open SSH via the pin above.
	skipGrace := record.ArchivedAt != nil || record.Phase == PhaseFailed
	idleSince := record.UpdatedAt
	if lastEnded != nil && lastEnded.After(idleSince) {
		idleSince = *lastEnded
	}
	if grace := c.idleTTL(); !skipGrace && grace > 0 && c.now().Sub(idleSince) < grace {
		c.logDeferral(record, "idle grace not elapsed")
		return
	}
	if harvested {
		c.captureTranscript(ctx, record) // a late flush since the first harvest
	}
	c.terminate(ctx, record)
}

// terminate is the w2/m67 reap: tear the pod down and drop the sandbox id. The
// session stays in its terminal phase with its durable transcript; a later Steer
// continues the conversation in a fresh sandbox (ADR047 D3 continuity).
// The id is cleared only after a confirmed terminate (or already-absent);
// a failure schedules backoff so the next tick retries (w5/m99 t002).
func (c *Completer) terminate(ctx context.Context, record store.AgentSession) {
	if err := c.Sandbox.CancelAgentSessionSandbox(ctx, record.WorkspaceID, record.ID, record.SandboxID); err != nil {
		log.Printf("agent-session completer: terminate sandbox failed (session=%s sandbox=%s): %v", record.ID, record.SandboxID, err)
		c.SandboxMetrics.ObserveTeardownFailure(sandbox.TeardownReasonIdleReap)
		if err := c.Store.ScheduleAgentSessionSandboxReap(ctx, record.ID, c.now()); err != nil {
			log.Printf("agent-session completer: schedule sandbox reap failed (session=%s): %v", record.ID, err)
		}
		return
	}
	if err := c.Store.ClearAgentSessionSandbox(ctx, record.ID); err != nil {
		log.Printf("agent-session completer: clear sandbox id failed (session=%s): %v", record.ID, err)
	}
}

// Transcript truncation reasons (shared with markTurnTranscript).
const (
	reasonTranscriptReadFailed  = "transcript read failed"
	reasonParseTruncated        = "driver log parse/part limit reached"
	reasonQuotaExceeded         = "session transcript byte quota reached"
	reasonTranscriptStoreFailed = "transcript store failed"
	reasonDriverTruncated       = "driver session log truncated"
)

// captureTranscript harvests the driver's per-part session log over the SAME
// pods/exec boundary the Completer already uses for the status read (ADR051), and
// appends it to the durable transcript before teardown. Riding the proven
// status-read path — rather than a separate gateway→driver stream dial — is what
// makes persistence reliable. Every harvest parses the full bounded log and
// idempotently merges by turn-local part index, so a live tee that stored only a
// prefix cannot suppress the missing suffix. It reports whether the log was read
// and stored, so a failed harvest is retried on the next tick.
func (c *Completer) captureTranscript(ctx context.Context, record store.AgentSession) bool {
	raw, err := c.Sandbox.ReadSessionTranscript(ctx, record.WorkspaceID, record.ID, record.SandboxID)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			log.Printf("agent-session completer: transcript read failed (session=%s): %v", record.ID, err)
		}
		c.markTurnTranscript(ctx, record, false, true, reasonTranscriptReadFailed)
		return false
	}
	parts, parseTruncated := parseTranscriptLog(raw, record.Turns)
	driverTruncated := driverLogTruncated(raw, parts)
	if len(parts) == 0 {
		reason := ""
		if parseTruncated {
			reason = reasonParseTruncated
		} else if driverTruncated {
			reason = reasonDriverTruncated
		}
		truncated := parseTruncated || driverTruncated
		c.markTurnTranscript(ctx, record, !truncated, truncated, reason)
		return true
	}
	// Charge only missing identities. Historical concurrent batch writers could
	// leave holes below the largest stored ordinal, so prefix suppression alone
	// would permanently lose those parts. The parsed log caps this lookup at
	// maxTranscriptParts, regardless of the durable transcript's size.
	indexes := make([]int64, len(parts))
	for i, p := range parts {
		indexes[i] = p.PartIndex
	}
	storedBytes, existing, err := c.Store.AgentSessionTranscriptProgress(ctx, record.ID, record.Turns, indexes)
	if err != nil {
		log.Printf("agent-session completer: transcript progress failed (session=%s): %v", record.ID, err)
		return false
	}
	seen := make(map[int64]bool, len(existing))
	for _, index := range existing {
		seen[index] = true
	}
	missing := parts[:0]
	for _, p := range parts {
		if !seen[p.PartIndex] {
			missing = append(missing, p)
			seen[p.PartIndex] = true
		}
	}
	parts = missing
	kept, quotaTruncated := filterPartsByQuota(parts, storedBytes, c.transcriptQuota())
	if quotaTruncated {
		log.Printf("agent-session completer: transcript quota reached, truncating harvest (session=%s)", record.ID)
	}
	if err := c.Store.AppendAgentSessionTranscript(ctx, record.ID, kept); err != nil {
		log.Printf("agent-session completer: transcript append failed (session=%s parts=%d): %v", record.ID, len(kept), err)
		c.markTurnTranscript(ctx, record, false, true, reasonTranscriptStoreFailed)
		return false
	}
	truncated := parseTruncated || quotaTruncated || driverTruncated
	reason := ""
	if parseTruncated {
		reason = reasonParseTruncated
	} else if quotaTruncated {
		reason = reasonQuotaExceeded
	} else if driverTruncated {
		reason = reasonDriverTruncated
	}
	c.markTurnTranscript(ctx, record, !truncated, truncated, reason)
	if len(kept) > 0 {
		log.Printf("agent-session completer: captured transcript (session=%s turn=%d parts=%d)", record.ID, record.Turns, len(kept))
	}
	return true
}

// filterPartsByQuota returns the prefix of parts that fits within the remaining
// quota (quota - alreadyStored), plus a flag indicating whether truncation occurred.
func filterPartsByQuota(parts []store.AgentSessionTranscriptPart, alreadyStored, quota int64) ([]store.AgentSessionTranscriptPart, bool) {
	kept := make([]store.AgentSessionTranscriptPart, 0, len(parts))
	total := alreadyStored
	for _, p := range parts {
		if total+int64(len(p.Part)) > quota {
			return kept, true
		}
		total += int64(len(p.Part))
		kept = append(kept, p)
	}
	return kept, false
}

func (c *Completer) markTurnTranscript(ctx context.Context, record store.AgentSession, complete, truncated bool, reason string) {
	if err := c.Store.CompleteAgentSessionTurn(ctx, record.ID, record.Turns, complete, truncated, reason); err != nil {
		log.Printf("agent-session completer: transcript completeness write failed (session=%s turn=%d): %v", record.ID, record.Turns, err)
	}
}

// driverSessionLogMaxBytes mirrors the driver's cumulative session-log cap
// (lego/agent-image/driver/src/session.ts maxSessionLogBytes). The driver
// refuses every part that no longer fits, so a log this close to the cap has
// dropped the rest of its turn.
const (
	driverSessionLogMaxBytes  = 16 << 20
	driverSessionLogFullSlack = 64 << 10
)

// driverLogTruncated reports that the driver dropped parts of this turn from
// its log (the driver's status file carries no such flag since ADR090): the
// turn's part indexes have a gap — the driver numbers every part,
// written or refused — or the log has reached the driver's byte cap, which
// silently drops the tail of a turn.
func driverLogTruncated(raw string, parts []store.AgentSessionTranscriptPart) bool {
	if len(raw) >= driverSessionLogMaxBytes-driverSessionLogFullSlack {
		return true
	}
	seen := make(map[int64]struct{}, len(parts))
	maxIndex := int64(-1)
	for _, p := range parts {
		seen[p.PartIndex] = struct{}{}
		maxIndex = max(maxIndex, p.PartIndex)
	}
	return int64(len(seen)) < maxIndex+1
}

// parseTranscriptLog extracts the UI-message parts from the driver's JSONL log
// (one `{"at":…,"type":"ui-message","part":{…}}` record per line, plus the raw
// data-acp lines). It keeps only the `.part` payload — the exact shape
// the GET /stream replay re-frames for a Vercel AI SDK client — stamping the turn;
// Seq is assigned by the caller. Unparseable lines (e.g. a partial leading line
// from the byte-capped tail) are skipped.
func parseTranscriptLog(raw string, turn int) ([]store.AgentSessionTranscriptPart, bool) {
	out := make([]store.AgentSessionTranscriptPart, 0)
	truncated := false
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec struct {
			Turn      int             `json:"turn"`
			PartIndex *int64          `json:"partIndex"`
			Part      json.RawMessage `json:"part"`
		}
		if json.Unmarshal([]byte(line), &rec) != nil || len(rec.Part) == 0 {
			truncated = true
			continue
		}
		if rec.Turn != 0 && rec.Turn != turn {
			continue
		}
		index := int64(len(out))
		if rec.PartIndex != nil {
			index = *rec.PartIndex
		}
		out = append(out, store.AgentSessionTranscriptPart{PartIndex: index, Turn: turn, Part: rec.Part})
		if len(out) >= maxTranscriptParts {
			truncated = true
			break
		}
	}
	return out, truncated
}
