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

package members

// m33_test.go covers the w1/m33 parity-completion behavior: seat usage,
// resend-invite, token acceptance, MFA enrichment, and the member-verb audit
// detail (targets + typed roles).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// auditRecorder collects every audit event a verb emits.
type auditRecorder struct {
	events []core.AuditEvent
}

func (a *auditRecorder) Record(_ context.Context, ev core.AuditEvent) error {
	a.events = append(a.events, ev)
	return nil
}

func withAudit(s *Service) *auditRecorder {
	rec := &auditRecorder{}
	s.Base.Audit = rec
	return rec
}

func TestSeatUsageCountsMembersPlusPendingInvites(t *testing.T) {
	st := newFakeStore(store.PlanHobby)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)

	u, err := s.SeatUsage(ctxWith("admin-1"), "tea-1")
	if err != nil {
		t.Fatalf("seat usage: %v", err)
	}
	if u.Used != 1 || u.Limit != 1 {
		t.Errorf("hobby single member: used=%d limit=%d, want 1/1", u.Used, u.Limit)
	}

	// An outstanding invite consumes a seat — the same formula the invite cap
	// enforces, so display and refusal can never disagree.
	st.invites["inv-x"] = store.Invite{ID: "inv-x", TenantID: "tea-1", Email: "x@example.com", Role: "admin", ExpiresAt: time.Now().Add(time.Hour)}
	u, err = s.SeatUsage(ctxWith("admin-1"), "tea-1")
	if err != nil {
		t.Fatalf("seat usage: %v", err)
	}
	if u.Used != 2 {
		t.Errorf("used with pending invite = %d, want 2", u.Used)
	}
}

func TestSeatUsageUnlimitedPlanReportsZeroLimit(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)
	u, err := s.SeatUsage(ctxWith("admin-1"), "tea-1")
	if err != nil {
		t.Fatalf("seat usage: %v", err)
	}
	if u.Limit != 0 {
		t.Errorf("pro limit = %d, want 0 (unlimited)", u.Limit)
	}
}

func TestSeatUsageIsViewerVisible(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, roleChecker{relation: "viewer"})
	if _, err := s.SeatUsage(ctxWith("viewer-1"), "tea-1"); err != nil {
		t.Fatalf("viewer should see seat usage: %v", err)
	}
}

func TestResendInviteRefreshesExpiryRotatesTokenAndMails(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	m := &fakeMailer{}
	s := svc(st, newFakeGranter(), m, nil)
	s.InviteTTL = 7 * 24 * time.Hour
	s.InviteBaseURL = "https://dash.example"

	orig, err := s.Invite(ctxWith("admin-1"), "tea-1", "late@example.com", "developer")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	token := st.invites[orig.ID].Token
	// Simulate a lapsed invite.
	lapsed := st.invites[orig.ID]
	lapsed.ExpiresAt = time.Now().Add(-time.Hour)
	st.invites[orig.ID] = lapsed

	resent, err := s.ResendInvite(ctxWith("admin-1"), "tea-1", orig.ID)
	if err != nil {
		t.Fatalf("resend: %v", err)
	}
	if resent.ID != orig.ID {
		t.Errorf("resend churned the invite id: %s -> %s", orig.ID, resent.ID)
	}
	// w1/041: only sha256(token) is at rest, so resend cannot reproduce the old
	// plaintext — it mints a fresh token and the resent mail's link supersedes
	// the original.
	if rotated := st.invites[orig.ID].Token; rotated == token || rotated == "" {
		t.Errorf("resend token = %q (orig %q), want a freshly minted one", rotated, token)
	}
	if !st.invites[orig.ID].ExpiresAt.After(time.Now()) {
		t.Error("resend did not refresh the expiry")
	}
	if m.calls != 2 || m.to != "late@example.com" {
		t.Errorf("mail: calls=%d to=%q, want a second delivery", m.calls, m.to)
	}
	if !strings.Contains(m.body, st.invites[orig.ID].Token) {
		t.Error("resent mail does not carry the fresh token's link")
	}
}

func TestResendInviteUnknownOrAcceptedIs404(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)

	if _, err := s.ResendInvite(ctxWith("admin-1"), "tea-1", "inv-nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown invite: %v, want ErrNotFound", err)
	}
	now := time.Now()
	st.invites["inv-done"] = store.Invite{ID: "inv-done", TenantID: "tea-1", Email: "a@example.com", Role: "developer", AcceptedAt: &now, ExpiresAt: now.Add(time.Hour)}
	if _, err := s.ResendInvite(ctxWith("admin-1"), "tea-1", "inv-done"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("accepted invite: %v, want ErrNotFound", err)
	}
}

func TestResendInviteIsAdminOnly(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	s := svc(st, newFakeGranter(), nil, denyChecker{})
	if _, err := s.ResendInvite(ctxWith("viewer-1"), "tea-1", "inv-1"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("resend by non-admin: %v, want ErrForbidden", err)
	}
}

func TestAcceptInviteJoinsCrossEmailAndGrantsRole(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	g := newFakeGranter()
	s := svc(st, g, nil, nil)

	inv, err := s.Invite(ctxWith("admin-1"), "tea-1", "alias@example.com", "developer")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	token := st.invites[inv.ID].Token

	// The recipient signed up under a DIFFERENT identity/email — the token is
	// the capability.
	acc, err := s.AcceptInvite(ctxWith("newcomer-1"), token)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if acc.WorkspaceID != "tea-1" || acc.Role != "DEVELOPER" || acc.WorkspaceName != "acme" {
		t.Errorf("accepted view = %+v", acc)
	}
	if m, ok := st.members["newcomer-1"]; !ok || m.Role != "developer" {
		t.Errorf("membership not created: %+v", st.members)
	}
	if len(g.granted) != 1 || g.granted[0] != key("developer", "tea-1", "user:newcomer-1") {
		t.Errorf("FGA grant = %v", g.granted)
	}
}

func TestAcceptInviteNamedRefusals(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)

	if _, err := s.AcceptInvite(ctxWith("someone"), "no-such-token"); !errors.Is(err, core.ErrNotFound) || codedErrorCode(err) != InviteErrorInvalid {
		t.Errorf("unknown token: %v code=%q, want ErrNotFound/%s", err, codedErrorCode(err), InviteErrorInvalid)
	}
	if _, err := s.AcceptInvite(ctxWith("someone"), "  "); !errors.Is(err, core.ErrBadRequest) || codedErrorCode(err) != InviteErrorInvalid {
		t.Errorf("blank token: %v code=%q, want ErrBadRequest/%s", err, codedErrorCode(err), InviteErrorInvalid)
	}

	inv, err := s.Invite(ctxWith("admin-1"), "tea-1", "twice@example.com", "developer")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	token := st.invites[inv.ID].Token
	if _, err := s.AcceptInvite(ctxWith("first"), token); err != nil {
		t.Fatalf("first accept: %v", err)
	}
	if _, err := s.AcceptInvite(ctxWith("second"), token); !errors.Is(err, core.ErrBadRequest) || codedErrorCode(err) != InviteErrorAlreadyAccepted {
		t.Errorf("second accept: %v code=%q, want %s", err, codedErrorCode(err), InviteErrorAlreadyAccepted)
	}
}

func TestAcceptInviteExpiredTokenRefused(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)
	st.invites["inv-old"] = store.Invite{
		ID: "inv-old", TenantID: "tea-1", Email: "old@example.com", Role: "developer",
		Token: "tok-old", ExpiresAt: time.Now().Add(-time.Minute),
	}
	if _, err := s.AcceptInvite(ctxWith("someone"), "tok-old"); !errors.Is(err, core.ErrBadRequest) || codedErrorCode(err) != InviteErrorExpired {
		t.Errorf("expired token: %v code=%q, want %s", err, codedErrorCode(err), InviteErrorExpired)
	}
}

func TestAcceptInvitePlanRefusalHasStableCode(t *testing.T) {
	st := newFakeStore(store.PlanHobby)
	st.seedMember("admin-1", "admin")
	st.invites["inv-full"] = store.Invite{
		ID: "inv-full", TenantID: "tea-1", Email: "second@example.com", Role: "admin",
		Token: "tok-full", ExpiresAt: time.Now().Add(time.Hour),
	}
	_, err := svc(st, newFakeGranter(), nil, nil).AcceptInvite(ctxWith("second"), "tok-full")
	if !errors.Is(err, core.ErrBadRequest) || codedErrorCode(err) != InviteErrorPlanLimit {
		t.Fatalf("plan refusal: %v code=%q, want %s", err, codedErrorCode(err), InviteErrorPlanLimit)
	}
}

func codedErrorCode(err error) string {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

func TestAcceptInviteGraphQLPublishesStableCode(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: s.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: s.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{
		Schema: schema, RequestString: `mutation { acceptWorkspaceInvite(token: "missing") { workspaceId } }`, Context: ctxWith("someone"),
	})
	if len(result.Errors) != 1 || result.Errors[0].Extensions["code"] != InviteErrorInvalid {
		t.Fatalf("GraphQL invite error = %+v, want code %s", result.Errors, InviteErrorInvalid)
	}
}

func TestListEnrichesMFAFromIdentityLookup(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("dev-1", "developer")
	s := svc(st, newFakeGranter(), nil, nil)
	s.Identities = fakeIdentities{
		"admin-1": {Email: "admin@example.com", MFAEnabled: true},
		"dev-1":   {Email: "dev@example.com"},
	}
	ms, err := s.List(ctxWith("admin-1"), "tea-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	byEmail := map[string]bool{}
	for _, m := range ms {
		byEmail[m.Email] = m.MFAEnabled
	}
	if !byEmail["admin@example.com"] || byEmail["dev@example.com"] {
		t.Errorf("mfa enrichment = %v", byEmail)
	}
}

func TestListMFAOmittedWhenLookupUnwired(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil) // Identities nil
	ms, err := s.List(ctxWith("admin-1"), "tea-1")
	if err != nil {
		t.Fatalf("list must succeed without the identity reader: %v", err)
	}
	if len(ms) != 1 || ms[0].MFAEnabled {
		t.Errorf("members = %+v, want honest-false mfaEnabled", ms)
	}
}

// --- audit detail (t007) ------------------------------------------------------

func TestInviteAuditRowCarriesInviteTargetAndRole(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)
	rec := withAudit(s)

	inv, err := s.Invite(ctxWith("admin-1"), "tea-1", "new@example.com", "developer")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if len(rec.events) != 1 {
		t.Fatalf("audit events = %d, want exactly 1 (deferred success row)", len(rec.events))
	}
	ev := rec.events[0]
	if ev.Verb != core.AuditVerbMemberInvited || ev.Target != core.InviteTarget(inv.ID) ||
		ev.TargetName != "new@example.com" || ev.RoleTo == nil || *ev.RoleTo != "developer" {
		t.Errorf("invite audit row = %+v", ev)
	}
	if ev.Outcome != core.AuditAllowed || ev.Caller != "admin-1" {
		t.Errorf("invite audit outcome/caller = %s/%s", ev.Outcome, ev.Caller)
	}
}

func TestChangeRoleAuditRowCarriesMemberTargetAndRolePair(t *testing.T) {
	st := newFakeStore(store.PlanScale)
	st.seedMember("admin-1", "admin")
	st.seedMember("dev-1", "developer")
	s := svc(st, newFakeGranter(), nil, nil)
	rec := withAudit(s)

	if _, err := s.ChangeRole(ctxWith("admin-1"), "tea-1", "dev-1", "viewer"); err != nil {
		t.Fatalf("change role: %v", err)
	}
	if len(rec.events) != 1 {
		t.Fatalf("audit events = %d, want exactly 1", len(rec.events))
	}
	ev := rec.events[0]
	if ev.Verb != core.AuditVerbMemberRoleChanged || ev.Target != core.MemberTarget("dev-1") {
		t.Errorf("role-change audit row = %+v", ev)
	}
	if ev.RoleFrom == nil || *ev.RoleFrom != "developer" || ev.RoleTo == nil || *ev.RoleTo != "viewer" {
		t.Errorf("role pair = %v -> %v, want developer -> viewer", ev.RoleFrom, ev.RoleTo)
	}
}

func TestChangeRoleRefusedLeavesNoAllowedRow(t *testing.T) {
	st := newFakeStore(store.PlanPro) // pro: viewer not assignable
	st.seedMember("admin-1", "admin")
	st.seedMember("dev-1", "developer")
	s := svc(st, newFakeGranter(), nil, nil)
	rec := withAudit(s)

	if _, err := s.ChangeRole(ctxWith("admin-1"), "tea-1", "dev-1", "viewer"); err == nil {
		t.Fatal("plan-gated role change should refuse")
	}
	for _, ev := range rec.events {
		if ev.Outcome == core.AuditAllowed {
			t.Errorf("refused change recorded an allowed row: %+v", ev)
		}
	}
}

func TestRemoveAndRevokeCarryTargets(t *testing.T) {
	st := newFakeStore(store.PlanScale)
	st.seedMember("admin-1", "admin")
	st.seedMember("dev-1", "developer")
	st.invites["inv-1"] = store.Invite{ID: "inv-1", TenantID: "tea-1", Email: "p@example.com", Role: "viewer", ExpiresAt: time.Now().Add(time.Hour)}
	s := svc(st, newFakeGranter(), nil, nil)
	rec := withAudit(s)

	if err := s.Remove(ctxWith("admin-1"), "tea-1", "dev-1"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := s.RevokeInvite(ctxWith("admin-1"), "tea-1", "inv-1"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if len(rec.events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(rec.events))
	}
	if rec.events[0].Target != core.MemberTarget("dev-1") {
		t.Errorf("remove target = %q", rec.events[0].Target)
	}
	if rec.events[1].Target != core.InviteTarget("inv-1") {
		t.Errorf("revoke target = %q", rec.events[1].Target)
	}
}

func TestAcceptInviteRecordsAcceptedRow(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)
	rec := withAudit(s)

	inv, err := s.Invite(ctxWith("admin-1"), "tea-1", "joiner@example.com", "developer")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	token := st.invites[inv.ID].Token
	if _, err := s.AcceptInvite(ctxWith("newcomer-1"), token); err != nil {
		t.Fatalf("accept: %v", err)
	}
	var accepted *core.AuditEvent
	for i := range rec.events {
		if rec.events[i].Verb == core.AuditVerbInviteAccepted {
			accepted = &rec.events[i]
		}
	}
	if accepted == nil {
		t.Fatalf("no members.AcceptInvite row in %+v", rec.events)
	}
	if accepted.Caller != "newcomer-1" || accepted.Target != core.InviteTarget(inv.ID) ||
		accepted.TargetName != "joiner@example.com" || accepted.RoleTo == nil || *accepted.RoleTo != "developer" {
		t.Errorf("accepted row = %+v", *accepted)
	}
}

// --- cross-surface parity (t008) -----------------------------------------------

// TestThreeSurfaceParity_SeatUsageAndMFA asserts REST, GraphQL, and MCP return
// the same seat usage and per-member mfaEnabled for the same workspace — the
// w1/m33 additions must not drift across the three fragments any more than
// userId/email may (TestThreeSurfaceParity_UserIDAndEmail).
func TestThreeSurfaceParity_SeatUsageAndMFA(t *testing.T) {
	st := newFakeStore("hobby")
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)
	s.Identities = fakeIdentities{"admin-1": {Email: "admin@example.com", MFAEnabled: true}}
	ctx := ctxWith("admin-1")

	// REST
	mux := http.NewServeMux()
	s.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/workspaces/tea-1/seat-usage", nil).WithContext(ctx))
	if rec.Code != http.StatusOK {
		t.Fatalf("REST seat-usage status = %d, body %s", rec.Code, rec.Body)
	}
	var restSeats SeatUsageView
	if err := json.Unmarshal(rec.Body.Bytes(), &restSeats); err != nil {
		t.Fatalf("REST decode: %v", err)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/workspaces/tea-1/members", nil).WithContext(ctx))
	var restMembers []MemberView
	if err := json.Unmarshal(rec.Body.Bytes(), &restMembers); err != nil || len(restMembers) != 1 {
		t.Fatalf("REST members: %v %+v", err, restMembers)
	}

	// GraphQL
	query := graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: s.GraphQLQuery()})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: query})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	result := graphql.Do(graphql.Params{
		Schema: schema,
		RequestString: `query($w: String!) {
			workspaceSeatUsage(workspaceId: $w) { used limit }
			workspaceMembers(workspaceId: $w) { mfaEnabled }
		}`,
		VariableValues: map[string]any{"w": "tea-1"},
		Context:        ctx,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("graphql errors: %v", result.Errors)
	}
	b, _ := json.Marshal(result.Data)
	var gqlOut struct {
		WorkspaceSeatUsage SeatUsageView `json:"workspaceSeatUsage"`
		WorkspaceMembers   []struct {
			MFAEnabled bool `json:"mfaEnabled"`
		} `json:"workspaceMembers"`
	}
	if err := json.Unmarshal(b, &gqlOut); err != nil {
		t.Fatalf("graphql decode: %v (%s)", err, b)
	}

	// MCP
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	s.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	mcpCtx := core.WithWorkspace(ctx, "tea-1")
	if _, err := srv.Connect(mcpCtx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil).Connect(mcpCtx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_workspace_seat_usage"})
	if err != nil || res.IsError {
		t.Fatalf("seat-usage tool: err=%v res=%+v", err, res)
	}
	mb, _ := json.Marshal(res.StructuredContent)
	var mcpSeats SeatUsageView
	if err := json.Unmarshal(mb, &mcpSeats); err != nil {
		t.Fatalf("mcp decode: %v (%s)", err, mb)
	}
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_workspace_members"})
	if err != nil || res.IsError {
		t.Fatalf("members tool: err=%v res=%+v", err, res)
	}
	mb, _ = json.Marshal(res.StructuredContent)
	var mcpMembers struct {
		Members []MemberView `json:"members"`
	}
	if err := json.Unmarshal(mb, &mcpMembers); err != nil || len(mcpMembers.Members) != 1 {
		t.Fatalf("mcp members: %v %+v", err, mcpMembers)
	}

	// Compare all three.
	if restSeats != gqlOut.WorkspaceSeatUsage || restSeats != mcpSeats {
		t.Errorf("seat-usage drift: rest=%+v graphql=%+v mcp=%+v", restSeats, gqlOut.WorkspaceSeatUsage, mcpSeats)
	}
	if restSeats.Used != 1 || restSeats.Limit != 1 {
		t.Errorf("hobby seats = %+v, want 1/1", restSeats)
	}
	if !restMembers[0].MFAEnabled || !gqlOut.WorkspaceMembers[0].MFAEnabled || !mcpMembers.Members[0].MFAEnabled {
		t.Errorf("mfaEnabled drift: rest=%v graphql=%v mcp=%v",
			restMembers[0].MFAEnabled, gqlOut.WorkspaceMembers[0].MFAEnabled, mcpMembers.Members[0].MFAEnabled)
	}
}

// TestAcceptInviteRESTRoute pins the token-accept REST shape: not
// workspace-scoped (the token identifies the invite), 200 with the joined
// workspace on success.
func TestAcceptInviteRESTRoute(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, nil)
	inv, err := s.Invite(ctxWith("admin-1"), "tea-1", "rest@example.com", "developer")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	token := st.invites[inv.ID].Token

	mux := http.NewServeMux()
	s.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodPost, "/v1/invites/accept",
		strings.NewReader(`{"token":"`+token+`"}`)).WithContext(ctxWith("newcomer-1"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("accept status = %d, body %s", rec.Code, rec.Body)
	}
	var acc AcceptedInviteView
	if err := json.Unmarshal(rec.Body.Bytes(), &acc); err != nil || acc.WorkspaceID != "tea-1" || acc.Role != "DEVELOPER" {
		t.Fatalf("accepted = %+v (%v)", acc, err)
	}
}
