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

package twofactor

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// gate_test.go pins the policy of docs/ADR012-auth.md §9a: which principal is
// checked, what counts as enrolled, the 60 s caches with an injectable clock,
// and the failure mode. The funnel that calls the gate is core's
// (core/twofactor_test.go); the surfaces are internal/api's.

type fakeSettings struct {
	mu    sync.Mutex
	on    map[string]bool
	err   error
	calls int
}

func (f *fakeSettings) RequireTwoFactor(_ context.Context, workspaceID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.on[workspaceID], f.err
}

func (f *fakeSettings) set(workspaceID string, on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.on[workspaceID] = on
}

func (f *fakeSettings) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// fakeEnrollment answers per subject; a subject absent from both maps is not a
// Kratos identity (human=false), the platform-principal case.
type fakeEnrollment struct {
	mu       sync.Mutex
	enrolled map[string]bool
	humans   map[string]bool
	err      error
	calls    map[string]int
	// release, when set, holds every lookup until it is closed, so a test can
	// pile concurrent callers onto one in-flight lookup.
	release chan struct{}
}

func newEnrollment(humans ...string) *fakeEnrollment {
	f := &fakeEnrollment{enrolled: map[string]bool{}, humans: map[string]bool{}, calls: map[string]int{}}
	for _, h := range humans {
		f.humans[h] = true
	}
	return f
}

func (f *fakeEnrollment) MFAEnrolled(_ context.Context, subject string) (bool, bool, error) {
	if f.release != nil {
		<-f.release
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[subject]++
	if f.err != nil {
		return false, false, f.err
	}
	return f.enrolled[subject], f.humans[subject], nil
}

func (f *fakeEnrollment) enroll(subject string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enrolled[subject] = true
}

func (f *fakeEnrollment) unenroll(subject string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enrolled[subject] = false
}

func (f *fakeEnrollment) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		n += c
	}
	return n
}

// fakeKeys maps an API key's client id to its creator. A client absent from
// creators and from keys is not a bex API key at all (a platform client).
type fakeKeys struct {
	creators map[string]string
	keys     map[string]bool
	err      error
	calls    int
}

func (f *fakeKeys) KeyCreator(_ context.Context, clientID string) (string, bool, error) {
	f.calls++
	if f.err != nil {
		return "", false, f.err
	}
	if c, ok := f.creators[clientID]; ok {
		return c, true, nil
	}
	return "", f.keys[clientID], nil
}

type fakeSessions map[string]string

func (f fakeSessions) AgentSessionWorkspace(_ context.Context, id string) (string, error) {
	return f[id], nil
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

type fixture struct {
	gate       *Gate
	settings   *fakeSettings
	enrollment *fakeEnrollment
	keys       *fakeKeys
	clock      *clock
}

func newFixture() *fixture {
	f := &fixture{
		settings:   &fakeSettings{on: map[string]bool{"tea-on": true}},
		enrollment: newEnrollment("human-unenrolled", "human-enrolled"),
		keys:       &fakeKeys{creators: map[string]string{}, keys: map[string]bool{}},
		clock:      &clock{t: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)},
	}
	f.enrollment.enroll("human-enrolled")
	f.gate = New(Config{
		Settings: f.settings, Enrollment: f.enrollment, Keys: f.keys,
		Sessions: fakeSessions{"ags-1": "tea-on"}, Now: f.clock.now,
	})
	return f
}

func session(subject string) core.Identity {
	return core.Identity{Subject: subject, Method: "session", Human: true}
}

func requireRefused(t *testing.T, err error, workspaceID string) {
	t.Helper()
	var coded *core.CodedError
	if !errors.Is(err, core.ErrForbidden) || !errors.As(err, &coded) || coded.Code != core.CodeWorkspaceTwoFactorRequired {
		t.Fatalf("err = %v, want the coded %s 403", err, core.CodeWorkspaceTwoFactorRequired)
	}
	if coded.Params["workspaceId"] != workspaceID {
		t.Fatalf("params = %v, want workspaceId %q", coded.Params, workspaceID)
	}
}

func TestASettingOffWorkspaceCostsNoLookupAtAll(t *testing.T) {
	f := newFixture()
	for _, id := range []core.Identity{
		session("human-unenrolled"),
		{Subject: "key-1", ClientID: "key-1", Method: "oauth2"},
	} {
		if err := f.gate.Check(context.Background(), "tea-off", id); err != nil {
			t.Fatalf("Check on a workspace that does not require 2FA = %v", err)
		}
	}
	if f.enrollment.total() != 0 || f.keys.calls != 0 {
		t.Fatalf("lookups with the setting off: enrollment=%d keys=%d, want none", f.enrollment.total(), f.keys.calls)
	}
	// The default workspace and an empty id are never enforceable.
	if err := f.gate.Check(context.Background(), core.DefaultTenant, session("human-unenrolled")); err != nil {
		t.Fatalf("default workspace = %v", err)
	}
	if err := f.gate.Check(context.Background(), "", session("human-unenrolled")); err != nil {
		t.Fatalf("empty workspace = %v", err)
	}
}

func TestAnUnenrolledMemberIsRefusedAndAnEnrolledOneIsNot(t *testing.T) {
	f := newFixture()
	requireRefused(t, f.gate.Check(context.Background(), "tea-on", session("human-unenrolled")), "tea-on")
	if err := f.gate.Check(context.Background(), "tea-on", session("human-enrolled")); err != nil {
		t.Fatalf("an enrolled member = %v, want allowed", err)
	}
	// Their other workspace, with the setting off, is unaffected.
	if err := f.gate.Check(context.Background(), "tea-off", session("human-unenrolled")); err != nil {
		t.Fatalf("a workspace that does not require 2FA = %v", err)
	}
}

func TestAHumanOAuthTokenIsCheckedAsItsSubject(t *testing.T) {
	f := newFixture()
	id := core.Identity{Subject: "human-unenrolled", ClientID: "dashboard", Method: "oauth2", Human: true}
	requireRefused(t, f.gate.Check(context.Background(), "tea-on", id), "tea-on")
}

func TestAPlatformPrincipalThatCannotEnrollIsExempt(t *testing.T) {
	f := newFixture()
	// A subject Kratos does not know (a platform machine principal such as the
	// bootstrap client) is not human: nothing to enroll, so nothing to block.
	for _, id := range []core.Identity{
		{Subject: "bex-bootstrap", Method: "session"},
		{Subject: "system", Method: "ssh"},
	} {
		if err := f.gate.Check(context.Background(), "tea-on", id); err != nil {
			t.Fatalf("%s = %v, want exempt", id.Subject, err)
		}
	}
	// A client that is not a bex API key (an operator-provisioned platform
	// client) has no creator to blame and is exempt too.
	if err := f.gate.Check(context.Background(), "tea-on", core.Identity{Subject: "platform-client", ClientID: "platform-client", Method: "oauth2"}); err != nil {
		t.Fatalf("a non-API-key machine client = %v, want exempt", err)
	}
	// An identity with no subject has nobody to check.
	if err := f.gate.Check(context.Background(), "tea-on", core.Identity{}); err != nil {
		t.Fatalf("empty identity = %v", err)
	}
}

func TestAnAPIKeyBorrowsItsCreatorsSecondFactor(t *testing.T) {
	f := newFixture()
	f.keys.creators["key-by-unenrolled"] = "human-unenrolled"
	f.keys.creators["key-by-enrolled"] = "human-enrolled"
	key := func(id string) core.Identity { return core.Identity{Subject: id, ClientID: id, Method: "oauth2"} }

	requireRefused(t, f.gate.Check(context.Background(), "tea-on", key("key-by-unenrolled")), "tea-on")
	if err := f.gate.Check(context.Background(), "tea-on", key("key-by-enrolled")); err != nil {
		t.Fatalf("a key created by an enrolled member = %v, want allowed", err)
	}
	// The key works again once its creator enrolls, within the cache TTL.
	f.enrollment.enroll("human-unenrolled")
	f.clock.advance(TTL + time.Second)
	if err := f.gate.Check(context.Background(), "tea-on", key("key-by-unenrolled")); err != nil {
		t.Fatalf("after the creator enrolled and the TTL passed = %v, want allowed", err)
	}
}

func TestAKeyWithNoRecordedCreatorIsRefusedOnlyWhenTheWorkspaceEnforces(t *testing.T) {
	f := newFixture()
	f.keys.keys["old-key"] = true // a bex API key minted before the created-by stamp
	old := core.Identity{Subject: "old-key", ClientID: "old-key", Method: "oauth2"}

	if err := f.gate.Check(context.Background(), "tea-off", old); err != nil {
		t.Fatalf("an unattributed key where 2FA is not required = %v, want allowed", err)
	}
	err := f.gate.Check(context.Background(), "tea-on", old)
	requireRefused(t, err, "tea-on")
	if got := err.Error(); got == core.NewWorkspaceTwoFactorRequiredError("tea-on").Error() {
		t.Errorf("message = %q, want one that tells the owner to re-create the key", got)
	}
}

func TestBackgroundSessionIdentitiesAreCheckedAsTheirSubject(t *testing.T) {
	// agentsessions/spawn.go and the automation scheduler fake Method "session"
	// and run as the member who started the work; Human is not set. The gate
	// must key on the subject, not on Human or the method.
	f := newFixture()
	bg := core.Identity{Subject: "human-unenrolled", Method: "session"}
	requireRefused(t, f.gate.Check(context.Background(), "tea-on", bg), "tea-on")
	bgOK := core.Identity{Subject: "human-enrolled", Method: "session"}
	if err := f.gate.Check(context.Background(), "tea-on", bgOK); err != nil {
		t.Fatalf("an enrolled member's background work = %v", err)
	}
}

func TestTicketAndGatewayIdentitiesAreCheckedAsTheirSubject(t *testing.T) {
	f := newFixture()
	for _, method := range []string{"ssh", "shell", "sandbox-exec", "agent-attach", "sandbox-connect", "sandbox-file-connect"} {
		requireRefused(t, f.gate.Check(context.Background(), "tea-on", core.Identity{Subject: "human-unenrolled", Method: method}), "tea-on")
		if err := f.gate.Check(context.Background(), "tea-on", core.Identity{Subject: "human-enrolled", Method: method}); err != nil {
			t.Fatalf("%s as an enrolled member = %v", method, err)
		}
	}
}

func TestLookupFailuresFailClosedOnlyWhenTheSettingIsOn(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(*fixture)
		id     core.Identity
	}{
		{"enrollment lookup errors", func(f *fixture) { f.enrollment.err = errors.New("kratos down") }, session("human-enrolled")},
		{"no identity provider wired", func(f *fixture) {
			f.gate = New(Config{Settings: f.settings, Now: f.clock.now})
		}, session("human-enrolled")},
		{"key creator lookup errors", func(f *fixture) { f.keys.err = errors.New("hydra down") }, core.Identity{Subject: "k", ClientID: "k", Method: "oauth2"}},
		{"key resolution not wired", func(f *fixture) {
			f.gate = New(Config{Settings: f.settings, Enrollment: f.enrollment, Now: f.clock.now})
		}, core.Identity{Subject: "k", ClientID: "k", Method: "oauth2"}},
		{"setting lookup errors", func(f *fixture) { f.settings.err = errors.New("db down") }, session("human-enrolled")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			tc.break_(f)
			if err := f.gate.Check(context.Background(), "tea-on", tc.id); !errors.Is(err, core.ErrAuthzUnavailable) {
				t.Fatalf("with the setting on = %v, want ErrAuthzUnavailable (fail closed)", err)
			}
			if tc.name == "setting lookup errors" {
				return // nothing is known to be off
			}
			if err := f.gate.Check(context.Background(), "tea-off", tc.id); err != nil {
				t.Fatalf("with the setting off = %v, want allowed: a broken lookup must not break workspaces that never asked", err)
			}
		})
	}
}

func TestTheEnrollmentAnswerIsCachedForSixtySecondsPerIdentity(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	for range 3 {
		requireRefused(t, f.gate.Check(ctx, "tea-on", session("human-unenrolled")), "tea-on")
	}
	if got := f.enrollment.calls["human-unenrolled"]; got != 1 {
		t.Fatalf("lookups within the TTL = %d, want 1", got)
	}
	// Enrolling does not lift the block until the TTL elapses: the documented
	// bound on "access returns after enrolling".
	f.enrollment.enroll("human-unenrolled")
	f.clock.advance(TTL - time.Second)
	requireRefused(t, f.gate.Check(ctx, "tea-on", session("human-unenrolled")), "tea-on")
	f.clock.advance(2 * time.Second)
	if err := f.gate.Check(ctx, "tea-on", session("human-unenrolled")); err != nil {
		t.Fatalf("after the TTL = %v, want the new enrollment seen", err)
	}
	if got := f.enrollment.calls["human-unenrolled"]; got != 2 {
		t.Fatalf("lookups across the TTL = %d, want 2", got)
	}
	// Other identities have their own entries.
	if err := f.gate.Check(ctx, "tea-on", session("human-enrolled")); err != nil {
		t.Fatal(err)
	}
	if got := f.enrollment.calls["human-enrolled"]; got != 1 {
		t.Fatalf("a second identity's lookups = %d, want 1", got)
	}
}

func TestALookupErrorIsNeverCached(t *testing.T) {
	f := newFixture()
	f.enrollment.err = errors.New("kratos down")
	if err := f.gate.Check(context.Background(), "tea-on", session("human-enrolled")); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("err = %v", err)
	}
	f.enrollment.err = nil
	if err := f.gate.Check(context.Background(), "tea-on", session("human-enrolled")); err != nil {
		t.Fatalf("the next call after the outage = %v, want the outage forgotten", err)
	}
}

func TestTheSettingIsCachedPerWorkspaceAndInvalidatedLocally(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	id := session("human-unenrolled")
	if err := f.gate.Check(ctx, "tea-late", id); err != nil {
		t.Fatal(err)
	}
	f.gate.Check(ctx, "tea-late", id) //nolint:errcheck // cache probe
	if got := f.settings.count(); got != 1 {
		t.Fatalf("setting reads within the TTL = %d, want 1", got)
	}

	// An admin turns it on: another replica may lag up to the TTL, but this
	// replica drops its entry the moment the mutation runs.
	f.settings.set("tea-late", true)
	if err := f.gate.Check(ctx, "tea-late", id); err != nil {
		t.Fatalf("before invalidation = %v, want the cached off answer", err)
	}
	f.gate.Invalidate("tea-late")
	requireRefused(t, f.gate.Check(ctx, "tea-late", id), "tea-late")

	// Without a local invalidation the TTL is the bound.
	f.settings.set("tea-late", false)
	requireRefused(t, f.gate.Check(ctx, "tea-late", id), "tea-late")
	f.clock.advance(TTL + time.Second)
	if err := f.gate.Check(ctx, "tea-late", id); err != nil {
		t.Fatalf("after the TTL = %v, want the setting re-read", err)
	}
}

func TestObjectWorkspaceResolvesAgentSessionsOnly(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	if ws, err := f.gate.ObjectWorkspace(ctx, "agent_session:ags-1"); err != nil || ws != "tea-on" {
		t.Fatalf("agent session = (%q, %v), want tea-on", ws, err)
	}
	if ws, err := f.gate.ObjectWorkspace(ctx, "agent_session:ags-missing"); err != nil || ws != "" {
		t.Fatalf("unknown session = (%q, %v), want no workspace", ws, err)
	}
	if ws, err := f.gate.ObjectWorkspace(ctx, "service:web"); err != nil || ws != "" {
		t.Fatalf("a non-session object = (%q, %v), want no workspace", ws, err)
	}
	var _ core.TwoFactorObjectResolver = f.gate
	var _ core.TwoFactorGate = f.gate
}

func TestRefusalsAreCountedByBoundedReasonNeverByWorkspace(t *testing.T) {
	reg := prometheus.NewRegistry()
	f := newFixture()
	f.keys.keys["old-key"] = true
	f.gate = New(Config{
		Settings: f.settings, Enrollment: f.enrollment, Keys: f.keys, Now: f.clock.now,
		Metrics: NewMetrics(reg),
	})
	ctx := context.Background()
	requireRefused(t, f.gate.Check(ctx, "tea-on", session("human-unenrolled")), "tea-on")
	requireRefused(t, f.gate.Check(ctx, "tea-on", session("human-unenrolled")), "tea-on")
	requireRefused(t, f.gate.Check(ctx, "tea-on", core.Identity{Subject: "old-key", ClientID: "old-key", Method: "oauth2"}), "tea-on")
	f.enrollment.err = errors.New("kratos down")
	if err := f.gate.Check(ctx, "tea-on", session("human-enrolled")); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatal(err)
	}
	// An allowed call counts nothing.
	f.enrollment.err = nil
	f.clock.advance(TTL + time.Second)
	if err := f.gate.Check(ctx, "tea-on", session("human-enrolled")); err != nil {
		t.Fatal(err)
	}

	families, err := reg.Gather()
	if err != nil || len(families) != 1 {
		t.Fatalf("gather = (%v, %v), want the one refusal family", families, err)
	}
	got := map[string]float64{}
	for _, m := range families[0].Metric {
		if len(m.Label) != 1 || m.Label[0].GetName() != "reason" {
			t.Fatalf("labels = %v, want only the bounded reason (a workspace id would be unbounded)", m.Label)
		}
		got[m.Label[0].GetValue()] = m.Counter.GetValue()
	}
	want := map[string]float64{"not_enrolled": 2, "unattributed_key": 1, "lookup_unavailable": 1}
	for reason, n := range want {
		if got[reason] != n {
			t.Errorf("refusals{reason=%q} = %v, want %v (all: %v)", reason, got[reason], n, got)
		}
	}
}

// Required is the setting alone, read through the same 60 s cache Check uses,
// for the SSH gateway's verb: it lets the gateway cache "this workspace never
// asked" for every member at once, and cost no enrollment lookup at all.
func TestRequiredReadsTheSettingThroughTheSameCacheAsCheck(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	for workspace, want := range map[string]bool{"tea-on": true, "tea-off": false, core.DefaultTenant: false, "": false} {
		if got, err := f.gate.Required(ctx, workspace); err != nil || got != want {
			t.Fatalf("Required(%q) = (%v, %v), want (%v, nil)", workspace, got, err, want)
		}
	}
	reads := f.settings.count()
	if err := f.gate.Check(ctx, "tea-on", session("human-enrolled")); err != nil {
		t.Fatal(err)
	}
	if f.settings.count() != reads {
		t.Fatalf("Check re-read a setting Required had just cached: %d reads, want %d", f.settings.count(), reads)
	}
	if f.enrollment.total() != 1 {
		t.Fatalf("enrollment lookups = %d, want only Check's one", f.enrollment.total())
	}

	f.settings.err = errors.New("db down")
	f.clock.advance(TTL + time.Second)
	if got, err := f.gate.Required(ctx, "tea-on"); !errors.Is(err, core.ErrAuthzUnavailable) || got {
		t.Fatalf("Required with the setting unreadable = (%v, %v), want (false, ErrAuthzUnavailable): never \"not required\"", got, err)
	}
}

// A key's creator that Kratos no longer knows (a deleted account) must not turn
// the key into a hole around the setting: the borrowed subject is not human, so
// the key is refused like one with no recorded creator, under the same reason.
// A platform principal's own non-human subject stays exempt.
func TestAKeyWhoseCreatorWasDeletedIsRefusedLikeAnUnattributedKey(t *testing.T) {
	reg := prometheus.NewRegistry()
	f := newFixture()
	f.gate = New(Config{
		Settings: f.settings, Enrollment: f.enrollment, Keys: f.keys, Now: f.clock.now,
		Metrics: NewMetrics(reg),
	})
	f.keys.creators["orphan-key"] = "deleted-human" // not a Kratos identity any more
	ctx := context.Background()
	orphan := core.Identity{Subject: "orphan-key", ClientID: "orphan-key", Method: "oauth2"}

	err := f.gate.Check(ctx, "tea-on", orphan)
	requireRefused(t, err, "tea-on")
	if got := err.Error(); got == core.NewWorkspaceTwoFactorRequiredError("tea-on").Error() {
		t.Errorf("message = %q, want the re-create-the-key message an unattributed key gets", got)
	}
	if err := f.gate.Check(ctx, "tea-off", orphan); err != nil {
		t.Fatalf("an orphaned key where 2FA is not required = %v, want allowed", err)
	}
	// The same non-human subject, not borrowed, is a platform principal: exempt.
	if err := f.gate.Check(ctx, "tea-on", core.Identity{Subject: "deleted-human", Method: "session"}); err != nil {
		t.Fatalf("a platform principal with a non-Kratos subject = %v, want exempt", err)
	}

	families, gatherErr := reg.Gather()
	if gatherErr != nil || len(families) != 1 {
		t.Fatalf("gather = (%v, %v)", families, gatherErr)
	}
	got := map[string]float64{}
	for _, m := range families[0].Metric {
		got[m.Label[0].GetValue()] = m.Counter.GetValue()
	}
	if got["unattributed_key"] != 1 || len(got) != 1 {
		t.Fatalf("refusals = %v, want exactly one unattributed_key", got)
	}
}

// An API key whose creator was removed from the workspace but still exists in
// Kratos is checked on its creator's second factor alone.
func TestAKeyWhoseCreatorLeftTheWorkspaceIsStillCheckedOnTheirSecondFactor(t *testing.T) {
	f := newFixture()
	f.keys.creators["key-by-departed"] = "human-unenrolled" // no membership is consulted here
	requireRefused(t, f.gate.Check(context.Background(), "tea-on", core.Identity{Subject: "key-by-departed", ClientID: "key-by-departed", Method: "oauth2"}), "tea-on")
}

// A machine token that names neither a client nor a subject cannot be
// attributed to anyone: it fails closed instead of being exempt.
func TestAMachineIdentityWithNoClientAndNoSubjectIsRefused(t *testing.T) {
	f := newFixture()
	anonymous := core.Identity{Method: "oauth2"}
	requireRefused(t, f.gate.Check(context.Background(), "tea-on", anonymous), "tea-on")
	if err := f.gate.Check(context.Background(), "tea-off", anonymous); err != nil {
		t.Fatalf("with the setting off = %v, want allowed", err)
	}
	if f.keys.calls != 0 {
		t.Fatalf("key lookups = %d, want none: there is no client id to look up", f.keys.calls)
	}
}

// With no session reader an agent_session object's workspace cannot be named,
// and "" would skip the check: it fails closed, as the gateway's gate does.
func TestObjectWorkspaceWithoutASessionReaderFailsClosed(t *testing.T) {
	gate := New(Config{Settings: &fakeSettings{on: map[string]bool{}}})
	if ws, err := gate.ObjectWorkspace(context.Background(), "agent_session:ags-1"); err == nil {
		t.Fatalf("ObjectWorkspace without a session reader = (%q, nil), want an error", ws)
	}
	if ws, err := gate.ObjectWorkspace(context.Background(), "service:web"); err != nil || ws != "" {
		t.Fatalf("a non-session object = (%q, %v), want no workspace and no error", ws, err)
	}
}

// Concurrent misses for one identity share a single Kratos lookup, and a
// caller that gives up does not fail the callers still waiting on it.
func TestConcurrentChecksOfOneIdentityShareOneEnrollmentLookup(t *testing.T) {
	f := newFixture()
	f.enrollment.release = make(chan struct{})
	var started atomic.Int32
	const callers = 12
	results := make(chan error, callers)
	cancelled, cancel := context.WithCancel(context.Background())
	for i := range callers {
		ctx := context.Background()
		if i == 0 {
			ctx = cancelled // the first caller will give up mid-lookup
		}
		go func() {
			started.Add(1)
			results <- f.gate.Check(ctx, "tea-on", session("human-unenrolled"))
		}()
	}
	for started.Load() < callers {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // every caller is now queued on the one lookup
	cancel()
	close(f.enrollment.release)

	var cancelledCallers, refused int
	for range callers {
		err := <-results
		switch {
		case errors.Is(err, core.ErrAuthzUnavailable):
			cancelledCallers++ // fail closed for the caller whose own context ended
		case errors.Is(err, core.ErrForbidden):
			refused++
		default:
			t.Fatalf("Check = %v, want the not-enrolled refusal or, for the cancelled caller, fail-closed", err)
		}
	}
	if cancelledCallers > 1 {
		t.Fatalf("%d callers failed closed, want at most the one whose context was cancelled", cancelledCallers)
	}
	if got := f.enrollment.calls["human-unenrolled"]; got != 1 {
		t.Fatalf("Kratos lookups for %d concurrent callers = %d, want 1", callers, got)
	}
	if refused < callers-1 {
		t.Fatalf("refused = %d, want every caller but the cancelled one", refused)
	}
}

// A member who removes their last second factor keeps access until the cached
// enrollment expires, which is the documented 60 s bound, and is blocked again
// the moment it does: the clock crosses the TTL without sleeping.
func TestAMemberWhoRemovesTheirLastFactorIsBlockedAgainWhenTheCacheExpires(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	member := session("human-enrolled")
	if err := f.gate.Check(ctx, "tea-on", member); err != nil {
		t.Fatalf("an enrolled member = %v", err)
	}

	f.enrollment.unenroll("human-enrolled")
	f.clock.advance(TTL - time.Second)
	if err := f.gate.Check(ctx, "tea-on", member); err != nil {
		t.Fatalf("inside the TTL the cached enrollment holds = %v, want still allowed", err)
	}
	f.clock.advance(2 * time.Second)
	requireRefused(t, f.gate.Check(ctx, "tea-on", member), "tea-on")
	if got := f.enrollment.calls["human-enrolled"]; got != 2 {
		t.Fatalf("Kratos lookups = %d, want a second one exactly when the TTL passed", got)
	}
}

// Kratos failing is a refusal where the setting is on (never a silent allow,
// and never remembered once it recovers), and costs nothing, not even a lookup,
// where the setting is off.
func TestAKratosOutageRefusesWhereRequiredAndIsNeverAskedWhereNot(t *testing.T) {
	reg := prometheus.NewRegistry()
	f := newFixture()
	f.gate = New(Config{Settings: f.settings, Enrollment: f.enrollment, Keys: f.keys, Now: f.clock.now, Metrics: NewMetrics(reg)})
	f.enrollment.err = errors.New("kratos down")
	ctx := context.Background()

	if err := f.gate.Check(ctx, "tea-off", session("human-unenrolled")); err != nil {
		t.Fatalf("setting off during an outage = %v, want allowed", err)
	}
	if got := f.enrollment.total(); got != 0 {
		t.Fatalf("Kratos lookups with the setting off = %d, want none", got)
	}

	for range 2 {
		if err := f.gate.Check(ctx, "tea-on", session("human-enrolled")); !errors.Is(err, core.ErrAuthzUnavailable) {
			t.Fatalf("setting on during an outage = %v, want ErrAuthzUnavailable", err)
		}
	}
	if got := f.enrollment.total(); got != 2 {
		t.Fatalf("Kratos lookups = %d, want one per refused call: a failure is never cached", got)
	}
	f.enrollment.err = nil
	if err := f.gate.Check(ctx, "tea-on", session("human-enrolled")); err != nil {
		t.Fatalf("after Kratos recovered = %v, want allowed at once", err)
	}

	families, err := reg.Gather()
	if err != nil || len(families) != 1 || len(families[0].Metric) != 1 ||
		families[0].Metric[0].Label[0].GetValue() != "lookup_unavailable" || families[0].Metric[0].Counter.GetValue() != 2 {
		t.Fatalf("metrics = %v (%v), want lookup_unavailable counted twice and nothing else", families, err)
	}
}

// The default workspace is the legacy tenant: nothing enforces there even if its
// row somehow says so, and the setting is never even read.
func TestTheDefaultWorkspaceIsNeverEnforcedEvenIfItsSettingIsOn(t *testing.T) {
	f := newFixture()
	f.settings.set(core.DefaultTenant, true)
	f.settings.set("", true)
	ctx := context.Background()
	for _, workspace := range []string{core.DefaultTenant, ""} {
		if err := f.gate.Check(ctx, workspace, session("human-unenrolled")); err != nil {
			t.Fatalf("Check(%q) = %v, want allowed: no workspace to enforce in", workspace, err)
		}
		if on, err := f.gate.Required(ctx, workspace); err != nil || on {
			t.Fatalf("Required(%q) = (%v, %v), want (false, nil)", workspace, on, err)
		}
	}
	if f.settings.count() != 0 || f.enrollment.total() != 0 {
		t.Fatalf("lookups for the default workspace: settings=%d enrollment=%d, want none", f.settings.count(), f.enrollment.total())
	}
}
