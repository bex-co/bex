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
	"log"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/bex-co/bex/lego/backend/internal/agenttranscript"
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
	State string `json:"state"` // one of the driverState* constants
	Error string `json:"error"`
	// Code is believed only when it is FailureCodeSetupFailed (ADR109 D5).
	Code string    `json:"code"`
	Git  statusGit `json:"git"`
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
// sandbox down. It never opens or edits a pull request; its one GitHub call
// reads the session branch's head (verifiedHead). A failed turn or a lost sandbox surfaces
// as a failed session with a named reason — never a hang. It performs NO tenant
// authorization: it acts only on durable sessions the platform owns.
type Completer struct {
	Store   Store
	Sandbox SandboxLifecycle
	// Heads reads the session branch's head on GitHub
	// (agentsession.BranchHeads). The completer records that head rather than
	// the one the sandbox reports, since CI-failure steering keys on it
	// (w5/m382). Nil (no GitHub App) keeps the reported head.
	Heads interface {
		Head(ctx context.Context, workspaceID, repository, branch string) (string, error)
	}
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

	// StatusReadFailureTTL is the retry window: how long status reads may keep
	// failing transiently (sandbox.Transient — a node that is not ready, a
	// gateway restart, a read timeout) before the session is failed honestly.
	// A coded missing/dead target converges immediately; a successful read
	// resets the streak; any other error (the platform unable to read
	// sandboxes at all) neither starts nor extends it, since it says nothing
	// about the sandbox. Zero uses ten minutes (BEX_AGENT_STATUS_RETRY_WINDOW).
	StatusReadFailureTTL time.Duration
	// TurnTimeout is the turn ceiling the driver enforces
	// (BEX_AGENT_TURN_TIMEOUT; zero ⇒ its 30m default). Once a turn has run
	// past it plus the retry window and its status still cannot be read, by
	// any error, the driver's own verdict is unreachable and the session is
	// failed, so no unreadable sandbox holds its slots until its lifetime ends.
	TurnTimeout time.Duration
	Metrics     *CompletionMetrics
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
	// LiveTranscriptInterval overrides liveTranscriptInterval (tests).
	LiveTranscriptInterval time.Duration
	liveMu                 sync.Mutex
	live                   map[string]liveHarvest
	liveWG                 sync.WaitGroup
}

// liveHarvest is one running turn's periodic harvest on this replica: when it
// last started, and whether one is still in flight.
type liveHarvest struct {
	turn     int
	started  time.Time
	inFlight bool
}

// liveTranscriptInterval is how often a replica harvests a running turn's
// driver log (w6/m398). A headless turn has no live tee (the gateway tees
// only an attached stream), so without it a sandbox that dies mid-turn leaves
// no transcript at all. Every replica runs the Completer, so with two the
// effective spacing is about half this.
const liveTranscriptInterval = time.Minute

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

const defaultStatusReadFailureTTL = 10 * time.Minute

// Per-read bounds. The Completer reads every session in turn on one loop, so
// one read hung on a stalled node must not hold up every other session's. A
// status file is a few hundred bytes; a transcript is up to the driver's
// 16 MiB log.
const (
	statusReadTimeout     = 10 * time.Second
	transcriptReadTimeout = 2 * time.Minute
)

// defaultLostSandboxReason is a lost sandbox's failure reason when nothing
// recorded how its container ended.
const defaultLostSandboxReason = "sandbox terminated before completion"

// lostSandboxReason names how the sandbox behind a NotFound status read
// ended (OOMKilled, an exit code) when the gateway saw its container, else
// the default reason.
func lostSandboxReason(readErr error) string {
	termination, _ := sandbox.TerminationOf(readErr)
	if reason := termination.Describe(); reason != "" {
		return reason
	}
	return defaultLostSandboxReason
}

// retryWindowReason is the failure reason of a session whose status stayed
// unreadable for the whole retry window; lastErr is the read that ended it.
func (c *Completer) retryWindowReason(lastErr error) string {
	if sandbox.StateUnknown(lastErr) {
		return fmt.Sprintf("sandbox state was unreadable for the %s retry window", shortDuration(c.statusReadFailureTTL()))
	}
	return fmt.Sprintf("sandbox status remained unavailable for the %s retry window", shortDuration(c.statusReadFailureTTL()))
}

// shortDuration prints d without trailing zero units: 10m, 4h, 1h30m.
func shortDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

// pastStatusDeadline reports a session whose current turn began longer ago
// than the turn ceiling plus the retry window: the driver has ended the turn
// by now, so a status that still cannot be read never will be.
func (c *Completer) pastStatusDeadline(ctx context.Context, record store.AgentSession) bool {
	deadline := turnCeiling(c.TurnTimeout) + c.statusReadFailureTTL()
	now := c.now()
	// A turn starts no earlier than its session: a young session needs no read.
	if now.Sub(record.CreatedAt) < deadline {
		return false
	}
	turns, err := c.Store.AgentSessionTurns(ctx, record.ID)
	if err != nil {
		return false
	}
	start := record.UpdatedAt
	for _, turn := range turns {
		if turn.Turn != record.Turns {
			continue
		}
		start = turn.CreatedAt
		if turn.StartedAt != nil {
			start = *turn.StartedAt
		}
	}
	return now.Sub(start) >= deadline
}

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

// sessionIDs is the set of rows' session ids.
func sessionIDs(rows []store.AgentSession) map[string]struct{} {
	ids := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		ids[row.ID] = struct{}{}
	}
	return ids
}

func (c *Completer) pruneStatusFailures(rows []store.AgentSession) {
	active := sessionIDs(rows)
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
	waiting := sessionIDs(rows)
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
	c.pruneLive(rows)
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
	readCtx, cancel := context.WithTimeout(ctx, statusReadTimeout)
	raw, err := c.Sandbox.ReadSessionStatus(readCtx, record.WorkspaceID, record.ID, record.SandboxID)
	timedOut := errors.Is(readCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil
	cancel()
	if err != nil {
		c.statusReadFailed(ctx, record, err, timedOut)
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
	case driverStateRunning:
		c.harvestRunningTurn(ctx, record)
	case driverStateSucceeded:
		c.complete(ctx, record, report)
	case driverStateFailed:
		reason := strings.TrimSpace(report.Error)
		if reason == "" {
			reason = "agent turn failed"
		}
		code := ""
		if report.Code == FailureCodeSetupFailed {
			code = FailureCodeSetupFailed
		}
		c.fail(ctx, record, reason, code)
	}
}

// verifiedHead is the head to record for a turn whose sandbox reported
// reported: GitHub's head of the session branch, so a sandbox cannot point
// CI-failure steering at a commit that is not on its branch. When the sandbox
// reported none, none is recorded (the previous head stays); when GitHub
// cannot be read, none either, rather than the sandbox's word.
func (c *Completer) verifiedHead(ctx context.Context, record store.AgentSession, reported string) string {
	if !core.ValidCommitSHA(reported) {
		return ""
	}
	if c.Heads == nil {
		return reported
	}
	// Bounded like a status read: the loop is serial, and an unreadable head
	// costs only the recording, never the finalize.
	readCtx, cancel := context.WithTimeout(ctx, statusReadTimeout)
	defer cancel()
	head, err := c.Heads.Head(readCtx, record.WorkspaceID, record.Repo, record.Branch)
	if err != nil || !core.ValidCommitSHA(head) {
		log.Printf("agent-session completer: session branch head unreadable on GitHub; recording none (session=%s branch=%s): %v", record.ID, record.Branch, err)
		return ""
	}
	if head != reported {
		log.Printf("agent-session completer: sandbox reported head %s, GitHub has %s; recording GitHub's (session=%s branch=%s)", reported, head, record.ID, record.Branch)
	}
	return head
}

// complete finalizes a successful turn from what the agent itself delivered
// (ADR090). The PR is the one bex-api recorded when the agent opened it through
// the gateway PR verb; the driver's report contributes the remote head (verified
// against GitHub, verifiedHead) and its reasons the work is undelivered. Undelivered work never
// fails the turn — the agent answered, and the work stays in the sandbox until
// it is closed after the idle TTL — it is named in the delivery note instead.
func (c *Completer) complete(ctx context.Context, record store.AgentSession, report statusReport) {
	missing, err := c.structuredOutputMissing(ctx, record)
	if err != nil {
		log.Printf("agent-session completer: read turn output contract failed (session=%s): %v", record.ID, err)
		return // retry next tick; the sandbox stays until the row is finalized
	}
	if missing {
		c.failMissingOutput(ctx, record)
		return
	}
	headSHA := c.verifiedHead(ctx, record, report.Git.RemoteHead)
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

// structuredOutputMissing reports a turn that was accepted with a required
// outputSchema and ended without a final valid result (ADR094 D5). The driver's
// status is not believed here: only a result bex-api itself validated and
// stored counts, so a sandbox cannot claim one it never submitted.
func (c *Completer) structuredOutputMissing(ctx context.Context, record store.AgentSession) (bool, error) {
	turns, err := c.Store.AgentSessionTurns(ctx, record.ID)
	if err != nil {
		return false, err
	}
	for _, turn := range turns {
		if turn.Turn == record.Turns {
			return turn.OutputRequired && len(turn.OutputSchema) > 0 && !turn.ResultFinal, nil
		}
	}
	return false, nil
}

// failMissingOutput ends a turn that required a structured result and has none.
// A pull request the agent opened stays on the session: the row keeps its PR
// fields and the code says why the turn failed, not that the work was lost.
func (c *Completer) failMissingOutput(ctx context.Context, record store.AgentSession) {
	c.fail(ctx, record, structuredOutputMissingReason, FailureCodeStructuredOutputMissing)
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

func (c *Completer) fail(ctx context.Context, record store.AgentSession, reason, code string) {
	final, fact, ok := c.finalizeFailure(ctx, record, reason)
	if !ok {
		return
	}
	if code != "" {
		if err := c.Store.StampAgentSessionFailureCode(ctx, record.ID, reason, code); err != nil {
			log.Printf("agent-session completer: stamp failure code failed (session=%s): %v", record.ID, err)
		}
	}
	c.Metrics.observeTerminalTurn(turnOutcomeFailed, fact)
	log.Printf("agent-session completer: failed session=%s code=%q reason=%q", record.ID, code, reason)
	c.teardown(ctx, final)
}

// statusReadFailed handles a failed status read. A lost sandbox can never
// finish its turn: it fails at once, with how its container ended when the
// gateway saw it. A transient failure is retried each tick and fails the
// session once it has lasted the whole retry window. Any other failure says
// nothing about the sandbox and is only logged. Whatever the error, a turn
// past its ceiling plus the retry window fails: the driver's own verdict can
// no longer be read. Every retried case is logged — a persistent read error
// silently strands a session in running (the exact failure mode the w3/m43
// live E2E hit), so this loop must never be unobservable.
func (c *Completer) statusReadFailed(ctx context.Context, record store.AgentSession, err error, timedOut bool) {
	if errors.Is(err, core.ErrNotFound) {
		c.Metrics.read(statusReadTerminal)
		c.failLostSandbox(ctx, record, lostSandboxReason(err), convergenceTargetTerminated)
		return
	}
	transient := sandbox.Transient(err) || timedOut
	if transient {
		c.Metrics.read(statusReadTransientError)
		log.Printf("agent-session completer: read status failed (session=%s sandbox=%s): %v", record.ID, record.SandboxID, err)
	} else {
		c.Metrics.read(statusReadError)
		log.Printf("agent-session completer: read status failed, not counted toward the retry window (session=%s sandbox=%s): %v", record.ID, record.SandboxID, err)
	}
	switch {
	case c.pastStatusDeadline(ctx, record):
		reason := fmt.Sprintf("sandbox status could not be read for %s after the turn's %s limit",
			shortDuration(c.statusReadFailureTTL()), shortDuration(turnCeiling(c.TurnTimeout)))
		c.failLostSandbox(ctx, record, reason, convergenceStatusDeadline)
	case transient && c.noteStatusFailure(record.ID):
		c.failLostSandbox(ctx, record, c.retryWindowReason(err), convergenceStatusUnavailable)
	}
}

// failLostSandbox is the dead-target backstop. Unlike an ordinary driver-
// reported failure, a terminal Pod cannot support editor reuse or transcript
// harvest, so it records the incomplete turn and reclaims the exact sandbox
// immediately instead of entering the idle grace.
func (c *Completer) failLostSandbox(ctx context.Context, record store.AgentSession, reason string, metricReason terminalConvergenceReason) {
	c.clearStatusFailure(record.ID)
	if metricReason != convergenceTargetTerminated {
		// The Pod may still hold the log: keep what it says before giving the
		// turn up, bounded like a status read so the loop is not held.
		readCtx, cancel := context.WithTimeout(ctx, statusReadTimeout)
		c.harvestTranscript(readCtx, record, false)
		cancel()
	}
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

// maxTranscriptItems bounds how many completed items one turn's harvest
// persists — a defense against a pathological driver log. Items, not chunks
// (w10/m16): the recorded 10-minute production turn that hit the old
// 5,000-chunk cap folds into 410 items, so a normal long turn never reaches it.
// The session's 64 MiB byte quota stays the backstop.
const maxTranscriptItems = 5000

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
	reasonParseTruncated        = "driver log parse failed"
	reasonItemLimit             = "transcript item limit reached"
	reasonQuotaExceeded         = "session transcript byte quota reached"
	reasonTranscriptStoreFailed = "transcript store failed"
	reasonDriverTruncated       = "driver session log truncated"
)

// captureTranscript harvests the driver's per-part session log over the SAME
// pods/exec boundary the Completer already uses for the status read (ADR051), and
// merges it into the durable transcript before teardown. Riding the proven
// status-read path — rather than a separate gateway→driver stream dial — is what
// makes persistence reliable. Every harvest parses the full bounded log, folds
// its chunks into completed items with the same coalescer as the live tee
// (agenttranscript), and idempotently merges by turn-local item index, so a live
// tee that stored only a prefix — or a partial item at a disconnect — cannot
// suppress the rest. It reports whether the log was read and stored, so a
// failed harvest is retried on the next tick.
func (c *Completer) captureTranscript(ctx context.Context, record store.AgentSession) bool {
	reason, ok := c.harvestTranscript(ctx, record, false)
	if !ok {
		c.markTurnTranscript(ctx, record, false, true, reason)
		return false
	}
	c.markTurnTranscript(ctx, record, reason == "", reason != "", reason)
	return true
}

// harvestTranscript reads the driver's log and merges the current turn's items.
// It reports whether the log was read and stored, and the truncation reason
// (or why the harvest failed). With completedOnly it leaves out the items still
// being written: a running turn's harvest stores what is final, and the merge
// never lets a later, longer partial copy replace an earlier one. It never
// marks the turn's transcript: a running turn is not complete.
func (c *Completer) harvestTranscript(ctx context.Context, record store.AgentSession, completedOnly bool) (string, bool) {
	readCtx, cancel := context.WithTimeout(ctx, transcriptReadTimeout)
	raw, err := c.Sandbox.ReadSessionTranscript(readCtx, record.WorkspaceID, record.ID, record.SandboxID)
	cancel()
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			log.Printf("agent-session completer: transcript read failed (session=%s): %v", record.ID, err)
		}
		return reasonTranscriptReadFailed, false
	}
	chunks, parseTruncated := parseTranscriptLog(raw, record.Turns)
	driverTruncated := driverLogTruncated(raw, chunks)
	items := agenttranscript.Coalesce(chunks)
	if completedOnly {
		items = slices.DeleteFunc(items, func(it agenttranscript.Item) bool { return it.Partial })
	}
	itemCapped := len(items) > maxTranscriptItems
	if itemCapped {
		items = items[:maxTranscriptItems]
	}
	var res store.TranscriptMergeResult
	if len(items) > 0 {
		merge := store.TranscriptItemMerge{Turn: record.Turns, Quota: c.transcriptQuota(),
			FirstChunk: chunks[0].Index, LastChunk: chunks[0].Index}
		for _, ch := range chunks {
			merge.FirstChunk = min(merge.FirstChunk, ch.Index)
			merge.LastChunk = max(merge.LastChunk, ch.Index)
		}
		if completedOnly {
			// Never swap a legacy raw-chunk turn for a fold that left items out.
			merge.FirstChunk, merge.LastChunk = -1, -1
		}
		merge.Items = make([]store.AgentSessionTranscriptPart, len(items))
		for i, it := range items {
			merge.Items[i] = store.AgentSessionTranscriptPart{Turn: record.Turns, PartIndex: it.Index, Part: it.Part, Item: true, Partial: it.Partial,
				FullOutputs: it.FullOutputs}
		}
		res, err = c.Store.MergeAgentSessionTranscriptItems(ctx, record.ID, merge)
		if err != nil {
			log.Printf("agent-session completer: transcript merge failed (session=%s items=%d): %v", record.ID, len(items), err)
			return reasonTranscriptStoreFailed, false
		}
		if res.QuotaReached {
			log.Printf("agent-session completer: transcript quota reached, truncating harvest (session=%s)", record.ID)
		}
	}
	reason := ""
	switch {
	case parseTruncated:
		reason = reasonParseTruncated
	case itemCapped:
		reason = reasonItemLimit
	case res.QuotaReached:
		reason = reasonQuotaExceeded
	case driverTruncated:
		reason = reasonDriverTruncated
	}
	if res.Inserted+res.Upgraded > 0 {
		log.Printf("agent-session completer: captured transcript (session=%s turn=%d running=%t chunks=%d items=%d inserted=%d upgraded=%d replaced_legacy=%d)",
			record.ID, record.Turns, completedOnly, len(chunks), len(items), res.Inserted, res.Upgraded, res.Replaced)
	}
	return reason, true
}

// harvestRunningTurn starts a background harvest of a running turn's
// completed items at most once per liveTranscriptInterval, one at a time per
// session, so the serial status loop never waits on a log read.
func (c *Completer) harvestRunningTurn(ctx context.Context, record store.AgentSession) {
	interval := c.LiveTranscriptInterval
	if interval <= 0 {
		interval = liveTranscriptInterval
	}
	now := c.now()
	c.liveMu.Lock()
	if c.live == nil {
		c.live = map[string]liveHarvest{}
	}
	prev, seen := c.live[record.ID]
	if seen && (prev.inFlight || (prev.turn == record.Turns && now.Sub(prev.started) < interval)) {
		c.liveMu.Unlock()
		return
	}
	c.live[record.ID] = liveHarvest{turn: record.Turns, started: now, inFlight: true}
	c.liveMu.Unlock()
	c.liveWG.Add(1)
	go func() {
		defer c.liveWG.Done()
		c.harvestTranscript(ctx, record, true)
		c.liveMu.Lock()
		defer c.liveMu.Unlock()
		if h, ok := c.live[record.ID]; ok {
			h.inFlight = false
			c.live[record.ID] = h
		}
	}()
}

// pruneLive forgets the periodic-harvest state of sessions no longer running.
func (c *Completer) pruneLive(rows []store.AgentSession) {
	c.liveMu.Lock()
	defer c.liveMu.Unlock()
	if len(c.live) == 0 {
		return
	}
	running := sessionIDs(rows)
	for id, h := range c.live {
		if _, ok := running[id]; !ok && !h.inFlight {
			delete(c.live, id)
		}
	}
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
func driverLogTruncated(raw string, chunks []agenttranscript.Chunk) bool {
	if len(raw) >= driverSessionLogMaxBytes-driverSessionLogFullSlack {
		return true
	}
	seen := make(map[int64]struct{}, len(chunks))
	maxIndex := int64(-1)
	for _, ch := range chunks {
		seen[ch.Index] = struct{}{}
		maxIndex = max(maxIndex, ch.Index)
	}
	return int64(len(seen)) < maxIndex+1
}

// parseTranscriptLog extracts this turn's UI-message chunks from the driver's
// JSONL log (one `{"at":…,"type":"ui-message","turn":…,"partIndex":…,"part":{…}}`
// record per line), keeping the `.part` payload under its driver ordinal.
// Unparseable lines (e.g. a partial leading line from the byte-capped tail) are
// skipped and reported. The log itself is bounded (16 MiB), so the chunk count
// is too; the item cap applies after folding.
func parseTranscriptLog(raw string, turn int) ([]agenttranscript.Chunk, bool) {
	out := make([]agenttranscript.Chunk, 0)
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
		out = append(out, agenttranscript.Chunk{Index: index, Part: rec.Part})
	}
	return out, truncated
}
