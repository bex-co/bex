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
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeConnMetadata struct {
	user string
}

func (m fakeConnMetadata) User() string        { return m.user }
func (fakeConnMetadata) SessionID() []byte     { return nil }
func (fakeConnMetadata) ClientVersion() []byte { return []byte("SSH-2.0-test") }
func (fakeConnMetadata) ServerVersion() []byte { return []byte("SSH-2.0-bex") }
func (fakeConnMetadata) RemoteAddr() net.Addr  { return fakeAddr("remote") }
func (fakeConnMetadata) LocalAddr() net.Addr   { return fakeAddr("local") }

type fakeAddr string

func (a fakeAddr) Network() string { return "tcp" }
func (a fakeAddr) String() string  { return string(a) }

func signer(t *testing.T) ssh.Signer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func rsaSigner(t *testing.T, algorithms ...string) ssh.Signer {
	t.Helper()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	base, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	algorithmSigner, ok := base.(ssh.AlgorithmSigner)
	if !ok {
		t.Fatal("RSA signer does not support selectable signature algorithms")
	}
	signer, err := ssh.NewSignerWithAlgorithms(algorithmSigner, algorithms)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func startGateway(t *testing.T, st *gatewaytest.FakeStore, resolver *gatewaytest.FakeResolver, executor sshgateway.Executor, clientSigner ssh.Signer) (string, context.CancelFunc) {
	return startGatewayConfigured(t, st, resolver, executor, clientSigner, nil)
}

func startGatewayConfigured(t *testing.T, st *gatewaytest.FakeStore, resolver *gatewaytest.FakeResolver, executor sshgateway.Executor, clientSigner ssh.Signer, configure func(*Server)) (string, context.CancelFunc) {
	t.Helper()
	st.Key = store.SSHKey{Subject: "user-1", Fingerprint: ssh.FingerprintSHA256(clientSigner.PublicKey())}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{Store: st, Apps: resolver, Executor: executor, Signer: signer(t), SessionTimeout: time.Minute}
	if configure != nil {
		configure(server)
	}
	go func() { _ = server.Serve(ctx, listener) }()
	return listener.Addr().String(), func() { cancel(); _ = listener.Close() }
}

func dialGateway(addr, username string, clientSigner ssh.Signer) (*ssh.Client, error) {
	return ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User: username, Auth: []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 2 * time.Second,
	})
}

func TestGatewayCryptographicPolicyRejectsLegacyRSASHA1(t *testing.T) {
	clientSigner := signer(t)
	config, err := (&Server{
		Store: &gatewaytest.FakeStore{Key: store.SSHKey{
			Subject: "user-1", Fingerprint: ssh.FingerprintSHA256(clientSigner.PublicKey()),
		}},
		Apps: &gatewaytest.FakeResolver{}, Executor: &gatewaytest.FakeExecutor{}, Signer: signer(t),
	}).config(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	algorithms := make(map[string]bool, len(config.PublicKeyAuthAlgorithms))
	for _, algorithm := range config.PublicKeyAuthAlgorithms {
		algorithms[algorithm] = true
	}
	if algorithms[ssh.KeyAlgoRSA] {
		t.Fatal("legacy ssh-rsa/SHA-1 authentication must remain disabled")
	}
	for _, required := range []string{
		ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512,
		ssh.KeyAlgoSKED25519, ssh.KeyAlgoSKECDSA256,
	} {
		if !algorithms[required] {
			t.Fatalf("RSA SHA-2 algorithm %q is missing", required)
		}
	}
}

func TestGatewayAcceptsRSASHA2AndRejectsRSASHA1OnTheWire(t *testing.T) {
	for _, tc := range []struct {
		name      string
		algorithm string
		wantDial  bool
	}{
		{name: "RSA SHA-256", algorithm: ssh.KeyAlgoRSASHA256, wantDial: true},
		{name: "legacy RSA SHA-1", algorithm: ssh.KeyAlgoRSA, wantDial: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clientSigner := rsaSigner(t, tc.algorithm)
			addr, stop := startGateway(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}, clientSigner)
			defer stop()
			client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
			if client != nil {
				_ = client.Close()
			}
			if tc.wantDial && err != nil {
				t.Fatalf("RSA SHA-2 authentication failed: %v", err)
			}
			if !tc.wantDial && err == nil {
				t.Fatal("legacy RSA SHA-1 authentication succeeded")
			}
		})
	}
}

func TestKeyDeletedDuringHandshakeIsRecheckedBeforeTargetAuthorization(t *testing.T) {
	clientSigner := signer(t)
	st := &gatewaytest.FakeStore{Key: store.SSHKey{
		Subject: "user-1", Fingerprint: ssh.FingerprintSHA256(clientSigner.PublicKey()),
	}}
	resolver := &gatewaytest.FakeResolver{}
	config, err := (&Server{
		Store: st, Apps: resolver, Executor: &gatewaytest.FakeExecutor{}, Signer: signer(t),
	}).config(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	metadata := fakeConnMetadata{user: "srv-abcdeabcdeabcdeabcde"}
	permissions, err := config.PublicKeyCallback(metadata, clientSigner.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	st.Missing = true // deletion races after the initial offer was accepted
	if _, err := config.VerifiedPublicKeyCallback(metadata, clientSigner.PublicKey(), permissions, ""); err == nil {
		t.Fatal("deleted key completed signature verification")
	}
	if resolver.Username != "" {
		t.Fatalf("deleted key reached target authorization for %q", resolver.Username)
	}
}

func TestAuthenticatedConnectionWithoutChannelTimesOutAndReleasesLimit(t *testing.T) {
	clientSigner := signer(t)
	addr, stop := startGatewayConfigured(
		t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}, clientSigner,
		func(server *Server) {
			server.SessionTimeout = 50 * time.Millisecond
			server.Limits = sshgateway.NewSessionLimiter(1, 0)
		},
	)
	defer stop()

	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	go func() { waited <- client.Wait() }()
	select {
	case <-waited:
	case <-time.After(time.Second):
		t.Fatal("authenticated connection without a channel outlived session timeout")
	}
	_ = client.Close()

	// The timed-out connection must release the global slot, not merely close
	// its socket while leaving capacity permanently consumed.
	second, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatalf("connection after idle timeout: %v", err)
	}
	_ = second.Close()
}

func TestGatewayPreAuthConnectionCapShedsExcessBeforeHandshake(t *testing.T) {
	clientSigner := signer(t)
	addr, stop := startGatewayConfigured(
		t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}, clientSigner,
		func(server *Server) {
			server.MaxPreAuthConns = 1
			server.HandshakeTimeout = 2 * time.Second
		},
	)
	defer stop()

	// Hold the single pre-auth slot with a silent connection that never sends its
	// SSH version string, so it stays inside the handshake until the deadline. An
	// admitted connection receives the server banner; wait for it so the slot is
	// provably held before the second dial.
	hog, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer hog.Close()
	_ = hog.SetReadDeadline(time.Now().Add(time.Second))
	banner := make([]byte, 4)
	if _, err := io.ReadFull(hog, banner); err != nil || string(banner) != "SSH-" {
		t.Fatalf("first connection was not admitted (banner=%q err=%v)", banner, err)
	}

	// A second connection must be shed immediately: accepted then closed with no
	// banner, because the only pre-auth slot is occupied. If the cap were absent,
	// the server would enter ssh.NewServerConn and write its banner here too.
	shed, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer shed.Close()
	_ = shed.SetReadDeadline(time.Now().Add(time.Second))
	n, err := shed.Read(make([]byte, 1))
	if err == nil {
		t.Fatalf("second connection read %d byte(s); expected an immediate close (was not shed)", n)
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		t.Fatal("second connection was held through the handshake window; pre-auth cap did not shed it")
	}
}

func TestGatewayExecPropagatesIdentityOutputExitAndAudit(t *testing.T) {
	clientSigner := signer(t)
	st, resolver, executor := &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{Code: 7}
	addr, stop := startGateway(t, st, resolver, executor, clientSigner)
	defer stop()
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	output, err := session.CombinedOutput("printf private-command")
	var exitErr *ssh.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitStatus() != 7 {
		t.Fatalf("exec error = %v, want SSH exit 7", err)
	}
	if string(output) != "inside-app\n" || resolver.Subject != "user-1" || resolver.Username != "srv-abcdeabcdeabcdeabcde" {
		t.Fatalf("output/identity/target = %q %q %q", output, resolver.Subject, resolver.Username)
	}
	if len(executor.Command) != 3 || executor.Command[0] != "/bin/sh" || executor.Command[2] != "printf private-command" {
		t.Fatalf("exec command = %#v", executor.Command)
	}
	time.Sleep(20 * time.Millisecond)
	started, ended := st.StartedSessions(), st.EndedSessions()
	if len(started) != 1 || started[0].Subject != "user-1" || started[0].ServiceID == "" || len(ended) != 1 {
		t.Fatalf("audit start/end = %+v / %+v", started, ended)
	}
}

func TestGatewayPTYResizeAndShell(t *testing.T) {
	clientSigner := signer(t)
	executor := &gatewaytest.FakeExecutor{}
	addr, stop := startGateway(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, executor, clientSigner)
	defer stop()
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde-pod01", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, _ := client.NewSession()
	if err := session.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{}); err != nil {
		t.Fatal(err)
	}
	if err := session.Shell(); err != nil {
		t.Fatal(err)
	}
	if err := session.WindowChange(40, 120); err != nil {
		t.Fatal(err)
	}
	if err := session.Wait(); err != nil {
		t.Fatal(err)
	}
	if !executor.TTY || len(executor.Sizes) != 2 || executor.Sizes[0].Width != 80 || executor.Sizes[1].Width != 120 {
		t.Fatalf("tty/sizes = %v %+v", executor.TTY, executor.Sizes)
	}
}

func TestGatewayRejectsTerminalDimensionsThatWouldOverflowKubernetes(t *testing.T) {
	clientSigner := signer(t)
	executor := &gatewaytest.FakeExecutor{}
	addr, stop := startGateway(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, executor, clientSigner)
	defer stop()
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.RequestPty("xterm", 24, 70000, ssh.TerminalModes{}); err == nil {
		t.Fatal("oversized PTY width was accepted")
	}
	if len(executor.Command) != 0 {
		t.Fatalf("oversized PTY reached executor: %#v", executor.Command)
	}
}

func TestGatewayShelllessImageReturnsBoundedExit126(t *testing.T) {
	clientSigner := signer(t)
	executor := &gatewaytest.FakeExecutor{Err: errors.New("container exec failed: executable file not found")}
	addr, stop := startGateway(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, executor, clientSigner)
	defer stop()
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	output, err := session.CombinedOutput("true")
	var exitErr *ssh.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitStatus() != 126 {
		t.Fatalf("shellless exec error = %v, want SSH exit 126", err)
	}
	if got := string(output); got != "unable to start /bin/sh in this image\n" {
		t.Fatalf("shellless output = %q", got)
	}
}

func TestGatewayRejectsDeletedKeyForbiddenTargetAndForwarding(t *testing.T) {
	clientSigner := signer(t)
	for name, tc := range map[string]struct {
		st       *gatewaytest.FakeStore
		resolver *gatewaytest.FakeResolver
	}{
		"deleted key": {&gatewaytest.FakeStore{Missing: true}, &gatewaytest.FakeResolver{}},
		"forbidden":   {&gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{Err: core.ErrForbidden}},
	} {
		t.Run(name, func(t *testing.T) {
			addr, stop := startGateway(t, tc.st, tc.resolver, &gatewaytest.FakeExecutor{}, clientSigner)
			defer stop()
			if client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner); err == nil {
				client.Close()
				t.Fatal("authentication unexpectedly succeeded")
			}
		})
	}

	addr, stop := startGateway(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}, clientSigner)
	defer stop()
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, _, err := client.OpenChannel("direct-tcpip", nil); err == nil {
		t.Fatal("direct-tcpip unexpectedly accepted")
	}
}

func TestGatewayRejectsUnsupportedRequestsBeforeExec(t *testing.T) {
	clientSigner := signer(t)
	executor := &gatewaytest.FakeExecutor{}
	addr, stop := startGateway(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, executor, clientSigner)
	defer stop()
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Setenv("SHOULD_NOT_REACH_APP", "secret"); err == nil {
		t.Fatal("environment request unexpectedly accepted")
	}
	if err := session.RequestSubsystem("sftp"); err == nil {
		t.Fatal("SFTP subsystem unexpectedly accepted")
	}
	output, err := session.CombinedOutput("true")
	if err != nil || string(output) != "inside-app\n" {
		t.Fatalf("valid exec after rejected requests = %q, %v", output, err)
	}
	if len(executor.Command) != 3 || executor.Command[2] != "true" {
		t.Fatalf("executor received unexpected command: %#v", executor.Command)
	}
}

func TestGatewayRejectsSCPProtocolBeforeExec(t *testing.T) {
	for _, command := range []string{"scp -t /tmp/file", "/usr/bin/scp -qpf /tmp/file"} {
		t.Run(command, func(t *testing.T) {
			clientSigner := signer(t)
			executor := &gatewaytest.FakeExecutor{}
			addr, stop := startGateway(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, executor, clientSigner)
			defer stop()
			client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			session, err := client.NewSession()
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			if err := session.Run(command); err == nil {
				t.Fatal("SCP protocol command unexpectedly accepted")
			}
			if executor.Command != nil {
				t.Fatalf("SCP protocol reached executor: %#v", executor.Command)
			}
		})
	}
}

func TestSCPCommandClassifierDoesNotBlockOrdinaryShellCommands(t *testing.T) {
	for _, command := range []string{"echo scp -t /tmp", "scp --help", "my-scp -t /tmp", "scp /tmp/a /tmp/b"} {
		if isSCPProtocolCommand(command) {
			t.Fatalf("ordinary command %q classified as SCP protocol", command)
		}
	}
}

func TestGatewayRejectsAdditionalSessionWhileFirstIsRunning(t *testing.T) {
	clientSigner := signer(t)
	executor := &gatewaytest.FakeExecutor{Block: true, Started: make(chan struct{}, 1), Stopped: make(chan error, 1)}
	addr, stop := startGateway(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, executor, clientSigner)
	defer stop()
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	first, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Start("hold"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-executor.Started:
	case <-time.After(time.Second):
		t.Fatal("first exec did not start")
	}
	secondResult := make(chan error, 1)
	go func() {
		second, err := client.NewSession()
		if second != nil {
			_ = second.Close()
		}
		secondResult <- err
	}()
	select {
	case err := <-secondResult:
		if err == nil {
			t.Fatal("second session channel unexpectedly accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("second session channel was queued instead of rejected")
	}
	_ = client.Close()
	select {
	case <-executor.Stopped:
	case <-time.After(time.Second):
		t.Fatal("closing the connection did not cancel the first exec")
	}
}

func TestGatewayCancelsExecOnTimeoutAndDisconnect(t *testing.T) {
	for _, tc := range []struct {
		name       string
		timeout    time.Duration
		disconnect bool
	}{
		{name: "session timeout", timeout: 40 * time.Millisecond},
		{name: "client disconnect", timeout: time.Minute, disconnect: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clientSigner := signer(t)
			executor := &gatewaytest.FakeExecutor{Block: true, Started: make(chan struct{}, 1), Stopped: make(chan error, 1)}
			addr, stop := startGatewayConfigured(t, &gatewaytest.FakeStore{}, &gatewaytest.FakeResolver{}, executor, clientSigner, func(server *Server) {
				server.SessionTimeout = tc.timeout
			})
			defer stop()
			client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
			if err != nil {
				t.Fatal(err)
			}
			session, err := client.NewSession()
			if err != nil {
				t.Fatal(err)
			}
			if err := session.Start("hold"); err != nil {
				t.Fatal(err)
			}
			select {
			case <-executor.Started:
			case <-time.After(time.Second):
				t.Fatal("exec did not start")
			}
			if tc.disconnect {
				_ = client.Close()
			}
			select {
			case err := <-executor.Stopped:
				if err == nil {
					t.Fatal("executor stopped without context cancellation")
				}
			case <-time.After(time.Second):
				t.Fatal("executor was not cancelled")
			}
			_ = client.Close()
		})
	}
}

// TestServeConnHonorsPROXYHeaderFromTrustedPeer proves w4/029.md #10's fix: a
// PROXY v1 header from a peer inside TrustedProxies (standing in for
// Traefik's ssh entrypoint forwarding a real client via
// lego/operator/config/ssh/ingressroutetcp.yaml's proxyProtocol) makes the
// session audit record the ORIGINAL client, not the immediate TCP peer.
func TestServeConnHonorsPROXYHeaderFromTrustedPeer(t *testing.T) {
	clientSigner := signer(t)
	st := &gatewaytest.FakeStore{}
	addr, stop := startGatewayConfigured(t, st, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}, clientSigner, func(server *Server) {
		server.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	})
	defer stop()

	raw, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Write([]byte("PROXY TCP4 203.0.113.9 127.0.0.1 49152 22\r\n")); err != nil {
		t.Fatal(err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(raw, addr, &ssh.ClientConfig{
		User: "srv-abcdeabcdeabcdeabcde", Auth: []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	_ = session.Close()

	time.Sleep(20 * time.Millisecond)
	started := st.StartedSessions()
	if len(started) != 1 || !strings.HasPrefix(started[0].RemoteAddress, "203.0.113.9:") {
		t.Fatalf("started sessions = %+v, want RemoteAddress from the PROXY header", started)
	}
}

// TestServeConnKeepsImmediatePeerWithoutTrustedProxies proves the default
// (unconfigured TrustedProxies) behavior is unchanged: a raw PROXY-shaped
// prefix is treated as opaque SSH banner bytes, not parsed, so the connection
// fails exactly as it would against any pre-proxyproto gateway.
func TestServeConnKeepsImmediatePeerWithoutTrustedProxies(t *testing.T) {
	clientSigner := signer(t)
	st := &gatewaytest.FakeStore{}
	addr, stop := startGateway(t, st, &gatewaytest.FakeResolver{}, &gatewaytest.FakeExecutor{}, clientSigner)
	defer stop()

	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	_ = session.Close()

	time.Sleep(20 * time.Millisecond)
	started := st.StartedSessions()
	if len(started) != 1 || strings.HasPrefix(started[0].RemoteAddress, "203.0.113.9:") {
		t.Fatalf("started sessions = %+v, want the immediate 127.0.0.1 peer", started)
	}
	if !strings.HasPrefix(started[0].RemoteAddress, "127.0.0.1:") {
		t.Fatalf("started sessions = %+v, want the immediate 127.0.0.1 peer", started)
	}
}
