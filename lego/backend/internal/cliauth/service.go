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

// Package cliauth adapts the official Render CLI's fixed OAuth wire protocol to
// Hydra. The CLI is unmodified: it speaks Render's JSON-at-form-content-type
// device endpoints while Hydra implements the RFC 8628 form endpoints.
package cliauth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const (
	// RenderCLIClientID is hard-coded by render-oss/cli. It is one permanent,
	// secretless platform client, never a tenant-created bex API key.
	RenderCLIClientID = "429024F5E608930E2A65EF92591A25CC"

	// MobileClientID is the permanent, secretless first-party native client
	// provisioned by scripts/auth-bootstrap-client.sh (ADR012 §8b).
	MobileClientID   = "bex-mobile"
	deviceGrantType  = "urn:ietf:params:oauth:grant-type:device_code"
	refreshGrantType = "refresh_token"
	maxUpstreamBody  = 1 << 20
	// DeviceGrantScope is what the CLI's device-authorization request asks Hydra
	// to bind onto the minted token. The two identity scopes alone (what the
	// official CLI would request for Render) leave the token with no granular
	// capability, so the dispatch-time scope matrix (RequireOpClass) refuses
	// every real API call with 403 → the CLI's "you are not allowed to take
	// this action". The three granular capabilities are what the Render-CLI
	// platform client is provisioned to allow (scripts/auth-bootstrap-client.sh)
	// with skip_consent, so requesting them here is what makes an ordinary
	// `bex login` able to list services (w8/m27, docs/ADR069-security-review-round14.md).
	// Exported so the composition root's end-to-end scope-matrix test drives the
	// exact scope the CLI receives, not a hand-copied duplicate.
	DeviceGrantScope = "openid offline_access " + core.ScopeRead + " " + core.ScopeWrite + " " + core.ScopeSensitive
	// One minute spans the official CLI TUI's concurrent refresh burst while
	// keeping a replay window no wider than Hydra's configured rotation grace.
	refreshIdempotencyTTL = time.Minute
)

// APIKeyRevoker is the existing machine-key self-revocation path. Keeping this
// narrow lets CLI logout preserve its historical behavior for RENDER_API_KEY
// callers without coupling the protocol adapter to the API-key implementation.
type APIKeyRevoker interface {
	RevokeAPIKey(context.Context, string, string) error
}

// RefreshIdempotencyStore serializes rotating-token refreshes across API
// replicas. The implementation owns the transaction and advisory lock. A
// duplicate on the same replica may receive cached response bytes; a duplicate
// on another replica may re-mint within Hydra's rotation grace period so live
// credentials never need to be persisted in the control-plane database.
type RefreshIdempotencyStore interface {
	IdempotentCLIRefresh(
		context.Context,
		[sha256.Size]byte,
		time.Duration,
		func(context.Context) ([]byte, int, error),
	) ([]byte, int, error)
}

type OAuthRevocationStore interface {
	BumpOAuthRevocation(context.Context, string, string) error
}

// Service owns the three public Render compatibility endpoints and the
// authenticated logout endpoint.
type Service struct {
	publicURL string
	adminURL  string
	client    *http.Client
	apiKeys   APIKeyRevoker
	// invalidate evicts the just-revoked access token from bex's positive
	// introspection cache so logout takes effect immediately, not after its
	// TTL. Injected by the composition root (the cache is the auth gate's).
	invalidate func(string, core.Identity)
	// Refreshes is the control-plane Postgres idempotency boundary for rotating
	// CLI refresh tokens. nil retains direct proxying for DB-free local mode.
	Refreshes RefreshIdempotencyStore
	// Revocations receives the dashboard's post-Hydra consent-revoke marker so
	// every bex-api replica invalidates its warm positive token cache.
	Revocations OAuthRevocationStore
	// RateLimiter guards the three public device-flow routes (w4/m31/t002).
	// nil (the New default) disables limiting — set post-construction from
	// the composition root once BEX_DEVICE_RATE_LIMIT is known, the same
	// convention Server.RateLimiter itself uses.
	RateLimiter *DeviceRateLimiter
}

// New returns a Render CLI authentication adapter. Empty URLs leave the routes
// mounted but honestly unavailable (503), which keeps server composition stable
// in partial local environments. invalidate may be nil (no cache to evict).
func New(publicURL, adminURL string, apiKeys APIKeyRevoker, invalidate func(string, core.Identity)) *Service {
	return &Service{
		publicURL:  strings.TrimSuffix(publicURL, "/"),
		adminURL:   strings.TrimSuffix(adminURL, "/"),
		client:     &http.Client{Timeout: 10 * time.Second, Transport: core.OryTransport},
		apiKeys:    apiKeys,
		invalidate: invalidate,
	}
}

// RegisterPublic mounts only the protocol initiation/exchange routes. They are
// public by design; every resource route and logout remain behind bex auth.
// Each route runs the IP-keyed device rate limiter (w4/m31/t002) before the
// composition root's bodyLimit middleware: a flood is shed without reading its
// body, while every admitted request is capped before JSON decoding or Hydra.
func (s *Service) RegisterPublic(mux *http.ServeMux, bodyLimit func(http.Handler) http.Handler) {
	mux.Handle("POST /v1/device-grant", s.rateLimited(bodyLimit(http.HandlerFunc(s.deviceGrant))))
	mux.Handle("POST /v1/device-token", s.rateLimited(bodyLimit(http.HandlerFunc(s.deviceToken))))
	mux.Handle("POST /v1/token/refresh/", s.rateLimited(bodyLimit(http.HandlerFunc(s.refreshToken))))
}

// rateLimited wraps handler with the device rate limiter check, never
// touching Hydra before the bucket check. A shed request gets an
// OAuth-dialect 429 with RFC 8628's slow_down code — the signal the official
// CLI's device-token poller (and any other RFC 8628 client) already knows to
// back off on — never Render's REST error shape (these are OAuth protocol
// routes, w9/done/m39's one deliberate dialect exception). s.RateLimiter nil
// (the default) disables limiting entirely — behavior identical to before.
func (s *Service) rateLimited(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.RateLimiter != nil {
			if ok, wait := s.RateLimiter.allow(r); !ok {
				w.Header().Set("Retry-After", strconv.Itoa(wait))
				writeOAuthError(w, http.StatusTooManyRequests, "slow_down")
				return
			}
		}
		handler.ServeHTTP(w, r)
	})
}

// RegisterREST contributes Render's authenticated POST /v1/oauth/revoke to the
// single REST router (inside the auth gate + rate limiter like every other
// /v1 resource route), so the composition root keeps one router and one
// wrapping site.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/oauth/revoke", s.revoke)
	mux.HandleFunc("POST /v1/oauth/revocations", s.recordRevocation)
}

func (s *Service) recordRevocation(w http.ResponseWriter, r *http.Request) {
	id, ok := core.IdentityFrom(r.Context())
	if !ok || !id.Human || id.Method != "session" {
		core.WriteErr(w, fmt.Errorf("%w: session authentication required", core.ErrForbidden))
		return
	}
	if s.Revocations == nil {
		core.WriteErr(w, core.ErrLogoutUnavailable)
		return
	}
	var in struct {
		ClientID string `json:"clientId"`
	}
	if !decodeRenderJSON(w, r, &in) {
		return
	}
	in.ClientID = strings.TrimSpace(in.ClientID)
	if in.ClientID == "" || len(in.ClientID) > 255 {
		core.WriteErr(w, fmt.Errorf("%w: invalid clientId", core.ErrBadRequest))
		return
	}
	if err := s.Revocations.BumpOAuthRevocation(r.Context(), id.Subject, in.ClientID); err != nil {
		core.WriteErr(w, core.ErrLogoutUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) revoke(w http.ResponseWriter, r *http.Request) {
	id, ok := core.IdentityFrom(r.Context())
	if !ok || id.Method != "oauth2" {
		core.WriteErr(w, fmt.Errorf("%w: no OAuth2 credential to revoke", core.ErrBadRequest))
		return
	}

	switch {
	case isPlatformHumanClient(id.ClientID) && id.Human:
		// /v1/oauth/revoke is a Render-shaped REST endpoint, not a token
		// endpoint — every failure branch speaks the one Render error dialect
		// via core.WriteErr (w9/m38, w9/008), not the OAuth {"error"} body the
		// RFC 8628 device endpoints use.
		if s.adminURL == "" {
			core.WriteErr(w, core.ErrLogoutUnavailable)
			return
		}
		q := url.Values{
			"subject": {id.Subject},
			"client":  {id.ClientID},
		}
		if err := core.DoJSON(r.Context(), s.client, http.MethodDelete,
			s.adminURL+"/admin/oauth2/auth/sessions/consent?"+q.Encode(), "", nil,
			http.StatusNoContent, nil); err != nil {
			core.WriteErr(w, core.ErrLogoutUnavailable)
			return
		}
	case !id.Human && id.ClientID != "" && id.Subject == id.ClientID:
		if s.apiKeys == nil {
			core.WriteErr(w, core.ErrAPIKeysUnavailable)
			return
		}
		if err := s.apiKeys.RevokeAPIKey(r.Context(), "", id.Subject); err != nil {
			core.WriteErr(w, err)
			return
		}
	default:
		core.WriteErr(w, fmt.Errorf("%w: unsupported OAuth2 credential", core.ErrBadRequest))
		return
	}

	if token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && token != "" && s.invalidate != nil {
		s.invalidate(token, id)
	}
	w.WriteHeader(http.StatusNoContent)
}

func isPlatformHumanClient(clientID string) bool {
	return clientID == RenderCLIClientID || clientID == MobileClientID
}

func (s *Service) deviceGrant(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClientID string `json:"client_id"`
	}
	if !decodeRenderJSON(w, r, &in) {
		return
	}
	if in.ClientID != RenderCLIClientID {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client")
		return
	}
	// Request the granular capabilities, not just identity (w7/037).
	//
	// This used to ask for "openid offline_access" alone and rely on the
	// platform-client exemption in core.CapabilityExempt to carry the CLI's
	// authority. 9081fbdb deliberately removed that exemption -- "Human OAuth
	// delegations are never exempt, including first-party public clients" -- but
	// nothing updated this request, so every CLI login received an identity-only
	// grant and then 403'd on every operation with INSUFFICIENT_SCOPE.
	//
	// Ask for what the Hydra client registration already advertises. That keeps
	// the hardening intact (no blanket exemption; the grant is explicit and shows
	// up in audit provenance) while making the token usable. Hydra narrows the
	// grant to the client's registered scope set, so this cannot request more
	// than the CLI is entitled to.
	s.proxyForm(w, r, "/oauth2/device/auth", url.Values{
		"client_id": {RenderCLIClientID},
		"scope":     {DeviceGrantScope},
	})
}

func (s *Service) deviceToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		GrantType  string `json:"grant_type"`
		ClientID   string `json:"client_id"`
		DeviceCode string `json:"device_code"`
	}
	if !decodeRenderJSON(w, r, &in) {
		return
	}
	if in.ClientID != RenderCLIClientID || in.GrantType != deviceGrantType || in.DeviceCode == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	s.proxyForm(w, r, "/oauth2/token", url.Values{
		"client_id":   {RenderCLIClientID},
		"grant_type":  {deviceGrantType},
		"device_code": {in.DeviceCode},
	})
}

func (s *Service) refreshToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		GrantType    string `json:"grant_type"`
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeRenderJSON(w, r, &in) {
		return
	}
	if in.GrantType != refreshGrantType || in.RefreshToken == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	form := url.Values{
		"client_id":     {RenderCLIClientID},
		"grant_type":    {refreshGrantType},
		"refresh_token": {in.RefreshToken},
	}
	if s.Refreshes == nil {
		s.proxyForm(w, r, "/oauth2/token", form)
		return
	}

	// Only the SHA-256 digest crosses the store boundary. The raw inbound
	// refresh token exists solely in this request and the one upstream form.
	tokenHash := sha256.Sum256([]byte(in.RefreshToken))
	body, status, err := s.Refreshes.IdempotentCLIRefresh(
		r.Context(), tokenHash, refreshIdempotencyTTL,
		func(ctx context.Context) ([]byte, int, error) {
			return s.postForm(ctx, "/oauth2/token", form)
		},
	)
	if err != nil {
		writeOAuthError(w, http.StatusServiceUnavailable, "temporarily_unavailable")
		return
	}
	writeProxyResponse(w, status, body)
}

func decodeRenderJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	if err := core.DecodeJSON(r, out); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request")
		return false
	}
	return true
}

func (s *Service) proxyForm(w http.ResponseWriter, r *http.Request, path string, form url.Values) {
	body, status, err := s.postForm(r.Context(), path, form)
	if err != nil {
		writeOAuthError(w, http.StatusServiceUnavailable, "temporarily_unavailable")
		return
	}
	writeProxyResponse(w, status, body)
}

func writeProxyResponse(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// postForm performs the upstream Hydra form POST and returns its verbatim
// body + status; any failure to reach or read Hydra is one error the caller
// maps to the single OAuth-shaped 503.
func (s *Service) postForm(ctx context.Context, path string, form url.Values) ([]byte, int, error) {
	if s.publicURL == "" {
		return nil, 0, fmt.Errorf("no public Hydra URL configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.publicURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer core.DrainClose(resp)
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamBody))
	if err != nil {
		return nil, 0, err
	}
	return body, resp.StatusCode, nil
}

// writeOAuthError writes the OAuth-shaped {"error":"<code>"} body. This is a
// deliberate survivor of the w9/m38 one-error-dialect sweep: the RFC 8628 device
// endpoints (deviceGrant/deviceToken/refreshToken) and their proxyForm path must
// stay OAuth-shaped because the unmodified official CLI parses their bodies as
// OAuth token responses. It is NOT used by /v1/oauth/revoke, which speaks the
// Render {"error","message","id"} dialect via core.WriteErr (w9/008).
func writeOAuthError(w http.ResponseWriter, status int, code string) {
	core.WriteJSON(w, status, map[string]string{"error": code})
}
