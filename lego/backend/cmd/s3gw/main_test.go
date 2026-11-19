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
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/s3gw"
	"github.com/bex-co/bex/lego/backend/internal/serve"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestHalfAWasabiPairIsAStartupError(t *testing.T) {
	_, err := loadConfig(env(map[string]string{"BEX_OBJECT_STORAGE_ACCESS_KEY": "a"}))
	if err == nil || !strings.Contains(err.Error(), "BEX_OBJECT_STORAGE_SECRET_KEY") {
		t.Errorf("loadConfig with half a pair = %v, want an error naming the missing half", err)
	}
}

func TestATrustedProxyListIsParsedAndABadOneIsAStartupError(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{"BEX_TRUSTED_PROXY_CIDRS": "10.244.0.0/16, 10.1.0.0/24"}))
	if err != nil || len(cfg.TrustedProxies) != 2 {
		t.Fatalf("loadConfig = %+v, %v, want two trusted prefixes", cfg, err)
	}
	if _, err := loadConfig(env(map[string]string{"BEX_TRUSTED_PROXY_CIDRS": "10.244.0.0/16,nonsense"})); err == nil || !strings.Contains(err.Error(), "BEX_TRUSTED_PROXY_CIDRS") {
		t.Errorf("a malformed CIDR = %v, want a startup error", err)
	}
}

func TestADatabaseWithNoOpenFGAIsRefusedBecauseItWouldAuthorizeNothing(t *testing.T) {
	_, err := loadConfig(env(map[string]string{
		"BEX_OBJECT_STORAGE_ACCESS_KEY": "a", "BEX_OBJECT_STORAGE_SECRET_KEY": "s", "BEX_CP_DB_URI": "postgres://x", "BEX_OPENBAO_URL": "http://bao",
	}))
	if err == nil || !strings.Contains(err.Error(), "BEX_OPENFGA_URL") {
		t.Errorf("loadConfig with a database and no OpenFGA = %v, want a refusal", err)
	}
}

func TestWithoutACredentialTheSurfaceIsDegradedNotCrashed(t *testing.T) {
	cases := map[string]map[string]string{
		"no Wasabi pair": {"BEX_CP_DB_URI": "postgres://x", "BEX_OPENFGA_URL": "http://fga", "BEX_OPENBAO_URL": "http://bao"},
		"no database":    {"BEX_OBJECT_STORAGE_ACCESS_KEY": "a", "BEX_OBJECT_STORAGE_SECRET_KEY": "s", "BEX_OPENBAO_URL": "http://bao"},
		"no OpenBao":     {"BEX_OBJECT_STORAGE_ACCESS_KEY": "a", "BEX_OBJECT_STORAGE_SECRET_KEY": "s", "BEX_CP_DB_URI": "postgres://x", "BEX_OPENFGA_URL": "http://fga"},
	}
	for name, vars := range cases {
		cfg, err := loadConfig(env(vars))
		if err != nil {
			t.Fatalf("%s: loadConfig = %v, want a degraded start and not a startup error", name, err)
		}
		if cfg.degradedReason() == "" {
			t.Errorf("%s: the surface would serve", name)
		}
	}
	full, err := loadConfig(env(map[string]string{
		"BEX_OBJECT_STORAGE_ACCESS_KEY": "a", "BEX_OBJECT_STORAGE_SECRET_KEY": "s", "BEX_CP_DB_URI": "postgres://x", "BEX_OPENFGA_URL": "http://fga", "BEX_OPENBAO_URL": "http://bao",
	}))
	if err != nil || full.degradedReason() != "" {
		t.Errorf("a complete environment = %v, %q, want it to serve", err, full.degradedReason())
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// A stream that is running when SIGTERM arrives completes, and a new request
// after the drain window is refused: a rollout never cuts an upload in flight
// (ADR112 D3).
func TestAStreamRunningAtSIGTERMCompletesAndANewRequestAfterTheWindowIsRefused(t *testing.T) {
	var started atomic.Bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// An upload: the handler reads the body to its end, which the client is
		// still sending when the signal arrives.
		started.Store(true)
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Errorf("the handler's body read failed mid-stream: %v", err)
		}
		_, _ = w.Write([]byte("done"))
	})
	addr := freeAddr(t)
	srv := s3gw.NewServer(addr, handler)
	ready := &serve.Readiness{}
	ctx, term := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runUntilStopped(ctx, srv, ready, 300*time.Millisecond) }()
	waitListening(t, addr)

	pr, pw := io.Pipe()
	respc := make(chan *http.Response, 1)
	errc := make(chan error, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodPut, "http://"+addr+"/photos/big.bin", pr)
		req.ContentLength = -1
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			errc <- err
			return
		}
		respc <- resp
	}()
	_, _ = pw.Write([]byte("first part of an upload"))
	for !started.Load() {
		time.Sleep(5 * time.Millisecond)
	}

	term() // SIGTERM
	deadline := time.Now().Add(2 * time.Second)
	for !ready.Draining() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !ready.Draining() {
		t.Fatal("readiness did not flip to draining on SIGTERM")
	}
	// Inside the drain window a new request is still served, so late-routed
	// traffic does not hit a closed listener.
	if resp, err := http.Get("http://" + addr + "/photos"); err != nil {
		t.Errorf("a request inside the drain window failed: %v", err)
	} else {
		_ = resp.Body.Close()
	}
	// The stream keeps going past the window and then finishes.
	time.Sleep(500 * time.Millisecond)
	_, _ = pw.Write([]byte(" and the rest"))
	_ = pw.Close()
	select {
	case resp := <-respc:
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if string(body) != "done" {
			t.Errorf("the stream answered %q, want done", body)
		}
	case err := <-errc:
		t.Fatalf("the stream was cut: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("the stream never finished")
	}
	// After the drain a new request is refused outright.
	if err := <-done; err != nil {
		t.Errorf("runUntilStopped = %v, want a clean drain", err)
	}
	if resp, err := http.Get("http://" + addr + "/photos"); err == nil {
		_ = resp.Body.Close()
		t.Errorf("a request after the drain was served (%d), want it refused", resp.StatusCode)
	}
}

func waitListening(t *testing.T, addr string) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if c, err := net.Dial("tcp", addr); err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("nothing listens on %s", addr)
}

func TestTheDrainFitsInsideThePodsGracePeriodTheManifestSets(t *testing.T) {
	raw, err := os.ReadFile("../../../operator/config/s3gw/deployment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^\s*terminationGracePeriodSeconds:\s*(\d+)\s*$`).FindSubmatch(raw)
	if m == nil {
		t.Fatal("deployment.yaml sets no terminationGracePeriodSeconds")
	}
	seconds, _ := strconv.Atoi(string(m[1]))
	grace := time.Duration(seconds) * time.Second
	// The drain window, the stream deadline and the shutdown margin must fit
	// inside it, or a rollout kills the streams it is meant to wait for.
	if total := drainWindow + objectstorage.StreamDeadline + shutdownMargin; total > grace {
		t.Errorf("drain %v + stream deadline %v + margin %v = %v, more than the grace period %v", drainWindow, objectstorage.StreamDeadline, shutdownMargin, total, grace)
	}
	if grace <= objectstorage.StreamDeadline {
		t.Errorf("the grace period %v is not above the stream deadline %v", grace, objectstorage.StreamDeadline)
	}
}
