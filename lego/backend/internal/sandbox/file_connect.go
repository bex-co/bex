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
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/hmacticket"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	ConnectFileUploadPattern   = "PUT /v1/sandboxes/{id}/files/upload/{executionId}"
	ConnectFileDownloadPattern = "GET /v1/sandboxes/{id}/files/download/{executionId}"
	fileConnectKeyLabel        = "bex sandbox file connect token v1"
)

var fileConnectCodec = hmacticket.New("sandbox file connect token")

// FileConnectRequest is the pinned CLI's query-only file-token mint input.
// Path is signed exactly as supplied; redemption cannot select another path.
type FileConnectRequest struct {
	OwnerID   string
	SandboxID string
	Operation string
	Path      string
}

type fileConnectClaims struct {
	Subject     string `json:"sub"`
	Workspace   string `json:"ws"`
	SandboxID   string `json:"sbx"`
	ExecutionID string `json:"exe"`
	Operation   string `json:"op"`
	Path        string `json:"path"`
	IssuedAt    int64  `json:"iat"`
	ExpiresAt   int64  `json:"exp"`
	Nonce       string `json:"nonce"`
}

func (c *ExecConfig) fileConnectKey() []byte {
	if c == nil || len(c.Secret) == 0 {
		return nil
	}
	mac := hmac.New(sha256.New, c.Secret)
	mac.Write([]byte(fileConnectKeyLabel))
	return mac.Sum(nil)
}

func (s *Service) fileAvailabilityError() error {
	if !s.enabled() || s.Exec == nil || len(s.Exec.Secret) == 0 || s.Exec.FileGatewayURL == "" {
		return core.Unavailable("sandbox file transfer is not configured")
	}
	if s.Exec.Nonces == nil || s.Exec.Nonces.Store == nil {
		return core.Unavailable("sandbox file transfers require a shared nonce store")
	}
	return nil
}

func fileOperationMethod(operation string) string {
	switch operation {
	case sandboxfiles.OperationUpload:
		return http.MethodPut
	case sandboxfiles.OperationDownload:
		return http.MethodGet
	default:
		return ""
	}
}

func validateFileConnectPath(value string) error {
	if err := sandboxfiles.ValidatePath(value); err != nil {
		return fmt.Errorf("%w: %v", core.ErrBadRequest, err)
	}
	return nil
}

// authorizeFileTarget preserves exec's owner/admin and agent-session boundary.
// Both reading credentials and writing executable content require can_create;
// the fresh check makes a role revoked since mint effective at redemption.
func (s *Service) authorizeFileTarget(ctx context.Context, req FileConnectRequest) (string, osSandbox, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return "", osSandbox{}, err
	}
	if err := s.fileAvailabilityError(); err != nil {
		return "", osSandbox{}, err
	}
	if req.SandboxID == "" || fileOperationMethod(req.Operation) == "" {
		return "", osSandbox{}, fmt.Errorf("%w: a sandbox and upload or download operation are required", core.ErrBadRequest)
	}
	if err := validateFileConnectPath(req.Path); err != nil {
		return "", osSandbox{}, err
	}
	ws, ok := s.Tenant(ctx)
	if !ok {
		return "", osSandbox{}, fmt.Errorf("%w: no workspace resolved for file transfer", core.ErrForbidden)
	}
	if err := s.AuthorizeFreshOn(ctx, core.RelCanCreate, core.WorkspaceObject(ws)); err != nil {
		return "", osSandbox{}, err
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return "", osSandbox{}, err
	}
	raw, err := s.ownedSandbox(ctx, key, ws, req.SandboxID)
	if err != nil {
		return "", osSandbox{}, err
	}
	if session := raw.Metadata[metadataAgentSession]; session != "" {
		if err := s.AuthorizeFreshOn(ctx, core.RelCanViewSensitive, agentsession.SessionObject(session)); err != nil {
			return "", osSandbox{}, err
		}
	}
	return ws, raw, nil
}

// ConnectFile mints a short-lived, single-use token for one exact file path.
// Only the returned transfer URI accepts it; run and gateway tickets use
// different signing keys even when all features share the configured secret.
func (s *Service) ConnectFile(ctx context.Context, req FileConnectRequest, baseURL string) (ConnectResponse, error) {
	ws, raw, err := s.authorizeFileTarget(ctx, req)
	if err != nil {
		return ConnectResponse{}, err
	}
	identity, ok := core.IdentityFrom(ctx)
	if !ok || identity.Subject == "" {
		return ConnectResponse{}, core.ErrForbidden
	}
	now := s.Now()
	claims := fileConnectClaims{
		Subject: identity.Subject, Workspace: ws, SandboxID: canonicalID(raw),
		ExecutionID: id.New(id.SandboxExecution), Operation: req.Operation, Path: req.Path,
		IssuedAt: now.Unix(), ExpiresAt: now.Add(s.Exec.connectTTL()).Unix(),
	}
	if err := hmacticket.EnsureNonce(&claims.Nonce); err != nil {
		return ConnectResponse{}, err
	}
	token, err := fileConnectCodec.Sign(s.Exec.fileConnectKey(), claims)
	if err != nil {
		return ConnectResponse{}, err
	}
	return ConnectResponse{
		ExecutionID: claims.ExecutionID, ExpiresAt: time.Unix(claims.ExpiresAt, 0).UTC(),
		Method: fileOperationMethod(claims.Operation), Token: token,
		URI: strings.TrimRight(baseURL, "/") + "/v1/sandboxes/" + url.PathEscape(claims.SandboxID) + "/files/" + claims.Operation + "/" + url.PathEscape(claims.ExecutionID) + "?" + url.Values{"path": {claims.Path}}.Encode(),
	}, nil
}

func (s *Service) verifyFileConnectToken(ctx context.Context, r *http.Request) (fileConnectClaims, error) {
	var claims fileConnectClaims
	if err := fileConnectCodec.Open(s.Exec.fileConnectKey(), bearerToken(r), &claims); err != nil {
		return claims, &connectStatusError{http.StatusUnauthorized, "invalid sandbox file connect token"}
	}
	now := s.Now()
	if err := fileConnectCodec.CheckBounds(now, claims.IssuedAt, claims.ExpiresAt); err != nil {
		if errors.Is(err, hmacticket.ErrExpired) {
			return claims, &connectStatusError{http.StatusGone, "sandbox file connect token expired; mint a new one"}
		}
		return claims, &connectStatusError{http.StatusUnauthorized, "invalid sandbox file connect token"}
	}
	if claims.Subject == "" || claims.Workspace == "" || claims.SandboxID == "" || claims.ExecutionID == "" || claims.Nonce == "" || validateFileConnectPath(claims.Path) != nil || fileOperationMethod(claims.Operation) == "" || claims.ExpiresAt-claims.IssuedAt > int64(connectTokenMaxTTL/time.Second) {
		return claims, &connectStatusError{http.StatusUnauthorized, "invalid sandbox file connect token"}
	}
	paths := r.URL.Query()["path"]
	if claims.SandboxID != r.PathValue("id") || claims.ExecutionID != r.PathValue("executionId") || fileOperationMethod(claims.Operation) != r.Method || len(paths) != 1 || claims.Path != paths[0] {
		return claims, &connectStatusError{http.StatusForbidden, "connect token was not minted for this sandbox file operation and path"}
	}
	if !s.Exec.Nonces.Consume(ctx, claims.Nonce, hmacticket.NonceExpiry(claims.ExpiresAt), now) {
		return claims, &connectStatusError{http.StatusConflict, "sandbox file connect token already used; mint a new one"}
	}
	return claims, nil
}

// ConnectFileHandler is mounted outside OAuth on the two transfer routes.
// The token supplies the identity; ordinary session/OAuth credentials confer
// no authority here. Reauthorization precedes body consumption and execution.
func (s *Service) ConnectFileHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := s.fileAvailabilityError(); err != nil {
			core.WriteErr(w, err)
			return
		}
		claims, err := s.verifyFileConnectToken(r.Context(), r)
		if err != nil {
			var status *connectStatusError
			if errors.As(err, &status) {
				core.WriteErrStatus(w, status.status, status.msg)
			} else {
				core.WriteErr(w, err)
			}
			return
		}
		ctx := core.WithIdentity(r.Context(), core.Identity{Subject: claims.Subject, Method: "sandbox-file-connect"})
		ctx, cancel := context.WithTimeout(ctx, sandboxfiles.DefaultTransferTimeout)
		defer cancel()
		// A canceled proxy context stops the gateway request, but alone cannot
		// interrupt a client that stalls its upload or stops reading a download.
		// Bound both directions on the underlying HTTP connection as well.
		controller := http.NewResponseController(w)
		deadline, _ := ctx.Deadline()
		_ = controller.SetReadDeadline(deadline)
		_ = controller.SetWriteDeadline(deadline)
		defer func() {
			_ = controller.SetReadDeadline(time.Time{})
			_ = controller.SetWriteDeadline(time.Time{})
		}()
		ws, raw, err := s.authorizeFileTarget(ctx, FileConnectRequest{OwnerID: claims.Workspace, SandboxID: claims.SandboxID, Operation: claims.Operation, Path: claims.Path})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		now := s.Now()
		ticket, err := sandboxfiles.Mint(s.Exec.Secret, sandboxfiles.Claims{
			Subject: claims.Subject, Workspace: ws, SandboxID: raw.ID, Namespace: store.SandboxNamespace(ws),
			AgentSessionID: raw.Metadata[metadataAgentSession], Operation: claims.Operation, Path: claims.Path,
			IssuedAt: now.Unix(), ExpiresAt: now.Add(s.Exec.connectTTL()).Unix(),
		})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		gateway, err := url.Parse(s.Exec.FileGatewayURL)
		if err != nil || (gateway.Scheme != "http" && gateway.Scheme != "https") || gateway.Host == "" {
			core.WriteErr(w, core.ErrSandboxesUnavailable)
			return
		}
		proxy := &httputil.ReverseProxy{
			Rewrite: func(p *httputil.ProxyRequest) {
				p.Out.URL = gateway
				p.Out.Host = gateway.Host
				p.Out.Header = make(http.Header)
				p.Out.Header.Set(sandboxfiles.TicketHeader, ticket)
				// The CLI checks the gzip trailer. Keep compressed bytes intact;
				// net/http otherwise transparently decompresses a GET response.
				p.Out.Header.Set("Accept-Encoding", "gzip")
				for _, name := range []string{"Content-Type", "Content-Encoding"} {
					if value := p.In.Header.Get(name); value != "" {
						p.Out.Header.Set(name, value)
					}
				}
			},
			ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
				core.WriteErrStatus(w, http.StatusServiceUnavailable, "sandbox file gateway unavailable")
			},
			FlushInterval: -1,
		}
		if s.Exec.Client != nil {
			proxy.Transport = s.Exec.Client.Transport
		}
		proxy.ServeHTTP(w, r.WithContext(ctx))
	})
}
