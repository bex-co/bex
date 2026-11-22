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

package sandboxbroker

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

func TestMatchPath(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		// Render OpenAPI examples.
		{"/a/*", "/a/b", true},
		{"/a/*", "/a", false},
		{"/a/*", "/a/", false},
		{"/a/*", "/a/b/c", false},
		{"/a/*", "/a/b/", false},
		{"/a/**", "/a/b", true},
		{"/a/**", "/a/b/c", true},
		{"/a/**", "/a/b/c/d", true},
		{"/a/**", "/a", false},
		{"/a/**", "/a/", false},
		// Root and exact matches.
		{"/", "/", true},
		{"/", "/a", false},
		{"/*", "/a", true},
		{"/*", "/", false},
		{"/**", "/a", true},
		{"/**", "/a/b", true},
		{"/**", "/", false},
		{"/v1/items", "/v1/items", true},
		{"/v1/items", "/v1/items/", false},
		// Mid-path wildcards.
		{"/a/*/c", "/a/b/c", true},
		{"/a/*/c", "/a/c", false},
		{"/a/*/c", "/a/b/d/c", false},
		{"/a/**/z", "/a/b/z", true},
		{"/a/**/z", "/a/b/c/z", true},
		{"/a/**/z", "/a/z", false},
		// Query-string invariance.
		{"/a/*", "/a/b?foo=bar&baz=1", true},
		{"/a/**", "/a/b/c?x=1#frag", true},
		// Dot-segment traversal defense.
		{"/a/*", "/a/../secret", false},
		{"/a/**", "/a/../secret", false},
		{"/a/**", "/a/b/../../secret", false},
		{"/a/*", "/a/./b", true},
		{"/a/*", "/a/b/../c", true},
		{"/a/*", "/a//b", true},
	}
	for _, tc := range cases {
		got := MatchPath(tc.pattern, tc.path)
		if got != tc.want {
			t.Errorf("MatchPath(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestMatchCredentialAndNonCombination(t *testing.T) {
	creds := []sandbox.BrokerCredential{
		{
			ID:      "cred-z",
			Domain:  "api.example.com",
			Methods: []string{"POST"},
			Paths:   []string{"/v1/**"},
			Value:   "secret-z",
		},
		{
			ID:      "cred-a",
			Domain:  "api.example.com",
			Methods: []string{"GET", "POST"},
			Paths:   []string{"/v1/*"},
			Value:   "secret-a",
		},
		{
			ID:      "cred-any",
			Domain:  "other.example.com",
			Methods: nil,
			Paths:   nil,
			Value:   "secret-any",
		},
	}

	// Overlapping POST /v1/users matches both cred-a and cred-z; cred-a wins deterministically.
	winner, overlap := MatchCredential(creds, "API.Example.COM.", "post", "/v1/users?limit=10")
	if winner == nil || winner.ID != "cred-a" || !overlap {
		t.Fatalf("overlap match = (%v, %v), want (cred-a, true)", winner, overlap)
	}

	// Single match: POST /v1/users/42 matches only cred-z.
	winner, overlap = MatchCredential(creds, "api.example.com", "POST", "/v1/users/42")
	if winner == nil || winner.ID != "cred-z" || overlap {
		t.Fatalf("single match = (%v, %v), want (cred-z, false)", winner, overlap)
	}

	// Non-matching method: DELETE /v1/users matches none.
	winner, overlap = MatchCredential(creds, "api.example.com", "DELETE", "/v1/users")
	if winner != nil || overlap {
		t.Fatalf("method mismatch = (%v, %v), want (nil, false)", winner, overlap)
	}

	// Non-matching path: GET /v1 matches neither /v1/* nor /v1/**.
	winner, overlap = MatchCredential(creds, "api.example.com", "GET", "/v1")
	if winner != nil || overlap {
		t.Fatalf("path mismatch = (%v, %v), want (nil, false)", winner, overlap)
	}

	// Empty methods and empty paths match any method/path on matching domain.
	winner, overlap = MatchCredential(creds, "other.example.com", "PATCH", "/anything/at/all")
	if winner == nil || winner.ID != "cred-any" || overlap {
		t.Fatalf("wildcard method/path = (%v, %v), want (cred-any, false)", winner, overlap)
	}
}

func TestAllowsDestination(t *testing.T) {
	allowList := sandbox.NetworkPolicy{
		Default: sandbox.NetworkPolicyAllowList,
		Rules: []sandbox.NetworkPolicyRule{
			{Domain: "api.example.com", Protocol: sandbox.NetworkPolicyProtocolHTTPS},
			{Domain: "*.sub.example.org", Protocol: sandbox.NetworkPolicyProtocolHTTPS},
		},
	}
	cases := []struct {
		policy sandbox.NetworkPolicy
		host   string
		want   bool
	}{
		{sandbox.NetworkPolicy{Default: sandbox.NetworkPolicyDenyAll}, "api.example.com", false},
		{sandbox.NetworkPolicy{}, "api.example.com", false},
		{sandbox.NetworkPolicy{Default: sandbox.NetworkPolicyAllowAll}, "anything.example.com", true},
		{allowList, "api.example.com", true},
		{allowList, "API.EXAMPLE.COM.", true},
		{allowList, "other.example.com", false},
		{allowList, "example.com", false},
		{allowList, "a.sub.example.org", true},
		{allowList, "sub.example.org", false},     // wildcard does not match apex
		{allowList, "a.b.sub.example.org", false}, // single leftmost label only
	}
	for _, tc := range cases {
		if got := AllowsDestination(tc.policy, tc.host); got != tc.want {
			t.Errorf("AllowsDestination(%q, %q) = %v, want %v", tc.policy.Default, tc.host, got, tc.want)
		}
	}
}

func TestInjectAllModesAndDefenseInDepth(t *testing.T) {
	t.Run("header replaces spoofed caller header and supports 1-5 templates", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/v1/x", nil)
		req.Header.Add("Authorization", "Bearer spoofed-1")
		req.Header.Add("Authorization", "Bearer spoofed-2")
		req.Header.Set("X-Keep", "yes")
		cred := sandbox.BrokerCredential{
			ID: "h1", Domain: "api.example.com", Value: "tok-123",
			Injection: sandbox.GroupCredentialInjection{
				Type:     sandbox.CredentialInjectionHeader,
				Name:     "Authorization",
				Template: "Bearer ${SECRET}:${SECRET}:${SECRET}:${SECRET}:${SECRET}",
			},
		}
		if err := Inject(req, cred); err != nil {
			t.Fatalf("Inject header: %v", err)
		}
		vals := req.Header.Values("Authorization")
		want := "Bearer tok-123:tok-123:tok-123:tok-123:tok-123"
		if len(vals) != 1 || vals[0] != want {
			t.Fatalf("Authorization = %v, want [%q]", vals, want)
		}
		if req.Header.Get("X-Keep") != "yes" {
			t.Fatal("unrelated header dropped")
		}
	})

	t.Run("header omits template uses raw secret", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/v1/x", nil)
		cred := sandbox.BrokerCredential{
			ID: "h2", Domain: "api.example.com", Value: "raw-key",
			Injection: sandbox.GroupCredentialInjection{Type: sandbox.CredentialInjectionHeader, Name: "X-Api-Key"},
		}
		if err := Inject(req, cred); err != nil {
			t.Fatalf("Inject raw header: %v", err)
		}
		if req.Header.Get("X-Api-Key") != "raw-key" {
			t.Fatalf("X-Api-Key = %q, want raw-key", req.Header.Get("X-Api-Key"))
		}
	})

	t.Run("basic replaces caller Authorization", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "https://api.example.com/v1/x", nil)
		req.Header.Set("Authorization", "Basic c3Bvb2Y6c3Bvb2Y=")
		cred := sandbox.BrokerCredential{
			ID: "b1", Domain: "api.example.com", Value: "pw-secret",
			Injection: sandbox.GroupCredentialInjection{
				Type:     sandbox.CredentialInjectionBasic,
				Template: "svc-user:${SECRET}",
			},
		}
		if err := Inject(req, cred); err != nil {
			t.Fatalf("Inject basic: %v", err)
		}
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("svc-user:pw-secret"))
		if req.Header.Get("Authorization") != want {
			t.Fatalf("Authorization = %q, want %q", req.Header.Get("Authorization"), want)
		}
	})

	t.Run("cookie replaces matching cookie and preserves unrelated cookies", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/v1/x", nil)
		req.Header.Add("Cookie", "theme=dark; session=spoofed; lang=en")
		req.Header.Add("Cookie", "session=spoofed2; keep=1")
		cred := sandbox.BrokerCredential{
			ID: "c1", Domain: "api.example.com", Value: "real-sess",
			Injection: sandbox.GroupCredentialInjection{
				Type:     sandbox.CredentialInjectionCookie,
				Name:     "session",
				Template: "v2-${SECRET}",
			},
		}
		if err := Inject(req, cred); err != nil {
			t.Fatalf("Inject cookie: %v", err)
		}
		got := req.Header.Get("Cookie")
		if got != "theme=dark; lang=en; keep=1; session=v2-real-sess" {
			t.Fatalf("Cookie = %q", got)
		}
	})

	t.Run("query replaces caller param and preserves unrelated params", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/v1/x?foo=bar&api_key=spoofed&api_key=spoofed2&z=1", nil)
		cred := sandbox.BrokerCredential{
			ID: "q1", Domain: "api.example.com", Value: "q-secret&more=1",
			Injection: sandbox.GroupCredentialInjection{
				Type: sandbox.CredentialInjectionQuery,
				Name: "api_key",
			},
		}
		if err := Inject(req, cred); err != nil {
			t.Fatalf("Inject query: %v", err)
		}
		q := req.URL.Query()
		if q.Get("foo") != "bar" || q.Get("z") != "1" {
			t.Fatalf("unrelated query params corrupted: %v", q)
		}
		if vals := q["api_key"]; len(vals) != 1 || vals[0] != "q-secret&more=1" {
			t.Fatalf("api_key = %v, want [q-secret&more=1]", vals)
		}
	})

	t.Run("defense in depth rejects forbidden headers and never leaks secret in error", func(t *testing.T) {
		secret := "ultra-secret-value-xyz"
		for _, forbidden := range []string{"Host", "Connection", "Proxy-Authorization", "Transfer-Encoding", "Content-Length", "X-Forwarded-For", "Forwarded"} {
			req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/v1/x", nil)
			cred := sandbox.BrokerCredential{
				ID: "bad", Domain: "api.example.com", Value: secret,
				Injection: sandbox.GroupCredentialInjection{Type: sandbox.CredentialInjectionHeader, Name: forbidden},
			}
			err := Inject(req, cred)
			if err == nil {
				t.Errorf("expected forbidden header %q to be rejected", forbidden)
				continue
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("error for %q leaked secret value", forbidden)
			}
		}
		// Malformed query fails closed without leaking secret.
		req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/v1/x", nil)
		req.URL.RawQuery = "%zz"
		cred := sandbox.BrokerCredential{
			ID: "badq", Domain: "api.example.com", Value: secret,
			Injection: sandbox.GroupCredentialInjection{Type: sandbox.CredentialInjectionQuery, Name: "k"},
		}
		err := Inject(req, cred)
		if err == nil || strings.Contains(err.Error(), secret) {
			t.Fatalf("malformed query err = %v, want non-nil secret-free error", err)
		}
	})
}
