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

package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// twofactor_test.go pins the shared enforcement point of w22/m402
// (docs/ADR012-auth.md §9a): every Base authorization entry point that returns
// a positive decision then asks Base.TwoFactor about the workspace it resolved,
// so REST, GraphQL and MCP cannot drift. The gate's own policy (who counts as
// enrolled, the caches) is internal/twofactor's; these tests use a recording
// gate and assert only WHICH workspace each entry point hands it, and what a
// refusal looks like.

// recordingGate refuses every workspace in refuse and records every call. It
// also resolves agent_session objects through objectWorkspace, the optional
// TwoFactorObjectResolver capability.
type recordingGate struct {
	mu              sync.Mutex
	calls           []string
	refuse          map[string]bool
	err             error
	objectWorkspace map[string]string
}

func (g *recordingGate) Check(_ context.Context, workspaceID string, id Identity) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, workspaceID+"|"+id.Subject)
	if g.err != nil {
		return g.err
	}
	if g.refuse[workspaceID] {
		return NewWorkspaceTwoFactorRequiredError(workspaceID)
	}
	return nil
}

func (g *recordingGate) ObjectWorkspace(_ context.Context, object string) (string, error) {
	return g.objectWorkspace[object], nil
}

func (g *recordingGate) seen() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.calls...)
}

func assertTwoFactorRefusal(t *testing.T, err error, workspaceID string) {
	t.Helper()
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want it to wrap ErrForbidden so every surface answers 403", err)
	}
	var coded *CodedError
	if !errors.As(err, &coded) || coded.Code != CodeWorkspaceTwoFactorRequired {
		t.Fatalf("err = %v, want the coded %s", err, CodeWorkspaceTwoFactorRequired)
	}
	if coded.Params["workspaceId"] != workspaceID {
		t.Fatalf("params = %v, want workspaceId %q", coded.Params, workspaceID)
	}
}

func TestTwoFactorErrorIsTheSameCoded403OnRESTAndGraphQLAndMCP(t *testing.T) {
	err := NewWorkspaceTwoFactorRequiredError("tea-a")

	rec := httptest.NewRecorder()
	WriteErr(rec, err)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("REST status = %d, want 403", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"code":"WORKSPACE_2FA_REQUIRED"`, `"workspaceId":"tea-a"`} {
		if !strings.Contains(body, want) {
			t.Errorf("REST body %s lacks %s", body, want)
		}
	}
	if ext := err.Extensions(); ext["code"] != CodeWorkspaceTwoFactorRequired || ext["workspaceId"] != "tea-a" {
		t.Errorf("GraphQL extensions = %v, want code and workspaceId", ext)
	}
	if msg := MCPError(err).Error(); !strings.HasPrefix(msg, CodeWorkspaceTwoFactorRequired+":") {
		t.Errorf("MCP text = %q, want the code prefixed", msg)
	}
}

func TestTwoFactorAdminAndUnavailableErrorsAre409(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  *CodedError
		code string
	}{
		{"admin not enrolled", NewWorkspaceTwoFactorAdminNotEnrolledError(), CodeWorkspaceTwoFactorAdminNotEnrolled},
		{"unavailable", NewWorkspaceTwoFactorUnavailableError(), CodeWorkspaceTwoFactorUnavailable},
	} {
		if tc.err.Code != tc.code || !errors.Is(tc.err, ErrConflict) {
			t.Errorf("%s = %+v, want a 409 coded %s", tc.name, tc.err, tc.code)
		}
	}
	if CodeWorkspaceTwoFactorRequired != "WORKSPACE_2FA_REQUIRED" ||
		CodeWorkspaceTwoFactorAdminNotEnrolled != "WORKSPACE_2FA_ADMIN_NOT_ENROLLED" ||
		CodeWorkspaceTwoFactorUnavailable != "WORKSPACE_2FA_UNAVAILABLE" {
		t.Error("the three codes are wire vocabulary (ADR012 §9a) and must not drift")
	}
}

func TestAuthorizeAsksTheGateOnlyAfterAPositiveDecision(t *testing.T) {
	gate := &recordingGate{refuse: map[string]bool{"tea-a": true}}
	sink := &fakeAuditSink{}
	b := resolvingBase()
	b.TwoFactor = gate
	b.Audit = sink

	err := b.Authorize(actingCtx(), RelCanManage)
	assertTwoFactorRefusal(t, err, "tea-a")
	if got := gate.seen(); len(got) != 1 || got[0] != "tea-a|identity-a" {
		t.Fatalf("gate calls = %v, want one call for tea-a as identity-a", got)
	}
	// A write verb refused by the gate is a DENIED row, never an allowed one.
	if sink.len() != 1 || sink.events[0].Outcome != AuditDenied {
		t.Fatalf("audit = %+v, want exactly one denied row", sink.events)
	}

	// An OpenFGA denial never reaches the gate: the member is refused for the
	// role they lack, and the 2FA code must not leak which workspaces enforce it.
	gate2 := &recordingGate{refuse: map[string]bool{"tea-a": true}}
	b2 := resolvingBase()
	b2.Authz = fakeDenyChecker{}
	b2.TwoFactor = gate2
	err = b2.Authorize(actingCtx(), RelCanView)
	var coded *CodedError
	if !errors.Is(err, ErrForbidden) || !errors.As(err, &coded) || coded.Code == CodeWorkspaceTwoFactorRequired {
		t.Fatalf("an OpenFGA denial = %v, want the plain ErrForbidden", err)
	}
	if got := gate2.seen(); len(got) != 0 {
		t.Fatalf("gate consulted %v for an OpenFGA-denied caller", got)
	}
}

func TestAnAllowingGateLeavesAuthorizeUntouched(t *testing.T) {
	gate := &recordingGate{}
	sink := &fakeAuditSink{}
	b := resolvingBase()
	b.TwoFactor = gate
	b.Audit = sink
	if err := b.Authorize(actingCtx(), RelCanManage); err != nil {
		t.Fatalf("Authorize = %v, want nil", err)
	}
	if sink.len() != 1 || sink.events[0].Outcome != AuditAllowed {
		t.Fatalf("audit = %+v, want the one allowed write row", sink.events)
	}
}

func TestNilGateIsOff(t *testing.T) {
	b := resolvingBase()
	if err := b.Authorize(actingCtx(), RelCanView); err != nil {
		t.Fatalf("Authorize with no gate = %v, want nil", err)
	}
}

func TestGateFailureFailsClosed(t *testing.T) {
	b := resolvingBase()
	b.TwoFactor = &recordingGate{err: ErrAuthzUnavailable}
	if err := b.Authorize(actingCtx(), RelCanView); !errors.Is(err, ErrAuthzUnavailable) {
		t.Fatalf("Authorize with a failing gate = %v, want ErrAuthzUnavailable", err)
	}
}

func TestEveryEntryPointHandsTheGateTheWorkspaceItResolved(t *testing.T) {
	ownApp := sampleApp("web", "tea-a")
	otherApp := sampleApp("other", "tea-a")
	otherApp.Labels[LabelAppID] = "srv-other"
	cases := []struct {
		name string
		call func(b *Base, ctx context.Context) error
		want string
	}{
		{"Authorize", func(b *Base, ctx context.Context) error { return b.Authorize(ctx, RelCanView) }, "tea-a"},
		{"AuthorizeTarget", func(b *Base, ctx context.Context) error {
			return b.AuthorizeTarget(ctx, RelCanView, ServiceTarget("web"))
		}, "tea-a"},
		{"AuthorizeWorkspace named", func(b *Base, ctx context.Context) error {
			_, err := b.AuthorizeWorkspace(ctx, RelCanView, "tea-a")
			return err
		}, "tea-a"},
		{"AuthorizeOn", func(b *Base, ctx context.Context) error {
			return b.AuthorizeOn(ctx, RelCanView, WorkspaceObject("tea-a"))
		}, "tea-a"},
		{"AuthorizeApp", func(b *Base, ctx context.Context) error {
			_, err := b.AuthorizeApp(ctx, RelCanView, "web")
			return err
		}, "tea-a"},
		{"AuthorizeDatabase", func(b *Base, ctx context.Context) error {
			_, err := b.AuthorizeDatabase(ctx, RelCanView, "dpg-x")
			return err
		}, "tea-a"},
		{"AuthorizeKeyValue", func(b *Base, ctx context.Context) error {
			_, err := b.AuthorizeKeyValue(ctx, RelCanView, "red-x")
			return err
		}, "tea-a"},
		{"AuthorizeFreshOn", func(b *Base, ctx context.Context) error {
			return b.AuthorizeFreshOn(ctx, RelCanView, WorkspaceObject("tea-a"))
		}, "tea-a"},
		{"AuthorizeFresh", func(b *Base, ctx context.Context) error { return b.AuthorizeFresh(ctx, RelCanView) }, "tea-a"},
		{"AuthorizeAppFresh", func(b *Base, ctx context.Context) error { return b.AuthorizeAppFresh(ctx, RelCanView, ownApp) }, "tea-a"},
		{"AuthorizeDatabaseFresh", func(b *Base, ctx context.Context) error {
			return b.AuthorizeDatabaseFresh(ctx, RelCanView, tenantDatabase("default", "dpg-x", "tea-a"))
		}, "tea-a"},
		{"AuthorizeKeyValueFresh", func(b *Base, ctx context.Context) error {
			return b.AuthorizeKeyValueFresh(ctx, RelCanView, tenantKeyValue("default", "red-x", "tea-a"))
		}, "tea-a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gate := &recordingGate{refuse: map[string]bool{tc.want: true}}
			b := resolvingBase(ownApp, otherApp, tenantDatabase("default", "dpg-x", "tea-a"), tenantKeyValue("default", "red-x", "tea-a"))
			b.TwoFactor = gate
			err := tc.call(b, actingCtx())
			assertTwoFactorRefusal(t, err, tc.want)
		})
	}
}

// A request that names another of the caller's workspaces is checked against
// THAT workspace, never the caller's default: the setting is per workspace.
func TestANamedWorkspaceIsCheckedNotTheDefault(t *testing.T) {
	gate := &recordingGate{refuse: map[string]bool{"tea-b": true}}
	b := resolvingBase()
	b.Workspace = twoWorkspaceMember{}
	b.TwoFactor = gate

	if err := b.Authorize(WithWorkspace(actingCtx(), "tea-a"), RelCanView); err != nil {
		t.Fatalf("default workspace tea-a does not enforce: %v", err)
	}
	err := b.Authorize(WithWorkspace(actingCtx(), "tea-b"), RelCanView)
	assertTwoFactorRefusal(t, err, "tea-b")
}

type twoWorkspaceMember struct{}

func (twoWorkspaceMember) Tenant(context.Context, Identity) (string, bool) { return "tea-a", true }
func (twoWorkspaceMember) IsMember(_ context.Context, _ Identity, tenantID string) (bool, error) {
	return tenantID == "tea-a" || tenantID == "tea-b", nil
}

// An App that lives in another of the caller's workspaces is reached through
// the cross-workspace gate, which must apply the enforcing workspace's setting
// too: a member blocked in B does not see B's services through A's lookup.
func TestCrossWorkspaceAccessAppliesTheOwnersSetting(t *testing.T) {
	gate := &recordingGate{refuse: map[string]bool{"tea-b": true}}
	b := resolvingBase()
	b.Workspace = twoWorkspaceMember{}
	b.TwoFactor = gate
	err := b.AuthorizeLabeled(actingCtx(), RelCanView, map[string]string{LabelTenant: "tea-b"})
	assertTwoFactorRefusal(t, err, "tea-b")
	if err := b.AuthorizeLabeled(actingCtx(), RelCanView, map[string]string{LabelTenant: "tea-a"}); err != nil {
		t.Fatalf("the acting workspace does not enforce: %v", err)
	}
}

// Agent-session objects carry no workspace in their name; a gate that can
// resolve them (TwoFactorObjectResolver) is asked for the session's workspace.
func TestAgentSessionObjectsResolveToTheSessionsWorkspace(t *testing.T) {
	gate := &recordingGate{
		refuse:          map[string]bool{"tea-b": true},
		objectWorkspace: map[string]string{"agent_session:ags-1": "tea-b", "agent_session:ags-2": "tea-a"},
	}
	b := resolvingBase()
	b.TwoFactor = gate
	for _, call := range []func(string) error{
		func(o string) error { return b.AuthorizeOn(actingCtx(), RelCanOperate, o) },
		func(o string) error { return b.AuthorizeFreshOn(actingCtx(), RelCanOperate, o) },
	} {
		assertTwoFactorRefusal(t, call("agent_session:ags-1"), "tea-b")
		if err := call("agent_session:ags-2"); err != nil {
			t.Fatalf("a session of a workspace that does not enforce = %v", err)
		}
		// An object the resolver does not know is not a workspace resource.
		if err := call("agent_session:unknown"); err != nil {
			t.Fatalf("an unresolvable object = %v, want it left to OpenFGA", err)
		}
	}
}

func TestExemptOperationsSkipTheGate(t *testing.T) {
	gate := &recordingGate{refuse: map[string]bool{"tea-a": true}}
	b := resolvingBase()
	b.TwoFactor = gate
	ctx := WithTwoFactorExempt(actingCtx())
	if err := b.Authorize(ctx, RelCanView); err != nil {
		t.Fatalf("an exempt operation = %v, want nil", err)
	}
	if _, err := b.AuthorizeWorkspace(ctx, RelCanView, "tea-a"); err != nil {
		t.Fatalf("an exempt AuthorizeWorkspace = %v, want nil", err)
	}
	if got := gate.seen(); len(got) != 0 {
		t.Fatalf("gate consulted %v for an exempt operation", got)
	}
	// The same call without the marker is still refused.
	assertTwoFactorRefusal(t, b.Authorize(actingCtx(), RelCanView), "tea-a")
}

// Probes (Can, CanDecision) shape a response and never gate a verb: they stay
// off the gate so a blocked member's capability read is not a Kratos lookup.
func TestProbesDoNotConsultTheGate(t *testing.T) {
	gate := &recordingGate{refuse: map[string]bool{"tea-a": true}}
	b := resolvingBase()
	b.TwoFactor = gate
	if !b.Can(actingCtx(), RelCanView) {
		t.Fatal("Can = false, want the probe to report the OpenFGA relation alone")
	}
	if got := gate.seen(); len(got) != 0 {
		t.Fatalf("probe consulted the gate: %v", got)
	}
}

// No identity in the context (a platform background caller) has nobody to ask
// about: the gate is never invoked and the decision is unchanged.
func TestACallerWithoutAnIdentityIsNeverGated(t *testing.T) {
	gate := &recordingGate{refuse: map[string]bool{"tea-a": true}}
	b := resolvingBase()
	b.Authz = nil
	b.TwoFactor = gate
	if err := b.AuthorizeOn(context.Background(), RelCanView, WorkspaceObject("tea-a")); err != nil {
		t.Fatalf("no identity = %v, want nil with authz off", err)
	}
	if got := gate.seen(); len(got) != 0 {
		t.Fatalf("gate consulted %v without an identity", got)
	}
}
