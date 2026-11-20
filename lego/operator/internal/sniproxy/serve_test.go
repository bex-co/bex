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
	"errors"
	"io"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// drainHarness runs Serve in front of a backend the test drives, with a
// handler shaped like the proxies' own: it takes a source slot, holds a
// handshake slot until the client's 5-byte "hello", then a route slot, and
// relays to the backend.
type drainHarness struct {
	t        *testing.T
	limiter  *Limiter
	addr     string
	backends chan net.Conn
	cancel   context.CancelFunc
	served   chan struct{}
}

func startDrainHarness(t *testing.T, drain Drain) *drainHarness {
	t.Helper()
	backendLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backendLn.Close() })
	backends := make(chan net.Conn, 8)
	go func() {
		for {
			conn, err := backendLn.Accept()
			if err != nil {
				return
			}
			backends <- conn
		}
	}()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	limiter := NewLimiter(8, 8)
	limiter.SetHandshakeLimit(8)
	meter := NewByteMeter(prometheus.NewRegistry(), "drain_test", "key_value")
	source := netip.MustParseAddr("192.0.2.10")
	handle := func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		releaseSource, ok := limiter.AcquireSource(source)
		if !ok {
			return
		}
		defer releaseSource()
		releaseHandshake, ok := limiter.AcquireHandshake(source)
		if !ok {
			return
		}
		defer releaseHandshake()
		var hello [5]byte
		if _, err := io.ReadFull(conn, hello[:]); err != nil {
			return
		}
		releaseHandshake()
		releaseRoute, ok := limiter.AcquireRoute("tenant/store", 8)
		if !ok {
			return
		}
		defer releaseRoute()
		backend, err := net.Dial("tcp", backendLn.Addr().String())
		if err != nil {
			return
		}
		defer func() { _ = backend.Close() }()
		CopyBidirectional(conn, backend, meter, "store", "key_value", 0, 0)
	}
	ctx, cancel := context.WithCancel(t.Context())
	h := &drainHarness{
		t: t, limiter: limiter, addr: ln.Addr().String(), backends: backends,
		cancel: cancel, served: make(chan struct{}),
	}
	go func() {
		defer close(h.served)
		Serve(ctx, ln, limiter, meter, drain, handle, nil)
	}()
	t.Cleanup(func() { cancel(); <-h.served })
	return h
}

// relayed dials the proxy and completes the handshake, returning the client
// and the backend's side of its stream.
func (h *drainHarness) relayed() (client, backend net.Conn) {
	h.t.Helper()
	client, err := net.Dial("tcp", h.addr)
	if err != nil {
		h.t.Fatal(err)
	}
	h.t.Cleanup(func() { _ = client.Close() })
	if _, err := io.WriteString(client, "hello"); err != nil {
		h.t.Fatal(err)
	}
	select {
	case backend = <-h.backends:
	case <-time.After(5 * time.Second):
		h.t.Fatal("the stream never reached the backend")
	}
	h.t.Cleanup(func() { _ = backend.Close() })
	return client, backend
}

// waitServed fails unless Serve returns within 5 s.
func (h *drainHarness) waitServed() {
	h.t.Helper()
	select {
	case <-h.served:
	case <-time.After(5 * time.Second):
		h.t.Fatal("Serve did not return within 5s")
	}
}

// requireNoSlotsHeld fails if any global, source, handshake or route slot is
// still taken.
func (h *drainHarness) requireNoSlotsHeld() {
	h.t.Helper()
	l := h.limiter
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.global != 0 || len(l.perSource) != 0 || len(l.handshakeNetwork) != 0 || len(l.perRoute) != 0 {
		h.t.Fatalf("slots still held: global %d, source %v, handshake %v, route %v",
			l.global, l.perSource, l.handshakeNetwork, l.perRoute)
	}
}

// waitGlobal waits until n connections hold a global slot.
func (h *drainHarness) waitGlobal(n int) {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		h.limiter.mu.Lock()
		got := h.limiter.global
		h.limiter.mu.Unlock()
		if got == n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	h.t.Fatalf("never reached %d open connections", n)
}

// requireClosed fails unless conn reads EOF or an error within d.
func requireClosed(t *testing.T, conn net.Conn, d time.Duration) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(d))
	n, err := conn.Read(make([]byte, 1))
	if err == nil {
		t.Fatalf("read %d bytes, want the connection closed", n)
	}
	if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
		t.Fatalf("still open after %v", d)
	}
}

// A client that sent its command before the drain reads the reply the backend
// writes after it, while the old listener accepts nothing new, and Serve
// returns as soon as that stream ends, well before the deadline.
func TestADrainRelaysAReplyWrittenAfterItBeganAndAcceptsNothingNew(t *testing.T) {
	drain := make(chan struct{})
	h := startDrainHarness(t, Drain{Start: drain, Timeout: time.Minute})
	client, backend := h.relayed()
	if _, err := io.WriteString(client, "BLPOP q 0\n"); err != nil {
		t.Fatal(err)
	}
	request := make([]byte, len("BLPOP q 0\n"))
	if _, err := io.ReadFull(backend, request); err != nil {
		t.Fatal(err)
	}

	close(drain)
	refused := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		conn, err := net.Dial("tcp", h.addr)
		if err != nil {
			refused = true
			break
		}
		_ = conn.Close()
	}
	if !refused {
		t.Fatal("the draining listener still accepts new connections")
	}

	if _, err := io.WriteString(backend, "reply\n"); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(5 * time.Second))
	reply := make([]byte, len("reply\n"))
	if _, err := io.ReadFull(client, reply); err != nil {
		t.Fatalf("the open stream lost its reply after the drain began: %v", err)
	}
	if string(reply) != "reply\n" {
		t.Fatalf("reply = %q", reply)
	}
	select {
	case <-h.served:
		t.Fatal("Serve returned while a stream was still open")
	case <-time.After(100 * time.Millisecond):
	}

	_ = client.Close()
	h.waitServed()
	h.requireNoSlotsHeld()
}

// An idle stream and a connection still in its handshake stay open until the
// drain's deadline, close then, and give back every slot they held.
func TestADrainClosesWhatIsStillOpenAtItsDeadline(t *testing.T) {
	const bound = 400 * time.Millisecond
	drain := make(chan struct{})
	h := startDrainHarness(t, Drain{Start: drain, Timeout: bound})
	idle, _ := h.relayed()
	handshaking, err := net.Dial("tcp", h.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = handshaking.Close() }()
	h.waitGlobal(2)

	began := time.Now()
	close(drain)
	requireClosed(t, idle, 5*time.Second)
	if elapsed := time.Since(began); elapsed < bound {
		t.Fatalf("the idle stream closed %v into the drain, before its %v bound", elapsed, bound)
	}
	requireClosed(t, handshaking, 5*time.Second)
	h.waitServed()
	h.requireNoSlotsHeld()
}

// Cancelling without a drain closes every stream at once, as before.
func TestAHardStopClosesEveryStreamAtOnce(t *testing.T) {
	h := startDrainHarness(t, Drain{})
	client, _ := h.relayed()
	h.cancel()
	requireClosed(t, client, time.Second)
	h.waitServed()
	h.requireNoSlotsHeld()
}

// A hard stop during a drain, a second signal, closes the drained streams at
// once rather than at the deadline.
func TestAHardStopDuringADrainClosesAtOnce(t *testing.T) {
	drain := make(chan struct{})
	h := startDrainHarness(t, Drain{Start: drain, Timeout: time.Hour})
	client, _ := h.relayed()
	close(drain)
	select {
	case <-h.served:
		t.Fatal("Serve returned with a stream still open")
	case <-time.After(100 * time.Millisecond):
	}
	h.cancel()
	requireClosed(t, client, time.Second)
	h.waitServed()
	h.requireNoSlotsHeld()
}

// A zero bound drains nothing: the drain closes every stream at once.
func TestAZeroBoundClosesEveryStreamAtOnce(t *testing.T) {
	drain := make(chan struct{})
	h := startDrainHarness(t, Drain{Start: drain})
	client, _ := h.relayed()
	close(drain)
	requireClosed(t, client, time.Second)
	h.waitServed()
	h.requireNoSlotsHeld()
}

// The release's signal stops the manager and starts the drain at once, as in
// both mains. The manager's return must not hard-stop Serve: the open stream
// still reads the reply the backend writes after the signal.
func TestAManagerStoppedByTheSignalLeavesTheDrainRunning(t *testing.T) {
	stopping, signal := context.WithCancel(t.Context())
	defer signal()
	h := startDrainHarness(t, Drain{Start: stopping.Done(), Timeout: time.Minute})
	exited := make(chan error, 1)
	go func() { <-stopping.Done(); exited <- nil }() // mgr.Start(stopping)
	watched := make(chan struct{})
	go func() {
		defer close(watched)
		HardStopOnManagerExit(stopping, exited, h.cancel, func(err error) {
			t.Errorf("a manager stopped by the signal reported %v", err)
		})
	}()
	client, backend := h.relayed()

	signal()
	select {
	case <-watched:
	case <-time.After(5 * time.Second):
		t.Fatal("HardStopOnManagerExit never saw the manager return")
	}
	select {
	case <-h.served:
		t.Fatal("the manager's return on the signal hard-stopped the drain")
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := io.WriteString(backend, "reply\n"); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(5 * time.Second))
	reply := make([]byte, len("reply\n"))
	if _, err := io.ReadFull(client, reply); err != nil {
		t.Fatalf("the drained stream lost its reply: %v", err)
	}
	_ = client.Close()
	h.waitServed()
	h.requireNoSlotsHeld()
}

// A manager that exits with no signal stops watching KeyValue and Database
// changes, so Serve stops at once and closes every stream, and the manager's
// error, if it has one, is reported.
func TestAManagerThatExitsOnItsOwnStopsServeAtOnce(t *testing.T) {
	for name, exitErr := range map[string]error{"with an error": errors.New("lost the API server"), "cleanly": nil} {
		t.Run(name, func(t *testing.T) {
			stopping := t.Context()
			h := startDrainHarness(t, Drain{Start: stopping.Done(), Timeout: time.Hour})
			client, _ := h.relayed()
			exited := make(chan error, 1)
			reported := make(chan error, 1)
			go HardStopOnManagerExit(stopping, exited, h.cancel, func(err error) { reported <- err })

			exited <- exitErr
			requireClosed(t, client, time.Second)
			h.waitServed()
			h.requireNoSlotsHeld()
			select {
			case got := <-reported:
				if exitErr == nil {
					t.Fatalf("a clean exit reported %v", got)
				}
				if got != exitErr {
					t.Fatalf("reported %v, want %v", got, exitErr)
				}
			default:
				if exitErr != nil {
					t.Fatalf("the manager's error %v was not reported", exitErr)
				}
			}
		})
	}
}

func TestDrainSecondsIsWholeSecondsAndFailsClosed(t *testing.T) {
	const name = "BEX_TEST_PROXY_DRAIN_SECONDS"
	for raw, want := range map[string]time.Duration{"": 0, "0": 0, "30": 30 * time.Second, "3600": time.Hour} {
		t.Setenv(name, raw)
		got, err := DrainSeconds(name)
		if err != nil || got != want {
			t.Errorf("%q: got %v, %v; want %v", raw, got, err, want)
		}
	}
	for _, raw := range []string{"-1", "1.5", "30s", "thirty", " 30", "9223372037"} {
		t.Setenv(name, raw)
		if _, err := DrainSeconds(name); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%q: err = %v, want a refusal naming %s", raw, err, name)
		}
	}
}
