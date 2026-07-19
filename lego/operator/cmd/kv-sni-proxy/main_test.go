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
	"net/netip"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/operator/internal/sniproxy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestRouterRoutesPublicKeyValueAndPreservesAllowlist(t *testing.T) {
	router := newRouter("kv.bex.co")
	kv := &appv1alpha1.KeyValue{}
	kv.Name = "kv-one"
	kv.Namespace = "default"
	kv.Spec.Public = true
	kv.Status.ExternalHost = "kv-one.kv.bex.co"
	kv.Spec.IPAllowList = []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.0/24"}}
	kv.Spec.EnvironmentIPAllowList = []string{"203.0.113.0/28"}
	if err := router.set(kv); err != nil {
		t.Fatal(err)
	}
	route, ok := router.resolve("kv-one.kv.bex.co", netip.MustParseAddr("203.0.113.9"))
	if !ok {
		t.Fatal("allowlisted client did not resolve")
	}
	if route.ResourceID != "kv-one" || route.Backend != "kv-one.default.svc.cluster.local:6380" {
		t.Fatalf("route = %#v", route)
	}
	if _, ok := router.resolve("kv-one.kv.bex.co", netip.MustParseAddr("198.51.100.9")); ok {
		t.Fatal("non-allowlisted client resolved")
	}
	if _, ok := router.resolve("kv-one.kv.bex.co", netip.MustParseAddr("203.0.113.20")); ok {
		t.Fatal("client outside the environment allowlist resolved")
	}
	if _, ok := router.resolve("other.kv.bex.co", netip.MustParseAddr("203.0.113.9")); ok {
		t.Fatal("unknown SNI resolved")
	}
}

func TestRouterPrivateAndInvalidCIDRFailClosed(t *testing.T) {
	router := newRouter("kv.bex.co")
	pending := &appv1alpha1.KeyValue{}
	pending.Name = "pending"
	pending.Spec.Public = true
	if err := router.set(pending); err != nil {
		t.Fatal(err)
	}
	if _, ok := router.resolve("pending.kv.bex.co", netip.MustParseAddr("1.1.1.1")); ok {
		t.Fatal("public spec without an operator-published external host gained a route")
	}
	private := &appv1alpha1.KeyValue{}
	private.Name = "private"
	if err := router.set(private); err != nil {
		t.Fatal(err)
	}
	if _, ok := router.resolve("private.kv.bex.co", netip.MustParseAddr("1.1.1.1")); ok {
		t.Fatal("private KeyValue gained a route")
	}
	private.Spec.Public = true
	private.Status.ExternalHost = "private.kv.bex.co"
	private.Spec.IPAllowList = []appv1alpha1.IPAllowEntry{{CIDR: "broken"}}
	if err := router.set(private); err == nil {
		t.Fatal("invalid allowlist must make the source unhealthy")
	}
	valid := &appv1alpha1.KeyValue{}
	valid.Name = "valid"
	valid.Namespace = "default"
	valid.Spec.Public = true
	valid.Status.ExternalHost = "valid.kv.bex.co"
	if err := router.set(valid); err != nil {
		t.Fatal(err)
	}
	if router.healthy() {
		t.Fatal("a valid reconcile masked another KeyValue's invalid route")
	}
	router.delete(private.Name)
	if !router.healthy() {
		t.Fatal("deleting the invalid KeyValue did not restore source health")
	}
}

func TestPublicProxyPreservesEndToEndTLSAndMetersOnlyBackendWrites(t *testing.T) {
	const host = "kv-one.kv.bex.co"
	certificate, roots := testCertificate(t, host)
	backendListener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = backendListener.Close() }()
	backendDone := make(chan error, 1)
	go func() {
		conn, acceptErr := backendListener.Accept()
		if acceptErr != nil {
			backendDone <- acceptErr
			return
		}
		defer func() { _ = conn.Close() }()
		request := make([]byte, 32*1024)
		if _, readErr := io.ReadFull(conn, request); readErr != nil {
			backendDone <- readErr
			return
		}
		_, writeErr := conn.Write([]byte("+PONG\r\n"))
		backendDone <- writeErr
	}()

	registry := prometheus.NewRegistry()
	meter := sniproxy.NewByteMeter(registry, "kv_proxy", "key_value")
	router := newRouter("kv.bex.co")
	router.table["kv-one"] = kvRoute{
		ResourceID: "kv-one",
		Backend:    backendListener.Addr().String(),
		Allow:      []netip.Prefix{netip.MustParsePrefix("203.0.113.9/32")},
		EnvAllow:   []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")},
	}
	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = proxyListener.Close() }()
	proxyDone := make(chan struct{})
	go func() {
		conn, acceptErr := proxyListener.Accept()
		if acceptErr == nil {
			handleConn(conn, router, meter, []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}, discardLogger{})
		}
		close(proxyDone)
	}()

	rawClient, err := net.Dial("tcp", proxyListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rawClient.Write([]byte("PROXY TCP4 203.0.113.9 49.12.20.236 49152 6379\r\n")); err != nil {
		t.Fatal(err)
	}
	client := tls.Client(rawClient, &tls.Config{
		RootCAs: roots, ServerName: host, MinVersion: tls.VersionTLS12,
	})
	if err := client.Handshake(); err != nil {
		t.Fatalf("TLS through pass-through proxy: %v", err)
	}
	if got := client.ConnectionState().PeerCertificates[0].DNSNames; len(got) != 1 || got[0] != host {
		t.Fatalf("client saw proxy certificate instead of backend certificate: %v", got)
	}
	before := gatheredCounter(t, registry, "bex_kv_proxy_egress_bytes_total")
	request := make([]byte, 32*1024)
	if _, err := client.Write(request); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, len("+PONG\r\n"))
	if _, err := io.ReadFull(client, response); err != nil {
		t.Fatal(err)
	}
	after := gatheredCounter(t, registry, "bex_kv_proxy_egress_bytes_total")
	if delta := after - before; delta <= 0 || delta >= float64(len(request)) {
		t.Fatalf("meter delta = %v; want backend TLS response bytes only, never %d request bytes", delta, len(request))
	}
	_ = client.Close()
	select {
	case err := <-backendDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("backend did not finish")
	}
	select {
	case <-proxyDone:
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not finish")
	}
}

type discardLogger struct{}

func (discardLogger) Info(string, ...any)         {}
func (discardLogger) Error(error, string, ...any) {}

func gatheredCounter(t *testing.T, registry *prometheus.Registry, name string) float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() == name && len(family.Metric) == 1 {
			return family.Metric[0].GetCounter().GetValue()
		}
	}
	return 0
}

func testCertificate(t *testing.T, host string) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
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
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certPEM) {
		t.Fatal("append test root")
	}
	return certificate, roots
}
