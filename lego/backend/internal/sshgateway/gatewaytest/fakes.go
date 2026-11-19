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

// Package gatewaytest holds the shared test fakes for the sshgateway feature
// packages (root native SSH, webshell) so the fakes cannot drift between the
// suites. It deliberately avoids importing the sshgateway packages themselves
// — the fakes satisfy their interfaces structurally.
package gatewaytest

import (
	"context"
	"io"
	"sync"
	"time"

	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// FakeStore implements the gateway's Store and NonceStore surfaces. Its nonce
// map mirrors the shared shell_ticket_nonces claim (w1/042 L7); share ONE
// FakeStore between two servers to model two gateway replicas.
type FakeStore struct {
	Key      store.SSHKey
	Missing  bool
	ClaimErr error

	mu             sync.Mutex
	failKeyLookups bool
	started        []store.SSHSessionAudit
	ended          []string
	nonces         map[string]bool
}

// SetFailKeyLookups flips subsequent SSHKeyByFingerprint calls to not-found —
// the mid-connection key-deletion case (codex round-8 #5's per-channel
// re-read). Safe to call while the server goroutines are serving.
func (f *FakeStore) SetFailKeyLookups(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failKeyLookups = v
}

func (f *FakeStore) SSHKeyByFingerprint(context.Context, string) (store.SSHKey, error) {
	f.mu.Lock()
	fail := f.failKeyLookups
	f.mu.Unlock()
	if fail || f.Missing {
		return store.SSHKey{}, store.ErrNotFound
	}
	return f.Key, nil
}

func (f *FakeStore) StartSSHSession(_ context.Context, audit store.SSHSessionAudit) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, audit)
	return nil
}

func (f *FakeStore) EndSSHSession(_ context.Context, id, result string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ended = append(f.ended, id+":"+result)
	return nil
}

func (f *FakeStore) ClaimShellNonce(_ context.Context, nonce string, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ClaimErr != nil {
		return false, f.ClaimErr
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

// StartedSessions returns a copy of the recorded session-start audit rows.
func (f *FakeStore) StartedSessions() []store.SSHSessionAudit {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.SSHSessionAudit(nil), f.started...)
}

// EndedSessions returns a copy of the recorded "<id>:<result>" session ends.
func (f *FakeStore) EndedSessions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.ended...)
}

// FakeResolver implements TargetResolver, recording the identity subject and
// username it resolved for.
type FakeResolver struct {
	Err      error
	Target   apps.SSHInstanceTarget
	Subject  string
	Username string

	mu   sync.Mutex
	flip error // when set, overrides Err — the mid-connection revocation case
}

// SetFlip makes subsequent ResolveSSHSession calls fail with err — the
// membership-revoked-after-transport-auth case (codex round-8 #5). Safe to call
// while the server goroutines are serving.
func (f *FakeResolver) SetFlip(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.flip = err
}

func (f *FakeResolver) ResolveSSHSession(ctx context.Context, username string) (apps.SSHInstanceTarget, error) {
	identity, _ := core.IdentityFrom(ctx)
	f.Subject, f.Username = identity.Subject, username
	f.mu.Lock()
	flip := f.flip
	f.mu.Unlock()
	if flip != nil {
		return apps.SSHInstanceTarget{}, flip
	}
	if f.Err != nil {
		return apps.SSHInstanceTarget{}, f.Err
	}
	if f.Target.ID != "" {
		return f.Target, nil
	}
	return apps.SSHInstanceTarget{
		ID: "srv-abcdeabcdeabcdeabcde-pod01", ServiceID: "srv-abcdeabcdeabcdeabcde",
		OwnerID: "tea-workspace", Namespace: "default", PodName: "web-rs-pod01", Container: core.AppContainer,
	}, nil
}

// FakeExecutor implements Executor. In TTY mode it reads two terminal sizes
// before writing output — both the native SSH and webshell suites depend on
// that contract to observe resize propagation.
type FakeExecutor struct {
	Command []string
	Target  apps.SSHInstanceTarget
	TTY     bool
	Sizes   []remotecommand.TerminalSize
	Code    int
	Err     error
	Block   bool
	Started chan struct{}
	Stopped chan error
}

func (f *FakeExecutor) Execute(ctx context.Context, target apps.SSHInstanceTarget, command []string, tty bool, queue remotecommand.TerminalSizeQueue, _ io.Reader, stdout, _ io.Writer) (int, error) {
	f.Command = append([]string(nil), command...)
	f.Target = target
	f.TTY = tty
	if f.Started != nil {
		select {
		case f.Started <- struct{}{}:
		default:
		}
	}
	if f.Block {
		<-ctx.Done()
		if f.Stopped != nil {
			f.Stopped <- ctx.Err()
		}
		return 126, ctx.Err()
	}
	if tty {
		if size := queue.Next(); size != nil {
			f.Sizes = append(f.Sizes, *size)
		}
		if size := queue.Next(); size != nil {
			f.Sizes = append(f.Sizes, *size)
		}
	}
	if f.Err == nil {
		_, _ = io.WriteString(stdout, "inside-app\n")
	}
	return f.Code, f.Err
}

// EchoExecutor plays an interactive shell: it echoes stdin to stdout until
// stdin closes (exit 0) or ctx ends, so a test can keep exchanging data over
// a live session. Started, when set, is signalled (without blocking) as each
// exec starts.
type EchoExecutor struct {
	Started chan struct{}
}

func (e *EchoExecutor) Execute(ctx context.Context, _ apps.SSHInstanceTarget, _ []string, _ bool, _ remotecommand.TerminalSizeQueue, stdin io.Reader, stdout, _ io.Writer) (int, error) {
	if e.Started != nil {
		select {
		case e.Started <- struct{}{}:
		default:
		}
	}
	copied := make(chan struct{})
	go func() {
		_, _ = io.Copy(stdout, stdin)
		close(copied)
	}()
	select {
	case <-copied:
		return 0, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// FakeTwoFactor plays bex-api's login-requirements verb
// (agentsession.TwoFactorHandler over the real twofactor.Gate) for the
// gateway's TwoFactorGate: workspaces that Require 2FA admit only Enrolled
// subjects, and workspaces that RequireGoogle admit only questions carrying
// the Google sign-in (a native SSH key added while signed in with Google). A
// workspace whose org RequireByOrganization adds the org's requirements, and
// a refusal they decide names the org (w7/090). It counts every question and
// remembers each one's method and sign-in. Safe to flip while server
// goroutines run.
type FakeTwoFactor struct {
	mu       sync.Mutex
	required map[string]bool
	google   map[string]bool
	orgs     map[string]core.LoginOrganization
	enrolled map[string]bool
	err      error
	asked    int
	methods  []string
	signIns  []core.SignIn
}

// RequireGoogle sets whether a workspace requires Google login (w7/m465).
func (f *FakeTwoFactor) RequireGoogle(workspaceID string, on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.google == nil {
		f.google = map[string]bool{}
	}
	f.google[workspaceID] = on
}

// RequireByOrganization sets the org a workspace belongs to and what the org's
// own settings require in it (w7/m466, w7/090).
func (f *FakeTwoFactor) RequireByOrganization(workspaceID string, org core.LoginOrganization) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.orgs == nil {
		f.orgs = map[string]core.LoginOrganization{}
	}
	f.orgs[workspaceID] = org
}

// SignIns returns the sign-in of every question, in order.
func (f *FakeTwoFactor) SignIns() []core.SignIn {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.SignIn(nil), f.signIns...)
}

// Require sets whether a workspace requires 2FA.
func (f *FakeTwoFactor) Require(workspaceID string, on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.required == nil {
		f.required = map[string]bool{}
	}
	f.required[workspaceID] = on
}

// Enroll sets whether a subject has a second factor.
func (f *FakeTwoFactor) Enroll(subject string, enrolled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.enrolled == nil {
		f.enrolled = map[string]bool{}
	}
	f.enrolled[subject] = enrolled
}

// Fail makes every later question fail like an unreachable bex-api (nil heals).
func (f *FakeTwoFactor) Fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// Ask answers like the verb: not required => (false, no refusal), else the
// Google refusal first, then the subject's enrollment, each naming the org
// when its setting requires what was refused.
func (f *FakeTwoFactor) Ask(_ context.Context, workspaceID, subject, method string, signIn core.SignIn) (required bool, refusal core.LoginRefusal, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked++
	f.methods = append(f.methods, method)
	f.signIns = append(f.signIns, signIn)
	if f.err != nil {
		return false, core.LoginRefusal{}, f.err
	}
	org := f.orgs[workspaceID]
	twoFactor, google := f.required[workspaceID] || org.TwoFactor, f.google[workspaceID] || org.GoogleLogin
	refuse := func(code string, byOrg bool) core.LoginRefusal {
		if !byOrg {
			return core.LoginRefusal{Code: code}
		}
		return core.LoginRefusal{Code: code, OrganizationID: org.ID, OrganizationName: org.Name}
	}
	switch {
	case !twoFactor && !google:
		return false, core.LoginRefusal{}, nil
	case google && signIn != core.SignInGoogle:
		return true, refuse(core.CodeWorkspaceGoogleLoginRequired, org.GoogleLogin), nil
	case twoFactor && !f.enrolled[subject]:
		return true, refuse(core.CodeWorkspaceTwoFactorRequired, org.TwoFactor), nil
	}
	return true, core.LoginRefusal{}, nil
}

// Questions is how many times the gateway asked.
func (f *FakeTwoFactor) Questions() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked
}

// Methods returns the method of every question, in order.
func (f *FakeTwoFactor) Methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.methods...)
}

// FakeClock is a settable clock safe for concurrent use, for caches the
// gateway tests need to age past a TTL without sleeping.
type FakeClock struct {
	mu sync.Mutex
	t  time.Time
}

// NewFakeClock starts a clock at a fixed instant.
func NewFakeClock() *FakeClock {
	return &FakeClock{t: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}
}

// Now is the current fake time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

// Advance moves the clock forward.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// FakeAudit is a core.AuditSink that keeps every row it is given — the
// workspace audit log's shell rows (w9/m451).
type FakeAudit struct {
	mu   sync.Mutex
	rows []core.AuditEvent
}

func (f *FakeAudit) Record(_ context.Context, ev core.AuditEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, ev)
	return nil
}

// Rows returns a copy of the recorded rows.
func (f *FakeAudit) Rows() []core.AuditEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.AuditEvent(nil), f.rows...)
}
