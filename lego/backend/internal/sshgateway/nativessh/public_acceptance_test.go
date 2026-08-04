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
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// TestPublicGatewayPTYResize is an opt-in, redacting public-edge probe used by
// scripts/ssh-verify.sh. It proves the real endpoint allocates a PTY, forwards a
// resize, and exposes the disposable service's runtime environment. It never
// prints terminal output or key material, even on failure.
func TestPublicGatewayPTYResize(t *testing.T) {
	addr := os.Getenv("BEX_TEST_SSH_PUBLIC_ADDR")
	username := os.Getenv("BEX_TEST_SSH_PUBLIC_USER")
	keyFile := os.Getenv("BEX_TEST_SSH_PRIVATE_KEY_FILE")
	wantFingerprint := os.Getenv("BEX_TEST_SSH_HOST_FINGERPRINT")
	if addr == "" || username == "" || keyFile == "" || wantFingerprint == "" {
		t.Skip("public SSH acceptance environment is not configured")
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatal("read disposable client key")
	}
	signer, err := ssh.ParsePrivateKey(keyPEM)
	keyPEM = nil
	if err != nil {
		t.Fatal("parse disposable client key")
	}
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			if ssh.FingerprintSHA256(key) != wantFingerprint {
				return fmt.Errorf("public host fingerprint mismatch")
			}
			return nil
		},
		Timeout: 10 * time.Second,
	}
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		t.Fatal("dial public SSH gateway")
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal("open public SSH session")
	}
	defer session.Close()
	stdin, err := session.StdinPipe()
	if err != nil {
		t.Fatal("attach public SSH stdin")
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		t.Fatal("attach public SSH stdout")
	}
	if err := session.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{ssh.ECHO: 0}); err != nil {
		t.Fatal("request public SSH PTY")
	}
	if err := session.Shell(); err != nil {
		t.Fatal("start public SSH shell")
	}
	lines := make(chan string, 16)
	scanDone := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			scanDone <- err
		} else {
			scanDone <- io.EOF
		}
	}()
	waitForLine := func(want string, timeout time.Duration) error {
		deadline := time.NewTimer(timeout)
		defer deadline.Stop()
		for {
			select {
			case line := <-lines:
				if strings.Contains(line, want) {
					return nil
				}
			case err := <-scanDone:
				return err
			case <-deadline.C:
				return fmt.Errorf("timed out waiting for redacted PTY marker")
			}
		}
	}
	const readyMarker = "__BEX_M39_PTY_READY__"
	command := `test "$BEX_SSH_SMOKE_VALUE" = "w2-m39-runtime" || exit 42; echo ` + readyMarker
	if _, err := fmt.Fprintln(stdin, command); err != nil {
		t.Fatal("drive public SSH shell")
	}
	if err := waitForLine(readyMarker, 10*time.Second); err != nil {
		t.Fatal("public SSH shell or runtime environment check failed")
	}
	// The readiness marker proves the trap is installed in the attached app
	// PTY, so this request cannot be mistaken for the initial terminal size.
	if err := session.WindowChange(41, 113); err != nil {
		t.Fatal("resize public SSH PTY")
	}
	time.Sleep(time.Second)
	if _, err := fmt.Fprintln(stdin, `stty size; exit`); err != nil {
		t.Fatal("inspect resized public SSH PTY")
	}
	if err := waitForLine("41 113", 10*time.Second); err != nil {
		t.Fatal("public SSH terminal resize was not observed")
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- session.Wait() }()
	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatal("public SSH shell or runtime environment check failed")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("public SSH shell did not exit after resize")
	}
}
