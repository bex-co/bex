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

package sandbox

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// runtime_unavailable_test.go is w8/248. The OpenSandbox lifecycle server is
// one replica that is recreated on every rollout, so each restart has a window
// with no server. In that window `bex sandboxes list`, a sandbox read and a
// stop answered a bare `500 internal error` in every workspace. They now
// answer a named, retryable 503.

// The repeats are the same code at any spacing, so no test waits out
// production's.
func init() {
	defaultReadRetryBackoff = []time.Duration{time.Millisecond, time.Millisecond}
}

// wantRuntimeUnavailable asserts err is the named 503 with its Retry-After.
func wantRuntimeUnavailable(t *testing.T, what string, err error) {
	t.Helper()
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != CodeSandboxRuntimeUnavailable {
		t.Fatalf("%s = %v, want %s", what, err, CodeSandboxRuntimeUnavailable)
	}
	if got := core.ErrorStatus(err); got != http.StatusServiceUnavailable {
		t.Errorf("%s answers %d, want 503", what, got)
	}
	if seconds, _ := coded.Params["retryAfterSeconds"].(int); seconds != runtimeRetryAfterSeconds {
		t.Errorf("%s retryAfterSeconds = %v, want %d", what, coded.Params["retryAfterSeconds"], runtimeRetryAfterSeconds)
	}
	rec := httptest.NewRecorder()
	core.WriteErr(rec, err)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != "5" {
		t.Errorf("%s over REST = %d Retry-After %q, want 503 and 5", what, rec.Code, rec.Header().Get("Retry-After"))
	}
}

// refusingRuntime is a Service whose runtime address accepts no connection:
// a listener that was closed, as a Service with no ready pod behind it is.
func refusingRuntime(t *testing.T) *Service {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	svc := stubServer(t, func(http.ResponseWriter, *http.Request) {})
	svc.Client = NewClient("http://" + addr)
	return svc
}

func TestUnreachableRuntimeAnswersANamed503(t *testing.T) {
	svc := refusingRuntime(t)
	_, err := svc.List(callerCtx())
	wantRuntimeUnavailable(t, "list with the runtime down", err)
	_, err = svc.Get(callerCtx(), "sbx-00000000000000000000")
	wantRuntimeUnavailable(t, "get with the runtime down", err)
	_, err = svc.Create(callerCtx(), CreateRequest{Template: "node"})
	wantRuntimeUnavailable(t, "create with the runtime down", err)
}

func TestAStatusThatSaysTheRuntimeCouldNotServeAnswersANamed503(t *testing.T) {
	for _, status := range []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusInternalServerError} {
		svc := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte("upstream connect error"))
		})
		svc.Client.readRetryBackoff = nil
		_, err := svc.List(callerCtx())
		wantRuntimeUnavailable(t, "list answered "+http.StatusText(status), err)
	}
	// A write is told the same for a 503 alone, the one status that says the
	// request was not taken. After any other 5xx the write may have been
	// applied, so the caller is not told to send it again.
	refused := stubServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	_, err := refused.Create(callerCtx(), CreateRequest{Template: "node"})
	wantRuntimeUnavailable(t, "create answered 503", err)
	var coded *core.CodedError
	for _, status := range []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout} {
		ambiguous := stubServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
		if _, err := ambiguous.Create(callerCtx(), CreateRequest{Template: "node"}); err == nil || errors.As(err, &coded) {
			t.Fatalf("create answered %d = %v, want the unmapped server error", status, err)
		}
	}
	// A 5xx the lifecycle server sent with its own error code is the server
	// answering, on a read too.
	own := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":"KUBERNETES::INTERNAL","message":"boom"}`))
	})
	own.Client.readRetryBackoff = nil
	if _, err := own.List(callerCtx()); err == nil || errors.As(err, &coded) {
		t.Fatalf("list answered the server's own 503 = %v, want the unmapped server error", err)
	}
}

// A read is repeated, and one that then succeeds is a success: the caller
// never sees the moment the server's pod was replaced. A write is sent once.
func TestAReadRidesOutAShortRuntimeGap(t *testing.T) {
	var gets, posts atomic.Int32
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if gets.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	})
	if _, err := svc.List(callerCtx()); err != nil {
		t.Fatalf("list after two failed attempts = %v, want the third attempt's answer", err)
	}
	if n := gets.Load(); n != 3 {
		t.Fatalf("list made %d requests, want 3", n)
	}
	if _, err := svc.Create(callerCtx(), CreateRequest{Template: "node"}); err == nil {
		t.Fatal("create against a 503 succeeded")
	}
	if n := posts.Load(); n != 1 {
		t.Fatalf("create was sent %d times, want once", n)
	}
	// The caller's own deadline is the caller's, not a runtime outage.
	gets.Store(-1000)
	ctx, cancel := context.WithTimeout(callerCtx(), 20*time.Millisecond)
	defer cancel()
	svc.Client.readRetryBackoff = []time.Duration{time.Second, time.Second}
	_, err := svc.List(ctx)
	var coded *core.CodedError
	if err == nil || errors.As(err, &coded) {
		t.Fatalf("list past the caller's deadline = %v, want the deadline error", err)
	}
}

// A create counts the workspace's sandboxes while it holds the workspace's
// admission lock, a transaction and a pooled connection. That read is sent
// once, so a runtime that is down does not make every create of the workspace
// hold all three for the length of the repeats.
func TestTheAdmissionReadIsSentOnce(t *testing.T) {
	f := newGroupFixture(t)
	g := f.createGroup(t, "down")
	var gets atomic.Int32
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets.Add(1)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(down.Close)
	f.svc.Client = NewClient(down.URL)
	code, body := f.create(t, g.ID)
	if code != http.StatusServiceUnavailable || !strings.Contains(body, CodeSandboxRuntimeUnavailable) {
		t.Fatalf("create with the runtime down = %d %s, want 503 %s", code, body, CodeSandboxRuntimeUnavailable)
	}
	if n := gets.Load(); n != 1 {
		t.Fatalf("the admission read was sent %d times, want once", n)
	}
}
