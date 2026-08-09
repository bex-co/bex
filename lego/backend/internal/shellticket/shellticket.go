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
// terminal. The ticket is an HMAC-SHA256-signed token so verification needs no
// shared database — only the secret both processes hold (BEX_SHELL_TICKET_SECRET).
// It carries no terminal content and is not an SSH key; the private key never
// enters the browser.
package shellticket

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// clockSkew tolerates small clock differences between bex-api and the gateway
// when checking the ticket's issue/expiry bounds.
const clockSkew = 30 * time.Second

var (
	// ErrMalformed is returned when a ticket is structurally invalid or missing a
	// required claim.
	ErrMalformed = errors.New("malformed shell ticket")
	// ErrSignature is returned when a ticket's HMAC does not verify against the
	// secret (tampered, or minted with a different secret).
	ErrSignature = errors.New("shell ticket signature mismatch")
	// ErrExpired is returned when a ticket is past its expiry (with clock skew).
	ErrExpired = errors.New("shell ticket expired")
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
	if len(secret) == 0 {
		return "", errors.New("shell ticket secret is empty")
	}
	if claims.Nonce == "" {
		nonce, err := newNonce()
		if err != nil {
			return "", err
		}
		claims.Nonce = nonce
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + sign(secret, body), nil
}

// Verify checks the signature and time bounds and returns the claims. It does
// NOT check single-use — the gateway tracks consumed nonces itself, so an
// in-memory reuse guard survives a stateless verify.
func Verify(secret []byte, token string, now time.Time) (Claims, error) {
	if len(secret) == 0 {
		return Claims{}, errors.New("shell ticket secret is empty")
	}
	body, sig, ok := strings.Cut(token, ".")
	if !ok || body == "" || sig == "" {
		return Claims{}, ErrMalformed
	}
	if !hmac.Equal([]byte(sig), []byte(sign(secret, body))) {
		return Claims{}, ErrSignature
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return Claims{}, ErrMalformed
	}
	var claims Claims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return Claims{}, ErrMalformed
	}
	if claims.Subject == "" || claims.ServiceID == "" || claims.Nonce == "" || claims.ExpiresAt == 0 {
		return Claims{}, ErrMalformed
	}
	if now.Add(-clockSkew).After(time.Unix(claims.ExpiresAt, 0)) {
		return Claims{}, ErrExpired
	}
	if claims.IssuedAt != 0 && now.Add(clockSkew).Before(time.Unix(claims.IssuedAt, 0)) {
		return Claims{}, fmt.Errorf("%w: issued in the future", ErrMalformed)
	}
	return claims, nil
}

// NonceExpiry is the instant this ticket's single-use nonce may be pruned from
// the replay guard. It matches the verifier's EFFECTIVE acceptance window
// (ExpiresAt + clockSkew), not the raw ExpiresAt — otherwise a still-verifiable
// ticket's nonce could be pruned and re-claimed during the skew interval,
// defeating single-use (codex #8).
func (c Claims) NonceExpiry() time.Time {
	return time.Unix(c.ExpiresAt, 0).Add(clockSkew)
}

// Username returns the SSH-style username the gateway feeds ResolveSSHSession:
// the specific instance id when the ticket pins one, else the bare service id
// (which selects a random Ready replica, matching native SSH).
func (c Claims) Username() string {
	if c.InstanceID != "" {
		return c.InstanceID
	}
	return c.ServiceID
}

func sign(secret []byte, body string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func newNonce() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
