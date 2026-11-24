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

// Package webshell is the Browser Web Shell transport of the isolated SSH
// gateway (docs/ADR035-ssh.md § Browser Web Shell). A browser opens a
// WebSocket to the gateway carrying the exec ticket bex-api minted after
// can_view_sensitive on its target: a service's Shell page, or an agent
// session's Terminal (w10/m615, docs/ADR054-open-in-zed.md D10). The gateway
// verifies the ticket, re-runs ResolveSSHSession (so it re-authorizes and
// re-selects a Ready pod itself — bex-api's decision is not trusted
// transitively), and bridges the browser to one pods/exec stream. An agent
// session's sandbox that is still starting is waited for on the open
// WebSocket. pods/exec stays confined to the gateway process; bex-api never
// gains it. The stream is never logged or persisted, and the audit row
// carries no terminal content — the same boundary as native SSH.
package webshell

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/shellticket"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = 50 * time.Second
	wsReadLimit  = 1 << 20 // bound a single inbound frame (paste), not the session

	// wsShellSubprotocol is the marker protocol the dashboard offers alongside
	// its ticket-bearing sibling and the only one the gateway ever selects — so
	// the credential never appears in the handshake RESPONSE either.
	wsShellSubprotocol = "bex.shell"
	// wsTicketPrefix carries the exec ticket inside the client's
	// Sec-WebSocket-Protocol offer (w1/042 L8): a header, unlike the request
	// line, never reaches Traefik's access log (headers are dropped there). The
	// ticket's base64url alphabet is entirely HTTP-token-safe.
	wsTicketPrefix = "bex.ticket."
)

// Store is the transport's audit authority: content-free session start/end
// rows, shared with native SSH. (The nonce claim reaches the database through
// the injected sshgateway.NonceGuard instead.)
type Store interface {
	StartSSHSession(context.Context, store.SSHSessionAudit) error
	EndSSHSession(context.Context, string, string, time.Time) error
}

// Server is the Browser Web Shell handler. Share Limits and Nonces (and
// Metrics) with the other gateway transports so session caps and exec-ticket
// single-use hold process-wide, not per feature.
type Server struct {
	// TicketSecret must equal bex-api's BEX_SHELL_TICKET_SECRET so the gateway
	// can verify the exec tickets bex-api mints. Empty => the transport is
	// disabled and every request is refused with 503.
	TicketSecret []byte

	Store Store
	Apps  sshgateway.TargetResolver
	// AgentSessions resolves a ticket naming an agent session (its
	// SessionID) to the session's sandbox: agentsessions.SSHResolver, the
	// resolver native SSH routes ags-… logins to. nil refuses every such
	// ticket. A ticket is routed by the field it names, never by an id's
	// prefix, so a service ticket keeps Apps byte-identically.
	AgentSessions sshgateway.TargetResolver
	Executor      sshgateway.Executor
	Metrics       *sshgateway.Metrics
	Limits        *sshgateway.SessionLimiter
	Nonces        *sshgateway.NonceGuard

	// HandshakeTimeout bounds the upgrade. A shell itself has no time limit,
	// as on Render (w6/174): it ends with the browser, its access, its
	// instance or the gateway's drain bound.
	HandshakeTimeout time.Duration

	// RevalidateInterval is how often an ESTABLISHED shell bridge re-runs the
	// target resolution + fresh can_operate authorization the upgrade path
	// performs (codex round-9 #6): a revocation mid-shell cancels the exec
	// context instead of waiting for the browser to disconnect.
	// 0 => the platform default; negative disables (pre-round-9 behavior).
	RevalidateInterval time.Duration

	// TrustedProxies contains the immediate HTTP peers allowed to assert the
	// original browser address through X-Forwarded-For/X-Real-IP. Empty trusts
	// nobody and preserves the direct WebSocket peer address.
	TrustedProxies core.TrustedProxies

	// Audit receives each shell session's StartShellEvent and EndShellEvent
	// rows (w9/m451, sshgateway.RecordShellEvent). nil records none.
	Audit core.AuditSink

	// KeyGrants holds a shell a bex API key opened (its ticket's KeyAction)
	// to the key's current grant at connect and on every revalidation tick
	// (w9/125). nil refuses every key's ticket; a person's never asks.
	KeyGrants *sshgateway.KeyGrantGate

	// SessionTimeout caps an agent session's Terminal, as it caps an SSH
	// connection to an agent session (BEX_SSH_SESSION_TIMEOUT, ADR054 D6),
	// whose open row keeps the sandbox from its idle close. 0 => no cap. A
	// service's shell has none, as on Render (w6/174).
	SessionTimeout time.Duration
	// WakeTimeout bounds how long a Terminal waits for an agent session's
	// sandbox that is starting: the start's own ceiling
	// (agentsessions.SSHWakeTimeout) when 0.
	WakeTimeout time.Duration
	// wakePollInterval is how often a waiting Terminal reads whether the
	// session settled (defaultWakePollInterval when 0), and wakeProgress how
	// often it says it still waits (wakeProgressInterval); tests shorten them.
	wakePollInterval time.Duration
	wakeProgress     time.Duration
}

func (s *Server) defaults() {
	if s.HandshakeTimeout <= 0 {
		s.HandshakeTimeout = 10 * time.Second
	}
	if s.RevalidateInterval == 0 {
		s.RevalidateInterval = sshgateway.DefaultRevalidateInterval
	}
	if s.Limits == nil {
		s.Limits = sshgateway.NewSessionLimiter(0, 0)
	}
	if s.Nonces == nil {
		s.Nonces = &sshgateway.NonceGuard{}
	}
	if s.WakeTimeout <= 0 {
		s.WakeTimeout = agentsessions.SSHWakeTimeout
	}
	if s.wakePollInterval <= 0 {
		s.wakePollInterval = defaultWakePollInterval
	}
	if s.wakeProgress <= 0 {
		s.wakeProgress = wakeProgressInterval
	}
}

// wsUpgrader upgrades the shell connection. Origin is not checked: the exec
// ticket (an unforgeable, short-lived HMAC token that only an authenticated
// bex-api caller can obtain) is the credential, not the browser origin.
// Subprotocols makes the upgrade select bex.shell out of the client's offer —
// never the ticket-bearing entry (browsers require the server to pick one of
// the offered protocols, so the dashboard always offers both).
var wsUpgrader = websocket.Upgrader{
	CheckOrigin:  func(*http.Request) bool { return true },
	Subprotocols: []string{wsShellSubprotocol},
}

// wsTicket extracts the exec ticket exclusively from the subprotocol carrier
// (w1/042 L8). Query-string credentials are deliberately not accepted: request
// paths are retained in ordinary edge access logs.
func wsTicket(r *http.Request) string {
	for _, proto := range websocket.Subprotocols(r) {
		if t, ok := strings.CutPrefix(proto, wsTicketPrefix); ok {
			return t
		}
	}
	return ""
}

// clientControl is a client→server text frame. Only terminal resize is honored;
// stdin arrives as binary frames.
type clientControl struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// serverControl is a server→client text frame: the terminal exit code or a
// bounded error message. Terminal output is sent as binary frames.
type serverControl struct {
	Type    string `json:"type"`              // "exit" | "error" | "starting" | "ready"
	Code    int    `json:"code,omitempty"`    // for type=exit
	Message string `json:"message,omitempty"` // for type=error
	// Reason classifies an error frame that ENDED a live session (w4/m352), one
	// of the end* codes, so the dashboard can name it in the reader's language;
	// Message stays the English fallback. It also names why a Terminal's wait
	// for an agent session's sandbox ended without one (w10/m615). Empty for a
	// refusal before the shell started and for a transport failure.
	Reason string `json:"reason,omitempty"`
	// Elapsed is how long a Terminal has waited for its sandbox, in seconds,
	// on a "starting" frame after the first (w10/m615).
	Elapsed int `json:"elapsed,omitempty"`
}

// The reasons a live session ended, sent as serverControl.Reason (w4/m352).
const (
	endRevoked      = "revoked"
	endSuspended    = apps.ShellRefusalSuspended
	endIneligible   = apps.ShellRefusalIneligible
	endInstanceGone = apps.ShellRefusalInstanceGone
	endTimeout      = "timeout"
	endUnverified   = "unverified"
	// A Terminal's wait for an agent session's sandbox ended without one
	// (w10/m615): the start ended with none, or its ceiling passed.
	endNoSandbox   = "no_sandbox"
	endWakeTimeout = "wake_timeout"
)

var endMessages = map[string]string{
	endRevoked:      "your access to this service ended",
	endNoSandbox:    "the sandbox did not start; open the Terminal again to try again",
	endWakeTimeout:  "the sandbox did not start in time; open the Terminal again to try again",
	endSuspended:    "the service was suspended, which ended this session; resume it to open a new one",
	endIneligible:   "the service no longer has shell access (its plan or type changed), which ended this session",
	endInstanceGone: "the instance this session ran on stopped",
	endTimeout:      "the session reached its time limit",
	endUnverified:   "your access couldn't be re-checked, so the session ended; reconnect to try again",
}

// watchdogEndReason classifies the error the revalidation watchdog ended a
// session on. A runtime-gate refusal is named: ResolveSSHSession reaches those
// only after the caller passed a fresh authorization, so the caller's own
// GET /v1/services/{id} shows the same fact. An authorization failure is only
// "your access ended", never why (a removed member, a role change, a newly
// protected environment and a deleted service all read the same). Anything
// else is the resolver failing closed on an unreachable store.
func watchdogEndReason(err error) string {
	var refusal *apps.ShellRefusal
	switch {
	case errors.As(err, &refusal):
		return refusal.Reason
	case errors.Is(err, core.ErrForbidden), errors.Is(err, core.ErrNotFound):
		return endRevoked
	default:
		return endUnverified
	}
}

// keyGrantRefusal is what the terminal shows when a key's grant refuses its
// shell at connect: the refusal itself when the grant no longer covers it,
// else that it could not be checked.
func keyGrantRefusal(err error, target string) string {
	if errors.Is(err, core.ErrForbidden) {
		return "this API key is no longer granted shell access to this " + target
	}
	return "this API key's permissions couldn't be checked; reconnect to try again"
}

// Enabled reports whether the Browser Web Shell transport is configured.
func (s *Server) Enabled() bool { return len(s.TicketSecret) > 0 }

// Handler returns the HTTP handler for the Browser Web Shell. Mount it on the
// gateway's browser-facing listener (BEX_SHELL_WS_ADDR). It refuses every
// request when the transport is disabled.
func (s *Server) Handler() http.Handler {
	s.defaults()
	return http.HandlerFunc(s.serveWebSocket)
}

func (s *Server) serveWebSocket(w http.ResponseWriter, r *http.Request) {
	if !s.Enabled() {
		http.Error(w, "web shell not configured", http.StatusServiceUnavailable)
		return
	}
	// Ticket verification happens BEFORE the upgrade so a bad or replayed ticket
	// is a clean HTTP 401 (the dashboard re-mints). Everything reachable only
	// after a valid ticket — target resolution, session caps, the stream itself
	// — happens after the upgrade so its outcome reaches the terminal as a
	// readable control frame rather than an unreadable failed handshake.
	token := wsTicket(r)
	if token == "" {
		s.Metrics.Authentication("rejected_key")
		http.Error(w, "missing ticket", http.StatusUnauthorized)
		return
	}
	claims, err := shellticket.Verify(s.TicketSecret, token, time.Now())
	if err != nil {
		s.Metrics.Authentication("rejected_key")
		http.Error(w, "invalid ticket", http.StatusUnauthorized)
		return
	}
	if !s.Nonces.Consume(r.Context(), claims.Nonce, claims.NonceExpiry(), time.Now()) {
		s.Metrics.Authentication("rejected_key")
		http.Error(w, "ticket already used", http.StatusUnauthorized)
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote the error response.
	}
	defer conn.Close()
	clientIP := s.TrustedProxies.ClientIP(r)
	remoteAddress := r.RemoteAddr
	if clientIP != core.PeerIP(r.RemoteAddr) {
		remoteAddress = clientIP
	}
	// Where the browser connected from, for the workspace audit log's shell
	// rows and any refusal the resolver records (w9/m451).
	origin := core.RequestOrigin{SourceIP: clientIP, UserAgent: r.UserAgent()}
	s.runWebSocketSession(r.Context(), &wsConn{conn: conn}, claims, remoteAddress, origin)
}

// runWebSocketSession resolves the target, enforces caps, and bridges the
// browser to one pods/exec stream. Post-upgrade failures are delivered as an
// {type:"error"} control frame so the terminal can display them. An agent
// session's sandbox that is starting is waited for first (awaitSandbox).
func (s *Server) runWebSocketSession(ctx context.Context, ws *wsConn, claims shellticket.Claims, remoteAddress string, origin core.RequestOrigin) {
	ctx = core.WithRequestOrigin(ctx, origin)
	t := s.ticketTarget(claims)
	resolveCtx, cancelResolve := context.WithTimeout(t.as(ctx), s.HandshakeTimeout)
	target, err := t.resolve(resolveCtx)
	starting := false
	if err != nil {
		switch t.sandboxState(err) {
		case agentsessions.SSHSandboxStarting:
			starting = true
		case agentsessions.SSHSandboxClosed:
			// The start bex-api's mint began ended without a sandbox.
			cancelResolve()
			s.Metrics.Authentication("rejected_target")
			s.Metrics.WebShellWake(wakeNoSandbox)
			t.end(ws, endNoSandbox)
			return
		default:
			cancelResolve()
			log.Printf("web shell target resolution failed: %v", err)
			s.Metrics.Authentication("rejected_target")
			ws.fail(t.refusal(err))
			return
		}
	}
	// A key's shell also needs the key's grant, as its mint did (w9/125).
	err = t.keyGrant(resolveCtx)
	cancelResolve()
	if err != nil {
		s.Metrics.Authentication("rejected_target")
		ws.fail(keyGrantRefusal(err, t.noun()))
		return
	}

	acquired, limitScope := s.Limits.Acquire(claims.Subject)
	if !acquired {
		s.Metrics.LimitRejected(limitScope)
		ws.fail("session limit reached; close another shell and try again")
		return
	}
	defer s.Limits.Release(claims.Subject)
	s.Metrics.Authentication("accepted")
	if t.session && s.SessionTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeoutCause(ctx, s.SessionTimeout, errSessionCap)
		defer cancel()
	}
	// One reader takes the browser's frames for the WebSocket's whole life
	// (a gorilla connection allows one). A Terminal on a sandbox that is
	// starting holds the WebSocket until it is up, so a closed tab ends the
	// wait (w10/m615).
	in := startBrowserInput(ctx, ws)
	if !starting {
		in.openStdin()
	} else if target, err = s.awaitSandbox(ctx, ws, in, t); err != nil {
		t.endWait(ws, err)
		return
	}

	sessionID := ids.New(ids.SSHSession)
	started := time.Now().UTC()
	s.Metrics.SessionStarted()
	result := "closed"
	defer func() { s.Metrics.SessionEnded(result, time.Since(started)) }()
	workspaceID := target.OwnerID
	if workspaceID == "" {
		workspaceID = core.DefaultTenant
	}
	auditCtx, cancelAudit := context.WithTimeout(context.Background(), 2*time.Second)
	err = s.Store.StartSSHSession(auditCtx, store.SSHSessionAudit{
		ID: sessionID, Subject: claims.Subject, WorkspaceID: workspaceID,
		ServiceID: target.ServiceID, InstanceID: target.ID,
		RemoteAddress: remoteAddress, StartedAt: started,
		Transport: store.SSHTransportWebShell,
	})
	cancelAudit()
	if err != nil {
		result = "failed"
		ws.fail("unable to start session")
		return
	}
	defer func() {
		endCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.Store.EndSSHSession(endCtx, sessionID, result, time.Now().UTC()); err != nil {
			log.Printf("web shell session audit end failed: %v", err)
		}
	}()
	// The workspace audit log's shell rows (w9/m451), with the browser's user
	// agent; the end row repeats the start row's metadata.
	shell := sshgateway.ShellSession{
		ID: sessionID, Subject: claims.Subject, Method: core.MethodShell, Target: target,
		SourceIP: origin.SourceIP, UserAgent: origin.UserAgent,
	}
	sshgateway.RecordShellEvent(s.Audit, core.AuditVerbStartShell, shell, started)
	defer func() { sshgateway.RecordShellEvent(s.Audit, core.AuditVerbEndShell, shell, time.Now().UTC()) }()

	// codex round-9 #6: keep re-running the ticket-time resolution + fresh
	// authorization while the shell is LIVE — a revocation cancels the exec
	// context (the bridge and both pump goroutines end with it) instead of
	// waiting for the browser to disconnect. The watchdog owns execCtx
	// exclusively (the bridge pumps cancel only bridgeCtx), so a canceled
	// execCtx under a live request context is unambiguously revoked.
	execCtx, cancelExec := sshgateway.WithRevalidation(ctx, s.RevalidateInterval, func(c context.Context) error {
		if _, err := t.resolve(t.as(c)); err != nil {
			return err
		}
		return t.keyGrant(c)
	})
	defer cancelExec()
	bridgeCtx, cancelBridge := context.WithCancel(execCtx)
	defer cancelBridge()
	err = bridgeShell(bridgeCtx, cancelBridge, ws, s.Executor, target, in)
	result = "failed"
	switch {
	case err == nil:
		result = "completed"
	case execCtx.Err() != nil && ctx.Err() == nil:
		result = "revoked" // the watchdog ended the stream, not the client
		t.end(ws, t.endReason(context.Cause(execCtx)))
	case timeLimited(context.Cause(ctx)):
		// A terminating gateway's drain bound passed (w6/m482), or an agent
		// session's Terminal reached its cap: a time limit ended the session.
		t.end(ws, endTimeout)
	case errors.Is(err, sshgateway.ErrTargetTerminated), errors.Is(err, sshgateway.ErrTargetNotReady):
		// The attached pod stopped under the stream: a restart, a deploy's old
		// instance going away once the new release is live, an eviction. The
		// stream never moves to another replica (ADR035 § Fail-closed
		// behavior); the end is named instead.
		t.end(ws, endInstanceGone)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		ws.fail("session closed") // the browser went away
	default:
		ws.fail("unable to start a shell in this image")
	}
}

// pumpBrowserInput forwards the browser's frames to the shell: binary frames are
// stdin bytes, text frames are resize control messages. It returns on any read
// error — a closed browser tab — which the caller watches for to tear the
// session down. Each frame also re-arms the read deadline the pong handler
// maintains, so a live session is never reaped as silent.
func pumpBrowserInput(ws *wsConn, stdin io.Writer, sizes *heldSizes) {
	ws.conn.SetReadLimit(wsReadLimit)
	_ = ws.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	ws.conn.SetPongHandler(func(string) error {
		return ws.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})
	for {
		mt, data, err := ws.conn.ReadMessage()
		if err != nil {
			return
		}
		_ = ws.conn.SetReadDeadline(time.Now().Add(wsPongWait))
		switch mt {
		case websocket.BinaryMessage:
			if _, err := stdin.Write(data); err != nil {
				return
			}
		case websocket.TextMessage:
			var ctrl clientControl
			if json.Unmarshal(data, &ctrl) == nil && ctrl.Type == "resize" && sshgateway.ValidTerminalSize(uint32(ctrl.Cols), uint32(ctrl.Rows)) {
				sizes.push(remotecommand.TerminalSize{Width: ctrl.Cols, Height: ctrl.Rows})
			}
		}
	}
}

// pumpKeepalive pings the browser on a timer so a silently dead connection is
// detected and the session closed rather than pinned open.
// A failed ping closes the connection, which ends the browser's frames and so
// the session.
func pumpKeepalive(ctx context.Context, ws *wsConn) {
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := ws.ping(); err != nil {
				_ = ws.conn.Close()
				return
			}
		}
	}
}

// bridgeShell runs one interactive /bin/sh in the target and pumps it over the
// WebSocket. It returns nil when the shell ran to completion (any exit code),
// after sending the exit frame, and otherwise the error that stopped it, which
// the caller classifies into the terminal's error frame. The browser leaving
// (its frames stopping) cancels the shell.
func bridgeShell(ctx context.Context, cancel context.CancelFunc, ws *wsConn, executor sshgateway.Executor, target apps.SSHInstanceTarget, in *browserInput) error {
	stdin, sizes := in.open(ctx)
	go func() {
		select {
		case <-in.done:
			cancel()
		case <-ctx.Done():
		}
	}()

	// One WebSocket maps to one interactive /bin/sh with a TTY. stderr is nil for
	// a TTY (Kubernetes merges it into stdout), matching the native SSH shell path.
	code, err := executor.Execute(ctx, target, []string{"/bin/sh"}, true, sizes, stdin, ws, nil)
	_ = stdin.Close()
	if err != nil {
		return err
	}
	_ = ws.control(serverControl{Type: "exit", Code: sshgateway.ClampExit(code)})
	ws.close(websocket.CloseNormalClosure, "")
	// Let the browser answer the close before the handler drops the TCP
	// connection (w6/102): closing with its frames unread resets the
	// connection and drops output still in flight, as native SSH did (w6/096).
	select {
	case <-in.done:
	case <-time.After(wsCloseGrace):
	}
	return nil
}

// wsCloseGrace bounds the wait for the browser's close reply.
const wsCloseGrace = time.Second

// wsConn serializes all writes to one gorilla connection (stdout frames, control
// frames, and keepalive pings can otherwise race).
type wsConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// Write sends p as one binary frame — the io.Writer the exec stream copies
// stdout into.
func (w *wsConn) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	if err := w.conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *wsConn) control(v serverControl) error {
	data, _ := json.Marshal(v)
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	return w.conn.WriteMessage(websocket.TextMessage, data)
}

func (w *wsConn) ping() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	return w.conn.WriteMessage(websocket.PingMessage, nil)
}

// fail delivers a bounded error message to the terminal and closes.
func (w *wsConn) fail(message string) {
	_ = w.control(serverControl{Type: "error", Message: message})
	w.close(websocket.CloseNormalClosure, "")
}

// end tells the terminal why the server ended its live session, then closes.
func (w *wsConn) end(reason string) { w.endWith(reason, endMessages[reason]) }

// endWith is end with message as the reason's English fallback.
func (w *wsConn) endWith(reason, message string) {
	_ = w.control(serverControl{Type: "error", Reason: reason, Message: message})
	w.close(websocket.CloseNormalClosure, "")
}

func (w *wsConn) close(code int, text string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, text), time.Now().Add(wsWriteWait))
}

var _ io.Writer = (*wsConn)(nil)
