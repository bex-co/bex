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
//     `[DONE]`. This replay mode IS the transcript-read API. `[DONE]` means the
//     turn has settled: a stream whose sandbox does not serve yet is held until
//     it does, and one that ends before the turn settles closes without
//     `[DONE]`, for the client to attach again (w3/m406).
//   - POST /v1/agent-sessions/{id}/stream — forwards a live prompt turn to the
//     driver and streams the turn's parts back (w3/m43 t004).
//
// Byte-transparent LIVE forwarding is a contract, not a convenience (ADR047
// D3): the gateway preserves the `x-vercel-ai-ui-message-stream: v1` marker and
// the exact `data:` payload bytes of every live driver chunk — it never
// re-encodes, filters, reorders, or injects live parts. The durable transcript
// stores completed items (ADR051 § Item model), so the replay re-emits each as
// its minimal standard chunk sequence. Its only additions are authentication,
// the transcript tee, and replay-then-live splicing. Cookies on this path are
// ignored: the ticket is the sole credential (the endpoint publishes under the
// api.bex.co origin via edge path-routing, so a Kratos cookie will arrive).
package agentattach

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/agenttranscript"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/drivergrant"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
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

// sandboxPollFirst and sandboxPollMax are the backoff between attempts to
// reach a held stream's sandbox.
const (
	sandboxPollFirst = 500 * time.Millisecond
	sandboxPollMax   = 5 * time.Second
)

// defaultSSEHeartbeatInterval keeps an otherwise-idle live attach below the
// browser-facing edge's idle timeout. Agent turns can spend minutes in a tool or
// credential scrub without producing a UI part; the comment is transport-only
// and is never teed into the durable transcript.
const defaultSSEHeartbeatInterval = 15 * time.Second

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

// Store is the transport's narrow transcript and content-free session-audit
// authority. The ticket nonce claim reaches shell_ticket_nonces through the
// injected sshgateway.NonceGuard.
type Store interface {
	AgentSessionTranscriptReplay(ctx context.Context, sessionID string, after store.TranscriptPosition, maxBytes int64, limit int) ([]store.AgentSessionTranscriptPart, error)
	AgentSessionTranscriptBytes(ctx context.Context, sessionID string) (int64, error)
	AgentSessionTurns(ctx context.Context, sessionID string) ([]store.AgentSessionTurn, error)
	// GetAgentSession reads the session's phase and sandbox while a stream
	// waits for a sandbox that does not serve yet (w3/m406).
	GetAgentSession(ctx context.Context, id string) (store.AgentSession, error)
	AppendAgentSessionTranscript(ctx context.Context, sessionID string, parts []store.AgentSessionTranscriptPart) error
	StartSSHSession(context.Context, store.SSHSessionAudit) error
	EndSSHSession(context.Context, string, string, time.Time) error
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
	if pod.Status.PodIP == "" || sshgateway.TargetTerminated(pod, sandboxexec.SandboxContainer) {
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
	// KeyGrants holds an attach a bex API key opened (its ticket's
	// KeyAction) to the key's current grant at redemption and on every
	// revalidation tick (w9/125). nil refuses every key's ticket; a person's
	// never asks.
	KeyGrants *sshgateway.KeyGrantGate
	// AllowedOrigins is the browser origin allowlist for CORS. The endpoint
	// publishes under the api origin but is consumed cross-subdomain by the
	// dashboard (dashboard.bex.co -> api.bex.co), so the gateway must echo the
	// matched Origin and expose the v1 marker or the browser blocks the stream.
	// Reuse bex-api's BEX_API_CORS_ORIGIN value. Empty => no CORS headers
	// (same-origin / curl only).
	AllowedOrigins []string

	// TrustedProxies contains the immediate HTTP peers allowed to assert the
	// client address through X-Forwarded-For/X-Real-IP — the same Traefik pod
	// network the web shell trusts. Empty (the default) leaves RemoteAddr as
	// the immediate peer, which is what made ssh_sessions.remote_address record
	// Traefik's own pod IP for every attach session (w1/107).
	TrustedProxies core.TrustedProxies

	Metrics *sshgateway.Metrics
	Limits  *sshgateway.SessionLimiter
	Nonces  *sshgateway.NonceGuard

	SessionTimeout time.Duration

	// RevalidateInterval is how often an ESTABLISHED read/turn stream re-runs
	// the redemption-time revalidation (codex round-9 #6): a membership
	// revocation or session cancellation mid-stream aborts the replay/splice
	// instead of waiting for the driver to finish or the session cap. 0 => the
	// platform default; negative disables (pre-round-9 behavior).
	RevalidateInterval time.Duration

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

	// HeartbeatInterval overrides the live read/turn SSE comment cadence. Zero
	// uses the platform default; negative disables it for focused tests.
	HeartbeatInterval time.Duration

	// SandboxStartWait overrides how long a read stream is held for a working
	// session whose sandbox does not serve yet; zero => the bound a starting
	// sandbox holds an editor's SSH login and the Terminal for
	// (agentsessions.SSHWakeTimeout), past which the session has settled.
	SandboxStartWait time.Duration

	// sandboxPoll replaces the backoff with a fixed interval, so tests poll in
	// milliseconds.
	sandboxPoll time.Duration
}

func (s *Server) defaults() {
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

func (s *Server) heartbeatInterval() time.Duration {
	if s.HeartbeatInterval < 0 {
		return 0
	}
	if s.HeartbeatInterval > 0 {
		return s.HeartbeatInterval
	}
	return defaultSSEHeartbeatInterval
}

func (s *Server) sandboxStartWait() time.Duration {
	if s.SandboxStartWait > 0 {
		return s.SandboxStartWait
	}
	return agentsessions.SSHWakeTimeout
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
		Transport: driverTransport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// driverTransport bounds how long the driver may take to answer a request's
// headers, which it flushes at once (driver server.ts streamHeaders): a driver
// that accepts a connection and says nothing is tried again, not waited on
// for the whole session.
var driverTransport = func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 10 * time.Second
	return t
}()

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
	if err := s.revalidate(r.Context(), claims); err != nil {
		s.Metrics.Authentication("rejected_authz")
		http.Error(w, "no longer authorized for this session", http.StatusForbidden)
		return
	}
	if acquired, scope := s.Limits.Acquire(claims.Subject); !acquired {
		s.Metrics.LimitRejected(scope)
		http.Error(w, "session limit reached", http.StatusTooManyRequests)
		return
	}
	defer s.Limits.Release(claims.Subject)
	s.Metrics.Authentication("accepted")
	// The active-sessions gauge brackets the shared limiter slot exactly — the
	// gauge answers "how close is the process to BEX_SSH_MAX_SESSIONS?", so every
	// holder of the shared limiter reports, like nativessh/webshell (w1/m76/t005).
	// Wall clock deliberately, not s.now() (the injected ticket clock): the
	// duration is an operational observation, like setDeadline's.
	started := time.Now()
	s.Metrics.SessionStarted()
	result := "closed"
	defer func() { s.Metrics.SessionEnded(result, time.Since(started)) }()
	sessionID := ids.New(ids.SSHSession)
	auditCtx, cancelAudit := context.WithTimeout(context.Background(), 2*time.Second)
	err = s.Store.StartSSHSession(auditCtx, store.SSHSessionAudit{
		ID:            sessionID,
		Subject:       claims.Subject,
		WorkspaceID:   claims.Workspace,
		ServiceID:     claims.SessionID,
		InstanceID:    claims.Pod,
		RemoteAddress: s.remoteAddress(r),
		StartedAt:     started.UTC(),
		// A reader of the conversation, not an editor: it never holds the
		// sandbox open (w6/m564).
		Transport: store.SSHTransportAgentAttach,
	})
	cancelAudit()
	if err != nil {
		result = "failed"
		http.Error(w, "unable to start session", http.StatusInternalServerError)
		return
	}
	defer func() {
		endCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.Store.EndSSHSession(endCtx, sessionID, result, time.Now().UTC()); err != nil {
			log.Printf("agent attach session audit end failed: %v", err)
		}
	}()

	// codex round-9 #6: keep re-running the redemption-time revalidation while
	// the stream is LIVE — a membership revocation or session cancellation
	// aborts the replay/splice (every pump runs on this ctx) instead of riding
	// until the driver finishes or the session cap.
	timedCtx, cancelTimeout := context.WithTimeout(r.Context(), s.SessionTimeout)
	defer cancelTimeout()
	ctx, cancelExec := sshgateway.WithRevalidation(timedCtx, s.RevalidateInterval, func(c context.Context) error {
		return s.revalidate(c, claims)
	})
	defer cancelExec()
	// Report a watchdog-ended stream through the shared session vocabulary
	// (result="revoked"), like the other transports (w1/m76/t005). Registered
	// after the cancels so it runs before them and observes the live contexts.
	defer func() {
		if ctx.Err() != nil && timedCtx.Err() == nil {
			result = "revoked" // the watchdog, not the client, ended the stream
		}
	}()

	if r.Method == http.MethodGet {
		s.streamAgentAttach(ctx, s.startAgentSSE(w, flusher), claims)
		return
	}

	// A turn needs a live driver. Verify guarantees a turn ticket binds a pod.
	podIP, ipErr := s.Pods.PodIP(ctx, claims.Namespace, claims.Pod)
	// A POST turn's body must be read BEFORE the SSE response starts: Go's http
	// server closes the request body the moment the handler writes a response
	// (ErrBodyReadAfterClose), so forwarding r.Body after startAgentSSE always
	// fails. The read stays bounded by maxAgentTurnRequest (silent truncation,
	// as before).
	var turnBody []byte
	if ipErr == nil {
		tb, err := io.ReadAll(io.LimitReader(r.Body, maxAgentTurnRequest))
		if err != nil {
			http.Error(w, "turn body read failed", http.StatusBadRequest)
			return
		}
		turnBody = tb
	}
	sse := s.startAgentSSE(w, flusher)
	if ipErr != nil {
		// A gone pod cannot accept a turn.
		sse.errorAndDone("session is not live")
		return
	}
	s.forwardAgentTurn(ctx, sse, podIP, claims.SessionID, claims.Turn, bytes.NewReader(turnBody))
}

// revalidate is the redemption check, re-run on every watchdog tick: the
// subject's relation on the session and its lifecycle (Revalidator), then,
// for a ticket a bex API key minted, the key's current grant (w9/125).
func (s *Server) revalidate(ctx context.Context, claims agentsessionticket.Claims) error {
	if s.Revalidator != nil {
		if err := s.Revalidator.RevalidateAttach(ctx, claims.Subject, claims.SessionID, claims.Action); err != nil {
			return err
		}
	}
	return s.KeyGrants.RequireTicket(ctx, claims.Subject, claims.KeyAction)
}

// remoteAddress resolves who actually connected, for the session audit row.
// Behind Traefik the immediate peer is a pod IP, so a raw RemoteAddr makes every
// row say the same thing; the web shell's handler derives it the same way
// (webshell/websocket.go). An untrusted peer keeps its own address, so a client
// cannot forge the field by sending the header itself.
func (s *Server) remoteAddress(r *http.Request) string {
	if clientIP := s.TrustedProxies.ClientIP(r); clientIP != core.PeerIP(r.RemoteAddr) {
		return clientIP
	}
	return r.RemoteAddr
}

// streamAgentAttach replays the durable transcript to the client, then follows
// the turn live from the ticket's sandbox, teeing new parts into the store.
//
// The stream ends with `[DONE]` only once the turn has settled (w3/m406,
// ADR047 D9): a replay-only ticket's terminal session, the driver ending its
// turn (or the stream stopping for good at the transcript quota or an
// oversized part), or a session that settled while the stream waited for its
// sandbox. Every other end — the sandbox not serving within SandboxStartWait,
// the session moving to another sandbox, a dropped driver connection, a store
// failure — closes the stream without `[DONE]`, and the client attaches again
// with a fresh ticket.
func (s *Server) streamAgentAttach(ctx context.Context, sse *agentSSE, claims agentsessionticket.Claims) {
	turns, err := s.Store.AgentSessionTurns(ctx, claims.SessionID)
	if err != nil {
		log.Printf("agent attach: durable turns unavailable (session=%s): %v", claims.SessionID, err)
		return
	}
	replay := &durableReplay{s: s, sse: sse, sessionID: claims.SessionID, turn: claims.Turn, turns: turns,
		after: store.TranscriptStart, replayed: newReplayedTurn()}
	// A UI-message response must start before custom data parts. Durable prompts
	// are replay-adapter events (the raw assistant chunks cannot encode a user
	// role), so establish one synthetic response message before interleaving them.
	if replay.adapted() {
		start, _ := json.Marshal(map[string]any{"type": "start", "messageId": "durable-" + claims.SessionID})
		if err := sse.frame(string(start)); err != nil {
			return
		}
	}
	if !replay.run(ctx) {
		return
	}
	// A replay-only ticket (empty pod claim, ADR065 D2 — minted for a settled
	// session) never dials: the replay above is the whole history.
	end := liveTurnEnded
	if claims.Pod != "" {
		stopHeartbeat := sse.startHeartbeat(ctx, s.heartbeatInterval())
		end = s.followLive(ctx, claims, replay)
		stopHeartbeat()
	}
	switch end {
	case liveCut:
		return // no [DONE]: the client attaches again
	case liveSettledWhileWaiting:
		// What the turn stored since the replay above (the Completer's
		// harvest), then [DONE].
		if !replay.run(ctx) {
			return
		}
	}
	replay.finish()
}

// durableReplay frames the durable transcript in bounded pages rather than
// materializing the whole (up to 64 MiB) transcript at once (F10 follow-up).
// Peak memory per reader is one page, not the full session quota, so concurrent
// replays can't sum past the pod memory limit; and a stalled or disconnected
// client aborts the replay — freeing the page buffer and the shared
// session-limiter slot — instead of pinning them for the whole write. It keeps
// its place, so a stream that waited for its sandbox frames only what was
// stored meanwhile.
type durableReplay struct {
	s         *Server
	sse       *agentSSE
	sessionID string
	turn      int // the ticket's turn: the splice skips its replayed items
	turns     []store.AgentSessionTurn
	nextTurn  int // the next durable prompt to frame
	after     store.TranscriptPosition
	bytes     int64 // framed so far, against the session quota
	capped    bool  // the quota ended the replay
	replayed  *replayedTurn
}

func (r *durableReplay) adapted() bool { return len(r.turns) > 0 }

// run frames every stored part after the last one framed, each after the
// durable prompts of the turns up to its own, then every prompt still unframed:
// a prompt is durable before async dispatch, so it may legitimately have no
// assistant part yet (provisioning/failure), and the accepted user intent is
// shown before the live splice or terminal sentinel. False when the stream
// cannot go on (client gone, store failed).
func (r *durableReplay) run(ctx context.Context) bool {
	quota, pageBudget := r.s.transcriptQuota(), r.s.replayPageQuota()
	for !r.capped {
		if ctx.Err() != nil {
			return false
		}
		// Row-unbounded (limit 0): the replay pages purely by byte budget.
		page, err := r.s.Store.AgentSessionTranscriptReplay(ctx, r.sessionID, r.after, pageBudget, 0)
		if err != nil {
			log.Printf("agent attach: transcript replay failed (session=%s): %v", r.sessionID, err)
			return false
		}
		if len(page) == 0 {
			break
		}
		for _, part := range page {
			if !r.promptsThrough(part.Turn) {
				return false
			}
			// Cap the cumulative replay at the session quota (a stored transcript
			// that ever outgrew it is truncated rather than framed without limit).
			if r.bytes+int64(len(part.Part)) > quota {
				log.Printf("agent attach: transcript replay truncated at cap (session=%s)", r.sessionID)
				r.capped = true
				break
			}
			if part.Turn == r.turn {
				r.replayed.add(part)
			}
			r.bytes += int64(len(part.Part))
			if !r.frame(part) {
				return false
			}
		}
		r.after = page[len(page)-1].Position()
	}
	return r.promptsThrough(math.MaxInt)
}

// finish ends the stream of a settled turn: the adapter's closing finish (a
// legacy prompt-less transcript got no synthetic start, so its own structural
// chunks remain the only safe framing), then [DONE].
func (r *durableReplay) finish() {
	if r.adapted() {
		_ = r.sse.frame(`{"type":"finish"}`)
	}
	r.sse.done()
}

// frame writes one stored part: an item as its minimal standard chunk
// sequence, a legacy row as the one chunk it already is (ADR051 § Item model).
func (r *durableReplay) frame(part store.AgentSessionTranscriptPart) bool {
	chunks := []json.RawMessage{part.Part}
	if part.Item {
		chunks = agenttranscript.Expand(part.Part)
	}
	for _, chunk := range chunks {
		// Each driver turn carries its own start/finish chunks. The replay
		// adapter has already opened one response message so durable prompts and
		// all turns remain one ordered part stream; forwarding the nested
		// structural chunks makes AI SDK duplicate cumulative messages.
		payload, skip := replayUIChunk(chunk, part.Turn, r.turn, r.adapted())
		if skip {
			continue
		}
		if err := r.sse.frame(string(payload)); err != nil {
			// Client gone or stalled past the write deadline: stop now so the
			// deferred limiter release + this return free the reader's memory.
			return false
		}
	}
	return true
}

// promptsThrough frames the durable prompt of every turn up to lastTurn that
// is not framed yet.
func (r *durableReplay) promptsThrough(lastTurn int) bool {
	for r.nextTurn < len(r.turns) && r.turns[r.nextTurn].Turn <= lastTurn {
		turn := r.turns[r.nextTurn]
		data := map[string]any{
			"turn":         turn.Turn,
			"text":         turn.Prompt,
			"complete":     turn.TranscriptComplete,
			"settled":      turn.CompletedAt != nil,
			"truncated":    turn.TranscriptTruncated,
			"reason":       turn.TruncationReason,
			"deliveryMode": turn.DeliveryMode,
		}
		// The files the turn attached (ADR095 D3), as snapshotted at binding.
		if len(turn.Attachments) > 0 {
			data["attachments"] = turn.Attachments
		}
		payload, _ := json.Marshal(map[string]any{"type": "data-user-prompt", "data": data})
		if err := r.sse.frame(string(payload)); err != nil {
			return false
		}
		r.nextTurn++
	}
	return true
}

// liveEnd is how the live half of a read stream ended.
type liveEnd int

const (
	// liveCut: the stream ended before its turn settled. It closes without
	// [DONE], and the client attaches again.
	liveCut liveEnd = iota
	// liveTurnEnded: the driver ended the turn, or the stream stopped for good
	// (the transcript quota, an oversized part).
	liveTurnEnded
	// liveSettledWhileWaiting: the session settled while its sandbox did not
	// serve.
	liveSettledWhileWaiting
)

// followLive splices the driver stream of the ticket's sandbox. A sandbox that
// does not serve yet — its pod has no address, or its driver does not answer —
// is not the end of the turn while the session still works on it (w3/m406):
// the stream is held (the caller's heartbeat keeps it open) and the sandbox is
// tried again with backoff, up to SandboxStartWait.
func (s *Server) followLive(ctx context.Context, claims agentsessionticket.Claims, replay *durableReplay) liveEnd {
	// Wall clock, not s.now() (the injected ticket clock): an operational bound.
	deadline := time.Now().Add(s.sandboxStartWait())
	wait := cmp.Or(s.sandboxPoll, sandboxPollFirst)
	for {
		resp, err := s.dialDriverStream(ctx, claims)
		if err == nil {
			defer func() { _ = resp.Body.Close() }()
			return s.spliceDriverStream(ctx, resp, replay)
		}
		switch s.sandboxState(ctx, claims) {
		case sandboxSettled:
			return liveSettledWhileWaiting
		case sandboxReplaced:
			log.Printf("agent attach: the session left the ticket's sandbox while its stream waited (session=%s)", claims.SessionID)
			return liveCut
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			log.Printf("agent attach: sandbox did not serve within %s (session=%s): %v", s.sandboxStartWait(), claims.SessionID, err)
			return liveCut
		}
		if replay.sse.Err() != nil {
			return liveCut // a client that stopped reading the heartbeat
		}
		timer := time.NewTimer(min(wait, remaining))
		select {
		case <-ctx.Done():
			timer.Stop()
			return liveCut
		case <-timer.C:
		}
		wait = min(2*wait, cmp.Or(s.sandboxPoll, sandboxPollMax))
	}
}

// sandboxState is where the session of a stream waiting for its sandbox stands.
type sandboxState int

const (
	sandboxStarting sandboxState = iota // still working on the ticket's sandbox
	sandboxSettled                      // settled, or deleted: nothing more will come
	sandboxReplaced                     // working on another sandbox, or none yet
)

func (s *Server) sandboxState(ctx context.Context, claims agentsessionticket.Claims) sandboxState {
	record, err := s.Store.GetAgentSession(ctx, claims.SessionID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return sandboxSettled
	case err != nil:
		if ctx.Err() == nil {
			log.Printf("agent attach: session read failed while its stream waited (session=%s): %v", claims.SessionID, err)
		}
		return sandboxStarting // ask again on the next attempt
	case slices.Contains(store.TerminalAgentSessionPhases, record.Phase):
		return sandboxSettled
	case record.SandboxID != claims.SandboxID:
		return sandboxReplaced
	}
	return sandboxStarting
}

// Decode once for both replay adaptations: structural chunks belong to the
// enclosing message, and old errors are content rather than fatal SDK errors.
func replayUIChunk(part []byte, partTurn, currentTurn int, adaptedReplay bool) ([]byte, bool) {
	if !adaptedReplay && partTurn >= currentTurn {
		return part, false
	}
	var chunk struct {
		Type      string `json:"type"`
		ErrorText string `json:"errorText"`
	}
	if json.Unmarshal(part, &chunk) != nil {
		return part, false
	}
	if adaptedReplay && (chunk.Type == "start" || chunk.Type == "finish") {
		return nil, true
	}
	if partTurn >= currentTurn || chunk.Type != "error" {
		return part, false
	}
	payload, _ := json.Marshal(map[string]any{
		"type": "data-bex-turn-error",
		"data": map[string]any{"turn": partTurn, "errorText": chunk.ErrorText},
	})
	return payload, false
}

func structuralUIChunk(part []byte) bool {
	t := agenttranscript.ChunkType(part)
	return t == "start" || t == "finish"
}

// spliceDriverStream reads the driver's /stream (which replays its full history
// then goes live), folds every chunk into the turn's items and tees each
// completed item into the durable store under its turn-local index (ADR051 §
// Item model), and forwards verbatim only the chunks of items this client did
// not already replay.
func (s *Server) spliceDriverStream(ctx context.Context, resp *http.Response, replay *durableReplay) liveEnd {
	sessionID := replay.sessionID
	tee := newTurnTee(ctx, s.Store, sessionID, replay.turn, replay.bytes, s.transcriptQuota(), replay.replayed)
	defer tee.close()

	var ordinal int64 = -1
	reader := bufio.NewReader(resp.Body)
	for {
		payload, done, err := readSSEData(reader, maxSSEPartBytes)
		switch {
		case done: // driver `data: [DONE]` sentinel: the turn ended
			return liveTurnEnded
		case errors.Is(err, errPartTooLarge):
			// Every reattach would meet the same part again (the driver replays
			// its history), so the stream stops for good.
			log.Printf("agent attach: live splice stopped (session=%s): %v", sessionID, err)
			return liveTurnEnded
		case err != nil:
			// The driver connection ended without the sentinel: the turn did not.
			log.Printf("agent attach: live splice cut (session=%s): %v", sessionID, err)
			return liveCut
		case payload == "":
			continue
		}
		ordinal++
		// Per-session byte quota (F10): stop teeing+streaming once the session's
		// stored transcript would exceed the cumulative cap, so tenant output
		// can't grow Postgres or the gateway without bound. The client is
		// settled with `[DONE]` by the caller.
		forward, ok := tee.push(ordinal, payload)
		if !ok {
			log.Printf("agent attach: session transcript byte quota reached, stopping splice (session=%s)", sessionID)
			return liveTurnEnded
		}
		// Only suppress what this client actually replayed. A DB max-index
		// query here could include a concurrent writer's not-yet-replayed items.
		if !forward || (replay.adapted() && structuralUIChunk([]byte(payload))) {
			continue
		}
		// Forward first; the tee persists completed items through a bounded
		// batch so browser delivery is not blocked on PostgreSQL latency (w5/m77).
		if err := replay.sse.frame(payload); err != nil {
			// Client gone or stalled past the write deadline. The tee still
			// persists completed items and flushes in-flight ones as partial;
			// the Completer's harvest replaces those at session end.
			return liveCut
		}
	}
}

// dialDriverStream opens the GET /stream of the ticket's sandbox's driver
// (which replays its full history then goes live) for browser live-splice.
func (s *Server) dialDriverStream(ctx context.Context, claims agentsessionticket.Claims) (*http.Response, error) {
	podIP, err := s.Pods.PodIP(ctx, claims.Namespace, claims.Pod)
	if err != nil {
		return nil, err
	}
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
	grant, err := drivergrant.Mint(s.Secret, sessionID, s.now(), 15*time.Second)
	if err != nil {
		sse.errorAndDone("turn dispatch failed")
		return
	}
	req.Header.Set(drivergrant.Header, grant)
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
	stopHeartbeat := sse.startHeartbeat(ctx, s.heartbeatInterval())
	defer stopHeartbeat()
	tee := newTurnTee(ctx, s.Store, sessionID, turn, storedBytes, s.transcriptQuota(), newReplayedTurn())
	defer tee.close()
	var partIndex int64 = -1
	reader := bufio.NewReader(resp.Body)
	for {
		payload, done, err := readSSEData(reader, maxSSEPartBytes)
		if err != nil || done {
			break
		}
		if payload == "" {
			continue
		}
		partIndex++
		// Cumulative per-session byte quota (F10): a runaway turn — or many
		// individually small turns — can't grow the stored transcript or
		// gateway memory past the cap.
		if _, ok := tee.push(partIndex, payload); !ok {
			log.Printf("agent turn: session transcript byte quota reached, stopping turn (session=%s)", sessionID)
			break
		}
		if err := sse.frame(payload); err != nil {
			// Client gone or stalled past the write deadline: stop forwarding.
			return
		}
	}
	stopHeartbeat()
	tee.close()
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
	mu           sync.Mutex
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
	a.mu.Lock()
	defer a.mu.Unlock()
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
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.err != nil {
		return
	}
	a.setDeadline()
	if a.put("data: [DONE]\n\n") {
		a.flush()
	}
}

func (a *agentSSE) comment(value string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.err != nil {
		return a.err
	}
	a.setDeadline()
	if a.put(": ") && a.put(value) && a.put("\n\n") {
		a.flush()
	}
	return a.err
}

// startHeartbeat serializes comment frames with transcript frames until the
// live driver splice returns. The returned stop waits for the writer goroutine,
// so `[DONE]` cannot race a late heartbeat onto the wire.
func (a *agentSSE) startHeartbeat(ctx context.Context, interval time.Duration) func() {
	if interval <= 0 {
		return func() {}
	}
	stop := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				if err := a.comment("keep-alive"); err != nil {
					return
				}
			}
		}
	}()
	var stopOnce sync.Once
	return func() {
		stopOnce.Do(func() {
			close(stop)
			<-stopped
		})
	}
}

// Err reports the first write failure observed on this stream, if any.
func (a *agentSSE) Err() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.err
}

// errorAndDone emits a UI-message `error` part then the terminator, so a Vercel
// AI SDK client surfaces the failure and settles rather than hanging.
func (a *agentSSE) errorAndDone(message string) {
	_ = a.frame(fmt.Sprintf(`{"type":"error","errorText":%q}`, message))
	a.done()
}
