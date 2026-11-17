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

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/signins"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// sessionKratos is Kratos' whoami as production answers it (keys checked
// against auth.bex.co, 2026-11-17): the session's id, when it authenticated
// and the device it authenticated from, around the identity.
func sessionKratos(t *testing.T, authenticatedAt string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions/whoami" || !strings.Contains(r.Header.Get("Cookie"), "ory_kratos_session=live") {
			http.Error(w, `{"error":{"code":401}}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":"sess-1","active":true,"authenticated_at":%q,
			"devices":[{"id":"dev-1","ip_address":"203.0.113.50","user_agent":"Kratos-seen agent","location":"Berlin, DE"}],
			"identity":{"id":"identity-1","traits":{"email":"ada@example.com"}}}`, authenticatedAt)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// observedSessions is the gate's SignInObserver, keeping what it was told.
type observedSessions struct {
	mu       sync.Mutex
	subjects []string
	sessions []signins.Session
}

func (o *observedSessions) ObserveSession(_ context.Context, subject string, s signins.Session) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.subjects = append(o.subjects, subject)
	o.sessions = append(o.sessions, s)
}

// The auth gate tells the sign-in recorder of every authenticated Kratos
// session request: the session's id and authentication time from whoami, and
// where the request came from through the trusted proxies, the device Kratos
// recorded standing in only when the request names no origin. A bearer
// request is never a sign-in, and a whoami whose time cannot be read still
// authenticates (w9/m452).
func TestAuthGateReportsEachSessionToTheSignInRecorder(t *testing.T) {
	authenticatedAt := "2026-11-17T11:59:30.123456Z"
	kratos := sessionKratos(t, authenticatedAt)
	s := &Server{HydraAdminURL: fakeHydraURL(t), KratosURL: kratos.URL}
	gate, err := s.newAuthGate()
	if err != nil {
		t.Fatal(err)
	}
	observed := &observedSessions{}
	gate.signIns = observed
	ok := gate.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	call := func(ctx context.Context, header, value string) int {
		r := httptest.NewRequest(http.MethodGet, "/probe", nil).WithContext(ctx)
		r.Header.Set(header, value)
		w := httptest.NewRecorder()
		ok.ServeHTTP(w, r)
		return w.Code
	}
	withOrigin := core.WithRequestOrigin(context.Background(), core.RequestOrigin{SourceIP: "198.51.100.23", UserAgent: "Mozilla/5.0"})
	for _, ctx := range []context.Context{withOrigin, context.Background()} {
		if code := call(ctx, "Cookie", "ory_kratos_session=live"); code != http.StatusNoContent {
			t.Fatalf("session request = %d", code)
		}
	}
	if code := call(context.Background(), "Authorization", "Bearer "+testToken); code != http.StatusNoContent {
		t.Fatalf("bearer request = %d", code)
	}
	at, _ := time.Parse(time.RFC3339Nano, authenticatedAt)
	want := []signins.Session{
		{ID: "sess-1", AuthenticatedAt: at, SourceIP: "198.51.100.23", UserAgent: "Mozilla/5.0"},
		{ID: "sess-1", AuthenticatedAt: at, SourceIP: "203.0.113.50", UserAgent: "Kratos-seen agent"},
	}
	if !reflect.DeepEqual(observed.sessions, want) || !reflect.DeepEqual(observed.subjects, []string{"identity-1", "identity-1"}) {
		t.Errorf("observed %v %+v, want %+v for identity-1 only", observed.subjects, observed.sessions, want)
	}

	untimed := sessionKratos(t, "not a time")
	s = &Server{HydraAdminURL: fakeHydraURL(t), KratosURL: untimed.URL}
	if gate, err = s.newAuthGate(); err != nil {
		t.Fatal(err)
	}
	observed = &observedSessions{}
	gate.signIns = observed
	ok = gate.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	if code := call(context.Background(), "Cookie", "ory_kratos_session=live"); code != http.StatusNoContent {
		t.Fatalf("a whoami with an unreadable time = %d, want it to authenticate", code)
	}
	if len(observed.sessions) != 1 || !observed.sessions[0].AuthenticatedAt.IsZero() {
		t.Errorf("observed %+v, want the session with no time, which records no sign-in", observed.sessions)
	}
}

// batchSink is an audit store for the composed server: it keeps every row
// written, once per id as the real store keeps it.
type batchSink struct {
	mu   sync.Mutex
	rows []core.AuditEvent
}

func (b *batchSink) Record(ctx context.Context, ev core.AuditEvent) error {
	return b.RecordAll(ctx, []core.AuditEvent{ev})
}

func (b *batchSink) RecordAll(_ context.Context, evs []core.AuditEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ev := range evs {
		if ev.ID != "" && slices.ContainsFunc(b.rows, func(row core.AuditEvent) bool { return row.ID == ev.ID }) {
			continue
		}
		b.rows = append(b.rows, ev)
	}
	return nil
}

func (b *batchSink) verbs() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, ev := range b.rows {
		out = append(out, ev.Verb+" "+ev.Resource+" "+ev.SourceIP)
	}
	return out
}

type subjectWorkspaces map[string][]string

func (m subjectWorkspaces) SubjectWorkspaceIDs(_ context.Context, subject string) ([]string, error) {
	return m[subject], nil
}

// Through the composed server, the dashboard's first request on a new session
// records one LoginEvent per workspace and later ones nothing; its
// recordSignOut records one LogoutEvent per workspace, both from the browser's
// address behind the edge; and a bearer cannot report a dashboard sign-out.
func TestDashboardSignInAndSignOutAreAudited(t *testing.T) {
	proxies, err := core.ParseTrustedProxies("10.244.0.0/16")
	if err != nil {
		t.Fatal(err)
	}
	sink := &batchSink{}
	recorder := &signins.Recorder{Workspaces: subjectWorkspaces{"identity-1": {"tea-a", "tea-b"}}, Audit: sink}
	kratos := sessionKratos(t, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano))
	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h := RequestOriginMiddleware(proxies, serverWithKratos(t, base, Deps{SignIns: recorder}, fakeHydraURL(t), kratos.URL))
	call := func(auth map[string]string, query string) map[string]any {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"query": query})
		r := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(string(body)))
		r.RemoteAddr = "10.244.3.17:52114"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("True-Client-IP", "198.51.100.23")
		for k, v := range auth {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	session := map[string]string{"Cookie": "ory_kratos_session=live"}
	for range 3 {
		call(session, `{ __typename }`)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(sink.verbs()) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	signedIn := []string{"auth.Login workspace:tea-a 198.51.100.23", "auth.Login workspace:tea-b 198.51.100.23"}
	if got := sink.verbs(); !reflect.DeepEqual(got, signedIn) {
		t.Fatalf("rows after three requests = %v, want %v", got, signedIn)
	}
	if out := call(map[string]string{"Authorization": "Bearer " + testToken}, `mutation { recordSignOut }`); out["errors"] == nil {
		t.Errorf("a bearer's recordSignOut = %v, want refused", out)
	}
	// The sign-out page retries after a provider failure; the session's
	// sign-out is still recorded once.
	for range 2 {
		if out := call(session, `mutation { recordSignOut }`); out["errors"] != nil || out["data"].(map[string]any)["recordSignOut"] != true {
			t.Fatalf("recordSignOut = %v", out)
		}
	}
	want := append(signedIn, "auth.Logout workspace:tea-a 198.51.100.23", "auth.Logout workspace:tea-b 198.51.100.23")
	if got := sink.verbs(); !reflect.DeepEqual(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

// signInRows are one member's sign-in and sign-out as internal/signins records
// them.
func signInRows(workspace string, at time.Time) []store.AuditRow {
	return []store.AuditRow{
		{ID: "aud-logout", WorkspaceID: workspace, Caller: "human-1", CallerMethod: core.MethodSession, Verb: core.AuditVerbLogout,
			Resource: core.WorkspaceObject(workspace), Outcome: string(core.AuditAllowed), At: at,
			SourceIP: "198.51.100.23", UserAgent: "Mozilla/5.0"},
		{ID: "aud-login", WorkspaceID: workspace, Caller: "human-1", CallerMethod: core.MethodSession, Verb: core.AuditVerbLogin,
			Resource: core.WorkspaceObject(workspace), Outcome: string(core.AuditAllowed), At: at.Add(-time.Hour),
			SourceIP: "198.51.100.23", UserAgent: "Mozilla/5.0"},
	}
}

// REST and GraphQL read the same two rows as Render's LogoutEvent and
// LoginEvent, by the member as a user actor, with Render's source_ip and
// user_agent metadata and nothing else.
func TestSignInEventsAgreeOnEverySurface(t *testing.T) {
	fakeStore := &fakeAuditStore{rows: signInRows("tea-a", time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))}
	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, _ := serverWith(t, base, Deps{Audit: &audit.Service{Base: base, Store: fakeStore}})
	rec := do(t, h, http.MethodGet, "/v1/owners/tea-a/audit-logs", testToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("REST audit-logs: %d %s", rec.Code, rec.Body.String())
	}
	var rest []restAuditLog
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil {
		t.Fatal(err)
	}
	rows, _ := gql(t, h, `{ auditLogs(ownerId: "tea-a") { id action actorMethod status sourceIp userAgent } }`)["auditLogs"].([]any)
	if len(rest) != 2 || len(rows) != 2 {
		t.Fatalf("REST %d rows, GraphQL %d rows; want the sign-out and the sign-in", len(rest), len(rows))
	}
	metadata := map[string]string{"source_ip": "198.51.100.23", "user_agent": "Mozilla/5.0"}
	for i, event := range []string{"LogoutEvent", "LoginEvent"} {
		got := rest[i].AuditLog
		if got.Event != event || got.Actor.Type != "user" || !reflect.DeepEqual(got.Metadata, metadata) {
			t.Errorf("REST row %d = %+v, want %s by the member with %v", i, got, event, metadata)
		}
		row := rows[i].(map[string]any)
		if row["id"] != got.ID || row["action"] != event || row["actorMethod"] != "session" || row["status"] != "success" ||
			row["sourceIp"] != "198.51.100.23" || row["userAgent"] != "Mozilla/5.0" {
			t.Errorf("GraphQL row %d = %v, diverges from REST %+v", i, row, got)
		}
	}
}
