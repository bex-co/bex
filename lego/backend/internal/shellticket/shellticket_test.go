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

package shellticket

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("shell-ticket-test-secret")

func mint(t *testing.T, claims Claims) string {
	t.Helper()
	token, err := Mint(testSecret, claims)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	return token
}

func TestMintVerifyRoundTrip(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	token := mint(t, Claims{
		Subject:   "user-1",
		ServiceID: "srv-abc",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(90 * time.Second).Unix(),
	})
	got, err := Verify(testSecret, token, now.Add(10*time.Second))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.Subject != "user-1" || got.ServiceID != "srv-abc" {
		t.Fatalf("claims round-tripped wrong: %+v", got)
	}
	if got.Nonce == "" {
		t.Error("Mint should fill a nonce so the gateway can enforce single-use")
	}
	if got.Username() != "srv-abc" {
		t.Errorf("Username() bare service = %q, want srv-abc", got.Username())
	}
}

func TestUsernamePrefersInstance(t *testing.T) {
	c := Claims{ServiceID: "srv-abc", InstanceID: "srv-abc-xyz"}
	if c.Username() != "srv-abc-xyz" {
		t.Errorf("Username() = %q, want the pinned instance", c.Username())
	}
}

func TestVerifyRejectsTamper(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	token := mint(t, Claims{Subject: "user-1", ServiceID: "srv-abc", IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()})
	body, _, _ := strings.Cut(token, ".")
	// Swap the signature for a forged one over a different body.
	forged := body + "." + strings.Repeat("A", 43)
	if _, err := Verify(testSecret, forged, now); !errors.Is(err, ErrSignature) {
		t.Errorf("forged signature => %v, want ErrSignature", err)
	}
	// A different secret must not verify a validly-structured token.
	if _, err := Verify([]byte("other-secret"), token, now); !errors.Is(err, ErrSignature) {
		t.Errorf("wrong secret => %v, want ErrSignature", err)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	token := mint(t, Claims{Subject: "user-1", ServiceID: "srv-abc", IssuedAt: now.Unix(), ExpiresAt: now.Add(30 * time.Second).Unix()})
	// Past expiry plus the clock-skew grace.
	if _, err := Verify(testSecret, token, now.Add(90*time.Second)); !errors.Is(err, ErrExpired) {
		t.Errorf("expired ticket => %v, want ErrExpired", err)
	}
	// Still valid within skew.
	if _, err := Verify(testSecret, token, now.Add(50*time.Second)); err != nil {
		t.Errorf("within-skew ticket => %v, want valid", err)
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	for _, tok := range []string{"", "nodot", ".", "a.", ".b", "not-base64!.sig"} {
		if _, err := Verify(testSecret, tok, now); err == nil {
			t.Errorf("Verify(%q) = nil, want an error", tok)
		}
	}
	// A well-signed token missing a required claim is malformed.
	token := mint(t, Claims{Subject: "", ServiceID: "srv-abc", ExpiresAt: now.Add(time.Minute).Unix()})
	if _, err := Verify(testSecret, token, now); !errors.Is(err, ErrMalformed) {
		t.Errorf("missing subject => %v, want ErrMalformed", err)
	}
}

func TestMintEmptySecret(t *testing.T) {
	if _, err := Mint(nil, Claims{Subject: "u", ServiceID: "srv-a"}); err == nil {
		t.Error("Mint with empty secret should error")
	}
}

// TestNonceExpiryCoversVerifyWindow pins codex #8: the replay guard must retain a
// ticket's single-use nonce at least as long as Verify still accepts the ticket.
// NonceExpiry (ExpiresAt + clockSkew) must not fall before the last instant Verify
// accepts, or a nonce pruned at raw ExpiresAt could be re-claimed and a
// still-verifiable ticket replayed during the clock-skew interval.
func TestNonceExpiryCoversVerifyWindow(t *testing.T) {
	exp := time.Unix(1_800_000_000, 0)
	token := mint(t, Claims{Subject: "user-1", ServiceID: "srv-abc", ExpiresAt: exp.Unix()})
	lastAccepted := exp.Add(clockSkew) // Verify accepts through ExpiresAt+clockSkew.
	claims, err := Verify(testSecret, token, lastAccepted)
	if err != nil {
		t.Fatalf("Verify at the last accepted instant: %v", err)
	}
	if claims.NonceExpiry().Before(lastAccepted) {
		t.Fatalf("NonceExpiry %v precedes the last verifiable instant %v — replay window open", claims.NonceExpiry(), lastAccepted)
	}
	if _, err := Verify(testSecret, token, lastAccepted.Add(time.Second)); !errors.Is(err, ErrExpired) {
		t.Fatalf("Verify just past the window = %v, want ErrExpired", err)
	}
}
