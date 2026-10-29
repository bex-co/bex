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

package httpport

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func endpoint(t *testing.T, handle func(net.Conn)) int32 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	active := make(map[net.Conn]struct{})
	var handlers sync.WaitGroup
	accepting := make(chan struct{})
	go func() {
		defer close(accepting)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			active[conn] = struct{}{}
			mu.Unlock()
			handlers.Go(func() {
				defer func() { _ = conn.Close(); mu.Lock(); delete(active, conn); mu.Unlock() }()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				handle(conn)
			})
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-accepting
		mu.Lock()
		for conn := range active {
			_ = conn.Close()
		}
		mu.Unlock()
		handlers.Wait()
	})
	return int32(listener.Addr().(*net.TCPAddr).Port)
}

// lowestHTTP is Probe's selected port for tests that only assert the choice.
func lowestHTTP(ctx context.Context, ip, host string, ports []int32) (int32, error) {
	result, err := Probe(ctx, ip, host, ports)
	return result.Port, err
}

func httpEndpoint(t *testing.T, line string) int32 {
	t.Helper()
	return endpoint(t, func(conn net.Conn) {
		if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
			return
		}
		// A complete status line is enough: never wait for headers or a body.
		_, _ = io.WriteString(conn, line)
	})
}

func TestProbeAcceptsAnyHTTPStatusAndVersionOne(t *testing.T) {
	for _, line := range []string{
		"HTTP/1.1 200 OK\r\n",
		"HTTP/1.1 301 Moved Permanently\r\n",
		"HTTP/1.1 401 Unauthorized\r\n",
		"HTTP/1.1 404 Not Found\r\n",
		"HTTP/1.1 500 Internal Server Error\r\n",
		"HTTP/1.0 204 No Content\r\n",
		"HTTP/1.1   503 Service Unavailable\r\n",
	} {
		t.Run(strings.TrimSpace(line), func(t *testing.T) {
			port := httpEndpoint(t, line)
			got, err := lowestHTTP(t.Context(), "127.0.0.1", "app.onbex.co", []int32{port})
			if err != nil || got != port {
				t.Fatalf("port=%d err=%v want=%d", got, err, port)
			}
		})
	}
}

func TestProbeRejectsNonHTTPAndOversizedStatusLines(t *testing.T) {
	for _, response := range []string{
		"\x00\x01\x00\x12binary\r\n",
		"SSH-2.0-test\r\n",
		"HTTP/2.0 200 OK\r\n",
		"HTTP/1.1 20 malformed\r\n",
		"HTTP/1.1 abc malformed\r\n",
		"HTTP/1.1 099 malformed\r\n",
		"HTTP/1.1 200 " + strings.Repeat("x", maxStatusLine) + "\r\n",
		"", // closes without replying
	} {
		port := httpEndpoint(t, response)
		got, err := Probe(t.Context(), "127.0.0.1", "app.onbex.co", []int32{port})
		if err != nil || got != (Result{}) {
			t.Fatalf("non-HTTP response selected or left unanswered: %+v err=%v", got, err)
		}
	}
}

func TestProbeChoosesLowestHTTPRegardlessOfCompletionOrder(t *testing.T) {
	var lower int32
	handler := func(conn net.Conn) {
		if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
			return
		}
		if int32(conn.LocalAddr().(*net.TCPAddr).Port) == lower {
			time.Sleep(60 * time.Millisecond)
		}
		_, _ = io.WriteString(conn, "HTTP/1.1 404 Not Found\r\n")
	}
	first := endpoint(t, handler)
	second := endpoint(t, handler)
	lower = min(first, second)
	binary := httpEndpoint(t, "\x00\x00epmd\r\n")
	for range 5 {
		got, err := lowestHTTP(t.Context(), "127.0.0.1", "app.onbex.co", []int32{max(first, second), binary, lower})
		if err != nil || got != lower {
			t.Fatalf("selected=%d err=%v want lower HTTP=%d", got, err, lower)
		}
	}
}

func TestProbeBoundsSlowAndSilentPorts(t *testing.T) {
	slow := endpoint(t, func(conn net.Conn) {
		if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
			return
		}
		_, _ = io.Copy(io.Discard, conn) // never sends a status line
	})
	httpPort := httpEndpoint(t, "HTTP/1.1 200 OK\r\n")
	started := time.Now()
	got, err := Probe(t.Context(), "127.0.0.1", "app.onbex.co", []int32{slow, httpPort})
	want := Result{Port: httpPort}
	if slow < httpPort {
		want.Unanswered = slow // a silent lower listener stays a candidate
	}
	if err != nil || got != want {
		t.Fatalf("result=%+v err=%v want=%+v", got, err, want)
	}
	if elapsed := time.Since(started); elapsed > portTimeout+time.Second {
		t.Fatalf("slow listener delayed probe %s", elapsed)
	}
}

// w8/m141 t007: RabbitMQ's management listener (15672) answered its cold
// first GET / after the per-port budget while Prometheus (15692) returned a
// fast 404. The slow lower port is reported unanswered rather than lost, and
// a retry once it is warm selects it.
func TestProbeReportsASlowFirstAnswerOnTheLowerPortAsUnanswered(t *testing.T) {
	var lower int32
	var cold atomic.Bool
	cold.Store(true)
	handler := func(conn net.Conn) {
		if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
			return
		}
		if int32(conn.LocalAddr().(*net.TCPAddr).Port) != lower {
			_, _ = io.WriteString(conn, "HTTP/1.1 404 Not Found\r\n")
			return
		}
		if cold.Swap(false) {
			time.Sleep(portTimeout + 300*time.Millisecond)
		}
		_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\n")
	}
	first := endpoint(t, handler)
	second := endpoint(t, handler)
	lower = min(first, second)
	higher := max(first, second)
	got, err := Probe(t.Context(), "127.0.0.1", "app.onbex.co", []int32{higher, lower})
	if err != nil || got != (Result{Port: higher, Unanswered: lower}) {
		t.Fatalf("cold scan=%+v err=%v want port %d with %d unanswered", got, err, higher, lower)
	}
	got, err = Probe(t.Context(), "127.0.0.1", "app.onbex.co", []int32{higher, lower})
	if err != nil || got != (Result{Port: lower}) {
		t.Fatalf("warm scan=%+v err=%v want lower port %d", got, err, lower)
	}
}

func TestProbeDoesNotSpeakTLSOrFollowRedirects(t *testing.T) {
	tlsServer := httptest.NewTLSServer(http.NotFoundHandler())
	defer tlsServer.Close()
	tlsConfig := tlsServer.TLS.Clone()
	tlsPort := endpoint(t, func(conn net.Conn) {
		secure := tls.Server(conn, tlsConfig)
		defer func() { _ = secure.Close() }()
		_ = secure.Handshake()
	})
	redirect := httpEndpoint(t, "HTTP/1.1 302 Found\r\nLocation: https://somewhere.invalid/\r\n\r\n")
	got, err := lowestHTTP(t.Context(), "127.0.0.1", "app.onbex.co", []int32{tlsPort, redirect})
	if err != nil || got != redirect {
		t.Fatalf("selected=%d err=%v want redirect listener=%d", got, err, redirect)
	}
}

func TestProbeSendsPlatformHostWithoutCredentialsOrProxy(t *testing.T) {
	requests := make(chan *http.Request, 1)
	port := endpoint(t, func(conn net.Conn) {
		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			return
		}
		requests <- req
		_, _ = io.WriteString(conn, "HTTP/1.1 401 Unauthorized\r\n")
	})
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("ALL_PROXY", "http://127.0.0.1:1")
	got, err := lowestHTTP(t.Context(), "127.0.0.1", "platform-app.onbex.co", []int32{port})
	if err != nil || got != port {
		t.Fatalf("selected=%d err=%v", got, err)
	}
	select {
	case req := <-requests:
		if req.Method != "GET" || req.RequestURI != "/" || req.Host != "platform-app.onbex.co" || !req.Close {
			t.Fatalf("unexpected request: %+v", req)
		}
		if req.Header.Get("Authorization") != "" || req.Header.Get("Proxy-Authorization") != "" || req.Header.Get("Cookie") != "" {
			t.Fatal("probe sent credentials")
		}
	case <-time.After(time.Second):
		t.Fatal("no direct HTTP request reached pod")
	}
}

func TestProbeCancellationClosesAWaitingRead(t *testing.T) {
	connected := make(chan struct{})
	port := endpoint(t, func(conn net.Conn) {
		if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
			return
		}
		close(connected)
		_, _ = io.Copy(io.Discard, conn)
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	type result struct {
		port int32
		err  error
	}
	done := make(chan result, 1)
	go func() {
		port, err := lowestHTTP(ctx, "127.0.0.1", "app.onbex.co", []int32{port})
		done <- result{port, err}
	}()
	select {
	case <-connected:
	case <-time.After(time.Second):
		t.Fatal("no probe connection")
	}
	cancel()
	select {
	case result := <-done:
		if result.port != 0 || result.err != context.Canceled {
			t.Fatalf("port=%d err=%v", result.port, result.err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cancellation waited for per-port timeout")
	}
}

func TestProbeRejectsInvalidInputsAndEmptyScan(t *testing.T) {
	for _, tc := range []struct {
		ip, host string
		ports    []int32
	}{
		{"pod.internal", "app.onbex.co", []int32{80}},
		{"127.0.0.1", "app.onbex.co\r\nAuthorization: injected", []int32{80}},
		{"127.0.0.1", "app.onbex.co", []int32{0}},
		{"127.0.0.1", "app.onbex.co", []int32{65536}},
		{"127.0.0.1", "app.onbex.co", slices.Repeat([]int32{80}, maxPorts+1)},
	} {
		if got, err := lowestHTTP(t.Context(), tc.ip, tc.host, tc.ports); err == nil || got != 0 {
			t.Fatalf("invalid input selected=%d err=%v", got, err)
		}
	}
	if got, err := lowestHTTP(t.Context(), "127.0.0.1", "app.onbex.co", nil); err != nil || got != 0 {
		t.Fatalf("empty probe=%d err=%v", got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got, err := lowestHTTP(ctx, "127.0.0.1", "app.onbex.co", nil); err != context.Canceled || got != 0 {
		t.Fatalf("cancelled empty probe=%d err=%v", got, err)
	}
}

func TestProbeDefaultsHostToThePodIP(t *testing.T) {
	host := make(chan string, 1)
	port := endpoint(t, func(conn net.Conn) {
		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			return
		}
		host <- req.Host
		_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\n")
	})
	if got, err := lowestHTTP(t.Context(), "127.0.0.1", "", []int32{port}); err != nil || got != port {
		t.Fatalf("port=%d err=%v", got, err)
	}
	if got := <-host; got != "127.0.0.1" {
		t.Fatalf("Host=%q", got)
	}
}
