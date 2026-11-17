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
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/operator/internal/kvinternalauth"
	"github.com/bex-co/bex/lego/operator/internal/sniproxy"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m457: kv-sni-proxy held every store's external clients to 64 and closed
// the 65th. A store's route now admits twice its plan's limit, so the client
// past the limit reaches kv-internal-proxy, which answers it inside TLS with
// Valkey's own refusal (w6/m456).

// tlsUpstream stands in for valkey-server's TLS port: it completes each
// handshake with the store's certificate and holds the connection.
func tlsUpstream(t *testing.T, certificate tls.Certificate) net.Listener {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var held []net.Conn
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			held = append(held, conn)
			mu.Unlock()
			go func() { _, _ = io.Copy(io.Discard, conn) }()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range held {
			_ = conn.Close()
		}
	})
	return ln
}

// writeKeyPair writes certificate as the PEM files of a store's <id>-kv-tls
// Secret, which kv-internal-proxy mounts to answer a refused TLS client.
func writeKeyPair(t *testing.T, certificate tls.Certificate) (certFile, keyFile string) {
	t.Helper()
	keyDER, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile = filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	for file, block := range map[string]*pem.Block{
		certFile: {Type: "CERTIFICATE", Bytes: certificate.Certificate[0]},
		keyFile:  {Type: "PRIVATE KEY", Bytes: keyDER},
	} {
		if err := os.WriteFile(file, pem.EncodeToMemory(block), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return certFile, keyFile
}

// TestAStoreAtItsLimitAnswersTheNextExternalClientWithValkeysRefusal drives a
// starter store (Render's 250 connections) end to end: kv-sni-proxy with its
// shipped process caps, then kv-internal-proxy's passthrough listener holding
// the plan's budget, then a TLS upstream for valkey-server. All 250 external
// clients are relayed to the store, and the 251st completes TLS with the
// store's certificate and reads "-ERR max number of clients reached", not a
// dropped connection; kv-sni-proxy sheds none of them.
func TestAStoreAtItsLimitAnswersTheNextExternalClientWithValkeysRefusal(t *testing.T) {
	const host = "red-one.kv.bex.co"
	certificate, roots := testCertificate(t, host)
	certFile, keyFile := writeKeyPair(t, certificate)
	limit := int(tiers.Valkey.Resolve("starter").ConnectionLimit)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	internal, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = kvinternalauth.Serve(ctx, internal, kvinternalauth.Config{
			Upstream: tlsUpstream(t, certificate).Addr().String(), Passthrough: true,
			Budget: kvinternalauth.NewBudget(limit), TLSCertFile: certFile, TLSKeyFile: keyFile,
		})
	}()

	router := newRouter("kv.bex.co")
	kv := &appv1alpha1.KeyValue{}
	kv.Name, kv.Namespace = "red-one", "tea-one"
	kv.Spec.Public, kv.Spec.Plan = true, "starter"
	kv.Status.ExternalHost = host
	if err := router.set(kv); err != nil {
		t.Fatal(err)
	}
	route := router.table["red-one"]
	route.Backend = internal.Addr().String()
	router.table["red-one"] = route

	registry := prometheus.NewRegistry()
	meter := sniproxy.NewByteMeter(registry, "kv_proxy", "key_value")
	limiter := sniproxy.NewLimiter(defaultMaxConns, defaultMaxConnsPerSource)
	limiter.SetHandshakeLimit(defaultMaxHandshakesPerNetwork)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go sniproxy.Serve(ctx, ln, limiter, meter, func(conn net.Conn) {
		handleConn(conn, router, meter, nil, limiter, 0, 0, discardLogger{})
	}, nil)

	dial := func() *tls.Conn {
		t.Helper()
		raw, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		conn := tls.Client(raw, &tls.Config{RootCAs: roots, ServerName: host, MinVersion: tls.VersionTLS12})
		t.Cleanup(func() { _ = conn.Close() })
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		if err := conn.Handshake(); err != nil {
			t.Fatalf("TLS through both proxies: %v", err)
		}
		return conn
	}
	admitted := make([]*tls.Conn, limit)
	for i := range admitted {
		admitted[i] = dial()
	}
	refused := dial()
	// A relayed client reads nothing from the holding upstream; a refused one
	// would read the refusal at once.
	var relayed sync.WaitGroup
	notRelayed := make(chan error, limit)
	for _, conn := range admitted {
		relayed.Go(func() {
			_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			var timeout net.Error
			if _, err := conn.Read(make([]byte, 1)); !errors.As(err, &timeout) || !timeout.Timeout() {
				notRelayed <- err
			}
		})
	}
	relayed.Wait()
	close(notRelayed)
	if n := len(notRelayed); n > 0 {
		t.Fatalf("%d of the plan's %d clients were not relayed to the store: %v", n, limit, <-notRelayed)
	}

	reply, err := io.ReadAll(refused)
	if err != nil {
		t.Fatalf("client %d read %q, then %v", limit+1, reply, err)
	}
	if string(reply) != "-ERR max number of clients reached\r\n" {
		t.Fatalf("client %d read %q, want Valkey's refusal", limit+1, reply)
	}
	for _, reason := range []string{sniproxy.ScopeResource, sniproxy.ScopeHandshake, "source", "global"} {
		if got := rejectedCount(t, registry, reason); got != 0 {
			t.Fatalf("kv-sni-proxy shed %v clients (%s) below the store's refusal", got, reason)
		}
	}
}
