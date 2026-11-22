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
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/agentcred"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestAdmissionListenerRejectsSlowHeadersBeforeHandler(t *testing.T) {
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := newAdmissionListener(inner, 1, 1, sshgateway.NewMetrics(prometheus.NewRegistry()))
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Close() }()

	first, err := net.Dial("tcp", inner.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Close() }()
	second, err := net.Dial("tcp", inner.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = second.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := second.Read(make([]byte, 1)); err == nil {
		t.Fatal("connection above pre-parser cap remained open")
	}
	_ = second.Close()

	_ = first.Close()
	var response *http.Response
	for attempt := 0; attempt < 20; attempt++ {
		conn, dialErr := net.Dial("tcp", inner.Addr().String())
		if dialErr == nil {
			_, _ = io.WriteString(conn, "GET /healthz HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n")
			response, err = http.ReadResponse(bufio.NewReader(conn), nil)
			_ = conn.Close()
			if err == nil {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if response == nil || response.StatusCode != http.StatusOK {
		t.Fatalf("admission did not recover after release: response=%v err=%v", response, err)
	}
}

// shedCounter reads bex_ssh_gateway_limit_rejections_total{scope=<scope>} from
// the registry; a family or scope never incremented reads as 0.
func shedCounter(t *testing.T, registry *prometheus.Registry, scope string) float64 {
	t.Helper()
	return gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_limit_rejections_total", map[string]string{"scope": scope})
}

// admissionHarness drives one admissionListener directly: an Accept loop feeds
// admitted connections to the channel, and shed connections surface to their
// dialers as an immediate close (EOF on read).
func admissionHarness(t *testing.T, global, perSource int, metrics *sshgateway.Metrics) (string, chan net.Conn) {
	t.Helper()
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := newAdmissionListener(inner, global, perSource, metrics)
	t.Cleanup(func() { _ = listener.Close() })
	admitted := make(chan net.Conn, 8)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			admitted <- conn
		}
	}()
	return inner.Addr().String(), admitted
}

// dialAdmitted opens a connection the harness admits (visible on the channel).
func dialAdmitted(t *testing.T, addr string, admitted chan net.Conn) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	select {
	case server := <-admitted:
		t.Cleanup(func() { _ = server.Close() })
	case <-time.After(2 * time.Second):
		t.Fatal("connection under the cap was not admitted")
	}
	return conn
}

// dialShed opens a connection the listener must shed and waits for the close.
func dialShed(t *testing.T, addr string) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("connection above the cap remained open")
	}
}

// w1/m76/t004: a shed on an aux gateway listener must increment
// bex_ssh_gateway_limit_rejections_total with a listener scope — floods against
// the web-shell/sandbox-exec/attach/git/model listeners were the fleet's only
// invisible shed path.
func TestAdmissionListenerEmitsShedMetricPerScope(t *testing.T) {
	t.Run("listener_global", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		metrics := sshgateway.NewMetrics(registry)
		addr, admitted := admissionHarness(t, 1, 1, metrics)
		dialAdmitted(t, addr, admitted)
		dialShed(t, addr)
		if got := shedCounter(t, registry, "listener_global"); got != 1 {
			t.Fatalf("limit_rejections_total{scope=listener_global} = %v, want 1", got)
		}
		if got := shedCounter(t, registry, "listener_source"); got != 0 {
			t.Fatalf("limit_rejections_total{scope=listener_source} = %v, want 0", got)
		}
	})
	t.Run("listener_source", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		metrics := sshgateway.NewMetrics(registry)
		// Global headroom remains (2), so the second same-source connection is
		// shed by the per-source share, not the global cap.
		addr, admitted := admissionHarness(t, 2, 1, metrics)
		dialAdmitted(t, addr, admitted)
		dialShed(t, addr)
		if got := shedCounter(t, registry, "listener_source"); got != 1 {
			t.Fatalf("limit_rejections_total{scope=listener_source} = %v, want 1", got)
		}
		if got := shedCounter(t, registry, "listener_global"); got != 0 {
			t.Fatalf("limit_rejections_total{scope=listener_global} = %v, want 0", got)
		}
	})
	t.Run("admitted connections emit nothing", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		metrics := sshgateway.NewMetrics(registry)
		addr, admitted := admissionHarness(t, 2, 2, metrics)
		dialAdmitted(t, addr, admitted)
		dialAdmitted(t, addr, admitted)
		for _, scope := range []string{"listener_global", "listener_source"} {
			if got := shedCounter(t, registry, scope); got != 0 {
				t.Fatalf("limit_rejections_total{scope=%s} = %v, want 0", scope, got)
			}
		}
	})
}

func TestMetricsHandlerServesHealthAndPrometheus(t *testing.T) {
	registry := prometheus.NewRegistry()
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "bex_ssh_gateway_test_metric"})
	registry.MustRegister(gauge)
	gauge.Set(1)
	ready := true
	handler := metricsHandler(registry, func() bool { return ready })

	probe := func(path string) int {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		return recorder.Code
	}
	if got := probe("/healthz"); got != http.StatusOK {
		t.Fatalf("healthz = %d, want 200", got)
	}
	if got := probe("/readyz"); got != http.StatusOK {
		t.Fatalf("readyz = %d, want 200 while the listeners accept", got)
	}
	// A terminating pod (w6/m482, w6/152) is no longer ready, so no Service
	// endpoint sends it new connections; it stays live, so the kubelet never
	// kills it.
	ready = false
	if got := probe("/readyz"); got != http.StatusServiceUnavailable {
		t.Fatalf("readyz = %d, want 503 once SIGTERM arrived", got)
	}
	if got := probe("/healthz"); got != http.StatusOK {
		t.Fatalf("healthz = %d, want 200 while draining", got)
	}
	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if metrics.Code != http.StatusOK || !strings.Contains(metrics.Body.String(), "bex_ssh_gateway_test_metric 1") {
		t.Fatalf("metrics endpoint = %d without registered collector", metrics.Code)
	}
}

func TestHostSignerRequiresEd25519(t *testing.T) {
	_, edPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	edDER, err := x509.MarshalPKCS8PrivateKey(edPrivate)
	if err != nil {
		t.Fatal(err)
	}
	edPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: edDER})
	if _, err := parseHostSigner(edPEM); err != nil {
		t.Fatalf("Ed25519 host key rejected: %v", err)
	}
	rsaPrivate, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rsaPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaPrivate)})
	if _, err := parseHostSigner(rsaPEM); err == nil {
		t.Fatal("RSA host key accepted, want Ed25519-only startup")
	}
	if _, err := parseHostSigner([]byte("not a key")); err == nil {
		t.Fatal("malformed host key accepted")
	}
}

// Every child-session method the driver sends reaches the spawn handler: a
// method missing here answers 405 in production while handler tests pass.
func TestCredentialMuxRoutesEveryChildSessionMethod(t *testing.T) {
	mux := newCredentialMux(&agentcred.Broker{})
	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
		req := httptest.NewRequest(method, agentsession.SpawnProxyPath+"bnM/c2Vzcw/ags-c185th5c2rvvnhbfiltg", nil)
		if _, pattern := mux.Handler(req); pattern != method+" "+agentsession.SpawnProxyPath {
			t.Errorf("%s child-session route = %q", method, pattern)
		}
	}
}

// R25-12 (w21/074): the listener's per-source cap counts an IPv6 client under its
// /64, so rotating through the addresses of its own prefix does not dodge it.
func TestConnectionSourceKeysIPv6OnItsSlash64(t *testing.T) {
	tcp := func(addr string) net.Addr { return &net.TCPAddr{IP: net.ParseIP(addr), Port: 40000} }
	if a, b := connectionSource(tcp("2001:db8:1:2::1")), connectionSource(tcp("2001:db8:1:2:ffff::9")); a != b {
		t.Errorf("two addresses of one /64 are two sources: %q and %q", a, b)
	}
	if a, b := connectionSource(tcp("2001:db8:1:2::1")), connectionSource(tcp("2001:db8:1:3::1")); a == b {
		t.Errorf("two /64s share a source: %q", a)
	}
	if a, b := connectionSource(tcp("203.0.113.1")), connectionSource(tcp("203.0.113.2")); a == b {
		t.Errorf("two IPv4 addresses share a source: %q", a)
	}
}

type fakeTenantLister []store.Tenant

func (f fakeTenantLister) ListTenants(context.Context) ([]store.Tenant, error) { return f, nil }

func TestBuildSandboxBrokerConfig(t *testing.T) {
	metrics := sshgateway.NewMetrics(prometheus.NewRegistry())
	k8s := fake.NewSimpleClientset()
	tenants := fakeTenantLister{{ID: "tea-1"}}

	// 1. Completely unconfigured -> skipped cleanly with no error.
	envMap := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	broker, addr, err := buildSandboxBroker(envMap(nil), "", k8s, tenants, metrics)
	if err != nil || broker != nil || addr != "" {
		t.Fatalf("unconfigured = (%v, %q, %v), want (nil, \"\", nil)", broker, addr, err)
	}

	// Write a valid test Broker CA keypair.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-broker-ca"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}

	// 2. Partial configs fail closed.
	partials := []struct {
		name   string
		vars   map[string]string
		secret string
		st     tenantLister
	}{
		{"addr only without CA", map[string]string{"BEX_SANDBOX_BROKER_ADDR": ":8086"}, "sec", tenants},
		{"cert without key", map[string]string{"BEX_SANDBOX_BROKER_CA_CERT_PATH": certPath}, "sec", tenants},
		{"CA without exec secret", map[string]string{"BEX_SANDBOX_BROKER_CA_CERT_PATH": certPath, "BEX_SANDBOX_BROKER_CA_KEY_PATH": keyPath}, "", tenants},
		{"CA without store", map[string]string{"BEX_SANDBOX_BROKER_CA_CERT_PATH": certPath, "BEX_SANDBOX_BROKER_CA_KEY_PATH": keyPath}, "sec", nil},
		{"invalid cache ttl", map[string]string{"BEX_SANDBOX_BROKER_CA_CERT_PATH": certPath, "BEX_SANDBOX_BROKER_CA_KEY_PATH": keyPath, "BEX_SANDBOX_BROKER_CACHE_TTL": "-1s"}, "sec", tenants},
	}
	for _, tc := range partials {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := buildSandboxBroker(envMap(tc.vars), tc.secret, k8s, tc.st, metrics); err == nil {
				t.Fatal("expected partial config to fail closed")
			}
		})
	}

	// 3. Complete config succeeds with default :8086 and Enabled() == true.
	broker, addr, err = buildSandboxBroker(envMap(map[string]string{
		"BEX_SANDBOX_BROKER_CA_CERT_PATH": certPath,
		"BEX_SANDBOX_BROKER_CA_KEY_PATH":  keyPath,
	}), "0123456789abcdef", k8s, tenants, metrics)
	if err != nil {
		t.Fatalf("valid config failed: %v", err)
	}
	if broker == nil || !broker.Enabled() || addr != ":8086" {
		t.Fatalf("valid config = (enabled=%v, addr=%q), want (true, \":8086\")", broker != nil && broker.Enabled(), addr)
	}

	// 4. Public-only CA endpoint serves ONLY the CERTIFICATE PEM (never the key)
	// when enabled, and 404 when disabled.
	disabledRec := httptest.NewRecorder()
	sandboxBrokerCACertHandler(nil).ServeHTTP(disabledRec, httptest.NewRequest(http.MethodGet, sandboxBrokerCACertPath, nil))
	if disabledRec.Code != http.StatusNotFound {
		t.Fatalf("disabled CA endpoint = %d, want 404", disabledRec.Code)
	}

	enabledRec := httptest.NewRecorder()
	sandboxBrokerCACertHandler(broker).ServeHTTP(enabledRec, httptest.NewRequest(http.MethodGet, sandboxBrokerCACertPath, nil))
	if enabledRec.Code != http.StatusOK {
		t.Fatalf("enabled CA endpoint = %d, want 200", enabledRec.Code)
	}
	if ct := enabledRec.Header().Get("Content-Type"); ct != "application/x-pem-file" {
		t.Errorf("Content-Type = %q, want application/x-pem-file", ct)
	}
	if strings.Contains(enabledRec.Body.String(), "PRIVATE KEY") {
		t.Fatalf("CA endpoint leaked private key material")
	}
	if _, err := sandbox.ValidatePublicBrokerCAPEM(enabledRec.Body.Bytes(), time.Now()); err != nil {
		t.Fatalf("served CA PEM failed ValidatePublicBrokerCAPEM: %v", err)
	}
}
