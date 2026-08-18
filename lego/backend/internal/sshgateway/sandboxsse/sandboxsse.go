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

// Package sandboxsse is the sandbox-exec transport of the isolated SSH
// gateway (w3/m33, docs/render-artifacts/ea-sandbox.md §exec). `render ea
// sandbox exec` POSTs to bex-api, which authorizes can_operate on the
// sandbox, mints a signed sandboxexec ticket binding the exact pod/namespace/
// command, and reverse-proxies to this endpoint. The gateway verifies the
// ticket, runs the command in the sandbox pod via one pods/exec stream, and
// emits the Render CLI's SSE event shape: `data: {"stdout"|"stderr":"…"}`
// output chunks then a terminal `data: {"exitCode":N}`. pods/exec stays
// confined to the gateway process; the ticket's HMAC is the trust (only
// bex-api holds the secret), and the signed argv means the gateway runs
// exactly what bex-api authorized. (The name avoids colliding with
// internal/sandboxexec, the ticket package.)
package sandboxsse

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

// Server is the sandbox-exec SSE handler. Share Limits and Nonces (and
// Metrics) with the other gateway transports so session caps and exec-ticket
// single-use hold process-wide, not per feature.
type Server struct {
	// Secret must equal bex-api's BEX_SANDBOX_EXEC_SECRET so the gateway can
	// verify the exec tickets bex-api mints after authorizing can_operate on a
	// sandbox. Empty => the transport is disabled and every request is refused
	// with 503.
	Secret []byte

	Executor  sshgateway.Executor
	Metrics   *sshgateway.Metrics
	Limits    *sshgateway.SessionLimiter
	Nonces    *sshgateway.NonceGuard
	Revalidator Revalidator

	SessionTimeout    time.Duration
	RevalidateInterval time.Duration
}

// Revalidator re-runs a verified ticket's authorization at redemption and on
// every live-stream revalidation tick (round-13 #3). Any error refuses the
// exchange / ends the stream. The claims — never the request — supply the
// subject, workspace, sandbox, and agent-session binding.
type Revalidator interface {
	RevalidateExec(ctx context.Context, claims sandboxexec.Claims) error
}

// ExecRevalidator is the authorization kernel implementing Revalidator: fresh
// can_operate on the ticket's workspace for its subject, plus — when the
// sandbox is agent-session-bound — the session object's stronger
// can_view_sensitive (round-13 #1, the same relation the dedicated ags-… SSH
// path enforces). Platform-internal tickets (the Completer's system sentinel)
// are exempt: their authority is the HMAC itself, minted by trusted system
// loops with no caller identity to re-check. Like agentsessions.AttachRevalidator
// it holds only the kernel — no ticket minting, no store mutation.
type ExecRevalidator struct {
	*core.Base
}

// RevalidateExec re-authorizes a verified exec ticket as its signed subject,
// uncached (a positive decision cached on another replica must not outlive a
// revocation — the round-5 finding-4 class this transport joins late).
func (r *ExecRevalidator) RevalidateExec(ctx context.Context, claims sandboxexec.Claims) error {
	if r == nil || r.Base == nil {
		return nil // authorization kernel not wired: admission-only (pre-round-13)
	}
	if claims.Subject == "" || claims.Subject == sandboxexec.SystemSubject {
		return nil
	}
	if claims.Workspace == "" {
		return core.ErrForbidden // a caller ticket always carries its workspace
	}
	ctx = core.WithIdentity(ctx, core.Identity{Subject: claims.Subject, Method: "sandbox-exec"})
	if err := r.AuthorizeFreshOn(ctx, core.RelCanOperate, core.WorkspaceObject(claims.Workspace)); err != nil {
		return err
	}
	if claims.AgentSessionID != "" {
		return r.AuthorizeFreshOn(ctx, core.RelCanViewSensitive, agentsession.SessionObject(claims.AgentSessionID))
	}
	return nil
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

// Enabled reports whether the sandbox-exec transport is configured.
func (s *Server) Enabled() bool { return len(s.Secret) > 0 }

// Handler returns the HTTP handler for the sandbox-exec SSE stream. Mount it
// on the gateway's internal listener (reachable only from bex-api).
func (s *Server) Handler() http.Handler {
	s.defaults()
	return http.HandlerFunc(s.serveSandboxExec)
}

func (s *Server) serveSandboxExec(w http.ResponseWriter, r *http.Request) {
	if !s.Enabled() {
		http.Error(w, "sandbox exec not configured", http.StatusServiceUnavailable)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	claims, err := sandboxexec.Verify(s.Secret, r.Header.Get(sandboxexec.TicketHeader), time.Now())
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

	// Cap the number of concurrent exec streams per caller, like the shell path.
	if acquired, scope := s.Limits.Acquire(claims.Subject); !acquired {
		s.Metrics.LimitRejected(scope)
		http.Error(w, "session limit reached", http.StatusTooManyRequests)
		return
	}
	defer s.Limits.Release(claims.Subject)

	// Redemption-time reauthorization (round-13 #3): the ticket froze bex-api's
	// mint-time decision; re-run it here as the ticket's subject so a revocation
	// inside the ticket's TTL+skew window is effective at the gateway too (the
	// agentattach redemption pattern, round-6 #11). Refused before the SSE
	// headers so the CLI sees a clean HTTP status.
	if s.Revalidator != nil {
		if err := s.Revalidator.RevalidateExec(r.Context(), claims); err != nil {
			s.Metrics.Authentication("rejected_target")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}
	s.Metrics.Authentication("accepted")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sse := &sseWriter{w: w, flusher: flusher}
	// Live-stream revalidation (round-13 #3): without this watchdog an admitted
	// exec ran to its disconnect or the 4h cap, so a membership/role revocation
	// only took effect at the NEXT admission — the exact gap web shell, native
	// SSH, and agent attach closed in round-9 #6. The watchdog re-runs the same
	// redemption check every interval and cancels the exec context on refusal;
	// a canceled execCtx under a live request context is unambiguously revoked.
	timedCtx, cancelTimeout := context.WithTimeout(r.Context(), s.SessionTimeout)
	defer cancelTimeout()
	execCtx, cancelExec := sshgateway.WithRevalidation(timedCtx, s.RevalidateInterval, func(c context.Context) error {
		if s.Revalidator == nil {
			return nil
		}
		return s.Revalidator.RevalidateExec(c, claims)
	})
	defer cancelExec()

	target := apps.SSHInstanceTarget{
		PodName:   claims.PodName(),
		Namespace: claims.Namespace,
		Container: "sandbox", // OpenSandbox names the workload container "sandbox"
		ServiceID: claims.SandboxID,
		OwnerID:   claims.Workspace,
	}
	// No TTY: separate stdout/stderr so each maps to its own SSE output stream; no
	// stdin — `ea sandbox exec` runs one non-interactive command.
	code, err := s.Executor.Execute(execCtx, target, claims.Command, false, nil, nil,
		sse.stream("stdout"), sse.stream("stderr"))
	switch {
	case err != nil && execCtx.Err() != nil && timedCtx.Err() == nil:
		// The watchdog ended the stream (revocation), not the client or the cap.
		log.Printf("sandbox exec revoked mid-stream (sandbox=%s subject=%s): %v", claims.SandboxID, claims.Subject, err)
		s.Metrics.Authentication("revoked")
		sse.emit("error", errorEvent{Error: "access was revoked during this exec"})
	case err != nil:
		log.Printf("sandbox exec stream failed (sandbox=%s): %v", claims.SandboxID, err)
		sse.emit("error", errorEvent{Error: "exec failed to start in this sandbox"})
	default:
		sse.emit("exit", exitEvent{ExitCode: sshgateway.ClampExit(code)})
	}
}

// The Render CLI reads the SSE `event:` line to discriminate, then unmarshals the
// `data:` JSON (render-oss/cli pkg/sandbox/sse.go): `event: output` carries
// {stream, data}, `event: exit` carries {exitCode}, `event: error` carries a
// message. bex mirrors that shape exactly so the unmodified CLI parses it.
type outputEvent struct {
	Stream string `json:"stream"` // "stdout" | "stderr"
	Data   string `json:"data"`
}
type exitEvent struct {
	ExitCode int `json:"exitCode"`
}
type errorEvent struct {
	Error string `json:"error"`
}

// sseWriter serializes SSE writes to one HTTP response (output chunks and the
// terminal event can otherwise interleave mid-line).
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
}

// stream returns an io.Writer that emits each write as an `output` SSE event of
// the given kind ("stdout" or "stderr").
func (s *sseWriter) stream(kind string) *sseStream { return &sseStream{sse: s, kind: kind} }

// emit writes one SSE event: an `event:` name line then a `data:` JSON line.
func (s *sseWriter) emit(event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.w.Write([]byte("event: "))
	_, _ = s.w.Write([]byte(event))
	_, _ = s.w.Write([]byte("\ndata: "))
	_, _ = s.w.Write(data)
	_, _ = s.w.Write([]byte("\n\n"))
	s.flusher.Flush()
}

type sseStream struct {
	sse  *sseWriter
	kind string
}

func (s *sseStream) Write(p []byte) (int, error) {
	s.sse.emit("output", outputEvent{Stream: s.kind, Data: string(p)})
	return len(p), nil
}
