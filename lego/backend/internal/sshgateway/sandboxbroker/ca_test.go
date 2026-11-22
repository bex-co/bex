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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func generateTestCAPEM(t *testing.T, isCA bool, notBefore, notAfter time.Time) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "bex-sandbox-broker-test-ca"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

func TestCALoadAndValidation(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	certPEM, keyPEM := generateTestCAPEM(t, true, now.Add(-time.Hour), now.Add(24*time.Hour))

	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}

	ca, err := LoadCA(certPath, keyPath, CAOptions{Now: clock})
	if err != nil {
		t.Fatalf("LoadCA valid: %v", err)
	}

	// Non-CA rejected.
	nonCACert, nonCAKey := generateTestCAPEM(t, false, now.Add(-time.Hour), now.Add(24*time.Hour))
	if _, err := NewCA(nonCACert, nonCAKey, CAOptions{Now: clock}); err == nil {
		t.Fatal("expected error for non-CA certificate")
	}

	// Expired CA rejected.
	expCert, expKey := generateTestCAPEM(t, true, now.Add(-48*time.Hour), now.Add(-time.Hour))
	if _, err := NewCA(expCert, expKey, CAOptions{Now: clock}); err == nil {
		t.Fatal("expected error for expired CA certificate")
	}

	// Mismatched keypair rejected.
	_, otherKey := generateTestCAPEM(t, true, now.Add(-time.Hour), now.Add(24*time.Hour))
	if _, err := NewCA(certPEM, otherKey, CAOptions{Now: clock}); err == nil {
		t.Fatal("expected error for mismatched keypair")
	}

	// Missing files rejected.
	if _, err := LoadCA(filepath.Join(dir, "missing.crt"), keyPath, CAOptions{Now: clock}); err == nil {
		t.Fatal("expected error for missing CA cert file")
	}
	if _, err := LoadCA("", "", CAOptions{Now: clock}); err == nil {
		t.Fatal("expected error for empty paths")
	}

	// Verify exact-hostname leaf against public CA root only.
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certPEM) {
		t.Fatal("failed to parse CA cert into root pool")
	}
	leaf, err := ca.CertForHost("API.Example.COM")
	if err != nil {
		t.Fatalf("CertForHost: %v", err)
	}
	if leaf.Leaf == nil {
		t.Fatal("expected parsed Leaf")
	}
	if len(leaf.Leaf.DNSNames) != 1 || leaf.Leaf.DNSNames[0] != "api.example.com" {
		t.Fatalf("DNSNames = %v, want [api.example.com]", leaf.Leaf.DNSNames)
	}
	if len(leaf.Leaf.IPAddresses) != 0 {
		t.Fatalf("expected zero IP SANs, got %v", leaf.Leaf.IPAddresses)
	}
	if leaf.Leaf.IsCA {
		t.Fatal("leaf must not be a CA")
	}
	if _, err := leaf.Leaf.Verify(x509.VerifyOptions{
		DNSName:     "api.example.com",
		Roots:       roots,
		CurrentTime: now,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Fatalf("verify exact host failed: %v", err)
	}
	if _, err := leaf.Leaf.Verify(x509.VerifyOptions{
		DNSName:     "other.example.com",
		Roots:       roots,
		CurrentTime: now,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err == nil {
		t.Fatal("expected verification failure for different hostname")
	}
}

func TestCARejectsInvalidHosts(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	certPEM, keyPEM := generateTestCAPEM(t, true, now.Add(-time.Hour), now.Add(24*time.Hour))
	ca, err := NewCA(certPEM, keyPEM, CAOptions{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	rejected := []string{
		"",
		"*.example.com",
		"127.0.0.1",
		"1.1.1.1",
		"::1",
		"[::1]",
		"localhost",
		"api.example.com:443",
		"bex-api.bex-system.svc.cluster.local",
		"foo.internal",
		"singlelabel",
		"bad host.com",
	}
	for _, host := range rejected {
		if _, err := ca.CertForHost(host); err == nil {
			t.Errorf("CertForHost(%q) succeeded, want error", host)
		}
	}
}

func TestCACacheRefreshEvictionAndConcurrency(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	certPEM, keyPEM := generateTestCAPEM(t, true, now.Add(-time.Hour), now.Add(24*time.Hour))
	ca, err := NewCA(certPEM, keyPEM, CAOptions{
		MaxEntries: 4,
		LeafTTL:    time.Hour,
		Now:        clock,
	})
	if err != nil {
		t.Fatal(err)
	}

	c1, err := ca.CertForHost("one.example.com")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := ca.CertForHost("one.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if c1.Leaf.SerialNumber.Cmp(c2.Leaf.SerialNumber) != 0 {
		t.Fatal("expected cache hit before half-life")
	}

	// Advance past half-life (30m): must mint a fresh cert with a new serial.
	mu.Lock()
	now = now.Add(31 * time.Minute)
	mu.Unlock()
	c3, err := ca.CertForHost("one.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if c1.Leaf.SerialNumber.Cmp(c3.Leaf.SerialNumber) == 0 {
		t.Fatal("expected fresh leaf cert with new serial past half-life")
	}

	// Bounded cache eviction: mint 10 distinct hosts with MaxEntries=4.
	for i := range 10 {
		if _, err := ca.CertForHost(fmt.Sprintf("h%d.example.com", i)); err != nil {
			t.Fatal(err)
		}
	}
	ca.mu.Lock()
	size := len(ca.entries)
	lruLen := ca.lru.Len()
	ca.mu.Unlock()
	if size > 4 || lruLen > 4 {
		t.Fatalf("cache grew past MaxEntries=4: entries=%d lru=%d", size, lruLen)
	}

	// Concurrency safety under -race.
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			host := fmt.Sprintf("concurrent-%d.example.com", i%6)
			if _, err := ca.CertForHost(host); err != nil {
				t.Errorf("concurrent CertForHost(%q): %v", host, err)
			}
		}(i)
	}
	wg.Wait()
}
