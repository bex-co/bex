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

package sandbox

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ExecConfig wires the sandbox-exec bridge (w3/m33, `render ea sandbox exec`).
// bex-api authorizes can_operate on the sandbox, mints a signed ticket binding
// the exact pod/namespace/command, and reverse-proxies the SSE stream from the
// isolated SSH gateway (which alone holds pods/exec, Option A). Zero value =>
// exec unconfigured, the verb 503s.
type ExecConfig struct {
	// Secret is the HMAC key shared ONLY with the gateway (BEX_SANDBOX_EXEC_SECRET).
	Secret []byte
	// GatewayURL is the gateway's internal sandbox-exec endpoint
	// (e.g. http://bex-ssh-gateway.bex-system.svc:8081/sandbox-exec).
	GatewayURL string
	// FileGatewayURL is the gateway's internal /sandbox-files endpoint. File
	// transfers use distinct tickets and never pass a caller-chosen command.
	FileGatewayURL string
	// Client is the HTTP client used to stream from the gateway:
	// sshgateway.NewUpstreamClient in production, which bounds dialing and the
	// gateway's answer but not the long-lived stream itself (the request
	// context bounds that).
	Client *http.Client
	// TTL bounds the minted ticket's lifetime (default 60s).
	TTL time.Duration
	// PublicURL is the externally reachable API origin (BEX_API_PUBLIC_URL) the
	// run connect-token `uri` is built on (w7/m147). Empty ⇒ the origin the
	// mint request arrived on.
	PublicURL string
	// Nonces makes run connect tokens single-use across bex-api replicas when
	// wired with the shared store (sshgateway.NonceGuard{Store}). nil ⇒ a
	// process-local guard is used, so a token still never redeems twice on one
	// pod (the cross-replica gap is the store-off deployment's, as for shells).
	Nonces     *sshgateway.NonceGuard
	noncesOnce sync.Once
}

// ExecRequest is the caller's exec input: OwnerID binds the workspace (Render's
// query/body ownerId), SandboxID is the target, Command is the single command
// string the Render CLI sends (run via `/bin/sh -c`).
type ExecRequest struct {
	OwnerID   string
	SandboxID string
	Command   string
	// ExecutionID names the execution record a run connect token already
	// opened; empty means the exec opens its own. When it is set, OwnerID is
	// the token's resolved workspace (the record's key), not a caller's raw
	// ownerId.
	ExecutionID string
}

// execTarget is the sandbox an exec resolved to: its workspace and public id.
type execTarget struct {
	workspace, sandboxID string
}

// systemExecSubject is the ticket subject the trusted Completer's status read
// mints under (it has no caller identity). It is the shared sentinel defined by
// the ticket package; aliased here for the existing call sites' readability.
const systemExecSubject = sandboxexec.SystemSubject

func (s *Service) execEnabled() bool {
	return s.Exec != nil && len(s.Exec.Secret) > 0 && s.Exec.GatewayURL != ""
}

// dialGateway authorizes the caller (can_create on the sandbox's workspace),
// mints a signed exec ticket, and opens the gateway's SSE stream. It returns the
// live response (the caller drains + closes Body) or a pre-stream error, so a
// failure (authz, ticket, gateway down) surfaces before any bytes are written.
//
// SECURITY (codex round-16 #7): the Command field is caller-selected executable
// content that reaches pods/exec, so this is create-like — matching Create and
// the m68 executable-selection class — not lifecycle. A demoted contributor
// who still owns a pre-existing sandbox must not keep choosing arbitrary shell
// after losing can_create. Agent-session sandboxes keep the stronger
// can_view_sensitive gate below.
//
// The returned release ends the command's hold on the sandbox's idle timer
// (holdActive); the caller runs it when the stream ends.
func (s *Service) dialGateway(ctx context.Context, req ExecRequest) (*http.Response, func(), execTarget, error) {
	ws, raw, err := s.authorizeExecTarget(ctx, req)
	if err != nil {
		return nil, nil, execTarget{}, err
	}
	release := s.holdActive(ctx, ws, raw)
	// The ticket binds the SUBSTRATE id, not the caller's: the gateway derives
	// the pod name as `<id>-0`, and a caller may address the sandbox by its
	// public `sbx-` id (w9/m94).
	resp, err := s.mintAndDial(ctx, ws, raw.ID, raw.Metadata[metadataAgentSession], []string{"/bin/sh", "-c", req.Command})
	if err != nil {
		release()
		return nil, nil, execTarget{}, err
	}
	return resp, release, execTarget{workspace: ws, sandboxID: canonicalID(raw)}, nil
}

// openExecution returns the record an exec reports to: the one its connect
// token opened, or a new one for an exec that had no token.
func (s *Service) openExecution(ctx context.Context, req ExecRequest, target execTarget, operation string) string {
	if req.ExecutionID != "" {
		return req.ExecutionID
	}
	executionID := id.New(id.SandboxExecution)
	subject := ""
	if idn, ok := core.IdentityFrom(ctx); ok {
		subject = idn.Subject
	}
	s.recordExecution(ctx, store.SandboxExecution{
		ID: executionID, WorkspaceID: target.workspace, SandboxID: target.sandboxID,
		Type: ExecutionTypeRuns, Operation: operation, Command: req.Command,
		Subject: subject, StartedAt: s.Now(),
	})
	return executionID
}

// authorizeExecTarget is the exec gate shared by the direct exec verbs and the
// run connect-token mint (ConnectRun): it resolves the workspace, authorizes
// can_create, resolves the owned sandbox, and applies the agent-session rule
// below. It returns the resolved workspace and the live sandbox object; it
// opens no stream and mints nothing, but it does record the interaction and
// resume a sandbox that was suspended for inactivity (ensureRunning), so the
// run token is minted, and the command runs, against a running sandbox.
func (s *Service) authorizeExecTarget(ctx context.Context, req ExecRequest) (string, osSandbox, error) {
	ctx, err := s.scopeSandbox(ctx, req.OwnerID, req.SandboxID)
	if err != nil {
		return "", osSandbox{}, err
	}
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return "", osSandbox{}, err
	}
	if !s.execEnabled() || !s.enabled() {
		return "", osSandbox{}, core.ErrSandboxesUnavailable
	}
	if req.SandboxID == "" || req.Command == "" {
		return "", osSandbox{}, fmt.Errorf("%w: sandbox id and command are required", core.ErrBadRequest)
	}
	ws, ok := s.Tenant(ctx)
	if !ok {
		return "", osSandbox{}, fmt.Errorf("%w: no workspace resolved for exec", core.ErrForbidden)
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return "", osSandbox{}, err
	}
	raw, err := s.ownedSandbox(ctx, key, ws, req.SandboxID)
	if err != nil {
		return "", osSandbox{}, s.endedConflict(ctx, ws, req.SandboxID, err)
	}
	// An agent-session sandbox is a credential-capable pod (the Git-write and
	// model proxies live behind it), and model.fga deliberately gates a real
	// shell into it on the session object's can_view_sensitive — can_operate is
	// documented as too weak (a contributor holds it). The generic exec verb
	// must not become the weaker side door, so a sandbox carrying the session
	// binding additionally requires that stronger relation, fresh (this gates a
	// privilege exercise, the round-5 finding-4 class). The dedicated surfaces
	// (ags-… SSH, agent attach) already enforce exactly this (round-13 #1).
	if sessionID := raw.Metadata[metadataAgentSession]; sessionID != "" {
		if err := s.AuthorizeFreshOn(ctx, core.RelCanViewSensitive, agentsession.SessionObject(sessionID)); err != nil {
			return "", osSandbox{}, err
		}
	}
	if endedLive(raw) {
		return "", osSandbox{}, sandboxEndedError(s.sandboxView(raw, ws))
	}
	raw, err = s.ensureRunning(ctx, key, ws, raw)
	if err != nil {
		return "", osSandbox{}, err
	}
	return ws, raw, nil
}

// mintAndDial signs an exec ticket binding the exact pod/namespace/command and
// opens the gateway SSE stream. It performs NO authorization or ownership check
// — every caller (dialGateway for the public verb, ReadSessionStatus for the
// trusted Completer) must gate the exact sandbox before calling it.
func (s *Service) mintAndDial(ctx context.Context, ws, sandboxID, agentSessionID string, command []string) (*http.Response, error) {
	// A caller-facing exec (dialGateway) always carries an identity; the trusted
	// Completer's status read runs in a system loop with none. The gateway rejects
	// an empty-subject ticket as malformed (sandboxexec.Verify), so fall back to a
	// stable system subject — otherwise every completer status read 401s and the
	// session strands in `running` forever (w3/m43 live E2E).
	subject := systemExecSubject
	if idn, ok := core.IdentityFrom(ctx); ok && idn.Subject != "" {
		subject = idn.Subject
	}
	ttl := s.Exec.TTL
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	now := s.Now()
	// The sandbox runs in the caller's own `<ws>-sandbox` namespace (workspace ==
	// namespace, m31/ADR043); deriving it from the RESOLVED workspace is what
	// scopes exec to the caller's own sandboxes — a foreign sandbox id resolves
	// to a pod that does not exist in this namespace.
	ticket, err := sandboxexec.Mint(s.Exec.Secret, sandboxexec.Claims{
		Subject:   subject,
		SandboxID: sandboxID,
		Namespace: ws + "-sandbox",
		Command:   command,
		Workspace: ws,
		// Signed into the claims (not re-derived at the gateway) so redemption and
		// live-stream revalidation can re-require the session's stronger relation
		// even though the gateway holds no OpenSandbox client (round-13 #1/#3).
		AgentSessionID: agentSessionID,
		IssuedAt:       now.Unix(),
		ExpiresAt:      now.Add(ttl).Unix(),
	})
	if err != nil {
		return nil, err
	}

	greq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Exec.GatewayURL, nil)
	if err != nil {
		return nil, err
	}
	greq.Header.Set(sandboxexec.TicketHeader, ticket)
	client := s.Exec.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(greq)
	if err != nil {
		// A configured gateway that does not answer is out of reach, not
		// unconfigured. The cause is not logged here: every caller logs the
		// error it is handed.
		return nil, unreachable("the exec gateway did not answer")
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, core.ErrForbidden
		case http.StatusTooManyRequests:
			return nil, errExecSessionLimit
		default:
			return nil, unreachable(fmt.Sprintf("the exec gateway answered %d", resp.StatusCode))
		}
	}
	return resp, nil
}

// StreamExec authorizes, opens the gateway stream, and copies its SSE response
// (stdout/stderr chunks + a terminal exit event) into w. It writes the SSE headers
// once the gateway accepts, so a pre-stream failure returns an error the adapter
// renders as a normal HTTP status instead of a half-open event stream — the
// Render CLI's single-POST-reads-SSE contract (docs/render-artifacts/ea-sandbox.md).
// The one exception is a wait for a resume (w32/m299): the stream opens and
// sends keep-alives, since a reader's idle timeout (the Render SDK's 45 s)
// would expire before a cold resume finishes, and a failure after that is an
// SSE error event.
func (s *Service) StreamExec(ctx context.Context, req ExecRequest, w http.ResponseWriter, flush func()) error {
	stream := &execStream{w: w, flush: flush}
	ctx = withResumeHeartbeat(ctx, stream.beat)
	resp, release, target, err := s.dialGateway(ctx, req)
	if err != nil {
		// A token's record ends here: its command never ran.
		s.stopExecution(ctx, req.OwnerID, req.ExecutionID, nil)
		if stream.started {
			stream.fail(err)
			return nil
		}
		return err
	}
	defer release()
	defer func() { _ = resp.Body.Close() }()
	executionID := s.openExecution(ctx, req, target, ConnectOperationStream)
	var exit exitSniffer
	// Synchronous, so a listing right after the stream ends sees it stopped;
	// the write is one bounded keyed UPDATE.
	defer func() { s.stopExecution(ctx, target.workspace, executionID, exit.exitCode) }()
	defer s.keepExecutionAlive(ctx, target.workspace, executionID)()
	stream.start()
	// The gateway accepted: the command now runs as long as it needs, bounded
	// by the gateway session and the sandbox, as on Render. allowLongResume's
	// write deadline only had to outlast a resume; left in place it cut every
	// exec at 210 s, printing or not (w8/m151). The log tail lifts it the same
	// way.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	// Either end failing — client disconnect, or upstream ending on context
	// cancel/timeout — closes the stream with nothing left to report, so the
	// copy's error is deliberately discarded.
	_, _ = io.Copy(flushingWriter{w: w, flush: flush}, io.TeeReader(resp.Body, sniffWriter{&exit}))
	return nil
}

// execStream is StreamExec's response before the gateway's bytes flow.
type execStream struct {
	w        http.ResponseWriter
	flush    func()
	started  bool
	lastSent time.Time
}

func (e *execStream) start() {
	if e.started {
		return
	}
	e.started = true
	e.w.Header().Set("Content-Type", "text/event-stream")
	e.w.Header().Set("Cache-Control", "no-cache")
	e.w.WriteHeader(http.StatusOK)
	e.sent()
}

func (e *execStream) sent() {
	if e.flush != nil {
		e.flush()
	}
	e.lastSent = time.Now()
}

// beat runs on each poll of a resume wait.
func (e *execStream) beat() {
	if !e.started {
		e.start()
		return
	}
	if time.Since(e.lastSent) >= sandboxexec.KeepAliveInterval {
		_, _ = io.WriteString(e.w, ": keep-alive\n\n")
		e.sent()
	}
}

// fail reports err as the gateway's `error` event.
func (e *execStream) fail(err error) {
	data, _ := json.Marshal(sandboxexec.NewErrorEvent(core.ErrorReply(err)))
	_, _ = fmt.Fprintf(e.w, "event: error\ndata: %s\n\n", data)
	e.sent()
}

// sniffWriter feeds an exitSniffer from an io.TeeReader.
type sniffWriter struct{ x *exitSniffer }

func (w sniffWriter) Write(p []byte) (int, error) {
	w.x.observe(p)
	return len(p), nil
}

// flushingWriter flushes the transport after every chunk, so an SSE consumer
// sees each event as it arrives rather than at the transport's buffering
// boundary.
type flushingWriter struct {
	w     io.Writer
	flush func()
}

func (f flushingWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if err == nil && f.flush != nil {
		f.flush()
	}
	return n, err
}

// ExecResult is the buffered outcome of a sandbox exec (MCP surface): the full
// stdout/stderr and the exit code. Agents drive exec over MCP without the CLI's
// SSE handshake (ADR014 D3), so they get the collected output, not a stream.
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
	// Truncated is set when the command's combined output exceeded
	// maxExecOutputBytes and the buffered result was cut off at the cap.
	Truncated bool `json:"truncated,omitempty"`
}

// maxExecOutputBytes caps the combined stdout+stderr a single buffered exec
// accumulates in the shared API process (w1/m65 F8). Each SSE line is already
// bounded (bufferExec's scanner cap), but the number of lines is not, so an
// authorized tenant command could otherwise grow an unbounded in-memory string
// and exhaust the API pod. At the cap bufferExec stops reading and lets the
// caller close the upstream stream; ExecResult.Truncated flags the cutoff. 2
// MiB mirrors bex-api's BEX_MAX_BODY_BYTES default. Prefer StreamExec for large
// output — the streaming path never buffers.
const maxExecOutputBytes = 2 << 20

// ExecBuffered runs one command and collects its output — the same authorize +
// ticket + gateway path as StreamExec, but it consumes the SSE and returns the
// assembled result rather than streaming. Used by MCP `sandbox_exec`.
func (s *Service) ExecBuffered(ctx context.Context, req ExecRequest) (ExecResult, error) {
	resp, release, target, err := s.dialGateway(ctx, req)
	if err != nil {
		return ExecResult{}, err
	}
	defer release()
	defer func() { _ = resp.Body.Close() }()
	executionID := s.openExecution(ctx, req, target, ExecutionOperationSync)
	stopHeartbeat := s.keepExecutionAlive(ctx, target.workspace, executionID)
	res, err := bufferExec(resp)
	stopHeartbeat()
	var exitCode *int
	if err == nil && !res.Truncated {
		exitCode = &res.ExitCode
	}
	s.stopExecution(ctx, target.workspace, executionID, exitCode)
	return res, err
}

// bufferExec drains one gateway SSE exec response into a collected result. It is
// shared by the public MCP verb and the trusted session-status read.
func bufferExec(resp *http.Response) (ExecResult, error) {
	return bufferExecWithLimit(resp, maxExecOutputBytes)
}

// bufferExecWithLimit is the bounded collector shared by ordinary MCP exec
// (2 MiB) and trusted agent-transcript harvest (the driver's 16 MiB log plus
// framing headroom). Callers must still reject Truncated when completeness is
// part of their contract.
func bufferExecWithLimit(resp *http.Response, maxOutputBytes int) (ExecResult, error) {
	var out ExecResult
	exitSeen := false
	total := 0 // combined stdout+stderr bytes accumulated so far
	// Builders, not string +=: the budget below admits up to maxExecOutputBytes
	// across many small SSE chunks, and appending to a string reallocates and
	// copies everything accumulated so far on each one.
	var stdout, stderr strings.Builder
	finish := func() ExecResult {
		out.Stdout, out.Stderr = stdout.String(), stderr.String()
		return out
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	event := ""
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			payload := strings.TrimPrefix(line, "data: ")
			switch event {
			case "output":
				var ev struct {
					Stream string `json:"stream"`
					Data   string `json:"data"`
				}
				if json.Unmarshal([]byte(payload), &ev) == nil {
					// Enforce a cumulative output budget: append only what fits, then
					// stop reading so the caller closes the upstream stream (F8). An
					// unbounded number of in-budget lines can no longer exhaust the pod.
					// remaining is always > 0 here: the loop returns the moment total
					// reaches the cap, so it never re-enters at or past it.
					data := ev.Data
					if remaining := maxOutputBytes - total; len(data) > remaining {
						data = data[:remaining]
						out.Truncated = true
					}
					total += len(data)
					if ev.Stream == "stderr" {
						stderr.WriteString(data)
					} else {
						stdout.WriteString(data)
					}
					if out.Truncated {
						return finish(), nil
					}
				}
			case "exit":
				if code := decodeExitCode([]byte(payload)); code != nil {
					out.ExitCode, exitSeen = *code, true
				}
			case "error":
				// Same transition: the pinned CLI's {status, message} beside the
				// internal {error, code}. `code` (or a 404 status) names the
				// terminated target either way.
				var ev sandboxexec.ErrorEvent
				if json.Unmarshal([]byte(payload), &ev) == nil {
					if err := execEventError(ev); err != nil {
						return finish(), err
					}
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return finish(), core.HideCause(unreachable("the exec stream broke"), err)
	}
	if !exitSeen {
		return finish(), unreachable("the exec stream ended without an exit status")
	}
	return finish(), nil
}

// execEventError maps the gateway's `error` event onto bex-api's error
// classes: a terminated target is gone (NotFound, with how its container
// ended when the gateway saw it), revoked access is forbidden, and anything
// else — a target that is not ready, an exec that could not start — is out of
// reach for now, never "sandbox runtime not configured". nil for an
// event without a message.
func execEventError(ev sandboxexec.ErrorEvent) error {
	msg := ev.Message
	if msg == "" {
		msg = ev.Error
	}
	switch {
	case msg == "":
		return nil
	case ev.Code == sandboxexec.ErrorCodeTargetTerminated || ev.Status == http.StatusNotFound:
		return &TerminatedError{Termination: Termination{Reason: ev.TerminatedReason, ExitCode: ev.TerminatedExitCode}}
	case ev.Code == sandboxexec.ErrorCodeTargetUnavailable:
		return unreachable("its pod is not ready; its node may be NotReady or unreachable")
	case ev.Status == http.StatusForbidden:
		return fmt.Errorf("%w: %s", core.ErrForbidden, msg)
	}
	return unreachable(msg)
}
