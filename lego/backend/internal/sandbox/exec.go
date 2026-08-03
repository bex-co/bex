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

func (s *Service) execEnabled() bool {
	return s.Exec != nil && len(s.Exec.Secret) > 0 && s.Exec.GatewayURL != ""
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
	subject := ""
	if idn, ok := core.IdentityFrom(ctx); ok {
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
}

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
					if ev.Stream == "stderr" {
						out.Stderr += ev.Data
					} else {
						out.Stdout += ev.Data
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
