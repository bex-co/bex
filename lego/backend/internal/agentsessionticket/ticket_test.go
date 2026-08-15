package agentsessionticket

import (
	"errors"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/hmacticket"
)

func TestTicketClaimsAndTamperResistance(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	want := Claims{Subject: "alice", SessionID: "ags-session", SandboxID: "sandbox-1", Pod: "sandbox-1-0", Workspace: "tea-a", Namespace: "tea-a-sandbox", IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()}
	token, err := Mint([]byte("secret"), want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Verify([]byte("secret"), token, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != want.Subject || got.SessionID != want.SessionID || got.Pod != want.Pod || got.Workspace != want.Workspace || got.Nonce == "" {
		t.Fatalf("claims = %+v", got)
	}
	if _, err := Verify([]byte("other"), token, now); !errors.Is(err, ErrSignature) {
		t.Fatalf("wrong secret = %v, want signature error", err)
	}
	if _, err := Verify([]byte("secret"), token, now.Add(2*time.Minute)); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired = %v, want expiry error", err)
	}
}

// TestNonceExpiryCoversVerifyWindow pins codex #8 for agent-attach tickets: see
// the shellticket equivalent. The nonce must be retained at least as long as
// Verify accepts the ticket, or a still-verifiable ticket could replay during the
// clock-skew interval.
func TestNonceExpiryCoversVerifyWindow(t *testing.T) {
	exp := time.Unix(1_800_000_000, 0)
	tok, err := Mint([]byte("secret"), Claims{
		Subject: "alice", SessionID: "ags-session", SandboxID: "sandbox-1",
		Pod: "sandbox-1-0", Workspace: "tea-a", Namespace: "tea-a-sandbox",
		ExpiresAt: exp.Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	lastAccepted := exp.Add(hmacticket.ClockSkew)
	claims, err := Verify([]byte("secret"), tok, lastAccepted)
	if err != nil {
		t.Fatalf("Verify at the last accepted instant: %v", err)
	}
	if claims.NonceExpiry().Before(lastAccepted) {
		t.Fatalf("NonceExpiry %v precedes the last verifiable instant %v — replay window open", claims.NonceExpiry(), lastAccepted)
	}
	if _, err := Verify([]byte("secret"), tok, lastAccepted.Add(time.Second)); !errors.Is(err, ErrExpired) {
		t.Fatalf("Verify just past the window = %v, want ErrExpired", err)
	}
}
