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

// Package agentattach is the agent-session conversation transport of the
// isolated SSH gateway (ADR047 D9, w3/m43). bex-api authorizes can_operate on
// the session and mints a signed agent-session ticket (agentsessionticket,
// the web-shell nonce pattern) binding the exact pod/namespace/session; the
// browser's Vercel AI SDK client (useChat) connects to THIS endpoint with
// that ticket. The gateway verifies + single-use-claims the ticket, then:
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
package agentattach

import (
	"bufio"
	"bytes"
	"context"
	"errors"
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
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TicketHeader carries the signed agent-session ticket. It is a header, never
// a URL parameter, so the credential stays out of proxy/access logs (the
// web-shell subprotocol precedent, applied to plain-HTTP SSE). Sourced from the
// ticket package so bex-api's recorder client and this transport share one name.
const TicketHeader = agentsessionticket.TicketHeader

const defaultDriverPort = 8787

const uiMessageStreamHeader = "x-vercel-ai-ui-message-stream"

// maxAgentTurnRequest bounds a live prompt-turn POST body forwarded to the
// driver (the AI SDK sendMessages payload). The session task cap is 100 KB
// (agentsessions.validateCreate); 256 KB leaves headroom for message framing.
const maxAgentTurnRequest = 256 << 10

// maxSSEPartBytes bounds a single serialized transcript part read from the
// driver (w1/m65 F10). A tenant-controlled provider part larger than this is
// refused rather than buffered without limit by the old ReadString('\n'), which
// could allocate the whole gateway's memory on one line. Matches the
// sandbox-exec per-line cap (1 MiB).
const maxSSEPartBytes = 1 << 20

// replayPageBytes bounds the peak memory one reader's transcript replay holds
// at once (F10 follow-up). The durable read is paged in these increments and
// each page is framed and dropped before the next is fetched, so a replica
// serving several concurrent replays never materializes multiples of the full
// per-session quota (up to 64 MiB) — only one page per reader. Comfortably above
// maxSSEPartBytes (1 MiB) so every part fits in a page and replay makes progress.
const replayPageBytes = 4 << 20

// defaultSSEWriteTimeout bounds a single SSE write to one client. A stalled
// reader that stops draining its socket would otherwise pin the handler — and
// with it the replay/page buffers and the shared session-limiter slot —
// indefinitely, so a write that cannot complete in this window fails the stream.
const defaultSSEWriteTimeout = 30 * time.Second

// maxSessionTranscriptBytes caps the total transcript bytes stored for one
// session and materialized on replay (w1/m65 F10), bounding both gateway
// memory and unbounded Postgres growth from tenant-controlled agent output.
// Every write path enforces the cap CUMULATIVELY: the live splice and the
// prompt-turn forwarder seed their byte counter from the session's
// already-stored transcript bytes, so the stored transcript can never exceed
// the cap regardless of turn count — and the replay read is itself budgeted
// to the same cap at the store. A stream that hits the cap is stopped and
// settled with `[DONE]`.
const maxSessionTranscriptBytes = store.MaxAgentSessionTranscriptBytes

// errPartTooLarge is returned by the bounded SSE reader when one part exceeds
// maxSSEPartBytes; the caller ends the stream rather than buffering it.
var errPartTooLarge = errors.New("agent attach: transcript part exceeds size limit")

// Store is the transport's narrow transcript authority: read the durable
// transcript for replay and append teed parts idempotently. It reads/writes
// only agent_session_transcripts — no session-row access is needed (a
// terminal session is detected by the driver being unreachable), so the
// least-privilege bex_ssh_gateway role gains only that grant. (The ticket
// nonce claim reaches shell_ticket_nonces through the injected
// sshgateway.NonceGuard.)
type Store interface {
	AgentSessionTranscript(ctx context.Context, sessionID string, afterSeq int64, maxBytes int64) ([]store.AgentSessionTranscriptPart, error)
	AgentSessionTranscriptMaxSeq(ctx context.Context, sessionID string) (int64, bool, error)
	AgentSessionTranscriptBytes(ctx context.Context, sessionID string) (int64, error)
	AppendAgentSessionTranscript(ctx context.Context, sessionID string, parts []store.AgentSessionTranscriptPart) error
}

// SessionRevalidator re-checks, at redemption time, that the ticket's subject
// still holds the mint-time relation on the session and (for turns) that the
// session is still live (codex-security round-6 #11). The signed single-use
// ticket alone freezes the mint-time decision for its TTL+skew window; this
// hook makes revocation and cancellation effective at the gateway too. An
// error refuses the attach. Implemented by agentsessions.AttachRevalidator.
type SessionRevalidator interface {
	RevalidateAttach(ctx context.Context, subject, sessionID, action string) error
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

// Server is the agent-session attach handler. Share Limits and Nonces (and
// Metrics) with the other gateway transports so session caps and ticket
// single-use hold process-wide, not per feature.
type Server struct {
	// Secret must equal bex-api's agent-session ticket secret
	// (BEX_SHELL_TICKET_SECRET); empty => the transport is disabled and every
	// request is refused with 503.
	Secret []byte

	Store      Store
	Pods       PodIPResolver
	DriverPort int
	HTTPClient *http.Client
	Now        func() time.Time
	// Revalidator, when set, re-authorizes every verified ticket at redemption
	// (round-6 #11); nil skips the re-check (unit tests exercising transport
	// behavior — cmd/ssh-gateway always wires it).
	Revalidator SessionRevalidator
	// AllowedOrigins is the browser origin allowlist for CORS. The endpoint
	// publishes under the api origin but is consumed cross-subdomain by the
	// dashboard (dashboard.bex.co -> api.bex.co), so the gateway must echo the
	// matched Origin and expose the v1 marker or the browser blocks the stream.
	// Reuse bex-api's BEX_API_CORS_ORIGIN value. Empty => no CORS headers
	// (same-origin / curl only).
	AllowedOrigins []string

	Metrics *sshgateway.Metrics
	Limits  *sshgateway.SessionLimiter
	Nonces  *sshgateway.NonceGuard

	SessionTimeout time.Duration

	// MaxTranscriptBytes overrides the per-session cumulative transcript quota
	// (maxSessionTranscriptBytes); zero => the platform default. Tests set it
	// small to exercise the quota without 64 MiB payloads.
	MaxTranscriptBytes int64

	// ReplayPageBytes overrides the per-page replay budget (replayPageBytes);
	// zero => the platform default. Tests set it small to exercise multi-page
	// replay without multi-megabyte payloads.
	ReplayPageBytes int64

	// WriteTimeout overrides the per-write SSE deadline (defaultSSEWriteTimeout);
	// zero => the platform default. Bounds how long one stalled client write may
	// block the handler before the stream is abandoned.
	WriteTimeout time.Duration
}

func (s *Server) defaults() {
	if s.SessionTimeout <= 0 {
		s.SessionTimeout = 4 * time.Hour
	}
	if s.Limits == nil {
		s.Limits = sshgateway.NewSessionLimiter(0, 0)
	}
	if s.Nonces == nil {
		s.Nonces = &sshgateway.NonceGuard{}
	}
}

// Enabled reports whether the agent-session attach transport is configured.
func (s *Server) Enabled() bool {
	return len(s.Secret) > 0 && s.Store != nil && s.Pods != nil
}

// Handler serves both GET (attach: replay + live) and POST (live prompt
// turn). Mount it on the api-origin path the edge routes to the gateway.
func (s *Server) Handler() http.Handler {
	s.defaults()
	return http.HandlerFunc(s.serveAgentAttach)
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Server) driverPort() int {
	if s.DriverPort > 0 {
		return s.DriverPort
	}
	return defaultDriverPort
}

// transcriptQuota is the per-session cumulative transcript byte cap enforced
// on every write path and on the replay read.
func (s *Server) transcriptQuota() int64 {
	if s.MaxTranscriptBytes > 0 {
		return s.MaxTranscriptBytes
	}
	return maxSessionTranscriptBytes
}

func (s *Server) replayPageQuota() int64 {
	if s.ReplayPageBytes > 0 {
		return s.ReplayPageBytes
	}
	return replayPageBytes
}

func (s *Server) writeTimeout() time.Duration {
	if s.WriteTimeout > 0 {
		return s.WriteTimeout
	}
	return defaultSSEWriteTimeout
}

func (s *Server) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	// No client-side timeout: the request context (bounded by SessionTimeout)
	// governs the long-lived stream; a Client.Timeout would truncate it.
	//
	// Never follow redirects (codex-security round-6 #2): the ticket binds the
	// exact sandbox pod, but a malicious in-sandbox driver could answer with a
	// 307/308 pointing at a PEER sandbox's driver — which this gateway identity
	// can reach cluster-wide — and Go's default client would re-send the turn
	// POST there. Surface the 3xx as-is; the call sites reject any non-2xx.
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (s *Server) serveAgentAttach(w http.ResponseWriter, r *http.Request) {
	if !s.Enabled() {
		http.Error(w, "agent session attach not configured", http.StatusServiceUnavailable)
		return
	}
	// CORS + preflight run before the ticket check: a browser's preflight carries
	// no ticket, and the actual GET/POST response must expose the v1 marker to the
	// cross-origin dashboard or the stream is unreadable.
	s.applyCORS(w, r)
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
	claims, err := agentsessionticket.Verify(s.Secret, r.Header.Get(TicketHeader), s.now())
	if err != nil {
		s.Metrics.Authentication("rejected_key")
		http.Error(w, "invalid ticket", http.StatusUnauthorized)
		return
	}
	// Verify that the ticket action matches the HTTP method.
	// GET (transcript replay) requires ActionRead, POST (live turn) requires ActionTurn.
	if r.Method == http.MethodGet && claims.Action != agentsessionticket.ActionRead {
		s.Metrics.Authentication("rejected_action")
		http.Error(w, "ticket action 'read' required for GET requests", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodPost && claims.Action != agentsessionticket.ActionTurn {
		s.Metrics.Authentication("rejected_action")
		http.Error(w, "ticket action 'turn' required for POST requests", http.StatusForbidden)
		return
	}
	// The session id is bound in the ticket; a mismatched path id is a caller
	// error, not an authorization bypass (the ticket still governs), but reject
	// it so a client cannot believe it attached to a different session.
	if id := r.PathValue("id"); id != "" && id != claims.SessionID {
		http.Error(w, "ticket/session mismatch", http.StatusForbidden)
		return
	}
	if !s.Nonces.Consume(r.Context(), claims.Nonce, claims.NonceExpiry(), s.now()) {
		s.Metrics.Authentication("rejected_key")
		http.Error(w, "ticket already used", http.StatusUnauthorized)
		return
	}
	// Redemption-time re-check (round-6 #11), after the nonce burn so a denied
	// ticket is also a spent one: the mint-time authorization/lifecycle decision
	// must still hold NOW — a member revoked (or a session canceled) inside the
	// ticket's TTL window is refused here instead of getting one last read/turn.
	if s.Revalidator != nil {
		if err := s.Revalidator.RevalidateAttach(r.Context(), claims.Subject, claims.SessionID, claims.Action); err != nil {
			s.Metrics.Authentication("rejected_authz")
			http.Error(w, "no longer authorized for this session", http.StatusForbidden)
			return
		}
	}
	if acquired, scope := s.Limits.Acquire(claims.Subject); !acquired {
		s.Metrics.LimitRejected(scope)
		http.Error(w, "session limit reached", http.StatusTooManyRequests)
		return
	}
	defer s.Limits.Release(claims.Subject)
	s.Metrics.Authentication("accepted")

	ctx, cancel := context.WithTimeout(r.Context(), s.SessionTimeout)
	defer cancel()

	podIP, ipErr := s.Pods.PodIP(ctx, claims.Namespace, claims.Pod)

	// A POST turn's body must be read BEFORE the SSE response starts: Go's http
	// server closes the request body the moment the handler writes a response
	// (ErrBodyReadAfterClose), so forwarding r.Body after startAgentSSE always
	// fails. The read stays bounded by maxAgentTurnRequest (silent truncation,
	// as before).
	var turnBody []byte
	if r.Method == http.MethodPost && ipErr == nil {
		tb, err := io.ReadAll(io.LimitReader(r.Body, maxAgentTurnRequest))
		if err != nil {
			http.Error(w, "turn body read failed", http.StatusBadRequest)
			return
		}
		turnBody = tb
	}

	sse := s.startAgentSSE(w, flusher)

	if r.Method == http.MethodPost {
		if ipErr != nil {
			// A turn needs a live driver; a gone pod cannot accept one.
			sse.errorAndDone("session is not live")
			return
		}
		s.forwardAgentTurn(ctx, sse, podIP, claims.SessionID, claims.Turn, bytes.NewReader(turnBody))
		return
	}
	s.streamAgentAttach(ctx, sse, podIP, ipErr, claims.SessionID, claims.Turn)
}

// streamAgentAttach replays the durable transcript to the client, then — if the
// driver is reachable — splices the live driver stream and tees new parts into
// the store. A terminal/gone session (ipErr) replays and closes with `[DONE]`.
func (s *Server) streamAgentAttach(ctx context.Context, sse *agentSSE, podIP string, ipErr error, sessionID string, turn int) {
	// Replay the durable transcript in bounded pages rather than materializing the
	// whole (up to 64 MiB) transcript at once (F10 follow-up). Peak memory per
	// reader is one page, not the full session quota, so concurrent replays can't
	// sum past the pod memory limit; and a stalled or disconnected client aborts
	// the replay — freeing the page buffer and the shared session-limiter slot —
	// instead of pinning them for the whole write.
	quota := s.transcriptQuota()
	pageBudget := s.replayPageQuota()
	var replayedMax int64 = -1
	var replayedBytes int64
	after := int64(-1)
replay:
	for {
		if ctx.Err() != nil {
			return
		}
		page, err := s.Store.AgentSessionTranscript(ctx, sessionID, after, pageBudget)
		if err != nil {
			log.Printf("agent attach: transcript replay failed (session=%s): %v", sessionID, err)
			sse.errorAndDone("transcript unavailable")
			return
		}
		if len(page) == 0 {
			break
		}
		for _, part := range page {
			// Cap the cumulative replay at the session quota (a stored transcript
			// that ever outgrew it is truncated rather than framed without limit).
			if replayedBytes+int64(len(part.Part)) > quota {
				log.Printf("agent attach: transcript replay truncated at cap (session=%s)", sessionID)
				break replay
			}
			if err := sse.frame(string(part.Part)); err != nil {
				// Client gone or stalled past the write deadline: stop now so the
				// deferred limiter release + this return free the reader's memory.
				return
			}
			replayedMax = part.Seq
			replayedBytes += int64(len(part.Part))
		}
		after = page[len(page)-1].Seq
	}
	if ipErr != nil {
		// No live driver: this is the terminal-session history path.
		sse.done()
		return
	}
	if err := s.spliceDriverStream(ctx, sse, podIP, sessionID, turn, replayedMax, replayedBytes); err != nil {
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
func (s *Server) spliceDriverStream(ctx context.Context, sse *agentSSE, podIP, sessionID string, turn int, replayedMax int64, storedBytes int64) error {
	resp, err := s.dialDriverStream(ctx, podIP)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var ordinal int64 = -1
	total := storedBytes // transcript bytes already persisted for this session
	reader := bufio.NewReader(resp.Body)
	for {
		payload, done, err := readSSEData(reader, maxSSEPartBytes)
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
		// Per-session byte quota (F10): stop teeing+streaming once the session's
		// stored transcript would exceed the cumulative cap, so tenant output
		// can't grow Postgres or the gateway without bound. The client is
		// settled with `[DONE]` by the caller.
		if total+int64(len(payload)) > s.transcriptQuota() {
			log.Printf("agent attach: session transcript byte quota reached, stopping splice (session=%s)", sessionID)
			return nil
		}
		total += int64(len(payload))
		ordinal++
		// Tee every part (idempotent on the driver-ordinal key); forward to the
		// client only the parts replay did not already deliver.
		if err := s.Store.AppendAgentSessionTranscript(ctx, sessionID, []store.AgentSessionTranscriptPart{
			{Seq: ordinal, Turn: turn, Part: []byte(payload)},
		}); err != nil {
			log.Printf("agent attach: tee failed (session=%s seq=%d): %v", sessionID, ordinal, err)
		}
		if ordinal > replayedMax {
			if err := sse.frame(payload); err != nil {
				// Client gone or stalled past the write deadline. The tee above
				// already persisted this part; stop forwarding (the Completer's
				// headless recorder captures the remainder at session end).
				return err
			}
		}
	}
}

// dialDriverStream opens the in-sandbox driver's GET /stream (which replays its
// full history then goes live). Shared by the browser live-splice and the
// headless recorder.
func (s *Server) dialDriverStream(ctx context.Context, podIP string) (*http.Response, error) {
	url := fmt.Sprintf("http://%s/stream", net.JoinHostPort(podIP, strconv.Itoa(s.driverPort())))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	// The pod named in the ticket is the ONLY endpoint this stream may read
	// from: a redirect (or any non-OK answer) from the tenant-controlled driver
	// must not be treated as a stream (codex-security round-6 #2).
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("driver stream returned status %d", resp.StatusCode)
	}
	return resp, nil
}

// forwardAgentTurn posts a live prompt turn to the driver and streams the turn's
// parts back to the client, teeing them into the transcript after the current
// stored max so a concurrently attached GET client sees the turn too (w3/m43 t004).
func (s *Server) forwardAgentTurn(ctx context.Context, sse *agentSSE, podIP, sessionID string, turn int, body io.Reader) {
	base, _, err := s.Store.AgentSessionTranscriptMaxSeq(ctx, sessionID)
	if err != nil {
		sse.errorAndDone("transcript unavailable")
		return
	}
	// The quota is CUMULATIVE (F10): seed this turn's byte counter from the
	// session's already-stored transcript bytes, exactly as the live splice
	// seeds from replayedBytes, so N turns can never grow the stored transcript
	// past the cap no matter how small each individual turn stays.
	storedBytes, err := s.Store.AgentSessionTranscriptBytes(ctx, sessionID)
	if err != nil {
		sse.errorAndDone("transcript unavailable")
		return
	}
	url := fmt.Sprintf("http://%s/turn", net.JoinHostPort(podIP, strconv.Itoa(s.driverPort())))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		sse.errorAndDone("turn dispatch failed")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient().Do(req)
	if err != nil {
		sse.errorAndDone("turn dispatch failed")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusConflict {
		sse.errorAndDone("a turn is already running")
		return
	}
	// Reject redirects and every other non-OK driver answer (codex-security
	// round-6 #2): the client never follows a 3xx, and the turn's parts may
	// only come from the exact pod the ticket authorized.
	if resp.StatusCode != http.StatusOK {
		sse.errorAndDone("turn dispatch failed")
		return
	}
	ordinal := base
	total := storedBytes
	reader := bufio.NewReader(resp.Body)
	for {
		payload, done, err := readSSEData(reader, maxSSEPartBytes)
		if err != nil || done {
			break
		}
		if payload == "" {
			continue
		}
		// Cumulative per-session byte quota (F10): a runaway turn — or many
		// individually small turns — can't grow the stored transcript or
		// gateway memory past the cap.
		if total+int64(len(payload)) > s.transcriptQuota() {
			log.Printf("agent turn: session transcript byte quota reached, stopping turn (session=%s)", sessionID)
			break
		}
		total += int64(len(payload))
		ordinal++
		if err := s.Store.AppendAgentSessionTranscript(ctx, sessionID, []store.AgentSessionTranscriptPart{
			{Seq: ordinal, Turn: turn, Part: []byte(payload)},
		}); err != nil {
			log.Printf("agent turn: tee failed (session=%s seq=%d): %v", sessionID, ordinal, err)
		}
		if err := sse.frame(payload); err != nil {
			// Client gone or stalled past the write deadline: stop forwarding.
			return
		}
	}
	sse.done()
}

// readSSEData reads one SSE `data:` payload from the driver framing. It returns
// the payload, whether it was the `[DONE]` sentinel, and io.EOF at stream end.
// The driver frames each part as a single `data: <json>\n\n` (JSON.stringify
// never emits raw newlines), so payloads are line-oriented. Each line is read
// through a bounded reader capped at maxBytes (F10): a part larger than the cap
// ends the stream with errPartTooLarge instead of allocating without limit.
func readSSEData(r *bufio.Reader, maxBytes int) (string, bool, error) {
	for {
		line, err := readLimitedLine(r, maxBytes)
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

// readLimitedLine reads one '\n'-terminated line but never buffers more than
// maxBytes of it (F10). ReadSlice returns at most the bufio buffer size per
// call, so a very long line without a newline is accumulated in bounded chunks;
// the moment the accumulation would exceed maxBytes it stops and returns
// errPartTooLarge instead of the unbounded ReadString('\n'). The returned string
// includes the trailing newline when one was found (callers trim it).
func readLimitedLine(r *bufio.Reader, maxBytes int) (string, error) {
	var sb strings.Builder
	for {
		chunk, err := r.ReadSlice('\n')
		if len(chunk) > 0 {
			if sb.Len()+len(chunk) > maxBytes {
				return "", errPartTooLarge
			}
			sb.Write(chunk)
		}
		switch err {
		case nil:
			return sb.String(), nil // reached the newline
		case bufio.ErrBufferFull:
			continue // long line, no newline yet — keep accumulating (bounded)
		default:
			return sb.String(), err // io.EOF or a read error
		}
	}
}

// applyCORS mirrors bex-api's withCORS (internal/api/auth.go): echo a matched
// Origin, allow the ticket + content-type request headers, expose the v1
// marker to the reader, and allow credentials (the AI SDK transport may send
// credentials:'include'; the gateway still ignores cookies and trusts only the
// ticket). Empty allowlist => no CORS headers (curl / same-origin).
func (s *Server) applyCORS(w http.ResponseWriter, r *http.Request) {
	allowed := s.AllowedOrigins
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
			w.Header().Set("Access-Control-Allow-Headers", TicketHeader+", Content-Type")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Expose-Headers", uiMessageStreamHeader)
			return
		}
	}
}

// agentSSE serializes SSE writes to one client response and preserves the
// verbatim UI-message-stream framing. Every write is bounded by a per-write
// deadline and the first failure is sticky: once a write fails (a gone or
// stalled client), later writes are no-ops and callers observe it via the
// frame error so they stop replaying/splicing and release the handler's memory
// and session-limiter slot.
type agentSSE struct {
	w            http.ResponseWriter
	flusher      http.Flusher
	rc           *http.ResponseController
	writeTimeout time.Duration
	err          error
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
	return &agentSSE{
		w:            w,
		flusher:      flusher,
		rc:           http.NewResponseController(w),
		writeTimeout: s.writeTimeout(),
	}
}

// setDeadline arms the per-write deadline on the underlying connection using the
// real wall clock (a socket operation — not the injected ticket clock). An
// unsupported ResponseWriter (e.g. a test recorder) returns ErrNotSupported,
// which is harmlessly ignored: the request context still bounds the stream.
func (a *agentSSE) setDeadline() {
	if a.rc != nil && a.writeTimeout > 0 {
		_ = a.rc.SetWriteDeadline(time.Now().Add(a.writeTimeout))
	}
}

func (a *agentSSE) put(str string) bool {
	if a.err != nil {
		return false
	}
	if _, err := io.WriteString(a.w, str); err != nil {
		a.err = err
		return false
	}
	return true
}

func (a *agentSSE) flush() {
	if a.err != nil {
		return
	}
	if a.rc != nil {
		if err := a.rc.Flush(); err == nil {
			return
		} else if !errors.Is(err, http.ErrNotSupported) {
			a.err = err
			return
		}
	}
	a.flusher.Flush()
}

// frame writes one part verbatim as `data: <payload>\n\n` — byte-identical to
// the driver's own framing, so the client stream matches what the driver
// emitted. It returns the sticky write error so a caller stops on a dead client.
func (a *agentSSE) frame(payload string) error {
	if a.err != nil {
		return a.err
	}
	a.setDeadline()
	if a.put("data: ") && a.put(payload) && a.put("\n\n") {
		a.flush()
	}
	return a.err
}

func (a *agentSSE) done() {
	if a.err != nil {
		return
	}
	a.setDeadline()
	if a.put("data: [DONE]\n\n") {
		a.flush()
	}
}

// Err reports the first write failure observed on this stream, if any.
func (a *agentSSE) Err() error { return a.err }

// errorAndDone emits a UI-message `error` part then the terminator, so a Vercel
// AI SDK client surfaces the failure and settles rather than hanging.
func (a *agentSSE) errorAndDone(message string) {
	_ = a.frame(fmt.Sprintf(`{"type":"error","errorText":%q}`, message))
	a.done()
}
