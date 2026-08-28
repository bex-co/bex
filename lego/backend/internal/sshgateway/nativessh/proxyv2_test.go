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

package nativessh

import (
	"encoding/binary"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// proxyV2Header builds a PROXY protocol v2 TCP4 header — the exact framing
// Traefik's ssh IngressRouteTCP (proxyProtocol.version: 2) prepends to every
// forwarded connection in production.
func proxyV2Header(src, dst netip.Addr, sport, dport uint16) []byte {
	header := []byte("\r\n\r\n\x00\r\nQUIT\n") // 12-byte v2 signature
	header = append(header, 0x21)              // version 2 (high nibble) | PROXY command (low)
	header = append(header, 0x11)              // AF_INET (high nibble) | STREAM (low)
	body := make([]byte, 12)
	srcBytes, dstBytes := src.As4(), dst.As4()
	copy(body[0:4], srcBytes[:])
	copy(body[4:8], dstBytes[:])
	binary.BigEndian.PutUint16(body[8:10], sport)
	binary.BigEndian.PutUint16(body[10:12], dport)
	length := make([]byte, 2)
	binary.BigEndian.PutUint16(length, uint16(len(body)))
	return append(append(header, length...), body...)
}

// TestServeConnCompletesHandshakeThroughProxyV2FromTrustedPeer is the fix, at the
// gateway level and over the production transport: with the trusted-proxy CIDR
// configured (BEX_SSH_PROXY_PROTOCOL_TRUSTED_CIDRS), a Traefik-shaped PROXY v2
// header is consumed before the SSH version exchange, the handshake completes,
// and the session audit records the ORIGINAL client the header asserted. Absent
// the fix (see the sibling test) this same header breaks the handshake.
func TestServeConnCompletesHandshakeThroughProxyV2FromTrustedPeer(t *testing.T) {
	clientSigner := signer(t)
	st := &gatewaytest.FakeStore{}
	registry := prometheus.NewRegistry()
	addr, stop := startGatewayConfigured(t, st, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}, clientSigner, func(server *Server) {
		server.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
		server.Metrics = sshgateway.NewMetrics(registry)
	})
	defer stop()

	raw, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	header := proxyV2Header(netip.MustParseAddr("203.0.113.9"), netip.MustParseAddr("127.0.0.1"), 49152, 22)
	if _, err := raw.Write(header); err != nil {
		t.Fatal(err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(raw, addr, &ssh.ClientConfig{
		User: "srv-abcdeabcdeabcdeabcde", Auth: []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("handshake through a stripped PROXY v2 header: %v", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	_ = session.Close()
	_ = client.Close()

	time.Sleep(20 * time.Millisecond)
	started := st.StartedSessions()
	if len(started) != 1 || !strings.HasPrefix(started[0].RemoteAddress, "203.0.113.9:") {
		t.Fatalf("started sessions = %+v, want RemoteAddress from the PROXY v2 header", started)
	}
	if v := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_handshakes_total", map[string]string{"result": "established"}); v != 1 {
		t.Fatalf("handshakes_total{result=established} = %v, want 1", v)
	}
}

// TestServeConnUnstrippedProxyV2HeaderBreaksHandshake reproduces the w6/m132
// regression exactly: Traefik forwards a PROXY v2 header, but the gateway is not
// configured to strip it (empty TrustedProxies, as the deployment shipped after
// w4/m82). The un-stripped binary header is fed into the SSH version exchange,
// the client never completes the handshake, and the gateway records a failed
// pre-authentication handshake — the loud, distinguishable signal (t003).
func TestServeConnUnstrippedProxyV2HeaderBreaksHandshake(t *testing.T) {
	clientSigner := signer(t)
	registry := prometheus.NewRegistry()
	addr, stop := startGatewayConfigured(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}, clientSigner, func(server *Server) {
		// No TrustedProxies: proxyproto.Wrap is a no-op, mirroring the shipped
		// deployment. A short handshake timeout keeps the deterministic failure fast.
		server.HandshakeTimeout = 500 * time.Millisecond
		server.Metrics = sshgateway.NewMetrics(registry)
	})
	defer stop()

	raw, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	header := proxyV2Header(netip.MustParseAddr("203.0.113.9"), netip.MustParseAddr("127.0.0.1"), 49152, 22)
	if _, err := raw.Write(header); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = ssh.NewClientConn(raw, addr, &ssh.ClientConfig{
		User: "srv-abcdeabcdeabcdeabcde", Auth: []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 2 * time.Second,
	})
	if err == nil {
		t.Fatal("handshake completed with an un-stripped PROXY v2 header; the regression is not reproduced")
	}

	// The gateway's readVersion never finds a valid SSH- line in the header bytes
	// and times out; poll for the recorded failure (server-side, deadline-driven).
	deadline := time.Now().Add(2 * time.Second)
	for {
		if gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_handshakes_total", map[string]string{"result": "failed"}) >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("handshakes_total{result=failed} was never recorded for the broken handshake")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if v := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_handshakes_total", map[string]string{"result": "established"}); v != 0 {
		t.Fatalf("handshakes_total{result=established} = %v, want 0 for a broken edge", v)
	}
}
