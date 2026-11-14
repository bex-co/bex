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
	"encoding/binary"
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

// R25-12 (w21/070): a connection that never finishes its handshake held one of
// the process's 1024 slots for the whole 10 s deadline, so a few sources could
// shed every other client. The first bytes now get PreHeaderTimeout, and a
// network's connections still in their handshake are capped.

func rejectedCount(t *testing.T, registry *prometheus.Registry, reason string) float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != "bex_pg_proxy_connections_rejected_total" {
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

func shortenPreHeader(t *testing.T, d time.Duration) {
	t.Helper()
	previous := sniproxy.SetPreHeaderTimeout(d)
	t.Cleanup(func() { sniproxy.SetPreHeaderTimeout(previous) })
}

func TestThePreHeaderTimeoutIsTwoSeconds(t *testing.T) {
	if sniproxy.PreHeaderTimeout() != 2*time.Second {
		t.Fatalf("PreHeaderTimeout = %v, want 2s", sniproxy.PreHeaderTimeout())
	}
}

// A client that sends nothing is closed after PreHeaderTimeout, not after the
// 10 s handshake deadline; one that speaks inside that window still has the
// rest of the handshake deadline for the rest of its handshake.
func TestASilentConnectionIsClosedAfterThePreHeaderTimeout(t *testing.T) {
	shortenPreHeader(t, 150*time.Millisecond)
	serveOne := func(t *testing.T) (client net.Conn, handled <-chan struct{}) {
		t.Helper()
		proxy, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = proxy.Close() })
		done := make(chan struct{})
		go func() {
			defer close(done)
			conn, err := proxy.Accept()
			if err == nil {
				meter := sniproxy.NewByteMeter(prometheus.NewRegistry(), "pg_proxy", "postgres")
				handleConn(conn, newRouter("db.bex.co"), meter, nil, sniproxy.NewLimiter(0, 0), 0, 0, testLogger{})
			}
		}()
		client, err = net.Dial("tcp", proxy.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		return client, done
	}

	t.Run("silent", func(t *testing.T) {
		_, handled := serveOne(t)
		start := time.Now()
		select {
		case <-handled:
		case <-time.After(3 * time.Second):
			t.Fatal("a connection that sent nothing was still held long after the pre-header timeout")
		}
		if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
			t.Fatalf("closed after %v, before the pre-header timeout", elapsed)
		}
	})

	t.Run("speaks in time", func(t *testing.T) {
		client, handled := serveOne(t)
		sslRequest := make([]byte, 8)
		binary.BigEndian.PutUint32(sslRequest[0:4], uint32(sslRequestLen))
		binary.BigEndian.PutUint32(sslRequest[4:8], uint32(sslRequestMagic))
		if _, err := client.Write(sslRequest); err != nil {
			t.Fatal(err)
		}
		reply := make([]byte, 1)
		_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, err := io.ReadFull(client, reply); err != nil || reply[0] != sslByte {
			t.Fatalf("SSLRequest answer = %v, %v", reply, err)
		}
		// Well past the pre-header timeout, the proxy is still waiting for the
		// ClientHello: reading times out instead of seeing the connection closed.
		time.Sleep(450 * time.Millisecond)
		_ = client.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		_, err := client.Read(reply)
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			t.Fatalf("the proxy closed a client that had spoken in time: %v", err)
		}
		select {
		case <-handled:
			t.Fatal("the handler returned while the handshake was still in progress")
		default:
		}
	})
}

// One network's stalled handshakes stop at its share: the rest are shed at once
// with the handshake reason, and a client elsewhere is still admitted.
func TestStalledHandshakesFromOneNetworkAreShedAtItsShare(t *testing.T) {
	shortenPreHeader(t, 5*time.Second)
	const attackers, share = 12, 4
	registry := prometheus.NewRegistry()
	meter := sniproxy.NewByteMeter(registry, "pg_proxy", "postgres")
	limiter := sniproxy.NewLimiter(64, 0)
	limiter.SetHandshakeLimit(share)
	trusted := []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}
	router := newRouter("db.bex.co")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = ln.Close() })
	go sniproxy.Serve(ctx, ln, limiter, meter, func(conn net.Conn) {
		handleConn(conn, router, meter, trusted, limiter, 0, 0, testLogger{})
	}, nil)

	proxyHeader := func(src string) []byte {
		return []byte(fmt.Sprintf("PROXY TCP4 %s 203.0.113.1 40000 5432\r\n", src))
	}
	dial := func(src string) net.Conn {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		if _, err := conn.Write(proxyHeader(src)); err != nil {
			t.Fatal(err)
		}
		return conn
	}

	// Twelve connections from twelve addresses of one /24, each stalled after
	// its PROXY header.
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

	// A client from another network gets through to the SSLRequest answer.
	legit := dial("192.0.2.50")
	sslRequest := make([]byte, 8)
	binary.BigEndian.PutUint32(sslRequest[0:4], uint32(sslRequestLen))
	binary.BigEndian.PutUint32(sslRequest[4:8], uint32(sslRequestMagic))
	if _, err := legit.Write(sslRequest); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 1)
	_ = legit.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(legit, reply); err != nil || reply[0] != sslByte {
		t.Fatalf("a legitimate client elsewhere was not admitted: %v, %v", reply, err)
	}
}
