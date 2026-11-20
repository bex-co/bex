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
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/bex-co/bex/lego/operator/internal/sniproxy"
)

// w6/m515: a release's SIGTERM used to close every external Postgres stream at
// once. main hands Serve the signal context's Done channel as the drain, so
// here a cancelled context stands in for the signal.

// TestADrainingProxyRelaysTheResultOfARunningQuery holds a libpq-preamble
// client in a long query through handleConn, starts the drain, and has the
// server answer afterwards: the client reads the result, the proxy accepts no
// new client, and Serve returns as soon as the client leaves.
func TestADrainingProxyRelaysTheResultOfARunningQuery(t *testing.T) {
	const sni = "mydb.db.bex.co"
	backendLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backendLn.Close() })
	asked, answer := make(chan string, 1), make(chan struct{})
	go func() {
		conn, err := backendLn.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		reader := bufio.NewReader(conn)
		if _, err := io.ReadFull(reader, make([]byte, 8)); err != nil {
			return
		}
		if _, err := conn.Write([]byte{sslByte}); err != nil {
			return
		}
		if _, err := io.ReadFull(reader, make([]byte, len(buildClientHello(sni)))); err != nil {
			return
		}
		// The ServerHello's place: a client speaks again only after it.
		if _, err := io.WriteString(conn, "H"); err != nil {
			return
		}
		query, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		asked <- query
		<-answer
		_, _ = io.WriteString(conn, "pg_sleep done\n")
		_, _ = io.Copy(io.Discard, reader)
	}()

	router := newRouter("db.bex.co")
	if err := router.set(&sniproxy.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "mydb", Namespace: "tea-one"},
		Spec:       sniproxy.DatabaseSpec{Public: true, Plan: "free"},
	}); err != nil {
		t.Fatal(err)
	}
	router.mu.Lock()
	for _, routes := range router.hosts {
		for i := range routes {
			routes[i].backend = backendLn.Addr().String()
		}
	}
	router.mu.Unlock()

	meter := sniproxy.NewByteMeter(prometheus.NewRegistry(), "pg_proxy", "postgres")
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
			handleConn(conn, router, meter, nil, limiter, 0, 0, testLogger{})
		}, nil)
	}()
	t.Cleanup(func() { sigterm(); hardStop(); <-served })

	client := dialThroughProxy(t, ln.Addr().String(), sni)
	_ = client.SetDeadline(time.Now().Add(10 * time.Second))
	results := bufio.NewReader(client)
	if hello, err := results.ReadByte(); err != nil || hello != 'H' {
		t.Fatalf("server hello = %q, %v", hello, err)
	}
	if _, err := io.WriteString(client, "SELECT pg_sleep(5)\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-asked:
	case <-time.After(10 * time.Second):
		t.Fatal("the query never reached the server")
	}

	sigterm()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			break
		}
		_ = conn.Close()
		if time.Now().After(deadline) {
			t.Fatal("the draining proxy still accepts new clients")
		}
	}
	close(answer)
	result, err := results.ReadString('\n')
	if err != nil || result != "pg_sleep done\n" {
		t.Fatalf("the running query lost its result to the drain: %q, %v", result, err)
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

// The shipped DaemonSet sets the drain bound this binary reads, pauses before
// SIGTERM so the old pod's endpoint leaves the nodes' service rules while it
// still accepts, and gives the pod a grace period that outlasts both and the
// wait for a scrape of the last streams' bytes (w6/151), so the kubelet never
// kills a draining stream or the bytes it counted. The bound is at most an hour
// (w6/m518): the edge load balancer closes an external stream that carries no
// data for an hour anyway (w6/149), and a stream that never ends is closed once
// whatever the bound, so a longer one would only keep old pods on their nodes.
// An idle timeout set as an incident lever caps it too.
func TestTheDaemonSetGivesTheDrainItsBound(t *testing.T) {
	const maxDrain = time.Hour
	raw, err := os.ReadFile("../../config/pg-sni-proxy/daemonset.yaml")
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
	if grace <= pause+bound+sniproxy.FinalScrapeWait {
		t.Fatalf("terminationGracePeriodSeconds = %v, want more than the %v pause, %v drain and %v final-scrape wait",
			grace, pause, bound, sniproxy.FinalScrapeWait)
	}
}
