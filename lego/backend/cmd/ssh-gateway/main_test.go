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
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
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
	handler := metricsHandler(registry)

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("healthz = %d, want 200", health.Code)
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
