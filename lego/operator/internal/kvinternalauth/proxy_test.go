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
	addr, _ := startProxy(t, Config{Upstream: upstream, Passthrough: true})
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
	tlsAddr, _ := startProxy(t, Config{Upstream: "127.0.0.1:1", Passthrough: true, Budget: budget, TLSCertFile: certFile, TLSKeyFile: keyFile})
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
	if !budget.acquire() {
		t.Fatal("empty budget refused")
	}
	addr, _ := startProxy(t, Config{Upstream: "127.0.0.1:1", Passthrough: true, Budget: budget})
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
		if !b.acquire() {
			t.Fatalf("refused connection %d of 5000", i+1)
		}
	}
	if b.acquire() {
		t.Fatal("admitted connection 5001")
	}
	b.release()
	if !b.acquire() {
		t.Fatal("a released slot was not reusable")
	}
	if got := cap(NewBudget(0).slots); got != 1024 {
		t.Fatalf("default budget %d, want 1024", got)
	}
}

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

func TestCheckRequiresAuthenticatedPong(t *testing.T) {
	for _, response := range []string{"+PONG\r\n", "-LOADING dataset\r\n"} {
		t.Run(strings.TrimSpace(response), func(t *testing.T) {
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
			err := Check(t.Context(), Config{Upstream: upstream, Password: testPassword})
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
