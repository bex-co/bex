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

// lifetimes are the three main runs drainOnSignal over.
type lifetimes struct {
	signals                 chan os.Signal
	ready, listen, sessions context.Context
	endSessions             context.CancelCauseFunc
}

func signalLifetimes(t *testing.T, window time.Duration) lifetimes {
	t.Helper()
	l := lifetimes{signals: make(chan os.Signal, 2)}
	var stopListening, stopReady context.CancelFunc
	l.listen, stopListening = context.WithCancel(context.Background())
	t.Cleanup(stopListening)
	l.ready, stopReady = context.WithCancel(l.listen)
	t.Cleanup(stopReady)
	l.sessions, l.endSessions = context.WithCancelCause(context.Background())
	t.Cleanup(func() { l.endSessions(nil) })
	go drainOnSignal(l.signals, window, stopReady, stopListening, l.endSessions)
	return l
}

func waitDone(t *testing.T, ctx context.Context, what string) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal(what)
	}
}

// The first SIGTERM turns readiness off at once and closes the listeners only
// after the window (w6/152); the second ends the sessions.
func TestDrainOnSignalClosesListenersAfterTheWindowAndSessionsOnTheSecond(t *testing.T) {
	const window = 300 * time.Millisecond
	l := signalLifetimes(t, window)

	l.signals <- syscall.SIGTERM
	sent := time.Now()
	waitDone(t, l.ready, "SIGTERM did not turn readiness off")
	if l.listen.Err() != nil {
		t.Fatal("SIGTERM closed the listeners at once, before the window")
	}
	waitDone(t, l.listen, "the listeners stayed open past the window")
	if elapsed := time.Since(sent); elapsed < window {
		t.Fatalf("the listeners closed %s after SIGTERM, inside the %s window", elapsed, window)
	}
	time.Sleep(20 * time.Millisecond)
	if l.sessions.Err() != nil {
		t.Fatal("the first SIGTERM ended the open sessions")
	}

	l.signals <- syscall.SIGTERM
	waitDone(t, l.sessions, "a second SIGTERM did not end the open sessions")
	if cause := context.Cause(l.sessions); !errors.Is(cause, sshgateway.ErrGatewayShutdown) {
		t.Fatalf("sessions ended with cause %v, want ErrGatewayShutdown", cause)
	}
}

// A second signal inside the window does not wait for it: the listeners close
// and the open sessions end at once.
func TestASecondSignalInsideTheWindowEndsEverythingAtOnce(t *testing.T) {
	l := signalLifetimes(t, time.Hour)
	l.signals <- syscall.SIGTERM
	waitDone(t, l.ready, "SIGTERM did not turn readiness off")
	l.signals <- syscall.SIGINT
	waitDone(t, l.listen, "a second signal inside the window left the listeners open")
	waitDone(t, l.sessions, "a second signal inside the window left the sessions open")
	if cause := context.Cause(l.sessions); !errors.Is(cause, sshgateway.ErrGatewayShutdown) {
		t.Fatalf("sessions ended with cause %v, want ErrGatewayShutdown", cause)
	}
}

// w6/152, the whole release path: Traefik keeps routing new connections to a
// terminating pod for a few seconds after its SIGTERM. A fresh SSH connection
// made in the window after SIGTERM completes its handshake and runs a session,
// as does a request to an auxiliary listener, while readiness is already off;
// once the window has passed both listeners refuse, and the session opened in
// it keeps running under the drain.
func TestANewConnectionInTheWindowAfterSIGTERMIsServedAndRefusedAfterIt(t *testing.T) {
	gateway, listener, dial := newTestGateway(t)
	addr := listener.Addr().String()
	const window = time.Second
	l := signalLifetimes(t, window)
	auxAddr, auxDrainer, _, _ := startTestAux(t, http.NewServeMux())
	served := make(chan error, 1)
	go func() {
		served <- serve(l.listen, l.sessions, l.endSessions, gateway, listener, time.Minute, []func(context.Context) error{auxDrainer})
	}()

	l.signals <- syscall.SIGTERM
	sent := time.Now()
	waitDone(t, l.ready, "SIGTERM did not turn readiness off")
	echo, err := dial()
	if err != nil {
		t.Fatalf("a new SSH connection in the window after SIGTERM failed: %v", err)
	}
	if err := echo("in the window"); err != nil {
		t.Fatalf("the session opened in the window: %v", err)
	}
	response, err := http.Get("http://" + auxAddr + "/healthz")
	if err != nil {
		t.Fatalf("a new request to an auxiliary listener in the window failed: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("auxiliary listener in the window = %d, want 200", response.StatusCode)
	}
	waitDone(t, l.listen, "the listeners stayed open past the window")
	if elapsed := time.Since(sent); elapsed < window {
		t.Fatalf("the listeners closed %s after SIGTERM, inside the %s window", elapsed, window)
	}
	waitDialable(t, addr, false)
	waitDialable(t, auxAddr, false)
	if err := echo("after the window"); err != nil {
		t.Fatalf("closing the listeners cut the session opened in the window: %v", err)
	}

	l.signals <- syscall.SIGTERM
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("serve = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after the second signal")
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

// newTestGateway builds a native SSH gateway over an echo executor and its
// loopback listener. dial opens a fresh SSH connection and shell session to it
// and returns a function that sends one line and checks it comes back.
func newTestGateway(t *testing.T) (*nativessh.Server, net.Listener, func() (func(string) error, error)) {
	t.Helper()
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
	config := &ssh.ClientConfig{
		User: "srv-abcdeabcdeabcdeabcde", Auth: []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 2 * time.Second,
	}
	dial := func() (func(string) error, error) {
		client, err := ssh.Dial("tcp", listener.Addr().String(), config)
		if err != nil {
			return nil, err
		}
		t.Cleanup(func() { _ = client.Close() })
		session, err := client.NewSession()
		if err != nil {
			return nil, err
		}
		stdin, _ := session.StdinPipe()
		stdout, _ := session.StdoutPipe()
		if err := session.Shell(); err != nil {
			return nil, err
		}
		lines := bufio.NewReader(stdout)
		return func(line string) error {
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
		}, nil
	}
	return gateway, listener, dial
}

// The whole release path, as main runs it: an SSH session open at SIGTERM
// keeps exchanging data while the gateway refuses new connections, and ends
// at the drain bound, after which serve returns and the process may exit.
func TestSIGTERMKeepsAnOpenSSHSessionUntilTheBound(t *testing.T) {
	gateway, listener, dial := newTestGateway(t)
	addr := listener.Addr().String()
	listenCtx, sigterm := context.WithCancel(context.Background())
	defer sigterm()
	sessionCtx, endSessions := context.WithCancelCause(context.Background())
	defer endSessions(nil)
	const bound = 400 * time.Millisecond
	served := make(chan error, 1)
	go func() { served <- serve(listenCtx, sessionCtx, endSessions, gateway, listener, bound, nil) }()

	echo, err := dial()
	if err != nil {
		t.Fatal(err)
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
	if ended := gateway.Store.(*gatewaytest.FakeStore).EndedSessions(); len(ended) != 1 || !strings.HasSuffix(ended[0], ":failed") {
		t.Fatalf("session audit result = %v, want one entry ending :failed", ended)
	}
}
