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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w21/m394 t012 (R25-02): a held request kept its global and per-App slot
// through the whole proxied exchange, and the proxy transport had no response
// header timeout, so an app that accepted a request and never answered pinned
// slots until the client gave up. Four such apps x 64 requests filled the 256
// slots of a replica and every other tenant's cold start was shed.

// hungUpstream answers /hang only when released and everything else at once.
type hungUpstream struct {
	release chan struct{}
	hanging atomic.Int32
}

func (u *hungUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/hang" {
		u.hanging.Add(1)
		select {
		case <-u.release:
		case <-r.Context().Done():
			return
		}
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = io.WriteString(w, "app")
}

// holdFor builds a holding handler over the sleeping App whose woken Service is
// upstream, with an already-ready wake and the given number of global slots.
func holdFor(t *testing.T, upstream http.Handler, slots int) (http.Handler, *holder) {
	t.Helper()
	app, dep := sleepingApp()
	app.Status.Phase = appv1alpha1.PhaseHibernated
	cache, base := primedHostCache(t, app, dep)
	cl := &countingClient{Client: base}
	srv := httptest.NewServer(upstream)
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	h := newHolder(cl)
	h.bound, h.pollInterval = 5*time.Second, 10*time.Millisecond
	h.total = make(chan struct{}, slots)
	h.target = func(*appv1alpha1.App) *url.URL { return target }
	h.ready = func(context.Context, *appv1alpha1.App) bool { return true }
	return newHoldingHandler(cl, cache, logr.Discard(), h), h
}

func jsonGet(path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "https://sleeping.onbex.co"+path, nil)
	req.Header.Set("Accept", "application/json")
	return req
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestHungAppDoesNotPinHoldSlots: once the App is awake and the exchange is
// proxied, its slot is free, so a request that never gets an answer cannot
// starve other cold starts. The slot covers only the wait.
func TestHungAppDoesNotPinHoldSlots(t *testing.T) {
	up := &hungUpstream{release: make(chan struct{})}
	handler, h := holdFor(t, up, 2)

	var done sync.WaitGroup
	for range 2 {
		done.Go(func() { handler.ServeHTTP(httptest.NewRecorder(), jsonGet("/hang")) })
	}
	t.Cleanup(func() {
		close(up.release)
		done.Wait()
	})
	waitUntil(t, "both requests reach the app", func() bool { return up.hanging.Load() == 2 })

	if n := len(h.total); n != 0 {
		t.Fatalf("%d hold slots in use while two proxied exchanges hang, want 0", n)
	}
	rr := httptest.NewRecorder()
	start := time.Now()
	handler.ServeHTTP(rr, jsonGet("/ok"))
	if rr.Code != http.StatusAccepted || rr.Body.String() != "app" {
		t.Fatalf("a third request = %d %q, want the app's answer (before: shed, two hung exchanges held every slot)",
			rr.Code, rr.Body.String())
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("third request took %v", time.Since(start))
	}
}

// TestUnansweringAppEndsAtTheResponseHeaderTimeout: an exchange with an app that
// accepts the request and never answers ends with a gateway error instead of
// holding its connection and goroutine until the client leaves.
func TestUnansweringAppEndsAtTheResponseHeaderTimeout(t *testing.T) {
	up := &hungUpstream{release: make(chan struct{})}
	t.Cleanup(func() { close(up.release) })
	handler, h := holdFor(t, up, 4)
	// Render allows HTTP responses up to 100 minutes; a woken app gets that long
	// too (w8/329).
	if got := h.proxy.Transport.(*http.Transport).ResponseHeaderTimeout; got != 100*time.Minute {
		t.Fatalf("transport ResponseHeaderTimeout = %v, want Render's 100 minutes", got)
	}
	// Shorten it so the timeout path itself runs.
	h.responseHeaderTimeout = 200 * time.Millisecond
	h.proxy = h.newProxy()

	rr := httptest.NewRecorder()
	start := time.Now()
	handler.ServeHTTP(rr, jsonGet("/hang"))
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("unanswered exchange = %d %q, want 502", rr.Code, rr.Body.String())
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("unanswered exchange took %v, want about the 200ms timeout", elapsed)
	}
}

// TestUpgradeThroughTheHoldPathStillWorks: a WebSocket-style upgrade is held
// through the wake, proxied, and keeps streaming after its hold slot is back.
func TestUpgradeThroughTheHoldPathStillWorks(t *testing.T) {
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "no upgrade", http.StatusBadRequest)
			return
		}
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		_ = rw.Flush()
		buf := make([]byte, 4)
		if _, err := io.ReadFull(rw, buf); err == nil {
			_, _ = conn.Write(buf)
		}
	})
	handler, h := holdFor(t, echo, 2)
	front := httptest.NewServer(handler)
	t.Cleanup(front.Close)

	conn, err := net.Dial("tcp", front.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = io.WriteString(conn, "GET /socket HTTP/1.1\r\nHost: sleeping.onbex.co\r\n"+
		"Connection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
	br := bufio.NewReader(conn)
	status, err := br.ReadString('\n')
	if err != nil || !strings.Contains(status, "101") {
		t.Fatalf("upgrade answered %q, %v, want 101", status, err)
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	if n := len(h.total); n != 0 {
		t.Fatalf("%d hold slots in use during an open upgrade, want 0", n)
	}
	_, _ = io.WriteString(conn, "ping")
	got := make([]byte, 4)
	if _, err := io.ReadFull(br, got); err != nil || string(got) != "ping" {
		t.Fatalf("echo = %q, %v, want ping", got, err)
	}
}
