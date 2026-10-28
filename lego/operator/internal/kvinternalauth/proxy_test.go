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
	"fmt"
	"io"
	"net"
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

func TestConnectionLimitShedsBeforeDialingUpstream(t *testing.T) {
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
	addr, _ := startProxy(t, Config{Upstream: upstream, Password: testPassword, MaxConnections: 1})
	first := dialProxy(t, addr)
	awaitSignal(t, authenticated)
	second := dialProxy(t, addr)
	var b [1]byte
	if n, err := second.Read(b[:]); n != 0 || err == nil {
		t.Fatalf("excess connection not shed: n=%d err=%v", n, err)
	}
	_ = first.Close()
	finish()
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
