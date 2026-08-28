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
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// sshMsgKexInit is the SSH_MSG_KEXINIT packet type (RFC 4253 §12). A conforming
// server sends it as its first packet, immediately after the version exchange.
const sshMsgKexInit = 20

// ProbeKEXINIT is a credential-free liveness check of an SSH endpoint: it
// completes the RFC 4253 version exchange and asserts the server's first packet
// is SSH_MSG_KEXINIT (type 20), exactly as github.com and gitlab.com do under an
// identical probe. That assertion is the whole of w6/m132 — a correct server
// sends KEXINIT the moment the version exchange completes; the regressed gateway
// (a Traefik PROXY header the gateway was not configured to strip, fed into the
// version exchange) sent 0 bytes after its banner and the handshake timed out.
//
// It needs no key, fixture, or eligible service, so it can run as an always-on
// synthetic (scripts/ssh-kexinit-probe.sh, wired to a scheduled workflow)
// against ssh.bex.co:22 without authenticating, opening a session, or touching a
// tenant workload — the failure it detects is pre-authentication and affects
// every connection alike, so an unauthenticated probe is sufficient and honest.
//
// A returned error is the honest failure: "no banner" (nothing spoke SSH),
// "read first packet: i/o timeout" (the exact dead-edge symptom — banner, then
// silence to the deadline), or a wrong first packet type. nil means the edge is
// live to KEXINIT.
func ProbeKEXINIT(address string, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return fmt.Errorf("dial %s: %w", address, err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	reader := bufio.NewReader(conn)
	banner, err := readIdentification(reader)
	if err != nil {
		return fmt.Errorf("read server identification: %w", err)
	}
	// Send a client identification string so a server that waits for it (Go's
	// ssh.NewServerConn does) advances to the key exchange. Its content is
	// irrelevant to the check; RFC 4253 §4.2.
	if _, err := conn.Write([]byte("SSH-2.0-bex_kexinit_probe\r\n")); err != nil {
		return fmt.Errorf("write client identification: %w", err)
	}
	msgType, err := readFirstPacketType(reader)
	if err != nil {
		return fmt.Errorf("after banner %q, read first packet: %w", banner, err)
	}
	if msgType != sshMsgKexInit {
		return fmt.Errorf("after banner %q, first packet type = %d, want %d (SSH_MSG_KEXINIT)", banner, msgType, sshMsgKexInit)
	}
	return nil
}

// readIdentification reads the server's SSH identification string, skipping any
// preamble lines that do not begin with "SSH-" (RFC 4253 §4.2 permits them),
// bounded so a chatty or non-SSH peer cannot stream forever.
func readIdentification(r *bufio.Reader) (string, error) {
	for lines := 0; lines < 50; lines++ {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		if line = strings.TrimRight(line, "\r\n"); strings.HasPrefix(line, "SSH-") {
			return line, nil
		}
	}
	return "", fmt.Errorf("no SSH- identification line in the first 50 lines")
}

// readFirstPacketType reads one unencrypted binary packet (RFC 4253 §6 — valid
// before keys are established, which is exactly where KEXINIT lives) and returns
// its message type. The blocking read on a dead edge is what surfaces the
// dead-handshake symptom as a deadline error rather than a false pass.
func readFirstPacketType(r *bufio.Reader) (byte, error) {
	var lengthBytes [4]byte
	if _, err := io.ReadFull(r, lengthBytes[:]); err != nil {
		return 0, err
	}
	// packet_length counts padding_length + payload + padding (min 1+1+4).
	packetLength := binary.BigEndian.Uint32(lengthBytes[:])
	if packetLength < 6 || packetLength > 35000 {
		return 0, fmt.Errorf("implausible packet length %d", packetLength)
	}
	// The byte after packet_length is padding_length; the next is the first
	// payload byte, which is the message type.
	var header [2]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, err
	}
	return header[1], nil
}
