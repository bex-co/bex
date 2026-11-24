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
	"fmt"
	"io"
	"log"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// An SSH login into an agent session whose sandbox closed wakes it (w10/m597,
// ADR054 D9), as SSH into a sleeping Devin session "wakes the machine and
// completes the request instead of showing 'not running'"
// (docs.devin.ai/release-notes/2026, 2026-09-25). The handshake cannot hold
// a sandbox start (HandshakeTimeout is 10 s), so the login asks bex-api to
// start it and is admitted with a target that names only the session. Its
// channels are accepted and held: each tells the client on stderr that the
// sandbox is starting, and runs its command once the session settles with a
// sandbox, re-authorized like any channel. A login into a session that is
// already starting its sandbox waits the same way without asking.

const (
	// defaultWakePollInterval is how often a held login reads whether its
	// session settled.
	defaultWakePollInterval = 2 * time.Second
	// wakeProgressInterval is how often a held channel says it still waits.
	wakeProgressInterval = 30 * time.Second
	// maxWakeMessageLength bounds bex-api's refusal message in a banner, in
	// characters.
	maxWakeMessageLength = 300
)

// SessionWaker asks bex-api to start a closed agent session's sandbox as the
// owner of the key that logged in: *agentsession.SessionWakeClient.
type SessionWaker interface {
	Wake(ctx context.Context, req agentsession.SessionWakeRequest) (agentsession.SessionWakeResponse, error)
}

// sessionWake is what a login refused for having no sandbox to open can
// still do: wake a closed session, or wait for one that is starting.
type sessionWake struct {
	state     string // agentsessions.SSHSandboxClosed or SSHSandboxStarting
	workspace string
}

// wakeOf reads err, the agent-session resolver's refusal of connUser, for a
// sandbox the login may wake or wait for. The zero value means neither: the
// wake is off (no verb wired, or the multi-channel carve-out disabled), or
// the session was canceled or archived, or err is some other refusal.
func (s *Server) wakeOf(connUser string, err error) sessionWake {
	if s.Wakes == nil || s.MaxChannelsPerConn <= 0 || !sshgateway.AgentSessionUsername(connUser) {
		return sessionWake{}
	}
	_, state, workspace := agentsessions.SSHNoSandbox(err)
	if (state != agentsessions.SSHSandboxClosed && state != agentsessions.SSHSandboxStarting) || workspace == "" {
		return sessionWake{}
	}
	return sessionWake{state: state, workspace: workspace}
}

// wakeSession runs once the login passed every check an admitted one does.
// A closed session is started through bex-api as the key's owner; a
// starting one is left to start. It returns the target the login waits on,
// or the refusal the client is told: refused is the resolver's banner
// (w10/043), kept for a caller who may not start the session and whenever
// bex-api gave no answer.
func (s *Server) wakeSession(ctx context.Context, conn ssh.ConnMetadata, key store.SSHKey, wake sessionWake, refused error) (apps.SSHInstanceTarget, *refusal) {
	user := conn.User()
	if wake.state == agentsessions.SSHSandboxClosed {
		answer, err := s.Wakes.Wake(ctx, agentsession.SessionWakeRequest{
			SessionID: user, Subject: key.Subject, Fingerprint: key.Fingerprint, SourceIP: sourceIP(conn.RemoteAddr()),
		})
		// wakeOf read refused as the resolver's session refusal, so it has a banner.
		banner, _ := sessionRefusal(refused)
		switch {
		case err != nil:
			s.Metrics.SessionWake("unavailable")
			log.Printf("ssh gateway: starting a session's sandbox got no answer, so the login is told to start it: %s", boundedError(err))
			return apps.SSHInstanceTarget{}, &banner
		case answer.Refusal == agentsession.SessionWakeForbidden:
			s.Metrics.SessionWake("refused")
			return apps.SSHInstanceTarget{}, &banner
		case answer.Refusal != "":
			s.Metrics.SessionWake("refused_start")
			reason := "agent session " + user + " is not running, and its sandbox could not start: " + bannerText(answer.Message)
			return apps.SSHInstanceTarget{}, &refusal{banner: "bex: " + reason + ".\n", reason: reason, metric: banner.metric}
		}
	}
	return apps.SSHInstanceTarget{ID: user, ServiceID: user, OwnerID: wake.workspace, Sandbox: true}, nil
}

// bannerText is bex-api's message as one bounded line of printable text,
// without its closing period.
func bannerText(msg string) string {
	msg = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return ' '
	}, msg)
	if runes := []rune(msg); len(runes) > maxWakeMessageLength {
		msg = string(runes[:maxWakeMessageLength]) + "…"
	}
	return strings.TrimRight(strings.TrimSpace(msg), ".")
}

// wakeFailure is why a held login ended without a sandbox, told to every
// channel that waited on it.
type wakeFailure struct{ reason string }

func (f *wakeFailure) Error() string { return f.reason }

// sandboxWait is a held login's wait for its session's sandbox: one per
// connection, shared by every channel opened before it ends. err is nil
// once the sandbox is up.
type sandboxWait struct {
	session string
	done    chan struct{}
	err     error
}

// awaitSandbox starts the wait of a held login. It ends when the session
// settles, the start's ceiling passes, or ctx (the connection, its session
// timeout or the gateway's drain) ends, and counts how.
func (s *Server) awaitSandbox(ctx context.Context, subject, fingerprint, connUser string) *sandboxWait {
	w := &sandboxWait{session: connUser, done: make(chan struct{})}
	go func() {
		defer close(w.done)
		var result string
		result, w.err = s.pollSandbox(ctx, subject, fingerprint, connUser)
		s.Metrics.SessionWake(result)
	}()
	return w
}

// pollSandbox reads, every wakePollInterval, whether the session settled,
// through the same key read and resolver the login passed, so a key deleted
// or access revoked meanwhile ends the wait. It returns the wait's metric
// result and, unless the sandbox is up, why it ended.
func (s *Server) pollSandbox(ctx context.Context, subject, fingerprint, connUser string) (string, error) {
	ceiling := time.NewTimer(s.WakeTimeout)
	defer ceiling.Stop()
	tick := time.NewTicker(s.wakePollInterval)
	defer tick.Stop()
	unanswered := 0
	for {
		select {
		case <-ctx.Done():
			return "abandoned", ctx.Err()
		case <-ceiling.C:
			return "timed_out", &wakeFailure{reason: fmt.Sprintf("the sandbox of agent session %s did not start within %s. Connect again to try again", connUser, s.WakeTimeout)}
		case <-tick.C:
		}
		err := s.sandboxUp(ctx, subject, fingerprint, connUser)
		if err == nil {
			return "opened", nil
		}
		r, refused := sessionRefusal(err)
		switch state := s.wakeOf(connUser, err).state; {
		case state == agentsessions.SSHSandboxStarting:
			unanswered = 0
			continue
		case state == agentsessions.SSHSandboxClosed: // the start ended without one
			return "no_sandbox", &wakeFailure{reason: "the sandbox of agent session " + connUser + " did not start. Connect again to try again"}
		case refused: // canceled or archived meanwhile
			return "no_sandbox", &wakeFailure{reason: r.reason}
		case definiteRefusal(err):
			return "abandoned", &wakeFailure{reason: reauthorizeFailed}
		}
		if unanswered++; unanswered >= maxUnansweredRevalidations {
			log.Printf("ssh gateway: ending a held login after %d reads in a row without an answer: %s", unanswered, boundedError(err))
			return "abandoned", &wakeFailure{reason: reauthorizeFailed}
		}
	}
}

// sandboxUp re-reads the key and resolves the session as its owner: nil
// once the sandbox is up, the resolver's refusal while it is not, and a
// *reauthFailure when the key is gone or the read got no answer.
func (s *Server) sandboxUp(ctx context.Context, subject, fingerprint, connUser string) error {
	lookupCtx, cancel := context.WithTimeout(ctx, s.HandshakeTimeout)
	defer cancel()
	registered, failure := s.rereadKey(lookupCtx, subject, fingerprint)
	if failure != nil {
		return failure
	}
	_, err := s.Apps.ResolveSSHSession(core.WithIdentity(lookupCtx, keyIdentity(registered)), connUser)
	if _, told := sessionRefusal(err); err == nil || told {
		return err
	}
	return resolveFailure(err)
}

// pending reports whether the wait is still on; false for no wait.
func (w *sandboxWait) pending() bool {
	if w == nil {
		return false
	}
	select {
	case <-w.done:
		return false
	default:
		return true
	}
}

// await holds a channel until the wait ends, telling the client on stderr
// that the sandbox is starting and, every wakeProgressInterval, that it
// still is. It returns why the wait ended without a sandbox, or ctx's error.
func (w *sandboxWait) await(ctx context.Context, stderr io.Writer) error {
	if !w.pending() {
		return w.err
	}
	_, _ = io.WriteString(stderr, "bex: starting the sandbox of agent session "+w.session+". This connection continues once it is up.\n")
	started := time.Now()
	progress := time.NewTicker(wakeProgressInterval)
	defer progress.Stop()
	for {
		select {
		case <-w.done:
			return w.err
		case <-ctx.Done():
			return ctx.Err()
		case <-progress.C:
			_, _ = fmt.Fprintf(stderr, "bex: still starting the sandbox (%s).\n", time.Since(started).Round(time.Second))
		}
	}
}

// pendingTarget is the target of a channel held for a waking session: the
// session re-authorized like any channel, once the wait saw its sandbox up.
type pendingTarget func(ctx context.Context, stderr io.Writer) (apps.SSHInstanceTarget, error)

func (s *Server) pendingTarget(wait *sandboxWait, subject, fingerprint, connUser string, sticky *connectionTarget) pendingTarget {
	return func(ctx context.Context, stderr io.Writer) (apps.SSHInstanceTarget, error) {
		if err := wait.await(ctx, stderr); err != nil {
			return apps.SSHInstanceTarget{}, err
		}
		target, err := s.reauthorize(ctx, subject, fingerprint, connUser, sticky)
		if err != nil {
			return apps.SSHInstanceTarget{}, &wakeFailure{reason: channelRefusal(err)}
		}
		return target, nil
	}
}

// endWithoutSandbox runs once a held login's wait ended without a sandbox.
// The channels that waited tell the client why on their own; this refuses
// new channels and gives the client clientCloseGrace to hang up, as a
// finished single-session connection does (w6/096), so the reason is not
// lost to a reset.
func endWithoutSandbox(ctx context.Context, channels <-chan ssh.NewChannel, reason string) {
	grace := time.NewTimer(clientCloseGrace)
	defer grace.Stop()
	for {
		select {
		case channel, ok := <-channels:
			if !ok {
				return
			}
			_ = channel.Reject(ssh.Prohibited, reason)
		case <-grace.C:
			return
		case <-ctx.Done():
			return
		}
	}
}
