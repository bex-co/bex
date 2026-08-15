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

// Package drivergrant signs one-shot, action-bound calls from the trusted SSH
// gateway to an in-sandbox agent driver. The driver receives only the derived
// Ed25519 public key; the HMAC ticket secret and signing key remain outside the
// sandbox, so same-Pod tenant code cannot launch a credential-bearing turn by
// calling localhost directly.
package drivergrant

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/hmacticket"
)

const (
	Header     = "X-Bex-Driver-Grant"
	ActionTurn = "turn"
	domain     = "bex-agent-driver-grant/v1"
)

type Claims struct {
	SessionID string `json:"ses"`
	Action    string `json:"act"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Nonce     string `json:"jti"`
}

// PublicKey returns the raw Ed25519 verification key as base64url. Derivation
// is domain-separated from every HMAC ticket use of the shared gateway secret.
func PublicKey(secret []byte) (string, error) {
	if len(secret) == 0 {
		return "", errors.New("driver grant secret is empty")
	}
	public := signingKey(secret).Public().(ed25519.PublicKey)
	return base64.RawURLEncoding.EncodeToString(public), nil
}

// Mint creates a short-lived single-action grant. The driver additionally
// consumes jti once, so retrying or replaying a captured gateway request fails.
func Mint(secret []byte, sessionID string, now time.Time, ttl time.Duration) (string, error) {
	if len(secret) == 0 || sessionID == "" || ttl <= 0 {
		return "", errors.New("invalid driver grant configuration")
	}
	nonce, err := hmacticket.Nonce()
	if err != nil {
		return "", err
	}
	claims := Claims{
		SessionID: sessionID,
		Action:    ActionTurn,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
		Nonce:     nonce,
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	signature := ed25519.Sign(signingKey(secret), []byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func signingKey(secret []byte) ed25519.PrivateKey {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(domain))
	return ed25519.NewKeyFromSeed(mac.Sum(nil))
}
