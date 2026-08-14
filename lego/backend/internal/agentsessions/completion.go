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

	// SSHGraceTTL bounds the "Open in Zed" teardown deferral (ADR054 D6): a
	// finished session's sandbox is kept alive while an editor SSH session is
	// open, and an ssh_sessions row older than this is treated as leaked (a
	// crashed gateway replica) and ignored. It equals the gateway's SSH session
	// cap; 0 => the 4h default (matching BEX_SSH_SESSION_TIMEOUT's default).
	SSHGraceTTL time.Duration
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
	for _, row := range rows {
		c.finalize(ctx, row)
	}
	c.reapDeferredSandboxes(ctx)
}

// reapDeferredSandboxes tears down the sandboxes of already-finished sessions
// whose teardown was deferred for an open editor SSH session (ADR054 D6). A
// terminal session drops out of activePhases, so its sandbox would otherwise
// linger forever; each tick this retries teardown for the small set of terminal
// sessions still holding a sandbox id, and teardown itself re-checks the
// deferral so it fires only once the connection has closed (or aged out).
func (c *Completer) reapDeferredSandboxes(ctx context.Context) {
	since := c.now().Add(-2 * c.sshGraceTTL())
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
			c.fail(ctx, record, "sandbox terminated before completion")
		} else {
			log.Printf("agent-session completer: read status failed (session=%s sandbox=%s): %v", record.ID, record.SandboxID, err)
		}
		return
	}
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
		if _, err := c.Store.FinalizeAgentSession(ctx, record.ID, PhaseCompleted, "", "", 0, evidenceJSON, ""); err == nil {
			log.Printf("agent-session completer: completed session=%s (no-op, nothing pushed)", record.ID)
			c.teardown(ctx, record)
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
	conn, err := c.Connections.GetGitConnection(ctx, record.WorkspaceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.fail(ctx, record, "workspace has no GitHub App installation to open a pull request")
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
	if _, err := c.Store.FinalizeAgentSession(ctx, record.ID, PhaseCompleted,
		report.Delivery.HeadSHA, pr.HTMLURL, pr.Number, evidenceJSON, ""); err != nil {
		log.Printf("agent-session completer: finalize failed (session=%s pr=%s): %v", record.ID, pr.HTMLURL, err)
		return // retry next tick; the sandbox stays until the row is finalized
	}
	log.Printf("agent-session completer: completed session=%s pr=%s head=%s", record.ID, pr.HTMLURL, report.Delivery.HeadSHA)
	c.teardown(ctx, record)
}

func (c *Completer) fail(ctx context.Context, record store.AgentSession, reason string) {
	if _, err := c.Store.FinalizeAgentSession(ctx, record.ID, PhaseFailed, "", "", 0, nil, reason); err != nil {
		log.Printf("agent-session completer: finalize-failed write errored (session=%s reason=%q): %v", record.ID, reason, err)
		return
	}
	log.Printf("agent-session completer: failed session=%s reason=%q", record.ID, reason)
	c.teardown(ctx, record)
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
	// Open in Zed (ADR054 D6): keep the sandbox alive while an editor SSH session
	// is connected, so a finishing turn does not kill a live edit. The transcript
	// is already captured above, delivery already happened, and the session is
	// already finalized — only the physical teardown waits. The reaper retries
	// each tick; an explicit Cancel (its own teardown path) is not gated by this,
	// so a user asking to kill the session always wins.
	if open, err := c.Store.HasOpenSSHSession(ctx, record.ID, c.now().Add(-c.sshGraceTTL())); err != nil {
		log.Printf("agent-session completer: ssh-session check failed (session=%s): %v", record.ID, err)
		return // transient store error: retry next tick rather than tear down blind
	} else if open {
		log.Printf("agent-session completer: deferring teardown, editor SSH open (session=%s)", record.ID)
		return
	}
	_ = c.Sandbox.CancelAgentSessionSandbox(ctx, record.WorkspaceID, record.ID, record.SandboxID)
	if err := c.Store.ClearAgentSessionSandbox(ctx, record.ID); err != nil {
		log.Printf("agent-session completer: clear sandbox id failed (session=%s): %v", record.ID, err)
	}
}

// captureTranscript harvests the driver's per-part session log over the SAME
// pods/exec boundary the Completer already uses for the status read (ADR051), and
// appends it to the durable transcript before teardown. Riding the proven
// status-read path — rather than a separate gateway→driver stream dial — is what
// makes persistence reliable. Idempotent per turn (a turn a live viewer already
// teed, or a retry, is skipped) and seq-seeded so a steered turn concatenates.
func (c *Completer) captureTranscript(ctx context.Context, record store.AgentSession) {
	recorded, err := c.Store.AgentSessionTranscriptTurnRecorded(ctx, record.ID, record.Turns)
	if err != nil {
		log.Printf("agent-session completer: transcript turn check failed (session=%s): %v", record.ID, err)
		return
	}
	if recorded {
		return // already captured (live tee or a prior tick)
	}
	raw, err := c.Sandbox.ReadSessionTranscript(ctx, record.WorkspaceID, record.ID, record.SandboxID)
	if err != nil {
		// A gone/unreachable sandbox has no log to harvest — honest empty, not a hang.
		if !errors.Is(err, core.ErrNotFound) {
			log.Printf("agent-session completer: transcript read failed (session=%s): %v", record.ID, err)
		}
		return
	}
	parts := parseTranscriptLog(raw, record.Turns)
	if len(parts) == 0 {
		return
	}
	base, _, err := c.Store.AgentSessionTranscriptMaxSeq(ctx, record.ID)
	if err != nil {
		log.Printf("agent-session completer: transcript max-seq failed (session=%s): %v", record.ID, err)
		return
	}
	// Cumulative quota (w1/m65 F10): the harvest shares the session transcript
	// cap with the gateway's live tee — seed from the already-stored bytes and
	// keep only the parts that fit, so steered/reharvested turns can never grow
	// the stored transcript past it.
	storedBytes, err := c.Store.AgentSessionTranscriptBytes(ctx, record.ID)
	if err != nil {
		log.Printf("agent-session completer: transcript bytes failed (session=%s): %v", record.ID, err)
		return
	}
	total := storedBytes
	kept := parts[:0]
	for _, p := range parts {
		if total+int64(len(p.Part)) > c.transcriptQuota() {
			log.Printf("agent-session completer: transcript quota reached, truncating harvest (session=%s)", record.ID)
			break
		}
		total += int64(len(p.Part))
		kept = append(kept, p)
	}
	if len(kept) == 0 {
		return
	}
	for i := range kept {
		kept[i].Seq = base + 1 + int64(i)
	}
	if err := c.Store.AppendAgentSessionTranscript(ctx, record.ID, kept); err != nil {
		log.Printf("agent-session completer: transcript append failed (session=%s parts=%d): %v", record.ID, len(kept), err)
		return
	}
	log.Printf("agent-session completer: captured transcript (session=%s turn=%d parts=%d)", record.ID, record.Turns, len(kept))
}

// parseTranscriptLog extracts the UI-message parts from the driver's redacted
// JSONL log (one `{"at":…,"type":"ui-message","part":{…}}` record per line, plus
// the raw data-acp lines). It keeps only the `.part` payload — the exact shape
// the GET /stream replay re-frames for a Vercel AI SDK client — stamping the turn;
// Seq is assigned by the caller. Unparseable lines (e.g. a partial leading line
// from the byte-capped tail) are skipped.
func parseTranscriptLog(raw string, turn int) []store.AgentSessionTranscriptPart {
	out := make([]store.AgentSessionTranscriptPart, 0)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec struct {
			Part json.RawMessage `json:"part"`
		}
		if json.Unmarshal([]byte(line), &rec) != nil || len(rec.Part) == 0 {
			continue
		}
		out = append(out, store.AgentSessionTranscriptPart{Turn: turn, Part: rec.Part})
		if len(out) >= maxTranscriptParts {
			break
		}
	}
	return out
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
