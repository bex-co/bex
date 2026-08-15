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

// Package shellticket mints and verifies the short-lived exec ticket that
// authorizes a Browser Web Shell session (docs/ADR035-ssh.md § Browser Web
// Shell). It is a leaf: bex-api (which can authorize but must never gain
// pods/exec) mints a ticket after AuthorizeApp(can_operate); the isolated SSH
// gateway (which holds pods/exec) verifies it before opening a browser
// terminal. The ticket is an HMAC-SHA256-signed token (the envelope is
// internal/hmacticket, shared with the other ticket flavors) so verification
// needs no shared database — only the secret both processes hold
// (BEX_SHELL_TICKET_SECRET).
// It carries no terminal content and is not an SSH key; the private key never
// enters the browser.
package shellticket

import (
	"time"

	"github.com/bex-co/bex/lego/backend/internal/hmacticket"
)

var codec = hmacticket.New("shell ticket")

var (
	// ErrMalformed is returned when a ticket is structurally invalid or missing a
	// required claim.
	ErrMalformed = codec.Malformed()
	// ErrSignature is returned when a ticket's HMAC does not verify against the
	// secret (tampered, or minted with a different secret).
	ErrSignature = codec.Signature()
	// ErrExpired is returned when a ticket is past its expiry (with clock skew).
	ErrExpired = codec.Expired()
)

// Claims is the content of an exec ticket. It binds the authenticated caller to
// one service (and optionally one instance) for a short window. It deliberately
// carries no command, argv, or terminal content — the audit boundary in
// docs/ADR035-ssh.md holds for the browser path too.
type Claims struct {
	Subject    string `json:"sub"`            // authenticated caller (Kratos identity id)
	ServiceID  string `json:"svc"`            // public srv-… id the session targets
	InstanceID string `json:"inst,omitempty"` // optional specific srv-…-<suffix> replica
	IssuedAt   int64  `json:"iat"`            // unix seconds
	ExpiresAt  int64  `json:"exp"`            // unix seconds
	Nonce      string `json:"jti"`            // unique id for best-effort single-use
}

// Mint returns a signed, URL-safe ticket string for claims. When Nonce is empty
// it fills a fresh random one so every minted ticket is single-use-trackable.
func Mint(secret []byte, claims Claims) (string, error) {
	if err := hmacticket.EnsureNonce(&claims.Nonce); err != nil {
		return "", err
	}
	return codec.Sign(secret, claims)
}

// Verify checks the signature and time bounds and returns the claims. It does
// NOT check single-use — the gateway tracks consumed nonces itself, so an
// in-memory reuse guard survives a stateless verify.
func Verify(secret []byte, token string, now time.Time) (Claims, error) {
	var claims Claims
	if err := codec.Open(secret, token, &claims); err != nil {
		return Claims{}, err
	}
	if claims.Subject == "" || claims.ServiceID == "" || claims.Nonce == "" || claims.ExpiresAt == 0 {
		return Claims{}, ErrMalformed
	}
	if err := codec.CheckBounds(now, claims.IssuedAt, claims.ExpiresAt); err != nil {
		return Claims{}, err
	}
	return claims, nil
}

// NonceExpiry is the instant this ticket's single-use nonce may be pruned from
// the replay guard — the verifier's effective acceptance window, not the raw
// ExpiresAt (see hmacticket.NonceExpiry).
func (c Claims) NonceExpiry() time.Time { return hmacticket.NonceExpiry(c.ExpiresAt) }

// Username returns the SSH-style username the gateway feeds ResolveSSHSession:
// the specific instance id when the ticket pins one, else the bare service id
// (which selects a random Ready replica, matching native SSH).
func (c Claims) Username() string {
	if c.InstanceID != "" {
		return c.InstanceID
	}
	return c.ServiceID
}
