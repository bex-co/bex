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
