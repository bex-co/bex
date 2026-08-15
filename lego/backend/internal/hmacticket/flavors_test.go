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

// This file holds the conformance suite every ticket flavor must satisfy, run
// through each flavor's OWN exported Mint/Verify rather than the envelope's, so
// it keeps testing the real contract if a flavor is ever restructured. It lives
// with the envelope (an external test package, so the flavors may import it)
// because the property it protects is cross-package.
package hmacticket_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/hmacticket"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/shellticket"
)

// flavor adapts one ticket package to the shared conformance suite.
type flavor struct {
	name string
	// mint returns a token whose claims are otherwise complete and valid.
	mint func(secret []byte, issuedAt, expiresAt int64) (string, error)
	// verify reports only the error; each flavor's claims shape differs.
	verify func(secret []byte, token string, now time.Time) error
	// nonceExpiry is the flavor's Claims.NonceExpiry for a given expiry.
	nonceExpiry func(expiresAt int64) time.Time
}

func flavors() []flavor {
	return []flavor{{
		name: "shellticket",
		mint: func(secret []byte, iat, exp int64) (string, error) {
			return shellticket.Mint(secret, shellticket.Claims{
				Subject: "id-user", ServiceID: "srv-abc", IssuedAt: iat, ExpiresAt: exp,
			})
		},
		verify: func(secret []byte, token string, now time.Time) error {
			_, err := shellticket.Verify(secret, token, now)
			return err
		},
		nonceExpiry: func(exp int64) time.Time {
			return shellticket.Claims{ExpiresAt: exp}.NonceExpiry()
		},
	}, {
		name: "sandboxexec",
		mint: func(secret []byte, iat, exp int64) (string, error) {
			return sandboxexec.Mint(secret, sandboxexec.Claims{
				Subject: "id-user", SandboxID: "os-1", Namespace: "tea-a-sandbox",
				Command: []string{"/bin/sh", "-c", "echo hi"}, IssuedAt: iat, ExpiresAt: exp,
			})
		},
		verify: func(secret []byte, token string, now time.Time) error {
			_, err := sandboxexec.Verify(secret, token, now)
			return err
		},
		nonceExpiry: func(exp int64) time.Time {
			return sandboxexec.Claims{ExpiresAt: exp}.NonceExpiry()
		},
	}, {
		name: "agentsessionticket",
		mint: func(secret []byte, iat, exp int64) (string, error) {
			return agentsessionticket.Mint(secret, agentsessionticket.Claims{
				Subject: "id-user", SessionID: "ags-1", SandboxID: "os-1", Pod: "os-1-0",
				Workspace: "tea-a", Namespace: "tea-a-sandbox", IssuedAt: iat, ExpiresAt: exp,
			})
		},
		verify: func(secret []byte, token string, now time.Time) error {
			_, err := agentsessionticket.Verify(secret, token, now)
			return err
		},
		nonceExpiry: func(exp int64) time.Time {
			return agentsessionticket.Claims{ExpiresAt: exp}.NonceExpiry()
		},
	}}
}

// Every flavor answers the same envelope failures the same way. This is the
// suite that would have caught the drift that motivated the shared envelope:
// one flavor's Verify had simply omitted the future-dated check.
func TestEveryFlavorEnforcesTheSameEnvelope(t *testing.T) {
	secret := []byte("conformance-secret")
	exp := time.Unix(1_800_000_000, 0)
	iat := exp.Add(-time.Minute)

	for _, f := range flavors() {
		t.Run(f.name, func(t *testing.T) {
			token, err := f.mint(secret, iat.Unix(), exp.Unix())
			if err != nil {
				t.Fatalf("Mint: %v", err)
			}

			// Accepted through exactly one skew window past expiry, and no further.
			if err := f.verify(secret, token, exp.Add(hmacticket.ClockSkew)); err != nil {
				t.Errorf("at the last accepted instant = %v, want accepted", err)
			}
			if err := f.verify(secret, token, exp.Add(hmacticket.ClockSkew+time.Second)); !errors.Is(err, hmacticket.ErrExpired) {
				t.Errorf("one second past the window = %v, want ErrExpired", err)
			}

			// A ticket dated further ahead than the skew allows is refused. A
			// verifier that accepts one lets a clock-skewed or deliberately
			// forward-dated ticket outlive its intended window.
			future, err := f.mint(secret, iat.Add(2*hmacticket.ClockSkew).Unix(), exp.Unix())
			if err != nil {
				t.Fatalf("Mint: %v", err)
			}
			if err := f.verify(secret, future, iat); !errors.Is(err, hmacticket.ErrMalformed) {
				t.Errorf("future-dated ticket = %v, want ErrMalformed", err)
			}
			// ...but one inside the skew is fine, which is the whole point of it.
			near, err := f.mint(secret, iat.Add(hmacticket.ClockSkew-time.Second).Unix(), exp.Unix())
			if err != nil {
				t.Fatalf("Mint: %v", err)
			}
			if err := f.verify(secret, near, iat); err != nil {
				t.Errorf("ticket issued within the skew = %v, want accepted", err)
			}

			// Signature failures stay signature failures.
			if err := f.verify([]byte("other-secret"), token, iat); !errors.Is(err, hmacticket.ErrSignature) {
				t.Errorf("wrong secret = %v, want ErrSignature", err)
			}
			body, mac, _ := strings.Cut(token, ".")
			tampered := body[:len(body)-1] + flipChar(body[len(body)-1]) + "." + mac
			if err := f.verify(secret, tampered, iat); !errors.Is(err, hmacticket.ErrSignature) {
				t.Errorf("tampered body = %v, want ErrSignature", err)
			}

			// Structural failures stay malformed.
			for _, bad := range []string{"", ".", "nodot", body, "." + mac, body + "."} {
				if err := f.verify(secret, bad, iat); !errors.Is(err, hmacticket.ErrMalformed) {
					t.Errorf("verify(%q) = %v, want ErrMalformed", bad, err)
				}
			}

			// A missing secret is a deployment error, not a bad ticket.
			if err := f.verify(nil, token, iat); !errors.Is(err, hmacticket.ErrNoSecret) {
				t.Errorf("no secret = %v, want ErrNoSecret", err)
			}
			if _, err := f.mint(nil, iat.Unix(), exp.Unix()); !errors.Is(err, hmacticket.ErrNoSecret) {
				t.Errorf("Mint with no secret = %v, want ErrNoSecret", err)
			}

			// The replay guard may not prune a nonce while the ticket still verifies.
			if got := f.nonceExpiry(exp.Unix()); got.Before(exp.Add(hmacticket.ClockSkew)) {
				t.Errorf("NonceExpiry = %v, want >= the last accepted instant %v", got, exp.Add(hmacticket.ClockSkew))
			}
		})
	}
}

// flipChar returns a base64url character that is not c, so a one-character edit
// of a token body is guaranteed to change it.
func flipChar(c byte) string {
	if c == 'A' {
		return "B"
	}
	return "A"
}

// Every flavor's Mint fills a nonce, so the verifier's single-use tracking has
// something unique to claim on every ticket.
func TestEveryFlavorMintsAUniqueNonce(t *testing.T) {
	secret := []byte("conformance-secret")
	exp := time.Now().Add(time.Minute).Unix()
	for _, f := range flavors() {
		t.Run(f.name, func(t *testing.T) {
			first, err := f.mint(secret, 0, exp)
			if err != nil {
				t.Fatalf("Mint: %v", err)
			}
			second, err := f.mint(secret, 0, exp)
			if err != nil {
				t.Fatalf("Mint: %v", err)
			}
			if first == second {
				t.Error("two mints of identical claims produced the same token; the nonce is not being filled")
			}
		})
	}
}

// A new ticket flavor that declares its own sentinels instead of calling
// hmacticket.New has almost certainly copied the envelope with them — that is
// how all three existing flavors were written — so the sentinel spelling is the
// cheap tell that catches it.
//
// This is deliberately narrow. It does NOT catch every hand-rolled signed token:
// internal/github/state.go frames its install-callback state as payload‖mac with
// its own sentinels, and internal/drivergrant signs with Ed25519 so an in-sandbox
// driver can verify with a public key alone. Both are different schemes, kept
// out on purpose. A broader regex over the signing idiom was tried and dropped:
// it matched any base64url-encoded MAC, so an unrelated correct change to
// internal/webhooks/signing.go would have failed a test about ticket envelopes.
func TestNoPackageRerollsTheTicketSentinels(t *testing.T) {
	sentinels := regexp.MustCompile(`errors\.New\("(malformed [^"]*ticket|[^"]*ticket (expired|signature mismatch|secret is empty))"\)`)

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	// Trailing separator: a future sibling package (hmacticketv2) must not be
	// excused by a bare prefix match.
	owner := filepath.Join(root, "internal", "hmacticket") + string(filepath.Separator)

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		// The envelope owns these, and a test may legitimately spell a message
		// out in a table without re-implementing anything.
		if strings.HasPrefix(path, owner) || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if m := sentinels.Find(src); m != nil {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s declares a ticket sentinel by hand (%s); build it from hmacticket.New so every flavor shares one envelope", rel, m)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}
