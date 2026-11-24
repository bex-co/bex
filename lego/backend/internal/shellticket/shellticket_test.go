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
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/hmacticket"
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
// NonceExpiry (ExpiresAt + hmacticket.ClockSkew) must not fall before the last instant Verify
// accepts, or a nonce pruned at raw ExpiresAt could be re-claimed and a
// still-verifiable ticket replayed during the clock-skew interval.
func TestNonceExpiryCoversVerifyWindow(t *testing.T) {
	exp := time.Unix(1_800_000_000, 0)
	token := mint(t, Claims{Subject: "user-1", ServiceID: "srv-abc", ExpiresAt: exp.Unix()})
	lastAccepted := exp.Add(hmacticket.ClockSkew) // Verify accepts through ExpiresAt+hmacticket.ClockSkew.
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

// w10/m615: a ticket for an agent session's Terminal names the session and
// nothing else, so the gateway routes it by the field it names.
func TestSessionTicketNamesOneAgentSession(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	const session = "ags-d4ae2ab0f8cs73a9ejo0"
	claims := func(c Claims) Claims {
		c.Subject, c.IssuedAt, c.ExpiresAt = "user-1", now.Unix(), now.Add(time.Minute).Unix()
		return c
	}
	got, err := Verify(testSecret, mint(t, claims(Claims{SessionID: session, KeyAction: "sandboxes.exec"})), now)
	if err != nil {
		t.Fatalf("Verify a session ticket: %v", err)
	}
	if got.SessionID != session || got.ServiceID != "" || got.Username() != session || got.KeyAction != "sandboxes.exec" {
		t.Fatalf("session ticket = %+v (username %q), want the session alone", got, got.Username())
	}
	for name, c := range map[string]Claims{
		"no target":                   {},
		"a service and a session":     {ServiceID: "srv-d4ae2ab0f8cs73a9ejo0", SessionID: session},
		"an instance and a session":   {InstanceID: "srv-d4ae2ab0f8cs73a9ejo0-abcde", SessionID: session},
		"a service id as the session": {SessionID: "srv-d4ae2ab0f8cs73a9ejo0"},
		"not an id":                   {SessionID: "ags-nope"},
	} {
		if _, err := Verify(testSecret, mint(t, claims(c)), now); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: Verify = %v, want ErrMalformed", name, err)
		}
	}
}

// A service ticket's encoding is unchanged by the session field: an older
// gateway reads it exactly as before, and refuses a session ticket, which
// names no service, as malformed.
func TestServiceTicketEncodingIsUnchanged(t *testing.T) {
	token := mint(t, Claims{Subject: "user-1", ServiceID: "srv-abc", IssuedAt: 1, ExpiresAt: 2, Nonce: "n"})
	body, _, _ := strings.Cut(token, ".")
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"sub":"user-1","svc":"srv-abc","iat":1,"exp":2,"jti":"n"}`; string(raw) != want {
		t.Fatalf("service ticket body = %s, want %s", raw, want)
	}
}
