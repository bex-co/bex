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

	mu      sync.Mutex
	started []store.SSHSessionAudit
	ended   []string
	nonces  map[string]bool
}

func (f *FakeStore) SSHKeyByFingerprint(context.Context, string) (store.SSHKey, error) {
	if f.Missing {
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
}

func (f *FakeResolver) ResolveSSHSession(ctx context.Context, username string) (apps.SSHInstanceTarget, error) {
	identity, _ := core.IdentityFrom(ctx)
	f.Subject, f.Username = identity.Subject, username
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
	TTY     bool
	Sizes   []remotecommand.TerminalSize
	Code    int
	Err     error
	Block   bool
	Started chan struct{}
	Stopped chan error
}

func (f *FakeExecutor) Execute(ctx context.Context, _ apps.SSHInstanceTarget, command []string, tty bool, queue remotecommand.TerminalSizeQueue, _ io.Reader, stdout, _ io.Writer) (int, error) {
	f.Command = append([]string(nil), command...)
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
