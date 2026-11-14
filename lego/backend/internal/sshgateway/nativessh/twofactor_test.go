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
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// twofactor_test.go proves workspace-enforced two-factor authentication
// (w22/m402, docs/ADR012-auth.md §9a) reaches native public-key SSH, which never
// passes bex-api's middleware: the gateway's own core.Base carries a
// sshgateway.TwoFactorGate, so the key owner's second factor is checked at
// authentication and again before every channel and on every revalidation tick.

const twoFactorWorkspace = "tea-workspace" // the FakeResolver's default target workspace

// funnelResolver resolves like apps.ResolveSSHSession does: an authorization on
// the target's workspace through core.Base (the funnel that asks the 2FA gate),
// then the fake target.
type funnelResolver struct {
	base  *core.Base
	inner *gatewaytest.FakeResolver
}

func (r funnelResolver) ResolveSSHSession(ctx context.Context, username string) (apps.SSHInstanceTarget, error) {
	if err := r.base.AuthorizeOn(ctx, core.RelCanViewSensitive, core.WorkspaceObject(twoFactorWorkspace)); err != nil {
		return apps.SSHInstanceTarget{}, err
	}
	return r.inner.ResolveSSHSession(ctx, username)
}

type allowAllChecker struct{}

func (allowAllChecker) Check(context.Context, string, string, string) (bool, error) { return true, nil }

type twoFactorHarness struct {
	asker    *gatewaytest.FakeTwoFactor
	clock    *gatewaytest.FakeClock
	registry *prometheus.Registry
	exec     *countingExecutor
	store    *gatewaytest.FakeStore
	addr     string
	signer   ssh.Signer
}

// startTwoFactorGateway runs a gateway whose workspace "tea-workspace" requires
// 2FA and whose only key owner, user-1, is enrolled or not as asked.
func startTwoFactorGateway(t *testing.T, require, enrolled bool, configure func(*Server)) *twoFactorHarness {
	t.Helper()
	h := &twoFactorHarness{
		asker: &gatewaytest.FakeTwoFactor{}, clock: gatewaytest.NewFakeClock(),
		registry: prometheus.NewRegistry(), exec: &countingExecutor{release: make(chan struct{})},
		store: &gatewaytest.FakeStore{}, signer: signer(t),
	}
	h.asker.Require(twoFactorWorkspace, require)
	h.asker.Enroll("user-1", enrolled)
	metrics := sshgateway.NewMetrics(h.registry)
	base := &core.Base{
		Authz:     allowAllChecker{},
		TwoFactor: &sshgateway.TwoFactorGate{Asker: h.asker, Metrics: metrics, Now: h.clock.Now},
	}
	resolver := funnelResolver{base: base, inner: &gatewaytest.FakeResolver{}}
	addr, stop := startGatewayConfigured(t, h.store, resolver, h.exec, h.signer, func(s *Server) {
		s.Metrics = metrics
		if configure != nil {
			configure(s)
		}
	})
	t.Cleanup(stop)
	h.addr = addr
	return h
}

// dialWithBanner dials like dialGateway and also returns what the server sent
// the client during authentication, which an OpenSSH client prints to stderr.
func dialWithBanner(addr, username string, clientSigner ssh.Signer) (*ssh.Client, string, error) {
	var mu sync.Mutex
	var banner strings.Builder
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User: username, Auth: []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 2 * time.Second,
		BannerCallback: func(message string) error {
			mu.Lock()
			defer mu.Unlock()
			banner.WriteString(message)
			return nil
		},
	})
	mu.Lock()
	defer mu.Unlock()
	return client, banner.String(), err
}

func (h *twoFactorHarness) metric(name string, labels map[string]string, t *testing.T) float64 {
	t.Helper()
	return gatewaytest.MetricValue(t, h.registry, name, labels)
}

// An unenrolled member's key on a workspace that requires 2FA is refused, and
// the SSH client is told why, in the banner it prints while authenticating.
func TestUnenrolledKeyOwnerIsRefusedWithAMessageOnAnEnforcingWorkspace(t *testing.T) {
	h := startTwoFactorGateway(t, true, false, nil)
	client, banner, err := dialWithBanner(h.addr, "srv-abcdeabcdeabcdeabcde", h.signer)
	if err == nil {
		client.Close()
		t.Fatal("an unenrolled member authenticated on a workspace that requires two-factor authentication")
	}
	for _, want := range []string{"two-factor", "enable"} {
		if !strings.Contains(strings.ToLower(banner), want) {
			t.Fatalf("banner = %q, want a message telling the member the workspace requires two-factor authentication (missing %q)", banner, want)
		}
	}
	if got := h.metric("bex_ssh_gateway_authentications_total", map[string]string{"result": "rejected_two_factor"}, t); got != 1 {
		t.Fatalf("rejected_two_factor authentications = %v, want 1", got)
	}
	if got := h.metric("bex_ssh_gateway_two_factor_refusals_total", map[string]string{"result": "not_enrolled"}, t); got < 1 {
		t.Fatalf("the refusal was not counted: %v", got)
	}
	if got := h.metric("bex_ssh_gateway_authentications_total", map[string]string{"result": "accepted"}, t); got != 0 {
		t.Fatalf("accepted authentications = %v, want 0", got)
	}
}

func TestEnrolledKeyOwnerAuthenticatesOnAnEnforcingWorkspace(t *testing.T) {
	h := startTwoFactorGateway(t, true, true, nil)
	client, banner, err := dialWithBanner(h.addr, "srv-abcdeabcdeabcdeabcde", h.signer)
	if err != nil {
		t.Fatalf("an enrolled member was refused: %v", err)
	}
	defer client.Close()
	if banner != "" {
		t.Fatalf("an admitted member was sent a banner: %q", banner)
	}
	if methods := h.asker.Methods(); len(methods) == 0 || methods[0] != "ssh" {
		t.Fatalf("the gate asked with methods %v, want the key login's \"ssh\"", methods)
	}
}

func TestKeyOwnerOfASettingOffWorkspaceNeedsNoSecondFactor(t *testing.T) {
	h := startTwoFactorGateway(t, false, false, nil)
	client, _, err := dialWithBanner(h.addr, "srv-abcdeabcdeabcdeabcde", h.signer)
	if err != nil {
		t.Fatalf("a workspace that does not require 2FA refused an unenrolled member: %v", err)
	}
	defer client.Close()
	// Re-authorizing every channel and tick must not add a question: the
	// workspace's "not required" answer is cached for every member.
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	_ = session.Close()
	if got := h.asker.Questions(); got != 1 {
		t.Fatalf("bex-api was asked %d times for one setting-off workspace, want 1", got)
	}
}

// If bex-api cannot be reached the gateway cannot know the setting, so it
// refuses (core.ErrAuthzUnavailable, as with an unreachable OpenFGA) instead of
// admitting; the refusal is generic because it is an outage, not the member's.
func TestUnreachableBexAPIRefusesNativeSSHFailClosed(t *testing.T) {
	h := startTwoFactorGateway(t, false, true, nil)
	h.asker.Fail(errors.New("bex-api unreachable"))
	client, banner, err := dialWithBanner(h.addr, "srv-abcdeabcdeabcdeabcde", h.signer)
	if err == nil {
		client.Close()
		t.Fatal("a key authenticated while the two-factor setting could not be established")
	}
	if strings.Contains(strings.ToLower(banner), "two-factor") {
		t.Fatalf("an outage was reported as the member's missing second factor: %q", banner)
	}
	if got := h.metric("bex_ssh_gateway_two_factor_refusals_total", map[string]string{"result": "unavailable"}, t); got < 1 {
		t.Fatalf("the fail-closed refusal was not counted: %v", got)
	}
}

// A live session is cut when its member stops being compliant, and not before
// the gateway's cache says so: within TwoFactorCacheTTL plus one revalidation
// interval, never later. The watchdog runs at 10 ms here; the cache is aged with
// the injectable clock.
func TestLiveSessionIsClosedAtTheRevalidationAfterTheMemberStopsBeingCompliant(t *testing.T) {
	h := startTwoFactorGateway(t, true, true, func(s *Server) { s.RevalidateInterval = 10 * time.Millisecond })
	client, _, err := dialWithBanner(h.addr, "srv-abcdeabcdeabcdeabcde", h.signer)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	holdChannel(t, client, h.exec)

	// The member removes their last second factor.
	h.asker.Enroll("user-1", false)

	// Inside the cache TTL the gateway still holds the member's positive answer:
	// many revalidation ticks pass and the session survives. This is the bound.
	time.Sleep(150 * time.Millisecond)
	if h.exec.activeNow() != 1 {
		t.Fatal("the session was cut inside the cache TTL, before the gateway could have learned the member stopped being compliant")
	}

	// Past the TTL the next tick asks again, is refused, and closes the stream.
	h.clock.Advance(sshgateway.TwoFactorCacheTTL + time.Second)
	waitFor(t, func() bool {
		ended := h.store.EndedSessions()
		return h.exec.activeNow() == 0 && len(ended) == 1 && endsWith(ended[0], ":revoked")
	})
}

// A member who becomes non-compliant is also refused the next channel on the
// connection they already hold, with a reason the SSH client prints.
func TestNewChannelIsRefusedWithAReasonAfterTheMemberStopsBeingCompliant(t *testing.T) {
	h := startTwoFactorGateway(t, true, true, nil)
	client, _, err := dialWithBanner(h.addr, "srv-abcdeabcdeabcdeabcde", h.signer)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	h.asker.Enroll("user-1", false)
	h.clock.Advance(sshgateway.TwoFactorCacheTTL + time.Second)

	_, err = client.NewSession()
	var open *ssh.OpenChannelError
	if !errors.As(err, &open) {
		t.Fatalf("NewSession = %v, want the server's channel refusal", err)
	}
	if !strings.Contains(strings.ToLower(open.Message), "two-factor") {
		t.Fatalf("channel refusal reason = %q, want it to name the two-factor requirement", open.Message)
	}
	waitFor(t, func() bool {
		ended := h.store.EndedSessions()
		return len(ended) == 1 && endsWith(ended[0], ":revoked")
	})
}

// The refusal is recognized by its code, so any resolver that surfaces the
// funnel's coded 403 gets the message, and nothing else does.
func TestOnlyTheTwoFactorCodeGetsTheMessage(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want bool
	}{
		"the coded refusal":    {core.NewWorkspaceTwoFactorRequiredError("tea-x"), true},
		"wrapped":              {errors.Join(errors.New("resolve"), core.NewWorkspaceTwoFactorRequiredError("tea-x")), true},
		"plain forbidden":      {core.ErrForbidden, false},
		"another coded 403":    {core.NewEmailVerificationRequiredError(), false},
		"outage":               {core.ErrAuthzUnavailable, false},
		"not found":            {store.ErrNotFound, false},
		"unrelated plain text": {errors.New(core.CodeWorkspaceTwoFactorRequired), false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := isTwoFactorRefusal(tc.err); got != tc.want {
				t.Fatalf("isTwoFactorRefusal(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
