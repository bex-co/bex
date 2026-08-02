package agentsessionticket

import (
	"errors"
	"testing"
	"time"
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
