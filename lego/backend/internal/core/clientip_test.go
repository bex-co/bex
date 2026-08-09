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

package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func mustTrustedProxies(t *testing.T, csv string) TrustedProxies {
	t.Helper()
	p, err := ParseTrustedProxies(csv)
	if err != nil {
		t.Fatalf("ParseTrustedProxies(%q): %v", csv, err)
	}
	return p
}

func TestParseTrustedProxies(t *testing.T) {
	if p, err := ParseTrustedProxies(""); err != nil || p != nil {
		t.Errorf("empty input: got (%v, %v), want (nil, nil)", p, err)
	}
	if p, err := ParseTrustedProxies("  "); err != nil || p != nil {
		t.Errorf("blank input: got (%v, %v), want (nil, nil)", p, err)
	}
	p := mustTrustedProxies(t, "10.0.0.0/8, 2001:db8::/32 ,192.168.0.0/16")
	if len(p) != 3 {
		t.Fatalf("parsed %d prefixes, want 3", len(p))
	}
	for _, bad := range []string{"10.0.0.0/33", "not-a-cidr", "10.0.0.0/8,,192.168.0.0/16", "10.0.0.1"} {
		if _, err := ParseTrustedProxies(bad); err == nil {
			t.Errorf("ParseTrustedProxies(%q): want error (fail closed), got nil", bad)
		}
	}
}

// reqFrom builds a request with the given immediate peer and optional
// forwarding headers.
func reqFrom(peer, xff, xri string) *http.Request {
	r := httptest.NewRequest("GET", "/v1/services", nil)
	r.RemoteAddr = peer
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	if xri != "" {
		r.Header.Set("X-Real-IP", xri)
	}
	return r
}

func TestClientIPNoTrustedProxiesIsByteIdentical(t *testing.T) {
	var none TrustedProxies
	// Headers present but no proxies configured ⇒ ignored, peer IP wins.
	if got := none.ClientIP(reqFrom("10.0.0.1:1234", "203.0.113.9", "203.0.113.10")); got != "10.0.0.1" {
		t.Errorf("host:port peer: got %q, want 10.0.0.1", got)
	}
	// The pre-existing fallback shapes stay exact: raw RemoteAddr when it is
	// not a host:port pair, or when the host part is empty.
	if got := none.ClientIP(reqFrom("192.0.2.55", "", "")); got != "192.0.2.55" {
		t.Errorf("bare-IP peer: got %q, want 192.0.2.55", got)
	}
	if got := none.ClientIP(reqFrom(":8080", "", "")); got != ":8080" {
		t.Errorf("empty-host peer: got %q, want :8080", got)
	}
}

func TestClientIPUntrustedPeerIgnoresSpoofedHeaders(t *testing.T) {
	p := mustTrustedProxies(t, "10.0.0.0/8")
	// The peer is not inside the trusted CIDRs, so its headers are fiction.
	if got := p.ClientIP(reqFrom("192.0.2.1:443", "203.0.113.9", "203.0.113.10")); got != "192.0.2.1" {
		t.Errorf("untrusted peer: got %q, want the peer 192.0.2.1", got)
	}
}

func TestClientIPTrustedPeerWalksXFFRightToLeft(t *testing.T) {
	p := mustTrustedProxies(t, "10.0.0.0/8")
	cases := []struct{ name, xff, want string }{
		// Traefik appends the real client rightmost; spoofed entries the
		// client prepended sit further left and are never reached.
		{"single client entry", "203.0.113.9", "203.0.113.9"},
		{"client-spoofed left entries skipped", "1.1.1.1, 2.2.2.2, 203.0.113.9", "203.0.113.9"},
		// A chain of trusted proxies: walk past every trusted hop to the
		// first untrusted address.
		{"trusted hop walked past", "203.0.113.9, 10.0.0.5", "203.0.113.9"},
		// Every entry trusted ⇒ the leftmost is the best available answer.
		{"all trusted falls to leftmost", "10.0.0.6, 10.0.0.5", "10.0.0.6"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := p.ClientIP(reqFrom("10.0.0.1:1234", tc.xff, "")); got != tc.want {
				t.Errorf("XFF %q: got %q, want %q", tc.xff, got, tc.want)
			}
		})
	}
}

func TestClientIPFallbacks(t *testing.T) {
	p := mustTrustedProxies(t, "10.0.0.0/8")
	// No X-Forwarded-For ⇒ X-Real-IP.
	if got := p.ClientIP(reqFrom("10.0.0.1:1234", "", "203.0.113.9")); got != "203.0.113.9" {
		t.Errorf("X-Real-IP fallback: got %q, want 203.0.113.9", got)
	}
	// A malformed XFF entry stops the walk; X-Real-IP is the next fallback.
	if got := p.ClientIP(reqFrom("10.0.0.1:1234", "garbage", "203.0.113.9")); got != "203.0.113.9" {
		t.Errorf("malformed XFF: got %q, want 203.0.113.9", got)
	}
	// Neither header usable ⇒ the peer IP, exactly as without the feature.
	if got := p.ClientIP(reqFrom("10.0.0.1:1234", "garbage", "also-garbage")); got != "10.0.0.1" {
		t.Errorf("no usable header: got %q, want the peer 10.0.0.1", got)
	}
	// IPv6 client entries normalize.
	p6 := mustTrustedProxies(t, "10.0.0.0/8")
	if got := p6.ClientIP(reqFrom("10.0.0.1:1234", "2001:0db8:0000:0000:0000:0000:0000:0001", "")); got != "2001:db8::1" {
		t.Errorf("IPv6 XFF entry: got %q, want 2001:db8::1", got)
	}
}
