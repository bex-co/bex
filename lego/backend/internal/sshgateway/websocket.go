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

package sshgateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/shellticket"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The Browser Web Shell transport (docs/ADR035-ssh.md § Browser Web Shell). A
// browser opens a WebSocket to the isolated gateway carrying the exec ticket
// bex-api minted after AuthorizeApp(can_operate). The gateway verifies the
// ticket, re-runs ResolveSSHSession (so it re-authorizes and re-selects a Ready
// pod itself — bex-api's decision is not trusted transitively), and bridges the
// browser to one pods/exec stream. pods/exec stays confined to this process;
// bex-api never gains it. The stream is never logged or persisted, and the
// audit row carries no terminal content — the same boundary as native SSH.

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = 50 * time.Second
	wsReadLimit  = 1 << 20 // bound a single inbound frame (paste), not the session
)

// wsUpgrader upgrades the shell connection. Origin is not checked: the exec
// ticket (an unforgeable, short-lived HMAC token that only an authenticated
// bex-api caller can obtain) is the credential, not the browser origin.
var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
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

// WebSocketEnabled reports whether the Browser Web Shell transport is configured.
func (s *Server) WebSocketEnabled() bool { return len(s.TicketSecret) > 0 }

// WebSocketHandler returns the HTTP handler for the Browser Web Shell. Mount it
// on the gateway's browser-facing listener (BEX_SHELL_WS_ADDR). It refuses every
// request when the transport is disabled.
func (s *Server) WebSocketHandler() http.Handler {
	s.defaults()
	return http.HandlerFunc(s.serveWebSocket)
}

func (s *Server) serveWebSocket(w http.ResponseWriter, r *http.Request) {
	if !s.WebSocketEnabled() {
		http.Error(w, "web shell not configured", http.StatusServiceUnavailable)
		return
	}
	// Ticket verification happens BEFORE the upgrade so a bad or replayed ticket
	// is a clean HTTP 401 (the dashboard re-mints). Everything reachable only
	// after a valid ticket — target resolution, session caps, the stream itself
	// — happens after the upgrade so its outcome reaches the terminal as a
	// readable control frame rather than an unreadable failed handshake.
	token := r.URL.Query().Get("ticket")
	if token == "" {
		s.Metrics.authentication("rejected_key")
		http.Error(w, "missing ticket", http.StatusUnauthorized)
		return
	}
	claims, err := shellticket.Verify(s.TicketSecret, token, time.Now())
	if err != nil {
		s.Metrics.authentication("rejected_key")
		http.Error(w, "invalid ticket", http.StatusUnauthorized)
		return
	}
	if !s.consumeTicket(claims, time.Now()) {
		s.Metrics.authentication("rejected_key")
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
		s.Metrics.authentication("rejected_target")
		ws.fail("this service has no instance available for a shell right now")
		return
	}

	acquired, limitScope := s.acquire(claims.Subject)
	if !acquired {
		s.Metrics.limitRejected(limitScope)
		ws.fail("session limit reached; close another shell and try again")
		return
	}
	defer s.release(claims.Subject)
	s.Metrics.authentication("accepted")

	sessionID := ids.New(ids.SSHSession)
	started := time.Now().UTC()
	s.Metrics.sessionStarted()
	result := "closed"
	defer func() { s.Metrics.sessionEnded(result, time.Since(started)) }()
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

	execCtx, cancel := context.WithTimeout(ctx, s.SessionTimeout)
	defer cancel()
	if bridgeShell(execCtx, cancel, ws, s.Executor, target) {
		result = "completed"
	} else {
		result = "failed"
	}
}

// bridgeShell runs one interactive /bin/sh in the target and pumps it over the
// WebSocket. It returns true when the shell ran to completion (any exit code),
// false when it could not start or the context ended. The terminal always
// receives an exit or error control frame.
func bridgeShell(ctx context.Context, cancel context.CancelFunc, ws *wsConn, executor Executor, target apps.SSHInstanceTarget) bool {
	pr, pw := io.Pipe()
	sizes := newSizeQueue(ctx, &remotecommand.TerminalSize{Width: 80, Height: 24})

	// Reader goroutine: browser → stdin / resize. Cancelling ends the exec stream.
	go func() {
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
				if json.Unmarshal(data, &ctrl) == nil && ctrl.Type == "resize" && validTerminalSize(uint32(ctrl.Cols), uint32(ctrl.Rows)) {
					sizes.push(remotecommand.TerminalSize{Width: ctrl.Cols, Height: ctrl.Rows})
				}
			}
		}
	}()

	// Keepalive pings so a silent connection is detected and closed.
	pingCtx, stopPing := context.WithCancel(ctx)
	defer stopPing()
	go func() {
		ticker := time.NewTicker(wsPingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-pingCtx.Done():
				return
			case <-ticker.C:
				if err := ws.ping(); err != nil {
					cancel()
					return
				}
			}
		}
	}()

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
	_ = ws.control(serverControl{Type: "exit", Code: clampExit(code)})
	ws.close(websocket.CloseNormalClosure, "")
	return true
}

// clampExit bounds a Kubernetes exit code to a byte, matching runExec.
func clampExit(code int) int {
	if code < 0 {
		return 255
	}
	if code > 255 {
		return 255
	}
	return code
}

// consumeTicket records a ticket nonce so the same ticket cannot open a second
// stream on this replica, and opportunistically drops expired nonces.
func (s *Server) consumeTicket(claims shellticket.Claims, now time.Time) bool {
	s.usedMu.Lock()
	defer s.usedMu.Unlock()
	if s.usedNonces == nil {
		s.usedNonces = map[string]time.Time{}
	}
	for nonce, exp := range s.usedNonces {
		if now.After(exp) {
			delete(s.usedNonces, nonce)
		}
	}
	if _, seen := s.usedNonces[claims.Nonce]; seen {
		return false
	}
	s.usedNonces[claims.Nonce] = time.Unix(claims.ExpiresAt, 0)
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
