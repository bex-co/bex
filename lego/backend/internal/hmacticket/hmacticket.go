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

// Package hmacticket is the signed-token envelope every bex ticket flavor
// shares. bex-api authorizes a verb and mints a short-lived token; an isolated
// process that holds a capability bex-api must never gain (pods/exec, the
// sandbox network path) verifies it. Because the token is HMAC-SHA256-signed,
// verification needs no shared database — only the secret both processes hold.
//
// A flavor package (internal/shellticket, internal/sandboxexec,
// internal/agentsessionticket) owns its Claims shape, its required-claim rules,
// and its error vocabulary; everything below the claims — the wire format, the
// constant-time signature check, the clock-skew bounds, the nonce — lives here
// once, so a rule like "a ticket dated in the future is refused" cannot hold in
// one flavor and be missing from another.
//
// The wire format is "<body>.<mac>", where body is base64url(JSON(claims)) and
// mac is base64url(HMAC-SHA256(secret, body)). It is deliberately NOT a JWT: no
// algorithm is named in the token, so there is no algorithm to confuse.
package hmacticket

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// ClockSkew is how far bex-api's and the verifier's clocks may differ before a
// ticket's time bounds are judged violated. Every flavor shares it: a nonce
// replay guard sized to one flavor's window must be correct for all of them.
const ClockSkew = 30 * time.Second

// The markers classify a rejection across every flavor, so a caller can ask
// "was this expired?" without naming the flavor that produced it. A flavor's
// own sentinels (built by New) wrap these while keeping their own message.
var (
	// ErrMalformed marks a token that is structurally invalid or missing a
	// required claim.
	ErrMalformed = errors.New("malformed ticket")
	// ErrSignature marks a token whose MAC does not verify against the secret —
	// tampered, or minted with a different secret.
	ErrSignature = errors.New("ticket signature mismatch")
	// ErrExpired marks a token past its expiry (clock skew already allowed for).
	ErrExpired = errors.New("ticket expired")
	// ErrNoSecret marks a mint or verify attempted with no secret configured,
	// which is a deployment error rather than a bad ticket.
	ErrNoSecret = errors.New("ticket secret is empty")
)

// flavorErr carries a flavor's own message while reporting membership in one of
// the marker classes above. It is a pointer type so two flavors that happen to
// share a message stay distinct under errors.Is, exactly like errors.New.
type flavorErr struct {
	msg    string
	marker error
}

func (e *flavorErr) Error() string { return e.msg }
func (e *flavorErr) Unwrap() error { return e.marker }

// Codec signs and opens one flavor's tokens, reporting failures through that
// flavor's own sentinels. Build one with New.
//
// The sentinels are unexported and read through the accessors below, which fall
// back to the package markers when unset. That is not defensive habit: this type
// decides whether a token is accepted, so a Codec that was never built by New
// must reject rather than return a nil error — a nil error here would be read by
// every caller as a verified ticket.
type Codec struct {
	malformed error
	signature error
	expired   error
	noSecret  error
}

// New builds a Codec for a flavor named in the singular and lower case, e.g.
// "shell ticket".
func New(name string) *Codec {
	return &Codec{
		malformed: &flavorErr{msg: "malformed " + name, marker: ErrMalformed},
		signature: &flavorErr{msg: name + " signature mismatch", marker: ErrSignature},
		expired:   &flavorErr{msg: name + " expired", marker: ErrExpired},
		noSecret:  &flavorErr{msg: name + " secret is empty", marker: ErrNoSecret},
	}
}

// Malformed, Signature, Expired, and NoSecret are this flavor's sentinels. A
// flavor package aliases them (var ErrExpired = codec.Expired()) so its callers
// keep naming errors in its own vocabulary.

// Each accessor tests the receiver before reading its field: an unbuilt Codec
// may be a nil pointer as easily as a zero value, and dereferencing it inside a
// helper's argument list would panic instead of falling back.

func (c *Codec) Malformed() error {
	if c == nil || c.malformed == nil {
		return ErrMalformed
	}
	return c.malformed
}

func (c *Codec) Signature() error {
	if c == nil || c.signature == nil {
		return ErrSignature
	}
	return c.signature
}

func (c *Codec) Expired() error {
	if c == nil || c.expired == nil {
		return ErrExpired
	}
	return c.expired
}

func (c *Codec) NoSecret() error {
	if c == nil || c.noSecret == nil {
		return ErrNoSecret
	}
	return c.noSecret
}

// Sign marshals claims and returns the signed, URL-safe token. Callers fill any
// single-use nonce first (see EnsureNonce) so the signature covers it.
func (c *Codec) Sign(secret []byte, claims any) (string, error) {
	if len(secret) == 0 {
		return "", c.NoSecret()
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + mac(secret, body), nil
}

// Open verifies token's signature against secret and decodes the payload into
// claims, which must be a pointer. It judges only the envelope: which claims a
// flavor requires, and whether they are in bounds, stay with the flavor (see
// CheckBounds).
//
// The signature is checked BEFORE the payload is decoded, so an unauthenticated
// caller cannot reach the JSON decoder at all. On any failure claims is left
// zeroed: encoding/json fills fields as it goes and only then reports a type
// error, which would otherwise leave attacker-chosen values in a claim set the
// caller was told to reject.
func (c *Codec) Open(secret []byte, token string, claims any) error {
	if len(secret) == 0 {
		return c.NoSecret()
	}
	body, signature, ok := strings.Cut(token, ".")
	if !ok || body == "" || signature == "" {
		return c.Malformed()
	}
	if !hmac.Equal([]byte(signature), []byte(mac(secret, body))) {
		return c.Signature()
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return c.Malformed()
	}
	if err := json.Unmarshal(payload, claims); err != nil {
		if v := reflect.ValueOf(claims); v.Kind() == reflect.Pointer && !v.IsNil() {
			v.Elem().SetZero()
		}
		return c.Malformed()
	}
	return nil
}

// CheckBounds enforces a verified ticket's time bounds: past its expiry is
// Expired, and dated further into the future than ClockSkew allows is Malformed
// (a zero issuedAt is unset and skipped — some flavors never stamp one).
func (c *Codec) CheckBounds(now time.Time, issuedAt, expiresAt int64) error {
	if now.Add(-ClockSkew).After(time.Unix(expiresAt, 0)) {
		return c.Expired()
	}
	if issuedAt != 0 && now.Add(ClockSkew).Before(time.Unix(issuedAt, 0)) {
		return fmt.Errorf("%w: issued in the future", c.Malformed())
	}
	return nil
}

// Nonce returns a fresh 128-bit single-use id.
func Nonce() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// EnsureNonce fills n with a fresh Nonce when it is empty, so every minted
// ticket is single-use-trackable whether or not the caller supplied one.
func EnsureNonce(n *string) error {
	if *n != "" {
		return nil
	}
	v, err := Nonce()
	if err != nil {
		return err
	}
	*n = v
	return nil
}

// NonceExpiry is the instant a ticket's single-use nonce may be pruned from the
// replay guard. It matches the verifier's EFFECTIVE acceptance window
// (expiresAt + ClockSkew), not the raw expiresAt — otherwise a still-verifiable
// ticket's nonce could be pruned and re-claimed during the skew interval,
// defeating single-use (codex #8).
func NonceExpiry(expiresAt int64) time.Time {
	return time.Unix(expiresAt, 0).Add(ClockSkew)
}

func mac(secret []byte, body string) string {
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
