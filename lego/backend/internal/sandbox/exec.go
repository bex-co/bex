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

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
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
	// GatewayURL is the gateway's internal sandbox-exec endpoint
	// (e.g. http://bex-ssh-gateway.bex-system.svc:8081/sandbox-exec).
	GatewayURL string
	// RecordURL is the gateway's internal headless-transcript-recorder endpoint
	// (ADR051), on the same internal listener as GatewayURL
	// (e.g. http://bex-ssh-gateway.bex-system.svc:8081/agent-record). Empty =>
	// transcript recording is disabled (the Completer's capture is a no-op).
	RecordURL string
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
// mints under (it has no caller identity). The gateway requires a non-empty
// subject and uses it only for per-caller concurrency scoping + metrics — never
// for authorization — so a stable sentinel is correct and self-describing.
const systemExecSubject = "system:agent-session-completer"

func (s *Service) execEnabled() bool {
	return s.Exec != nil && len(s.Exec.Secret) > 0 && s.Exec.GatewayURL != ""
}

func (s *Service) recordEnabled() bool {
	return s.Exec != nil && len(s.Exec.Secret) > 0 && s.Exec.RecordURL != ""
}

// dialGateway authorizes the caller (can_operate on the sandbox's workspace),
// mints a signed exec ticket, and opens the gateway's SSE stream. It returns the
// live response (the caller drains + closes Body) or a pre-stream error, so a
// failure (authz, ticket, gateway down) surfaces before any bytes are written.
func (s *Service) dialGateway(ctx context.Context, req ExecRequest) (*http.Response, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
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
	if _, err := s.ownedSandbox(ctx, key, ws, req.SandboxID); err != nil {
		return nil, err
	}
	return s.mintAndDial(ctx, ws, req.SandboxID, []string{"/bin/sh", "-c", req.Command})
}

// mintAndDial signs an exec ticket binding the exact pod/namespace/command and
// opens the gateway SSE stream. It performs NO authorization or ownership check
// — every caller (dialGateway for the public verb, ReadSessionStatus for the
// trusted Completer) must gate the exact sandbox before calling it.
func (s *Service) mintAndDial(ctx context.Context, ws, sandboxID string, command []string) (*http.Response, error) {
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
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
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

// recordTranscriptTimeout bounds the synchronous recorder POST so a driver that
// never emits `[DONE]` cannot stall the Completer's teardown (which runs serially
// per session). A fire-and-forget turn's replay drains in seconds; this is the
// generous ceiling, distinct from the 4h browser attach timeout.
const recordTranscriptTimeout = 2 * time.Minute

// recordTranscript mints an agent-session ticket signed with the exec HMAC and
// POSTs the gateway's recorder endpoint (ADR051). Like ReadSessionStatus it runs
// under the system subject with no tenant authorization, and bex-api never gains
// pods/exec — the gateway alone dials the driver.
func (s *Service) recordTranscript(ctx context.Context, ws, sessionID, sandboxID string, turn int) error {
	if !s.recordEnabled() || !s.enabled() {
		return nil // recorder unconfigured: capture is a best-effort no-op
	}
	if ws == "" || sessionID == "" || sandboxID == "" {
		return fmt.Errorf("%w: workspace, session, and sandbox are required", core.ErrBadRequest)
	}
	ttl := s.Exec.TTL
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	now := s.Now()
	ticket, err := agentsessionticket.Mint(s.Exec.Secret, agentsessionticket.Claims{
		Subject:   systemExecSubject,
		SessionID: sessionID,
		SandboxID: sandboxID,
		Pod:       sandboxID + "-0", // deterministic, matches the attach mint
		Workspace: ws,
		Namespace: ws + "-sandbox",
		Turn:      turn,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, recordTranscriptTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Exec.RecordURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set(agentsessionticket.TicketHeader, ticket)
	client := s.Exec.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: transcript recorder unreachable", core.ErrSandboxesUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: transcript recorder returned %d", core.ErrSandboxesUnavailable, resp.StatusCode)
	}
	return nil
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
	buf := make([]byte, 4096)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return nil // client disconnected; nothing more to do
			}
			if flush != nil {
				flush()
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				return nil
			}
			return nil // upstream ended (context cancel/timeout); stream is closed
		}
	}
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
	var out ExecResult
	exitSeen := false
	total := 0 // combined stdout+stderr bytes accumulated so far
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
					if remaining := maxExecOutputBytes - total; len(data) > remaining {
						data = data[:remaining]
						out.Truncated = true
					}
					total += len(data)
					if ev.Stream == "stderr" {
						out.Stderr += data
					} else {
						out.Stdout += data
					}
					if out.Truncated {
						return out, nil
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
				}
				if json.Unmarshal([]byte(payload), &ev) == nil && ev.Error != "" {
					return out, fmt.Errorf("%w: %s", core.ErrSandboxesUnavailable, ev.Error)
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return out, fmt.Errorf("%w: sandbox exec stream failed: %v", core.ErrSandboxesUnavailable, err)
	}
	if !exitSeen {
		return out, fmt.Errorf("%w: sandbox exec stream ended without an exit event", core.ErrSandboxesUnavailable)
	}
	return out, nil
}
