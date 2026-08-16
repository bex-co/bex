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

package proxyproto

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"strings"
	"testing"
)

func TestParseTrustedProxyCIDRs(t *testing.T) {
	got, err := ParseTrustedProxyCIDRs("10.10.0.7/32, 2001:db8::7/128")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].String() != "10.10.0.7/32" || got[1].String() != "2001:db8::7/128" {
		t.Fatalf("prefixes = %v", got)
	}
	if _, err := ParseTrustedProxyCIDRs("10.10.0.7"); err == nil {
		t.Fatal("address without a prefix was accepted")
	}
	if got, err := ParseTrustedProxyCIDRs(""); err != nil || got != nil {
		t.Fatalf("empty = %v, %v, want nil, nil", got, err)
	}
}

func TestReadProxySourcePreservesHeaderlessProtocol(t *testing.T) {
	protocol := []byte{0x00, 0x00, 0x00, 0x08, 0x04, 0xd2, 0x16, 0x2f}
	reader := bufio.NewReader(bytes.NewReader(protocol))
	got, err := ReadProxySource(reader, tcpAddr("198.51.100.9"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "198.51.100.9" {
		t.Fatalf("source = %s", got)
	}
	remaining := make([]byte, len(protocol))
	if _, err := reader.Read(remaining); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(remaining, protocol) {
		t.Fatalf("protocol bytes changed: %x", remaining)
	}
}

func TestReadProxySourceV1(t *testing.T) {
	for _, test := range []struct {
		name   string
		header string
		want   string
	}{
		{name: "IPv4", header: "PROXY TCP4 203.0.113.9 49.12.20.236 49152 6379\r\n", want: "203.0.113.9"},
		{name: "IPv6", header: "PROXY TCP6 2001:db8::9 2a01:4f8:c01e:3d1f::1 49152 5432\r\n", want: "2001:db8::9"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(test.header + "SSH"))
			got, err := ReadProxySource(reader, tcpAddr("10.10.0.7"), trustedEdge())
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != test.want {
				t.Fatalf("source = %s, want %s", got, test.want)
			}
			remaining, err := reader.Peek(3)
			if err != nil || string(remaining) != "SSH" {
				t.Fatalf("remaining = %q, %v", remaining, err)
			}
		})
	}
}

func TestReadProxySourceV2(t *testing.T) {
	for _, test := range []struct {
		name string
		src  string
		dst  string
	}{
		{name: "IPv4", src: "203.0.113.9", dst: "49.12.20.236"},
		{name: "IPv6", src: "2001:db8::9", dst: "2a01:4f8:c01e:3d1f::1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(append(proxyV2Header(t, test.src, test.dst), 0x16)))
			got, err := ReadProxySource(reader, tcpAddr("10.10.0.7"), trustedEdge())
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != test.src {
				t.Fatalf("source = %s, want %s", got, test.src)
			}
			remaining, err := reader.ReadByte()
			if err != nil || remaining != 0x16 {
				t.Fatalf("remaining = 0x%02x, %v", remaining, err)
			}
		})
	}
}

func TestReadProxySourceRejectsUntrustedAndMalformedHeaders(t *testing.T) {
	for _, test := range []struct {
		name    string
		remote  string
		header  []byte
		trusted []netip.Prefix
	}{
		{name: "untrusted v1", remote: "198.51.100.7", header: []byte("PROXY TCP4 203.0.113.9 49.12.20.236 49152 6379\r\n"), trusted: trustedEdge()},
		{name: "invalid source", remote: "10.10.0.7", header: []byte("PROXY TCP4 nope 49.12.20.236 49152 6379\r\n"), trusted: trustedEdge()},
		{name: "zero port", remote: "10.10.0.7", header: []byte("PROXY TCP4 203.0.113.9 49.12.20.236 0 6379\r\n"), trusted: trustedEdge()},
		{name: "oversized v1", remote: "10.10.0.7", header: []byte("PROXY " + strings.Repeat("x", maxProxyV1Header) + "\r\n"), trusted: trustedEdge()},
		{name: "untrusted v2", remote: "198.51.100.7", header: proxyV2Header(t, "203.0.113.9", "49.12.20.236"), trusted: trustedEdge()},
		{name: "zero v2 port", remote: "10.10.0.7", header: zeroProxyV2SourcePort(proxyV2Header(t, "203.0.113.9", "49.12.20.236")), trusted: trustedEdge()},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ReadProxySource(bufio.NewReader(bytes.NewReader(test.header)), tcpAddr(test.remote), test.trusted); err == nil {
				t.Fatal("header was accepted")
			}
		})
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

func zeroProxyV2SourcePort(header []byte) []byte {
	header[24], header[25] = 0, 0
	return header
}

func proxyV2Header(t *testing.T, source, destination string) []byte {
	t.Helper()
	src := netip.MustParseAddr(source)
	dst := netip.MustParseAddr(destination)
	header := append([]byte{}, proxyV2Signature...)
	header = append(header, 0x21)
	var body []byte
	if src.Is4() && dst.Is4() {
		header = append(header, 0x11)
		src4, dst4 := src.As4(), dst.As4()
		body = append(body, src4[:]...)
		body = append(body, dst4[:]...)
	} else if src.Is6() && dst.Is6() {
		header = append(header, 0x21)
		src16, dst16 := src.As16(), dst.As16()
		body = append(body, src16[:]...)
		body = append(body, dst16[:]...)
	} else {
		t.Fatal("mixed address families")
	}
	ports := make([]byte, 4)
	binary.BigEndian.PutUint16(ports[:2], 49152)
	binary.BigEndian.PutUint16(ports[2:], 6379)
	body = append(body, ports...)
	length := make([]byte, 2)
	binary.BigEndian.PutUint16(length, uint16(len(body)))
	header = append(header, length...)
	return append(header, body...)
}

func tcpAddr(address string) net.Addr {
	return &net.TCPAddr{IP: net.ParseIP(address), Port: 32123}
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
