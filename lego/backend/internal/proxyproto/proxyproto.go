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

// Package proxyproto lets the SSH gateway recover the real client address of
// a connection Traefik's `ssh` entrypoint forwards, instead of recording
// Traefik's own pod IP into ssh_sessions.remote_address (w4/029.md #10). It is
// a deliberate, self-contained duplicate of
// lego/operator/internal/sniproxy's PROXY protocol v1/v2 parser: the backend
// module never imports the operator module (see the repo CLAUDE.md), and this
// parser has zero bex dependencies, so copying the ~150 lines here is cheaper
// and safer than inventing a shared third module for one function.
package proxyproto

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

const (
	proxyV1Prefix    = "PROXY "
	proxyTCP4        = "TCP4"
	proxyTCP6        = "TCP6"
	maxProxyV1Header = 108
	maxProxyV2Body   = 216
)

var proxyV2Signature = []byte("\r\n\r\n\x00\r\nQUIT\n")

// ParseTrustedProxyCIDRs parses the comma-separated immediate peers that may
// assert an original client address. An empty value disables PROXY trust while
// preserving direct, headerless connections.
func ParseTrustedProxyCIDRs(raw string) ([]netip.Prefix, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	items := strings.Split(raw, ",")
	prefixes := make([]netip.Prefix, 0, len(items))
	for _, item := range items {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(item))
		if err != nil {
			return nil, fmt.Errorf("parse trusted PROXY CIDR %q: %w", item, err)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

// RemoteIP extracts the IP from a net.Addr's string form.
func RemoteIP(addr net.Addr) (netip.Addr, error) {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return netip.Addr{}, err
	}
	return netip.ParseAddr(host)
}

// ReadProxySource consumes an optional PROXY protocol v1 or v2 header and
// returns the original client address. Headers are authoritative only from an
// explicitly trusted immediate peer. Non-PROXY bytes stay buffered for the
// caller's own protocol reader.
func ReadProxySource(reader *bufio.Reader, remote net.Addr, trusted []netip.Prefix) (netip.Addr, error) {
	immediate, err := RemoteIP(remote)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("parse immediate peer: %w", err)
	}
	first, err := reader.Peek(1)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("read connection prefix: %w", err)
	}

	switch first[0] {
	case proxyV1Prefix[0]:
		prefix, peekErr := reader.Peek(len(proxyV1Prefix))
		if peekErr != nil || string(prefix) != proxyV1Prefix {
			return immediate.Unmap(), nil
		}
		if !trustedProxyPeer(immediate, trusted) {
			return netip.Addr{}, fmt.Errorf("PROXY header from untrusted peer")
		}
		return readProxyV1(reader, immediate)
	case proxyV2Signature[0]:
		signature, peekErr := reader.Peek(len(proxyV2Signature))
		if peekErr != nil || !bytes.Equal(signature, proxyV2Signature) {
			return immediate.Unmap(), nil
		}
		if !trustedProxyPeer(immediate, trusted) {
			return netip.Addr{}, fmt.Errorf("PROXY header from untrusted peer")
		}
		return readProxyV2(reader, immediate)
	default:
		return immediate.Unmap(), nil
	}
}

func trustedProxyPeer(peer netip.Addr, trusted []netip.Prefix) bool {
	peer = peer.Unmap()
	for _, prefix := range trusted {
		if prefix.Contains(peer) {
			return true
		}
	}
	return false
}

func readProxyV1(reader *bufio.Reader, immediate netip.Addr) (netip.Addr, error) {
	lineBytes, err := reader.ReadSlice('\n')
	if err != nil {
		if err == bufio.ErrBufferFull {
			return netip.Addr{}, fmt.Errorf("PROXY v1 header too large")
		}
		return netip.Addr{}, fmt.Errorf("read PROXY v1 header: %w", err)
	}
	if len(lineBytes) > maxProxyV1Header {
		return netip.Addr{}, fmt.Errorf("PROXY v1 header too large")
	}
	line := string(lineBytes)
	if !strings.HasSuffix(line, "\r\n") {
		return netip.Addr{}, fmt.Errorf("PROXY v1 header missing CRLF")
	}
	fields := strings.Fields(strings.TrimSuffix(line, "\r\n"))
	if len(fields) == 2 && fields[1] == "UNKNOWN" {
		return immediate.Unmap(), nil
	}
	if len(fields) != 6 || fields[0] != "PROXY" || (fields[1] != proxyTCP4 && fields[1] != proxyTCP6) {
		return netip.Addr{}, fmt.Errorf("invalid PROXY v1 header")
	}
	source, err := netip.ParseAddr(fields[2])
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid PROXY v1 source address: %w", err)
	}
	destination, err := netip.ParseAddr(fields[3])
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid PROXY v1 destination address: %w", err)
	}
	if (fields[1] == proxyTCP4) != source.Is4() || (fields[1] == proxyTCP4) != destination.Is4() {
		return netip.Addr{}, fmt.Errorf("PROXY v1 address family mismatch")
	}
	for _, rawPort := range fields[4:6] {
		port, portErr := strconv.ParseUint(rawPort, 10, 16)
		if portErr != nil || port == 0 {
			return netip.Addr{}, fmt.Errorf("invalid PROXY v1 port")
		}
	}
	return source.Unmap(), nil
}

func readProxyV2(reader *bufio.Reader, immediate netip.Addr) (netip.Addr, error) {
	header := make([]byte, 16)
	if _, err := io.ReadFull(reader, header); err != nil {
		return netip.Addr{}, fmt.Errorf("read PROXY v2 header: %w", err)
	}
	if !bytes.Equal(header[:12], proxyV2Signature) || header[12]>>4 != 2 {
		return netip.Addr{}, fmt.Errorf("invalid PROXY v2 header")
	}
	bodyLen := int(binary.BigEndian.Uint16(header[14:16]))
	if bodyLen > maxProxyV2Body {
		return netip.Addr{}, fmt.Errorf("PROXY v2 header too large")
	}
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(reader, body); err != nil {
		return netip.Addr{}, fmt.Errorf("read PROXY v2 body: %w", err)
	}

	switch header[12] & 0x0f {
	case 0: // LOCAL: retain the trusted immediate peer.
		return immediate.Unmap(), nil
	case 1: // PROXY
	default:
		return netip.Addr{}, fmt.Errorf("invalid PROXY v2 command")
	}
	if header[13]&0x0f != 1 {
		return netip.Addr{}, fmt.Errorf("PROXY v2 transport is not STREAM")
	}
	switch header[13] >> 4 {
	case 1:
		if len(body) < 12 {
			return netip.Addr{}, fmt.Errorf("short PROXY v2 IPv4 address block")
		}
		if binary.BigEndian.Uint16(body[8:10]) == 0 || binary.BigEndian.Uint16(body[10:12]) == 0 {
			return netip.Addr{}, fmt.Errorf("invalid PROXY v2 port")
		}
		return netip.AddrFrom4([4]byte(body[:4])).Unmap(), nil
	case 2:
		if len(body) < 36 {
			return netip.Addr{}, fmt.Errorf("short PROXY v2 IPv6 address block")
		}
		if binary.BigEndian.Uint16(body[32:34]) == 0 || binary.BigEndian.Uint16(body[34:36]) == 0 {
			return netip.Addr{}, fmt.Errorf("invalid PROXY v2 port")
		}
		return netip.AddrFrom16([16]byte(body[:16])).Unmap(), nil
	default:
		return netip.Addr{}, fmt.Errorf("unsupported PROXY v2 address family")
	}
}

// Conn wraps a net.Conn so Read continues from a bufio.Reader that may already
// hold bytes buffered while Wrap peeked for a PROXY header, and RemoteAddr
// reports the resolved original client address instead of the immediate TCP
// peer (Traefik's own pod IP).
type Conn struct {
	net.Conn
	reader *bufio.Reader
	remote net.Addr
}

func (c *Conn) Read(b []byte) (int, error) { return c.reader.Read(b) }
func (c *Conn) RemoteAddr() net.Addr       { return c.remote }

// Wrap consumes an optional PROXY protocol v1/v2 header from conn — honored
// only when the immediate peer is in trusted — and returns a net.Conn whose
// RemoteAddr reflects the resolved original client. An empty trusted list
// returns conn unchanged (no Peek, no buffering): the default, off state.
func Wrap(conn net.Conn, trusted []netip.Prefix) (net.Conn, error) {
	if len(trusted) == 0 {
		return conn, nil
	}
	reader := bufio.NewReader(conn)
	source, err := ReadProxySource(reader, conn.RemoteAddr(), trusted)
	if err != nil {
		return nil, err
	}
	remote := net.Addr(&net.TCPAddr{IP: source.AsSlice()})
	if tcp, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		remote = &net.TCPAddr{IP: source.AsSlice(), Port: tcp.Port, Zone: tcp.Zone}
	}
	return &Conn{Conn: conn, reader: reader, remote: remote}, nil
}
