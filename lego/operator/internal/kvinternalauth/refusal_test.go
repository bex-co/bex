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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// w8/m562: an external client the store's allow list refuses completes TLS
// under the store's certificate and reads Render's refusal for every command,
// from a listener that has no upstream.

const refusalReply = "-Client IP address is not in the allowlist.\r\n"

// refusalListener serves an AllowListRefusal listener with the test
// certificate and the given lifetime per connection.
func refusalListener(t *testing.T, config Config) (addr string, roots *x509.CertPool) {
	t.Helper()
	certFile, keyFile, roots := writeTestCertificate(t)
	config.Mode, config.TLSCertFile, config.TLSKeyFile = AllowListRefusal, certFile, keyFile
	addr, _ = startProxy(t, config)
	return addr, roots
}

func refusedClient(t *testing.T, addr string, roots *x509.CertPool) *tls.Conn {
	t.Helper()
	client := tls.Client(dialProxy(t, addr), &tls.Config{ServerName: "kv.example.test", RootCAs: roots, MinVersion: tls.VersionTLS12})
	if err := client.Handshake(); err != nil {
		t.Fatalf("the refused client could not verify the store's certificate: %v", err)
	}
	return client
}

func write(t *testing.T, conn net.Conn, data string) {
	t.Helper()
	if _, err := io.WriteString(conn, data); err != nil {
		t.Fatal(err)
	}
}

// valkey-cli's opening AUTH, the command after it, a pipeline mixing both
// forms, and inline commands each read the refusal; empty commands read
// nothing, as on Valkey. A full budget changes nothing: the refusal draws no
// slot.
func TestAllowListRefusalAnswersEveryCommandInsideTLS(t *testing.T) {
	budget := NewBudget(1)
	if !budget.admit(stream(0)) {
		t.Fatal("empty budget refused")
	}
	addr, roots := refusalListener(t, Config{Budget: budget})
	client := refusedClient(t, addr, roots)

	write(t, client, authFrame())
	if err := readExact(client, refusalReply); err != nil {
		t.Fatalf("the opening AUTH: %v", err)
	}
	write(t, client, "*1\r\n$4\r\nPING\r\n")
	if err := readExact(client, refusalReply); err != nil {
		t.Fatalf("the command after it: %v", err)
	}
	write(t, client, authFrame()+"*2\r\n$4\r\nINFO\r\n$7\r\nclients\r\nPING\r\n\r\n*0\r\n  \r\nGET \"a b\"\n")
	if err := readExact(client, strings.Repeat(refusalReply, 4)); err != nil {
		t.Fatalf("the pipeline: %v", err)
	}
	if n := budget.held(); n != 1 {
		t.Fatalf("the refusal holds %d budget slots, want the 1 held before it", n-1)
	}
	// The empty commands were answered with nothing: once the client closes
	// its side, no reply is left to read.
	if err := client.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	if rest, err := io.ReadAll(client); err != nil || len(rest) != 0 {
		t.Fatalf("after the pipeline the client read %q (%v), want nothing", rest, err)
	}
}

// The largest command Valkey reads from an unauthenticated client is answered
// too: 10 arguments, one of them 16 KiB.
func TestAllowListRefusalAnswersACommandAtItsBounds(t *testing.T) {
	addr, roots := refusalListener(t, Config{})
	client := refusedClient(t, addr, roots)
	command := fmt.Sprintf("*%d\r\n$%d\r\n%s\r\n", maxRefusedArgs, maxRefusedArgBytes, strings.Repeat("v", maxRefusedArgBytes))
	command += strings.Repeat("$1\r\na\r\n", maxRefusedArgs-1)
	write(t, client, command+strings.Repeat("x", maxRefusedArgBytes)+"\r\n")
	if err := readExact(client, refusalReply+refusalReply); err != nil {
		t.Fatal(err)
	}
}

// A malformed or oversized command closes the connection unanswered, so the
// refusal never buffers more than its bounds.
func TestAllowListRefusalClosesAMalformedOrOversizedCommand(t *testing.T) {
	addr, roots := refusalListener(t, Config{})
	for name, command := range map[string]string{
		"too many arguments":      "*11\r\n" + strings.Repeat("$1\r\na\r\n", 11),
		"an oversized argument":   fmt.Sprintf("*1\r\n$%d\r\n", maxRefusedArgBytes+1),
		"an oversized inline":     strings.Repeat("a", maxRefusedArgBytes+3) + "\r\n",
		"a negative length":       "*1\r\n$-1\r\n",
		"no bulk header":          "*1\r\n:4\r\nPING\r\n",
		"an unterminated bulk":    "*1\r\n$4\r\nPINGxx",
		"a non-numeric arg count": "*x\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			client := refusedClient(t, addr, roots)
			_, _ = io.WriteString(client, command)
			// The close may arrive as a reset while the command is unread.
			if got, _ := io.ReadAll(client); len(got) != 0 {
				t.Fatalf("read %q, want the connection closed unanswered", got)
			}
		})
	}
}

// The refusal answers maxRefusedCommands commands, then closes.
func TestAllowListRefusalClosesAfterItsCommandBound(t *testing.T) {
	addr, roots := refusalListener(t, Config{})
	client := refusedClient(t, addr, roots)
	write(t, client, strings.Repeat("PING\r\n", maxRefusedCommands))
	if err := readExact(client, strings.Repeat(refusalReply, maxRefusedCommands)); err != nil {
		t.Fatal(err)
	}
	if rest, err := io.ReadAll(client); err != nil || len(rest) != 0 {
		t.Fatalf("after %d commands the client read %q (%v), want the close", maxRefusedCommands, rest, err)
	}
}

// One deadline bounds a refused connection's whole life, however the client
// behaves.
func TestAllowListRefusalClosesAnIdleClientAtItsDeadline(t *testing.T) {
	const lifetime = 300 * time.Millisecond
	addr, roots := refusalListener(t, Config{HandshakeTimeout: lifetime})
	began := time.Now()
	client := refusedClient(t, addr, roots)
	_, err := client.Read(make([]byte, 1))
	var timeout net.Error
	if err == nil || errors.As(err, &timeout) && timeout.Timeout() {
		t.Fatalf("the idle refused client was not closed: %v", err)
	}
	if held := time.Since(began); held < lifetime-50*time.Millisecond || held > 2*time.Second {
		t.Fatalf("the idle refused client was closed after %v, want about %v", held, lifetime)
	}
}

// Without the store's certificate the client is closed before any handshake
// completes, as Valkey's TLS port is closed without it.
func TestAllowListRefusalWithoutTheCertificateCloses(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	addr, _ := startProxy(t, Config{Mode: AllowListRefusal, TLSCertFile: missing + ".crt", TLSKeyFile: missing + ".key"})
	// Verification is off, so only the server can fail this handshake.
	client := tls.Client(dialProxy(t, addr), &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
	err := client.Handshake()
	var timeout net.Error
	if err == nil || errors.As(err, &timeout) && timeout.Timeout() {
		t.Fatalf("handshake = %v, want the connection closed", err)
	}
}

// At most maxRefusing refusals run at once; the next client is closed at
// accept, and a slot that frees up serves the next one.
func TestAllowListRefusalsPastTheBoundAreClosed(t *testing.T) {
	addr, roots := refusalListener(t, Config{HandshakeTimeout: 5 * time.Second})
	held := make([]*tls.Conn, maxRefusing)
	for i := range held {
		held[i] = refusedClient(t, addr, roots)
	}
	flood := dialProxy(t, addr)
	if got, err := io.ReadAll(flood); err != nil || len(got) != 0 {
		t.Fatalf("a client past the bound read %q (%v), want a bare close", got, err)
	}
	_ = held[0].Close()
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		client := tls.Client(dialProxy(t, addr), &tls.Config{ServerName: "kv.example.test", RootCAs: roots, MinVersion: tls.VersionTLS12})
		if client.Handshake() == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a freed refusal slot did not serve the next client")
		}
	}
}

// The listener has nothing to dial: a configuration that names an upstream or
// a password, or lacks the certificate, is refused before it accepts.
func TestAllowListRefusalNamesNoUpstreamOrPassword(t *testing.T) {
	upstream, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = upstream.Close() })
	var dialed atomic.Int64
	go func() {
		for {
			conn, err := upstream.Accept()
			if err != nil {
				return
			}
			dialed.Add(1)
			_ = conn.Close()
		}
	}()
	certFile, keyFile, _ := writeTestCertificate(t)
	for name, config := range map[string]Config{
		"an upstream":       {Upstream: upstream.Addr().String(), TLSCertFile: certFile, TLSKeyFile: keyFile},
		"a password":        {Password: testPassword, TLSCertFile: certFile, TLSKeyFile: keyFile},
		"no certificate":    {TLSKeyFile: keyFile},
		"no key":            {TLSCertFile: certFile},
		"an empty identity": {},
	} {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		config.Mode = AllowListRefusal
		if err := Serve(t.Context(), listener, config); err == nil {
			t.Errorf("%s: an allow-list refusal listener served", name)
		} else if strings.Contains(err.Error(), testPassword) {
			t.Errorf("%s: the error exposes the password", name)
		}
	}
	if n := dialed.Load(); n != 0 {
		t.Fatalf("the upstream saw %d connections", n)
	}
}
