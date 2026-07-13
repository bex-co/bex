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

package id

import (
	"regexp"
	"testing"
)

// dns1123Label is Kubernetes' object-name / DNS-label grammar (RFC 1123): lower
// alphanumerics and hyphens, must start and end alphanumeric, ≤63 chars.
// Crucially it forbids "_" — the property that rules out Stripe-style ids.
var dns1123Label = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// TestRegistryContract holds every registered Kind to the prefix contract: a
// short lowercase tag, unique across kinds. A new Kind that breaks this (mixed
// case, collision, wrong length) fails here the moment it's added.
func TestRegistryContract(t *testing.T) {
	prefixRE := regexp.MustCompile(`^[a-z]{2,4}$`)
	seen := map[string]Kind{}
	for _, k := range Kinds() {
		if !prefixRE.MatchString(k.Prefix()) {
			t.Errorf("kind %q: prefix %q must be 2-4 lowercase letters", k.Desc(), k.Prefix())
		}
		if prev, dup := seen[k.Prefix()]; dup {
			t.Errorf("prefix %q is used by both %q and %q — prefixes must be unique", k.Prefix(), prev.Desc(), k.Desc())
		}
		seen[k.Prefix()] = k
		if k.Desc() == "" {
			t.Errorf("prefix %q: Desc must be set (used in docs/errors)", k.Prefix())
		}
	}
}

// TestNewIsWellFormedAndDNSSafe is the load-bearing guard: every kind's minted
// id must be well-formed, round-trip through KindOf, AND be a valid DNS-1123
// label. The last property is why ids use a hyphen — it machine-checks that a
// bex id can be dropped into a hostname or k8s object name unescaped.
func TestNewIsWellFormedAndDNSSafe(t *testing.T) {
	for _, k := range Kinds() {
		got := New(k)
		if !WellFormed(got) {
			t.Errorf("New(%s) = %q is not well-formed", k.Desc(), got)
		}
		if kind, ok := KindOf(got); !ok || kind.Prefix() != k.Prefix() {
			t.Errorf("KindOf(%q) = %+v, %v; want kind %s", got, kind, ok, k.Prefix())
		}
		if !dns1123Label.MatchString(got) {
			t.Errorf("New(%s) = %q is not a valid DNS-1123 label (unsafe in hostnames/k8s names)", k.Desc(), got)
		}
		if len(got) > 63 {
			t.Errorf("New(%s) = %q exceeds the 63-char DNS-label limit", k.Desc(), got)
		}
	}
}

// TestDeriveIsDeterministicAndWellFormed pins the contract the service-events
// feed rests on (w3/m7): a derived id is a pure function of its parts, is
// shaped exactly like a minted one (so it passes WellFormed/KindOf and Render's
// ^evt-[0-9a-z]{20}$ pattern), and cannot be confused across part boundaries.
func TestDeriveIsDeterministicAndWellFormed(t *testing.T) {
	for _, k := range Kinds() {
		got := Derive(k, "dep-c185th5c2rvvnhbfiltg", "started")
		if got != Derive(k, "dep-c185th5c2rvvnhbfiltg", "started") {
			t.Errorf("Derive(%s, …) is not deterministic — a client pages and dedupes on this id", k.Desc())
		}
		if !WellFormed(got) {
			t.Errorf("Derive(%s, …) = %q is not well-formed", k.Desc(), got)
		}
		if kind, ok := KindOf(got); !ok || kind.Prefix() != k.Prefix() {
			t.Errorf("KindOf(%q) = %+v, %v; want kind %s", got, kind, ok, k.Prefix())
		}
		if !dns1123Label.MatchString(got) {
			t.Errorf("Derive(%s, …) = %q is not a valid DNS-1123 label", k.Desc(), got)
		}
	}
	// Different parts, different ids — including across the part boundary, so a
	// deploy's start and end events can never collide with one another.
	a := Derive(Event, "dep-1", "started")
	b := Derive(Event, "dep-1", "ended")
	c := Derive(Event, "dep-1started")
	if a == b || a == c || b == c {
		t.Errorf("Derive collided: %q %q %q — parts must not be ambiguously joined", a, b, c)
	}
}

func TestDerivePanicsOnUnregisteredKind(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Derive(Kind{}, …) should panic on an unregistered kind")
		}
	}()
	_ = Derive(Kind{}, "x")
}

// TestNewPanicsOnUnregisteredKind documents the compile-time closure's runtime
// backstop: the one Kind a caller can construct without the package's blessing
// is the zero Kind{} (unexported fields block any other), and New rejects it
// fail-fast rather than minting a malformed "-<xid>". (Custom-prefix Kinds can't
// even be written outside this package — that's a compile error, not a test.)
func TestNewPanicsOnUnregisteredKind(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New(Kind{}) should panic on an unregistered kind")
		}
	}()
	_ = New(Kind{})
}

// TestUnderscoreFormIsRejected pins the ADR decision (docs/ADR020-identifiers.md): the
// Stripe-style "tea_…" form is NOT a valid bex id. If someone switches the
// separator to an underscore, New's output stops matching WellFormed and this
// test — plus TestNewIsWellFormedAndDNSSafe — fails loudly.
func TestUnderscoreFormIsRejected(t *testing.T) {
	underscore := Workspace.Prefix() + "_" + "c185th5c2rvvnhbfiltg"
	if WellFormed(underscore) {
		t.Errorf("%q must not be a well-formed id — ids use a hyphen, never an underscore", underscore)
	}
	if _, ok := KindOf(underscore); ok {
		t.Errorf("KindOf(%q) should not resolve an underscore-separated id", underscore)
	}
}

func TestWellFormedAndKindOfRejectJunk(t *testing.T) {
	bad := []string{
		"",
		"tea-",                      // no xid
		"tea-TOOSHORT",              // wrong xid shape
		"tea-c185th5c2rvvnhbfiltgX", // 21 chars / uppercase
		"TEA-c185th5c2rvvnhbfiltg",  // uppercase prefix
		"zzz-c185th5c2rvvnhbfiltg",  // well-formed shape but UNREGISTERED prefix
		"c185th5c2rvvnhbfiltg",      // no prefix
	}
	for _, s := range bad {
		if _, ok := KindOf(s); ok {
			t.Errorf("KindOf(%q) resolved a kind, want none", s)
		}
	}
	// The unregistered-but-shaped one is WellFormed (shape ok) yet not a KindOf —
	// documents the deliberate split between shape and registration.
	if !WellFormed("zzz-c185th5c2rvvnhbfiltg") {
		t.Error("a shaped id with an unregistered prefix should still be WellFormed")
	}
}
