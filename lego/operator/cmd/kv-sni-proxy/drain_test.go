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
	"context"
	"crypto/tls"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"

	"github.com/bex-co/bex/lego/operator/internal/sniproxy"
)

// w6/m515: a release's SIGTERM used to close every external Key Value stream at
// once. main hands Serve the signal context's Done channel as the drain, so
// here a cancelled context stands in for the signal.

// TestADrainingProxyRelaysTheReplyToABlockedCommand holds an external TLS
// client in a BLPOP through handleConn, starts the drain, and has the store
// answer afterwards: the client reads the reply, the proxy accepts no new
// client, and Serve returns as soon as the client leaves.
func TestADrainingProxyRelaysTheReplyToABlockedCommand(t *testing.T) {
	const host = "red-one.kv.bex.co"
	certificate, roots := testCertificate(t, host)
	upstream, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = upstream.Close() })
	asked, answer := make(chan string, 1), make(chan struct{})
	go func() {
		conn, err := upstream.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		line, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			return
		}
		asked <- line
		<-answer
		_, _ = io.WriteString(conn, "*2\r\n$1\r\nq\r\n$5\r\nvalue\r\n")
		_, _ = io.Copy(io.Discard, conn)
	}()

	router := newRouter("kv.bex.co")
	kv := &sniproxy.KeyValue{}
	kv.Name, kv.Namespace = "red-one", "tea-one"
	kv.Spec.Public, kv.Spec.Plan = true, "starter"
	kv.Status.ExternalHost = host
	if err := router.set(kv); err != nil {
		t.Fatal(err)
	}
	route := router.table["red-one"]
	route.Backend = upstream.Addr().String()
	router.table["red-one"] = route

	meter := sniproxy.NewByteMeter(prometheus.NewRegistry(), "kv_proxy", "key_value")
	limiter := sniproxy.NewLimiter(defaultMaxConns, defaultMaxConnsPerSource)
	limiter.SetHandshakeLimit(defaultMaxHandshakesPerNetwork)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	signal, sigterm := context.WithCancel(context.Background())
	stop, hardStop := context.WithCancel(context.Background())
	drain := sniproxy.Drain{Start: signal.Done(), Timeout: time.Minute}
	served := make(chan struct{})
	go func() {
		defer close(served)
		sniproxy.Serve(stop, ln, limiter, meter, drain, func(conn net.Conn) {
			handleConn(conn, router, meter, nil, limiter, 0, 0, discardLogger{})
		}, nil)
	}()
	t.Cleanup(func() { sigterm(); hardStop(); <-served })

	raw, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	client := tls.Client(raw, &tls.Config{RootCAs: roots, ServerName: host, MinVersion: tls.VersionTLS12})
	t.Cleanup(func() { _ = client.Close() })
	_ = client.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := io.WriteString(client, "*3\r\n$5\r\nBLPOP\r\n$1\r\nq\r\n$1\r\n0\r\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-asked:
	case <-time.After(10 * time.Second):
		t.Fatal("the BLPOP never reached the store")
	}

	sigterm()
	waitRefused(t, ln.Addr().String())
	close(answer)
	reply := make([]byte, len("*2\r\n$1\r\nq\r\n$5\r\nvalue\r\n"))
	if _, err := io.ReadFull(client, reply); err != nil {
		t.Fatalf("the blocked client lost its reply to the drain: %v", err)
	}
	if string(reply) != "*2\r\n$1\r\nq\r\n$5\r\nvalue\r\n" {
		t.Fatalf("reply = %q", reply)
	}
	select {
	case <-served:
		t.Fatal("the proxy stopped while a client was still open")
	case <-time.After(100 * time.Millisecond):
	}
	_ = client.Close()
	select {
	case <-served:
	case <-time.After(5 * time.Second):
		t.Fatal("the proxy did not stop once its last client left")
	}
}

// waitRefused waits until the draining proxy's port refuses a new client.
func waitRefused(t *testing.T, addr string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return
		}
		_ = conn.Close()
	}
	t.Fatal("the draining proxy still accepts new clients")
}

// The shipped DaemonSet sets the drain bound this binary reads, pauses before
// SIGTERM so the old pod's endpoint leaves the nodes' service rules while it
// still accepts, and gives the pod a grace period that outlasts both, so the
// kubelet never kills a draining stream. The bound is at most an hour
// (w6/m518): the edge load balancer closes an external stream that carries no
// data for an hour anyway (w6/149), and a stream that never ends is closed once
// whatever the bound, so a longer one would only keep old pods on their nodes.
// An idle timeout set as an incident lever caps it too.
func TestTheDaemonSetGivesTheDrainItsBound(t *testing.T) {
	const maxDrain = time.Hour
	raw, err := os.ReadFile("../../config/kv-sni-proxy/daemonset.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var ds appsv1.DaemonSet
	if err := yaml.Unmarshal(raw, &ds); err != nil {
		t.Fatal(err)
	}
	var bound, idle time.Duration
	for _, env := range ds.Spec.Template.Spec.Containers[0].Env {
		switch env.Name {
		case drainSecondsEnv:
			t.Setenv(env.Name, env.Value)
			if bound, err = sniproxy.DrainSeconds(drainSecondsEnv); err != nil {
				t.Fatal(err)
			}
		case idleTimeoutEnv:
			if idle, err = time.ParseDuration(env.Value); err != nil {
				t.Fatal(err)
			}
		}
	}
	if bound <= 0 {
		t.Fatalf("the DaemonSet does not set %s to a positive bound", drainSecondsEnv)
	}
	if bound > maxDrain {
		t.Fatalf("%s = %v, want at most %v", drainSecondsEnv, bound, maxDrain)
	}
	if idle > 0 && bound > idle {
		t.Fatalf("%s = %v, want at most the %v idle timeout", drainSecondsEnv, bound, idle)
	}
	var pause time.Duration
	if lifecycle := ds.Spec.Template.Spec.Containers[0].Lifecycle; lifecycle != nil &&
		lifecycle.PreStop != nil && lifecycle.PreStop.Sleep != nil {
		pause = time.Duration(lifecycle.PreStop.Sleep.Seconds) * time.Second
	}
	if pause <= 0 {
		t.Fatal("the DaemonSet does not pause before SIGTERM, so a client routed to the old pod is refused")
	}
	var grace time.Duration
	if seconds := ds.Spec.Template.Spec.TerminationGracePeriodSeconds; seconds != nil {
		grace = time.Duration(*seconds) * time.Second
	}
	if grace <= pause+bound {
		t.Fatalf("terminationGracePeriodSeconds = %v, want more than the %v pause plus the %v drain", grace, pause, bound)
	}
}
