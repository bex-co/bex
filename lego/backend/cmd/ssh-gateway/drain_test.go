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
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/nativessh"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w6/m482: the auxiliary listeners (sandbox exec, agent Git, attach, model and
// egress proxies, and the web shell) stop accepting at SIGTERM and let their
// in-flight streams finish within the same drain bound as native SSH.

// freeAddr returns a loopback address nothing listens on.
func freeAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	return addr
}

// startTestAux starts one auxiliary listener whose requests run under a
// session context the test ends.
func startTestAux(t *testing.T, handler http.Handler) (string, func(context.Context) error, context.Context, context.CancelCauseFunc) {
	t.Helper()
	sessionCtx, end := context.WithCancelCause(context.Background())
	t.Cleanup(func() { end(nil) })
	addr := freeAddr(t)
	drainer := startAuxListener("test", "test listener", addr, handler, sessionCtx, func() {}, sshgateway.NewMetrics(prometheus.NewRegistry()))
	waitDialable(t, addr, true)
	return addr, drainer, sessionCtx, end
}

// waitDialable waits until addr accepts connections (want) or refuses them
// (!want: the listener is closed; one left open but unserved times out).
func waitDialable(t *testing.T, addr string, want bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
		}
		if want && err == nil || !want && errors.Is(err, syscall.ECONNREFUSED) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%s did not become %s within 2s", addr, map[bool]string{true: "dialable", false: "refused"}[want])
}

func asyncDrain(drainer func(context.Context) error, ctx context.Context) chan error {
	result := make(chan error, 1)
	go func() { result <- drainer(ctx) }()
	return result
}

// A streaming response that started before the drain (a model-proxy answer)
// completes; no new connection is accepted meanwhile; the drain returns once
// the response is done.
func TestAuxListenerDrainLetsAnInFlightResponseFinish(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /stream", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "first\n")
		w.(http.Flusher).Flush()
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, "last\n")
	})
	addr, drainer, _, _ := startTestAux(t, mux)

	response, err := http.Get("http://" + addr + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body := bufio.NewReader(response.Body)
	if line, err := body.ReadString('\n'); err != nil || line != "first\n" {
		t.Fatalf("first line = %q, %v", line, err)
	}
	<-started

	drained := asyncDrain(drainer, context.Background())
	waitDialable(t, addr, false)
	select {
	case err := <-drained:
		t.Fatalf("drain returned %v with a response in flight", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	if rest, err := io.ReadAll(body); err != nil || string(rest) != "last\n" {
		t.Fatalf("rest of the response = %q, %v; want it to finish", rest, err)
	}
	select {
	case err := <-drained:
		if err != nil {
			t.Fatalf("drain = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("drain did not return after the response finished")
	}
}

// A hijacked connection (a WebSocket, a CONNECT tunnel) is waited for too:
// http.Server.Shutdown alone would return at once and let the process cut it.
func TestAuxListenerDrainWaitsForAHijackedConnection(t *testing.T) {
	hijacked := make(chan net.Conn, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, buffered, err := http.NewResponseController(w).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		hijacked <- conn
		_, _ = io.Copy(conn, buffered) // echo until the client closes
	})
	addr, drainer, _, _ := startTestAux(t, handler)

	client, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_, _ = io.WriteString(client, "CONNECT example.com:443 HTTP/1.1\r\nHost: example.com:443\r\n\r\n")
	reader := bufio.NewReader(client)
	if status, err := reader.ReadString('\n'); err != nil || status != "HTTP/1.1 200 Connection Established\r\n" {
		t.Fatalf("CONNECT answer = %q, %v", status, err)
	}
	_, _ = reader.ReadString('\n')
	<-hijacked

	drained := asyncDrain(drainer, context.Background())
	waitDialable(t, addr, false)
	_, _ = io.WriteString(client, "still open\n")
	if line, err := reader.ReadString('\n'); err != nil || line != "still open\n" {
		t.Fatalf("tunnel echo during the drain = %q, %v", line, err)
	}
	select {
	case err := <-drained:
		t.Fatalf("drain returned %v with a hijacked connection open", err)
	case <-time.After(50 * time.Millisecond):
	}

	_ = client.Close()
	select {
	case err := <-drained:
		if err != nil {
			t.Fatalf("drain = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("drain did not return after the tunnel closed")
	}
}

// At the bound, drain ends the open streams with ErrGatewayShutdown, and it
// returns even while a hijacked tunnel that ignores its context stays open.
func TestDrainEndsAtTheBoundEvenWithATunnelOpen(t *testing.T) {
	stuck := make(chan struct{})
	t.Cleanup(func() { close(stuck) })
	causes := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /stream", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		causes <- context.Cause(r.Context())
	})
	mux.HandleFunc("GET /tunnel", func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := http.NewResponseController(w).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		<-stuck // ignores every context, as a CONNECT splice does
	})
	addr, drainer, sessionCtx, end := startTestAux(t, mux)

	stream, err := http.Get("http://" + addr + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	tunnel, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer tunnel.Close()
	_, _ = io.WriteString(tunnel, "GET /tunnel HTTP/1.1\r\nHost: test\r\n\r\n")

	start := time.Now()
	done := make(chan struct{})
	go func() {
		drain(sessionCtx, end, 150*time.Millisecond, 150*time.Millisecond, []func(context.Context) error{drainer})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("drain did not return at the bound plus its grace")
	}
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
		t.Fatalf("drain returned after %s, before the bound", elapsed)
	}
	select {
	case cause := <-causes:
		if !errors.Is(cause, sshgateway.ErrGatewayShutdown) {
			t.Fatalf("stream ended with cause %v, want ErrGatewayShutdown", cause)
		}
	default:
		t.Fatal("the open stream was not ended at the bound")
	}
}

// A second signal ends the sessions at once, so the drain stops waiting long
// before its bound, even for a tunnel that ignores its context.
func TestDrainReturnsWhenTheSessionsEndEarly(t *testing.T) {
	stuck := make(chan struct{})
	t.Cleanup(func() { close(stuck) })
	mux := http.NewServeMux()
	mux.HandleFunc("GET /stream", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	mux.HandleFunc("GET /tunnel", func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := http.NewResponseController(w).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		<-stuck
	})
	addr, drainer, sessionCtx, end := startTestAux(t, mux)
	stream, err := http.Get("http://" + addr + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	tunnel, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer tunnel.Close()
	_, _ = io.WriteString(tunnel, "GET /tunnel HTTP/1.1\r\nHost: test\r\n\r\n")

	done := make(chan struct{})
	go func() {
		drain(sessionCtx, end, time.Hour, 200*time.Millisecond, []func(context.Context) error{drainer})
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	end(sshgateway.ErrGatewayShutdown)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("drain kept waiting after the sessions ended")
	}
}

// The first SIGTERM only closes the listeners; the second ends the sessions.
func TestDrainOnSignalClosesListenersFirstAndSessionsOnTheSecond(t *testing.T) {
	signals := make(chan os.Signal, 2)
	listenCtx, stopListening := context.WithCancel(context.Background())
	defer stopListening()
	sessionCtx, endSessions := context.WithCancelCause(context.Background())
	defer endSessions(nil)
	go drainOnSignal(signals, stopListening, endSessions)

	signals <- syscall.SIGTERM
	select {
	case <-listenCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("SIGTERM did not close the listeners")
	}
	time.Sleep(20 * time.Millisecond)
	if sessionCtx.Err() != nil {
		t.Fatal("the first SIGTERM ended the open sessions")
	}

	signals <- syscall.SIGTERM
	select {
	case <-sessionCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("a second SIGTERM did not end the open sessions")
	}
	if cause := context.Cause(sessionCtx); !errors.Is(cause, sshgateway.ErrGatewayShutdown) {
		t.Fatalf("sessions ended with cause %v, want ErrGatewayShutdown", cause)
	}
}

func newSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

// The whole release path, as main runs it: an SSH session open at SIGTERM
// keeps exchanging data while the gateway refuses new connections, and ends
// at the drain bound, after which serve returns and the process may exit.
func TestSIGTERMKeepsAnOpenSSHSessionUntilTheBound(t *testing.T) {
	clientSigner := newSigner(t)
	st := &gatewaytest.FakeStore{Key: store.SSHKey{Subject: "user-1", Fingerprint: ssh.FingerprintSHA256(clientSigner.PublicKey())}}
	gateway := &nativessh.Server{
		Store: st, Apps: &gatewaytest.FakeResolver{}, Executor: &gatewaytest.EchoExecutor{},
		Signer: newSigner(t), SessionTimeout: time.Minute, RevalidateInterval: -1,
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listenCtx, sigterm := context.WithCancel(context.Background())
	defer sigterm()
	sessionCtx, endSessions := context.WithCancelCause(context.Background())
	defer endSessions(nil)
	const bound = 400 * time.Millisecond
	served := make(chan error, 1)
	go func() { served <- serve(listenCtx, sessionCtx, endSessions, gateway, listener, bound, nil) }()

	config := &ssh.ClientConfig{
		User: "srv-abcdeabcdeabcdeabcde", Auth: []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 2 * time.Second,
	}
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()
	if err := session.Shell(); err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewReader(stdout)
	echo := func(line string) error {
		if _, err := io.WriteString(stdin, line+"\n"); err != nil {
			return err
		}
		got, err := lines.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.TrimSpace(got) != line {
			return errors.New("echo " + got)
		}
		return nil
	}
	if err := echo("before SIGTERM"); err != nil {
		t.Fatal(err)
	}

	sigterm()
	waitDialable(t, addr, false)
	if err := echo("after SIGTERM"); err != nil {
		t.Fatalf("SIGTERM cut the open session: %v", err)
	}

	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("serve = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return at the drain bound")
	}
	if err := echo("after the bound"); err == nil {
		t.Fatal("the session outlived the drain bound")
	}
	if ended := st.EndedSessions(); len(ended) != 1 || !strings.HasSuffix(ended[0], ":failed") {
		t.Fatalf("session audit result = %v, want one entry ending :failed", ended)
	}
}
