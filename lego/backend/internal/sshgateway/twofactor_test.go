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
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// twofactor_test.go pins the gateway's half of workspace-enforced two-factor
// authentication (w22/m402, docs/ADR012-auth.md §9a): the gateway has no Kratos
// client, so its core.TwoFactorGate asks bex-api and only caches the answer.

type fakeSessions map[string]string

func (f fakeSessions) AgentSessionWorkspace(_ context.Context, id string) (string, error) {
	return f[id], nil // "" for a session that does not exist, like the store
}

type twoFactorFixture struct {
	gate  *TwoFactorGate
	asker *gatewaytest.FakeTwoFactor
	clock *gatewaytest.FakeClock
	reg   *prometheus.Registry
}

func newTwoFactorFixture() *twoFactorFixture {
	f := &twoFactorFixture{asker: &gatewaytest.FakeTwoFactor{}, clock: gatewaytest.NewFakeClock(), reg: prometheus.NewRegistry()}
	f.asker.Require("tea-on", true)
	f.asker.Enroll("enrolled", true)
	f.gate = &TwoFactorGate{
		Asker: f.asker, Sessions: fakeSessions{"ags-1": "tea-on"},
		Metrics: NewMetrics(f.reg), Now: f.clock.Now,
	}
	return f
}

func sshIdentity(subject string) core.Identity {
	return core.Identity{Subject: subject, Method: "ssh"}
}

func requireRefusedWith(t *testing.T, err error, workspaceID string) {
	t.Helper()
	var coded *core.CodedError
	if !errors.Is(err, core.ErrForbidden) || !errors.As(err, &coded) || coded.Code != core.CodeWorkspaceTwoFactorRequired {
		t.Fatalf("err = %v, want the coded %s 403", err, core.CodeWorkspaceTwoFactorRequired)
	}
	if coded.Params["workspaceId"] != workspaceID {
		t.Fatalf("params = %v, want workspaceId %q", coded.Params, workspaceID)
	}
}

// A workspace that never asked costs one question per TTL for ALL of its
// members and methods: the answer is "not required", which holds for everyone.
func TestTwoFactorGateCachesASettingOffWorkspaceForEveryMember(t *testing.T) {
	f := newTwoFactorFixture()
	ctx := context.Background()
	for _, id := range []core.Identity{sshIdentity("a"), sshIdentity("b"), {Subject: "c", Method: "shell"}, {Subject: "a", Method: "agent-attach"}} {
		if err := f.gate.Check(ctx, "tea-off", id); err != nil {
			t.Fatalf("Check(%+v) on a setting-off workspace = %v", id, err)
		}
	}
	if got := f.asker.Questions(); got != 1 {
		t.Fatalf("bex-api was asked %d times for four members of one setting-off workspace, want 1", got)
	}
	// The cache is bounded: past the TTL it asks again, so turning the setting
	// on reaches the gateway within it.
	f.clock.Advance(TwoFactorCacheTTL + time.Second)
	f.asker.Require("tea-off", true)
	requireRefusedWith(t, f.gate.Check(ctx, "tea-off", sshIdentity("a")), "tea-off")
	if got := f.asker.Questions(); got != 2 {
		t.Fatalf("questions after the TTL = %d, want 2", got)
	}
}

func TestTwoFactorGateAdmitsEnrolledAndRefusesUnenrolledMembers(t *testing.T) {
	f := newTwoFactorFixture()
	ctx := context.Background()
	if err := f.gate.Check(ctx, "tea-on", sshIdentity("enrolled")); err != nil {
		t.Fatalf("an enrolled member = %v", err)
	}
	requireRefusedWith(t, f.gate.Check(ctx, "tea-on", sshIdentity("unenrolled")), "tea-on")
	// The question names how the gateway authenticated the member, so the real
	// gate sees the identity the funnel was handed.
	if methods := f.asker.Methods(); len(methods) != 2 || methods[0] != "ssh" || methods[1] != "ssh" {
		t.Fatalf("question methods = %v, want the identity's method (ssh) each time", methods)
	}
}

// Both answers are cached for the TTL on a requiring workspace, so the hot path
// of every channel open and revalidation tick costs no round trip, and a change
// in either direction is seen once the TTL passes: that TTL is the gateway's
// share of the documented bound.
func TestTwoFactorGateCachesPositiveAndNegativeAnswersForTheTTL(t *testing.T) {
	f := newTwoFactorFixture()
	ctx := context.Background()
	for range 5 {
		if err := f.gate.Check(ctx, "tea-on", sshIdentity("enrolled")); err != nil {
			t.Fatal(err)
		}
		requireRefusedWith(t, f.gate.Check(ctx, "tea-on", sshIdentity("unenrolled")), "tea-on")
	}
	if got := f.asker.Questions(); got != 2 {
		t.Fatalf("questions = %d, want one per member", got)
	}

	// The unenrolled member enrolls and the enrolled one removes their last factor.
	f.asker.Enroll("unenrolled", true)
	f.asker.Enroll("enrolled", false)

	f.clock.Advance(TwoFactorCacheTTL - time.Second)
	if err := f.gate.Check(ctx, "tea-on", sshIdentity("enrolled")); err != nil {
		t.Fatalf("inside the TTL the cached positive must hold: %v", err)
	}
	requireRefusedWith(t, f.gate.Check(ctx, "tea-on", sshIdentity("unenrolled")), "tea-on")

	f.clock.Advance(2 * time.Second)
	requireRefusedWith(t, f.gate.Check(ctx, "tea-on", sshIdentity("enrolled")), "tea-on")
	if err := f.gate.Check(ctx, "tea-on", sshIdentity("unenrolled")); err != nil {
		t.Fatalf("after the TTL an enrolled-since member must be admitted: %v", err)
	}
}

func TestTwoFactorGateFailsClosedAndNeverCachesAFailure(t *testing.T) {
	f := newTwoFactorFixture()
	ctx := context.Background()
	f.asker.Fail(errors.New("bex-api unreachable"))
	for range 3 {
		if err := f.gate.Check(ctx, "tea-on", sshIdentity("enrolled")); !errors.Is(err, core.ErrAuthzUnavailable) {
			t.Fatalf("with bex-api unreachable = %v, want ErrAuthzUnavailable (fail closed)", err)
		}
	}
	if got := f.asker.Questions(); got != 3 {
		t.Fatalf("a failure was cached: %d questions for 3 checks", got)
	}
	f.asker.Fail(nil)
	if err := f.gate.Check(ctx, "tea-on", sshIdentity("enrolled")); err != nil {
		t.Fatalf("recovered bex-api must be asked again at once: %v", err)
	}

	// No way to ask at all (BEX_SANDBOX_EXEC_SECRET unset) is the same refusal.
	blind := &TwoFactorGate{Now: f.clock.Now}
	if err := blind.Check(ctx, "tea-off", sshIdentity("enrolled")); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("a gate that cannot ask = %v, want ErrAuthzUnavailable", err)
	}
}

// An agent_session:<id> object names no workspace. The gateway reads it from
// the session row it can already SELECT, never over the network.
func TestTwoFactorGateResolvesAgentSessionObjectsLocally(t *testing.T) {
	f := newTwoFactorFixture()
	ctx := context.Background()
	if ws, err := f.gate.ObjectWorkspace(ctx, "agent_session:ags-1"); err != nil || ws != "tea-on" {
		t.Fatalf("ObjectWorkspace = (%q, %v), want tea-on", ws, err)
	}
	if ws, err := f.gate.ObjectWorkspace(ctx, "agent_session:ags-gone"); err != nil || ws != "" {
		t.Fatalf("a session that does not exist = (%q, %v), want nothing to enforce", ws, err)
	}
	for _, object := range []string{"app:srv-1", "sandbox:x", "agent_sessions:ags-1"} {
		if ws, err := f.gate.ObjectWorkspace(ctx, object); err != nil || ws != "" {
			t.Fatalf("ObjectWorkspace(%q) = (%q, %v), want not resolved", object, ws, err)
		}
	}
	if f.asker.Questions() != 0 {
		t.Fatal("resolving an object went to bex-api")
	}

	broken := &TwoFactorGate{Asker: f.asker, Sessions: failingSessions{}}
	if _, err := broken.ObjectWorkspace(ctx, "agent_session:ags-1"); err == nil {
		t.Fatal("a session lookup failure must fail closed, not resolve to no workspace")
	}
	unwired := &TwoFactorGate{Asker: f.asker}
	if _, err := unwired.ObjectWorkspace(ctx, "agent_session:ags-1"); err == nil {
		t.Fatal("a gate with no session reader must fail closed for agent_session objects")
	}
}

type failingSessions struct{}

func (failingSessions) AgentSessionWorkspace(context.Context, string) (string, error) {
	return "", errors.New("db down")
}

// The funnel runs the gate inside the gateway process: an agent-session object
// resolves its workspace through the gate and an unenrolled member is refused
// after a positive OpenFGA decision, for fresh and cached decisions alike.
func TestTwoFactorGateEnforcesThroughTheAuthorizationFunnel(t *testing.T) {
	f := newTwoFactorFixture()
	base := &core.Base{Authz: allowAll{}, TwoFactor: f.gate}
	for _, tc := range []struct {
		subject string
		refused bool
	}{{"enrolled", false}, {"unenrolled", true}} {
		ctx := core.WithIdentity(context.Background(), core.Identity{Subject: tc.subject, Method: "agent-attach"})
		for name, authorize := range map[string]func() error{
			"AuthorizeOn":      func() error { return base.AuthorizeOn(ctx, core.RelCanOperate, "agent_session:ags-1") },
			"AuthorizeFreshOn": func() error { return base.AuthorizeFreshOn(ctx, core.RelCanOperate, "agent_session:ags-1") },
		} {
			err := authorize()
			if tc.refused {
				requireRefusedWith(t, err, "tea-on")
			} else if err != nil {
				t.Fatalf("%s as %s = %v", name, tc.subject, err)
			}
		}
	}
}

type allowAll struct{}

func (allowAll) Check(context.Context, string, string, string) (bool, error) { return true, nil }

func (allowAll) CheckFresh(context.Context, string, string, string) (bool, error) {
	return true, nil
}

// The gate counts what it refuses by a closed reason, never by workspace or
// subject, whether the refusal was asked for or served from the cache.
func TestTwoFactorGateCountsRefusalsWithoutWorkspaceLabels(t *testing.T) {
	f := newTwoFactorFixture()
	ctx := context.Background()
	_ = f.gate.Check(ctx, "tea-on", sshIdentity("enrolled"))
	for range 2 {
		_ = f.gate.Check(ctx, "tea-on", sshIdentity("unenrolled")) // asked once, then cached: both refusals count
	}
	f.asker.Fail(errors.New("down"))
	_ = f.gate.Check(ctx, "tea-on", sshIdentity("someone-new"))
	_ = f.gate.Check(ctx, "tea-on", sshIdentity("enrolled")) // cached positive: not a refusal

	if got := gatewaytest.MetricValue(t, f.reg, "bex_ssh_gateway_two_factor_refusals_total", map[string]string{"result": "not_enrolled"}); got != 2 {
		t.Fatalf("not_enrolled refusals = %v, want 2", got)
	}
	if got := gatewaytest.MetricValue(t, f.reg, "bex_ssh_gateway_two_factor_refusals_total", map[string]string{"result": "unavailable"}); got != 1 {
		t.Fatalf("unavailable refusals = %v, want 1", got)
	}
}

// holdingAsker answers "required, allowed" only once released, counting the
// questions, so concurrent callers can pile onto one in-flight question.
type holdingAsker struct {
	release chan struct{}
	asked   atomic.Int32
}

func (a *holdingAsker) Ask(context.Context, string, string, string) (bool, bool, error) {
	a.asked.Add(1)
	<-a.release
	return true, true, nil
}

// Concurrent checks of one member on one workspace put one question to bex-api,
// and a caller that gives up (its connection closed) does not fail the others.
func TestTwoFactorGateCollapsesConcurrentQuestionsForOneMember(t *testing.T) {
	asker := &holdingAsker{release: make(chan struct{})}
	gate := &TwoFactorGate{Asker: asker}
	const callers = 12
	var started atomic.Int32
	var wg sync.WaitGroup
	cancelled, cancel := context.WithCancel(context.Background())
	errs := make([]error, callers)
	for i := range callers {
		ctx := context.Background()
		if i == 0 {
			ctx = cancelled
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			started.Add(1)
			errs[i] = gate.Check(ctx, "tea-on", sshIdentity("enrolled"))
		}()
	}
	for started.Load() < callers {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	cancel()
	close(asker.release)
	wg.Wait()

	if got := asker.asked.Load(); got != 1 {
		t.Fatalf("questions for %d concurrent checks of one member = %d, want 1", callers, got)
	}
	if !errors.Is(errs[0], core.ErrAuthzUnavailable) {
		t.Fatalf("the caller that gave up = %v, want fail-closed", errs[0])
	}
	for i, err := range errs[1:] {
		if err != nil {
			t.Fatalf("caller %d = %v, want admitted: one caller's cancellation must not fail the rest", i+1, err)
		}
	}
	// The answer the flight fetched is cached for everyone after it.
	if err := gate.Check(context.Background(), "tea-on", sshIdentity("enrolled")); err != nil || asker.asked.Load() != 1 {
		t.Fatalf("a check after the flight = %v with %d questions, want the cached answer", err, asker.asked.Load())
	}
	// A different member is a different question.
	if err := gate.Check(context.Background(), "tea-on", sshIdentity("other")); err != nil || asker.asked.Load() != 2 {
		t.Fatalf("another member = %v with %d questions, want their own", err, asker.asked.Load())
	}
}
