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
	evidence := buildEvidence(report)
	evidenceJSON, _ := json.Marshal(evidence) // Evidence is plain data; marshal cannot fail

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
		record.Branch, base, prTitle(record), prBody(record, report, evidence, c.APIPublicURL))
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

// teardown records the turn's conversation transcript (ADR051), then terminates
// the finished session's sandbox and removes its egress policy (idempotent).
// Steering re-dispatches a fresh sandbox, so a completed session never needs its
// sandbox kept warm in phase 1.
//
// The transcript capture MUST happen before the sandbox is destroyed: a
// fire-and-forget turn runs headless with no browser attached, so nothing has
// teed its conversation yet, and the driver's in-memory history dies with the
// pod. This is the only writer for the shipped fire-and-forget product; without
// it every completed session replays empty ("No conversation yet."). It is
// best-effort — a capture failure is logged but never blocks teardown, so a
// session is never stranded waiting on its transcript.
func (c *Completer) teardown(ctx context.Context, record store.AgentSession) {
	if record.SandboxID == "" {
		return
	}
	if err := c.Sandbox.RecordTranscript(ctx, record.WorkspaceID, record.ID, record.SandboxID, record.Turns); err != nil {
		log.Printf("agent-session completer: transcript record failed (session=%s turn=%d): %v", record.ID, record.Turns, err)
	}
	_ = c.Sandbox.CancelAgentSessionSandbox(ctx, record.WorkspaceID, record.ID, record.SandboxID)
}

func prTitle(record store.AgentSession) string {
	task := firstLine(taskOf(record))
	if task == "" {
		return "bex agent session " + record.ID
	}
	if len(task) > 72 {
		task = task[:72]
	}
	return "bex agent: " + task
}

func prBody(record store.AgentSession, report statusReport, ev Evidence, apiURL string) string {
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
	if len(ev.ChangedFiles) > 0 {
		b.WriteString("\n**Changed files**\n\n")
		for _, f := range ev.ChangedFiles {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
	}
	if len(ev.CommandLog) > 0 {
		b.WriteString("\n**Commands run**\n\n```\n")
		b.WriteString(strings.Join(ev.CommandLog, "\n"))
		b.WriteString("\n```\n")
	}
	if len(ev.TestOutput) > 0 {
		b.WriteString("\n**Test output (tail)**\n\n```\n")
		b.WriteString(strings.Join(ev.TestOutput, "\n"))
		b.WriteString("\n```\n")
	}
	if ev.Truncated {
		b.WriteString("\n_Evidence truncated to bounded limits._\n")
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

func taskOf(record store.AgentSession) string {
	config, err := decodeAgentConfig(record)
	if err != nil {
		return ""
	}
	return config.Task
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
