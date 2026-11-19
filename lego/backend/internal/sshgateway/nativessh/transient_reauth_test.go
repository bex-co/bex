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
	"bytes"
	"context"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w6/147: on production, live `ssh <srv>@ssh.bex.co` sessions ended every
// hour or two with no revocation, deploy or instance change. Each revalidation
// tick re-runs three checks (the key lookup, the owner's email check and the
// target resolution), and any error from any of them ended the session. Now
// only a definite no ends it at the next tick; a check that gets no answer is
// retried, and only maxUnansweredRevalidations of those in a row end it.

// failNext makes a check fail its next n calls with err, then answer again.
// calls counts every call since the last arm.
type failNext struct {
	mu    sync.Mutex
	n     int
	err   error
	calls int
}

func (f *failNext) arm(n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n, f.err, f.calls = n, err, 0
}

func (f *failNext) next() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.n > 0 {
		f.n--
		return f.err
	}
	return nil
}

func (f *failNext) callsSinceArmed() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type failingKeyStore struct {
	*gatewaytest.FakeStore
	fail *failNext
}

func (s *failingKeyStore) SSHKeyByFingerprint(ctx context.Context, fingerprint string) (store.SSHKey, error) {
	if err := s.fail.next(); err != nil {
		return store.SSHKey{}, err
	}
	return s.FakeStore.SSHKeyByFingerprint(ctx, fingerprint)
}

type failingResolver struct {
	*gatewaytest.FakeResolver
	fail *failNext
}

func (r *failingResolver) ResolveSSHSession(ctx context.Context, username string) (apps.SSHInstanceTarget, error) {
	if err := r.fail.next(); err != nil {
		return apps.SSHInstanceTarget{}, err
	}
	return r.FakeResolver.ResolveSSHSession(ctx, username)
}

// reauthCheck is one of the three checks a reauthorization runs, with an error
// that is no answer and one that is a definite no.
type reauthCheck struct {
	install             func(*Server, *failNext)
	unanswered, refused error
}

var reauthChecks = map[string]reauthCheck{
	"key lookup": {
		install: func(s *Server, fail *failNext) {
			s.Store = &failingKeyStore{FakeStore: s.Store.(*gatewaytest.FakeStore), fail: fail}
		},
		unanswered: context.DeadlineExceeded, // the lookup's HandshakeTimeout
		refused:    store.ErrNotFound,        // the key was deleted
	},
	"email check": {
		install: func(s *Server, fail *failNext) {
			s.EmailVerified = func(context.Context, string) error { return fail.next() }
		},
		// The 401 bex-api answers when its nonce claim times out.
		unanswered: agentsession.ErrForbidden,
		refused:    agentsession.ErrEmailUnverified,
	},
	"resolver": {
		install: func(s *Server, fail *failNext) {
			s.Apps = &failingResolver{FakeResolver: s.Apps.(*gatewaytest.FakeResolver), fail: fail}
		},
		unanswered: core.ErrAuthzUnavailable,
		refused:    core.ErrForbidden,
	},
}

// heldAppSession is one live single-channel App session (`ssh <srv>@…`) whose
// checks revalidate every 10 ms, with check failing on demand.
func heldAppSession(t *testing.T, check reauthCheck) (*failNext, *countingExecutor, *gatewaytest.FakeStore) {
	t.Helper()
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	st := &gatewaytest.FakeStore{}
	fail := &failNext{}
	addr, stop := startGatewayConfigured(t, st, &gatewaytest.FakeResolver{}, exec, clientSigner, func(s *Server) {
		s.RevalidateInterval = 10 * time.Millisecond
		check.install(s, fail)
	})
	t.Cleanup(stop)
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	holdChannel(t, client, exec)
	return fail, exec, st
}

func TestUnansweredRevalidationKeepsTheLiveSession(t *testing.T) {
	for name, check := range reauthChecks {
		t.Run(name, func(t *testing.T) {
			fail, exec, st := heldAppSession(t, check)
			// Fail as many ticks in a row as may go unanswered (once, and more),
			// then let a few answered ticks pass.
			fail.arm(maxUnansweredRevalidations-1, check.unanswered)
			waitFor(t, func() bool { return fail.callsSinceArmed() >= maxUnansweredRevalidations+2 })
			if exec.activeNow() != 1 || len(st.EndedSessions()) != 0 {
				t.Fatalf("the session ended on %v, a check that got no answer (active %d, ended %v)", check.unanswered, exec.activeNow(), st.EndedSessions())
			}
		})
	}
}

func TestDefiniteRefusalEndsTheLiveSessionAtTheNextTick(t *testing.T) {
	for name, check := range reauthChecks {
		t.Run(name, func(t *testing.T) {
			fail, exec, st := heldAppSession(t, check)
			fail.arm(1<<30, check.refused)
			waitFor(t, func() bool {
				ended := st.EndedSessions()
				return exec.activeNow() == 0 && len(ended) == 1 && endsWith(ended[0], ":revoked")
			})
			if calls := fail.callsSinceArmed(); calls != 1 {
				t.Fatalf("the session ended after %d refused checks, want the first (%v)", calls, check.refused)
			}
		})
	}
}

// An outage cannot keep a session open without checks: no answer, tick after
// tick, ends it too.
func TestUnansweredRevalidationsInARowEndTheSession(t *testing.T) {
	for name, check := range reauthChecks {
		t.Run(name, func(t *testing.T) {
			fail, exec, st := heldAppSession(t, check)
			fail.arm(1<<30, check.unanswered)
			waitFor(t, func() bool {
				ended := st.EndedSessions()
				return exec.activeNow() == 0 && len(ended) == 1 && endsWith(ended[0], ":revoked")
			})
			if calls := fail.callsSinceArmed(); calls != maxUnansweredRevalidations {
				t.Fatalf("the session ended after %d unanswered checks, want %d", calls, maxUnansweredRevalidations)
			}
		})
	}
}

// The budget counts failures in a row: an answered tick starts it again.
func TestAnsweredRevalidationResetsTheUnansweredCount(t *testing.T) {
	check := reauthChecks["resolver"]
	fail, exec, st := heldAppSession(t, check)
	for range 3 {
		fail.arm(maxUnansweredRevalidations-1, check.unanswered)
		waitFor(t, func() bool { return fail.callsSinceArmed() >= maxUnansweredRevalidations })
	}
	if exec.activeNow() != 1 || len(st.EndedSessions()) != 0 {
		t.Fatalf("the session ended though no %d checks in a row went unanswered (active %d, ended %v)", maxUnansweredRevalidations, exec.activeNow(), st.EndedSessions())
	}
}

// A channel never opens on a check that got no answer, but on a multiplexed
// connection that refuses only the new channel: the live ones keep running and
// the next open succeeds once the check answers.
func TestUnansweredChannelOpenRefusesOnlyThatChannel(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	st := &gatewaytest.FakeStore{}
	fail := &failNext{}
	addr, stop := startGatewayConfigured(t, st, &gatewaytest.FakeResolver{Target: sandboxTarget()}, exec, clientSigner, func(s *Server) {
		s.MaxChannelsPerConn = 4
		s.RevalidateInterval = -1
		reauthChecks["resolver"].install(s, fail)
	})
	defer stop()
	client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	holdChannel(t, client, exec)

	fail.arm(1, core.ErrAuthzUnavailable)
	if _, err := client.NewSession(); err == nil {
		t.Fatal("a channel opened on a check that got no answer")
	}
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("the connection did not survive one unanswered channel check: %v", err)
	}
	_ = session.Close()
	if exec.activeNow() != 1 || len(st.EndedSessions()) != 0 {
		t.Fatalf("the live channel ended (active %d, ended %v)", exec.activeNow(), st.EndedSessions())
	}
}

// On the single-channel App path the refused channel was the session's only
// one, so the session ends, recorded as failed rather than revoked.
func TestUnansweredChannelOpenOnTheAppPathIsNotARevocation(t *testing.T) {
	clientSigner := signer(t)
	st := &gatewaytest.FakeStore{}
	fail := &failNext{}
	addr, stop := startGatewayConfigured(t, st, &gatewaytest.FakeResolver{}, &countingExecutor{}, clientSigner, func(s *Server) {
		reauthChecks["resolver"].install(s, fail)
	})
	defer stop()
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	fail.arm(1, core.ErrAuthzUnavailable)
	if _, err := client.NewSession(); err == nil {
		t.Fatal("a channel opened on a check that got no answer")
	}
	waitFor(t, func() bool {
		ended := st.EndedSessions()
		return len(ended) == 1 && endsWith(ended[0], ":failed")
	})
}

// The gateway logs why it ended a session, by cause only: never the subject or
// the key.
func TestReauthorizationFailuresLogTheirCauseWithoutSubjectOrKey(t *testing.T) {
	logged := captureLog(t)
	for name, tc := range map[string]struct {
		check reauthCheck
		err   error
		want  string
	}{
		"deleted key":    {reauthChecks["key lookup"], store.ErrNotFound, "reauthorization rejected: " + causeKeyGone},
		"key lookup":     {reauthChecks["key lookup"], context.DeadlineExceeded, "reauthorization unanswered: " + causeKeyLookup + ": public key lookup: context deadline exceeded"},
		"unverified":     {reauthChecks["email check"], agentsession.ErrEmailUnverified, "reauthorization rejected: " + causeEmailUnverified},
		"email check":    {reauthChecks["email check"], agentsession.ErrForbidden, "reauthorization unanswered: " + causeEmailCheck},
		"denied":         {reauthChecks["resolver"], core.ErrForbidden, "reauthorization rejected: " + causeDenied},
		"login refusal":  {reauthChecks["resolver"], core.NewWorkspaceTwoFactorRequiredError("tea-workspace"), "reauthorization rejected: " + causeDenied},
		"service gone":   {reauthChecks["resolver"], core.ErrNotFound, "reauthorization rejected: " + causeTargetNotFound},
		"authz down":     {reauthChecks["resolver"], core.ErrAuthzUnavailable, "reauthorization unanswered: " + causeResolveFailed},
		"no answer, end": {reauthChecks["resolver"], core.ErrAuthzUnavailable, "ending a session after 3 revalidations in a row without an answer"},
	} {
		t.Run(name, func(t *testing.T) {
			logged.Reset()
			fail, _, st := heldAppSession(t, tc.check)
			fail.arm(1<<30, tc.err)
			waitFor(t, func() bool { return len(st.EndedSessions()) == 1 })
			out := logged.String()
			if !strings.Contains(out, tc.want) {
				t.Fatalf("log = %q, want %q", out, tc.want)
			}
			for _, secret := range []string{st.Key.Subject, st.Key.Fingerprint, strings.TrimPrefix(st.Key.Fingerprint, "SHA256:")} {
				if secret != "" && strings.Contains(out, secret) {
					t.Fatalf("log names the subject or the key (%q): %q", secret, out)
				}
			}
		})
	}
}

// The instance a session runs on going away is a definite end, named as such,
// through the production resolver's own refusal (apps.ShellRefusal).
func TestDeletedInstanceEndsTheSessionAsInstanceGone(t *testing.T) {
	logged := captureLog(t)
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	serving := gatewaytest.NewServingApp(t)
	st := &gatewaytest.FakeStore{}
	addr, stop := startGatewayConfigured(t, st, serving.Resolver, exec, clientSigner,
		func(s *Server) { s.RevalidateInterval = 10 * time.Millisecond })
	defer stop()
	client, err := dialGateway(addr, gatewaytest.ServingAppID, clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	holdChannel(t, client, exec)

	logged.Reset()
	serving.DeletePod(t)
	waitFor(t, func() bool {
		ended := st.EndedSessions()
		return exec.activeNow() == 0 && len(ended) == 1 && endsWith(ended[0], ":revoked")
	})
	out := logged.String()
	if !strings.Contains(out, "reauthorization rejected: "+apps.ShellRefusalInstanceGone) || strings.Contains(out, "unanswered") {
		t.Fatalf("log = %q, want one definite instance_gone refusal", out)
	}
}

// captureLog sends the standard logger to a buffer for the rest of the test.
func captureLog(t *testing.T) *lockedBuffer {
	t.Helper()
	buf := &lockedBuffer{}
	previous := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	return buf
}

// lockedBuffer is a bytes.Buffer the gateway's goroutines may log into while
// the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *lockedBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}
