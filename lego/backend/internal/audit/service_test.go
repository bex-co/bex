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

package audit

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeAllowChecker allows everything.
type fakeAllowChecker struct{}

func (fakeAllowChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

// fakeStore is a spy AuditStore: it records the arguments ListAuditEvents/
// PurgeAuditEvents were called with and returns canned results/errors.
// Mutex-guarded because TestRunWithIntervalSweepsOnStartupAndOnTick reads its
// fields from the test goroutine while the sweep loop writes them from its own.
type fakeStore struct {
	mu           sync.Mutex
	gotWorkspace string
	gotFilter    store.AuditFilter
	listRows     []store.AuditRow
	listErr      error

	gotPurgeBefore time.Time
	purgeN         int64
	purgeErr       error
	purgeCalls     int

	gotSessionPurgeBefore time.Time
	sessionPurgeN         int64
	sessionPurgeErr       error
	sessionPurgeCalls     int

	telemetryPurgeCalls int
	productPurgeCalls   int
	productPurgeBefore  time.Time
}

func (f *fakeStore) ListAuditEvents(_ context.Context, workspaceID string, filter store.AuditFilter) ([]store.AuditRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotWorkspace, f.gotFilter = workspaceID, filter
	return f.listRows, f.listErr
}

func (f *fakeStore) PurgeAuditEvents(_ context.Context, before time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotPurgeBefore = before
	f.purgeCalls++
	return f.purgeN, f.purgeErr
}

func (f *fakeStore) PurgeSSHSessions(_ context.Context, before time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotSessionPurgeBefore = before
	f.sessionPurgeCalls++
	return f.sessionPurgeN, f.sessionPurgeErr
}

func (f *fakeStore) PurgeCLITelemetryEvents(_ context.Context, before time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.telemetryPurgeCalls++
	return 0, nil
}

func (f *fakeStore) calls() (int, time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.purgeCalls, f.gotPurgeBefore
}

// TestListPassesFilterThroughAndScopesToOwner proves List forwards the
// caller-supplied filter unchanged and scopes the store query to ownerID —
// not the caller's own workspace, which is what would leak another
// workspace's trail if List silently substituted it.
func TestListPassesFilterThroughAndScopesToOwner(t *testing.T) {
	fs := &fakeStore{listRows: []store.AuditRow{{ID: "aud-1", WorkspaceID: "tea-owner"}}}
	base := &core.Base{Authz: fakeAllowChecker{}}
	svc := &Service{Base: base, Store: fs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})

	since := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	events, err := svc.List(ctx, "tea-owner", Filter{Since: since, Until: until, Cursor: "aud-cursor", Limit: 7})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if fs.gotWorkspace != "tea-owner" {
		t.Errorf("store queried workspace %q, want tea-owner", fs.gotWorkspace)
	}
	if fs.gotFilter.Since != since || fs.gotFilter.Until != until || fs.gotFilter.Cursor != "aud-cursor" || fs.gotFilter.Limit != 7 {
		t.Errorf("filter not forwarded unchanged: got %+v", fs.gotFilter)
	}
	if len(events) != 1 || events[0].ID != "aud-1" {
		t.Errorf("events = %+v, want the store's single row projected", events)
	}
}

// w8/m138: Render's audit logs are Pro and higher. A Hobby workspace's admin
// gets PLAN_FEATURE_REQUIRED and the store is never read; Pro reads as before.
func TestListFollowsThePlan(t *testing.T) {
	for plan, refused := range map[string]bool{"hobby": true, "pro": false, "enterprise": false} {
		fs := &fakeStore{listRows: []store.AuditRow{{ID: "aud-1", WorkspaceID: "tea-owner"}}}
		base := &core.Base{Authz: fakeAllowChecker{}, Plans: core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return plan, nil })}
		svc := &Service{Base: base, Store: fs}
		ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})
		events, err := svc.List(ctx, "tea-owner", Filter{})
		if !refused {
			if err != nil || len(events) != 1 {
				t.Fatalf("%s: %v (%d events)", plan, err, len(events))
			}
			continue
		}
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != core.PlanFeatureRequiredCode || coded.Params["feature"] != "auditLogs" ||
			err.Error() != "bad request: audit logs require the Pro plan or higher; this workspace is on Hobby" {
			t.Fatalf("hobby: got %v", err)
		}
		if fs.gotWorkspace != "" {
			t.Fatalf("hobby: the store was read")
		}
	}
}

// TestListDirectionMapsToStoreOrdering proves the Render direction enum
// (w4/013) reaches the store as its ordering flag: forward => oldest-first,
// backward/empty => newest-first. Before this, direction was accepted and
// silently ignored — the exact drift ADR018's audit row documented.
func TestListDirectionMapsToStoreOrdering(t *testing.T) {
	for _, tc := range []struct {
		direction   string
		oldestFirst bool
	}{
		{direction: "", oldestFirst: false},
		{direction: DirectionBackward, oldestFirst: false},
		{direction: DirectionForward, oldestFirst: true},
	} {
		fs := &fakeStore{}
		svc := &Service{Base: &core.Base{Authz: fakeAllowChecker{}}, Store: fs}
		ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})
		if _, err := svc.List(ctx, "tea-owner", Filter{Direction: tc.direction}); err != nil {
			t.Fatalf("List(direction=%q): %v", tc.direction, err)
		}
		if fs.gotFilter.OldestFirst != tc.oldestFirst {
			t.Errorf("direction %q: store OldestFirst = %v, want %v", tc.direction, fs.gotFilter.OldestFirst, tc.oldestFirst)
		}
	}
}

// TestListRejectsUnknownDirection is the "nothing accepted is ignored" check
// (w3/m8's principle, w4/013): an unrecognized direction is a named
// core.ErrBadRequest — before the store is ever consulted — never a silent
// newest-first fallback.
func TestListRejectsUnknownDirection(t *testing.T) {
	fs := &fakeStore{}
	svc := &Service{Base: &core.Base{Authz: fakeAllowChecker{}}, Store: fs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})

	_, err := svc.List(ctx, "tea-owner", Filter{Direction: "sideways"})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("List(direction=sideways): err = %v, want ErrBadRequest", err)
	}
	if !strings.Contains(err.Error(), "sideways") {
		t.Errorf("error %q should name the offending value", err)
	}
	if fs.gotWorkspace != "" {
		t.Errorf("store consulted despite invalid direction")
	}
}

// TestListStoreLessIsUnavailable proves the store-off degrade is
// core.ErrAuditUnavailable, not a nil-pointer panic or a silent empty list —
// the DoD's "store-less mode ... 503 reads (omitted, not faked)".
func TestListStoreLessIsUnavailable(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: fakeAllowChecker{}}, Store: nil}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})
	if _, err := svc.List(ctx, "tea-owner", Filter{}); !errors.Is(err, core.ErrAuditUnavailable) {
		t.Fatalf("List with no store: err = %v, want ErrAuditUnavailable", err)
	}
}

func TestRenderMaintenanceAuditTypesAndMetadata(t *testing.T) {
	disabled := false
	toggle := toRenderAuditLog(Event{Verb: "apps.SetMaintenanceMode", MaintenanceModeTo: &disabled})
	if toggle.Event != "MaintenanceModeEnabledEvent" || toggle.Metadata["to"] != "false" {
		t.Fatalf("toggle audit = %+v, want MaintenanceModeEnabledEvent metadata.to=false", toggle)
	}
	uri := toRenderAuditLog(Event{Verb: "apps.SetMaintenanceModeURI"})
	if uri.Event != "MaintenanceModeURIUpdatedEvent" || uri.Metadata == nil || len(uri.Metadata) != 0 {
		t.Fatalf("URI audit = %+v, want MaintenanceModeURIUpdatedEvent with empty metadata", uri)
	}
}

// TestRenderAuditLogShape pins the Render-captured wire shape (w4/m26,
// docs/render-artifacts/audit-logs-api.md): event (not action), the closed
// success|error status enum with the denial preserved in metadata, the
// {type, id} actor object, and the target keyed by kind in the string-map
// metadata.
func TestRenderAuditLogShape(t *testing.T) {
	denied := toRenderAuditLog(Event{
		ID: "aud-1", Caller: "client-1", CallerMethod: "oauth2",
		Verb: "apps.Suspend", Target: "service:my-api", Outcome: string(core.AuditDenied),
	})
	if denied.Event != "SuspendServiceEvent" {
		t.Errorf("event = %q, want SuspendServiceEvent", denied.Event)
	}
	if denied.Status != "error" || denied.Metadata["outcome"] != "denied" {
		t.Errorf("denied row = status %q metadata %v, want error + outcome:denied", denied.Status, denied.Metadata)
	}
	if denied.Actor != (renderAuditActor{Type: "rest_api", ID: "client-1"}) {
		t.Errorf("actor = %+v, want {rest_api client-1}", denied.Actor)
	}
	if denied.Metadata["service"] != "my-api" {
		t.Errorf("metadata = %v, want service:my-api keyed by kind", denied.Metadata)
	}

	allowed := toRenderAuditLog(Event{Verb: "custom.Verb", Caller: "usr-1", CallerMethod: "session", Outcome: "allowed"})
	if allowed.Event != "custom.Verb" {
		t.Errorf("unmapped verb = %q, want passthrough", allowed.Event)
	}
	if allowed.Status != "success" || allowed.Actor.Type != "user" {
		t.Errorf("allowed session row = status %q actor %+v, want success/user", allowed.Status, allowed.Actor)
	}
	if allowed.Metadata == nil || len(allowed.Metadata) != 0 {
		t.Errorf("metadata = %v, want present-and-empty map", allowed.Metadata)
	}
	if actorType("") != "system" || actorType("system") != "system" {
		t.Errorf("unattributed/system callers should map to system")
	}
}

func TestRenderOAuthAuditProvenanceIsAdditive(t *testing.T) {
	oauth := toRenderAuditLog(Event{
		ID: "aud-oauth", Caller: "user-1", CallerMethod: "oauth2",
		Verb: "apps.Suspend", Target: "service:my-api", Outcome: string(core.AuditDenied),
		Relation: core.RelCanOperate, OAuthClientID: "dcr-agent",
		OAuthAudience: "https://api.bex.co/mcp", OAuthScopes: []string{core.ScopeRead},
	})
	if oauth.Metadata["relation"] != core.RelCanOperate {
		t.Errorf("relation = %q, want %s", oauth.Metadata["relation"], core.RelCanOperate)
	}
	if oauth.Metadata["oauthClientId"] != "dcr-agent" {
		t.Errorf("oauthClientId = %q", oauth.Metadata["oauthClientId"])
	}
	if oauth.Metadata["oauthAudience"] != "https://api.bex.co/mcp" {
		t.Errorf("oauthAudience = %q", oauth.Metadata["oauthAudience"])
	}
	if oauth.Metadata["oauthScopes"] != core.ScopeRead {
		t.Errorf("oauthScopes = %q, want %s", oauth.Metadata["oauthScopes"], core.ScopeRead)
	}
	if oauth.Metadata["service"] != "my-api" || oauth.Metadata["outcome"] != "denied" {
		t.Errorf("additive oauth fields must not drop existing metadata: %v", oauth.Metadata)
	}

	session := toRenderAuditLog(Event{
		Verb: "apps.Suspend", Caller: "usr-1", CallerMethod: "session", Outcome: "allowed",
	})
	if _, ok := session.Metadata["oauthClientId"]; ok {
		t.Errorf("session row leaked oauth metadata: %v", session.Metadata)
	}
}

// TestGraphQLTargetNameFallsBackToNull is w10/m5: the AuditLog type returns
// the stored display name (migration 0038) and resolves a pre-0038 row's
// stored "" to null, so the dashboard can fall back to the raw id instead of
// rendering an empty cell.
func TestGraphQLTargetNameFallsBackToNull(t *testing.T) {
	fs := &fakeStore{listRows: []store.AuditRow{
		{ID: "aud-named", WorkspaceID: "tea-owner", TargetName: "my-api"},
		{ID: "aud-pre-0038", WorkspaceID: "tea-owner", TargetName: ""},
	}}
	svc := &Service{Base: &core.Base{Authz: fakeAllowChecker{}}, Store: fs}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ auditLogs(ownerId:"tea-owner") { id targetName } }`,
		Context:       ctx,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	rows := res.Data.(map[string]any)["auditLogs"].([]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	named := rows[0].(map[string]any)
	if named["targetName"] != "my-api" {
		t.Errorf("named row targetName = %v, want my-api", named["targetName"])
	}
	pre := rows[1].(map[string]any)
	if pre["targetName"] != nil {
		t.Errorf("pre-0038 row targetName = %v, want null", pre["targetName"])
	}
}

func TestGraphQLExposesOAuthAuditProvenance(t *testing.T) {
	fs := &fakeStore{listRows: []store.AuditRow{
		{
			ID: "aud-oauth", WorkspaceID: "tea-owner", Caller: "user-1",
			Verb: "apps.Suspend", Relation: core.RelCanOperate,
			OAuthClientID: "dcr-agent", OAuthAudience: "https://api.bex.co/mcp",
			OAuthScopes: []string{core.ScopeRead, core.ScopeWrite},
		},
		{ID: "aud-session", WorkspaceID: "tea-owner", Caller: "user-1"},
	}}
	svc := &Service{Base: &core.Base{Authz: fakeAllowChecker{}}, Store: fs}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ auditLogs(ownerId:"tea-owner") { id relation oauthClientId oauthAudience oauthScopes } }`,
		Context:       ctx,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	rows := res.Data.(map[string]any)["auditLogs"].([]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	oauth := rows[0].(map[string]any)
	if oauth["relation"] != core.RelCanOperate || oauth["oauthClientId"] != "dcr-agent" {
		t.Errorf("oauth row = %v", oauth)
	}
	if oauth["oauthAudience"] != "https://api.bex.co/mcp" {
		t.Errorf("oauthAudience = %v", oauth["oauthAudience"])
	}
	scopes, _ := oauth["oauthScopes"].([]any)
	if len(scopes) != 2 || scopes[0] != core.ScopeRead || scopes[1] != core.ScopeWrite {
		t.Errorf("oauthScopes = %v", oauth["oauthScopes"])
	}
	session := rows[1].(map[string]any)
	if session["relation"] != nil || session["oauthClientId"] != nil || session["oauthAudience"] != nil || session["oauthScopes"] != nil {
		t.Errorf("session row leaked oauth fields: %v", session)
	}
}

// TestListPropagatesStoreError proves a store failure surfaces to the caller
// rather than being swallowed into an empty list (which would look
// indistinguishable from "no events").
func TestListPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("connection reset")
	fs := &fakeStore{listErr: wantErr}
	svc := &Service{Base: &core.Base{Authz: fakeAllowChecker{}}, Store: fs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})
	if _, err := svc.List(ctx, "tea-owner", Filter{}); !errors.Is(err, wantErr) {
		t.Fatalf("List: err = %v, want %v", err, wantErr)
	}
}

// TestPurgeUsesDefaultRetention proves an unset/invalid RetentionDays falls
// back to DefaultRetentionDays (90) rather than purging with a zero/negative
// window (which would delete everything or nothing depending on the SQL
// driver's handling of a zero time.Time).
func TestPurgeUsesDefaultRetention(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	fs := &fakeStore{}
	svc := &Service{Base: &core.Base{Clock: func() time.Time { return now }}, Store: fs}

	svc.purge(context.Background())

	want := now.AddDate(0, 0, -DefaultRetentionDays)
	if !fs.gotPurgeBefore.Equal(want) {
		t.Errorf("purge before = %s, want %s (now - %d default days)", fs.gotPurgeBefore, want, DefaultRetentionDays)
	}
	if !fs.gotSessionPurgeBefore.Equal(want) {
		t.Errorf("SSH-session purge before = %s, want %s", fs.gotSessionPurgeBefore, want)
	}
}

// TestPurgeUsesConfiguredRetention proves a caller-set RetentionDays actually
// changes the purge boundary, not just the default.
func TestPurgeUsesConfiguredRetention(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	fs := &fakeStore{}
	svc := &Service{Base: &core.Base{Clock: func() time.Time { return now }}, Store: fs, RetentionDays: 7}

	svc.purge(context.Background())

	want := now.AddDate(0, 0, -7)
	if !fs.gotPurgeBefore.Equal(want) {
		t.Errorf("purge before = %s, want %s (now - 7 configured days)", fs.gotPurgeBefore, want)
	}
	if !fs.gotSessionPurgeBefore.Equal(want) {
		t.Errorf("SSH-session purge before = %s, want %s", fs.gotSessionPurgeBefore, want)
	}
}

// TestPurgeErrorDoesNotPanicOrRetryImmediately proves a store error during
// the sweep is handled gracefully (logged, loop continues) rather than
// crashing the background goroutine it runs in.
func TestPurgeErrorDoesNotPanicOrRetryImmediately(t *testing.T) {
	fs := &fakeStore{purgeErr: errors.New("db unreachable")}
	svc := &Service{Base: &core.Base{}, Store: fs}
	svc.purge(context.Background()) // must not panic
	if fs.purgeCalls != 1 {
		t.Errorf("purge called the store %d times, want exactly 1", fs.purgeCalls)
	}
	if fs.sessionPurgeCalls != 0 {
		t.Errorf("SSH-session purge called %d times after event purge failed, want 0", fs.sessionPurgeCalls)
	}
}

func TestSSHSessionPurgeErrorDoesNotPanicOrRetryImmediately(t *testing.T) {
	fs := &fakeStore{sessionPurgeErr: errors.New("db unreachable")}
	svc := &Service{Base: &core.Base{}, Store: fs}
	svc.purge(context.Background())
	if fs.purgeCalls != 1 || fs.sessionPurgeCalls != 1 {
		t.Errorf("event/session purge calls = %d/%d, want 1/1", fs.purgeCalls, fs.sessionPurgeCalls)
	}
}

// TestRunWithIntervalSweepsOnStartupAndOnTick proves the loop purges once
// immediately (so a restart doesn't defer the first sweep a full interval)
// and again on every tick, stopping cleanly when ctx is cancelled.
func TestRunWithIntervalSweepsOnStartupAndOnTick(t *testing.T) {
	fs := &fakeStore{}
	svc := &Service{Base: &core.Base{}, Store: fs}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		svc.RunWithInterval(ctx, 10*time.Millisecond)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for {
		if n, _ := fs.calls(); n >= 3 {
			break
		}
		select {
		case <-deadline:
			n, _ := fs.calls()
			t.Fatalf("only %d purge calls after 2s, want at least 3 (startup + ticks)", n)
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunWithInterval did not return after ctx cancellation")
	}
}
func (f *fakeStore) PurgeProductAnalytics(_ context.Context, before time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.productPurgeCalls++
	f.productPurgeBefore = before
	return 0, nil
}

func TestPurgeIncludesProductAnalyticsRetention(t *testing.T) {
	st := &fakeStore{}
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	s := &Service{Base: &core.Base{Clock: func() time.Time { return now }}, Store: st, RetentionDays: 30}
	s.purge(context.Background())
	if st.productPurgeCalls != 1 || !st.productPurgeBefore.Equal(now.AddDate(0, 0, -30)) {
		t.Fatalf("product purge calls=%d before=%s", st.productPurgeCalls, st.productPurgeBefore)
	}
}

type fakeUserIDs map[string]string

func (f fakeUserIDs) LookupOwnerIDs(_ context.Context, subjects []string) (map[string]string, error) {
	out := map[string]string{}
	for _, s := range subjects {
		if id, ok := f[s]; ok {
			out[s] = id
		}
	}
	return out, nil
}

type fakeEmails struct {
	emails map[string]string
	err    error
	asked  []string
}

func (f *fakeEmails) LookupEmails(_ context.Context, subjects []string) (map[string]string, error) {
	f.asked = append(f.asked, subjects...)
	out := map[string]string{}
	for _, s := range subjects {
		if email, ok := f.emails[s]; ok {
			out[s] = email
		}
	}
	return out, f.err
}

// w5/m277 + w9/m450: on Render's REST audit log a human actor's id is the
// usr- id the members list answers for that person and its email is the
// person's; API-key and system actors keep their subject and have no email,
// and a deleted user (no identity left) keeps the usr- id alone.
func TestRenderAuditLogUserActorsCarryTheirUsrIDAndEmail(t *testing.T) {
	events := []Event{
		{ID: "aud-1", Caller: "kratos-a", CallerMethod: "session"},
		{ID: "aud-2", Caller: "client-1", CallerMethod: "oauth2"},
		{ID: "aud-3", Caller: "", CallerMethod: ""},
		{ID: "aud-4", Caller: "deleted:own-00000000000000000009", CallerMethod: "session"},
		// The same human subject through an OAuth token is a rest_api actor:
		// no email, even though the person has one.
		{ID: "aud-5", Caller: "kratos-a", CallerMethod: "oauth2"},
	}
	emails := &fakeEmails{emails: map[string]string{"kratos-a": "a@example.com", "client-1": "never@example.com"}}
	svc := &Service{
		UserIDs: fakeUserIDs{"kratos-a": "own-00000000000000000007", "client-1": "own-00000000000000000008", "deleted:own-00000000000000000009": "own-00000000000000000009"},
		Emails:  emails,
	}
	if err := svc.resolveActors(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	list := toAuditLogList(events)
	if got := list[0].AuditLog.Actor; got != (renderAuditActor{Type: "user", ID: "usr-00000000000000000007", Email: "a@example.com"}) {
		t.Errorf("user actor = %+v, want usr- id and email", got)
	}
	if got := list[1].AuditLog.Actor; got != (renderAuditActor{Type: "rest_api", ID: "client-1"}) {
		t.Errorf("API actor = %+v, want its subject and no email", got)
	}
	if got := list[2].AuditLog.Actor; got != (renderAuditActor{Type: "system"}) {
		t.Errorf("system actor = %+v", got)
	}
	if got := list[3].AuditLog.Actor; got != (renderAuditActor{Type: "user", ID: "usr-00000000000000000009"}) {
		t.Errorf("deleted user actor = %+v, want the usr- id alone", got)
	}
	if got := list[4].AuditLog.Actor; got != (renderAuditActor{Type: "rest_api", ID: "kratos-a"}) {
		t.Errorf("human OAuth actor = %+v, want rest_api with no email", got)
	}
	// One batched read of the page's people, each once; never an API client.
	if want := []string{"kratos-a", "deleted:own-00000000000000000009"}; !slices.Equal(emails.asked, want) {
		t.Errorf("emails asked for %q, want %q", emails.asked, want)
	}
	// Without resolvers the subject stays, as before.
	if got := toAuditLogList([]Event{{Caller: "kratos-a", CallerMethod: "session"}})[0].AuditLog.Actor; got != (renderAuditActor{Type: "user", ID: "kratos-a"}) {
		t.Errorf("no resolver: actor = %+v", got)
	}
}

// An identity outage costs the emails it could not read, never the log.
func TestResolveActorsKeepsTheLogWhenEmailsFail(t *testing.T) {
	events := []Event{
		{ID: "aud-1", Caller: "kratos-a", CallerMethod: "session"},
		{ID: "aud-2", Caller: "kratos-b", CallerMethod: "session"},
	}
	svc := &Service{
		UserIDs: fakeUserIDs{"kratos-a": "own-00000000000000000007", "kratos-b": "own-00000000000000000008"},
		Emails:  &fakeEmails{emails: map[string]string{"kratos-a": "a@example.com"}, err: errors.New("kratos down")},
	}
	if err := svc.resolveActors(context.Background(), events); err != nil {
		t.Fatalf("an email failure failed the log: %v", err)
	}
	if events[0].ActorEmail != "a@example.com" || events[1].ActorEmail != "" || events[1].ActorID != "usr-00000000000000000008" {
		t.Errorf("partial resolution = %+v", events)
	}
}

// w5/183: a rename, a custom-domain verb and an env-group link name what they
// changed in the audit log's metadata, in the keys the service's events use.
func TestAuditMetadataNamesWhatChanged(t *testing.T) {
	rename := toRenderAuditLog(Event{Verb: core.AuditVerbSetDisplayName, Target: "service:srv-1", DisplayNameFrom: "api", DisplayNameTo: "Checkout API"})
	if rename.Event != "UpdateServiceNameEvent" || rename.Metadata["from"] != "api" || rename.Metadata["to"] != "Checkout API" || rename.Metadata["service"] != "srv-1" {
		t.Errorf("rename metadata = %v (%s), want service, from and to", rename.Metadata, rename.Event)
	}
	if domain := toRenderAuditLog(Event{Verb: core.AuditVerbAddDomain, Target: "service:srv-1", Domain: "app.example.com"}); domain.Metadata["domain"] != "app.example.com" {
		t.Errorf("domain metadata = %v, want domain", domain.Metadata)
	}
	link := toRenderAuditLog(Event{Verb: core.AuditVerbLinkService, Target: "service:srv-1", EnvGroupID: "evg-1", EnvGroupName: "shared"})
	if link.Metadata["envGroupId"] != "evg-1" || link.Metadata["envGroupName"] != "shared" {
		t.Errorf("link metadata = %v, want envGroupId and envGroupName", link.Metadata)
	}
	if bare := toRenderAuditLog(Event{Verb: core.AuditVerbSetDisplayName, Target: "service:srv-1"}); len(bare.Metadata) != 1 {
		t.Errorf("a row recorded before the names = %v, want only the service", bare.Metadata)
	}
}

// w22/m402: a workspace admin turning "require two-factor authentication" on or
// off is Render's ChangeTeam2FAEnforcementEvent, and the entry carries the value
// it was set to on REST (metadata.to) and GraphQL (metadata.to), like the
// maintenance toggle.
func TestRenderTwoFactorEnforcementAuditTypeAndMetadata(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		log := toRenderAuditLog(Event{Verb: core.AuditVerbSetRequireTwoFactor, TwoFactorRequiredTo: &enabled, Outcome: "allowed"})
		if log.Event != "ChangeTeam2FAEnforcementEvent" || log.Metadata["to"] != strconv.FormatBool(enabled) {
			t.Fatalf("enabled=%v audit = %+v, want ChangeTeam2FAEnforcementEvent metadata.to=%v", enabled, log, enabled)
		}
	}
	// The denied attempt of a non-admin names the same event, without a value.
	denied := toRenderAuditLog(Event{Verb: core.AuditVerbSetRequireTwoFactor, Outcome: string(core.AuditDenied)})
	if denied.Event != "ChangeTeam2FAEnforcementEvent" || denied.Status != "error" {
		t.Fatalf("denied attempt = %+v", denied)
	}
	if _, has := denied.Metadata["to"]; has {
		t.Errorf("a denied attempt changed nothing and must not claim a value: %v", denied.Metadata)
	}
}

func TestGraphQLExposesTheTwoFactorEnforcementValue(t *testing.T) {
	on := true
	fs := &fakeStore{listRows: []store.AuditRow{
		{ID: "aud-2fa", WorkspaceID: "tea-owner", Verb: core.AuditVerbSetRequireTwoFactor, TwoFactorRequiredTo: &on},
	}}
	svc := &Service{Base: &core.Base{Authz: fakeAllowChecker{}}, Store: fs}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ auditLogs(ownerId:"tea-owner") { action metadata { to } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	row := res.Data.(map[string]any)["auditLogs"].([]any)[0].(map[string]any)
	if row["action"] != "ChangeTeam2FAEnforcementEvent" {
		t.Errorf("action = %v, want ChangeTeam2FAEnforcementEvent", row["action"])
	}
	if md, _ := row["metadata"].(map[string]any); md["to"] != true {
		t.Errorf("metadata = %v, want to=true", row["metadata"])
	}
}

// w4/m321 t011: REST metadata keeps bex's OpenFGA relation only on a denied
// row, where it explains the refusal; an allowed row names its target alone.
func TestRenderMetadataKeepsTheRelationOnlyOnADenial(t *testing.T) {
	allowed := toRenderAuditLog(Event{Verb: "apps.Create", Outcome: string(core.AuditAllowed),
		Relation: core.RelCanCreate, Target: core.ServiceTarget("srv-1"), TargetName: "web"})
	if _, ok := allowed.Metadata["relation"]; ok {
		t.Errorf("allowed row metadata = %v, want no relation", allowed.Metadata)
	}
	if allowed.Metadata["service"] != "srv-1" || allowed.Metadata["targetName"] != "web" {
		t.Errorf("allowed row metadata = %v, want service srv-1 named web", allowed.Metadata)
	}
	denied := toRenderAuditLog(Event{Verb: "apps.Create", Outcome: string(core.AuditDenied), Relation: core.RelCanCreate})
	if denied.Metadata["relation"] != core.RelCanCreate {
		t.Errorf("denied row metadata = %v, want relation %s", denied.Metadata, core.RelCanCreate)
	}
}

// w9/m449: REST answers Render's event for each member, cron job, domain,
// auto-deploy, build-source, env var and environment verb; the actions Render
// has no event for (resending or revoking an invite, verifying a domain) keep
// bex's names. An env var row names its keys in metadata.
func TestRESTAnswersRendersWorkspaceEventNames(t *testing.T) {
	for verb, want := range map[string]string{
		core.AuditVerbMemberInvited:             "InviteToTeamEvent",
		core.AuditVerbInviteAccepted:            "AcceptTeamInviteEvent",
		core.AuditVerbMemberRoleChanged:         "ChangeTeamMemberRoleEvent",
		core.AuditVerbMemberRemoved:             "RemoveUserFromTeamEvent",
		core.AuditVerbMemberLeft:                "RemoveUserFromTeamEvent",
		"members.ResendInvite":                  "members.ResendInvite",
		"members.RevokeInvite":                  "members.RevokeInvite",
		core.AuditVerbCreateCronJob:             "CreateCronJobEvent",
		core.AuditVerbDeleteCronJob:             "DeleteCronJobEvent",
		"apps.Create":                           "CreateServerEvent",
		core.AuditVerbAddDomain:                 "CreateCustomDomainEvent",
		core.AuditVerbDeleteDomain:              "DeleteCustomDomainEvent",
		core.AuditVerbVerifyDomain:              core.AuditVerbVerifyDomain,
		core.AuditVerbSetAutoDeploy:             "UpdateAutoDeployEvent",
		core.AuditVerbSetAutoDeployTrigger:      "UpdateAutoDeployTriggerEvent",
		core.AuditVerbSetBuildFilter:            "UpdateAutoDeployTriggerEvent",
		core.AuditVerbLinkBuildSource:           "UpdateServiceBuildSourceEvent",
		core.AuditVerbLinkBuildSourceEnvGroup:   "LinkBuildSourceEnvGroupEvent",
		core.AuditVerbUnlinkBuildSourceEnvGroup: "UnlinkBuildSourceEnvGroupEvent",
		core.AuditVerbCreateEnvVars:             "CreateEnvVarsEvent",
		core.AuditVerbUpdateEnvVars:             "UpdateEnvVarsEvent",
		core.AuditVerbDeleteEnvVars:             "DeleteEnvVarsEvent",
		core.AuditVerbSetEnvironmentProtection:  "ChangeEnvironmentProtectionEvent",
		core.AuditVerbSetEnvironmentIsolation:   "UpdateEnvironmentIsolatedEvent",
		core.AuditVerbSetEnvironmentIPAllowList: "UpdateIPAllowListEvent",
	} {
		if got := toRenderAuditLog(Event{Verb: verb, Outcome: string(core.AuditAllowed)}).Event; got != want {
			t.Errorf("%s answers %q, want %q", verb, got, want)
		}
	}
	row := toRenderAuditLog(Event{Verb: core.AuditVerbUpdateEnvVars, Outcome: string(core.AuditAllowed),
		Target: core.ServiceTarget("srv-1"), EnvVarKeys: []string{"A", "B"}})
	if row.Metadata["envVarKeys"] != "A,B" || row.Metadata["service"] != "srv-1" {
		t.Errorf("env var row metadata = %v, want envVarKeys A,B on service srv-1", row.Metadata)
	}
}

// w4/m321 t011: GraphQL's resource is the row's target when it names one, so
// the dashboard's Resource column names a created or deleted service rather
// than its workspace; a workspace-wide row keeps the workspace object.
func TestGraphQLResourceNamesTheTarget(t *testing.T) {
	fs := &fakeStore{listRows: []store.AuditRow{
		{ID: "aud-targeted", WorkspaceID: "tea-owner", Resource: "workspace:tea-owner", Target: "project:prj-1"},
		{ID: "aud-workspace", WorkspaceID: "tea-owner", Resource: "workspace:tea-owner"},
	}}
	svc := &Service{Base: &core.Base{Authz: fakeAllowChecker{}}, Store: fs}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: `{ auditLogs(ownerId:"tea-owner") { id resource } }`, Context: ctx})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	rows := res.Data.(map[string]any)["auditLogs"].([]any)
	if got := rows[0].(map[string]any)["resource"]; got != "project:prj-1" {
		t.Errorf("targeted row resource = %v, want project:prj-1", got)
	}
	if got := rows[1].(map[string]any)["resource"]; got != "workspace:tea-owner" {
		t.Errorf("workspace row resource = %v, want workspace:tea-owner", got)
	}
}

// w21/094: an object row names its bucket and its key as two metadata keys and a
// bucket row its bucket, the way Render's own example keys its target by kind; the
// objectstorage verbs pass through as bex's own event names. A denied row keeps
// its relation and outcome beside them, and an old row with no target stays bare.
func TestRenderMetadataNamesTheBucketAndKeyOfAnObjectRow(t *testing.T) {
	put := toRenderAuditLog(Event{Verb: "objectstorage.PutObject", Outcome: string(core.AuditAllowed), Relation: core.RelCanCreate,
		Target: core.ObjectTarget("assets", "img/logo.png")})
	if put.Event != "objectstorage.PutObject" || put.Status != "success" || put.Metadata["bucket"] != "assets" || put.Metadata["key"] != "img/logo.png" {
		t.Errorf("put = %+v, want bucket assets and key img/logo.png", put)
	}
	if _, ok := put.Metadata["object"]; ok || len(put.Metadata) != 2 {
		t.Errorf("put metadata = %v, want exactly bucket and key", put.Metadata)
	}
	// A key with a colon or a slash is cut at the first slash only.
	odd := toRenderAuditLog(Event{Verb: "objectstorage.GetObject", Target: core.ObjectTarget("assets", "a:b/c")})
	if odd.Metadata["bucket"] != "assets" || odd.Metadata["key"] != "a:b/c" {
		t.Errorf("metadata = %v", odd.Metadata)
	}
	bucket := toRenderAuditLog(Event{Verb: "objectstorage.CreateBucket", Outcome: string(core.AuditAllowed), Target: core.BucketTarget("assets")})
	if bucket.Metadata["bucket"] != "assets" || len(bucket.Metadata) != 1 {
		t.Errorf("bucket metadata = %v, want only the bucket", bucket.Metadata)
	}
	denied := toRenderAuditLog(Event{Verb: "objectstorage.GetObject", Outcome: string(core.AuditDenied), Relation: core.RelCanViewSensitive,
		Target: core.ObjectTarget("assets", "secret.txt")})
	if denied.Status != "error" || denied.Metadata["key"] != "secret.txt" || denied.Metadata["relation"] != core.RelCanViewSensitive || denied.Metadata["outcome"] != "denied" {
		t.Errorf("denied = %+v", denied)
	}
	if bare := toRenderAuditLog(Event{Verb: "objectstorage.PutObject", Outcome: string(core.AuditAllowed)}); len(bare.Metadata) != 0 {
		t.Errorf("a row recorded before the target = %v, want none", bare.Metadata)
	}
}
