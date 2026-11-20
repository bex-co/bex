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

package agentsession

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// The gateway's signed calls to bex-api (w6/153): a pooled connection that died
// while idle is the production failure. On a node whose conntrack table is
// full, the idle connection's service entry is evicted and its next segment is
// balanced to the other bex-api pod, which resets it. These tests reset the
// connection the same way, with an RST and no answer.

const transportTestSubject = "kratos-subject-w6-153"

// seenRequest is one request as bex-api received it.
type seenRequest struct {
	body                 []byte
	timestamp, signature string
	remote               string
}

// connResetServer is bex-api's email verb behind a connection that can die: a
// request reset picks (by its 1-based number and the client's address) gets
// its connection reset instead of an answer. With claimFirst, the verb runs
// before the reset (the nonce is claimed and the answer lost); without it, the
// request never reaches the verb, as a segment balanced to the wrong pod never
// does. hold, when set, runs before each request is served.
type connResetServer struct {
	t          *testing.T
	verb       http.Handler
	reset      func(n int, remote string) bool
	claimFirst bool
	hold       func(n int)

	mu       sync.Mutex
	requests []seenRequest
	resolved int
}

func newConnResetServer(t *testing.T, reset func(n int, remote string) bool, claimFirst bool) (*connResetServer, *EmailVerificationClient) {
	t.Helper()
	secret := []byte("exec-secret")
	s := &connResetServer{t: t, reset: reset, claimFirst: claimFirst}
	s.verb = &EmailVerificationHandler{
		Secret: secret,
		Nonce:  &fakeNonceClaimer{},
		Resolve: func(context.Context, string) (bool, bool, error) {
			s.mu.Lock()
			s.resolved++
			s.mu.Unlock()
			return true, true, nil
		},
	}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	client := &EmailVerificationClient{URL: srv.URL + InternalEmailVerificationPath, Secret: secret, HTTP: &http.Client{Timeout: 5 * time.Second, Transport: srv.Client().Transport}}
	return s, client
}

// resetRequest resets the request numbered n.
func resetRequest(n int) func(int, string) bool {
	return func(got int, _ string) bool { return got == n }
}

func (s *connResetServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.requests = append(s.requests, seenRequest{body: body, timestamp: r.Header.Get(TimestampHeader), signature: r.Header.Get(SignatureHeader), remote: r.RemoteAddr})
	n := len(s.requests)
	s.mu.Unlock()
	r.Body = io.NopCloser(bytes.NewReader(body))
	if s.hold != nil {
		s.hold(n)
	}
	if !s.reset(n, r.RemoteAddr) {
		s.verb.ServeHTTP(w, r)
		return
	}
	if s.claimFirst {
		s.verb.ServeHTTP(httptest.NewRecorder(), r)
	}
	conn, _, err := http.NewResponseController(w).Hijack()
	if err != nil {
		s.t.Errorf("hijack request %d: %v", n, err)
		return
	}
	_ = conn.(*net.TCPConn).SetLinger(0) // close with an RST, as a pod without the connection answers
	_ = conn.Close()
}

func (s *connResetServer) seen() ([]seenRequest, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]seenRequest(nil), s.requests...), s.resolved
}

// captureBrokerLog sends the standard logger to a buffer for the rest of the
// test.
func captureBrokerLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	previous := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	return buf
}

func TestSignedPostReplaysADeadPooledConnectionOnANewOne(t *testing.T) {
	logged := captureBrokerLog(t)
	s, client := newConnResetServer(t, resetRequest(2), false)
	ctx := context.Background()

	if err := client.Check(ctx, transportTestSubject); err != nil {
		t.Fatalf("first check = %v, want allowed", err)
	}
	if err := client.Check(ctx, transportTestSubject); err != nil {
		t.Fatalf("check on the dead pooled connection = %v, want it answered by the retry", err)
	}

	requests, resolved := s.seen()
	if len(requests) != 3 || resolved != 2 {
		t.Fatalf("bex-api saw %d requests and answered %d, want 3 and 2 (the dead connection's request reached no handler)", len(requests), resolved)
	}
	dead, replay := requests[1], requests[2]
	if dead.remote != requests[0].remote {
		t.Fatalf("second check ran on %s, want the pooled connection %s", dead.remote, requests[0].remote)
	}
	if replay.remote == dead.remote {
		t.Fatalf("the replay ran on the dead connection %s, want a new one", dead.remote)
	}
	if !bytes.Equal(replay.body, dead.body) || replay.timestamp != dead.timestamp || replay.signature != dead.signature {
		t.Fatal("the replay re-signed or changed the request; it must resend the same signed bytes, so its nonce stays single-use")
	}
	out := logged.String()
	for _, want := range []string{
		"agent credential broker: POST " + InternalEmailVerificationPath + " attempt 1 failed: cause=reset (reused connection idle ",
		"retrying on a new connection: ",
		"connection reset by peer",
		"agent credential broker: POST " + InternalEmailVerificationPath + " answered on retry (status 200)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("log = %q, want it to carry %q", out, want)
		}
	}
	if strings.Contains(out, transportTestSubject) {
		t.Fatalf("log = %q, carries the subject", out)
	}
}

// Two held sessions keep two connections in the pool, and a full conntrack
// table evicts both; the replay must not take the second dead one.
func TestSignedPostReplaysOnANewConnectionWhenEveryPooledOneIsDead(t *testing.T) {
	logged := captureBrokerLog(t)
	var mu sync.Mutex
	dead := map[string]bool{}
	s, client := newConnResetServer(t, func(_ int, remote string) bool {
		mu.Lock()
		defer mu.Unlock()
		return dead[remote]
	}, false)
	var bothArrived sync.WaitGroup
	bothArrived.Add(2)
	s.hold = func(n int) {
		if n <= 2 { // the two warm-up checks wait for each other, so each takes a connection
			bothArrived.Done()
			bothArrived.Wait()
		}
	}
	ctx := context.Background()

	warm := make(chan error, 2)
	for range 2 {
		go func() { warm <- client.Check(ctx, transportTestSubject) }()
	}
	for range 2 {
		if err := <-warm; err != nil {
			t.Fatalf("warm-up check = %v, want allowed", err)
		}
	}
	requests, _ := s.seen()
	mu.Lock()
	for _, r := range requests {
		dead[r.remote] = true
	}
	pooled := len(dead)
	mu.Unlock()
	if pooled != 2 {
		t.Fatalf("warm-up used %d connections, want 2 in the pool", pooled)
	}

	if err := client.Check(ctx, transportTestSubject); err != nil {
		t.Fatalf("check with every pooled connection dead = %v, want it answered on a new connection", err)
	}
	requests, resolved := s.seen()
	if len(requests) != 4 || resolved != 3 {
		t.Fatalf("bex-api saw %d requests and answered %d, want 4 and 3", len(requests), resolved)
	}
	if dead[requests[3].remote] {
		t.Fatalf("the replay ran on pooled connection %s, which had died too", requests[3].remote)
	}
	if out := logged.String(); strings.Count(out, "failed: cause=reset (reused connection") != 1 || !strings.Contains(out, "answered on retry") {
		t.Fatalf("log = %q, want one reset on a pooled connection and the answered retry", out)
	}
}

func TestSignedPostReportsAReplayBexAPIRefusesAsSeenUnavailable(t *testing.T) {
	logged := captureBrokerLog(t)
	s, client := newConnResetServer(t, resetRequest(2), true)
	ctx := context.Background()

	if err := client.Check(ctx, transportTestSubject); err != nil {
		t.Fatalf("first check = %v, want allowed", err)
	}
	err := client.Check(ctx, transportTestSubject)
	if err == nil || err.Error() != "agent credential broker unavailable" || errors.Is(err, ErrForbidden) {
		t.Fatalf("check whose answer was lost = %v, want the unavailable error, never a refusal", err)
	}
	if requests, resolved := s.seen(); len(requests) != 3 || resolved != 2 {
		t.Fatalf("bex-api saw %d requests and answered %d, want 3 and 2: the replay's claimed nonce must stop it before the verb", len(requests), resolved)
	}
	if out := logged.String(); !strings.Contains(out, "cause=reset") || !strings.Contains(out, "retry refused (401)") {
		t.Fatalf("log = %q, want the reset and the refused retry", out)
	}
}

func TestSignedPostDoesNotReplayARequestOnANewConnection(t *testing.T) {
	logged := captureBrokerLog(t)
	s, client := newConnResetServer(t, resetRequest(1), false)

	err := client.Check(context.Background(), transportTestSubject)
	if err == nil || err.Error() != "agent credential broker unavailable" {
		t.Fatalf("check = %v, want the unavailable error", err)
	}
	if requests, _ := s.seen(); len(requests) != 1 {
		t.Fatalf("bex-api saw %d requests, want 1: a new connection that took the request may have been answered", len(requests))
	}
	if out := logged.String(); !strings.Contains(out, "attempt 1 failed: cause=reset (new connection); giving up: ") {
		t.Fatalf("log = %q, want the reset on a new connection and no retry", out)
	}
}

func TestSignedPostRetriesAFailedDialOnce(t *testing.T) {
	logged := captureBrokerLog(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := "http://" + listener.Addr().String() + InternalEmailVerificationPath
	_ = listener.Close()
	client := &EmailVerificationClient{URL: closed, Secret: []byte("exec-secret"), HTTP: &http.Client{Timeout: 5 * time.Second}}

	err = client.Check(context.Background(), transportTestSubject)
	if err == nil || err.Error() != "agent credential broker unavailable" {
		t.Fatalf("check = %v, want the unavailable error", err)
	}
	out := logged.String()
	for _, want := range []string{
		"POST " + InternalEmailVerificationPath + " attempt 1 failed: cause=dial (new connection); retrying on a new connection: ",
		"POST " + InternalEmailVerificationPath + " attempt 2 failed: cause=dial (new connection); giving up: ",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("log = %q, want it to carry %q", out, want)
		}
	}
}

func TestSignedPostDoesNotReplayATimeout(t *testing.T) {
	logged := captureBrokerLog(t)
	release := make(chan struct{})
	var mu sync.Mutex
	served := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		mu.Lock()
		served++
		n := served
		mu.Unlock()
		if n > 1 { // the pooled connection's next request may be running in bex-api: it never answers in time
			<-release
			return
		}
		WriteNoStoreJSON(w, http.StatusOK, EmailVerificationResponse{Subject: transportTestSubject})
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	client := &EmailVerificationClient{URL: srv.URL, Secret: []byte("exec-secret"), HTTP: &http.Client{Timeout: 200 * time.Millisecond}}

	if err := client.Check(context.Background(), transportTestSubject); err != nil {
		t.Fatalf("first check = %v, want allowed", err)
	}
	err := client.Check(context.Background(), transportTestSubject)
	if err == nil || err.Error() != "agent credential broker unavailable" {
		t.Fatalf("check = %v, want the unavailable error", err)
	}
	mu.Lock()
	defer mu.Unlock()
	out := logged.String()
	if served != 2 || !strings.Contains(out, "attempt 1 failed: cause=timeout (reused connection") || !strings.Contains(out, "giving up") || strings.Contains(out, "attempt 2") {
		t.Fatalf("bex-api saw %d requests, log = %q; want one timed-out attempt on the pooled connection and no retry", served, out)
	}
}
