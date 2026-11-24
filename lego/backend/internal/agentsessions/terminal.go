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
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/shellticket"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The session page's Terminal (w10/m615, ADR054 D10): a shell in the
// session's sandbox from the browser, over the gateway's Browser Web Shell
// (ADR035), as Devin's session workspace has "a new Terminal tab for running
// commands on Devin's machine", and "opening Files or Terminal wakes Devin"
// (docs.devin.ai/release-notes/2026, 2026-10-05). A browser has no SSH key, so
// the gateway's wake verb (D9) cannot start a closed sandbox for it; bex-api
// holds the caller's identity when it mints the Terminal's ticket and starts
// the sandbox itself, through the same Resume an editor's start runs.

// ShellKeyAction is what a bex API key's Terminal ticket names: Sandbox
// execution, the action native SSH holds an agent-session login to
// (nativessh keyOwnerAction) and every other command run in a sandbox needs.
// The gateway holds the shell to it for as long as it runs (w9/125).
const ShellKeyAction = core.KeySandboxesExec

// What a minted Terminal ticket's sandbox is (ShellSessionView.Sandbox).
const (
	// ShellSandboxUp: the sandbox is up, and the shell opens at once.
	ShellSandboxUp = "up"
	// ShellSandboxStarting: the sandbox is starting, because this mint
	// started it or a turn or another start is under way. The gateway holds
	// the WebSocket until it is up, saying so, then opens the shell.
	ShellSandboxStarting = "starting"
)

// shellTicketTTL bounds a Terminal ticket, as it bounds a service's Shell
// ticket: the browser opens the WebSocket at once, and the gateway, not the
// ticket, holds the wait for a sandbox that is starting.
const shellTicketTTL = 90 * time.Second

// ShellSessionView is a minted Terminal ticket: what the browser presents to
// the gateway's WebSocket at URL, and whether the sandbox is up or starting.
// It carries no terminal content and no SSH key.
type ShellSessionView struct {
	Ticket    string `json:"ticket"`
	URL       string `json:"url"`
	ExpiresAt string `json:"expiresAt"`
	Sandbox   string `json:"sandbox"`
}

// terminalEnabled reports whether this bex-api can mint Terminal tickets.
func (s *Service) terminalEnabled() bool {
	return len(s.ShellTicketSecret) > 0 && strings.TrimSpace(s.ShellWSURL) != ""
}

// CreateShellSession mints a Terminal ticket for the session's sandbox. The
// caller needs can_view_sensitive on the session, as an SSH login does
// (ADR054 D2): a shell reads everything the sandbox holds. A sandbox that is
// up opens at once, and one that is starting is waited for. A completed or
// failed, unarchived session whose sandbox closed is started here as the
// caller through Resume, which needs can_operate and a fresh can_create and
// admits the start like a turn. A caller who may view it but not start it is
// told so with the SSH path's words, and resume's own refusals (payment
// method, billing, the live-sandbox cap) keep their codes. A canceled or
// archived session has no sandbox to open. bex-api only authorizes and
// signs: the gateway re-authorizes as the ticket's subject before every
// shell and on every revalidation tick, and alone holds pods/exec.
func (s *Service) CreateShellSession(ctx context.Context, sessionID string) (ShellSessionView, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanViewSensitive, sessionObject(sessionID)); err != nil {
		return ShellSessionView{}, err
	}
	if !s.terminalEnabled() {
		return ShellSessionView{}, core.ErrShellUnavailable
	}
	if !s.enabled() {
		return ShellSessionView{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return ShellSessionView{}, err
	}
	identity, _ := core.IdentityFrom(ctx)
	if identity.Subject == "" {
		return ShellSessionView{}, core.ErrForbidden
	}
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return ShellSessionView{}, mapStoreError(sessionID, err)
	}
	sandbox, err := s.terminalSandbox(ctx, record)
	if err != nil {
		return ShellSessionView{}, err
	}
	now := s.Now()
	expires := now.Add(shellTicketTTL)
	token, err := shellticket.Mint(s.ShellTicketSecret, shellticket.Claims{
		Subject:   identity.Subject,
		SessionID: record.ID,
		KeyAction: core.KeyStreamAction(ctx, ShellKeyAction),
		IssuedAt:  now.Unix(),
		ExpiresAt: expires.Unix(),
	})
	if err != nil {
		return ShellSessionView{}, err
	}
	return ShellSessionView{Ticket: token, URL: s.ShellWSURL, ExpiresAt: expires.UTC().Format(time.RFC3339), Sandbox: sandbox}, nil
}

// terminalSandbox is whether the Terminal's shell opens at once or after a
// start, starting a closed sandbox as the caller. It refuses a session with
// no sandbox to open, with the resolver's own refusal (errNoSandboxToOpen),
// the words an SSH login's banner uses (w10/043).
func (s *Service) terminalSandbox(ctx context.Context, record store.AgentSession) (string, error) {
	switch {
	case openableSandbox(record):
		return ShellSandboxUp, nil
	case liveSandboxPhase(record.Phase):
		return ShellSandboxStarting, nil
	case !steerable(record.Phase, record.ArchivedAt != nil):
		return "", errNoSandboxToOpen(record) // canceled, canceling or archived
	}
	// Closed. A bex API key starts it only when its grant covers starting a
	// session, as its own resume needs (agents.manage), besides the Sandbox
	// execution its mint already needed.
	if granted, err := s.KeyGrantAllows(ctx, core.KeyAgentsManage); err != nil {
		return "", err
	} else if !granted {
		return "", errNoSandboxToOpen(record)
	}
	started, err := s.Resume(ctx, record.ID)
	switch {
	case core.IsRoleRefusal(err):
		// No can_operate, or no fresh can_create: the caller may view the
		// sandbox but not start it. A refusal that names its own cause (an
		// OAuth scope, a login requirement) passes through below.
		return "", errNoSandboxToOpen(record)
	case err != nil:
		return "", err
	case started.SandboxID != "" && started.Phase != PhaseResuming:
		// A turn's sandbox came up meanwhile, or Resume kept one for the
		// editor.
		return ShellSandboxUp, nil
	}
	return ShellSandboxStarting, nil
}
