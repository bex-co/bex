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
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

func sandboxTarget() apps.SSHInstanceTarget {
	return apps.SSHInstanceTarget{
		ID: "ags-abcdeabcdeabcdeabcde", ServiceID: "ags-abcdeabcdeabcdeabcde",
		OwnerID: "tea-a", Namespace: "tea-a-sandbox", PodName: "os-x-0",
		Container: "sandbox", Sandbox: true,
	}
}

// countingExecutor tracks concurrent Execute calls and, when release is set,
// blocks each call until the channel is closed — so a test can observe several
// channels executing at once over one connection.
type countingExecutor struct {
	mu             sync.Mutex
	active, max, n int
	release        chan struct{}
}

func (e *countingExecutor) enter() {
	e.mu.Lock()
	e.active++
	e.n++
	if e.active > e.max {
		e.max = e.active
	}
	e.mu.Unlock()
}

func (e *countingExecutor) leave() {
	e.mu.Lock()
	e.active--
	e.mu.Unlock()
}

func (e *countingExecutor) peakConcurrency() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.max
}

func (e *countingExecutor) activeNow() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.active
}

func (e *countingExecutor) Execute(ctx context.Context, _ apps.SSHInstanceTarget, _ []string, tty bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, stdout, _ io.Writer) (int, error) {
	e.enter()
	defer e.leave()
	if e.release != nil {
		select {
		case <-e.release:
		case <-ctx.Done():
			return 126, ctx.Err()
		}
	}
	_, _ = io.WriteString(stdout, "ok\n")
	return 0, nil
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within 2s")
}

// A sandbox target multiplexes several concurrent session channels over one
// connection (Zed's ControlMaster remoting), each its own exec stream — the
// exact thing the single-channel App path rejects (ADR054 D3).
func TestSandboxConnectionMultiplexesChannels(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	resolver := &gatewaytest.FakeResolver{Target: sandboxTarget()}
	addr, stop := startGatewayConfigured(t, &gatewaytest.FakeStore{}, resolver, exec, clientSigner,
		func(s *Server) { s.MaxChannelsPerConn = 4 })
	defer stop()

	client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session, err := client.NewSession()
			if err != nil {
				return
			}
			defer session.Close()
			_ = session.Run("work")
		}()
	}
	// All three execs must be in flight at once — impossible on the single-channel
	// path, which serializes behind the first.
	waitFor(t, func() bool { return exec.activeNow() == 3 })
	close(exec.release)
	wg.Wait()

	if exec.peakConcurrency() != 3 {
		t.Fatalf("peak concurrency = %d, want 3", exec.peakConcurrency())
	}
}

// The per-connection channel cap sheds an over-budget channel without tearing
// down the connection or its live channels.
func TestSandboxChannelCapShedsExcess(t *testing.T) {
	clientSigner := signer(t)
	resolver := &gatewaytest.FakeResolver{Target: sandboxTarget()}
	addr, stop := startGatewayConfigured(t, &gatewaytest.FakeStore{}, resolver, &gatewaytest.FakeExecutor{}, clientSigner,
		func(s *Server) { s.MaxChannelsPerConn = 2 })
	defer stop()

	client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Two idle session channels fill the cap (each holds its slot until closed).
	s1, err := client.NewSession()
	if err != nil {
		t.Fatalf("channel 1 should be accepted: %v", err)
	}
	defer s1.Close()
	s2, err := client.NewSession()
	if err != nil {
		t.Fatalf("channel 2 should be accepted: %v", err)
	}
	defer s2.Close()
	if _, err := client.NewSession(); err == nil {
		t.Fatal("channel 3 should be shed once the per-connection cap is reached")
	}
}

// SFTP is honored for a sandbox target so Zed can upload its remote-server, and
// it does not depend on the multi-channel cap (works on the single-channel
// fallback too). App targets keep rejecting every subsystem.
func TestSandboxAcceptsSFTPSubsystem(t *testing.T) {
	clientSigner := signer(t)
	exec := &gatewaytest.FakeExecutor{}
	resolver := &gatewaytest.FakeResolver{Target: sandboxTarget()}
	// MaxChannelsPerConn left 0 => single-channel fallback; SFTP still honored.
	addr, stop := startGateway(t, &gatewaytest.FakeStore{}, resolver, exec, clientSigner)
	defer stop()

	client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := session.RequestSubsystem("sftp"); err != nil {
		t.Fatalf("sftp subsystem should be accepted for a sandbox target: %v", err)
	}
	if len(exec.Command) != 1 || exec.Command[0] != "/usr/lib/openssh/sftp-server" {
		t.Fatalf("sftp exec argv = %#v, want [/usr/lib/openssh/sftp-server]", exec.Command)
	}
	_ = session.Close()
}

// A non-sftp subsystem stays rejected even on a sandbox target — only the exact
// `sftp` name is bridged.
func TestSandboxRejectsNonSFTPSubsystem(t *testing.T) {
	clientSigner := signer(t)
	resolver := &gatewaytest.FakeResolver{Target: sandboxTarget()}
	addr, stop := startGateway(t, &gatewaytest.FakeStore{}, resolver, &gatewaytest.FakeExecutor{}, clientSigner)
	defer stop()

	client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.RequestSubsystem("exec-something"); err == nil {
		t.Fatal("only the sftp subsystem may be accepted on a sandbox target")
	}
}

// With the cap disabled (0), a sandbox target falls back to the single-channel
// contract: a second channel is rejected exactly like an App target.
func TestSandboxCapZeroRestoresSingleChannel(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	resolver := &gatewaytest.FakeResolver{Target: sandboxTarget()}
	addr, stop := startGatewayConfigured(t, &gatewaytest.FakeStore{}, resolver, exec, clientSigner,
		func(s *Server) { s.MaxChannelsPerConn = 0 })
	defer stop()

	client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	s1, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer s1.Close()
	go func() { _ = s1.Run("hold") }()
	waitFor(t, func() bool { return exec.activeNow() == 1 })

	if _, err := client.NewSession(); err == nil {
		t.Fatal("second channel should be rejected when the multi-channel cap is disabled")
	}
	close(exec.release)
}

