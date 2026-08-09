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

package sandboxexec

import (
	"errors"
	"testing"
	"time"
)

func TestMintVerifyRoundTrip(t *testing.T) {
	secret := []byte("sekret")
	now := time.Unix(1_800_000_000, 0)
	claims := Claims{
		Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox",
		Command: []string{"/bin/sh", "-c", "echo hi"}, Workspace: "tea-a",
		IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(),
	}
	tok, err := Mint(secret, claims)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Verify(secret, tok, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.SandboxID != "os-1" || got.Namespace != "tea-a-sandbox" || got.PodName() != "os-1-0" {
		t.Errorf("claims round-trip = %+v", got)
	}
	if len(got.Command) != 3 || got.Command[2] != "echo hi" {
		t.Errorf("command not preserved: %v", got.Command)
	}
	if got.Nonce == "" {
		t.Error("nonce not auto-filled")
	}
}

func TestVerifyRejectsTamperExpiryAndMissing(t *testing.T) {
	secret := []byte("sekret")
	now := time.Unix(1_800_000_000, 0)
	base := Claims{Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox",
		Command: []string{"sh"}, ExpiresAt: now.Add(time.Minute).Unix()}

	tok, _ := Mint(secret, base)
	// Wrong secret → signature mismatch.
	if _, err := Verify([]byte("other"), tok, now); !errors.Is(err, ErrSignature) {
		t.Errorf("tampered secret err = %v, want ErrSignature", err)
	}
	// Tampered body.
	if _, err := Verify(secret, "AAAA."+tok[len(tok)-10:], now); err == nil {
		t.Error("tampered body accepted")
	}
	// Expired.
	if _, err := Verify(secret, tok, now.Add(2*time.Minute)); !errors.Is(err, ErrExpired) {
		t.Errorf("expired err = %v, want ErrExpired", err)
	}
	// Missing required claim (no command).
	bad, _ := Mint(secret, Claims{Subject: "id-a", SandboxID: "os-1", Namespace: "ns", ExpiresAt: now.Add(time.Minute).Unix()})
	if _, err := Verify(secret, bad, now); !errors.Is(err, ErrMalformed) {
		t.Errorf("missing command err = %v, want ErrMalformed", err)
	}
}

// TestNonceExpiryCoversVerifyWindow pins codex #8 for sandbox-exec tickets: see
// the shellticket equivalent. The nonce must be retained at least as long as
// Verify accepts the ticket, or a still-verifiable ticket could replay during the
// clock-skew interval.
func TestNonceExpiryCoversVerifyWindow(t *testing.T) {
	secret := []byte("sekret")
	exp := time.Unix(1_800_000_000, 0)
	tok, err := Mint(secret, Claims{
		Subject: "id-a", SandboxID: "os-1", Namespace: "tea-a-sandbox",
		Command: []string{"/bin/sh", "-c", "echo hi"}, Workspace: "tea-a",
		ExpiresAt: exp.Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	lastAccepted := exp.Add(clockSkew)
	claims, err := Verify(secret, tok, lastAccepted)
	if err != nil {
		t.Fatalf("Verify at the last accepted instant: %v", err)
	}
	if claims.NonceExpiry().Before(lastAccepted) {
		t.Fatalf("NonceExpiry %v precedes the last verifiable instant %v — replay window open", claims.NonceExpiry(), lastAccepted)
	}
	if _, err := Verify(secret, tok, lastAccepted.Add(time.Second)); !errors.Is(err, ErrExpired) {
		t.Fatalf("Verify just past the window = %v, want ErrExpired", err)
	}
}
