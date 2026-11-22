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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/operator/internal/controller"
	"github.com/bex-co/bex/lego/operator/internal/kvinternalauth"
	"github.com/bex-co/bex/lego/operator/internal/sniproxy"
)

// w8/m562: a source a known store's allow list refuses completes TLS under
// the store's certificate and reads Render's "Client IP address is not in the
// allowlist." from the store's kv-internal-proxy, instead of a bare close
// after its ClientHello. Nothing it sends reaches the store's backend.

const (
	refusedHost   = "red-one.kv.bex.co"
	refusalReply  = "-Client IP address is not in the allowlist.\r\n"
	admittedSrc   = "203.0.113.9"
	refusedSrc    = "198.51.100.9"
	refusedSrcTwo = "198.51.100.10"
)

// countingListener counts the connections it accepts.
type countingListener struct {
	net.Listener
	accepted atomic.Int64
}

func (l *countingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err == nil {
		l.accepted.Add(1)
	}
	return conn, err
}

func listen(t *testing.T) *countingListener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return &countingListener{Listener: ln}
}

// holdingListener counts and holds every connection, reading and answering
// nothing: the store's TLS backend, so a test can tell whether anything
// reached it, or a refusal port that never answers.
func holdingListener(t *testing.T) *countingListener {
	t.Helper()
	ln := listen(t)
	var mu sync.Mutex
	var held []net.Conn
	t.Cleanup(func() {
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range held {
			_ = conn.Close()
		}
	})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			held = append(held, conn)
			mu.Unlock()
			go func() { _, _ = io.Copy(io.Discard, conn) }()
		}
	}()
	return ln
}

type refusalFixture struct {
	addr     string
	roots    *x509.CertPool
	backend  *countingListener
	refusal  *countingListener
	registry *prometheus.Registry
	route    kvRoute
}

// refusalEdge serves kv-sni-proxy for one public store whose allow list is
// 203.0.113.0/24, with its backend held by holdingBackend and its refusal port
// served by a real kv-internal-proxy refusal listener with the store's
// certificate. PROXY headers from 127.0.0.1 assert each client's source.
func refusalEdge(t *testing.T, limiter *sniproxy.Limiter, capacity int) refusalFixture {
	t.Helper()
	certificate, roots := testCertificate(t, refusedHost)
	certFile, keyFile := writeKeyPair(t, certificate)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	f := refusalFixture{roots: roots, backend: holdingListener(t), refusal: listen(t), registry: prometheus.NewRegistry()}
	go func() {
		_ = kvinternalauth.Serve(ctx, f.refusal, kvinternalauth.Config{
			Mode: kvinternalauth.AllowListRefusal, TLSCertFile: certFile, TLSKeyFile: keyFile,
		})
	}()
	router := newRouter("kv.bex.co")
	kv := &sniproxy.KeyValue{}
	kv.Name, kv.Namespace = "red-one", "tea-one"
	kv.Spec.Public, kv.Spec.Plan = true, "free"
	kv.Spec.IPAllowList = []sniproxy.IPAllowEntry{{CIDR: "203.0.113.0/24"}}
	kv.Status.ExternalHost = refusedHost
	if err := router.set(kv); err != nil {
		t.Fatal(err)
	}
	route := router.table["red-one"]
	route.Backend, route.Refusal, route.Capacity = f.backend.Addr().String(), f.refusal.Addr().String(), capacity
	router.table["red-one"], f.route = route, route

	meter := sniproxy.NewByteMeter(f.registry, "kv_proxy", "key_value")
	edge := listen(t)
	trusted := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	go sniproxy.Serve(ctx, edge, limiter, meter, sniproxy.Drain{}, func(conn net.Conn) {
		handleConn(conn, router, meter, trusted, limiter, 0, 0, discardLogger{})
	}, nil)
	f.addr = edge.Addr().String()
	return f
}

// dialFrom opens a connection to the edge whose PROXY header asserts source.
func (f refusalFixture) dialFrom(t *testing.T, source string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", f.addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprintf(conn, "PROXY TCP4 %s 203.0.113.1 40000 6379\r\n", source); err != nil {
		t.Fatal(err)
	}
	return conn
}

// tlsFrom completes a TLS handshake for name from source, verifying the
// store's certificate.
func (f refusalFixture) tlsFrom(t *testing.T, source, name string) (*tls.Conn, error) {
	t.Helper()
	config := &tls.Config{RootCAs: f.roots, ServerName: name, MinVersion: tls.VersionTLS12}
	client := tls.Client(f.dialFrom(t, source), config)
	return client, client.Handshake()
}

func expectRefusals(t *testing.T, client *tls.Conn) {
	t.Helper()
	for _, command := range []string{
		"*3\r\n$4\r\nAUTH\r\n$7\r\ndefault\r\n$6\r\nsecret\r\n", // valkey-cli's opening AUTH
		"*1\r\n$4\r\nPING\r\n", // and the command it sends after it
	} {
		if _, err := io.WriteString(client, command); err != nil {
			t.Fatal(err)
		}
		reply := make([]byte, len(refusalReply))
		if _, err := io.ReadFull(client, reply); err != nil || string(reply) != refusalReply {
			t.Fatalf("%q read %q (%v), want Render's refusal", command, reply, err)
		}
	}
}

func TestARefusedSourceReadsRendersRefusalUnderTheStoresCertificate(t *testing.T) {
	f := refusalEdge(t, sniproxy.NewLimiter(defaultMaxConns, defaultMaxConnsPerSource), 0)
	client, err := f.tlsFrom(t, refusedSrc, refusedHost)
	if err != nil {
		t.Fatalf("the refused client could not complete TLS with the store's certificate: %v", err)
	}
	if names := client.ConnectionState().PeerCertificates[0].DNSNames; len(names) != 1 || names[0] != refusedHost {
		t.Fatalf("the refused client saw %v, not the store's certificate", names)
	}
	expectRefusals(t, client)
	if n := f.backend.accepted.Load(); n != 0 {
		t.Fatalf("the store's backend saw %d connections from a refused source", n)
	}
	if got := rejectedCount(t, f.registry, rejectAllowList); got != 1 {
		t.Fatalf("allow-list rejections = %v, want 1", got)
	}
	for _, reason := range []string{sniproxy.ScopeResource, sniproxy.ScopeHandshake, "source", "global"} {
		if got := rejectedCount(t, f.registry, reason); got != 0 {
			t.Fatalf("the refusal was also counted as %s: %v", reason, got)
		}
	}
	// The refusal is no use of the store: none of its bytes is the store's
	// egress.
	if got := gatheredCounter(t, f.registry, "bex_kv_proxy_egress_bytes_total"); got != 0 {
		t.Fatalf("the refusal metered %v bytes as the store's egress", got)
	}

	// The admitted control: its ClientHello goes to the backend unchanged,
	// and the refusal port sees nothing of it.
	refused := f.refusal.accepted.Load()
	admitted := f.dialFrom(t, admittedSrc)
	hello := tls.Client(admitted, &tls.Config{RootCAs: f.roots, ServerName: refusedHost, MinVersion: tls.VersionTLS12})
	go func() { _ = hello.Handshake() }()
	for deadline := time.Now().Add(3 * time.Second); f.backend.accepted.Load() != 1; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the admitted source never reached the store's backend")
		}
	}
	if n := f.refusal.accepted.Load(); n != refused {
		t.Fatalf("an admitted source reached the refusal port")
	}
}

// Whatever the source, a name no public store has is closed after its
// ClientHello, before any handshake and without a dial.
func TestAnUnknownNameIsStillDroppedBeforeTheHandshake(t *testing.T) {
	f := refusalEdge(t, sniproxy.NewLimiter(0, 0), 0)
	for _, name := range []string{"nope.kv.bex.co", "red-one.example.com", "red-one.kv.bex.co.evil.test"} {
		for _, source := range []string{admittedSrc, refusedSrc} {
			_, err := f.tlsFrom(t, source, name)
			var timeout net.Error
			if err == nil || errors.As(err, &timeout) && timeout.Timeout() {
				t.Fatalf("%s from %s: handshake %v, want the connection closed", name, source, err)
			}
		}
	}
	if b, r := f.backend.accepted.Load(), f.refusal.accepted.Load(); b != 0 || r != 0 {
		t.Fatalf("an unknown name was dialed: backend %d, refusal %d", b, r)
	}
	if got := rejectedCount(t, f.registry, rejectAllowList); got != 0 {
		t.Fatalf("an unknown name was counted as an allow-list refusal: %v", got)
	}
}

// A refused connection holds its network's handshake slot for its whole life
// (w21/070), so refused clients cannot exceed the handshake cap, and it takes
// none of the store's route capacity, so it never sheds an admitted client.
func TestARefusedConnectionHoldsItsHandshakeSlotAndNoRouteSlot(t *testing.T) {
	limiter := sniproxy.NewLimiter(0, 0)
	limiter.SetHandshakeLimit(1)
	f := refusalEdge(t, limiter, 1)
	held, err := f.tlsFrom(t, refusedSrc, refusedHost)
	if err != nil {
		t.Fatal(err)
	}
	expectRefusals(t, held)

	// Another client of the same network is shed at its handshake.
	shed := f.dialFrom(t, refusedSrcTwo)
	if got, err := io.ReadAll(shed); err != nil || len(got) != 0 {
		t.Fatalf("a second client of the refused network read %q (%v), want it shed", got, err)
	}
	if got := rejectedCount(t, f.registry, sniproxy.ScopeHandshake); got != 1 {
		t.Fatalf("handshake rejections = %v, want 1", got)
	}

	// The store's one route slot is free for an admitted client.
	admitted := f.dialFrom(t, admittedSrc)
	go func() { _ = tls.Client(admitted, &tls.Config{RootCAs: f.roots, ServerName: refusedHost}).Handshake() }()
	for deadline := time.Now().Add(3 * time.Second); f.backend.accepted.Load() != 1; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the refused connection took the route slot an admitted client needed")
		}
	}
	if got := rejectedCount(t, f.registry, sniproxy.ScopeResource); got != 0 {
		t.Fatalf("route rejections = %v, want 0", got)
	}

	// Once the refused client leaves, its network's slot serves the next one.
	_ = held.Close()
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		next, err := f.tlsFrom(t, refusedSrcTwo, refusedHost)
		if err == nil {
			expectRefusals(t, next)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the refused connection kept its handshake slot after it ended: %v", err)
		}
	}
}

// Degraded path: a store whose proxy has no refusal listener yet (a held
// template), or whose Service still selects Valkey, refuses the dial, and the
// client is closed as before, with nothing reaching the backend.
func TestARefusalPortThatRefusesTheDialClosesTheClient(t *testing.T) {
	f := refusalEdge(t, sniproxy.NewLimiter(0, 0), 0)
	_ = f.refusal.Close()
	began := time.Now()
	_, err := f.tlsFrom(t, refusedSrc, refusedHost)
	var timeout net.Error
	if err == nil || errors.As(err, &timeout) && timeout.Timeout() {
		t.Fatalf("handshake %v, want the connection closed", err)
	}
	if elapsed := time.Since(began); elapsed > 2*time.Second {
		t.Fatalf("the failed dial held the client %v", elapsed)
	}
	if n := f.backend.accepted.Load(); n != 0 {
		t.Fatalf("the store's backend saw %d connections", n)
	}
	if got := rejectedCount(t, f.registry, rejectAllowList); got != 1 {
		t.Fatalf("allow-list rejections = %v, want 1", got)
	}
}

// One deadline bounds the relay both ways: a refusal port that never answers
// cannot hold the client past it.
func TestTheRefusalRelayEndsAtItsDeadline(t *testing.T) {
	silent := holdingListener(t)
	edge := listen(t)
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := edge.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	client, err := net.Dial("tcp", edge.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	server := <-accepted
	t.Cleanup(func() { _ = server.Close() })

	const bound = 300 * time.Millisecond
	began := time.Now()
	refuseAllowList(server, server, []byte("hello"), kvRoute{ResourceID: "red-one", Refusal: silent.Addr().String()},
		began.Add(bound), discardLogger{})
	if elapsed := time.Since(began); elapsed < bound-50*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("the relay ended after %v, want about %v", elapsed, bound)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	if n, err := client.Read(make([]byte, 1)); n != 0 || err == nil {
		t.Fatalf("the client was not closed at the deadline: n=%d err=%v", n, err)
	}
}

// The port kv-sni-proxy dials is the one the operator renders.
func TestTheRefusalPortIsTheOperatorsPort(t *testing.T) {
	if allowListRefusalPort != controller.KeyValueAllowListRefusalPort {
		t.Fatalf("kv-sni-proxy dials %d, the operator renders %d",
			allowListRefusalPort, controller.KeyValueAllowListRefusalPort)
	}
}
