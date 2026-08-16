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
// AuthorizeApp(can_operate). The gateway verifies the ticket, re-runs
// ResolveSSHSession (so it re-authorizes and re-selects a Ready pod itself —
// bex-api's decision is not trusted transitively), and bridges the browser to
// one pods/exec stream. pods/exec stays confined to the gateway process;
// bex-api never gains it. The stream is never logged or persisted, and the
// audit row carries no terminal content — the same boundary as native SSH.
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

	Store    Store
	Apps     sshgateway.TargetResolver
	Executor sshgateway.Executor
	Metrics  *sshgateway.Metrics
	Limits   *sshgateway.SessionLimiter
	Nonces   *sshgateway.NonceGuard

	HandshakeTimeout time.Duration
	SessionTimeout   time.Duration

	// RevalidateInterval is how often an ESTABLISHED shell bridge re-runs the
	// target resolution + fresh can_operate authorization the upgrade path
	// performs (codex round-9 #6): a revocation mid-shell cancels the exec
	// context instead of waiting for the browser to disconnect or the 4h cap.
	// 0 => the platform default; negative disables (pre-round-9 behavior).
	RevalidateInterval time.Duration
}

func (s *Server) defaults() {
	if s.HandshakeTimeout <= 0 {
		s.HandshakeTimeout = 10 * time.Second
	}
	if s.SessionTimeout <= 0 {
		s.SessionTimeout = 4 * time.Hour
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
	Type    string `json:"type"`              // "exit" | "error"
	Code    int    `json:"code,omitempty"`    // for type=exit
	Message string `json:"message,omitempty"` // for type=error
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
	s.runWebSocketSession(r.Context(), &wsConn{conn: conn}, claims)
}

// runWebSocketSession resolves the target, enforces caps, and bridges the
// browser to one pods/exec stream. Post-upgrade failures are delivered as an
// {type:"error"} control frame so the terminal can display them.
func (s *Server) runWebSocketSession(ctx context.Context, ws *wsConn, claims shellticket.Claims) {
	resolveCtx := core.WithIdentity(ctx, core.Identity{Subject: claims.Subject, Method: "shell"})
	resolveCtx, cancelResolve := context.WithTimeout(resolveCtx, s.HandshakeTimeout)
	target, err := s.Apps.ResolveSSHSession(resolveCtx, claims.Username())
	cancelResolve()
	if err != nil {
		log.Printf("web shell target resolution failed: %v", err)
		s.Metrics.Authentication("rejected_target")
		ws.fail("this service has no instance available for a shell right now")
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
		RemoteAddress: ws.conn.RemoteAddr().String(), StartedAt: started,
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

	// codex round-9 #6: keep re-running the ticket-time resolution + fresh
	// authorization while the shell is LIVE — a revocation cancels the exec
	// context (the bridge and both pump goroutines end with it) instead of
	// waiting for the browser to disconnect or the session cap. The watchdog
	// owns execCtx exclusively (the bridge pumps cancel only bridgeCtx), so a
	// canceled execCtx under a live request context is unambiguously revoked.
	timedCtx, cancelTimeout := context.WithTimeout(ctx, s.SessionTimeout)
	defer cancelTimeout()
	execCtx, cancelExec := sshgateway.WithRevalidation(timedCtx, s.RevalidateInterval, func(c context.Context) error {
		resolveCtx := core.WithIdentity(c, core.Identity{Subject: claims.Subject, Method: "shell"})
		_, err := s.Apps.ResolveSSHSession(resolveCtx, claims.Username())
		return err
	})
	defer cancelExec()
	bridgeCtx, cancelBridge := context.WithCancel(execCtx)
	defer cancelBridge()
	completed := bridgeShell(bridgeCtx, cancelBridge, ws, s.Executor, target)
	switch {
	case execCtx.Err() != nil && timedCtx.Err() == nil:
		result = "revoked" // the watchdog ended the stream, not the client
	case completed:
		result = "completed"
	default:
		result = "failed"
	}
}

// bridgeShell runs one interactive /bin/sh in the target and pumps it over the
// WebSocket. It returns true when the shell ran to completion (any exit code),
// false when it could not start or the context ended. The terminal always
// receives an exit or error control frame.
// pumpBrowserInput forwards the browser's frames to the shell: binary frames are
// stdin bytes, text frames are resize control messages. Any read error ends the
// exec stream via cancel — that is how a closed browser tab tears the session
// down. Each frame also re-arms the read deadline the pong handler maintains, so
// a live session is never reaped as silent.
func pumpBrowserInput(ws *wsConn, pw *io.PipeWriter, sizes *sshgateway.SizeQueue, cancel context.CancelFunc) {
	defer func() { _ = pw.Close() }()
	ws.conn.SetReadLimit(wsReadLimit)
	_ = ws.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	ws.conn.SetPongHandler(func(string) error {
		return ws.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})
	for {
		mt, data, err := ws.conn.ReadMessage()
		if err != nil {
			cancel()
			return
		}
		_ = ws.conn.SetReadDeadline(time.Now().Add(wsPongWait))
		switch mt {
		case websocket.BinaryMessage:
			if _, err := pw.Write(data); err != nil {
				return
			}
		case websocket.TextMessage:
			var ctrl clientControl
			if json.Unmarshal(data, &ctrl) == nil && ctrl.Type == "resize" && sshgateway.ValidTerminalSize(uint32(ctrl.Cols), uint32(ctrl.Rows)) {
				sizes.Push(remotecommand.TerminalSize{Width: ctrl.Cols, Height: ctrl.Rows})
			}
		}
	}
}

// pumpKeepalive pings the browser on a timer so a silently dead connection is
// detected and the session closed rather than pinned open.
func pumpKeepalive(ctx context.Context, ws *wsConn, cancel context.CancelFunc) {
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := ws.ping(); err != nil {
				cancel()
				return
			}
		}
	}
}

func bridgeShell(ctx context.Context, cancel context.CancelFunc, ws *wsConn, executor sshgateway.Executor, target apps.SSHInstanceTarget) bool {
	pr, pw := io.Pipe()
	sizes := sshgateway.NewSizeQueue(ctx, &remotecommand.TerminalSize{Width: 80, Height: 24})

	go pumpBrowserInput(ws, pw, sizes, cancel)

	pingCtx, stopPing := context.WithCancel(ctx)
	defer stopPing()
	go pumpKeepalive(pingCtx, ws, cancel)

	// One WebSocket maps to one interactive /bin/sh with a TTY. stderr is nil for
	// a TTY (Kubernetes merges it into stdout), matching the native SSH shell path.
	code, err := executor.Execute(ctx, target, []string{"/bin/sh"}, true, sizes, pr, ws, nil)
	_ = pr.Close()
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			ws.fail("session closed")
			return false
		}
		ws.fail("unable to start a shell in this image")
		return false
	}
	_ = ws.control(serverControl{Type: "exit", Code: sshgateway.ClampExit(code)})
	ws.close(websocket.CloseNormalClosure, "")
	return true
}

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

func (w *wsConn) close(code int, text string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, text), time.Now().Add(wsWriteWait))
}

var _ io.Writer = (*wsConn)(nil)
