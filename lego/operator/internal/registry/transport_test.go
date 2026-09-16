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

package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNormalizeBaseDefaultsPlaintextOnlyForClusterLocalHosts pins w8/013: a
// bare BEX_REGISTRY is plain HTTP only when the host is the in-cluster or
// local-dev endpoint. Before this, EVERY schemeless host became http://, so
// pointing BEX_REGISTRY at a real registry silently downgraded the transport.
func TestNormalizeBaseDefaultsPlaintextOnlyForClusterLocalHosts(t *testing.T) {
	for _, tc := range []struct {
		host string
		want string
	}{
		// Cluster-local: the shipped defaults and their dev aliases.
		{"127.0.0.1:5050", "http://127.0.0.1:5050"},
		{"localhost:5000", "http://localhost:5000"},
		{"[::1]:5000", "http://[::1]:5000"},
		{"zot.bex-registry.svc:5000", "http://zot.bex-registry.svc:5000"},
		{"zot.bex-registry.svc.cluster.local:5000", "http://zot.bex-registry.svc.cluster.local:5000"},
		{"zot.local:5000", "http://zot.local:5000"},
		{"zot", "http://zot"},
		// Off-cluster: must default to TLS.
		{"registry.example.com", "https://registry.example.com"},
		{"registry.example.com:5000", "https://registry.example.com:5000"},
		{"ghcr.io", "https://ghcr.io"},
		{"203.0.113.10:5000", "https://203.0.113.10:5000"},
		// An explicit scheme is always honored as written.
		{"http://registry.example.com", "http://registry.example.com"},
		{"https://zot.bex-registry.svc:5000", "https://zot.bex-registry.svc:5000"},
	} {
		if got := NormalizeBase(tc.host); got != tc.want {
			t.Errorf("NormalizeBase(%q) = %q, want %q", tc.host, got, tc.want)
		}
	}
}

// TestCredentialedBaseRefusesPlaintextOffCluster: the only way to put an
// Authorization header on a cleartext wire is to name an off-cluster registry
// with an explicit http:// scheme. That has no safe interpretation, so it is an
// error rather than a silent downgrade.
func TestCredentialedBaseRefusesPlaintextOffCluster(t *testing.T) {
	if _, err := CredentialedBase("http://registry.example.com"); err == nil {
		t.Error("CredentialedBase accepted explicit plaintext for an off-cluster registry")
	}
	// Cluster-local plaintext stays allowed — it is the in-cluster Zot default,
	// reached over cluster networking.
	for _, host := range []string{"127.0.0.1:5050", "zot.bex-registry.svc:5000"} {
		base, err := CredentialedBase(host)
		if err != nil {
			t.Errorf("CredentialedBase(%q) = %v, want the cluster-local plaintext base", host, err)
		}
		if !strings.HasPrefix(base, "http://") {
			t.Errorf("CredentialedBase(%q) = %q, want an http:// base", host, base)
		}
	}
	// A bare off-cluster host is upgraded, not refused.
	base, err := CredentialedBase("registry.example.com")
	if err != nil {
		t.Errorf("CredentialedBase(bare off-cluster host) = %v, want an https:// base", err)
	}
	if !strings.HasPrefix(base, "https://") {
		t.Errorf("CredentialedBase(bare off-cluster host) = %q, want https://", base)
	}
}

// TestCredentialedReadsFailClosedRatherThanLeakBasicAuth is the end-to-end
// shape of the finding: a digest/tags read that carries a username must not
// reach the network at all when the transport would be cleartext off-cluster.
// The anonymous read (the dev default) discloses nothing and is unaffected.
func TestCredentialedReadsFailClosedRatherThanLeakBasicAuth(t *testing.T) {
	var sawAuth, reached bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		if _, _, ok := r.BasicAuth(); ok {
			sawAuth = true
		}
		w.Header().Set("Docker-Content-Digest", "sha256:"+strings.Repeat("a", 64))
		_, _ = w.Write([]byte(`{"tags":["gen-1"]}`))
	}))
	defer srv.Close()

	// httptest serves plain HTTP on 127.0.0.1, which IS cluster-local, so an
	// authenticated read against it is legitimate and must still work.
	local := strings.TrimPrefix(srv.URL, "http://")
	if _, err := ResolveDigest(context.Background(), srv.Client(), local, "repo", "gen-1", "bex-builder", "pw"); err != nil {
		t.Fatalf("authenticated read against a loopback registry failed: %v", err)
	}
	if !reached || !sawAuth {
		t.Fatal("loopback read did not reach the registry with credentials")
	}

	// An off-cluster host forced to plaintext must be refused before any request.
	reached = false
	_, err := ResolveDigest(context.Background(), srv.Client(), "http://registry.example.com", "repo", "gen-1", "bex-builder", "pw")
	if err == nil {
		t.Error("ResolveDigest sent credentials to an off-cluster plaintext registry")
	}
	if reached {
		t.Error("ResolveDigest reached the network before refusing")
	}
	if _, err := ListTags(context.Background(), srv.Client(), "http://registry.example.com", "repo", "bex-builder", "pw"); err == nil {
		t.Error("ListTags sent credentials to an off-cluster plaintext registry")
	}
}
