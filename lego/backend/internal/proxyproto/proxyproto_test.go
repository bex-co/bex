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

// The PROXY v1/v2 parser conformance suite lives with the shared parser in
// lego/types/proxyproto (w1/m77). This file keeps only the gateway-specific
// net.Conn wrapper tests plus the delegation guard.
package proxyproto

import (
	"io"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	shared "github.com/bex-co/bex/lego/types/proxyproto"
)

// TestDelegatesToSharedParser pins each re-export to the shared parser's own
// function by identity: a future re-fork (a local body shadowing
// lego/types/proxyproto — how the pre-w1/m77 copies drifted) changes the
// function pointer and fails here instead of waiting for a security review to
// notice. The parser's output is the trusted client address used for
// per-source admission, so the two modules must never diverge again.
func TestDelegatesToSharedParser(t *testing.T) {
	for name, funcs := range map[string][2]any{
		"ParseTrustedProxyCIDRs": {ParseTrustedProxyCIDRs, shared.ParseTrustedProxyCIDRs},
		"RemoteIP":               {RemoteIP, shared.RemoteIP},
		"ReadProxySource":        {ReadProxySource, shared.ReadProxySource},
	} {
		if reflect.ValueOf(funcs[0]).Pointer() != reflect.ValueOf(funcs[1]).Pointer() {
			t.Errorf("%s no longer delegates to types/proxyproto", name)
		}
	}
}

// TestWrapUnconfiguredReturnsConnUnchanged proves the default (no trusted
// CIDRs) path never buffers or peeks — a direct pass-through, byte-identical
// to the pre-proxyproto behavior.
func TestWrapUnconfiguredReturnsConnUnchanged(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	wrapped, err := Wrap(server, nil)
	if err != nil {
		t.Fatal(err)
	}
	if wrapped != net.Conn(server) {
		t.Fatal("Wrap with no trusted CIDRs must return the original conn")
	}
}

// TestWrapHonorsHeaderFromTrustedPeer proves the fix end to end: a real
// net.Conn pair with a PROXY v1 header written by a trusted peer yields a
// wrapped conn whose RemoteAddr is the header's source, and whose Read stream
// still delivers the bytes written after the header untouched.
func TestWrapHonorsHeaderFromTrustedPeer(t *testing.T) {
	client, server := pipeWithRemoteAddr(t, "10.10.0.7:41000")
	defer client.Close()
	defer server.Close()

	go func() {
		_, _ = client.Write([]byte("PROXY TCP4 203.0.113.9 49.12.20.236 49152 22\r\n"))
		_, _ = client.Write([]byte("SSH-2.0-OpenSSH\r\n"))
	}()

	wrapped, err := Wrap(server, trustedEdge())
	if err != nil {
		t.Fatal(err)
	}
	if got := wrapped.RemoteAddr().String(); !strings.HasPrefix(got, "203.0.113.9") {
		t.Fatalf("RemoteAddr = %s, want the PROXY source", got)
	}
	buf := make([]byte, len("SSH-2.0-OpenSSH\r\n"))
	if _, err := io.ReadFull(wrapped, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "SSH-2.0-OpenSSH\r\n" {
		t.Fatalf("post-header bytes = %q", buf)
	}
}

// TestWrapFallsBackWithoutHeaderFromTrustedPeer proves a trusted-but-headerless
// connection (rollout window before Traefik's IngressRouteTCP sends the
// header) still works, keeping the immediate peer as RemoteAddr.
func TestWrapFallsBackWithoutHeaderFromTrustedPeer(t *testing.T) {
	client, server := pipeWithRemoteAddr(t, "10.10.0.7:41000")
	defer client.Close()
	defer server.Close()

	go func() { _, _ = client.Write([]byte("SSH-2.0-OpenSSH\r\n")) }()

	wrapped, err := Wrap(server, trustedEdge())
	if err != nil {
		t.Fatal(err)
	}
	if got := wrapped.RemoteAddr().String(); !strings.HasPrefix(got, "10.10.0.7") {
		t.Fatalf("RemoteAddr = %s, want the immediate peer", got)
	}
}

func trustedEdge() []netip.Prefix {
	return []netip.Prefix{netip.MustParsePrefix("10.10.0.7/32")}
}

// fakeAddrConn overrides net.Pipe's synthetic RemoteAddr (which is not a real
// TCPAddr) with a caller-supplied address, so Wrap's peer-trust check against
// a CIDR is exercisable without a real socket.
type fakeAddrConn struct {
	net.Conn
	remote net.Addr
}

func (c fakeAddrConn) RemoteAddr() net.Addr { return c.remote }

func pipeWithRemoteAddr(t *testing.T, addr string) (net.Conn, net.Conn) {
	t.Helper()
	tcp, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	client, server := net.Pipe()
	return client, fakeAddrConn{Conn: server, remote: tcp}
}
