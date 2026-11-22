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

// Package sandboxbroker implements the Pod-bound outbound HTTPS credential
// broker inside bex-ssh-gateway (w2/m534, docs/ADR122-sandbox-credential-brokering.md).
package sandboxbroker

import (
	"container/list"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

const (
	defaultLeafCacheMaxEntries = 4096
	defaultLeafTTL             = time.Hour
	leafBackdateSkew           = 5 * time.Minute
)

// CAOptions configures the Sandbox Broker CA leaf signer. Zero values select
// safe defaults (4096 cached hosts, 1h leaf lifetime, system wall clock).
type CAOptions struct {
	MaxEntries int
	LeafTTL    time.Duration
	Now        func() time.Time
}

// CA holds the cluster-private Sandbox Broker CA keypair and mints short-lived
// exact-hostname ECDSA P-256 server leaf certificates in a bounded LRU cache.
// The private key lives exclusively inside bex-ssh-gateway.
type CA struct {
	caCert     *x509.Certificate
	caKey      crypto.Signer
	maxEntries int
	leafTTL    time.Duration
	nowFn      func() time.Time

	mu      sync.Mutex
	lru     *list.List
	entries map[string]*list.Element
}

type leafCacheEntry struct {
	host         string
	cert         *tls.Certificate
	refreshAfter time.Time
}

// LoadCA loads the Sandbox Broker CA certificate and private key from PEM
// files on disk. Fails closed on missing, unreadable, malformed, mismatched,
// non-CA, or expired certificates.
func LoadCA(certPath, keyPath string, opts CAOptions) (*CA, error) {
	if strings.TrimSpace(certPath) == "" || strings.TrimSpace(keyPath) == "" {
		return nil, errors.New("sandbox broker CA cert and key paths are required")
	}
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read sandbox broker CA cert: %w", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read sandbox broker CA key: %w", err)
	}
	return NewCA(certPEM, keyPEM, opts)
}

// NewCA validates a PEM-encoded CA certificate and private key and constructs a
// bounded in-memory leaf signer.
func NewCA(certPEM, keyPEM []byte, opts CAOptions) (*CA, error) {
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return nil, errors.New("sandbox broker CA cert and key PEM must not be empty")
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse sandbox broker CA keypair: %w", err)
	}
	if len(pair.Certificate) == 0 {
		return nil, errors.New("sandbox broker CA contains no certificate")
	}
	caCert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse sandbox broker CA x509 certificate: %w", err)
	}
	if !caCert.IsCA {
		return nil, errors.New("sandbox broker certificate is not a CA (IsCA=false)")
	}
	signer, ok := pair.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, errors.New("sandbox broker CA private key does not implement crypto.Signer")
	}
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn().UTC()
	if now.Before(caCert.NotBefore) || !now.Before(caCert.NotAfter) {
		return nil, errors.New("sandbox broker CA certificate is expired or not yet valid")
	}
	maxEntries := opts.MaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultLeafCacheMaxEntries
	}
	leafTTL := opts.LeafTTL
	if leafTTL <= 0 {
		leafTTL = defaultLeafTTL
	}
	return &CA{
		caCert:     caCert,
		caKey:      signer,
		maxEntries: maxEntries,
		leafTTL:    leafTTL,
		nowFn:      nowFn,
		lru:        list.New(),
		entries:    make(map[string]*list.Element),
	}, nil
}

// CertPEM returns the PEM-encoded public CA certificate (never the private
// key), suitable for distribution to bex-api and sandbox Pod trust stores.
func (c *CA) CertPEM() []byte {
	if c == nil || c.caCert == nil || len(c.caCert.Raw) == 0 {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.caCert.Raw})
}

// CertForHost returns a cached or freshly minted short-lived ECDSA P-256 TLS
// server certificate for an exact public DNS hostname. Never issues wildcards
// or IP SANs.
func (c *CA) CertForHost(host string) (*tls.Certificate, error) {
	if c == nil || c.caCert == nil || c.caKey == nil {
		return nil, errors.New("sandbox broker CA is not initialized")
	}
	// Exact public DNS hostname validation (rejects wildcards, IP literals,
	// ports, single-label names, and private/cluster hostnames).
	normalized, err := sandbox.NormalizeCredentialDomain(host)
	if err != nil {
		return nil, fmt.Errorf("sandbox broker leaf host rejected: %w", err)
	}
	now := c.nowFn().UTC()

	c.mu.Lock()
	if elem, ok := c.entries[normalized]; ok {
		entry := elem.Value.(*leafCacheEntry)
		if now.Before(entry.refreshAfter) {
			c.lru.MoveToFront(elem)
			cert := entry.cert
			c.mu.Unlock()
			return cert, nil
		}
		c.lru.Remove(elem)
		delete(c.entries, normalized)
	}
	c.mu.Unlock()

	cert, refreshAfter, err := c.mintLeaf(normalized, now)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.entries[normalized]; ok {
		entry := elem.Value.(*leafCacheEntry)
		if now.Before(entry.refreshAfter) {
			c.lru.MoveToFront(elem)
			return entry.cert, nil
		}
		c.lru.Remove(elem)
		delete(c.entries, normalized)
	}
	elem := c.lru.PushFront(&leafCacheEntry{
		host:         normalized,
		cert:         cert,
		refreshAfter: refreshAfter,
	})
	c.entries[normalized] = elem
	for len(c.entries) > c.maxEntries {
		tail := c.lru.Back()
		if tail == nil {
			break
		}
		evicted := tail.Value.(*leafCacheEntry)
		c.lru.Remove(tail)
		delete(c.entries, evicted.host)
	}
	return cert, nil
}

func (c *CA) mintLeaf(host string, now time.Time) (*tls.Certificate, time.Time, error) {
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("generate leaf key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("generate leaf serial: %w", err)
	}
	notBefore := now.Add(-leafBackdateSkew)
	notAfter := now.Add(c.leafTTL)
	// Refresh before half-life so callers never see a near-expiry leaf.
	refreshAfter := now.Add(c.leafTTL / 2)

	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: host},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		DNSNames:              []string{host},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.caCert, &leafKey.PublicKey, c.caKey)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("sign leaf certificate: %w", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("parse signed leaf certificate: %w", err)
	}
	tlsCert := &tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  leafKey,
		Leaf:        leaf,
	}
	return tlsCert, refreshAfter, nil
}

func randomSerial() (*big.Int, error) {
	// 127-bit positive serial number (RFC 5280 <= 20 octets, positive, unique).
	limit := new(big.Int).Lsh(big.NewInt(1), 127)
	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, err
	}
	if n.Sign() == 0 {
		n.SetInt64(1)
	}
	return n, nil
}
