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

package signins

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

var now = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

// memberships lists each subject's workspaces.
type memberships map[string][]string

func (m memberships) SubjectWorkspaceIDs(_ context.Context, subject string) ([]string, error) {
	return m[subject], nil
}

// sink is an audit store that keeps a row once per id, as the real one does,
// and can be made to fail or to hold its writes until released.
type sink struct {
	mu     sync.Mutex
	rows   []core.AuditEvent
	ids    map[string]bool
	calls  int
	fail   error
	hold   chan struct{}
	inside chan struct{}
}

func (s *sink) Record(ctx context.Context, ev core.AuditEvent) error {
	return s.RecordAll(ctx, []core.AuditEvent{ev})
}

func (s *sink) RecordAll(_ context.Context, evs []core.AuditEvent) error {
	if s.inside != nil {
		s.inside <- struct{}{}
	}
	if s.hold != nil {
		<-s.hold
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.fail != nil {
		return s.fail
	}
	if s.ids == nil {
		s.ids = map[string]bool{}
	}
	for _, ev := range evs {
		if ev.ID != "" && s.ids[ev.ID] {
			continue
		}
		s.ids[ev.ID] = true
		s.rows = append(s.rows, ev)
	}
	return nil
}

func (s *sink) snapshot() ([]core.AuditEvent, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]core.AuditEvent(nil), s.rows...), s.calls
}

func newRecorder(audit *sink) *Recorder {
	return &Recorder{
		Workspaces: memberships{"kratos-1": {"tea-a", "tea-b"}},
		Audit:      audit,
		Now:        func() time.Time { return now },
	}
}

var fresh = Session{
	ID: "sess-1", AuthenticatedAt: now.Add(-30 * time.Second),
	SourceIP: "198.51.100.7", UserAgent: "Mozilla/5.0\x00",
}

// A new session is one LoginEvent in each of the member's workspaces, by the
// member, dated when it authenticated, from where it signed in; later
// requests on it record nothing more.
func TestANewSessionIsOneSignInPerWorkspace(t *testing.T) {
	audit := &sink{}
	r := newRecorder(audit)
	for range 3 {
		r.ObserveSession(context.Background(), "kratos-1", fresh)
		r.pending.Wait()
	}
	rows, calls := audit.snapshot()
	if calls != 1 {
		t.Errorf("writes = %d, want 1: a seen session records nothing more", calls)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want one per workspace", rows)
	}
	for i, workspace := range []string{"tea-a", "tea-b"} {
		got := rows[i]
		if got.Verb != core.AuditVerbLogin || got.Caller != "kratos-1" || got.CallerMethod != core.MethodSession ||
			got.Resource != core.WorkspaceObject(workspace) || got.Outcome != core.AuditAllowed || got.Target != "" {
			t.Errorf("row %d = %+v, want auth.Login by kratos-1 in %s", i, got, workspace)
		}
		if !got.At.Equal(fresh.AuthenticatedAt) || got.SourceIP != "198.51.100.7" || got.UserAgent != "Mozilla/5.0" {
			t.Errorf("row %d at %v from %q / %q, want the session's time and origin, the agent cleaned", i, got.At, got.SourceIP, got.UserAgent)
		}
	}
	if rows[0].ID == "" || rows[0].ID == rows[1].ID {
		t.Errorf("row ids %q / %q, want one derived id per workspace", rows[0].ID, rows[1].ID)
	}
}

// Two replicas seeing one new session derive the same row ids, so the store
// keeps one row per workspace however many write it.
func TestTwoReplicasDeriveTheSameRows(t *testing.T) {
	audit := &sink{}
	a, b := newRecorder(audit), newRecorder(audit)
	a.ObserveSession(context.Background(), "kratos-1", fresh)
	b.ObserveSession(context.Background(), "kratos-1", fresh)
	a.pending.Wait()
	b.pending.Wait()
	if rows, calls := audit.snapshot(); len(rows) != 2 || calls != 2 {
		t.Errorf("rows = %d from %d writes, want 2 rows from 2 writes", len(rows), calls)
	}
	other := fresh
	other.ID = "sess-2"
	a.ObserveSession(context.Background(), "kratos-1", other)
	a.pending.Wait()
	if rows, _ := audit.snapshot(); len(rows) != 4 {
		t.Errorf("rows = %d, want a second session to be a second sign-in", len(rows))
	}
}

// One session's concurrent first requests on one replica write once.
func TestConcurrentFirstRequestsWriteOnce(t *testing.T) {
	audit := &sink{hold: make(chan struct{}), inside: make(chan struct{}, 8)}
	r := newRecorder(audit)
	r.ObserveSession(context.Background(), "kratos-1", fresh)
	<-audit.inside
	for range 5 {
		r.ObserveSession(context.Background(), "kratos-1", fresh)
	}
	close(audit.hold)
	r.pending.Wait()
	if _, calls := audit.snapshot(); calls != 1 {
		t.Errorf("writes = %d, want 1", calls)
	}
}

// A write that fails is tried again on the session's next request, and the
// request itself never waits for it.
func TestAFailedSignInIsRetriedOnTheNextRequest(t *testing.T) {
	audit := &sink{fail: errors.New("database down")}
	r := newRecorder(audit)
	r.ObserveSession(context.Background(), "kratos-1", fresh)
	r.pending.Wait()
	audit.mu.Lock()
	audit.fail = nil
	audit.mu.Unlock()
	r.ObserveSession(context.Background(), "kratos-1", fresh)
	r.pending.Wait()
	if rows, calls := audit.snapshot(); len(rows) != 2 || calls != 2 {
		t.Errorf("rows = %d from %d writes, want 2 rows on the retry", len(rows), calls)
	}

	stalled := &sink{hold: make(chan struct{})}
	r = newRecorder(stalled)
	returned := make(chan struct{})
	go func() {
		r.ObserveSession(context.Background(), "kratos-1", fresh)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("ObserveSession waited on the audit write")
	}
	close(stalled.hold)
	r.pending.Wait()
}

// Only a session that authenticated within SignInWindow is a sign-in: a
// session that was already live when this replica first saw it (a rollout)
// records none, nor does one without an id or an authentication time.
func TestOnlyAFreshSessionIsASignIn(t *testing.T) {
	for name, s := range map[string]Session{
		"signed in before the window": {ID: "sess-old", AuthenticatedAt: now.Add(-SignInWindow - time.Second)},
		"from the future":             {ID: "sess-future", AuthenticatedAt: now.Add(2 * clockSkew)},
		"no id":                       {AuthenticatedAt: now},
		"no time":                     {ID: "sess-untimed"},
	} {
		audit := &sink{}
		r := newRecorder(audit)
		r.ObserveSession(context.Background(), "kratos-1", s)
		r.pending.Wait()
		if _, calls := audit.snapshot(); calls != 0 {
			t.Errorf("%s: %d writes, want none", name, calls)
		}
	}
	audit := &sink{}
	r := newRecorder(audit)
	r.ObserveSession(context.Background(), "kratos-1", Session{ID: "sess-edge", AuthenticatedAt: now.Add(-SignInWindow)})
	r.pending.Wait()
	if rows, _ := audit.snapshot(); len(rows) != 2 {
		t.Errorf("a session authenticated exactly SignInWindow ago: rows = %d, want 2", len(rows))
	}
}

// A sign-out is one LogoutEvent in each workspace, now, by the credential
// method that reported it, from the request's own origin.
func TestASignOutIsOneRowPerWorkspace(t *testing.T) {
	audit := &sink{}
	r := newRecorder(audit)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "kratos-1", Method: core.MethodOAuth2, Human: true})
	ctx = core.WithRequestOrigin(ctx, core.RequestOrigin{SourceIP: "203.0.113.4", UserAgent: "render-cli/2.27.0"})
	for range 2 {
		if err := r.RecordSignOut(ctx, "kratos-1", core.MethodOAuth2); err != nil {
			t.Fatal(err)
		}
	}
	rows, _ := audit.snapshot()
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want one per workspace per sign-out", len(rows))
	}
	for _, got := range rows {
		if got.Verb != core.AuditVerbLogout || got.CallerMethod != core.MethodOAuth2 || !got.At.Equal(now) ||
			got.SourceIP != "203.0.113.4" || got.UserAgent != "render-cli/2.27.0" {
			t.Errorf("row = %+v, want auth.Logout now from the request", got)
		}
	}
	if err := r.RecordSignOut(ctx, "", core.MethodSession); err == nil {
		t.Error("a sign-out without a subject recorded")
	}

	// A Kratos session's sign-out reported twice (a retried sign-out) is one
	// row per workspace.
	dashboard := &sink{}
	r = newRecorder(dashboard)
	ctx = WithSessionID(core.WithIdentity(context.Background(), core.Identity{Subject: "kratos-1", Method: core.MethodSession, Human: true}), "sess-1")
	for range 2 {
		if err := r.RecordSignOut(ctx, "kratos-1", core.MethodSession); err != nil {
			t.Fatal(err)
		}
	}
	if rows, calls := dashboard.snapshot(); len(rows) != 2 || calls != 2 {
		t.Errorf("rows = %d from %d reports, want 2 from 2", len(rows), calls)
	}
}

func signOut(t *testing.T, r *Recorder, caller core.Identity) *graphql.Result {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{"ok": &graphql.Field{Type: graphql.Boolean}}}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: r.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return graphql.Do(graphql.Params{
		Schema: schema, RequestString: `mutation { recordSignOut }`,
		Context: core.WithIdentity(context.Background(), caller),
	})
}

// recordSignOut records the dashboard member's own sign-out; a token or an
// API key cannot report one, and a write that fails is an error, not a
// silent true.
func TestRecordSignOutMutation(t *testing.T) {
	audit := &sink{}
	r := newRecorder(audit)
	res := signOut(t, r, core.Identity{Subject: "kratos-1", Method: core.MethodSession, Human: true})
	if len(res.Errors) > 0 || res.Data.(map[string]any)["recordSignOut"] != true {
		t.Fatalf("session sign-out = %+v", res)
	}
	if rows, _ := audit.snapshot(); len(rows) != 2 || rows[0].CallerMethod != core.MethodSession {
		t.Errorf("rows = %+v, want two session LogoutEvents", rows)
	}
	for _, caller := range []core.Identity{
		{Subject: "kratos-1", Method: core.MethodOAuth2, Human: true},
		{Subject: "key-1", Method: core.MethodOAuth2},
	} {
		if res := signOut(t, r, caller); len(res.Errors) == 0 {
			t.Errorf("%+v reported a dashboard sign-out", caller)
		}
	}
	if rows, _ := audit.snapshot(); len(rows) != 2 {
		t.Errorf("rows = %d after refused reports, want still 2", len(rows))
	}
	audit.fail = errors.New("database down")
	if res := signOut(t, r, core.Identity{Subject: "kratos-1", Method: core.MethodSession, Human: true}); len(res.Errors) == 0 {
		t.Error("a sign-out whose rows did not land answered true")
	}
}
