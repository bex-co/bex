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

package sshgateway

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeGatewayStore struct {
	key     store.SSHKey
	missing bool
	mu      sync.Mutex
	started []store.SSHSessionAudit
	ended   []string
	// nonces mirrors the shared shell_ticket_nonces claim (w1/042 L7); share
	// ONE fakeGatewayStore between two Servers to model two gateway replicas.
	nonces   map[string]bool
	claimErr error
}

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

func (f *fakeGatewayStore) SSHKeyByFingerprint(context.Context, string) (store.SSHKey, error) {
	if f.missing {
		return store.SSHKey{}, store.ErrNotFound
	}
	return f.key, nil
}
func (f *fakeGatewayStore) StartSSHSession(_ context.Context, audit store.SSHSessionAudit) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, audit)
	return nil
}
func (f *fakeGatewayStore) EndSSHSession(_ context.Context, id, result string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ended = append(f.ended, id+":"+result)
	return nil
}
func (f *fakeGatewayStore) ClaimShellNonce(_ context.Context, nonce string, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimErr != nil {
		return false, f.claimErr
	}
	if f.nonces == nil {
		f.nonces = map[string]bool{}
	}
	if f.nonces[nonce] {
		return false, nil
	}
	f.nonces[nonce] = true
	return true, nil
}

type fakeResolver struct {
	err      error
	subject  string
	username string
	target   apps.SSHInstanceTarget
}

func (f *fakeResolver) ResolveSSHSession(ctx context.Context, username string) (apps.SSHInstanceTarget, error) {
	identity, _ := core.IdentityFrom(ctx)
	f.subject, f.username = identity.Subject, username
	if f.err != nil {
		return apps.SSHInstanceTarget{}, f.err
	}
	if f.target.ID != "" {
		return f.target, nil
	}
	return apps.SSHInstanceTarget{
		ID: "srv-abcdeabcdeabcdeabcde-pod01", ServiceID: "srv-abcdeabcdeabcdeabcde",
		OwnerID: "tea-workspace", Namespace: "default", PodName: "web-rs-pod01", Container: core.AppContainer,
	}, nil
}

type fakeExecutor struct {
	command []string
	tty     bool
	sizes   []remotecommand.TerminalSize
	code    int
	err     error
	block   bool
	started chan struct{}
	stopped chan error
}

func (f *fakeExecutor) Execute(ctx context.Context, _ apps.SSHInstanceTarget, command []string, tty bool, queue remotecommand.TerminalSizeQueue, _ io.Reader, stdout, _ io.Writer) (int, error) {
	f.command = append([]string(nil), command...)
	f.tty = tty
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	if f.block {
		<-ctx.Done()
		if f.stopped != nil {
			f.stopped <- ctx.Err()
		}
		return 126, ctx.Err()
	}
	if tty {
		if size := queue.Next(); size != nil {
			f.sizes = append(f.sizes, *size)
		}
		if size := queue.Next(); size != nil {
			f.sizes = append(f.sizes, *size)
		}
	}
	if f.err == nil {
		_, _ = io.WriteString(stdout, "inside-app\n")
	}
	return f.code, f.err
}

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

func startGateway(t *testing.T, st *fakeGatewayStore, resolver *fakeResolver, executor Executor, clientSigner ssh.Signer) (string, context.CancelFunc) {
	return startGatewayConfigured(t, st, resolver, executor, clientSigner, nil)
}

func startGatewayConfigured(t *testing.T, st *fakeGatewayStore, resolver *fakeResolver, executor Executor, clientSigner ssh.Signer, configure func(*Server)) (string, context.CancelFunc) {
	t.Helper()
	st.key = store.SSHKey{Subject: "user-1", Fingerprint: ssh.FingerprintSHA256(clientSigner.PublicKey())}
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
		Store: &fakeGatewayStore{key: store.SSHKey{
			Subject: "user-1", Fingerprint: ssh.FingerprintSHA256(clientSigner.PublicKey()),
		}},
		Apps: &fakeResolver{}, Executor: &fakeExecutor{}, Signer: signer(t),
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
			addr, stop := startGateway(t, &fakeGatewayStore{}, &fakeResolver{}, &fakeExecutor{}, clientSigner)
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
	st := &fakeGatewayStore{key: store.SSHKey{
		Subject: "user-1", Fingerprint: ssh.FingerprintSHA256(clientSigner.PublicKey()),
	}}
	resolver := &fakeResolver{}
	config, err := (&Server{
		Store: st, Apps: resolver, Executor: &fakeExecutor{}, Signer: signer(t),
	}).config(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	metadata := fakeConnMetadata{user: "srv-abcdeabcdeabcdeabcde"}
	permissions, err := config.PublicKeyCallback(metadata, clientSigner.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	st.missing = true // deletion races after the initial offer was accepted
	if _, err := config.VerifiedPublicKeyCallback(metadata, clientSigner.PublicKey(), permissions, ""); err == nil {
		t.Fatal("deleted key completed signature verification")
	}
	if resolver.username != "" {
		t.Fatalf("deleted key reached target authorization for %q", resolver.username)
	}
}

func TestAuthenticatedConnectionWithoutChannelTimesOutAndReleasesLimit(t *testing.T) {
	clientSigner := signer(t)
	addr, stop := startGatewayConfigured(
		t, &fakeGatewayStore{}, &fakeResolver{}, &fakeExecutor{}, clientSigner,
		func(server *Server) {
			server.SessionTimeout = 50 * time.Millisecond
			server.MaxSessions = 1
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

func TestGatewayExecPropagatesIdentityOutputExitAndAudit(t *testing.T) {
	clientSigner := signer(t)
	st, resolver, executor := &fakeGatewayStore{}, &fakeResolver{}, &fakeExecutor{code: 7}
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
	if string(output) != "inside-app\n" || resolver.subject != "user-1" || resolver.username != "srv-abcdeabcdeabcdeabcde" {
		t.Fatalf("output/identity/target = %q %q %q", output, resolver.subject, resolver.username)
	}
	if len(executor.command) != 3 || executor.command[0] != "/bin/sh" || executor.command[2] != "printf private-command" {
		t.Fatalf("exec command = %#v", executor.command)
	}
	time.Sleep(20 * time.Millisecond)
	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.started) != 1 || st.started[0].Subject != "user-1" || st.started[0].ServiceID == "" || len(st.ended) != 1 {
		t.Fatalf("audit start/end = %+v / %+v", st.started, st.ended)
	}
}

func TestGatewayPTYResizeAndShell(t *testing.T) {
	clientSigner := signer(t)
	executor := &fakeExecutor{}
	addr, stop := startGateway(t, &fakeGatewayStore{}, &fakeResolver{}, executor, clientSigner)
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
	if !executor.tty || len(executor.sizes) != 2 || executor.sizes[0].Width != 80 || executor.sizes[1].Width != 120 {
		t.Fatalf("tty/sizes = %v %+v", executor.tty, executor.sizes)
	}
}

func TestGatewayRejectsTerminalDimensionsThatWouldOverflowKubernetes(t *testing.T) {
	clientSigner := signer(t)
	executor := &fakeExecutor{}
	addr, stop := startGateway(t, &fakeGatewayStore{}, &fakeResolver{}, executor, clientSigner)
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
	if len(executor.command) != 0 {
		t.Fatalf("oversized PTY reached executor: %#v", executor.command)
	}
}

func TestGatewayShelllessImageReturnsBoundedExit126(t *testing.T) {
	clientSigner := signer(t)
	executor := &fakeExecutor{err: errors.New("container exec failed: executable file not found")}
	addr, stop := startGateway(t, &fakeGatewayStore{}, &fakeResolver{}, executor, clientSigner)
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
		st       *fakeGatewayStore
		resolver *fakeResolver
	}{
		"deleted key": {&fakeGatewayStore{missing: true}, &fakeResolver{}},
		"forbidden":   {&fakeGatewayStore{}, &fakeResolver{err: core.ErrForbidden}},
	} {
		t.Run(name, func(t *testing.T) {
			addr, stop := startGateway(t, tc.st, tc.resolver, &fakeExecutor{}, clientSigner)
			defer stop()
			if client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner); err == nil {
				client.Close()
				t.Fatal("authentication unexpectedly succeeded")
			}
		})
	}

	addr, stop := startGateway(t, &fakeGatewayStore{}, &fakeResolver{}, &fakeExecutor{}, clientSigner)
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
	executor := &fakeExecutor{}
	addr, stop := startGateway(t, &fakeGatewayStore{}, &fakeResolver{}, executor, clientSigner)
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
	if len(executor.command) != 3 || executor.command[2] != "true" {
		t.Fatalf("executor received unexpected command: %#v", executor.command)
	}
}

func TestGatewayRejectsSCPProtocolBeforeExec(t *testing.T) {
	for _, command := range []string{"scp -t /tmp/file", "/usr/bin/scp -qpf /tmp/file"} {
		t.Run(command, func(t *testing.T) {
			clientSigner := signer(t)
			executor := &fakeExecutor{}
			addr, stop := startGateway(t, &fakeGatewayStore{}, &fakeResolver{}, executor, clientSigner)
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
			if executor.command != nil {
				t.Fatalf("SCP protocol reached executor: %#v", executor.command)
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
	executor := &fakeExecutor{block: true, started: make(chan struct{}, 1), stopped: make(chan error, 1)}
	addr, stop := startGateway(t, &fakeGatewayStore{}, &fakeResolver{}, executor, clientSigner)
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
	case <-executor.started:
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
	case <-executor.stopped:
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
			executor := &fakeExecutor{block: true, started: make(chan struct{}, 1), stopped: make(chan error, 1)}
			addr, stop := startGatewayConfigured(t, &fakeGatewayStore{}, &fakeResolver{}, executor, clientSigner, func(server *Server) {
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
			case <-executor.started:
			case <-time.After(time.Second):
				t.Fatal("exec did not start")
			}
			if tc.disconnect {
				_ = client.Close()
			}
			select {
			case err := <-executor.stopped:
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

func TestSessionLimits(t *testing.T) {
	server := &Server{MaxSessions: 2, MaxPerIdentity: 1}
	server.defaults()
	a, _ := server.acquire("a")
	aAgain, identityScope := server.acquire("a")
	b, _ := server.acquire("b")
	c, globalScope := server.acquire("c")
	if !a || aAgain || identityScope != "identity" || !b || c || globalScope != "global" {
		t.Fatal("global/per-identity limits not enforced")
	}
	server.release("a")
	c, _ = server.acquire("c")
	if !c {
		t.Fatal("released capacity was not reusable")
	}
}
