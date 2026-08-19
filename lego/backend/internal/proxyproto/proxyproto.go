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
// Traefik's own pod IP into ssh_sessions.remote_address (w4/029.md #10). The
// PROXY protocol v1/v2 parser itself lives in the shared leaf module
// lego/types/proxyproto (w1/m77) — one implementation for this gateway and
// the operator's pg/kv SNI proxies, so a parser fix can never land in one
// copy and miss the other. This package re-exports the parser and keeps the
// gateway-only net.Conn plumbing (Conn/Wrap).
package proxyproto

import (
	"bufio"
	"net"
	"net/netip"

	shared "github.com/bex-co/bex/lego/types/proxyproto"
)

// ParseTrustedProxyCIDRs, RemoteIP, and ReadProxySource are direct re-exports
// of the shared parser — assignments, not wrapper funcs, so the delegation
// guard test can prove by function identity that no local fork has crept back
// in. See the shared package for their documentation.
var (
	ParseTrustedProxyCIDRs = shared.ParseTrustedProxyCIDRs
	RemoteIP               = shared.RemoteIP
	ReadProxySource        = shared.ReadProxySource
)

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
	source, err := shared.ReadProxySource(reader, conn.RemoteAddr(), trusted)
	if err != nil {
		return nil, err
	}
	remote := net.Addr(&net.TCPAddr{IP: source.AsSlice()})
	if tcp, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		remote = &net.TCPAddr{IP: source.AsSlice(), Port: tcp.Port, Zone: tcp.Zone}
	}
	return &Conn{Conn: conn, reader: reader, remote: remote}, nil
}
