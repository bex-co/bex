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

// Package agentsessionticket is the signed handoff from bex-api to the coming
// isolated gateway attach path (ADR047 D3). It mirrors the Browser Web Shell
// design: a short-lived HMAC token plus a random nonce claimed atomically in
// the shared control-plane nonce table by the gateway. bex-api authorizes and
// signs but never connects to the sandbox network path.
package agentsessionticket

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

const (
	// ActionRead authorizes transcript replay only (GET requests).
	ActionRead = "read"
	// ActionTurn authorizes live prompt execution (POST requests).
	ActionTurn = "turn"
)

var (
	ErrMalformed     = errors.New("malformed agent session ticket")
	ErrSignature     = errors.New("agent session ticket signature mismatch")
	ErrExpired       = errors.New("agent session ticket expired")
	ErrInvalidAction = errors.New("agent session ticket action must be 'read' or 'turn'")
)

// TicketHeader carries a signed agent-session ticket on the transports that use
// this claim shape (the browser attach stream and the internal headless
// recorder). It lives here — with the claim type both sides marshal — so bex-api
// and the gateway share one symbol instead of duplicating the literal.
const TicketHeader = "X-Bex-Agent-Ticket"

// Claims bind one authenticated subject to one durable session and one exact
// sandbox pod in one workspace. Pod is explicit (rather than merely derivable)
// so the gateway never interprets a caller-controlled sandbox identifier.
type Claims struct {
	Subject   string `json:"sub"`
	SessionID string `json:"ses"`
	SandboxID string `json:"sbx"`
	Pod       string `json:"pod"`
	Workspace string `json:"ws"`
	Namespace string `json:"ns"`
	// Action is the authorized operation: "read" for transcript replay (GET),
	// "turn" for live prompt execution (POST). Required on new tickets; omitted
	// on legacy tickets (treated as "read" for compatibility).
	Action string `json:"act,omitempty"`
	// Turn is the session turn this ticket is scoped to (the session's turn
	// counter at mint time, ADR051). It is optional — omitted/zero on legacy
	// tickets — and lets the transcript tee/recorder stamp each part with its
	// turn so the headless recorder's per-turn idempotency guard is meaningful.
	Turn      int    `json:"trn,omitempty"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Nonce     string `json:"jti"`
}

func Mint(secret []byte, claims Claims) (string, error) {
	if len(secret) == 0 {
		return "", errors.New("agent session ticket secret is empty")
	}
	if claims.Nonce == "" {
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return "", err
		}
		claims.Nonce = base64.RawURLEncoding.EncodeToString(nonce[:])
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + sign(secret, body), nil
}

func Verify(secret []byte, token string, now time.Time) (Claims, error) {
	if len(secret) == 0 {
		return Claims{}, errors.New("agent session ticket secret is empty")
	}
	body, signature, ok := strings.Cut(token, ".")
	if !ok || body == "" || signature == "" || !hmac.Equal([]byte(signature), []byte(sign(secret, body))) {
		if ok && body != "" && signature != "" {
			return Claims{}, ErrSignature
		}
		return Claims{}, ErrMalformed
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return Claims{}, ErrMalformed
	}
	var claims Claims
	if json.Unmarshal(raw, &claims) != nil || claims.ExpiresAt == 0 {
		return Claims{}, ErrMalformed
	}
	// Every binding claim is required: a ticket missing any of them would not be
	// scoped to an exact subject, session, sandbox pod, and workspace.
	for _, required := range []string{
		claims.Subject, claims.SessionID, claims.SandboxID,
		claims.Pod, claims.Workspace, claims.Namespace, claims.Nonce,
	} {
		if required == "" {
			return Claims{}, ErrMalformed
		}
	}
	// An omitted action is "read" for legacy ticket compatibility; a present one
	// must name a known action.
	switch claims.Action {
	case "":
		claims.Action = ActionRead
	case ActionRead, ActionTurn:
	default:
		return Claims{}, ErrInvalidAction
	}
	if now.Add(-clockSkew).After(time.Unix(claims.ExpiresAt, 0)) {
		return Claims{}, ErrExpired
	}
	if claims.IssuedAt != 0 && now.Add(clockSkew).Before(time.Unix(claims.IssuedAt, 0)) {
		return Claims{}, ErrMalformed
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

func sign(secret []byte, body string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
