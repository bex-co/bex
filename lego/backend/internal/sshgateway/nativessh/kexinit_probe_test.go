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
	"net"
	"os"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// TestProbeKEXINITAgainstLiveGateway is the protocol-level assertion whose
// absence defined w6/m132: the real gateway, driven by the same server code
// production runs, sends SSH_MSG_KEXINIT after the version exchange. No key,
// fixture, or eligible service is involved — the check is pre-authentication.
func TestProbeKEXINITAgainstLiveGateway(t *testing.T) {
	clientSigner := signer(t)
	addr, stop := startGateway(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}, clientSigner)
	defer stop()

	if err := ProbeKEXINIT(addr, 2*time.Second); err != nil {
		t.Fatalf("ProbeKEXINIT against a healthy gateway: %v", err)
	}
}

// TestProbeKEXINITDetectsSilentEdge reproduces the regression: a server that
// writes its banner and then never sends KEXINIT (the exact w6/m132 symptom, and
// what an un-stripped PROXY header did to the version exchange). The probe must
// fail — this is what makes the scheduled synthetic loud instead of a dead edge
// going unnoticed for weeks.
func TestProbeKEXINITDetectsSilentEdge(t *testing.T) {
	addr, stop := fakeSSHListener(t, func(conn net.Conn, hold <-chan struct{}) {
		_, _ = conn.Write([]byte("SSH-2.0-bex\r\n"))
		<-hold // banner written, then silence — never a KEXINIT
	})
	defer stop()

	if err := ProbeKEXINIT(addr, 400*time.Millisecond); err == nil {
		t.Fatal("ProbeKEXINIT passed against a banner-then-silence edge; it must detect the dead handshake")
	}
}

// TestProbeKEXINITDetectsWrongFirstPacket covers a server that speaks SSH framing
// but whose first packet is not KEXINIT — the probe checks the message type, not
// merely that some bytes arrived.
func TestProbeKEXINITDetectsWrongFirstPacket(t *testing.T) {
	addr, stop := fakeSSHListener(t, func(conn net.Conn, hold <-chan struct{}) {
		_, _ = conn.Write([]byte("SSH-2.0-bex\r\n"))
		// A minimal, well-formed unencrypted packet whose payload is a single
		// non-KEXINIT message type (SSH_MSG_IGNORE = 2): length=6, pad_len=4,
		// type=2, then 4 padding bytes.
		_, _ = conn.Write([]byte{0, 0, 0, 6, 4, 2, 0, 0, 0, 0})
		<-hold
	})
	defer stop()

	if err := ProbeKEXINIT(addr, 400*time.Millisecond); err == nil {
		t.Fatal("ProbeKEXINIT passed on a non-KEXINIT first packet; it must check the message type")
	}
}

// TestProbeKEXINITDetectsNoBanner covers a peer that accepts the TCP connection
// and immediately hangs up without speaking SSH.
func TestProbeKEXINITDetectsNoBanner(t *testing.T) {
	addr, stop := fakeSSHListener(t, func(conn net.Conn, _ <-chan struct{}) {
		_ = conn.Close()
	})
	defer stop()

	if err := ProbeKEXINIT(addr, 400*time.Millisecond); err == nil {
		t.Fatal("ProbeKEXINIT passed against a peer that never sent an SSH banner")
	}
}

// TestPublicGatewayKEXINITLiveness is the scheduled synthetic itself: pointed at
// a real endpoint via BEX_TEST_SSH_KEXINIT_ADDR (ssh.bex.co:22 in the guard
// workflow, scripts/ssh-kexinit-probe.sh), it asserts the live edge reaches
// KEXINIT. Skipped when the address is unset so ordinary `go test ./...` never
// dials the public internet.
func TestPublicGatewayKEXINITLiveness(t *testing.T) {
	address := os.Getenv("BEX_TEST_SSH_KEXINIT_ADDR")
	if address == "" {
		t.Skip("set BEX_TEST_SSH_KEXINIT_ADDR=host:port to probe a live SSH edge")
	}
	if err := ProbeKEXINIT(address, 10*time.Second); err != nil {
		t.Fatalf("live SSH edge %s did not reach KEXINIT: %v", address, err)
	}
}

// fakeSSHListener accepts one connection and runs handle against it. hold is
// closed on teardown so a handler can keep a connection open (holding a banner
// without ever sending KEXINIT) for the duration of the test.
func fakeSSHListener(t *testing.T, handle func(conn net.Conn, hold <-chan struct{})) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	hold := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		handle(conn, hold)
	}()
	return listener.Addr().String(), func() {
		close(hold)
		_ = listener.Close()
	}
}
