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

package sniproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestLimiterGlobalCapRejectsAndReleases(t *testing.T) {
	l := NewLimiter(2, 0)
	r1, ok := l.AcquireGlobal()
	if !ok {
		t.Fatal("first global acquire rejected")
	}
	if _, ok := l.AcquireGlobal(); !ok {
		t.Fatal("second global acquire rejected")
	}
	if _, ok := l.AcquireGlobal(); ok {
		t.Fatal("third global acquire admitted past the cap of 2")
	}
	r1() // free a slot
	if _, ok := l.AcquireGlobal(); !ok {
		t.Fatal("global acquire rejected after a slot was released")
	}
	// A double release must not over-credit the counter.
	r1()
	if _, ok := l.AcquireGlobal(); ok {
		t.Fatal("double release leaked a global slot")
	}
}

func TestLimiterPerSourceCapIsolatesClients(t *testing.T) {
	l := NewLimiter(0, 2)
	a := netip.MustParseAddr("203.0.113.9")
	b := netip.MustParseAddr("198.51.100.7")
	ra, ok := l.AcquireSource(a)
	if !ok {
		t.Fatal("first per-source acquire rejected")
	}
	if _, ok := l.AcquireSource(a); !ok {
		t.Fatal("second per-source acquire rejected")
	}
	if _, ok := l.AcquireSource(a); ok {
		t.Fatal("third per-source acquire admitted past the per-source cap of 2")
	}
	// A different client is unaffected by a's saturation.
	if _, ok := l.AcquireSource(b); !ok {
		t.Fatal("a different source was starved by another client's cap")
	}
	ra()
	if _, ok := l.AcquireSource(a); !ok {
		t.Fatal("per-source acquire rejected after a slot was released")
	}
}

// w21/m394 t017 (R25-12 mitigation): one IPv6 subscriber holds a whole /64, so a
// per-source cap keyed on the exact address let a single client take 2^64
// distinct "sources" and never meet the cap.
func TestLimiterPerSourceKeysIPv6OnSlash64(t *testing.T) {
	const perSource = 128
	l := NewLimiter(0, perSource)
	slash64 := func(prefix [8]byte, host int) netip.Addr {
		var b [16]byte
		copy(b[:8], prefix[:])
		b[14], b[15] = byte(host>>8), byte(host)
		return netip.AddrFrom16(b)
	}
	first := [8]byte{0x20, 0x01, 0x0d, 0xb8, 0, 1, 0, 2}
	admitted := 0
	for host := range 1024 {
		if _, ok := l.AcquireSource(slash64(first, host)); ok {
			admitted++
		}
	}
	if admitted != perSource {
		t.Fatalf("1024 addresses of one /64 admitted %d connections, want the per-source cap of %d", admitted, perSource)
	}
	// Another /64 is another source, and an IPv4-mapped form is its IPv4 address.
	other := [8]byte{0x20, 0x01, 0x0d, 0xb8, 0, 1, 0, 3}
	if _, ok := l.AcquireSource(slash64(other, 1)); !ok {
		t.Fatal("a different /64 shares the first one's cap")
	}
	if _, ok := l.AcquireSource(netip.MustParseAddr("198.51.100.7")); !ok {
		t.Fatal("an IPv4 source shares the IPv6 cap")
	}
}

func TestLimiterDisabledDimensionsAlwaysAdmit(t *testing.T) {
	l := NewLimiter(0, 0)
	for range 1000 {
		if _, ok := l.AcquireGlobal(); !ok {
			t.Fatal("disabled global cap rejected")
		}
		if _, ok := l.AcquireSource(netip.MustParseAddr("203.0.113.9")); !ok {
			t.Fatal("disabled per-source cap rejected")
		}
	}
}

// TestLimiterRouteCapacityIsPerRoute pins w6/m457: each route admits exactly
// the capacity it is acquired with, and nothing else shares its count — not the
// same database's pooled or replica hostname, not another workspace's
// identically named resource, and no per-workspace total. The capacity comes
// with each admission, so a plan change applies to the next client.
func TestLimiterRouteCapacityIsPerRoute(t *testing.T) {
	l := NewLimiter(0, 0)
	held := make([]func(), 0, 3)
	for i := range 3 {
		release, ok := l.AcquireRoute("tea-a/dpg-a", 3)
		if !ok {
			t.Fatalf("route refused connection %d of its 3", i+1)
		}
		held = append(held, release)
	}
	if _, ok := l.AcquireRoute("tea-a/dpg-a", 3); ok {
		t.Fatal("route admitted a fourth connection past its capacity of 3")
	}
	for _, other := range []string{"tea-a/dpg-a-pool", "tea-a/dpg-b", "tea-b/dpg-a"} {
		for i := range 3 {
			if _, ok := l.AcquireRoute(other, 3); !ok {
				t.Fatalf("%s refused connection %d: it shares a full route's count", other, i+1)
			}
		}
	}

	// An upgraded plan admits the next client at once; a downgrade below what
	// is held admits none until enough have left.
	upgraded, ok := l.AcquireRoute("tea-a/dpg-a", 4)
	if !ok {
		t.Fatal("a larger capacity did not admit the next client")
	}
	if _, ok := l.AcquireRoute("tea-a/dpg-a", 3); ok {
		t.Fatal("the smaller capacity admitted a client while the route holds 4")
	}
	upgraded()
	held[0]()
	held[0]() // a double release must not create capacity
	if _, ok := l.AcquireRoute("tea-a/dpg-a", 3); !ok {
		t.Fatal("route refused a client after one of its 3 left")
	}
	if _, ok := l.AcquireRoute("tea-a/dpg-a", 3); ok {
		t.Fatal("a double release leaked a route slot")
	}
}

// TestRouteCapacityIsTwiceTheDatastoreLimit pins the margin the proxies admit
// above a datastore's limit so the server, not the proxy, refuses the client
// past it (w6/m457).
func TestRouteCapacityIsTwiceTheDatastoreLimit(t *testing.T) {
	for limit, want := range map[int32]int{50: 100, 100: 200, 250: 500, 30000: 60000} {
		if got := RouteCapacity(limit); got != want {
			t.Errorf("RouteCapacity(%d) = %d, want %d", limit, got, want)
		}
	}
	if _, ok := NewLimiter(0, 0).AcquireRoute("tea-a/dpg-a", 0); !ok {
		t.Error("a route without a capacity refused a client")
	}
}

// TestServeShedsBeyondGlobalCapWithoutDispatch proves the accept-loop admission
// (finding 6): with both global slots held, a further connection is closed at
// accept time — no handler goroutine is dispatched (so no backend dial), and the
// rejection is metered.
func TestServeShedsBeyondGlobalCapWithoutDispatch(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	registry := prometheus.NewRegistry()
	meter := NewByteMeter(registry, "serve_test", "postgres")
	limiter := NewLimiter(2, 0)

	started := make(chan struct{}, 8)
	release := make(chan struct{})
	var handled atomic.Int64
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go Serve(ctx, ln, limiter, meter, Drain{}, func(c net.Conn) {
		handled.Add(1)
		started <- struct{}{}
		<-release
		_ = c.Close()
	}, nil)

	// Occupy both global slots.
	c1, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c1.Close() }()
	<-started
	c2, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()
	<-started

	// The third connection is shed at accept time.
	c3, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c3.Close() }()
	_ = c3.SetReadDeadline(time.Now().Add(2 * time.Second))
	if n, rerr := c3.Read(make([]byte, 1)); rerr == nil {
		t.Fatalf("shed connection stayed open (read %d bytes), want immediate close", n)
	}
	select {
	case <-started:
		t.Fatal("shed connection was dispatched to a handler goroutine")
	case <-time.After(100 * time.Millisecond):
	}
	if got := handled.Load(); got != 2 {
		t.Fatalf("handled = %d, want 2 (the third was shed)", got)
	}
	// At least c3 was shed. Not exactly 1: on a shared CI runner a stray dial
	// from another package can land on this loopback port while both slots are
	// held and is shed too. handled == 2 above already proves nothing extra was
	// dispatched.
	if got := testutil.ToFloat64(meter.rejected.WithLabelValues("global")); got < 1 {
		t.Fatalf("rejected(global) = %v, want at least 1", got)
	}
	close(release)
}

// TestCopyBidirectionalIdleTimeoutTearsDown proves the routed copy no longer
// blocks forever on an idle connection (finding 6): with an idle bound and no
// bytes flowing, CopyBidirectional returns at the bound and closes both conns.
func TestCopyBidirectionalIdleTimeoutTearsDown(t *testing.T) {
	registry := prometheus.NewRegistry()
	meter := NewByteMeter(registry, "idle_test", "postgres")
	otherClient, proxyClient := net.Pipe()
	proxyBackend, otherBackend := net.Pipe()
	defer func() { _ = otherClient.Close(); _ = otherBackend.Close() }()

	const idle = 50 * time.Millisecond
	start := time.Now()
	done := make(chan struct{})
	go func() {
		CopyBidirectional(proxyClient, proxyBackend, meter, "dpg-one", "postgres", idle, 0)
		close(done)
	}()
	select {
	case <-done:
		if elapsed := time.Since(start); elapsed < idle {
			t.Fatalf("idle connection torn down after %v, before the %v bound", elapsed, idle)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("idle connection was not torn down by the idle timeout")
	}
	// Both proxy-side conns are closed on teardown.
	if _, err := proxyClient.Write([]byte("x")); err == nil {
		t.Fatal("client conn was not closed on idle teardown")
	}
}

// TestCopyBidirectionalMaxLifetimeTearsDown proves the max-lifetime budget bounds
// a connection even while the idle bound is disabled (finding 6).
func TestCopyBidirectionalMaxLifetimeTearsDown(t *testing.T) {
	registry := prometheus.NewRegistry()
	meter := NewByteMeter(registry, "life_test", "key_value")
	otherClient, proxyClient := net.Pipe()
	proxyBackend, otherBackend := net.Pipe()
	defer func() { _ = otherClient.Close(); _ = otherBackend.Close() }()

	start := time.Now()
	done := make(chan struct{})
	go func() {
		CopyBidirectional(proxyClient, proxyBackend, meter, "kv-one", "key_value", 0, 80*time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
		if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
			t.Fatalf("connection torn down after %v, before the max lifetime", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("connection outlived its max-lifetime budget")
	}
}

// A Key Value subscriber sends nothing after SUBSCRIBE while its messages
// arrive, and a Postgres client may send while the server answers nothing.
// Either way bytes move, so the stream is not idle: it stays open past the
// bound and closes only once it has been silent both ways for the bound
// (w6/148). Until then each direction counted alone, so the silent one closed
// the stream at the bound however busy the other was.
func TestCopyBidirectionalOneWayTrafficKeepsTheStreamOpen(t *testing.T) {
	const idle = 300 * time.Millisecond
	const every = idle / 6
	for _, tc := range []struct {
		name         string
		backendSends bool
	}{
		{"the backend writes to a client that sends nothing", true},
		{"the client writes to a backend that answers nothing", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, backend, done := relayedStream(t, idle, 0)
			from, to := client, backend
			if tc.backendSends {
				from, to = backend, client
			}
			start := time.Now()
			var lastSent time.Time
			got := make([]byte, len("message"))
			for time.Since(start) < 3*idle {
				time.Sleep(every)
				lastSent = time.Now()
				if _, err := io.WriteString(from, "message"); err != nil {
					t.Fatalf("write %v into the stream: %v", time.Since(start), err)
				}
				_ = to.SetReadDeadline(time.Now().Add(5 * time.Second))
				if _, err := io.ReadFull(to, got); err != nil {
					t.Fatalf("the stream closed %v after it opened, with bytes moving every %v under a %v bound: %v",
						time.Since(start), every, idle, err)
				}
			}
			select {
			case <-done:
				if since := time.Since(lastSent); since < idle {
					t.Fatalf("the stream closed %v after its last bytes, before the %v bound", since, idle)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the stream stayed open after going silent both ways")
			}
		})
	}
}

// The max lifetime still ends a stream however busy it is: activity extends
// only the idle deadline (w6/148).
func TestCopyBidirectionalMaxLifetimeEndsABusyStream(t *testing.T) {
	const lifetime = 300 * time.Millisecond
	start := time.Now()
	client, backend, done := relayedStream(t, time.Minute, lifetime)
	go func() { _, _ = io.Copy(io.Discard, client) }()
	go func() {
		for {
			if _, err := io.WriteString(backend, "message"); err != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	select {
	case <-done:
		if elapsed := time.Since(start); elapsed < lifetime {
			t.Fatalf("a busy stream closed after %v, before its %v lifetime", elapsed, lifetime)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a busy stream outlived its max lifetime")
	}
}

// relayedStream runs CopyBidirectional over loopback TCP, the sockets the
// proxies relay, between a client and a backend the test holds. done closes
// when the copy returns.
func relayedStream(t *testing.T, idle, maxLifetime time.Duration) (client, backend net.Conn, done <-chan struct{}) {
	t.Helper()
	client, proxyClient := tcpPair(t)
	proxyBackend, backend := tcpPair(t)
	meter := NewByteMeter(prometheus.NewRegistry(), "activity_test", "key_value")
	finished := make(chan struct{})
	go func() {
		CopyBidirectional(proxyClient, proxyBackend, meter, "red-one", "key_value", idle, maxLifetime)
		close(finished)
	}()
	return client, backend, finished
}

// tcpPair returns the two ends of one loopback TCP connection.
func tcpPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, _ := ln.Accept()
		accepted <- conn
	}()
	dialed, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn := <-accepted
	if conn == nil {
		t.Fatal("loopback accept failed")
	}
	t.Cleanup(func() { _ = dialed.Close(); _ = conn.Close() })
	return dialed, conn
}

// A connection that has not finished its handshake holds a slot of its
// network's share, so one network cannot keep the whole process waiting on
// handshakes that never finish.
func TestHandshakeCapIsPerNetwork(t *testing.T) {
	limiter := NewLimiter(0, 0)
	limiter.SetHandshakeLimit(3)

	// Every address of one IPv4 /24 draws on the same share.
	releases := make([]func(), 0, 3)
	for host := range 3 {
		release, ok := limiter.AcquireHandshake(netip.MustParseAddr(fmt.Sprintf("198.51.100.%d", host+1)))
		if !ok {
			t.Fatalf("handshake %d of the /24 was refused under its cap", host+1)
		}
		releases = append(releases, release)
	}
	if _, ok := limiter.AcquireHandshake(netip.MustParseAddr("198.51.100.200")); ok {
		t.Fatal("a 4th handshake from one /24 was admitted: rotating addresses dodges the cap")
	}
	// Another network is untouched.
	if _, ok := limiter.AcquireHandshake(netip.MustParseAddr("198.51.101.1")); !ok {
		t.Fatal("a handshake from a different /24 was refused")
	}
	// An IPv4-mapped IPv6 address is the same network as the IPv4 one.
	if _, ok := limiter.AcquireHandshake(netip.MustParseAddr("::ffff:198.51.100.9")); ok {
		t.Fatal("an IPv4-mapped address escaped its /24's cap")
	}

	// Releasing is exactly once.
	releases[0]()
	releases[0]()
	if _, ok := limiter.AcquireHandshake(netip.MustParseAddr("198.51.100.7")); !ok {
		t.Fatal("a release did not free a slot")
	}
	if _, ok := limiter.AcquireHandshake(netip.MustParseAddr("198.51.100.8")); ok {
		t.Fatal("a double release freed a second slot")
	}
}

func TestHandshakeCapKeysIPv6OnSlash64(t *testing.T) {
	limiter := NewLimiter(0, 0)
	limiter.SetHandshakeLimit(1)
	if _, ok := limiter.AcquireHandshake(netip.MustParseAddr("2001:db8:1:2::1")); !ok {
		t.Fatal("the first handshake of a /64 was refused")
	}
	if _, ok := limiter.AcquireHandshake(netip.MustParseAddr("2001:db8:1:2:ffff::9")); ok {
		t.Fatal("a second address of the same /64 was admitted")
	}
	if _, ok := limiter.AcquireHandshake(netip.MustParseAddr("2001:db8:1:3::1")); !ok {
		t.Fatal("a different /64 was refused")
	}
}

// One /24 cannot hold every global slot: with the defaults' shape (a global cap
// of 1024 and a handshake share of 32), 256 addresses opening handshakes that
// never finish fill 32 slots, not 1024.
func TestOneNetworkCannotFillTheGlobalCapWithHandshakes(t *testing.T) {
	const global, share = 1024, 32
	limiter := NewLimiter(global, 0)
	limiter.SetHandshakeLimit(share)
	held := 0
	for host := range 256 {
		for range 8 {
			release, ok := limiter.AcquireHandshake(netip.MustParseAddr(fmt.Sprintf("203.0.113.%d", host)))
			if !ok {
				continue
			}
			if _, ok := limiter.AcquireGlobal(); !ok {
				t.Fatal("the global cap was reached")
			}
			_ = release // never released: the handshake stalls
			held++
		}
	}
	if held != share {
		t.Fatalf("one /24 held %d connections in their handshake, want %d", held, share)
	}
	// A legitimate client elsewhere still gets a global slot and a handshake slot.
	if _, ok := limiter.AcquireHandshake(netip.MustParseAddr("192.0.2.50")); !ok {
		t.Fatal("a legitimate client's handshake was refused")
	}
	if _, ok := limiter.AcquireGlobal(); !ok {
		t.Fatal("no global slot is left for a legitimate client")
	}
}

func TestHandshakeLimitDisabledAdmits(t *testing.T) {
	limiter := NewLimiter(0, 0)
	for range 50 {
		if _, ok := limiter.AcquireHandshake(netip.MustParseAddr("198.51.100.1")); !ok {
			t.Fatal("an unset handshake limit refused a connection")
		}
	}
	var nilLimiter *Limiter
	if _, ok := nilLimiter.AcquireHandshake(netip.MustParseAddr("198.51.100.1")); !ok {
		t.Fatal("a nil limiter refused a connection")
	}
}
