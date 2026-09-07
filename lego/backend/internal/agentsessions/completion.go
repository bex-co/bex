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
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Evidence bounds, mirrored from the driver's own caps
// (lego/agent-image/driver/src/delivery.mjs) as defense-in-depth: a misbehaving
// or forward-version driver must not store an unbounded blob. Keep the two sets
// in sync. Truncation past these caps is always marked on Evidence.Truncated.
const (
	maxEvidenceCommands   = 40
	maxEvidenceTestLines  = 40
	maxEvidenceLineLen    = 2000
	maxEvidenceOutputTail = 8000
	maxEvidenceChanged    = 100
)

// statusReport is the driver's machine-readable status file (lego/agent-image/
// driver/src/session.mjs + delivery.mjs). Only these fields are consumed; the
// driver may add more without breaking the reader.
type statusReport struct {
	State    string         `json:"state"` // one of the driverState* constants
	Error    string         `json:"error"`
	Delivery statusDelivery `json:"delivery"`
	Evidence statusEvidence `json:"evidence"`
}

type statusDelivery struct {
	Branch       string   `json:"branch"`
	BaseBranch   string   `json:"baseBranch"`
	HeadSHA      string   `json:"headSha"`
	Pushed       bool     `json:"pushed"`
	Commits      int      `json:"commits"`
	ChangedFiles []string `json:"changedFiles"`
}

type statusEvidence struct {
	CommandLog []string `json:"commandLog"`
	TestOutput []string `json:"testOutput"`
	OutputTail string   `json:"outputTail"`
	Truncated  bool     `json:"truncated"`
}

// Completer is the trusted background loop that finalizes fire-and-forget
// sessions (ADR047 D4). Each tick it reads every running session's
// driver status file through the gateway exec boundary and, on a pushed
// successful turn, opens/updates the draft PR, records the result + bounded
// evidence, and tears the sandbox down. A failed turn, a lost sandbox, or a
// failed PR-open surfaces as a failed session with a named reason — never a
// hang. It performs NO tenant authorization: it acts only on durable sessions
// the platform owns.
type Completer struct {
	Store        Store
	Sandbox      SandboxLifecycle
	GitHub       PullRequestOpener
	Connections  ConnectionStore
	APIPublicURL string
	Interval     time.Duration
	Now          func() time.Time
	// MaxTranscriptBytes overrides the per-session cumulative transcript quota
	// (store.MaxAgentSessionTranscriptBytes); zero => the platform default.
	// Tests set it small to exercise the quota without 64 MiB payloads.
	MaxTranscriptBytes int64

	// SSHGraceTTL bounds how long an OPEN editor SSH session pins a finished
	// session's sandbox (ADR054 D6): an ssh_sessions row older than this is
	// treated as leaked (a crashed gateway replica) and ignored. It equals the
	// gateway's SSH session cap; 0 => the 4h default (BEX_SSH_SESSION_TIMEOUT's).
	SSHGraceTTL time.Duration

	// IdleTTL is the Active-tier idle grace (ADR059 D2, w2/m67): a finished
	// session's sandbox is kept alive until it has been idle this long, where
	// idle = now − max(last turn end, last SSH disconnect) and a currently-open
	// editor SSH session pins idle at zero regardless. 0 ⇒ no extra grace (reap
	// as soon as no editor is connected — byte-identical to ADR054 D6). Set from
	// BEX_AGENT_SANDBOX_IDLE_TTL (main.go default 30m); the zero value here keeps
	// tests on the pre-m67 behavior so the ADR054 D6 suite is unchanged.
	IdleTTL time.Duration

	// Snapshots, when set (BEX_AGENT_SNAPSHOT_S3_* wired), turns the reclaim
	// action from Terminate into ADR059 D3 hibernation: the idle sandbox's mutable
	// state is snapshotted to object storage and the row moves to `hibernated`
	// instead of being torn down. nil ⇒ reclaim = Terminate, byte-identical to
	// w2/m67 (the safe default).
	Snapshots SnapshotStore
	// RetentionTTL is the ADR059 D5 hibernation retention window: an unpinned
	// hibernated session is deleted this long after it hibernated. 0 ⇒ the 7d
	// default when hibernation is enabled.
	RetentionTTL time.Duration

	// StatusReadFailureTTL bounds a continuous ambiguous gateway/status-read
	// failure before the session is terminalized honestly. A coded missing/dead
	// target converges immediately. Zero uses two minutes; successful reads reset
	// the streak, so a one-off gateway rollout remains transient.
	StatusReadFailureTTL time.Duration
	Metrics              *CompletionMetrics
	statusFailuresMu     sync.Mutex
	statusFailures       map[string]time.Time
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

func (c *Completer) idleTTL() time.Duration {
	if c.IdleTTL > 0 {
		return c.IdleTTL
	}
	return 0
}

// defaultRetentionTTL is the ADR059 D5 hibernation retention window.
const defaultRetentionTTL = 7 * 24 * time.Hour

func (c *Completer) retentionTTL() time.Duration {
	if c.RetentionTTL > 0 {
		return c.RetentionTTL
	}
	return defaultRetentionTTL
}

// retentionWindow is the retention duration for a freshly hibernated session:
// the base window, doubled when the git tree was dirty at hibernation so a user
// with uncommitted work gets longer to come back before the snapshot is deleted
// (ADR059 D5 dirty-git extension).
func (c *Completer) retentionWindow(dirtyGit bool) time.Duration {
	base := c.retentionTTL()
	if dirtyGit {
		return 2 * base
	}
	return base
}

// hibernationEnabled reports whether the reclaim action is hibernate (ADR059 D3)
// rather than the default Terminate reap (w2/m67).
func (c *Completer) hibernationEnabled() bool {
	return c.Snapshots != nil
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
	return c != nil && c.Store != nil && c.Sandbox != nil && c.GitHub != nil && c.Connections != nil
}

// Run drives the loop until ctx is canceled. It is a no-op (returns immediately)
// when any dependency is unwired, keeping store-off/GitHub-off builds unchanged.
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
	c.sweepExpiredHibernations(ctx)
	c.recoverDispatches(ctx)
}

// sweepExpiredHibernations deletes the snapshots + finalizes the rows of
// unpinned hibernated sessions past their retention deadline (ADR059 D5). It is
// a no-op unless hibernation is enabled. The snapshot object is deleted first;
// only on a successful delete is the row expired, so a failed object delete
// retries next tick rather than orphaning the blob. Pinned rows never appear in
// the store's retention list, so pinning alone removes the delete edge.
func (c *Completer) sweepExpiredHibernations(ctx context.Context) {
	if !c.hibernationEnabled() {
		return
	}
	rows, err := c.Store.ListHibernatedForRetention(ctx, c.now(), 0)
	if err != nil {
		log.Printf("agent-session completer: list expired hibernations failed: %v", err)
		return
	}
	for _, row := range rows {
		if err := c.Snapshots.Delete(ctx, row.SnapshotRef); err != nil {
			log.Printf("agent-session completer: delete expired snapshot failed (session=%s ref=%s): %v", row.ID, row.SnapshotRef, err)
			continue // retry next tick; don't expire the row while the blob survives
		}
		if _, err := c.Store.ExpireHibernatedAgentSession(ctx, row.ID, row.SnapshotRef); err != nil {
			log.Printf("agent-session completer: expire hibernated row failed (session=%s): %v", row.ID, err)
			continue
		}
		log.Printf("agent-session completer: retention-expired hibernated session=%s ref=%s", row.ID, row.SnapshotRef)
	}
}

// reapIdleSandboxes tears down the sandboxes of already-finished sessions once
// they pass the Active-tier idle grace (ADR059 D2 / ADR054 D6). A terminal
// session drops out of activePhases, so its sandbox would otherwise linger
// forever; each tick this re-evaluates the small set of terminal sessions still
// holding a sandbox id — reading each row's FRESH updated_at (the turn-end time
// FinalizeAgentSession stamped) so the idle clock is measured from completion,
// not the stale record the completion path held.
func (c *Completer) reapIdleSandboxes(ctx context.Context) {
	// Widen the scan to cover the idle grace on top of the leaked-open window.
	since := c.now().Add(-(2*c.sshGraceTTL() + c.idleTTL()))
	rows, err := c.Store.ListTerminalAgentSessionsWithSandbox(ctx, since)
	if err != nil {
		return
	}
	for _, row := range rows {
		c.teardown(ctx, row)
	}
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

func (c *Completer) complete(ctx context.Context, record store.AgentSession, report statusReport) {
	evidenceJSON, _ := json.Marshal(buildEvidence(report)) // Evidence is plain data; marshal cannot fail

	// The config supplies the PR title; an unreadable one yields an untitled PR
	// rather than blocking the completion (the branch is already pushed).
	config, err := decodeAgentConfig(record)
	if err != nil {
		log.Printf("agent-session completer: agent config unreadable (session=%s): %v", record.ID, err)
	}

	// A turn that pushed nothing is an honest no-op completion: record it (with
	// evidence) but open no PR — there is nothing to review.
	if !report.Delivery.Pushed || report.Delivery.HeadSHA == "" {
		if final, err := c.Store.FinalizeAgentSession(ctx, record.ID, PhaseCompleted, "", "", 0, evidenceJSON, ""); err == nil {
			c.Metrics.observeTurn(turnOutcomeCompleted, c.now().Sub(record.UpdatedAt))
			log.Printf("agent-session completer: completed session=%s (no-op, nothing pushed)", record.ID)
			c.teardown(ctx, final)
		} else {
			log.Printf("agent-session completer: finalize no-op failed (session=%s): %v", record.ID, err)
		}
		return
	}

	base := strings.TrimSpace(report.Delivery.BaseBranch)
	if base == "" {
		c.fail(ctx, record, "delivery reported no base branch for the pull request")
		return
	}
	owner, name, ok := strings.Cut(record.Repo, "/")
	if !ok {
		c.fail(ctx, record, "session repository is not owner/name")
		return
	}
	conn, err := c.Connections.GetGitConnectionByOwner(ctx, record.WorkspaceID, owner)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.fail(ctx, record, "workspace has no GitHub App installation for this repository's account to open a pull request")
		} else {
			log.Printf("agent-session completer: git connection lookup failed (session=%s ws=%s): %v", record.ID, record.WorkspaceID, err)
		}
		return // transient store error: retry next tick
	}
	pr, err := c.GitHub.OpenDraftPullRequest(ctx, conn.InstallationID, owner, name,
		record.Branch, base, prTitle(config, record.ID), prBody(record, report, c.APIPublicURL))
	if err != nil {
		log.Printf("agent-session completer: open draft PR failed (session=%s repo=%s branch=%s base=%s): %v", record.ID, record.Repo, record.Branch, base, err)
		c.fail(ctx, record, "draft pull request could not be opened")
		return
	}
	final, err := c.Store.FinalizeAgentSession(ctx, record.ID, PhaseCompleted,
		report.Delivery.HeadSHA, pr.HTMLURL, pr.Number, evidenceJSON, "")
	if err != nil {
		log.Printf("agent-session completer: finalize failed (session=%s pr=%s): %v", record.ID, pr.HTMLURL, err)
		return // retry next tick; the sandbox stays until the row is finalized
	}
	c.Metrics.observeTurn(turnOutcomeCompleted, c.now().Sub(record.UpdatedAt))
	log.Printf("agent-session completer: completed session=%s pr=%s head=%s", record.ID, pr.HTMLURL, report.Delivery.HeadSHA)
	// Tear down against the FINALIZED row so the idle clock (record.UpdatedAt) is
	// the turn-end time, not the stale pre-finalize timestamp; SandboxID survives
	// finalize, so it is still present here.
	c.teardown(ctx, final)
}

func (c *Completer) fail(ctx context.Context, record store.AgentSession, reason string) {
	final, ok := c.finalizeFailure(ctx, record, reason)
	if !ok {
		return
	}
	c.Metrics.observeTurn(turnOutcomeFailed, c.now().Sub(record.UpdatedAt))
	log.Printf("agent-session completer: failed session=%s reason=%q", record.ID, reason)
	c.teardown(ctx, final)
}

// failLostSandbox is the dead-target backstop. Unlike an ordinary driver-
// reported failure, a terminal Pod cannot support editor reuse or transcript
// harvest, so it records the incomplete turn and reclaims the exact sandbox
// immediately instead of entering the Active-tier idle grace.
func (c *Completer) failLostSandbox(ctx context.Context, record store.AgentSession, reason string, metricReason terminalConvergenceReason) {
	c.clearStatusFailure(record.ID)
	final, ok := c.finalizeFailure(ctx, record, reason)
	if !ok {
		return
	}
	c.Metrics.converged(metricReason)
	c.Metrics.observeTurn(turnOutcomeLost, c.now().Sub(record.UpdatedAt))
	c.markTurnTranscript(ctx, final, false, true, reason)
	log.Printf("agent-session completer: terminal fallback converged session=%s reason=%q", record.ID, reason)
	c.terminate(ctx, final)
}

func (c *Completer) finalizeFailure(ctx context.Context, record store.AgentSession, reason string) (store.AgentSession, bool) {
	final, err := c.Store.FinalizeAgentSession(ctx, record.ID, PhaseFailed, "", "", 0, nil, reason)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("agent-session completer: finalize failed-state write errored (session=%s reason=%q): %v", record.ID, reason, err)
		}
		return store.AgentSession{}, false
	}
	return final, true
}

// maxTranscriptParts bounds how many parts one turn's harvest persists — a
// defense against a pathological driver log, far above any real turn.
const maxTranscriptParts = 5000

// teardown captures the turn's conversation transcript (ADR051), then terminates
// the finished session's sandbox and removes its egress policy (idempotent).
// Steering re-dispatches a fresh sandbox, so a completed session never needs its
// sandbox kept warm in phase 1.
//
// Capture MUST happen before the sandbox is destroyed: a fire-and-forget turn
// runs headless with no browser attached, so nothing has teed its conversation
// yet, and the sandbox (with the driver's log) dies on teardown. Without it every
// completed session replays empty ("No conversation yet."). It is best-effort —
// a failure is logged but never blocks teardown, so a session is never stranded.
func (c *Completer) teardown(ctx context.Context, record store.AgentSession) {
	if record.SandboxID == "" {
		return
	}
	c.captureTranscript(ctx, record)
	// Active-tier idle grace (ADR059 D2), generalizing ADR054 D6. The sandbox is
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
		log.Printf("agent-session completer: deferring teardown, editor SSH open (session=%s)", record.ID)
		return
	}
	// idleSince is the most recent interaction: the turn end (this finalized row's
	// updated_at) or a later SSH disconnect. IdleTTL=0 reduces this to "reap as
	// soon as no editor is connected" — byte-identical to ADR054 D6. Two cases
	// skip the grace entirely and reclaim at this very tick (the still-open-editor
	// pin above still applies to both — never kill a live edit):
	//   - an ARCHIVED session (ADR065 D1): archive is an explicit disinterest signal.
	//   - a FAILED session (w5/m80 t004): the Active-tier grace exists to let a user
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
		log.Printf("agent-session completer: deferring teardown, idle grace not elapsed (session=%s)", record.ID)
		return
	}
	c.reclaim(ctx, record)
}

// reclaim releases an idle finished session's compute. With hibernation enabled
// (ADR059 D3) it snapshots the mutable state to object storage and moves the row
// to `hibernated`, preserving resumability at storage cost; otherwise — or on
// ANY hibernation failure — it falls back to the w2/m67 Terminate reap, so a
// broken snapshot store never strands a sandbox or loses the reclaim. The two
// paths are byte-identical when Snapshots is nil.
func (c *Completer) reclaim(ctx context.Context, record store.AgentSession) {
	if c.hibernationEnabled() {
		if c.hibernate(ctx, record) {
			return // hibernated; the pod is gone and the row holds the snapshot
		}
		log.Printf("agent-session completer: hibernation failed, falling back to terminate (session=%s)", record.ID)
	}
	c.terminate(ctx, record)
}

// terminate is the w2/m67 reap: tear the pod down and drop the sandbox id.
func (c *Completer) terminate(ctx context.Context, record store.AgentSession) {
	_ = c.Sandbox.CancelAgentSessionSandbox(ctx, record.WorkspaceID, record.ID, record.SandboxID)
	if err := c.Store.ClearAgentSessionSandbox(ctx, record.ID); err != nil {
		log.Printf("agent-session completer: clear sandbox id failed (session=%s): %v", record.ID, err)
	}
}

// hibernate runs the ADR059 D3 reclaim: claim the row (CAS to `hibernating`),
// mint a presigned upload URL, snapshot+terminate the sandbox, then record the
// durable snapshot (ref/size/digest) and move the row to `hibernated` with a
// retention deadline. Returns true only when the row is durably hibernated; any
// failure returns false (and un-claims where safe) so the caller falls back to
// Terminate. Idempotent per tick — a lost claim (concurrent Cancel/Resume) just
// returns false.
func (c *Completer) hibernate(ctx context.Context, record store.AgentSession) bool {
	claimed, err := c.Store.ClaimAgentSessionForHibernation(ctx, record.ID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("agent-session completer: hibernation claim failed (session=%s): %v", record.ID, err)
		}
		return false // lost the claim (cancel/resume raced) or transient — don't terminate blind
	}
	ref, putURL, err := c.Snapshots.PrepareUpload(ctx, claimed.WorkspaceID, claimed.ID)
	if err != nil {
		log.Printf("agent-session completer: presign upload failed (session=%s): %v", claimed.ID, err)
		c.unclaimHibernation(ctx, claimed, record.Phase)
		return false
	}
	snap, err := c.Sandbox.HibernateAgentSessionSandbox(ctx, claimed.WorkspaceID, claimed.ID, claimed.SandboxID, putURL)
	if err != nil {
		log.Printf("agent-session completer: snapshot exec failed (session=%s): %v", claimed.ID, err)
		_ = c.Snapshots.Delete(ctx, ref) // best-effort: drop a partial upload
		c.unclaimHibernation(ctx, claimed, record.Phase)
		return false
	}
	// The snapshot is durable in object storage AND the pod is still alive. Record
	// it on the row FIRST (keeping sandbox_id), so a crash here leaves a live pod
	// for the fallback reap — never a stranded row with a lost pod handle. A dirty
	// git tree (uncommitted work) extends the retention window (ADR059 D5).
	retainUntil := c.now().Add(c.retentionWindow(snap.DirtyGit))
	if _, err := c.Store.HibernateAgentSession(ctx, claimed.ID, ref, snap.Bytes, snap.SHA256, retainUntil); err != nil {
		log.Printf("agent-session completer: hibernate row write failed (session=%s ref=%s): %v", claimed.ID, ref, err)
		_ = c.Snapshots.Delete(ctx, ref) // orphaned blob; the pod stays for the fallback
		c.unclaimHibernation(ctx, claimed, record.Phase)
		return false
	}
	// Durable + hibernated. Terminate the now-idle pod and drop its id. A terminate
	// failure leaves a lingering pod (OpenSandbox GCs it on its own timeout) but the
	// row is already correct — never a double-billed *stranded* row.
	_ = c.Sandbox.CancelAgentSessionSandbox(ctx, claimed.WorkspaceID, claimed.ID, claimed.SandboxID)
	if err := c.Store.ClearAgentSessionSandbox(ctx, claimed.ID); err != nil {
		log.Printf("agent-session completer: clear sandbox id after hibernate failed (session=%s): %v", claimed.ID, err)
	}
	log.Printf("agent-session completer: hibernated session=%s ref=%s bytes=%d dirty=%v", claimed.ID, ref, snap.Bytes, snap.DirtyGit)
	return true
}

// unclaimHibernation returns a claimed row (now `hibernating`) to its original
// terminal phase so the fallback Terminate — or a later retry — proceeds. A
// failure is only logged (the reaper retries next tick).
func (c *Completer) unclaimHibernation(ctx context.Context, claimed store.AgentSession, originalPhase string) {
	if _, err := c.Store.FinalizeAgentSession(ctx, claimed.ID, originalPhase, "", "", 0, nil, claimed.FailureReason); err != nil {
		log.Printf("agent-session completer: unclaim hibernation failed (session=%s): %v", claimed.ID, err)
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
// prefix cannot suppress the missing suffix.
func (c *Completer) captureTranscript(ctx context.Context, record store.AgentSession) {
	raw, err := c.Sandbox.ReadSessionTranscript(ctx, record.WorkspaceID, record.ID, record.SandboxID)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			log.Printf("agent-session completer: transcript read failed (session=%s): %v", record.ID, err)
		}
		c.markTurnTranscript(ctx, record, false, true, reasonTranscriptReadFailed)
		return
	}
	parts, parseTruncated := parseTranscriptLog(raw, record.Turns)
	if len(parts) == 0 {
		reason := ""
		if parseTruncated {
			reason = reasonParseTruncated
		}
		c.markTurnTranscript(ctx, record, !parseTruncated, parseTruncated, reason)
		return
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
		return
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
		return
	}
	// Compute final truncation state. Cache the evidence check to avoid redundant JSON unmarshal.
	evidenceTruncated := transcriptEvidenceTruncated(record.Evidence)
	truncated := parseTruncated || quotaTruncated || evidenceTruncated
	reason := ""
	if parseTruncated {
		reason = reasonParseTruncated
	} else if quotaTruncated {
		reason = reasonQuotaExceeded
	} else if evidenceTruncated {
		reason = reasonDriverTruncated
	}
	c.markTurnTranscript(ctx, record, !truncated, truncated, reason)
	log.Printf("agent-session completer: captured transcript (session=%s turn=%d parts=%d)", record.ID, record.Turns, len(kept))
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

func transcriptEvidenceTruncated(raw json.RawMessage) bool {
	var evidence struct {
		Truncated bool `json:"truncated"`
	}
	return json.Unmarshal(raw, &evidence) == nil && evidence.Truncated
}

// parseTranscriptLog extracts the UI-message parts from the driver's redacted
// JSONL log (one `{"at":…,"type":"ui-message","part":{…}}` record per line, plus
// the raw data-acp lines). It keeps only the `.part` payload — the exact shape
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

func prTitle(config AgentConfig, sessionID string) string {
	task := firstLine(config.Task)
	if task == "" {
		return "bex agent session " + sessionID
	}
	if len(task) > 72 {
		task = task[:72]
	}
	return "bex agent: " + task
}

// prBody is the draft PR's description: session metadata only (w5/m65). The
// evidence digest it used to inline was a lossy re-derivation of what the PR
// already shows better — GitHub renders the real diff and commit list, and the
// "test output" was whatever tool call happened to emit stdout (the driver's
// extractor cannot tell a test run from a grep).
func prBody(record store.AgentSession, report statusReport, apiURL string) string {
	var b strings.Builder
	b.WriteString("Opened by a bex cloud coding-agent session.\n\n")
	fmt.Fprintf(&b, "- Session: `%s`\n- Branch: `%s`\n- Head: `%s`\n",
		record.ID, record.Branch, report.Delivery.HeadSHA)
	if report.Delivery.Commits > 0 {
		fmt.Fprintf(&b, "- Commits: %d\n", report.Delivery.Commits)
	}
	if apiURL != "" {
		fmt.Fprintf(&b, "- API: %s/v1/agent-sessions/%s\n", strings.TrimRight(apiURL, "/"), record.ID)
	}
	return b.String()
}

// buildEvidence bounds the driver-reported evidence a second time on the server
// so a misbehaving or forward-version driver cannot store an unbounded blob.
// Any cap that drops content marks Truncated.
func buildEvidence(report statusReport) Evidence {
	truncated := report.Evidence.Truncated
	commands, t1 := capLines(report.Evidence.CommandLog, maxEvidenceCommands)
	tests, t2 := capLines(report.Evidence.TestOutput, maxEvidenceTestLines)
	changed, t3 := capLines(report.Delivery.ChangedFiles, maxEvidenceChanged)
	tail := report.Evidence.OutputTail
	if len(tail) > maxEvidenceOutputTail {
		tail = tail[len(tail)-maxEvidenceOutputTail:]
		truncated = true
	}
	return Evidence{
		CommandLog:   commands,
		TestOutput:   tests,
		OutputTail:   tail,
		ChangedFiles: changed,
		Commits:      report.Delivery.Commits,
		Truncated:    truncated || t1 || t2 || t3,
	}
}

// capLines caps a slice's length and each element's length, reporting whether
// anything was dropped.
func capLines(in []string, max int) ([]string, bool) {
	truncated := false
	if len(in) > max {
		in = in[:max]
		truncated = true
	}
	out := make([]string, 0, len(in))
	for _, line := range in {
		if len(line) > maxEvidenceLineLen {
			line = line[:maxEvidenceLineLen]
			truncated = true
		}
		out = append(out, line)
	}
	return out, truncated
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
