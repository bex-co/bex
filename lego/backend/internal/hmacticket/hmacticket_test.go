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

package hmacticket

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

var secret = []byte("test-secret")

type claims struct {
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// A flavor's sentinels keep their own verbatim message — the messages are what
// operators read in gateway logs, so folding three copies into one builder must
// not reword any of them.
func TestNewKeepsEachFlavorsMessages(t *testing.T) {
	for name, want := range map[string][4]string{
		"shell ticket": {
			"malformed shell ticket", "shell ticket signature mismatch",
			"shell ticket expired", "shell ticket secret is empty",
		},
		"sandbox exec ticket": {
			"malformed sandbox exec ticket", "sandbox exec ticket signature mismatch",
			"sandbox exec ticket expired", "sandbox exec ticket secret is empty",
		},
		"agent session ticket": {
			"malformed agent session ticket", "agent session ticket signature mismatch",
			"agent session ticket expired", "agent session ticket secret is empty",
		},
	} {
		c := New(name)
		got := [4]string{c.Malformed().Error(), c.Signature().Error(), c.Expired().Error(), c.NoSecret().Error()}
		if got != want {
			t.Errorf("New(%q) messages = %q, want %q", name, got, want)
		}
	}
}

// Each sentinel reports its shared class, so a caller can ask "was this
// expired?" without naming the flavor — while staying distinct from every other
// flavor's sentinel of the same class, so a per-flavor errors.Is check cannot
// match a foreign flavor's failure.
func TestSentinelsClassifyWithoutCollapsing(t *testing.T) {
	a, b := New("shell ticket"), New("sandbox exec ticket")
	for _, tc := range []struct {
		name   string
		err    error
		marker error
	}{
		{"malformed", a.Malformed(), ErrMalformed},
		{"signature", a.Signature(), ErrSignature},
		{"expired", a.Expired(), ErrExpired},
		{"no secret", a.NoSecret(), ErrNoSecret},
	} {
		if !errors.Is(tc.err, tc.marker) {
			t.Errorf("%s: errors.Is(flavor sentinel, marker) = false, want true", tc.name)
		}
	}
	for _, tc := range []struct {
		name string
		x, y error
	}{
		{"malformed", a.Malformed(), b.Malformed()},
		{"signature", a.Signature(), b.Signature()},
		{"expired", a.Expired(), b.Expired()},
		{"no secret", a.NoSecret(), b.NoSecret()},
	} {
		if errors.Is(tc.x, tc.y) || errors.Is(tc.y, tc.x) {
			t.Errorf("%s: two flavors' sentinels compare equal; a per-flavor check would match a foreign ticket", tc.name)
		}
	}
	// Two Codecs built from the SAME name must still be distinct values, exactly
	// like two errors.New calls sharing a message.
	if x, y := New("shell ticket"), New("shell ticket"); errors.Is(x.Expired(), y.Expired()) {
		t.Error("same-named flavors share a sentinel identity; want distinct values")
	}
}

// A Codec that was never built by New must REJECT, not return a nil error. Its
// sentinel fields are unexported precisely so this cannot be reached by setting
// them wrong; the accessors' fallback covers the zero value and a nil pointer.
// A nil error from Open reads to every caller as a verified ticket, which for
// these flavors means handing an unauthenticated request a pods/exec stream.
func TestCodecNotBuiltByNewFailsClosed(t *testing.T) {
	valid, err := New("test ticket").Sign(secret, claims{Subject: "user-1", ExpiresAt: 42})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	for _, tc := range []struct {
		name  string
		codec *Codec
	}{
		{"zero value", &Codec{}},
		{"nil pointer", nil},
	} {
		var got claims
		if err := tc.codec.Open(secret, "garbage-with-no-separator", &got); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: Open(garbage) = %v, want ErrMalformed", tc.name, err)
		}
		if err := tc.codec.Open([]byte("wrong"), valid, &got); !errors.Is(err, ErrSignature) {
			t.Errorf("%s: Open(wrong secret) = %v, want ErrSignature", tc.name, err)
		}
		if err := tc.codec.Open(nil, valid, &got); !errors.Is(err, ErrNoSecret) {
			t.Errorf("%s: Open(no secret) = %v, want ErrNoSecret", tc.name, err)
		}
		if _, err := tc.codec.Sign(nil, claims{}); !errors.Is(err, ErrNoSecret) {
			t.Errorf("%s: Sign(no secret) = %v, want ErrNoSecret", tc.name, err)
		}
		if err := tc.codec.CheckBounds(time.Unix(100, 0), 0, 1); !errors.Is(err, ErrExpired) {
			t.Errorf("%s: CheckBounds(expired) = %v, want ErrExpired", tc.name, err)
		}
	}
}

func TestSignOpenRoundTrip(t *testing.T) {
	c := New("test ticket")
	token, err := c.Sign(secret, claims{Subject: "user-1", ExpiresAt: 42})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	body, mac, ok := strings.Cut(token, ".")
	if !ok || body == "" || mac == "" {
		t.Fatalf("token %q is not <body>.<mac>", token)
	}
	// The body is a readable, unencrypted claim set: a ticket authenticates, it
	// does not conceal. Anything secret would leak here.
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		t.Fatalf("body is not base64url: %v", err)
	}
	var decoded claims
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("body is not JSON claims: %v", err)
	}
	var got claims
	if err := c.Open(secret, token, &got); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got != (claims{Subject: "user-1", ExpiresAt: 42}) {
		t.Errorf("round trip = %+v", got)
	}
}

func TestOpenRejectsEveryEnvelopeFailure(t *testing.T) {
	c := New("test ticket")
	token, err := c.Sign(secret, claims{Subject: "user-1", ExpiresAt: 42})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	body, mac, _ := strings.Cut(token, ".")

	// An attacker who can craft a payload but not the MAC gets Signature, never
	// a decoded claim set: the MAC is checked BEFORE the JSON decoder runs.
	forgedBody := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"attacker","exp":42}`))

	for _, tc := range []struct {
		name   string
		secret []byte
		token  string
		want   error
	}{
		{"unsigned payload", secret, forgedBody + "." + mac, ErrSignature},
		{"wrong secret", []byte("other"), token, ErrSignature},
		{"truncated mac", secret, body + "." + mac[:len(mac)-1], ErrSignature},
		{"no separator", secret, body + mac, ErrMalformed},
		{"empty body", secret, "." + mac, ErrMalformed},
		{"empty mac", secret, body + ".", ErrMalformed},
		{"empty token", secret, "", ErrMalformed},
		{"no secret", nil, token, ErrNoSecret},
	} {
		var got claims
		err := c.Open(tc.secret, tc.token, &got)
		if !errors.Is(err, tc.want) {
			t.Errorf("Open(%s) = %v, want %v", tc.name, err, tc.want)
		}
		if got != (claims{}) {
			t.Errorf("Open(%s) decoded %+v into the caller's claims despite failing", tc.name, got)
		}
	}

	// A correctly signed body that is not decodable is malformed, not a
	// signature failure — the ticket is authentic, its content is not usable.
	// Every case above fails before the decoder runs, so these two are what
	// actually exercise "nothing is left in the caller's claims": encoding/json
	// fills fields as it goes and reports a type error only afterwards, so a
	// half-decoded claim set would otherwise carry attacker-chosen values out of
	// a call that returned an error.
	for _, payload := range []any{
		json.RawMessage(`"not an object"`),
		json.RawMessage(`{"sub":"attacker","exp":"not-a-number"}`),
	} {
		junkToken, err := c.Sign(secret, payload)
		if err != nil {
			t.Fatalf("Sign: %v", err)
		}
		var got claims
		if err := c.Open(secret, junkToken, &got); !errors.Is(err, ErrMalformed) {
			t.Errorf("Open(%s) = %v, want ErrMalformed", payload, err)
		}
		if got != (claims{}) {
			t.Errorf("Open(%s) left %+v in the caller's claims despite failing", payload, got)
		}
	}
}

func TestSignRequiresASecret(t *testing.T) {
	c := New("test ticket")
	if _, err := c.Sign(nil, claims{Subject: "user-1"}); !errors.Is(err, ErrNoSecret) {
		t.Errorf("Sign(nil secret) = %v, want ErrNoSecret", err)
	}
	if _, err := c.Sign([]byte{}, claims{Subject: "user-1"}); !errors.Is(err, ErrNoSecret) {
		t.Errorf("Sign(empty secret) = %v, want ErrNoSecret", err)
	}
}

func TestCheckBoundsAllowsExactlyOneSkewWindow(t *testing.T) {
	c := New("test ticket")
	exp := time.Unix(1_800_000_000, 0)
	iat := exp.Add(-time.Minute)

	for _, tc := range []struct {
		name string
		now  time.Time
		iat  int64
		want error
	}{
		{"before expiry", exp.Add(-time.Second), iat.Unix(), nil},
		{"at expiry", exp, iat.Unix(), nil},
		{"within skew past expiry", exp.Add(ClockSkew - time.Second), iat.Unix(), nil},
		{"last accepted instant", exp.Add(ClockSkew), iat.Unix(), nil},
		{"one second past the window", exp.Add(ClockSkew + time.Second), iat.Unix(), ErrExpired},
		{"unset issuedAt is skipped", exp.Add(-time.Second), 0, nil},
		{"issued within skew ahead", exp.Add(-time.Minute), exp.Add(-time.Minute).Add(ClockSkew - time.Second).Unix(), nil},
		{"issued at the skew edge", exp.Add(-time.Minute), exp.Add(-time.Minute).Add(ClockSkew).Unix(), nil},
		{"issued beyond the skew", exp.Add(-time.Minute), exp.Add(-time.Minute).Add(ClockSkew + time.Second).Unix(), ErrMalformed},
	} {
		err := c.CheckBounds(tc.now, tc.iat, exp.Unix())
		if tc.want == nil && err != nil {
			t.Errorf("CheckBounds(%s) = %v, want nil", tc.name, err)
			continue
		}
		if tc.want != nil && !errors.Is(err, tc.want) {
			t.Errorf("CheckBounds(%s) = %v, want %v", tc.name, err, tc.want)
		}
	}

	// The future-dated rejection keeps the flavor's own message, so the log line
	// names the ticket kind and the reason.
	err := c.CheckBounds(exp.Add(-time.Minute), exp.Unix(), exp.Unix())
	if !strings.Contains(err.Error(), "malformed test ticket") || !strings.Contains(err.Error(), "issued in the future") {
		t.Errorf("future-dated error = %q, want the flavor message and the reason", err)
	}
}

// NonceExpiry must cover the verifier's whole acceptance window: a nonce pruned
// at the raw ExpiresAt could be re-claimed, replaying a still-verifiable ticket
// during the skew interval (codex #8).
func TestNonceExpiryCoversTheAcceptanceWindow(t *testing.T) {
	exp := time.Unix(1_800_000_000, 0)
	lastAccepted := exp.Add(ClockSkew)
	if got := NonceExpiry(exp.Unix()); got.Before(lastAccepted) {
		t.Errorf("NonceExpiry = %v, want >= the last accepted instant %v", got, lastAccepted)
	}
	if err := New("test ticket").CheckBounds(NonceExpiry(exp.Unix()), 0, exp.Unix()); err != nil {
		t.Errorf("a ticket is still accepted at its own NonceExpiry (%v) — pruning there is safe only if it is not", err)
	}
}

func TestNonceIsUniqueAndURLSafe(t *testing.T) {
	seen := make(map[string]bool, 128)
	for range 128 {
		n, err := Nonce()
		if err != nil {
			t.Fatalf("Nonce: %v", err)
		}
		if seen[n] {
			t.Fatalf("Nonce repeated %q; single-use tracking would collide", n)
		}
		seen[n] = true
		raw, err := base64.RawURLEncoding.DecodeString(n)
		if err != nil {
			t.Fatalf("Nonce %q is not base64url: %v", n, err)
		}
		if len(raw) != 16 {
			t.Fatalf("Nonce decodes to %d bytes, want 16", len(raw))
		}
		if strings.ContainsAny(n, "+/=") {
			t.Fatalf("Nonce %q is not URL-safe", n)
		}
	}
}

func TestEnsureNonceFillsOnlyWhenEmpty(t *testing.T) {
	n := ""
	if err := EnsureNonce(&n); err != nil {
		t.Fatalf("EnsureNonce: %v", err)
	}
	if n == "" {
		t.Fatal("EnsureNonce left the nonce empty; the ticket would not be single-use-trackable")
	}
	kept := n
	if err := EnsureNonce(&n); err != nil {
		t.Fatalf("EnsureNonce: %v", err)
	}
	if n != kept {
		t.Errorf("EnsureNonce overwrote a caller-supplied nonce %q with %q", kept, n)
	}
}
