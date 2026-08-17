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

// TestPodTripleAllOrNothing pins the ADR065 D2 replay-only ticket shape: the
// pod triple (sandbox/pod/namespace) is either fully present (a normal attach
// ticket) or fully empty (a replay-only READ ticket for a reaped session). A
// partially-filled triple is malformed, and a turn ticket must always bind a pod.
func TestPodTripleAllOrNothing(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	base := Claims{Subject: "alice", SessionID: "ags-session", Workspace: "tea-a",
		IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()}

	// Fully-empty triple + read: valid (replay-only).
	replay := base
	replay.Action = ActionRead
	tok, err := Mint([]byte("secret"), replay)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Verify([]byte("secret"), tok, now)
	if err != nil || got.Pod != "" || got.SandboxID != "" || got.Namespace != "" {
		t.Fatalf("replay-only verify = %+v err=%v", got, err)
	}

	// Fully-empty triple + turn: malformed (a turn must bind a pod).
	turn := base
	turn.Action = ActionTurn
	tok, err = Mint([]byte("secret"), turn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify([]byte("secret"), tok, now); !errors.Is(err, ErrMalformed) {
		t.Fatalf("pod-less turn ticket = %v, want ErrMalformed", err)
	}

	// Legacy empty action on a pod-less ticket defaults to read: valid.
	legacy := base
	tok, err = Mint([]byte("secret"), legacy)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Verify([]byte("secret"), tok, now); err != nil || got.Action != ActionRead {
		t.Fatalf("legacy pod-less verify = %+v err=%v", got, err)
	}

	// A partially-filled triple is malformed in every combination.
	for _, mutate := range []func(*Claims){
		func(c *Claims) { c.SandboxID = "sandbox-1" },
		func(c *Claims) { c.Pod = "sandbox-1-0" },
		func(c *Claims) { c.Namespace = "tea-a-sandbox" },
		func(c *Claims) { c.SandboxID, c.Pod = "sandbox-1", "sandbox-1-0" },
	} {
		partial := base
		partial.Action = ActionRead
		mutate(&partial)
		tok, err := Mint([]byte("secret"), partial)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Verify([]byte("secret"), tok, now); !errors.Is(err, ErrMalformed) {
			t.Fatalf("partial pod triple %+v = %v, want ErrMalformed", partial, err)
		}
	}
}
