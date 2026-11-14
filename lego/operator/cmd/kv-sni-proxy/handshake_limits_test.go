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
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/operator/internal/sniproxy"
)

// R25-12 (w21/070): see cmd/pg-sni-proxy/handshake_limits_test.go. The Key Value
// proxy reads the client's TLS ClientHello straight after the PROXY header.

func shortenPreHeader(t *testing.T, d time.Duration) {
	t.Helper()
	previous := sniproxy.SetPreHeaderTimeout(d)
	t.Cleanup(func() { sniproxy.SetPreHeaderTimeout(previous) })
}

func rejectedCount(t *testing.T, registry *prometheus.Registry, reason string) float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != "bex_kv_proxy_connections_rejected_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "reason" && label.GetValue() == reason {
					return metric.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}

func TestASilentConnectionIsClosedAfterThePreHeaderTimeout(t *testing.T) {
	shortenPreHeader(t, 150*time.Millisecond)
	meter := sniproxy.NewByteMeter(prometheus.NewRegistry(), "kv_proxy", "key_value")
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proxy.Close() })
	handled := make(chan struct{})
	go func() {
		defer close(handled)
		conn, err := proxy.Accept()
		if err == nil {
			handleConn(conn, newRouter("kv.bex.co"), meter, nil, sniproxy.NewLimiter(0, 0), 0, 0, discardLogger{})
		}
	}()
	client, err := net.Dial("tcp", proxy.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	start := time.Now()
	select {
	case <-handled:
	case <-time.After(3 * time.Second):
		t.Fatal("a connection that sent nothing was still held long after the pre-header timeout")
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("closed after %v, before the pre-header timeout", elapsed)
	}
}

// A client that sends its first byte inside the window keeps the rest of the
// handshake deadline for the rest of its ClientHello.
func TestAClientThatSpeaksInTimeKeepsTheRestOfTheHandshakeDeadline(t *testing.T) {
	shortenPreHeader(t, 150*time.Millisecond)
	meter := sniproxy.NewByteMeter(prometheus.NewRegistry(), "kv_proxy", "key_value")
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proxy.Close() })
	handled := make(chan struct{})
	go func() {
		defer close(handled)
		conn, err := proxy.Accept()
		if err == nil {
			handleConn(conn, newRouter("kv.bex.co"), meter, nil, sniproxy.NewLimiter(0, 0), 0, 0, discardLogger{})
		}
	}()
	client, err := net.Dial("tcp", proxy.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	// The first byte of a TLS record, then a pause well past the pre-header timeout.
	if _, err := client.Write([]byte{0x16}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(450 * time.Millisecond)
	_ = client.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, err = client.Read(make([]byte, 1))
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("the proxy closed a client that had spoken in time: %v", err)
	}
	select {
	case <-handled:
		t.Fatal("the handler returned while the handshake was still in progress")
	default:
	}
}

func TestStalledHandshakesFromOneNetworkAreShedAtItsShare(t *testing.T) {
	shortenPreHeader(t, 5*time.Second)
	const attackers, share = 12, 4
	registry := prometheus.NewRegistry()
	meter := sniproxy.NewByteMeter(registry, "kv_proxy", "key_value")
	limiter := sniproxy.NewLimiter(64, 0)
	limiter.SetHandshakeLimit(share)
	trusted := []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}
	router := newRouter("kv.bex.co")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = ln.Close() })
	go sniproxy.Serve(ctx, ln, limiter, meter, func(conn net.Conn) {
		handleConn(conn, router, meter, trusted, limiter, 0, 0, discardLogger{})
	}, nil)

	// Each connection opens with a PROXY header and then stalls: a client's
	// ClientHello never comes.
	dial := func(src string) net.Conn {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		if _, err := fmt.Fprintf(conn, "PROXY TCP4 %s 203.0.113.1 40000 6379\r\n", src); err != nil {
			t.Fatal(err)
		}
		return conn
	}
	conns := make([]net.Conn, attackers)
	for i := range conns {
		conns[i] = dial(fmt.Sprintf("198.51.100.%d", i+1))
	}
	var shed, held int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, conn := range conns {
		wg.Go(func() {
			_ = conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
			_, err := conn.Read(make([]byte, 1))
			mu.Lock()
			defer mu.Unlock()
			if errors.Is(err, io.EOF) {
				shed++
			} else {
				held++
			}
		})
	}
	wg.Wait()
	if held != share || shed != attackers-share {
		t.Fatalf("held %d and shed %d of %d stalled handshakes, want %d and %d",
			held, shed, attackers, share, attackers-share)
	}
	if got := rejectedCount(t, registry, sniproxy.ScopeHandshake); got != attackers-share {
		t.Fatalf("handshake rejections metered = %v, want %d", got, attackers-share)
	}

	// A client from another network is admitted: its connection stays open,
	// waiting for its ClientHello, instead of being shed.
	legit := dial("192.0.2.50")
	_ = legit.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	_, err = legit.Read(make([]byte, 1))
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("a legitimate client elsewhere was shed: %v", err)
	}
}
