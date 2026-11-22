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

package kvinternalauth

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testPassword = "test-$\r\n-pass"

func authFrame() string {
	return fmt.Sprintf("*3\r\n$4\r\nAUTH\r\n$7\r\ndefault\r\n$%d\r\n%s\r\n", len(testPassword), testPassword)
}

func scriptedUpstream(t *testing.T, script func(net.Conn) error) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		done <- script(conn)
	}()
	t.Cleanup(func() { _ = listener.Close() })
	return listener.Addr().String(), func() {
		t.Helper()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(4 * time.Second):
			t.Fatal("upstream script did not finish")
		}
	}
}

func readExact(conn net.Conn, want string) error {
	data := make([]byte, len(want))
	if _, err := io.ReadFull(conn, data); err != nil {
		return err
	}
	if string(data) != want {
		return fmt.Errorf("unexpected wire bytes: got %q want %q", data, want)
	}
	return nil
}

func startProxy(t *testing.T, config Config) (string, context.CancelFunc) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, listener, config) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("proxy shutdown did not finish")
		}
	})
	return listener.Addr().String(), cancel
}

func awaitSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream connection did not reach expected state")
	}
}

func dialProxy(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return conn
}

func TestAuthenticatesBeforeForwardingAndPreservesStream(t *testing.T) {
	// Includes a credentialed client's own AUTH, a RESP3 HELLO, a binary SET,
	// pipelined commands, and pubsub-shaped output. Only the injected AUTH reply
	// may be consumed by the proxy; all other bytes must survive unchanged.
	payload := authFrame() + "*2\r\n$5\r\nHELLO\r\n$1\r\n3\r\n*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$3\r\na\x00b\r\n*1\r\n$4\r\nPING\r\n"
	response := "+OK\r\n%1\r\n+proto\r\n:3\r\n+OK\r\n+PONG\r\n>3\r\n+message\r\n+channel\r\n$3\r\na\x00b\r\n"
	upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
		if err := readExact(conn, authFrame()); err != nil {
			return err
		}
		// The client already sent its pipeline, but authentication has not yet
		// succeeded. A read here must time out rather than reveal tenant commands.
		if err := conn.SetReadDeadline(time.Now().Add(40 * time.Millisecond)); err != nil {
			return err
		}
		var b [1]byte
		if n, err := conn.Read(b[:]); n != 0 || err == nil {
			return fmt.Errorf("client bytes crossed before AUTH success: n=%d err=%v", n, err)
		}
		if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			return err
		}
		if _, err := io.WriteString(conn, "+OK\r\n"); err != nil {
			return err
		}
		if err := readExact(conn, payload); err != nil {
			return err
		}
		_, err := io.WriteString(conn, response)
		return err
	})
	addr, _ := startProxy(t, Config{Upstream: upstream, Password: testPassword})
	conn := dialProxy(t, addr)
	if _, err := io.WriteString(conn, payload); err != nil {
		t.Fatal(err)
	}
	if err := readExact(conn, response); err != nil {
		t.Fatal(err)
	}
	finish()
}

func TestAuthenticationFailureForwardsNoClientBytes(t *testing.T) {
	upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
		if err := readExact(conn, authFrame()); err != nil {
			return err
		}
		if _, err := io.WriteString(conn, "-WRONGPASS invalid credentials\r\n"); err != nil {
			return err
		}
		var b [1]byte
		if n, err := conn.Read(b[:]); n != 0 || err == nil {
			return fmt.Errorf("client bytes forwarded after refusal: n=%d err=%v", n, err)
		}
		return nil
	})
	addr, _ := startProxy(t, Config{Upstream: upstream, Password: testPassword})
	conn := dialProxy(t, addr)
	if _, err := io.WriteString(conn, "PING\r\n"); err != nil {
		t.Fatal(err)
	}
	var b [64]byte
	if n, err := conn.Read(b[:]); n != 0 || err == nil {
		t.Fatalf("injected auth reply leaked: n=%d err=%v", n, err)
	}
	finish()
}

func TestAuthenticationDeadlineClosesWaitingClient(t *testing.T) {
	upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
		if err := readExact(conn, authFrame()); err != nil {
			return err
		}
		var b [1]byte
		if n, err := conn.Read(b[:]); n != 0 || err == nil {
			return fmt.Errorf("timeout left upstream open: n=%d err=%v", n, err)
		}
		return nil
	})
	addr, _ := startProxy(t, Config{Upstream: upstream, Password: testPassword, HandshakeTimeout: 200 * time.Millisecond})
	conn := dialProxy(t, addr)
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var b [1]byte
	if n, err := conn.Read(b[:]); n != 0 || err == nil {
		t.Fatalf("waiting client stayed open: n=%d err=%v", n, err)
	} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		t.Fatal("client timeout, want proxy to close after AUTH deadline")
	}
	finish()
}

func TestClientHalfCloseStillReceivesPipelineReplies(t *testing.T) {
	const payload = "PING\r\nPING\r\n"
	upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
		if err := readExact(conn, authFrame()); err != nil {
			return err
		}
		if _, err := io.WriteString(conn, "+OK\r\n"); err != nil {
			return err
		}
		data, err := io.ReadAll(conn)
		if err != nil {
			return err
		}
		if string(data) != payload {
			return fmt.Errorf("pipeline=%q", data)
		}
		_, err = io.WriteString(conn, "+PONG\r\n+PONG\r\n")
		return err
	})
	addr, _ := startProxy(t, Config{Upstream: upstream, Password: testPassword})
	conn := dialProxy(t, addr)
	if _, err := io.WriteString(conn, payload); err != nil {
		t.Fatal(err)
	}
	if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	if err := readExact(conn, "+PONG\r\n+PONG\r\n"); err != nil {
		t.Fatal(err)
	}
	finish()
}

func TestCancellationClosesActiveAndAuthenticatingStreams(t *testing.T) {
	for _, authenticated := range []bool{false, true} {
		t.Run(fmt.Sprint(authenticated), func(t *testing.T) {
			connected := make(chan struct{})
			upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
				if err := readExact(conn, authFrame()); err != nil {
					return err
				}
				if authenticated {
					if _, err := io.WriteString(conn, "+OK\r\n"); err != nil {
						return err
					}
				}
				close(connected)
				var b [1]byte
				if n, err := conn.Read(b[:]); n != 0 || err == nil {
					return fmt.Errorf("cancel left upstream open: n=%d err=%v", n, err)
				}
				return nil
			})
			addr, cancel := startProxy(t, Config{Upstream: upstream, Password: testPassword})
			conn := dialProxy(t, addr)
			awaitSignal(t, connected)
			cancel()
			var b [1]byte
			if n, err := conn.Read(b[:]); n != 0 || err == nil {
				t.Fatalf("cancel left client open: n=%d err=%v", n, err)
			}
			finish()
		})
	}
}

func TestTLSPassthroughDoesNotInjectOrConsumeBytes(t *testing.T) {
	// A representative binary ClientHello and encrypted response; the proxy
	// should never parse TLS or place an AUTH before the ClientHello.
	hello := []byte{0x16, 3, 1, 0, 6, 1, 0, 0, 2, 0, 0}
	response := []byte{0x16, 3, 3, 0, 4, 2, 0, 0, 0}
	upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
		if err := readExact(conn, string(hello)); err != nil {
			return err
		}
		_, err := conn.Write(response)
		return err
	})
	addr, _ := startProxy(t, Config{Upstream: upstream, Mode: TLSPassthrough})
	conn := dialProxy(t, addr)
	if _, err := conn.Write(hello); err != nil {
		t.Fatal(err)
	}
	if err := readExact(conn, string(response)); err != nil {
		t.Fatal(err)
	}
	finish()
}

// A client past the budget reads Valkey's own maxclients refusal, and no
// upstream is dialed for it (w6/m456).
func TestClientPastBudgetReadsValkeysRefusal(t *testing.T) {
	authenticated := make(chan struct{})
	upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
		if err := readExact(conn, authFrame()); err != nil {
			return err
		}
		if _, err := io.WriteString(conn, "+OK\r\n"); err != nil {
			return err
		}
		close(authenticated)
		var b [1]byte
		_, err := conn.Read(b[:])
		if err != io.EOF {
			return fmt.Errorf("unexpected active connection read: %v", err)
		}
		return nil
	})
	addr, _ := startProxy(t, Config{Upstream: upstream, Password: testPassword, Budget: NewBudget(1)})
	first := dialProxy(t, addr)
	awaitSignal(t, authenticated)
	second := dialProxy(t, addr)
	if got, err := io.ReadAll(second); err != nil || string(got) != maxClientsReply {
		t.Fatalf("client past the budget read %q (%v), want Valkey's refusal", got, err)
	}
	_ = first.Close()
	finish()
}

// The plain and TLS listeners in front of one store draw on one budget, and a
// TLS client past it reads the same refusal inside TLS, under the store's own
// certificate.
func TestBudgetIsSharedAndRefusesInsideTLS(t *testing.T) {
	certFile, keyFile, roots := writeTestCertificate(t)
	authenticated := make(chan struct{})
	upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
		if err := readExact(conn, authFrame()); err != nil {
			return err
		}
		if _, err := io.WriteString(conn, "+OK\r\n"); err != nil {
			return err
		}
		close(authenticated)
		var b [1]byte
		_, _ = conn.Read(b[:])
		return nil
	})
	budget := NewBudget(1)
	plain, _ := startProxy(t, Config{Upstream: upstream, Password: testPassword, Budget: budget})
	tlsAddr, _ := startProxy(t, Config{Upstream: "127.0.0.1:1", Mode: TLSPassthrough, Budget: budget, TLSCertFile: certFile, TLSKeyFile: keyFile})
	first := dialProxy(t, plain)
	awaitSignal(t, authenticated)

	client := tls.Client(dialProxy(t, tlsAddr), &tls.Config{ServerName: "kv.example.test", RootCAs: roots, MinVersion: tls.VersionTLS12})
	if err := client.Handshake(); err != nil {
		t.Fatalf("the refused TLS client could not verify the store's certificate: %v", err)
	}
	if got, err := io.ReadAll(client); err != nil || string(got) != maxClientsReply {
		t.Fatalf("TLS client past the budget read %q (%v), want Valkey's refusal", got, err)
	}
	_ = first.Close()
	finish()
}

// Without the certificate a refused TLS client is closed, as Valkey closes one
// past its maxclients before the handshake.
func TestTLSRefusalWithoutCertificateCloses(t *testing.T) {
	budget := NewBudget(1)
	if !budget.admit(stream(0)) {
		t.Fatal("empty budget refused")
	}
	addr, _ := startProxy(t, Config{Upstream: "127.0.0.1:1", Mode: TLSPassthrough, Budget: budget})
	conn := dialProxy(t, addr)
	if got, err := io.ReadAll(conn); err != nil || len(got) != 0 {
		t.Fatalf("read %q (%v), want a bare close", got, err)
	}
}

// Valkey writes its refusal on accept, before reading the injected AUTH; the
// client reads that refusal rather than a dropped connection, and nothing of
// the injected exchange.
func TestUpstreamRefusalReachesClient(t *testing.T) {
	upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
		_, err := io.WriteString(conn, maxClientsReply)
		return err
	})
	addr, _ := startProxy(t, Config{Upstream: upstream, Password: testPassword})
	conn := dialProxy(t, addr)
	if got, err := io.ReadAll(conn); err != nil || string(got) != maxClientsReply {
		t.Fatalf("client read %q (%v), want the upstream's refusal", got, err)
	}
	finish()
}

// An upstream error other than the refusal still fails closed without
// reaching the client (the injected AUTH's reply is never exposed).
func TestOtherUpstreamErrorsStayHidden(t *testing.T) {
	upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
		if err := readExact(conn, authFrame()); err != nil {
			return err
		}
		_, err := io.WriteString(conn, "-ERR max number of things\r\n")
		return err
	})
	addr, _ := startProxy(t, Config{Upstream: upstream, Password: testPassword})
	conn := dialProxy(t, addr)
	if got, _ := io.ReadAll(conn); len(got) != 0 {
		t.Fatalf("client read the upstream's AUTH reply %q", got)
	}
	finish()
}

// A store whose plan allows more than the old 1,024 cap gets all of them: the
// budget, not a per-listener default, bounds the proxy.
func TestBudgetAdmitsPastTheOld1024Cap(t *testing.T) {
	const n = 1030
	upstream := servingUpstream(t)
	addr, _ := startProxy(t, Config{Upstream: upstream, Password: testPassword, Budget: NewBudget(n)})
	conns := make([]net.Conn, 0, n)
	for i := range n {
		conn := dialProxy(t, addr)
		if _, err := io.WriteString(conn, "PING\r\n"); err != nil {
			t.Fatal(err)
		}
		if err := readExact(conn, "+PONG\r\n"); err != nil {
			t.Fatalf("connection %d of %d: %v", i+1, n, err)
		}
		conns = append(conns, conn)
	}
	over := dialProxy(t, addr)
	if got, err := io.ReadAll(over); err != nil || string(got) != maxClientsReply {
		t.Fatalf("connection %d read %q (%v), want Valkey's refusal", n+1, got, err)
	}
	_ = conns[0].Close()
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		again := dialProxy(t, addr)
		_, _ = io.WriteString(again, "PING\r\n")
		if readExact(again, "+PONG\r\n") == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a closed connection's slot never returned to the budget")
		}
	}
}

func TestNewBudgetHoldsExactlyItsSize(t *testing.T) {
	b := NewBudget(5000)
	for i := range 5000 {
		if !b.admit(stream(i)) {
			t.Fatalf("refused connection %d of 5000", i+1)
		}
	}
	if b.admit(stream(5000)) {
		t.Fatal("admitted connection 5001")
	}
	b.release(stream(0))
	if !b.admit(stream(5000)) {
		t.Fatal("a released slot was not reusable")
	}
	if got := NewBudget(0).limit; got != 1024 {
		t.Fatalf("default budget %d, want 1024", got)
	}
}

func (b *Budget) held() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.streams)
}

// stream is a distinct, closable stand-in for an admitted client.
type stream int

func (stream) Read([]byte) (int, error)         { return 0, io.EOF }
func (stream) Write(b []byte) (int, error)      { return len(b), nil }
func (stream) Close() error                     { return nil }
func (stream) LocalAddr() net.Addr              { return nil }
func (stream) RemoteAddr() net.Addr             { return nil }
func (stream) SetDeadline(time.Time) error      { return nil }
func (stream) SetReadDeadline(time.Time) error  { return nil }
func (stream) SetWriteDeadline(time.Time) error { return nil }

// servingUpstream answers the injected AUTH and then PING on every
// connection, as an authenticated Valkey does.
func servingUpstream(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				if readExact(conn, authFrame()) != nil {
					return
				}
				if _, err := io.WriteString(conn, "+OK\r\n"); err != nil {
					return
				}
				for readExact(conn, "PING\r\n") == nil {
					if _, err := io.WriteString(conn, "+PONG\r\n"); err != nil {
						return
					}
				}
			}()
		}
	}()
	return listener.Addr().String()
}

// writeTestCertificate writes a self-signed certificate for kv.example.test
// and returns its files and a pool that trusts it.
func writeTestCertificate(t *testing.T) (certFile, keyFile string, roots *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "kv.example.test"}, DNSNames: []string{"kv.example.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile = filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots = x509.NewCertPool()
	roots.AddCert(cert)
	return certFile, keyFile, roots
}

// The check is an authenticated PING in every mode: a proxy whose clients
// authenticate themselves still reports a loading server unready (w6/m463).
func TestCheckRequiresAuthenticatedPong(t *testing.T) {
	for _, mode := range []Mode{InjectAuth, PlainPassthrough} {
		for _, response := range []string{"+PONG\r\n", "-LOADING dataset\r\n"} {
			t.Run(fmt.Sprintf("%d/%s", mode, strings.TrimSpace(response)), func(t *testing.T) {
				upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
					if err := readExact(conn, authFrame()); err != nil {
						return err
					}
					if _, err := io.WriteString(conn, "+OK\r\n"); err != nil {
						return err
					}
					if err := readExact(conn, "*1\r\n$4\r\nPING\r\n"); err != nil {
						return err
					}
					_, err := io.WriteString(conn, response)
					return err
				})
				err := Check(t.Context(), Config{Upstream: upstream, Password: testPassword, Mode: mode})
				if (err == nil) != (response == "+PONG\r\n") {
					t.Fatalf("response=%q err=%v", response, err)
				}
				if err != nil && bytes.Contains([]byte(err.Error()), []byte(testPassword)) {
					t.Fatal("check error exposed credentials")
				}
				finish()
			})
		}
	}
}

// A store with internal authentication (w6/m463): the plain listener writes
// nothing of its own upstream, even holding the password for its check, so
// the client's own AUTH decides its access, and Valkey's answers, including
// its NOAUTH and its own maxclients refusal, reach the client unchanged.
func TestPlainPassthroughForwardsTheClientsOwnAuth(t *testing.T) {
	for _, c := range []struct {
		name, client, upstream string
	}{
		{"unauthenticated", "PING\r\n", "-NOAUTH Authentication required.\r\n"},
		{"authenticated", authFrame() + "PING\r\n", "+OK\r\n+PONG\r\n"},
		{"upstream full", "PING\r\n", maxClientsReply},
	} {
		t.Run(c.name, func(t *testing.T) {
			upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
				if c.upstream == maxClientsReply {
					// Valkey refuses on accept, before it reads anything.
					_, err := io.WriteString(conn, maxClientsReply)
					return err
				}
				if err := readExact(conn, c.client); err != nil {
					return err
				}
				_, err := io.WriteString(conn, c.upstream)
				return err
			})
			addr, _ := startProxy(t, Config{Upstream: upstream, Password: testPassword, Mode: PlainPassthrough})
			conn := dialProxy(t, addr)
			if _, err := io.WriteString(conn, c.client); err != nil {
				t.Fatal(err)
			}
			if err := readExact(conn, c.upstream); err != nil {
				t.Fatal(err)
			}
			finish()
		})
	}
}

// The plain passthrough and TLS listeners in front of a store with internal
// authentication draw on one budget, and the client past it reads Valkey's
// refusal in plain text on the plain listener and inside TLS on the TLS one.
func TestPlainPassthroughSharesTheBudgetAndRefusesInPlainText(t *testing.T) {
	certFile, keyFile, roots := writeTestCertificate(t)
	connected := make(chan struct{})
	upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
		if err := readExact(conn, "PING\r\n"); err != nil {
			return err
		}
		close(connected)
		var b [1]byte
		_, _ = conn.Read(b[:])
		return nil
	})
	budget := NewBudget(1)
	plain, _ := startProxy(t, Config{Upstream: upstream, Password: testPassword, Mode: PlainPassthrough, Budget: budget})
	tlsAddr, _ := startProxy(t, Config{Upstream: "127.0.0.1:1", Mode: TLSPassthrough, Budget: budget, TLSCertFile: certFile, TLSKeyFile: keyFile})
	first := dialProxy(t, plain)
	if _, err := io.WriteString(first, "PING\r\n"); err != nil {
		t.Fatal(err)
	}
	awaitSignal(t, connected)

	if got, err := io.ReadAll(dialProxy(t, plain)); err != nil || string(got) != maxClientsReply {
		t.Fatalf("plain client past the budget read %q (%v), want Valkey's refusal in plain text", got, err)
	}
	client := tls.Client(dialProxy(t, tlsAddr), &tls.Config{ServerName: "kv.example.test", RootCAs: roots, MinVersion: tls.VersionTLS12})
	if err := client.Handshake(); err != nil {
		t.Fatalf("the refused TLS client could not verify the store's certificate: %v", err)
	}
	if got, err := io.ReadAll(client); err != nil || string(got) != maxClientsReply {
		t.Fatalf("TLS client past the budget read %q (%v), want Valkey's refusal", got, err)
	}
	_ = first.Close()
	finish()
}

func TestConfigRefusesAnUnknownMode(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := Serve(t.Context(), listener, Config{Upstream: "127.0.0.1:1", Password: testPassword, Mode: AllowListRefusal + 1}); err == nil {
		t.Fatal("served an unknown mode")
	}
}

// drainingProxy serves config with a drain the test starts, which returns once
// the listener is closed, and reports when Serve returns.
func drainingProxy(t *testing.T, config Config) (addr string, drain func(), served <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	watched := &closeWatch{Listener: listener, closed: make(chan struct{})}
	start := make(chan struct{})
	config.Drain = start
	done := make(chan error, 1)
	go func() { done <- Serve(t.Context(), watched, config) }()
	return listener.Addr().String(), sync.OnceFunc(func() {
		close(start)
		awaitSignal(t, watched.closed)
	}), done
}

// closeWatch reports its listener's first Close.
type closeWatch struct {
	net.Listener
	once   sync.Once
	closed chan struct{}
}

// Close signals only once the listener is closed: signalled first, a dial
// racing the close could still be accepted (red main, 7fd0b5a50).
func (w *closeWatch) Close() error {
	err := w.Listener.Close()
	w.once.Do(func() { close(w.closed) })
	return err
}

// refusesDial fails the test if addr still accepts a connection.
func refusesDial(t *testing.T, addr string) {
	t.Helper()
	if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		_ = conn.Close()
		t.Fatal("the draining listener still accepts")
	}
}

// A command in flight when the drain begins gets its reply, a new dial is not
// accepted, and Serve returns as soon as the stream ends, long before the
// deadline, with every budget slot back (w6/m512).
func TestDrainLetsACommandInFlightFinish(t *testing.T) {
	received, reply := make(chan struct{}), make(chan struct{})
	upstream, finish := scriptedUpstream(t, func(conn net.Conn) error {
		if err := readExact(conn, authFrame()); err != nil {
			return err
		}
		if _, err := io.WriteString(conn, "+OK\r\n"); err != nil {
			return err
		}
		if err := readExact(conn, "BLPOP k 0\r\n"); err != nil {
			return err
		}
		close(received)
		<-reply
		_, err := io.WriteString(conn, "*2\r\n$1\r\nk\r\n$1\r\nv\r\n")
		return err
	})
	budget := NewBudget(4)
	addr, drain, served := drainingProxy(t, Config{Upstream: upstream, Password: testPassword, Budget: budget, DrainTimeout: time.Minute})
	conn := dialProxy(t, addr)
	if _, err := io.WriteString(conn, "BLPOP k 0\r\n"); err != nil {
		t.Fatal(err)
	}
	awaitSignal(t, received)
	drain()
	refusesDial(t, addr)
	close(reply)
	if err := readExact(conn, "*2\r\n$1\r\nk\r\n$1\r\nv\r\n"); err != nil {
		t.Fatalf("the command in flight lost its reply: %v", err)
	}
	finish()
	_ = conn.Close()
	select {
	case err := <-served:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return when its last stream ended")
	}
	if n := budget.held(); n != 0 {
		t.Fatalf("%d budget slots held after the drain", n)
	}
}

// An idle client keeps its stream until the deadline, and is closed then.
func TestDrainClosesAnIdleClientAtTheDeadline(t *testing.T) {
	const timeout = 400 * time.Millisecond
	upstream := servingUpstream(t)
	budget := NewBudget(4)
	addr, drain, served := drainingProxy(t, Config{Upstream: upstream, Password: testPassword, Budget: budget, DrainTimeout: timeout})
	conn := dialProxy(t, addr)
	if _, err := io.WriteString(conn, "PING\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := readExact(conn, "+PONG\r\n"); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	drain()
	refusesDial(t, addr)
	if _, err := io.WriteString(conn, "PING\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := readExact(conn, "+PONG\r\n"); err != nil {
		t.Fatalf("the idle client lost its stream before the deadline: %v", err)
	}
	var b [1]byte
	if n, err := conn.Read(b[:]); n != 0 || err == nil {
		t.Fatalf("the idle client was not closed: n=%d err=%v", n, err)
	}
	if held := time.Since(began); held < timeout {
		t.Fatalf("the idle client was closed after %v, before the %v deadline", held, timeout)
	}
	select {
	case err := <-served:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return at the deadline")
	}
	if n := budget.held(); n != 0 {
		t.Fatalf("%d budget slots held after the drain", n)
	}
}

// Cancellation is still the hard stop, before a drain or during one.
func TestCancellationClosesAtOnceWhetherOrNotDraining(t *testing.T) {
	for _, drainFirst := range []bool{false, true} {
		t.Run(fmt.Sprint(drainFirst), func(t *testing.T) {
			upstream := servingUpstream(t)
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			drain := make(chan struct{})
			served := make(chan error, 1)
			go func() {
				served <- Serve(ctx, listener, Config{Upstream: upstream, Password: testPassword, Drain: drain, DrainTimeout: time.Minute})
			}()
			conn := dialProxy(t, listener.Addr().String())
			if _, err := io.WriteString(conn, "PING\r\n"); err != nil {
				t.Fatal(err)
			}
			if err := readExact(conn, "+PONG\r\n"); err != nil {
				t.Fatal(err)
			}
			if drainFirst {
				close(drain)
				for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(5 * time.Millisecond) {
					probe, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
					if err != nil {
						break
					}
					_ = probe.Close()
					if time.Now().After(deadline) {
						t.Fatal("the drain never closed the listener")
					}
				}
			}
			cancel()
			var b [1]byte
			if n, err := conn.Read(b[:]); n != 0 || err == nil {
				t.Fatalf("cancel left the client open: n=%d err=%v", n, err)
			}
			select {
			case err := <-served:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Serve did not return on cancellation")
			}
		})
	}
}

// countingUpstream is a Valkey stand-in for KeepReserve: it answers the
// injected AUTH, PING and INFO clients, whose connected_clients is the
// connections it holds open, as Valkey's is.
func countingUpstream(t *testing.T) (addr string, open func() int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	var mu sync.Mutex
	count := 0
	open = func() int {
		mu.Lock()
		defer mu.Unlock()
		return count
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			count++
			mu.Unlock()
			go func() {
				defer func() {
					_ = conn.Close()
					mu.Lock()
					count--
					mu.Unlock()
				}()
				if readExact(conn, authFrame()) != nil {
					return
				}
				if _, err := io.WriteString(conn, "+OK\r\n"); err != nil {
					return
				}
				buf := make([]byte, 64)
				for {
					n, err := conn.Read(buf)
					if err != nil {
						return
					}
					reply := "+PONG\r\n"
					if strings.Contains(string(buf[:n]), "INFO") {
						info := fmt.Sprintf("# Clients\r\nconnected_clients:%d\r\nmaxclients:%d\r\n", open(), 1000)
						reply = fmt.Sprintf("$%d\r\n%s\r\n", len(info), info)
					}
					if _, err := io.WriteString(conn, reply); err != nil {
						return
					}
				}
			}()
		}
	}()
	return listener.Addr().String(), open
}

// While a proxy drains, the store's clients besides KeepReserve's own stay
// within the plan's limit: as tenants dial through the new pod, the old pod
// closes as many of its draining streams, so they never take the reserve.
func TestKeepReserveGivesTheNewPodsClientsTheDrainingStreams(t *testing.T) {
	const limit = 5
	upstream, open := countingUpstream(t)
	budget := NewBudget(limit)
	addr, drain, _ := drainingProxy(t, Config{Upstream: upstream, Password: testPassword, Budget: budget, DrainTimeout: time.Minute})
	old := make([]net.Conn, limit)
	for i := range old {
		old[i] = dialProxy(t, addr)
		if _, err := io.WriteString(old[i], "PING\r\n"); err != nil {
			t.Fatal(err)
		}
		if err := readExact(old[i], "+PONG\r\n"); err != nil {
			t.Fatal(err)
		}
	}
	drain()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go KeepReserve(ctx, Config{Upstream: upstream, Password: testPassword}, budget)

	// The new pod's tenants: three more upstream connections.
	newPod := startServing(t, upstream, 3)
	settled := func() bool { return open()-1 <= limit && budget.held() == limit-3 }
	for deadline := time.Now().Add(3 * time.Second); !settled(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the store holds %d clients besides KeepReserve's, past the limit of %d; the old pod holds %d", open()-1, limit, budget.held())
		}
	}
	for _, conn := range newPod {
		if _, err := io.WriteString(conn, "PING\r\n"); err != nil {
			t.Fatal(err)
		}
		if err := readExact(conn, "+PONG\r\n"); err != nil {
			t.Fatalf("a new pod's client was closed: %v", err)
		}
	}
	closed := 0
	for _, conn := range old {
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		var b [1]byte
		if _, err := conn.Read(b[:]); err == io.EOF {
			closed++
		}
	}
	if closed != 3 {
		t.Fatalf("the old pod closed %d of its streams, want the 3 the new pod's clients took", closed)
	}
}

// startServing opens n authenticated upstream connections, as a new proxy
// pod's tenants do.
func startServing(t *testing.T, upstream string, n int) []net.Conn {
	t.Helper()
	conns := make([]net.Conn, n)
	for i := range conns {
		conn, err := net.DialTimeout("tcp", upstream, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		if _, err := io.WriteString(conn, authFrame()); err != nil {
			t.Fatal(err)
		}
		if err := readExact(conn, "+OK\r\n"); err != nil {
			t.Fatal(err)
		}
		conns[i] = conn
	}
	return conns
}

// Below the limit KeepReserve closes nothing, however long the drain.
func TestKeepReserveClosesNothingBelowTheLimit(t *testing.T) {
	const limit = 5
	upstream, _ := countingUpstream(t)
	budget := NewBudget(limit)
	addr, drain, _ := drainingProxy(t, Config{Upstream: upstream, Password: testPassword, Budget: budget, DrainTimeout: time.Minute})
	held := dialProxy(t, addr)
	if _, err := io.WriteString(held, "PING\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := readExact(held, "+PONG\r\n"); err != nil {
		t.Fatal(err)
	}
	drain()
	ctx, cancel := context.WithTimeout(t.Context(), 10*reserveInterval)
	defer cancel()
	startServing(t, upstream, limit-1)
	KeepReserve(ctx, Config{Upstream: upstream, Password: testPassword}, budget)
	if _, err := io.WriteString(held, "PING\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := readExact(held, "+PONG\r\n"); err != nil {
		t.Fatalf("KeepReserve closed a stream at the limit: %v", err)
	}
}

// An upstream already refusing past its maxclients means the reserve is
// taken: KeepReserve closes a draining stream for each refusal.
func TestKeepReserveShedsWhenTheUpstreamIsFull(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_, _ = io.WriteString(conn, maxClientsReply)
			_ = conn.Close()
		}
	}()
	budget := NewBudget(2)
	for i := range 2 {
		budget.admit(stream(i))
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	go KeepReserve(ctx, Config{Upstream: listener.Addr().String(), Password: testPassword}, budget)
	for budget.held() > 0 {
		if ctx.Err() != nil {
			t.Fatalf("a full upstream left %d draining streams open", budget.held())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestConnectedClientsRefusesAnUnexpectedReply(t *testing.T) {
	for _, reply := range []string{
		"-NOPERM this user has no permissions to run the 'info' command\r\n",
		"$99999999\r\n",
		"$20\r\n# Clients\r\nblocked:0\r\n\r\n",
	} {
		client, server := net.Pipe()
		go func() {
			buf := make([]byte, 64)
			_, _ = server.Read(buf)
			_, _ = io.WriteString(server, reply)
			_ = server.Close()
		}()
		if n, err := connectedClients(client, bufio.NewReader(client), time.Second); err == nil {
			t.Fatalf("reply %q read as %d clients", reply, n)
		}
		_ = client.Close()
	}
}
