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
)

func TestAdmissionListenerRejectsSlowHeadersBeforeHandler(t *testing.T) {
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := newAdmissionListener(inner, 1, 1)
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
