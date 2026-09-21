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
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/hmacticket"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

// The pinned Render CLI (v2.24.0 and later) runs a sandbox command in two
// steps (w7/m147, render-oss/cli pkg/sandbox/repo.go): it first mints a run
// connect token with `POST /v1/sandboxes/{id}/runs/{operation}/token`, then
// redeems that token — as the Bearer credential, NOT its OAuth token — at the
// `uri` bex returned, and reads the response as the exec SSE stream. The token
// is a short-lived, single-use HMAC ticket bound to the workspace, sandbox,
// execution id, operation, and the exact command, so the redeem route can be
// served outside the OAuth gate without becoming a second way to choose a
// command or a sandbox.
const (
	// ConnectOperationStream is the only run operation bex serves; the
	// pinned client sends it for `ea sandboxes exec`.
	ConnectOperationStream = "stream"
	// ConnectStreamPattern is the redeem route, mounted on the root mux OUTSIDE
	// the auth gate (internal/api/server.go) and classified in the always-public
	// inventory. Its Bearer is the connect token minted by ConnectRun.
	ConnectStreamPattern = "POST /v1/sandboxes/{id}/runs/{executionId}/stream"
	// connectTokenMaxTTL caps the token lifetime: the client redeems it
	// immediately, and a longer window only widens a leaked token's use.
	connectTokenMaxTTL = 60 * time.Second
	// connectKeyLabel domain-separates the connect-token key from the
	// gateway exec-ticket key derived from the same BEX_SANDBOX_EXEC_SECRET.
	// A connect token therefore never verifies as a gateway ticket, and a
	// gateway ticket never redeems here, even though one secret is configured.
	connectKeyLabel = "bex sandbox run connect token v1"
)

var connectCodec = hmacticket.New("sandbox connect token")

// connectClaims is the signed body of a run connect token. Every field the
// redeem route checks is here, so the token — not request-time state — is
// what authorizes exactly one stream of one command in one sandbox.
type connectClaims struct {
	Subject     string `json:"sub"`
	Workspace   string `json:"ws"`
	SandboxID   string `json:"sbx"`
	ExecutionID string `json:"exe"`
	Operation   string `json:"op"`
	Command     string `json:"cmd"`
	IssuedAt    int64  `json:"iat"`
	ExpiresAt   int64  `json:"exp"`
	Nonce       string `json:"nonce"`
}

// ConnectRequest is the mint input: OwnerID binds the workspace (Render's
// query/body ownerId), Operation is the path's `{operation}`, Command is the
// body's `command` — always sent by the pinned exec client, and required here
// because it is what the token binds.
type ConnectRequest struct {
	OwnerID   string
	SandboxID string
	Operation string
	Command   string
}

// ConnectResponse is Render's SandboxConnectResponse: the pinned client sends
// the exec body with Method to URI, carrying Token as its Bearer.
type ConnectResponse struct {
	ExecutionID string    `json:"executionId"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Method      string    `json:"method"`
	Token       string    `json:"token"`
	URI         string    `json:"uri"`
}

// connectKey derives the connect-token signing key from the gateway secret.
func (c *ExecConfig) connectKey() []byte {
	if c == nil || len(c.Secret) == 0 {
		return nil
	}
	mac := hmac.New(sha256.New, c.Secret)
	mac.Write([]byte(connectKeyLabel))
	return mac.Sum(nil)
}

// nonces returns the single-use guard for connect tokens. Production wires a
// store-backed guard (cross-replica); an unwired config falls back to a
// process-local guard so a token is still never redeemed twice on one pod.
func (c *ExecConfig) nonces() *sshgateway.NonceGuard {
	c.noncesOnce.Do(func() {
		if c.Nonces == nil {
			c.Nonces = &sshgateway.NonceGuard{}
		}
	})
	return c.Nonces
}

// connectTTL is the connect-token lifetime: the exec ticket TTL, capped.
func (c *ExecConfig) connectTTL() time.Duration {
	ttl := c.TTL
	if ttl <= 0 || ttl > connectTokenMaxTTL {
		ttl = connectTokenMaxTTL
	}
	return ttl
}

// ConnectRun mints a run connect token for one command in one sandbox. It
// applies exactly the exec gate (can_create, ownership, the agent-session
// can_view_sensitive rule) before anything is signed, so a refused caller
// gets no token. baseURL is the public API origin the returned URI is built
// on (the pinned client follows it verbatim).
func (s *Service) ConnectRun(ctx context.Context, req ConnectRequest, baseURL string) (ConnectResponse, error) {
	ws, raw, err := s.authorizeExecTarget(ctx, ExecRequest{OwnerID: req.OwnerID, SandboxID: req.SandboxID, Command: req.Command})
	if err != nil {
		return ConnectResponse{}, err
	}
	// Bind and advertise the canonical id whichever form the caller used, so the
	// token, the stream URI, and every other surface name one sandbox one way.
	sandboxID := canonicalID(raw)
	if req.Operation != ConnectOperationStream {
		return ConnectResponse{}, fmt.Errorf("%w: unsupported run operation %q (only %q is supported)", core.ErrBadRequest, req.Operation, ConnectOperationStream)
	}
	idn, ok := core.IdentityFrom(ctx)
	if !ok || idn.Subject == "" {
		return ConnectResponse{}, fmt.Errorf("%w: a connect token needs a caller identity", core.ErrForbidden)
	}
	now := s.Now()
	ttl := s.Exec.connectTTL()
	claims := connectClaims{
		Subject:     idn.Subject,
		Workspace:   ws,
		SandboxID:   sandboxID,
		ExecutionID: id.New(id.SandboxExecution),
		Operation:   req.Operation,
		Command:     req.Command,
		IssuedAt:    now.Unix(),
		ExpiresAt:   now.Add(ttl).Unix(),
	}
	if err := hmacticket.EnsureNonce(&claims.Nonce); err != nil {
		return ConnectResponse{}, err
	}
	token, err := connectCodec.Sign(s.Exec.connectKey(), claims)
	if err != nil {
		return ConnectResponse{}, err
	}
	return ConnectResponse{
		ExecutionID: claims.ExecutionID,
		ExpiresAt:   time.Unix(claims.ExpiresAt, 0).UTC(),
		Method:      http.MethodPost,
		Token:       token,
		URI:         connectStreamURI(baseURL, sandboxID, claims.ExecutionID),
	}, nil
}

func connectStreamURI(baseURL, sandboxID, executionID string) string {
	return strings.TrimRight(baseURL, "/") + "/v1/sandboxes/" + url.PathEscape(sandboxID) + "/runs/" + url.PathEscape(executionID) + "/stream"
}

// connectStatusError is a pre-stream redeem refusal with its exact HTTP status.
// The pinned client turns 401 and 403 into sentinels and surfaces every other
// status's `message`, so the status is part of the user-facing contract.
type connectStatusError struct {
	status int
	msg    string
}

func (e *connectStatusError) Error() string { return e.msg }

// verifyConnectToken checks a redeemed token against the route it was
// presented on and the body it was presented with. It consumes the nonce, so
// a token that passes here can never pass again on any replica. A token that
// was minted for another sandbox, another execution, or another command is
// refused BEFORE the nonce is consumed: misuse must not burn the legitimate
// redemption. The command comparison is exact, not normalized — the token
// binds bytes, and the gateway ticket is minted from the token's copy.
func (s *Service) verifyConnectToken(ctx context.Context, token, sandboxID, executionID, command string) (connectClaims, error) {
	var claims connectClaims
	if token == "" {
		return claims, &connectStatusError{http.StatusUnauthorized, "a sandbox run connect token is required as the Bearer credential"}
	}
	if err := connectCodec.Open(s.Exec.connectKey(), token, &claims); err != nil {
		return claims, &connectStatusError{http.StatusUnauthorized, "invalid sandbox run connect token"}
	}
	now := s.Now()
	if err := connectCodec.CheckBounds(now, claims.IssuedAt, claims.ExpiresAt); err != nil {
		if errors.Is(err, hmacticket.ErrExpired) {
			return claims, &connectStatusError{http.StatusGone, "sandbox run connect token expired; mint a new one"}
		}
		return claims, &connectStatusError{http.StatusUnauthorized, "invalid sandbox run connect token"}
	}
	if claims.Subject == "" || claims.Workspace == "" || claims.SandboxID == "" || claims.ExecutionID == "" || claims.Nonce == "" {
		return claims, &connectStatusError{http.StatusUnauthorized, "invalid sandbox run connect token"}
	}
	if claims.Operation != ConnectOperationStream || claims.SandboxID != sandboxID || claims.ExecutionID != executionID {
		return claims, &connectStatusError{http.StatusForbidden, "connect token was not minted for this sandbox run"}
	}
	if claims.Command != command {
		return claims, &connectStatusError{http.StatusForbidden, "command does not match the one this connect token was minted for"}
	}
	if !s.Exec.nonces().Consume(ctx, claims.Nonce, hmacticket.NonceExpiry(claims.ExpiresAt), now) {
		return claims, &connectStatusError{http.StatusConflict, "sandbox run connect token already used; mint a new one"}
	}
	return claims, nil
}

// ConnectStreamHandler serves ConnectStreamPattern: it redeems a run connect
// token and streams the bound command's exec. It runs OUTSIDE the OAuth gate,
// so it takes no identity from the request — the identity is the token's
// minter, and the exec gate (dialGateway) is applied again under that identity
// at redeem time: the sandbox must still exist in the bound workspace and the
// caller must still hold can_create. An OAuth access token presented here is
// just an unsigned string and is refused.
func (s *Service) ConnectStreamHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.execEnabled() || !s.enabled() {
			core.WriteErrStatus(w, http.StatusServiceUnavailable, "sandbox exec is not configured")
			return
		}
		var body struct {
			Command string `json:"command"`
		}
		if err := core.DecodeJSON(r, &body); err != nil && !errors.Is(err, io.EOF) {
			core.WriteErrStatus(w, http.StatusBadRequest, "malformed request body: "+err.Error())
			return
		}
		claims, err := s.verifyConnectToken(r.Context(), bearerToken(r), r.PathValue("id"), r.PathValue("executionId"), body.Command)
		if err != nil {
			var se *connectStatusError
			if errors.As(err, &se) {
				core.WriteErrStatus(w, se.status, se.msg)
				return
			}
			core.WriteErr(w, err)
			return
		}
		ctx := core.WithIdentity(r.Context(), core.Identity{Subject: claims.Subject, Method: "sandbox-connect"})
		flush := func() {
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if err := s.StreamExec(ctx, ExecRequest{OwnerID: claims.Workspace, SandboxID: claims.SandboxID, Command: claims.Command}, w, flush); err != nil {
			core.WriteErr(w, err)
		}
	})
}

func bearerToken(r *http.Request) string {
	scheme, token, ok := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// connectBaseURL is the origin the minted URI is built on: the configured
// public API URL (BEX_API_PUBLIC_URL) when set, else the origin this request
// arrived on — a client that reached the mint route can reach the same host.
func (s *Service) connectBaseURL(r *http.Request) string {
	if s.Exec != nil && s.Exec.PublicURL != "" {
		return s.Exec.PublicURL
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
