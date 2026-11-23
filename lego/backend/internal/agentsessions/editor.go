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
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Opening a closed session in an editor (w6/m564, ADR054 D8). Since w6/m563 a
// person's session closes its sandbox 5 minutes after its turn, and opening a
// sleeping Devin session's Files or Terminal wakes it (docs.devin.ai release
// notes, 2026-10-05). Resume does that here: it starts the sandbox the next
// turn would get, from the kept disk or the branch, and runs no turn in it.

const (
	// envEditor tells the driver its sandbox is an editor's: it checks out
	// the workspace, reports driverStateReady and runs no turn.
	envEditor = "BEX_AGENT_EDITOR"
	// editorStartBound is how long a session may read resuming with no
	// sandbox: the start's own ceiling and a minute. Past it the start died
	// with its replica, and the session returns to its finished phase.
	editorStartBound = dispatchTimeout + time.Minute
)

// Resume gives the session a sandbox an editor can open (ADR054 D8), as
// opening a sleeping Devin session's workspace wakes it. A session whose turn
// runs or starts has one: it is returned as it is. A finished session whose
// sandbox is still up keeps it for the editor: it closes on a person's idle
// timeout from now. A finished session with none starts one in the
// background and reads resuming until it is up; a message sent meanwhile
// waits in the queue. Resume never runs a turn.
func (s *Service) Resume(ctx context.Context, sessionID string) (View, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return View{}, err
	}
	if !s.enabled() || !s.modelProxyEnabled() || !s.ticketEnabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return View{}, err
	}
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	// Resume provisions broker-eligible compute. A contributor may operate a
	// live session, but cannot use that lifecycle permission to revive it
	// after losing the workspace's create authority.
	if err := s.AuthorizeFreshOn(ctx, core.RelCanCreate, sessionObject(sessionID)); err != nil {
		return View{}, err
	}
	if record.SandboxID != "" && steerable(record.Phase, record.ArchivedAt != nil) {
		kept, err := s.Store.KeepAgentSessionSandboxForEditor(ctx, record.ID, record.SandboxID)
		if !errors.Is(err, store.ErrNotFound) {
			if err != nil {
				return View{}, mapStoreError(record.ID, err)
			}
			return s.toView(kept)
		}
		// It closed, or a turn began, since the read.
		if record, err = s.Store.GetAgentSession(ctx, sessionID); err != nil {
			return View{}, mapStoreError(sessionID, err)
		}
	}
	switch {
	case record.ArchivedAt != nil:
		return View{}, errArchived(record.Phase) // ADR065 D1: unarchive first
	case record.Phase == PhaseCanceled || record.Phase == PhaseCanceling:
		return View{}, core.NewConflictError("AGENT_SESSION_NOT_RESUMABLE", "agent session cannot be resumed from its current phase", map[string]any{"phase": record.Phase})
	case liveSandboxPhase(record.Phase) || record.SandboxID != "":
		return s.toView(record)
	}
	// A fresh sandbox is admitted as a turn's is: payment method, billing and
	// the live-sandbox cap.
	if err := s.admitDispatch(ctx, record.WorkspaceID); err != nil {
		return View{}, err
	}
	started, err := s.Store.BeginAgentSessionEditorStart(ctx, record.ID)
	if errors.Is(err, store.ErrNotFound) {
		// Another resume, a message or an archive got there first.
		if started, err = s.Store.GetAgentSession(ctx, sessionID); err != nil {
			return View{}, mapStoreError(sessionID, err)
		}
		return s.toView(started)
	}
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	s.detach(ctx, func(ctx context.Context) { s.runEditorStart(ctx, started) })
	return s.toView(started)
}

// runEditorStart starts the editor's sandbox in the background. A start that
// fails returns the session to its finished phase with no sandbox.
func (s *Service) runEditorStart(ctx context.Context, record store.AgentSession) {
	err := s.startEditorSandbox(ctx, record)
	if err == nil {
		return
	}
	log.Printf("agent-session resume: the editor's sandbox did not start (session=%s): %v", record.ID, err)
	if _, settleErr := s.Store.SettleAgentSessionEditorStart(context.WithoutCancel(ctx), record.ID); settleErr != nil && !errors.Is(settleErr, store.ErrNotFound) {
		// The Completer settles it once editorStartBound has passed.
		log.Printf("agent-session resume: return the session to its finished phase failed (session=%s): %v", record.ID, settleErr)
	}
}

// startEditorSandbox creates the sandbox the session's next turn would get,
// with no prompt: its kept disk when retention kept one, else its branch,
// with its secrets, repository environment and egress. It binds it to the
// session only while the session still waits for it; a session that was
// canceled, archived or given up on meanwhile has the sandbox ended.
func (s *Service) startEditorSandbox(ctx context.Context, record store.AgentSession) error {
	config, err := decodeAgentConfig(record)
	if err != nil {
		return err
	}
	workspaceEnv, err := s.workspaceEnvironment(ctx, record.WorkspaceID)
	if err != nil {
		return err
	}
	githubCLIOff, err := s.githubCLIOffLevel(ctx, record, workspaceEnv)
	if err != nil {
		return err
	}
	modelEndpoint, allowlist, err := createEgress(config, effectiveHosts(workspaceEnv), nil, workspaceEnv.ModelKeySet())
	if err != nil {
		return err
	}
	modelAPIKey, err := s.modelCredential(ctx, record.WorkspaceID, record.ID)
	if err != nil {
		return err
	}
	env := s.withNetworkEnv(s.driverEnv(config, record, workspaceEnv, githubCLIOff), workspaceEnv.NetworkTier, record.WorkspaceID, record.ID)
	// No prompt, so the driver runs no turn, and no turn number, so no turn's
	// cleanup takes the sandbox for its own.
	delete(env, "BEX_AGENT_PROMPT")
	delete(env, "BEX_AGENT_TURN")
	env[envEditor] = "1"
	if env, _, err = s.withSessionSecrets(ctx, record, env); err != nil {
		return err
	}
	env, repoEnv, err := s.withRepoEnvironment(ctx, record, env)
	if err != nil {
		return err
	}
	spec := dispatchSpec{template: strings.TrimSpace(config.Template), modelEndpoint: modelEndpoint, modelAPIKey: modelAPIKey,
		egress: sessionEgress(workspaceEnv, allowlist), env: env, editor: true}
	spec.retained = s.Retention.claim(ctx, s.Now, record.ID, spec.template)
	claimed, bound := spec.retained, false
	defer func() {
		if claimed != nil && !bound {
			s.Retention.unclaim(claimed.ID)
		}
	}()
	boot, err := s.bootSandbox(ctx, record, spec, repoEnv)
	if err != nil {
		return err
	}
	end := func(cause error) error {
		if err := s.Sandbox.CancelAgentSessionSandbox(context.WithoutCancel(ctx), record.WorkspaceID, record.ID, boot.ID); err != nil {
			log.Printf("agent-session resume: end the unbound sandbox failed (session=%s sandbox=%s): %v", record.ID, boot.ID, err)
		}
		return cause
	}
	if err := s.Sandbox.EnterAgentSessionPhase(ctx, record.WorkspaceID, record.ID, boot.ID, modelEndpoint, spec.egress); err != nil {
		return end(err)
	}
	if _, err := s.Store.BindAgentSessionEditorSandbox(ctx, record.ID, boot.ID); err != nil {
		return end(err)
	}
	bound = true
	if boot.retained != nil {
		// The sandbox holds the kept disk's work now; its close keeps it again.
		s.Retention.finish(ctx, s.Now, record.ID, boot.retained.ID)
	}
	// The checkout is ready within seconds: look for it on the short tick.
	s.Stops.Note(record.ID, s.Now())
	return nil
}

// settleEditorStart converges a session resume is starting a sandbox for. It
// returns to its finished phase, keeping the sandbox for the idle close, once
// the driver has checked the workspace out, failed to, or said nothing within
// the turn ceiling; a sandbox that is gone is dropped. A start that bound no
// sandbox within its own ceiling died with its replica: the session returns
// with none, and the inventory reconcile ends whatever that start created.
func (c *Completer) settleEditorStart(ctx context.Context, record store.AgentSession) {
	waited := c.now().Sub(record.UpdatedAt)
	if record.SandboxID == "" {
		if waited >= editorStartBound {
			c.endEditorStart(ctx, record, "it bound no sandbox", false)
		}
		return
	}
	readCtx, cancel := context.WithTimeout(ctx, statusReadTimeout)
	raw, err := c.Sandbox.ReadSessionStatus(readCtx, record.WorkspaceID, record.ID, record.SandboxID)
	cancel()
	var report statusReport
	switch {
	case errors.Is(err, core.ErrNotFound):
		c.endEditorStart(ctx, record, "its sandbox is gone", true)
		return
	case err == nil && json.Unmarshal([]byte(strings.TrimSpace(raw)), &report) == nil && report.State != "":
		if report.State != driverStateReady {
			log.Printf("agent-session completer: the editor's checkout reported %s (session=%s): %s", report.State, record.ID, boundedText(report.Error, maxDeliveryReasonRunes))
		}
	case waited < turnCeiling(c.TurnTimeout):
		return // still checking out, or a read to retry
	}
	c.endEditorStart(ctx, record, "the sandbox is up", false)
}

// endEditorStart returns the session to its finished phase, and with lost its
// sandbox is dropped.
func (c *Completer) endEditorStart(ctx context.Context, record store.AgentSession, why string, lost bool) {
	settled, err := c.Store.SettleAgentSessionEditorStart(ctx, record.ID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("agent-session completer: settle the editor's start failed (session=%s): %v", record.ID, err)
		}
		return
	}
	log.Printf("agent-session completer: the editor's start settled, %s (session=%s phase=%s sandbox=%s)", why, record.ID, settled.Phase, settled.SandboxID)
	if lost {
		c.terminate(ctx, settled)
	}
}
