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
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/operator/internal/sniproxy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m457: before, every database's external clients stopped at 64 per
// resource and 256 per workspace, and the 65th read a dropped connection. A
// route now admits twice its server's limit, so PostgreSQL itself refuses the
// client past its max_connections.

// sslBackend stands in for CNPG: it answers the proxied SSLRequest with 'S' and
// then holds the connection, counting every dial the proxy makes.
func sslBackend(t *testing.T) (addr string, dials *atomic.Int64) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dials = new(atomic.Int64)
	var mu sync.Mutex
	var held []net.Conn
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			dials.Add(1)
			mu.Lock()
			held = append(held, conn)
			mu.Unlock()
			go func() {
				request := make([]byte, 8)
				if _, err := io.ReadFull(conn, request); err == nil {
					_, _ = conn.Write([]byte{sslByte})
				}
				_, _ = io.Copy(io.Discard, conn)
			}()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range held {
			_ = conn.Close()
		}
	})
	return ln.Addr().String(), dials
}

// dialThroughProxy speaks the libpq preamble to the proxy up to the TLS
// ClientHello, after which the proxy either dials the backend or sheds.
func dialThroughProxy(t *testing.T, proxy, sni string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", proxy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	sslRequest := make([]byte, 8)
	binary.BigEndian.PutUint32(sslRequest[0:4], uint32(sslRequestLen))
	binary.BigEndian.PutUint32(sslRequest[4:8], uint32(sslRequestMagic))
	if _, err := conn.Write(sslRequest); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 1)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(conn, reply); err != nil || reply[0] != sslByte {
		t.Fatalf("SSLRequest answer = %v, %v", reply, err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	if _, err := conn.Write(buildClientHello(sni)); err != nil {
		t.Fatal(err)
	}
	return conn
}

// waitForDials waits until the backend has been dialed want times in all.
func waitForDials(t *testing.T, dials *atomic.Int64, want int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for dials.Load() < want {
		if time.Now().After(deadline) {
			t.Fatalf("backend dials = %d, want %d: the proxy shed an admissible client", dials.Load(), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// requireShed reads the proxy's answer to a client it must shed: the
// connection closed at once (the stub backend never answers, so a relayed
// client would read nothing until the deadline).
func requireShed(t *testing.T, conn net.Conn) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err := conn.Read(make([]byte, 1))
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		t.Fatal("the proxy relayed a client it should have shed")
	}
}

// TestARouteAdmitsTwiceItsPlansLimitAndShedsOnlyBeyond drives a basic-256mb
// database (Render's 100 connections) through handleConn: 200 clients reach the
// backend, the 201st is shed without a dial and metered as reason=resource,
// the same database's pooled hostname and another tenant's database still
// admit, and a client that leaves frees its slot.
func TestARouteAdmitsTwiceItsPlansLimitAndShedsOnlyBeyond(t *testing.T) {
	backend, dials := sslBackend(t)
	router := newRouter("db.bex.co")
	for _, db := range []*appv1alpha1.Database{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "mydb", Namespace: "tea-one"},
			Spec:       appv1alpha1.DatabaseSpec{Public: true, Plan: "basic-256mb", Pooler: true},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "tea-two"},
			Spec:       appv1alpha1.DatabaseSpec{Public: true, Plan: "basic-256mb"},
		},
	} {
		if err := router.set(db); err != nil {
			t.Fatal(err)
		}
	}
	router.mu.Lock()
	for _, routes := range router.hosts {
		for i := range routes {
			routes[i].backend = backend
		}
	}
	router.mu.Unlock()

	registry := prometheus.NewRegistry()
	meter := sniproxy.NewByteMeter(registry, "pg_proxy", "postgres")
	// The shipped process caps, so the test also proves none of them binds
	// below a route's capacity for one client address.
	limiter := sniproxy.NewLimiter(defaultMaxConns, defaultMaxConnsPerSource)
	limiter.SetHandshakeLimit(defaultMaxHandshakesPerNetwork)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = ln.Close() })
	go sniproxy.Serve(ctx, ln, limiter, meter, func(conn net.Conn) {
		handleConn(conn, router, meter, nil, limiter, 0, 0, testLogger{})
	}, nil)
	proxy := ln.Addr().String()

	const capacity = 200 // twice Render's 100 for basic-256mb
	admitted := make([]net.Conn, 0, capacity)
	for range capacity {
		admitted = append(admitted, dialThroughProxy(t, proxy, "mydb.db.bex.co"))
	}
	waitForDials(t, dials, capacity)
	requireShed(t, dialThroughProxy(t, proxy, "mydb.db.bex.co"))
	if got := dials.Load(); got != capacity {
		t.Fatalf("the shed client was dialed: backend dials = %d", got)
	}
	if got := rejectedCount(t, registry, sniproxy.ScopeResource); got != 1 {
		t.Fatalf("resource rejections = %v, want 1", got)
	}
	for i, sni := range []string{"mydb-pool.db.bex.co", "other.db.bex.co"} {
		dialThroughProxy(t, proxy, sni)
		waitForDials(t, dials, capacity+int64(i)+1)
	}

	_ = admitted[0].Close()
	deadline := time.Now().Add(3 * time.Second)
	for {
		release, ok := limiter.AcquireRoute("tea-one/mydb/mydb", capacity)
		if ok {
			release()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a client that left still holds its route slot")
		}
		time.Sleep(10 * time.Millisecond)
	}
	dialThroughProxy(t, proxy, "mydb.db.bex.co")
	waitForDials(t, dials, capacity+3)
}
