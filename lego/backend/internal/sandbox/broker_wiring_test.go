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

package sandbox

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func generateTestCA(t *testing.T, cn string, isCA bool, notBefore, notAfter time.Time) (certPEM, keyPEM []byte, cert *x509.Certificate, key *ecdsa.PrivateKey) {
	t.Helper()
	var err error
	key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, cert, key
}

func generateSignedLeaf(t *testing.T, dnsName string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(42),
		Subject:               pkix.Name{CommonName: dnsName},
		DNSNames:              []string{dnsName},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return leaf
}

func TestValidateBrokerProxyURL(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"  ", "", false},
		{"http://bex-ssh-gateway.bex-system.svc.cluster.local:8086", "http://bex-ssh-gateway.bex-system.svc.cluster.local:8086", false},
		{"http://127.0.0.1:8086", "http://127.0.0.1:8086", false},
		{"https://bex-ssh-gateway.bex-system.svc.cluster.local:8086", "", true},
		{"http://user:pass@bex-ssh-gateway.bex-system.svc.cluster.local:8086", "", true},
		{"http://bex-ssh-gateway.bex-system.svc.cluster.local:8086/path", "", true},
		{"http://bex-ssh-gateway.bex-system.svc.cluster.local:8086?a=1", "", true},
		{"http://bex-ssh-gateway.bex-system.svc.cluster.local", "", true},
		{"http://bex-ssh-gateway.bex-system.svc.cluster.local:99999", "", true},
		{"http://:8086", "", true},
	} {
		got, err := ValidateBrokerProxyURL(tc.raw)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateBrokerProxyURL(%q) err=%v, wantErr=%v", tc.raw, err, tc.wantErr)
		}
		if err == nil && got != tc.want {
			t.Errorf("ValidateBrokerProxyURL(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestValidatePublicBrokerCAPEM(t *testing.T) {
	now := time.Now().UTC()
	validPEM, keyPEM, _, _ := generateTestCA(t, "broker-ca", true, now.Add(-time.Hour), now.Add(24*time.Hour))
	nonCAPEM, _, _, _ := generateTestCA(t, "not-a-ca", false, now.Add(-time.Hour), now.Add(24*time.Hour))
	expiredPEM, _, _, _ := generateTestCA(t, "expired-ca", true, now.Add(-48*time.Hour), now.Add(-time.Hour))

	got, err := ValidatePublicBrokerCAPEM(validPEM, now)
	if err != nil {
		t.Fatalf("valid CA rejected: %v", err)
	}
	if !bytes.Contains(got, []byte("BEGIN CERTIFICATE")) || bytes.Contains(got, []byte("PRIVATE KEY")) {
		t.Fatalf("unexpected canonical PEM: %s", string(got))
	}

	rejects := []struct {
		name string
		raw  []byte
	}{
		{"empty", nil},
		{"private key only", keyPEM},
		{"cert plus private key", append(bytes.Clone(validPEM), keyPEM...)},
		{"non-CA cert", nonCAPEM},
		{"expired CA", expiredPEM},
		{"two certs", append(bytes.Clone(validPEM), validPEM...)},
		{"trailing garbage", append(bytes.Clone(validPEM), []byte("rm -rf /\n")...)},
		{"oversized", bytes.Repeat([]byte("A"), maxBrokerCAPEMBytes+1)},
	}
	for _, tc := range rejects {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidatePublicBrokerCAPEM(tc.raw, now); err == nil {
				t.Fatalf("expected %s to fail closed", tc.name)
			}
		})
	}
}

func TestBrokerConfigResolveCAPEMFromURLAndCacheFallback(t *testing.T) {
	now := time.Now().UTC()
	caPEM1, _, _, _ := generateTestCA(t, "broker-ca-1", true, now.Add(-time.Hour), now.Add(24*time.Hour))
	caPEM2, _, _, _ := generateTestCA(t, "broker-ca-2", true, now.Add(-time.Hour), now.Add(24*time.Hour))

	var calls atomic.Int32
	var mode atomic.Int32 // 0=ca1, 1=503, 2=ca2
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		switch mode.Load() {
		case 0:
			w.Header().Set("Content-Type", "application/x-pem-file")
			_, _ = w.Write(caPEM1)
		case 1:
			http.Error(w, "restarting", http.StatusServiceUnavailable)
		case 2:
			w.Header().Set("Content-Type", "application/x-pem-file")
			_, _ = w.Write(caPEM2)
		}
	}))
	defer srv.Close()

	clock := now
	cfg := &BrokerConfig{
		ProxyURL:  "http://bex-ssh-gateway.bex-system.svc.cluster.local:8086",
		CACertURL: srv.URL + GatewayBrokerCACertPath,
		CacheTTL:  5 * time.Minute,
		Now:       func() time.Time { return clock },
	}

	got1, err := cfg.ResolveCAPEM(context.Background())
	if err != nil || !bytes.Equal(got1, caPEM1) {
		t.Fatalf("initial fetch = (%d bytes, %v), want caPEM1", len(got1), err)
	}
	// Cached within TTL: no second HTTP call.
	clock = clock.Add(2 * time.Minute)
	if _, err := cfg.ResolveCAPEM(context.Background()); err != nil {
		t.Fatalf("cached fetch failed: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1 (cached)", calls.Load())
	}

	// Past TTL, gateway transiently 503s: still-valid cached CA is reused.
	mode.Store(1)
	clock = clock.Add(10 * time.Minute)
	fallback, err := cfg.ResolveCAPEM(context.Background())
	if err != nil || !bytes.Equal(fallback, caPEM1) {
		t.Fatalf("fallback on transient 503 = (%d bytes, %v), want cached caPEM1", len(fallback), err)
	}

	// Once gateway returns rotated CA, refresh picks it up.
	mode.Store(2)
	got2, err := cfg.ResolveCAPEM(context.Background())
	if err != nil || !bytes.Equal(got2, caPEM2) {
		t.Fatalf("rotated fetch = (%d bytes, %v), want caPEM2", len(got2), err)
	}
}

func TestBrokerConfigInjectMatrix(t *testing.T) {
	now := time.Now().UTC()
	caPEM, _, _, _ := generateTestCA(t, "broker-ca", true, now.Add(-time.Hour), now.Add(24*time.Hour))
	const proxyURL = "http://bex-ssh-gateway.bex-system.svc.cluster.local:8086"
	cfg := &BrokerConfig{ProxyURL: proxyURL, CACertPEM: caPEM}
	baseTmpl := Template{
		Image:      "docker.io/library/bash:5.3",
		Entrypoint: []string{"/bin/sh", "-c", "exec sleep infinity"},
		Env:        map[string]string{"HOME": "/home/render"},
	}

	for _, policy := range []*NetworkPolicy{
		{Default: NetworkPolicyAllowAll},
		{Default: NetworkPolicyAllowList, Rules: []NetworkPolicyRule{{Domain: "api.example.com", Protocol: NetworkPolicyProtocolHTTPS}}},
	} {
		t.Run(string(policy.Default), func(t *testing.T) {
			entry, env := cfg.Inject(context.Background(), policy, baseTmpl, map[string]string{
				"USER_VAR": "ok",
				"NO_PROXY": "custom.internal,localhost",
			})
			if len(entry) != 3 || entry[0] != "/bin/sh" || entry[1] != "-c" {
				t.Fatalf("unexpected entrypoint: %v", entry)
			}
			if !strings.Contains(entry[2], string(bytes.TrimSpace(caPEM))) || !strings.Contains(entry[2], "exec sleep infinity") {
				t.Fatalf("entrypoint missing CA bootstrap or original command: %s", entry[2])
			}
			wantEnv := map[string]string{
				"HTTPS_PROXY":         proxyURL,
				"https_proxy":         proxyURL,
				"NO_PROXY":            "custom.internal,localhost", // caller wins
				"no_proxy":            SandboxBrokerNoProxy,
				"SSL_CERT_FILE":       SandboxBrokerCABundleFile,
				"REQUESTS_CA_BUNDLE":  SandboxBrokerCABundleFile,
				"CURL_CA_BUNDLE":      SandboxBrokerCABundleFile,
				"NODE_EXTRA_CA_CERTS": SandboxBrokerCACertFile,
				"USER_VAR":            "ok",
			}
			for k, want := range wantEnv {
				if env[k] != want {
					t.Errorf("env[%q] = %q, want %q", k, env[k], want)
				}
			}
		})
	}

	t.Run("custom non-shell entrypoint wraps with exec $@", func(t *testing.T) {
		customTmpl := Template{Image: "python:3.13", Entrypoint: []string{"python3", "-m", "http.server"}}
		entry, env := cfg.Inject(context.Background(), &NetworkPolicy{Default: NetworkPolicyAllowAll}, customTmpl, nil)
		if len(entry) != 7 || entry[0] != "/bin/sh" || entry[1] != "-c" || entry[3] != "--" || entry[4] != "python3" {
			t.Fatalf("unexpected wrapped custom entrypoint: %v", entry)
		}
		if env["REQUESTS_CA_BUNDLE"] != SandboxBrokerCABundleFile {
			t.Fatalf("REQUESTS_CA_BUNDLE = %q", env["REQUESTS_CA_BUNDLE"])
		}
	})

	t.Run("deny-all receives zero broker wiring", func(t *testing.T) {
		entry, env := cfg.Inject(context.Background(), &NetworkPolicy{Default: NetworkPolicyDenyAll}, baseTmpl, map[string]string{"FOO": "bar"})
		if strings.Join(entry, " ") != strings.Join(baseTmpl.Entrypoint, " ") {
			t.Fatalf("deny-all entrypoint was mutated: %v", entry)
		}
		if _, ok := env["HTTPS_PROXY"]; ok {
			t.Fatalf("deny-all must not receive HTTPS_PROXY: %v", env)
		}
		if _, ok := env["SSL_CERT_FILE"]; ok {
			t.Fatalf("deny-all must not receive SSL_CERT_FILE: %v", env)
		}
	})

	t.Run("agent-session template receives zero broker wiring", func(t *testing.T) {
		agentTmpl := Template{Image: "agent:latest", Entrypoint: []string{"bex-agent-driver"}, SessionOnly: true}
		entry, env := cfg.Inject(context.Background(), &NetworkPolicy{Default: NetworkPolicyAllowAll}, agentTmpl, nil)
		if strings.Join(entry, " ") != "bex-agent-driver" || len(env) != 0 {
			t.Fatalf("agent-session template mutated: entry=%v env=%v", entry, env)
		}
	})

	t.Run("nil or empty proxy URL is a graceful no-op", func(t *testing.T) {
		var nilCfg *BrokerConfig
		entry, env := nilCfg.Inject(context.Background(), &NetworkPolicy{Default: NetworkPolicyAllowAll}, baseTmpl, map[string]string{"A": "1"})
		if strings.Join(entry, " ") != strings.Join(baseTmpl.Entrypoint, " ") || len(env) != 1 || env["A"] != "1" {
			t.Fatalf("nil broker mutated create: entry=%v env=%v", entry, env)
		}
		emptyCfg := &BrokerConfig{CACertPEM: caPEM}
		entry, env = emptyCfg.Inject(context.Background(), &NetworkPolicy{Default: NetworkPolicyAllowAll}, baseTmpl, nil)
		if strings.Join(entry, " ") != strings.Join(baseTmpl.Entrypoint, " ") || len(env) != 0 {
			t.Fatalf("empty ProxyURL mutated create: entry=%v env=%v", entry, env)
		}
	})

	t.Run("CA unavailable skips both proxy and CA atomically", func(t *testing.T) {
		downSrv := httptest.NewServer(http.NotFoundHandler())
		defer downSrv.Close()
		downCfg := &BrokerConfig{ProxyURL: proxyURL, CACertURL: downSrv.URL + GatewayBrokerCACertPath}
		entry, env := downCfg.Inject(context.Background(), &NetworkPolicy{Default: NetworkPolicyAllowAll}, baseTmpl, nil)
		if strings.Join(entry, " ") != strings.Join(baseTmpl.Entrypoint, " ") || len(env) != 0 {
			t.Fatalf("unavailable CA partially wired sandbox: entry=%v env=%v", entry, env)
		}
	})
}

// TestBrokerCABootstrapScriptExecution runs the real POSIX /bin/sh CA bootstrap
// snippet against a temp rootfs to verify both public CAs and the Sandbox
// Broker CA are trusted, and that re-running on resume/snapshot restore (even
// after a Broker CA rotation) is idempotent with zero duplicate CAs.
func TestBrokerCABootstrapScriptExecution(t *testing.T) {
	now := time.Now().UTC()
	publicCAPEM, _, publicCACert, publicCAKey := generateTestCA(t, "public-root-ca", true, now.Add(-time.Hour), now.Add(24*time.Hour))
	brokerCA1PEM, _, brokerCA1Cert, brokerCA1Key := generateTestCA(t, "broker-ca-v1", true, now.Add(-time.Hour), now.Add(24*time.Hour))
	brokerCA2PEM, _, brokerCA2Cert, brokerCA2Key := generateTestCA(t, "broker-ca-v2", true, now.Add(-time.Hour), now.Add(24*time.Hour))

	publicLeaf := generateSignedLeaf(t, "public.example.com", publicCACert, publicCAKey)
	broker1Leaf := generateSignedLeaf(t, "api.example.com", brokerCA1Cert, brokerCA1Key)
	broker2Leaf := generateSignedLeaf(t, "api.example.com", brokerCA2Cert, brokerCA2Key)

	root := t.TempDir()
	sslDir := filepath.Join(root, "etc", "ssl", "certs")
	if err := os.MkdirAll(sslDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sysBundlePath := filepath.Join(sslDir, "ca-certificates.crt")
	// Deliberately omit trailing newline to test newline separation before appending.
	if err := os.WriteFile(sysBundlePath, bytes.TrimRight(publicCAPEM, "\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runSnippet := func(caPEM []byte) {
		t.Helper()
		script := brokerCABootstrapScript(caPEM)
		// Redirect /etc/bex and /etc/ssl/... into the isolated temp rootfs.
		script = strings.ReplaceAll(script, "/etc/bex", filepath.Join(root, "etc", "bex"))
		script = strings.ReplaceAll(script, "/etc/ssl", filepath.Join(root, "etc", "ssl"))
		script = strings.ReplaceAll(script, "/etc/pki", filepath.Join(root, "etc", "pki"))
		cmd := exec.Command("/bin/sh", "-c", script)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("bootstrap script failed: %v\n%s", err, string(out))
		}
	}

	verifyPool := func(path string, host string, leaf *x509.Certificate, wantOK bool) {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(raw) {
			t.Fatalf("no valid PEM certs in %s", path)
		}
		_, err = leaf.Verify(x509.VerifyOptions{DNSName: host, Roots: pool, CurrentTime: now})
		if wantOK && err != nil {
			t.Errorf("%s failed to verify %s: %v", path, host, err)
		}
		if !wantOK && err == nil {
			t.Errorf("%s unexpectedly verified %s", path, host)
		}
	}

	// First boot (create) with Broker CA v1.
	runSnippet(brokerCA1PEM)
	bundlePath := filepath.Join(root, "etc", "bex", "sandbox-broker-ca-bundle.crt")
	standalonePath := filepath.Join(root, "etc", "bex", "sandbox-broker-ca.crt")
	verifyPool(bundlePath, "public.example.com", publicLeaf, true)
	verifyPool(bundlePath, "api.example.com", broker1Leaf, true)
	verifyPool(sysBundlePath, "public.example.com", publicLeaf, true)
	verifyPool(sysBundlePath, "api.example.com", broker1Leaf, true)
	verifyPool(standalonePath, "api.example.com", broker1Leaf, true)
	verifyPool(standalonePath, "public.example.com", publicLeaf, false)

	// Re-run (idle resume / snapshot restore) after Broker CA rotates to v2.
	runSnippet(brokerCA2PEM)
	verifyPool(bundlePath, "public.example.com", publicLeaf, true)
	verifyPool(bundlePath, "api.example.com", broker2Leaf, true)
	verifyPool(bundlePath, "api.example.com", broker1Leaf, false)
	verifyPool(sysBundlePath, "public.example.com", publicLeaf, true)
	verifyPool(sysBundlePath, "api.example.com", broker2Leaf, true)

	// Exactly 2 certificates in combined bundle (1 public + 1 broker, zero duplication).
	combined, _ := os.ReadFile(bundlePath)
	if count := bytes.Count(combined, []byte("BEGIN CERTIFICATE")); count != 2 {
		t.Fatalf("combined bundle has %d certs after rotation, want 2", count)
	}

	// Minimal image with NO system CA bundle still produces a working broker CA bundle.
	minimalRoot := t.TempDir()
	minScript := brokerCABootstrapScript(brokerCA1PEM)
	minScript = strings.ReplaceAll(minScript, "/etc/bex", filepath.Join(minimalRoot, "etc", "bex"))
	minScript = strings.ReplaceAll(minScript, "/etc/ssl", filepath.Join(minimalRoot, "etc", "ssl"))
	minScript = strings.ReplaceAll(minScript, "/etc/pki", filepath.Join(minimalRoot, "etc", "pki"))
	// Hide python3 so certifi fallback doesn't pull host roots into this minimal-image assertion.
	minCmd := exec.Command("/bin/sh", "-c", minScript)
	minCmd.Env = []string{"PATH=/bin:/usr/bin"}
	if out, err := minCmd.CombinedOutput(); err != nil {
		t.Fatalf("minimal bootstrap failed: %v\n%s", err, string(out))
	}
	verifyPool(filepath.Join(minimalRoot, "etc", "bex", "sandbox-broker-ca-bundle.crt"), "api.example.com", broker1Leaf, true)
}

func TestBrokerConfigResolveCAPEMFromPath(t *testing.T) {
	now := time.Now().UTC()
	caPEM, keyPEM, _, _ := generateTestCA(t, "disk-broker-ca", true, now.Add(-time.Hour), now.Add(24*time.Hour))
	dir := t.TempDir()
	pubPath := filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(pubPath, caPEM, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &BrokerConfig{
		ProxyURL:   "http://bex-ssh-gateway.bex-system.svc.cluster.local:8086",
		CACertPath: pubPath,
		Now:        func() time.Time { return now },
	}
	got, err := cfg.ResolveCAPEM(context.Background())
	if err != nil || !bytes.Equal(got, caPEM) {
		t.Fatalf("ResolveCAPEM from disk = (%d bytes, %v)", len(got), err)
	}
	// Disk file containing a private key must fail closed.
	keyedPath := filepath.Join(dir, "leaked.pem")
	if err := os.WriteFile(keyedPath, append(bytes.Clone(caPEM), keyPEM...), 0o600); err != nil {
		t.Fatal(err)
	}
	badCfg := &BrokerConfig{
		ProxyURL:   "http://bex-ssh-gateway.bex-system.svc.cluster.local:8086",
		CACertPath: keyedPath,
		Now:        func() time.Time { return now },
	}
	if _, err := badCfg.ResolveCAPEM(context.Background()); err == nil {
		t.Fatal("ResolveCAPEM accepted a PEM file containing a private key")
	}
}

type fakeGroupStore struct {
	defaultGroup store.SandboxGroup
	groups       map[string]store.SandboxGroup
	members      map[string]store.SandboxGroupMember
}

func (f *fakeGroupStore) CreateSandboxGroup(context.Context, string, string, string, string, int, int) (store.SandboxGroup, error) {
	return store.SandboxGroup{}, nil
}
func (f *fakeGroupStore) EnsureDefaultSandboxGroup(context.Context, string, int) (store.SandboxGroup, error) {
	return f.defaultGroup, nil
}
func (f *fakeGroupStore) SandboxGroups(context.Context, string) ([]store.SandboxGroup, error) {
	out := make([]store.SandboxGroup, 0, len(f.groups))
	for _, g := range f.groups {
		out = append(out, g)
	}
	return out, nil
}
func (f *fakeGroupStore) SandboxGroup(_ context.Context, _, id string) (store.SandboxGroup, error) {
	g, ok := f.groups[id]
	if !ok {
		return store.SandboxGroup{}, store.ErrNotFound
	}
	return g, nil
}
func (f *fakeGroupStore) SandboxGroupWorkspace(_ context.Context, id string) (string, error) {
	g, ok := f.groups[id]
	if !ok {
		return "", store.ErrNotFound
	}
	return g.WorkspaceID, nil
}
func (f *fakeGroupStore) SetSandboxGroupLimit(context.Context, string, string, int) (store.SandboxGroup, error) {
	return store.SandboxGroup{}, nil
}
func (f *fakeGroupStore) SetDefaultSandboxGroup(context.Context, string, string) (store.SandboxGroup, error) {
	return store.SandboxGroup{}, nil
}
func (f *fakeGroupStore) MarkSandboxGroupDeleting(context.Context, string, string) (store.SandboxGroup, error) {
	return store.SandboxGroup{}, nil
}
func (f *fakeGroupStore) ClaimStaleSandboxGroupDeletes(context.Context, time.Duration) ([]store.SandboxGroup, error) {
	return nil, nil
}
func (f *fakeGroupStore) DeleteSandboxGroup(context.Context, string, string) error { return nil }
func (f *fakeGroupStore) SandboxGroupMembers(context.Context, string) (map[string]store.SandboxGroupMember, error) {
	return f.members, nil
}
func (f *fakeGroupStore) WithSandboxAdmission(ctx context.Context, _ string, fn func(store.SandboxAdmission) error) error {
	return fn(&fakeAdmission{f: f})
}
func (f *fakeGroupStore) SetSandboxSlotLease(context.Context, string, string, *time.Time) error {
	return nil
}
func (f *fakeGroupStore) ReleaseSandboxSlot(context.Context, string, string) error { return nil }
func (f *fakeGroupStore) PruneSandboxGroupMembers(context.Context, string, []string) error {
	return nil
}

type fakeAdmission struct{ f *fakeGroupStore }

func (a *fakeAdmission) Members(context.Context) (map[string]store.SandboxGroupMember, error) {
	return a.f.members, nil
}
func (a *fakeAdmission) Bind(_ context.Context, sandboxID, groupID string, _ time.Time) error {
	if a.f.members == nil {
		a.f.members = map[string]store.SandboxGroupMember{}
	}
	a.f.members[sandboxID] = store.SandboxGroupMember{GroupID: groupID}
	return nil
}

type noopSessionEgress struct{}

func (noopSessionEgress) PrepareSetup(context.Context, string, string, string, sessionegress.Egress) error {
	return nil
}
func (noopSessionEgress) PrepareBuild(context.Context, string, string, sessionegress.Egress) error {
	return nil
}
func (noopSessionEgress) TransitionToAgent(context.Context, string, string, string, sessionegress.Egress) error {
	return nil
}
func (noopSessionEgress) Delete(context.Context, string, string) error { return nil }

// TestSandboxGroupLabelAndBrokerWiringAcrossCreateAndResume verifies t002 & t003
// invariants across default-group create, explicit-group create, allow-list,
// deny-all, agent-session exclusion, and resume metadata patching.
func TestSandboxGroupLabelAndBrokerWiringAcrossCreateAndResume(t *testing.T) {
	now := time.Now().UTC()
	caPEM, _, _, _ := generateTestCA(t, "broker-ca", true, now.Add(-time.Hour), now.Add(24*time.Hour))
	const proxyURL = "http://bex-ssh-gateway.bex-system.svc.cluster.local:8086"

	ws := ids.New(ids.Workspace)
	defaultGroupID := ids.New(ids.SandboxGroup)
	explicitGroupID := ids.New(ids.SandboxGroup)

	groups := &fakeGroupStore{
		defaultGroup: store.SandboxGroup{ID: defaultGroupID, WorkspaceID: ws, Name: "default", IsDefault: true, ConcurrencyLimit: 100},
		groups: map[string]store.SandboxGroup{
			defaultGroupID:  {ID: defaultGroupID, WorkspaceID: ws, Name: "default", IsDefault: true, ConcurrencyLimit: 100},
			explicitGroupID: {ID: explicitGroupID, WorkspaceID: ws, Name: "custom", IsDefault: false, ConcurrencyLimit: 100},
		},
		members: map[string]store.SandboxGroupMember{},
	}

	var (
		created []createRequest
		patched []map[string]string
		boxes   = map[string]osSandbox{}
		seq     int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sandboxes":
			var req createRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode create: %v", err)
			}
			created = append(created, req)
			seq++
			id := fmt.Sprintf("os-%d", seq)
			box := osSandbox{ID: id, Metadata: req.Metadata, Image: req.Image}
			box.Status.State = "Running"
			boxes[id] = box
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(box)
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes":
			list := make([]osSandbox, 0, len(boxes))
			for _, b := range boxes {
				list = append(list, b)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": list})
		case r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/metadata"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/sandboxes/"), "/metadata")
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			patched = append(patched, body)
			if b, ok := boxes[id]; ok {
				for k, v := range body {
					b.Metadata[k] = v
				}
				boxes[id] = b
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(boxes[id])
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/renew-expiration"):
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/resume"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/sandboxes/"), "/resume")
			if b, ok := boxes[id]; ok {
				b.Status.State = "Running"
				boxes[id] = b
			}
			w.WriteHeader(http.StatusOK)
		default:
			if strings.HasPrefix(r.URL.Path, "/sandboxes/") {
				id := strings.TrimPrefix(r.URL.Path, "/sandboxes/")
				if b, ok := boxes[id]; ok {
					_ = json.NewEncoder(w).Encode(b)
					return
				}
			}
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc := &Service{
		Base: &core.Base{
			Namespace: "default",
			Workspace: fakeWorkspace{"usr-test": ws},
		},
		Client:          NewClient(srv.URL),
		DefaultTemplate: "base",
		DefaultPlan:     Plan1C2G,
		Templates: map[string]Template{
			"base":  {Image: "bash:5.3", Entrypoint: []string{"/bin/sh", "-c", "exec sleep infinity"}, Env: map[string]string{"HOME": "/home/render"}},
			"agent": {Image: "agent:latest", Entrypoint: []string{"bex-agent-driver"}, SessionOnly: true},
		},
		Groups:        groups,
		SandboxEgress: &fakeSandboxEgress{},
		SessionEgress: noopSessionEgress{},
		Broker:        &BrokerConfig{ProxyURL: proxyURL, CACertPEM: caPEM},
	}

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "usr-test"})

	// 1. Default group + allow-all
	if _, err := svc.Create(ctx, CreateRequest{OwnerID: ws}); err != nil {
		t.Fatalf("create default allow-all: %v", err)
	}
	req1 := created[len(created)-1]
	if req1.Metadata[sessionegress.SandboxGroupLabel] != defaultGroupID {
		t.Fatalf("default create group label = %q, want %q", req1.Metadata[sessionegress.SandboxGroupLabel], defaultGroupID)
	}
	if req1.Env["HTTPS_PROXY"] != proxyURL || req1.Env["SSL_CERT_FILE"] != SandboxBrokerCABundleFile || req1.Env["HOME"] != "/home/render" {
		t.Fatalf("default allow-all env missing broker wiring: %v", req1.Env)
	}

	// 2. Explicit group + allow-list
	if _, err := svc.Create(ctx, CreateRequest{
		OwnerID:        ws,
		SandboxGroupID: explicitGroupID,
		NetworkPolicy: &NetworkPolicy{
			Default: NetworkPolicyAllowList,
			Rules:   []NetworkPolicyRule{{Domain: "api.example.com", Protocol: NetworkPolicyProtocolHTTPS}},
		},
	}); err != nil {
		t.Fatalf("create explicit allow-list: %v", err)
	}
	req2 := created[len(created)-1]
	if req2.Metadata[sessionegress.SandboxGroupLabel] != explicitGroupID {
		t.Fatalf("explicit create group label = %q, want %q", req2.Metadata[sessionegress.SandboxGroupLabel], explicitGroupID)
	}
	if req2.Env["HTTPS_PROXY"] != proxyURL || req2.Env["NODE_EXTRA_CA_CERTS"] != SandboxBrokerCACertFile {
		t.Fatalf("allow-list env missing broker wiring: %v", req2.Env)
	}

	// 3. Explicit group + deny-all: stamps group label, ZERO proxy/CA wiring
	if _, err := svc.Create(ctx, CreateRequest{
		OwnerID:        ws,
		SandboxGroupID: explicitGroupID,
		NetworkPolicy:  &NetworkPolicy{Default: NetworkPolicyDenyAll},
	}); err != nil {
		t.Fatalf("create deny-all: %v", err)
	}
	req3 := created[len(created)-1]
	if req3.Metadata[sessionegress.SandboxGroupLabel] != explicitGroupID {
		t.Fatalf("deny-all group label = %q, want %q", req3.Metadata[sessionegress.SandboxGroupLabel], explicitGroupID)
	}
	if _, ok := req3.Env["HTTPS_PROXY"]; ok {
		t.Fatalf("deny-all must not inject HTTPS_PROXY: %v", req3.Env)
	}
	if strings.Contains(strings.Join(req3.Entrypoint, " "), "sandbox-broker-ca") {
		t.Fatalf("deny-all must not inject broker CA bootstrap: %v", req3.Entrypoint)
	}

	// 4. Agent-session sandbox: NO bex.co/sandbox-group label, NO broker proxy env
	sessionID := ids.New(ids.AgentSession)
	if _, err := NewAgentSessionLifecycle(svc).CreateAgentSessionSandbox(ctx, AgentSessionSandbox{
		WorkspaceID:   ws,
		Template:      "agent",
		SessionID:     sessionID,
		Repository:    "bex-co/bex",
		Branch:        "bex-agent/test",
		ModelEndpoint: "api.anthropic.com",
		ModelAPIKey:   agentsession.ModelKeyPlaceholder(sessionID),
	}); err != nil {
		t.Fatalf("create agent session sandbox: %v", err)
	}
	req4 := created[len(created)-1]
	if _, hasGroup := req4.Metadata[sessionegress.SandboxGroupLabel]; hasGroup {
		t.Fatalf("agent-session sandbox must never carry %s", sessionegress.SandboxGroupLabel)
	}
	if _, hasProxy := req4.Env["HTTPS_PROXY"]; hasProxy {
		t.Fatalf("agent-session sandbox must never receive /v1/sandboxes broker HTTPS_PROXY: %v", req4.Env)
	}

	// 5. Resuming a pre-m535 suspended sandbox missing bex.co/sandbox-group patches it before resume.
	legacyPubID := ids.New(ids.Sandbox)
	groups.members[legacyPubID] = store.SandboxGroupMember{GroupID: explicitGroupID}
	boxes["os-legacy"] = osSandbox{
		ID: "os-legacy",
		Metadata: map[string]string{
			metadataOwner:         "usr-test",
			metadataWorkspace:     ws,
			metadataRegime:        metadataSandboxRegime,
			metadataNetworkPolicy: string(NetworkPolicyAllowAll),
			metadataEgress:        string(NetworkPolicyAllowAll),
			metadataPublicID:      legacyPubID,
			metadataPlan:          string(Plan1C2G),
			metadataTimeout:       "3600",
			metadataIdleTimeout:   "900",
		},
	}
	b := boxes["os-legacy"]
	b.Status.State = "Paused"
	boxes["os-legacy"] = b

	if err := svc.Resume(core.WithWorkspace(ctx, ws), legacyPubID); err != nil {
		t.Fatalf("resume legacy suspended sandbox: %v", err)
	}
	if len(patched) == 0 || patched[len(patched)-1][sessionegress.SandboxGroupLabel] != explicitGroupID {
		t.Fatalf("resume patch = %v, want %s=%s", patched, sessionegress.SandboxGroupLabel, explicitGroupID)
	}
}
