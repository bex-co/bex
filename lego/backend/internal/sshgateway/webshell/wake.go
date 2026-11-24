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

package webshell

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/shellticket"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

// An agent session's Terminal (w10/m615, ADR054 D10). Its ticket names the
// session, and the gateway opens a shell in the session's sandbox through the
// agent-session resolver, as native SSH does for an ags-… login. bex-api's
// mint already started a sandbox that closed, as the caller, so a sandbox
// that is starting is waited for here: the WebSocket stays open, the browser
// is told the sandbox is starting and, every wakeProgressInterval, that it
// still is, and the shell opens once the session settles with a sandbox,
// re-authorized as the ticket's subject on every read. Opening Terminal on a
// sleeping Devin session wakes it the same way
// (docs.devin.ai/release-notes/2026, 2026-10-05).

const (
	// defaultWakePollInterval is how often a waiting Terminal reads whether
	// its session settled, as a held SSH login does.
	defaultWakePollInterval = 2 * time.Second
	// wakeProgressInterval is how often a waiting Terminal says it still waits.
	wakeProgressInterval = 30 * time.Second
	// maxUnansweredPolls is how many reads in a row may go unanswered (an
	// authorization store that did not answer) before the wait ends, as a
	// held SSH login allows.
	maxUnansweredPolls = 3
)

// The outcomes bex_ssh_gateway_web_shell_wakes_total counts.
const (
	wakeOpened    = "opened"
	wakeNoSandbox = "no_sandbox"
	wakeTimedOut  = "timed_out"
	wakeAbandoned = "abandoned"
)

// errSessionCap is the cause an agent session's Terminal ends on at
// SessionTimeout.
var errSessionCap = errors.New("the agent session's shell reached its time limit")

// errBrowserGone ends a wait whose browser closed the WebSocket.
var errBrowserGone = errors.New("the browser closed the shell")

// timeLimited reports whether cause, why the request context ended, is a
// time limit: the gateway's drain bound or an agent session's Terminal cap.
func timeLimited(cause error) bool {
	return errors.Is(cause, sshgateway.ErrGatewayShutdown) || errors.Is(cause, errSessionCap)
}

// sessionEndMessages are the English fallbacks of an agent session's
// Terminal where the service wording does not fit.
var sessionEndMessages = map[string]string{
	endRevoked:      "your access to this agent session ended",
	endInstanceGone: "the sandbox this session ran in stopped",
}

// ticketTarget is what a verified ticket opens: a service, through Apps
// exactly as before, or an agent session's sandbox, through AgentSessions.
// The ticket's field decides, never the id's prefix.
type ticketTarget struct {
	session   bool
	username  string
	subject   string
	keyAction string
	keyGrants *sshgateway.KeyGrantGate
	resolver  sshgateway.TargetResolver
}

func (s *Server) ticketTarget(claims shellticket.Claims) ticketTarget {
	t := ticketTarget{username: claims.Username(), subject: claims.Subject, keyAction: claims.KeyAction,
		keyGrants: s.KeyGrants, resolver: s.Apps}
	if claims.SessionID != "" {
		t.session, t.resolver = true, s.AgentSessions
	}
	return t
}

func (t ticketTarget) noun() string {
	if t.session {
		return "agent session"
	}
	return "service"
}

// as is ctx acting as the ticket's subject, the identity every resolution of
// the ticket runs under.
func (t ticketTarget) as(ctx context.Context) context.Context {
	return core.WithIdentity(ctx, core.Identity{Subject: t.subject, Method: core.MethodShell})
}

func (t ticketTarget) resolve(ctx context.Context) (apps.SSHInstanceTarget, error) {
	if t.resolver == nil {
		return apps.SSHInstanceTarget{}, core.ErrForbidden // no agent-session resolver wired
	}
	return t.resolver.ResolveSSHSession(ctx, t.username)
}

// keyGrant holds a ticket a bex API key minted to the key's current grant
// (w9/125); a person's ticket names no action and never asks.
func (t ticketTarget) keyGrant(ctx context.Context) error {
	return t.keyGrants.RequireTicket(ctx, t.subject, t.keyAction)
}

// sandboxState reads a refused resolution of an agent session for the
// sandbox it says the session has: agentsessions.SSHSandboxStarting (a turn
// or bex-api's mint is starting one, which the Terminal waits for) or
// SSHSandboxClosed (that start ended without one); "" for any other refusal.
func (t ticketTarget) sandboxState(err error) string {
	if !t.session {
		return ""
	}
	_, state, workspace := agentsessions.SSHNoSandbox(err)
	if state == agentsessions.SSHSandboxStarting && workspace == "" {
		return ""
	}
	return state
}

// refusal is what the terminal shows when the first resolution refused: a
// service's shell says only that no instance is available, as before. An
// agent session's names why its sandbox cannot open (canceled, archived),
// which the resolver says only after can_view_sensitive passed, so no more
// than the caller's own read of the session would.
func (t ticketTarget) refusal(err error) string {
	if !t.session {
		return "this service has no instance available for a shell right now"
	}
	if refusal, _, _ := agentsessions.SSHNoSandbox(err); refusal != nil {
		return refusal.Error()
	}
	return "this agent session's sandbox can't be opened right now"
}

// endReason classifies the error the revalidation watchdog ended a live
// shell on. An agent session's sandbox that closed under it ended with its
// instance.
func (t ticketTarget) endReason(err error) string {
	if refusal, _, _ := agentsessions.SSHNoSandbox(err); t.session && refusal != nil {
		return endInstanceGone
	}
	return watchdogEndReason(err)
}

// end tells the terminal why the server ended its live session, in the
// target's words.
func (t ticketTarget) end(ws *wsConn, reason string) {
	if msg, ok := sessionEndMessages[reason]; ok && t.session {
		ws.endWith(reason, msg)
		return
	}
	ws.end(reason)
}

// endWait tells the browser why its wait ended without a shell: a named
// end, the resolver's message, or a time limit. A browser that left is told
// nothing.
func (t ticketTarget) endWait(ws *wsConn, err error) {
	var ended *waitEnd
	switch {
	case errors.As(err, &ended):
		if ended.reason != "" {
			t.end(ws, ended.reason)
		} else {
			ws.fail(ended.message)
		}
	case errors.Is(err, errBrowserGone):
		ws.close(websocket.CloseNormalClosure, "")
	case timeLimited(err):
		t.end(ws, endTimeout)
	default:
		ws.fail("session closed")
	}
}

// waitEnd is why a Terminal's wait ended without a sandbox: an end reason
// the dashboard names, or the resolver's own message.
type waitEnd struct {
	reason  string
	message string
}

func (e *waitEnd) Error() string {
	if e.message != "" {
		return e.message
	}
	return endMessages[e.reason]
}

// browserInput is the browser's side of a shell: one pump reads its frames
// for the WebSocket's whole life (a gorilla connection takes one reader),
// and a keepalive pings it. Until the shell opens, keystrokes are dropped and
// only the latest size is kept, so a Terminal waiting for its sandbox
// (w10/m615) needs no reader of its own. done closes once the frames stop.
type browserInput struct {
	stdin *io.PipeReader
	held  *heldStdin
	sizes *heldSizes
	done  chan struct{}
}

func startBrowserInput(ctx context.Context, ws *wsConn) *browserInput {
	pr, pw := io.Pipe()
	in := &browserInput{stdin: pr, held: &heldStdin{pw: pw}, sizes: &heldSizes{}, done: make(chan struct{})}
	go func() {
		pumpBrowserInput(ws, in.held, in.sizes)
		_ = in.held.pw.Close()
		close(in.done)
	}()
	go pumpKeepalive(ctx, ws)
	return in
}

// openStdin connects the browser's keystrokes to the shell's stdin.
func (in *browserInput) openStdin() { in.held.open() }

// open connects the browser to the shell that starts now: its stdin, and its
// sizes from the default 80x24 on, the latest the browser sent first.
func (in *browserInput) open(ctx context.Context) (*io.PipeReader, *sshgateway.SizeQueue) {
	in.openStdin()
	return in.stdin, in.sizes.open(ctx)
}

// heldStdin drops what the browser types until the shell's stdin is open.
type heldStdin struct {
	mu     sync.Mutex
	pw     *io.PipeWriter
	opened bool
}

func (h *heldStdin) Write(p []byte) (int, error) {
	h.mu.Lock()
	opened := h.opened
	h.mu.Unlock()
	if !opened {
		return len(p), nil
	}
	return h.pw.Write(p)
}

func (h *heldStdin) open() {
	h.mu.Lock()
	h.opened = true
	h.mu.Unlock()
}

// heldSizes keeps the browser's latest terminal size until the shell's size
// queue exists, so resizes sent while a sandbox starts are not lost to the
// queue's bound before anything reads it.
type heldSizes struct {
	mu     sync.Mutex
	latest *remotecommand.TerminalSize
	queue  *sshgateway.SizeQueue
}

func (h *heldSizes) push(size remotecommand.TerminalSize) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.queue != nil {
		h.queue.Push(size)
		return
	}
	h.latest = &size
}

func (h *heldSizes) open(ctx context.Context) *sshgateway.SizeQueue {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.queue == nil {
		h.queue = sshgateway.NewSizeQueue(ctx, &remotecommand.TerminalSize{Width: 80, Height: 24})
		if h.latest != nil {
			h.queue.Push(*h.latest)
		}
	}
	return h.queue
}

// awaitSandbox holds a Terminal whose agent session's sandbox is starting
// until it is up, and returns it. It tells the browser the sandbox is
// starting, every wakeProgress that it still is, then that it is ready. It
// ends without one when the start ends without a sandbox, the start's
// ceiling passes, the session is canceled or archived, the subject's access
// or the key's grant ends, or the browser, the Terminal's cap or the
// gateway's drain ends the WebSocket; and counts how, with no identifying
// label.
func (s *Server) awaitSandbox(ctx context.Context, ws *wsConn, in *browserInput, t ticketTarget) (apps.SSHInstanceTarget, error) {
	_ = ws.control(serverControl{Type: "starting"})
	result, target, err := s.pollSandbox(ctx, ws, in, t)
	s.Metrics.WebShellWake(result)
	if err != nil {
		return apps.SSHInstanceTarget{}, err
	}
	_ = ws.control(serverControl{Type: "ready"})
	return target, nil
}

func (s *Server) pollSandbox(ctx context.Context, ws *wsConn, in *browserInput, t ticketTarget) (string, apps.SSHInstanceTarget, error) {
	started := time.Now()
	ceiling := time.NewTimer(s.WakeTimeout)
	defer ceiling.Stop()
	tick := time.NewTicker(s.wakePollInterval)
	defer tick.Stop()
	progress := time.NewTicker(s.wakeProgress)
	defer progress.Stop()
	unanswered := 0
	for {
		select {
		case <-ctx.Done():
			return wakeAbandoned, apps.SSHInstanceTarget{}, context.Cause(ctx)
		case <-in.done:
			return wakeAbandoned, apps.SSHInstanceTarget{}, errBrowserGone
		case <-ceiling.C:
			return wakeTimedOut, apps.SSHInstanceTarget{}, &waitEnd{reason: endWakeTimeout}
		case <-progress.C:
			_ = ws.control(serverControl{Type: "starting", Elapsed: int(time.Since(started).Round(time.Second) / time.Second)})
			continue
		case <-tick.C:
		}
		target, err := s.readSandbox(ctx, t)
		if err == nil {
			return wakeOpened, target, nil
		}
		refusal, _, _ := agentsessions.SSHNoSandbox(err)
		switch reason := watchdogEndReason(err); {
		case t.sandboxState(err) == agentsessions.SSHSandboxStarting:
			unanswered = 0
			continue
		case t.sandboxState(err) == agentsessions.SSHSandboxClosed: // the start ended without a sandbox
			return wakeNoSandbox, apps.SSHInstanceTarget{}, &waitEnd{reason: endNoSandbox}
		case refusal != nil: // canceled or archived meanwhile
			return wakeNoSandbox, apps.SSHInstanceTarget{}, &waitEnd{message: refusal.Error()}
		case reason != endUnverified: // access or the key's grant ended
			return wakeAbandoned, apps.SSHInstanceTarget{}, &waitEnd{reason: reason}
		}
		if unanswered++; unanswered >= maxUnansweredPolls {
			log.Printf("web shell: ending a Terminal's wait after %d reads in a row without an answer: %v", unanswered, err)
			return wakeAbandoned, apps.SSHInstanceTarget{}, &waitEnd{reason: endUnverified}
		}
	}
}

// readSandbox re-asks the key's grant and resolves the session as the
// ticket's subject: the target once the sandbox is up, else why not. A grant
// that narrowed ends the wait whatever the sandbox is doing.
func (s *Server) readSandbox(ctx context.Context, t ticketTarget) (apps.SSHInstanceTarget, error) {
	readCtx, cancel := context.WithTimeout(t.as(ctx), s.HandshakeTimeout)
	defer cancel()
	if err := t.keyGrant(readCtx); err != nil {
		return apps.SSHInstanceTarget{}, err
	}
	return t.resolve(readCtx)
}
