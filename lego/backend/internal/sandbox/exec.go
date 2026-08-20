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
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
)

// ExecConfig wires the sandbox-exec bridge (w3/m33, `render ea sandbox exec`).
// bex-api authorizes can_operate on the sandbox, mints a signed ticket binding
// the exact pod/namespace/command, and reverse-proxies the SSE stream from the
// isolated SSH gateway (which alone holds pods/exec, Option A). Zero value =>
// exec unconfigured, the verb 503s.
type ExecConfig struct {
	// Secret is the HMAC key shared ONLY with the gateway (BEX_SANDBOX_EXEC_SECRET).
	Secret []byte
	// DriverGrantSecret signs action-bound calls into the in-sandbox driver.
	// It is the gateway trust secret, never injected into the sandbox.
	DriverGrantSecret []byte
	// GatewayURL is the gateway's internal sandbox-exec endpoint
	// (e.g. http://bex-ssh-gateway.bex-system.svc:8081/sandbox-exec).
	GatewayURL string
	// Client is the HTTP client used to stream from the gateway (no timeout —
	// the stream is long-lived; the request context bounds it).
	Client *http.Client
	// TTL bounds the minted ticket's lifetime (default 60s).
	TTL time.Duration
}

// ExecRequest is the caller's exec input: OwnerID binds the workspace (Render's
// query/body ownerId), SandboxID is the target, Command is the single command
// string the Render CLI sends (run via `/bin/sh -c`).
type ExecRequest struct {
	OwnerID   string
	SandboxID string
	Command   string
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
func (s *Service) dialGateway(ctx context.Context, req ExecRequest) (*http.Response, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return nil, err
	}
	if !s.execEnabled() || !s.enabled() {
		return nil, core.ErrSandboxesUnavailable
	}
	if req.SandboxID == "" || req.Command == "" {
		return nil, fmt.Errorf("%w: sandbox id and command are required", core.ErrBadRequest)
	}
	ws, ok := s.Tenant(ctx)
	if !ok {
		return nil, fmt.Errorf("%w: no workspace resolved for exec", core.ErrForbidden)
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := s.ownedSandbox(ctx, key, ws, req.SandboxID)
	if err != nil {
		return nil, err
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
			return nil, err
		}
	}
	return s.mintAndDial(ctx, ws, req.SandboxID, raw.Metadata[metadataAgentSession], []string{"/bin/sh", "-c", req.Command})
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
		return nil, fmt.Errorf("%w: sandbox exec gateway unreachable", core.ErrSandboxesUnavailable)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return nil, core.ErrForbidden
		case http.StatusTooManyRequests:
			return nil, fmt.Errorf("%w: too many concurrent exec sessions", core.ErrBadRequest)
		default:
			return nil, fmt.Errorf("%w: sandbox exec unavailable", core.ErrSandboxesUnavailable)
		}
	}
	return resp, nil
}

// StreamExec authorizes, opens the gateway stream, and copies its SSE response
// (stdout/stderr chunks + a terminal exitCode) into w. It writes the SSE headers
// only once the gateway accepts, so a pre-stream failure returns an error the
// adapter renders as a normal HTTP status instead of a half-open event stream —
// the Render CLI's single-POST-reads-SSE contract (docs/render-artifacts/ea-sandbox.md).
func (s *Service) StreamExec(ctx context.Context, req ExecRequest, w http.ResponseWriter, flush func()) error {
	resp, err := s.dialGateway(ctx, req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if flush != nil {
		flush()
	}
	// Either end failing — client disconnect, or upstream ending on context
	// cancel/timeout — closes the stream with nothing left to report, so the
	// copy's error is deliberately discarded.
	_, _ = io.Copy(flushingWriter{w: w, flush: flush}, resp.Body)
	return nil
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

// systemBufferedExec runs ONE platform-fixed command in a sandbox through the
// gateway boundary without the caller-facing exec gate (dialGateway). It exists
// for trusted lifecycle mechanics — the agent-session pre-snapshot credential
// scrub (clientSuspend) — whose command is assembled entirely platform-side
// (agentSnapshotCommand); the caller controls only that it runs, never its
// bytes, and the output is checked for exit code alone, never returned. The
// agent-session can_view_sensitive gate in dialGateway must not reach this
// path: a contributor (can_operate) may suspend a session sandbox, and the
// scrub is what makes that suspend safe (round-13 #1 keeps the boundary on
// CALLER-chosen commands, not platform-chosen ones). Unexported on purpose: it
// is mechanics, not an API verb — the relation-guard sweep treats every
// exported Service method as a verb.
func (s *Service) systemBufferedExec(ctx context.Context, workspace, sandboxID, agentSessionID, command string) (ExecResult, error) {
	resp, err := s.mintAndDial(ctx, workspace, sandboxID, agentSessionID, []string{"/bin/sh", "-c", command})
	if err != nil {
		return ExecResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	return bufferExec(resp)
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
	resp, err := s.dialGateway(ctx, req)
	if err != nil {
		return ExecResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	return bufferExec(resp)
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
				var ev struct {
					ExitCode int `json:"exitCode"`
				}
				if json.Unmarshal([]byte(payload), &ev) == nil {
					out.ExitCode = ev.ExitCode
					exitSeen = true
				}
			case "error":
				var ev struct {
					Error string `json:"error"`
					Code  string `json:"code"`
				}
				if json.Unmarshal([]byte(payload), &ev) == nil && ev.Error != "" {
					if ev.Code == sandboxexec.ErrorCodeTargetTerminated {
						return finish(), fmt.Errorf("%w: sandbox terminated", core.ErrNotFound)
					}
					return finish(), fmt.Errorf("%w: %s", core.ErrSandboxesUnavailable, ev.Error)
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return finish(), fmt.Errorf("%w: sandbox exec stream failed: %v", core.ErrSandboxesUnavailable, err)
	}
	if !exitSeen {
		return finish(), fmt.Errorf("%w: sandbox exec stream ended without an exit event", core.ErrSandboxesUnavailable)
	}
	return finish(), nil
}
