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

// Package sandboxexec mints and verifies the short-lived ticket that authorizes
// one `render ea sandbox exec` command (w3/m33, docs/render-artifacts/ea-sandbox.md).
// It is a leaf, mirroring internal/shellticket: bex-api (which authorizes
// can_operate but must never gain pods/exec) mints a ticket that binds the exact
// sandbox pod, namespace, and command; the isolated SSH gateway (which holds
// pods/exec) verifies it and runs the command, streaming stdout/stderr back.
// Because the ticket is HMAC-SHA256-signed, the gateway needs no shared database
// — only the secret both processes hold (BEX_SANDBOX_EXEC_SECRET). Unlike the
// Browser Web Shell ticket, the COMMAND is signed: the gateway runs exactly what
// bex-api authorized, never an argv the caller could swap after the fact.
package sandboxexec

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const clockSkew = 30 * time.Second

// TicketHeader carries the signed exec ticket from bex-api to the gateway. A
// header (not a query param) keeps it out of edge access logs.
const TicketHeader = "X-Bex-Sandbox-Exec-Ticket"

var (
	// ErrMalformed is returned when a ticket is structurally invalid or missing a
	// required claim.
	ErrMalformed = errors.New("malformed sandbox exec ticket")
	// ErrSignature is returned when a ticket's HMAC does not verify.
	ErrSignature = errors.New("sandbox exec ticket signature mismatch")
	// ErrExpired is returned when a ticket is past its expiry (with clock skew).
	ErrExpired = errors.New("sandbox exec ticket expired")
)

// Claims binds an authenticated caller to one exec: a specific sandbox pod in a
// specific `<ws>-sandbox` namespace running one signed command, for a short
// window. bex-api fills Namespace after resolving the caller's workspace, so the
// gateway never has to map a sandbox id to a tenant itself — it trusts the
// signed namespace (only bex-api holds the secret) and runs there.
type Claims struct {
	Subject   string   `json:"sub"`          // authenticated caller
	SandboxID string   `json:"sbx"`          // OpenSandbox sandbox id (pod is <sbx>-0)
	Namespace string   `json:"ns"`           // the resolved <ws>-sandbox namespace
	Command   []string `json:"cmd"`          // the exact argv to run
	Workspace string   `json:"ws,omitempty"` // owning workspace (audit)
	IssuedAt  int64    `json:"iat"`          // unix seconds
	ExpiresAt int64    `json:"exp"`          // unix seconds
	Nonce     string   `json:"jti"`          // unique id for best-effort single-use
}

// Mint returns a signed, URL-safe ticket for claims, filling a fresh Nonce when
// empty so every ticket is single-use-trackable.
func Mint(secret []byte, claims Claims) (string, error) {
	if len(secret) == 0 {
		return "", errors.New("sandbox exec ticket secret is empty")
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

// Verify checks the signature and time bounds and returns the claims. Single-use
// is the caller's concern (the gateway tracks consumed nonces), so verify stays
// stateless.
func Verify(secret []byte, token string, now time.Time) (Claims, error) {
	if len(secret) == 0 {
		return Claims{}, errors.New("sandbox exec ticket secret is empty")
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
	if claims.Subject == "" || claims.SandboxID == "" || claims.Namespace == "" ||
		len(claims.Command) == 0 || claims.Nonce == "" || claims.ExpiresAt == 0 {
		return Claims{}, ErrMalformed
	}
	if now.Add(-clockSkew).After(time.Unix(claims.ExpiresAt, 0)) {
		return Claims{}, ErrExpired
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

// PodName is the sandbox's pod name: OpenSandbox names the workload pod
// `<sandbox-id>-0` (validated live, w3/m32).
func (c Claims) PodName() string { return c.SandboxID + "-0" }

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
