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
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The agent-session conversation transport (ADR047 D9, w3/m43). bex-api
// authorizes can_operate on the session and mints a signed agent-session ticket
// (agentsessionticket, the web-shell nonce pattern) binding the exact
// pod/namespace/session; the browser's Vercel AI SDK client (useChat) connects
// to THIS endpoint with that ticket. The gateway verifies + single-use-claims
// the ticket, then:
//
//   - GET  /v1/agent-sessions/{id}/stream — replays the durable transcript, then
//     splices the live driver stream, teeing new parts into the store. A terminal
//     session (its pod gone) replays the stored transcript and closes with
//     `[DONE]`. This replay mode IS the transcript-read API.
//   - POST /v1/agent-sessions/{id}/stream — forwards a live prompt turn to the
//     driver and streams the turn's parts back (w3/m43 t004).
//
// Byte-transparent forwarding is a contract, not a convenience (ADR047 D3): the
// gateway preserves the `x-vercel-ai-ui-message-stream: v1` marker and the exact
// `data:` payload bytes end to end — it never re-encodes, filters, reorders, or
// injects stream parts. Its only additions are authentication, the transcript
// tee, and replay-then-live splicing. Cookies on this path are ignored: the
// ticket is the sole credential (the endpoint publishes under the api.bex.co
// origin via edge path-routing, so a Kratos cookie will arrive).

// AgentAttachTicketHeader carries the signed agent-session ticket. It is a
// header, never a URL parameter, so the credential stays out of proxy/access
// logs (the web-shell subprotocol precedent, applied to plain-HTTP SSE).
const AgentAttachTicketHeader = "X-Bex-Agent-Ticket"

const defaultDriverPort = 8787

const uiMessageStreamHeader = "x-vercel-ai-ui-message-stream"

// maxAgentTurnRequest bounds a live prompt-turn POST body forwarded to the
// driver (the AI SDK sendMessages payload). The session task cap is 100 KB
// (agentsessions.validateCreate); 256 KB leaves headroom for message framing.
const maxAgentTurnRequest = 256 << 10

// AgentAttachStore is the gateway's narrow transcript authority: read the
// durable transcript for replay, append teed parts idempotently, and claim the
// ticket nonce for single use across replicas. It reads/writes only
// agent_session_transcripts and shell_ticket_nonces — no session-row access is
// needed (a terminal session is detected by the driver being unreachable), so
// the least-privilege bex_ssh_gateway role gains only those two grants.
type AgentAttachStore interface {
	AgentSessionTranscript(ctx context.Context, sessionID string, afterSeq int64) ([]store.AgentSessionTranscriptPart, error)
	AgentSessionTranscriptMaxSeq(ctx context.Context, sessionID string) (int64, bool, error)
	AppendAgentSessionTranscript(ctx context.Context, sessionID string, parts []store.AgentSessionTranscriptPart) error
	ClaimShellNonce(ctx context.Context, nonce string, expiresAt time.Time) (bool, error)
}

// PodIPResolver maps a claimed pod (namespace + name from the verified ticket)
// to its current IP so the gateway can dial the in-sandbox driver directly. The
// sandbox NetworkPolicy admits ingress only from the gateway, so this direct
// dial — not pods/exec — is the sanctioned path to the driver's stream port.
type PodIPResolver interface {
	PodIP(ctx context.Context, namespace, podName string) (string, error)
}

type KubePodIPResolver struct{ Client kubernetes.Interface }

func (r KubePodIPResolver) PodIP(ctx context.Context, namespace, podName string) (string, error) {
	if r.Client == nil || namespace == "" || podName == "" {
		return "", fmt.Errorf("pod ip resolver: missing client/namespace/pod")
	}
	pod, err := r.Client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if pod.DeletionTimestamp != nil || pod.Status.PodIP == "" {
		return "", fmt.Errorf("pod %s/%s has no live IP", namespace, podName)
	}
	return pod.Status.PodIP, nil
}

// AgentAttachConfig enables the agent-session conversation transport. Secret
// must equal bex-api's agent-session ticket secret (BEX_SHELL_TICKET_SECRET);
// empty => the listener is not started (feature off, byte-identical default).
type AgentAttachConfig struct {
	Store      AgentAttachStore
	Pods       PodIPResolver
	Secret     []byte
	DriverPort int
	HTTPClient *http.Client
	Now        func() time.Time
	// AllowedOrigins is the browser origin allowlist for CORS. The endpoint
	// publishes under the api origin but is consumed cross-subdomain by the
	// dashboard (dashboard.bex.co -> api.bex.co), so the gateway must echo the
	// matched Origin and expose the v1 marker or the browser blocks the stream.
	// Reuse bex-api's BEX_API_CORS_ORIGIN value. Empty => no CORS headers
	// (same-origin / curl only).
	AllowedOrigins []string
}

func (s *Server) AgentAttachEnabled() bool {
	return s.AgentAttach != nil && len(s.AgentAttach.Secret) > 0 &&
		s.AgentAttach.Store != nil && s.AgentAttach.Pods != nil
}

// AgentAttachHandler serves both GET (attach: replay + live) and POST (live
// prompt turn). Mount it on the api-origin path the edge routes to the gateway.
func (s *Server) AgentAttachHandler() http.Handler {
	s.defaults()
	return http.HandlerFunc(s.serveAgentAttach)
}

func (c *AgentAttachConfig) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *AgentAttachConfig) driverPort() int {
	if c.DriverPort > 0 {
		return c.DriverPort
	}
	return defaultDriverPort
}

func (c *AgentAttachConfig) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	// No client-side timeout: the request context (bounded by SessionTimeout)
	// governs the long-lived stream; a Client.Timeout would truncate it.
	return &http.Client{}
}

func (s *Server) serveAgentAttach(w http.ResponseWriter, r *http.Request) {
	if !s.AgentAttachEnabled() {
		http.Error(w, "agent session attach not configured", http.StatusServiceUnavailable)
		return
	}
	// CORS + preflight run before the ticket check: a browser's preflight carries
	// no ticket, and the actual GET/POST response must expose the v1 marker to the
	// cross-origin dashboard or the stream is unreadable.
	s.applyAgentAttachCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	cfg := s.AgentAttach
	claims, err := agentsessionticket.Verify(cfg.Secret, r.Header.Get(AgentAttachTicketHeader), cfg.now())
	if err != nil {
		s.Metrics.authentication("rejected_key")
		http.Error(w, "invalid ticket", http.StatusUnauthorized)
		return
	}
	// The session id is bound in the ticket; a mismatched path id is a caller
	// error, not an authorization bypass (the ticket still governs), but reject
	// it so a client cannot believe it attached to a different session.
	if id := r.PathValue("id"); id != "" && id != claims.SessionID {
		http.Error(w, "ticket/session mismatch", http.StatusForbidden)
		return
	}
	if !s.consumeAgentAttachNonce(r.Context(), claims.Nonce, time.Unix(claims.ExpiresAt, 0), cfg.now()) {
		s.Metrics.authentication("rejected_key")
		http.Error(w, "ticket already used", http.StatusUnauthorized)
		return
	}
	if acquired, scope := s.acquire(claims.Subject); !acquired {
		s.Metrics.limitRejected(scope)
		http.Error(w, "session limit reached", http.StatusTooManyRequests)
		return
	}
	defer s.release(claims.Subject)
	s.Metrics.authentication("accepted")

	ctx, cancel := context.WithTimeout(r.Context(), s.SessionTimeout)
	defer cancel()

	podIP, ipErr := cfg.Pods.PodIP(ctx, claims.Namespace, claims.Pod)

	sse := s.startAgentSSE(w, flusher)

	if r.Method == http.MethodPost {
		if ipErr != nil {
			// A turn needs a live driver; a gone pod cannot accept one.
			sse.errorAndDone("session is not live")
			return
		}
		s.forwardAgentTurn(ctx, sse, cfg, podIP, claims.SessionID, r.Body)
		return
	}
	s.streamAgentAttach(ctx, sse, cfg, podIP, ipErr, claims.SessionID)
}

// streamAgentAttach replays the durable transcript to the client, then — if the
// driver is reachable — splices the live driver stream and tees new parts into
// the store. A terminal/gone session (ipErr) replays and closes with `[DONE]`.
func (s *Server) streamAgentAttach(ctx context.Context, sse *agentSSE, cfg *AgentAttachConfig, podIP string, ipErr error, sessionID string) {
	stored, err := cfg.Store.AgentSessionTranscript(ctx, sessionID, -1)
	if err != nil {
		log.Printf("agent attach: transcript replay failed (session=%s): %v", sessionID, err)
		sse.errorAndDone("transcript unavailable")
		return
	}
	var replayedMax int64 = -1
	for _, part := range stored {
		sse.frame(string(part.Part))
		replayedMax = part.Seq
	}
	if ipErr != nil {
		// No live driver: this is the terminal-session history path.
		sse.done()
		return
	}
	if err := s.spliceDriverStream(ctx, sse, cfg, podIP, sessionID, replayedMax); err != nil {
		log.Printf("agent attach: live splice ended (session=%s): %v", sessionID, err)
	}
	sse.done()
}

// spliceDriverStream reads the driver's /stream (which replays its full history
// then goes live), tees every part into the durable store keyed by the driver's
// emission ordinal, and forwards to the client only the parts beyond what replay
// already sent. The ordinal-keyed idempotent append means re-reading the
// driver's replayed history (across reconnects/replicas) never duplicates a
// stored part.
func (s *Server) spliceDriverStream(ctx context.Context, sse *agentSSE, cfg *AgentAttachConfig, podIP, sessionID string, replayedMax int64) error {
	url := fmt.Sprintf("http://%s/stream", net.JoinHostPort(podIP, strconv.Itoa(cfg.driverPort())))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := cfg.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var ordinal int64 = -1
	reader := bufio.NewReader(resp.Body)
	for {
		payload, done, err := readSSEData(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if done { // driver `data: [DONE]` sentinel
			return nil
		}
		if payload == "" {
			continue
		}
		ordinal++
		// Tee every part (idempotent on the driver-ordinal key); forward to the
		// client only the parts replay did not already deliver.
		if err := cfg.Store.AppendAgentSessionTranscript(ctx, sessionID, []store.AgentSessionTranscriptPart{
			{Seq: ordinal, Part: []byte(payload)},
		}); err != nil {
			log.Printf("agent attach: tee failed (session=%s seq=%d): %v", sessionID, ordinal, err)
		}
		if ordinal > replayedMax {
			sse.frame(payload)
		}
	}
}

// forwardAgentTurn posts a live prompt turn to the driver and streams the turn's
// parts back to the client, teeing them into the transcript after the current
// stored max so a concurrently attached GET client sees the turn too (w3/m43 t004).
func (s *Server) forwardAgentTurn(ctx context.Context, sse *agentSSE, cfg *AgentAttachConfig, podIP, sessionID string, body io.Reader) {
	base, _, err := cfg.Store.AgentSessionTranscriptMaxSeq(ctx, sessionID)
	if err != nil {
		sse.errorAndDone("transcript unavailable")
		return
	}
	url := fmt.Sprintf("http://%s/turn", net.JoinHostPort(podIP, strconv.Itoa(cfg.driverPort())))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, io.LimitReader(body, maxAgentTurnRequest))
	if err != nil {
		sse.errorAndDone("turn dispatch failed")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := cfg.httpClient().Do(req)
	if err != nil {
		sse.errorAndDone("turn dispatch failed")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusConflict {
		sse.errorAndDone("a turn is already running")
		return
	}
	ordinal := base
	reader := bufio.NewReader(resp.Body)
	for {
		payload, done, err := readSSEData(reader)
		if err != nil || done {
			break
		}
		if payload == "" {
			continue
		}
		ordinal++
		if err := cfg.Store.AppendAgentSessionTranscript(ctx, sessionID, []store.AgentSessionTranscriptPart{
			{Seq: ordinal, Part: []byte(payload)},
		}); err != nil {
			log.Printf("agent turn: tee failed (session=%s seq=%d): %v", sessionID, ordinal, err)
		}
		sse.frame(payload)
	}
	sse.done()
}

// readSSEData reads one SSE `data:` payload from the driver framing. It returns
// the payload, whether it was the `[DONE]` sentinel, and io.EOF at stream end.
// The driver frames each part as a single `data: <json>\n\n` (JSON.stringify
// never emits raw newlines), so payloads are line-oriented.
func readSSEData(r *bufio.Reader) (string, bool, error) {
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF && strings.TrimSpace(line) == "" {
				return "", false, io.EOF
			}
			if err != io.EOF {
				return "", false, err
			}
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			if err == io.EOF {
				return "", false, io.EOF
			}
			continue // event separator
		}
		if payload, ok := strings.CutPrefix(trimmed, "data: "); ok {
			if payload == "[DONE]" {
				return "", true, nil
			}
			return payload, false, nil
		}
		// Ignore comment/heartbeat/other field lines (`:` keep-alives, `event:`).
		if err == io.EOF {
			return "", false, io.EOF
		}
	}
}

// applyAgentAttachCORS mirrors bex-api's withCORS (internal/api/auth.go): echo a
// matched Origin, allow the ticket + content-type request headers, expose the
// v1 marker to the reader, and allow credentials (the AI SDK transport may send
// credentials:'include'; the gateway still ignores cookies and trusts only the
// ticket). Empty allowlist => no CORS headers (curl / same-origin).
func (s *Server) applyAgentAttachCORS(w http.ResponseWriter, r *http.Request) {
	allowed := s.AgentAttach.AllowedOrigins
	if len(allowed) == 0 {
		return
	}
	w.Header().Add("Vary", "Origin")
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	for _, o := range allowed {
		if o == origin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", AgentAttachTicketHeader+", Content-Type")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Expose-Headers", uiMessageStreamHeader)
			return
		}
	}
}

// agentSSE serializes SSE writes to one client response and preserves the
// verbatim UI-message-stream framing.
type agentSSE struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func (s *Server) startAgentSSE(w http.ResponseWriter, flusher http.Flusher) *agentSSE {
	// Ignore any cookies on this path (the ticket is the sole credential) and
	// preserve the v1 marker end to end.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set(uiMessageStreamHeader, "v1")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return &agentSSE{w: w, flusher: flusher}
}

// frame writes one part verbatim as `data: <payload>\n\n` — byte-identical to
// the driver's own framing, so the client stream matches what the driver emitted.
func (a *agentSSE) frame(payload string) {
	_, _ = io.WriteString(a.w, "data: ")
	_, _ = io.WriteString(a.w, payload)
	_, _ = io.WriteString(a.w, "\n\n")
	a.flusher.Flush()
}

func (a *agentSSE) done() {
	_, _ = io.WriteString(a.w, "data: [DONE]\n\n")
	a.flusher.Flush()
}

// errorAndDone emits a UI-message `error` part then the terminator, so a Vercel
// AI SDK client surfaces the failure and settles rather than hanging.
func (a *agentSSE) errorAndDone(message string) {
	a.frame(fmt.Sprintf(`{"type":"error","errorText":%q}`, message))
	a.done()
}

// consumeAgentAttachNonce enforces single-use of an attach ticket across
// replicas, mirroring the sandbox-exec guard: an in-memory fast path plus the
// authoritative shared-store claim (a reconnect always mints a fresh ticket, so
// single-use never blocks a legitimate reattach).
func (s *Server) consumeAgentAttachNonce(ctx context.Context, nonce string, exp, now time.Time) bool {
	if nonce == "" {
		return false
	}
	s.usedMu.Lock()
	if s.usedNonces == nil {
		s.usedNonces = map[string]time.Time{}
	}
	for n, e := range s.usedNonces {
		if now.After(e) {
			delete(s.usedNonces, n)
		}
	}
	if _, seen := s.usedNonces[nonce]; seen {
		s.usedMu.Unlock()
		return false
	}
	s.usedNonces[nonce] = exp
	s.usedMu.Unlock()

	claimCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	claimed, err := s.AgentAttach.Store.ClaimShellNonce(claimCtx, nonce, exp)
	if err != nil {
		log.Printf("agent attach nonce claim failed: %v", err)
		return false
	}
	return claimed
}
