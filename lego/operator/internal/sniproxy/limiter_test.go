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
	var handled int64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Serve(ctx, ln, limiter, meter, func(c net.Conn) {
		atomic.AddInt64(&handled, 1)
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
	if got := atomic.LoadInt64(&handled); got != 2 {
		t.Fatalf("handled = %d, want 2 (the third was shed)", got)
	}
	if got := testutil.ToFloat64(meter.rejected.WithLabelValues("global")); got != 1 {
		t.Fatalf("rejected(global) = %v, want 1", got)
	}
	close(release)
}

// TestCopyBidirectionalIdleTimeoutTearsDown proves the routed copy no longer
// blocks forever on an idle connection (finding 6): with an idle bound and no
// bytes flowing, CopyBidirectional returns promptly and closes both conns.
func TestCopyBidirectionalIdleTimeoutTearsDown(t *testing.T) {
	registry := prometheus.NewRegistry()
	meter := NewByteMeter(registry, "idle_test", "postgres")
	otherClient, proxyClient := net.Pipe()
	proxyBackend, otherBackend := net.Pipe()
	defer func() { _ = otherClient.Close(); _ = otherBackend.Close() }()

	done := make(chan struct{})
	go func() {
		CopyBidirectional(proxyClient, proxyBackend, meter, "dpg-one", "postgres", 50*time.Millisecond, 0)
		close(done)
	}()
	select {
	case <-done:
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
