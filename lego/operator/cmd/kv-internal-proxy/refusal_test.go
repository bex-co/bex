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

package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeCertificate writes a self-signed certificate for kv.example.test as
// the PEM files of a store's <id>-kv-tls Secret, and a pool that trusts it.
func writeCertificate(t *testing.T) (certFile, keyFile string, roots *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "kv.example.test"}, DNSNames: []string{"kv.example.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile = filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	for file, block := range map[string]*pem.Block{
		certFile: {Type: "CERTIFICATE", Bytes: der},
		keyFile:  {Type: "PRIVATE KEY", Bytes: keyDER},
	} {
		if err := os.WriteFile(file, pem.EncodeToMemory(block), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots = x509.NewCertPool()
	roots.AddCert(cert)
	return certFile, keyFile, roots
}

// BEX_KV_ALLOWLIST_REFUSAL_ADDR (w8/m562): a client of the refusal listener
// completes TLS under the store's certificate and reads Render's refusal for
// its opening AUTH and for the command after it, and the store's upstream
// sees no connection from it.
func TestRunAnswersARefusedExternalClientWithRendersText(t *testing.T) {
	const password = "secret"
	seen := make(chan string, 4)
	upstream := fakeValkey(t, password, seen)
	certFile, keyFile, roots := writeCertificate(t)
	addr, refusalAddr := freeAddr(t), freeAddr(t)
	env := map[string]string{
		"BEX_KV_UPSTREAM_ADDR": upstream, "VALKEY_PASSWORD": password, "BEX_KV_INTERNAL_ADDR": addr,
		"BEX_KV_ALLOWLIST_REFUSAL_ADDR": refusalAddr, "BEX_KV_TLS_CERT_FILE": certFile, "BEX_KV_TLS_KEY_FILE": keyFile,
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- run(ctx, func(key string) string { return env[key] }, false, nil) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	var raw net.Conn
	var err error
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if raw, err = net.Dial("tcp", refusalAddr); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = raw.Close() })
	_ = raw.SetDeadline(time.Now().Add(3 * time.Second))
	client := tls.Client(raw, &tls.Config{ServerName: "kv.example.test", RootCAs: roots, MinVersion: tls.VersionTLS12})
	const refusal = "-Client IP address is not in the allowlist.\r\n"
	for _, command := range []string{
		"*3\r\n$4\r\nAUTH\r\n$7\r\ndefault\r\n$6\r\nsecret\r\n",
		"*1\r\n$4\r\nPING\r\n",
	} {
		if _, err := io.WriteString(client, command); err != nil {
			t.Fatal(err)
		}
		reply := make([]byte, len(refusal))
		if _, err := io.ReadFull(client, reply); err != nil || string(reply) != refusal {
			t.Fatalf("%q read %q (%v), want Render's refusal", command, reply, err)
		}
	}
	select {
	case got := <-seen:
		t.Fatalf("the store's upstream read %q from a refused client", got)
	default:
	}
}
