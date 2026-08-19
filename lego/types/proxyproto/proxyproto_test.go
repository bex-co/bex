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

// This is the merged conformance suite for the shared PROXY v1/v2 parser
// (w1/m77): the union of the former backend/internal/proxyproto and
// operator/internal/sniproxy suites (the wrapper packages keep only their
// module-specific tests), plus the v1/v2 boundary and malformed-header cases
// neither prior suite covered (w1/m77 t005).
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

func TestRemoteIP(t *testing.T) {
	got, err := RemoteIP(tcpAddr("203.0.113.9"))
	if err != nil || got.String() != "203.0.113.9" {
		t.Fatalf("RemoteIP = %s, %v", got, err)
	}
	got, err = RemoteIP(tcpAddr("2001:db8::9"))
	if err != nil || got.String() != "2001:db8::9" {
		t.Fatalf("RemoteIP = %s, %v", got, err)
	}
	if _, err := RemoteIP(&net.UnixAddr{Name: "/run/x.sock", Net: "unix"}); err == nil {
		t.Fatal("portless address was accepted")
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

// TestReadProxySourcePreservesNearMissPrefixes covers protocols whose first
// byte collides with a PROXY v1 ("P") or v2 ("\r") header: the peek must fall
// through to the immediate peer and leave every byte buffered for the caller's
// own protocol reader.
func TestReadProxySourcePreservesNearMissPrefixes(t *testing.T) {
	for _, test := range []struct {
		name     string
		protocol string
	}{
		{name: "P but not PROXY", protocol: "POST / HTTP/1.1\r\n"},
		{name: "short P prefix", protocol: "P"},
		{name: "CR but not v2 signature", protocol: "\r\nhello"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(test.protocol))
			got, err := ReadProxySource(reader, tcpAddr("10.10.0.7"), trustedEdge())
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != "10.10.0.7" {
				t.Fatalf("source = %s, want the immediate peer", got)
			}
			remaining := make([]byte, len(test.protocol))
			if _, err := io.ReadFull(reader, remaining); err != nil {
				t.Fatal(err)
			}
			if string(remaining) != test.protocol {
				t.Fatalf("protocol bytes changed: %q", remaining)
			}
		})
	}
}

func TestReadProxySourceV1(t *testing.T) {
	for _, test := range []struct {
		name    string
		header  string
		trailer string
		want    string
	}{
		// The SSH trailers come from the former backend suite, the TLS
		// trailers from the former operator suite — the union keeps both.
		{name: "IPv4 before SSH", header: "PROXY TCP4 203.0.113.9 49.12.20.236 49152 6379\r\n", trailer: "SSH", want: "203.0.113.9"},
		{name: "IPv6 before SSH", header: "PROXY TCP6 2001:db8::9 2a01:4f8:c01e:3d1f::1 49152 5432\r\n", trailer: "SSH", want: "2001:db8::9"},
		{name: "IPv4 before TLS", header: "PROXY TCP4 203.0.113.9 49.12.20.236 49152 6379\r\n", trailer: "TLS", want: "203.0.113.9"},
		{name: "IPv6 before TLS", header: "PROXY TCP6 2001:db8::9 2a01:4f8:c01e:3d1f::1 49152 5432\r\n", trailer: "TLS", want: "2001:db8::9"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(test.header + test.trailer))
			got, err := ReadProxySource(reader, tcpAddr("10.10.0.7"), trustedEdge())
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != test.want {
				t.Fatalf("source = %s, want %s", got, test.want)
			}
			remaining, err := reader.Peek(3)
			if err != nil || string(remaining) != test.trailer {
				t.Fatalf("remaining = %q, %v", remaining, err)
			}
		})
	}
}

// TestReadProxySourceV1Unknown covers the spec's UNKNOWN transport: the header
// is consumed, the immediate (trusted) peer is retained, and the protocol
// bytes after the header are untouched.
func TestReadProxySourceV1Unknown(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("PROXY UNKNOWN\r\nSSH"))
	got, err := ReadProxySource(reader, tcpAddr("10.10.0.7"), trustedEdge())
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "10.10.0.7" {
		t.Fatalf("source = %s, want the immediate peer", got)
	}
	remaining, err := reader.Peek(3)
	if err != nil || string(remaining) != "SSH" {
		t.Fatalf("remaining = %q, %v", remaining, err)
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

// TestReadProxySourceV2Local covers the LOCAL command (health checks from the
// proxy itself): the header is consumed and the trusted immediate peer is
// retained.
func TestReadProxySourceV2Local(t *testing.T) {
	header := append([]byte{}, proxyV2Signature...)
	header = append(header, 0x20, 0x00, 0x00, 0x00) // LOCAL, UNSPEC, zero-length body.
	reader := bufio.NewReader(bytes.NewReader(append(header, 0x16)))
	got, err := ReadProxySource(reader, tcpAddr("10.10.0.7"), trustedEdge())
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "10.10.0.7" {
		t.Fatalf("source = %s, want the immediate peer", got)
	}
	remaining, err := reader.ReadByte()
	if err != nil || remaining != 0x16 {
		t.Fatalf("remaining = 0x%02x, %v", remaining, err)
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
		// The cases below extend the merged suite (w1/m77 t005); everything
		// above is verbatim from both prior suites.
		{name: "empty input", remote: "10.10.0.7", header: nil, trusted: trustedEdge()},
		{name: "v1 missing CRLF", remote: "10.10.0.7", header: []byte("PROXY TCP4 203.0.113.9 49.12.20.236 49152 6379\n"), trusted: trustedEdge()},
		{name: "v1 truncated", remote: "10.10.0.7", header: []byte("PROXY TCP4 203.0.113.9"), trusted: trustedEdge()},
		{name: "v1 family mismatch", remote: "10.10.0.7", header: []byte("PROXY TCP4 2001:db8::9 49.12.20.236 49152 6379\r\n"), trusted: trustedEdge()},
		{name: "v1 non-numeric port", remote: "10.10.0.7", header: []byte("PROXY TCP4 203.0.113.9 49.12.20.236 49152 http\r\n"), trusted: trustedEdge()},
		{name: "v2 wrong version", remote: "10.10.0.7", header: setProxyV2Byte(proxyV2Header(t, "203.0.113.9", "49.12.20.236"), 12, 0x11), trusted: trustedEdge()},
		{name: "v2 invalid command", remote: "10.10.0.7", header: setProxyV2Byte(proxyV2Header(t, "203.0.113.9", "49.12.20.236"), 12, 0x22), trusted: trustedEdge()},
		{name: "v2 non-stream transport", remote: "10.10.0.7", header: setProxyV2Byte(proxyV2Header(t, "203.0.113.9", "49.12.20.236"), 13, 0x12), trusted: trustedEdge()},
		{name: "v2 unsupported family", remote: "10.10.0.7", header: setProxyV2Byte(proxyV2Header(t, "203.0.113.9", "49.12.20.236"), 13, 0x31), trusted: trustedEdge()},
		{name: "v2 oversized body", remote: "10.10.0.7", header: proxyV2Frame(0x21, 0x11, maxProxyV2Body+1), trusted: trustedEdge()},
		{name: "v2 truncated body", remote: "10.10.0.7", header: proxyV2Header(t, "203.0.113.9", "49.12.20.236")[:20], trusted: trustedEdge()},
		{name: "v2 short IPv4 block", remote: "10.10.0.7", header: proxyV2Frame(0x21, 0x11, 4), trusted: trustedEdge()},
		{name: "v2 short IPv6 block", remote: "10.10.0.7", header: proxyV2Frame(0x21, 0x21, 12), trusted: trustedEdge()},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ReadProxySource(bufio.NewReader(bytes.NewReader(test.header)), tcpAddr(test.remote), test.trusted); err == nil {
				t.Fatal("header was accepted")
			}
		})
	}
}

func zeroProxyV2SourcePort(header []byte) []byte {
	header[24], header[25] = 0, 0
	return header
}

func setProxyV2Byte(header []byte, index int, value byte) []byte {
	header[index] = value
	return header
}

// proxyV2Frame builds a v2 header with an arbitrary version/command byte,
// family/protocol byte, and declared body length, followed by that many zero
// body bytes (capped so the oversized-length case truly overdeclares).
func proxyV2Frame(verCmd, famProto byte, bodyLen int) []byte {
	header := append([]byte{}, proxyV2Signature...)
	header = append(header, verCmd, famProto)
	length := make([]byte, 2)
	binary.BigEndian.PutUint16(length, uint16(bodyLen))
	header = append(header, length...)
	if bodyLen <= maxProxyV2Body {
		header = append(header, make([]byte, bodyLen)...)
	}
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
