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

package oauthgrant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// accessNow is the clock of every Access test: the consent fixtures below are
// handled a minute before it, except the stale one.
var accessNow = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

// consentFixture is one consent session as Hydra lists it.
func consentFixture(client, name string, skipConsent bool, handledAt time.Time, scopes ...string) map[string]any {
	return map[string]any{
		"grant_scope": scopes,
		"handled_at":  handledAt.Format(time.RFC3339Nano),
		"consent_request": map[string]any{"client": map[string]any{
			"client_id": client, "client_name": name, "skip_consent": skipConsent,
		}},
	}
}

// accessHydra serves the two reads Access makes: a subject's consent sessions
// and a client's registration. A nil clients map answers every client lookup
// with 500.
type accessHydra struct {
	sessions    []map[string]any
	listStatus  int
	clients     map[string]string
	clientCalls int
}

func (h *accessHydra) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/admin/oauth2/auth/sessions/consent":
			if h.listStatus != 0 {
				w.WriteHeader(h.listStatus)
				return
			}
			_ = jsonEncode(w, h.sessions)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/admin/clients/"):
			h.clientCalls++
			if h.clients == nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			name, ok := h.clients[strings.TrimPrefix(r.URL.Path, "/admin/clients/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_ = jsonEncode(w, map[string]string{"client_name": name})
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func jsonEncode(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}

// memberships is a WorkspaceLister over a fixed subject -> workspaces map.
type memberships struct {
	of  map[string][]string
	err error
}

func (m memberships) SubjectWorkspaceIDs(_ context.Context, subject string) ([]string, error) {
	return m.of[subject], m.err
}

// trail records every audit row, as a batch sink does.
type trail struct{ rows []core.AuditEvent }

func (t *trail) Record(_ context.Context, ev core.AuditEvent) error {
	t.rows = append(t.rows, ev)
	return nil
}

func (t *trail) RecordAll(_ context.Context, evs []core.AuditEvent) error {
	t.rows = append(t.rows, evs...)
	return nil
}

func newAccess(srv *httptest.Server, audit core.AuditSink) *Access {
	return &Access{
		HydraAdminURL: srv.URL,
		Workspaces:    memberships{of: map[string][]string{"human-1": {"tea-a", "tea-b"}}},
		Audit:         audit,
		Now:           func() time.Time { return accessNow },
	}
}

func TestRecordGrantWritesOneRowPerWorkspaceFromHydrasNewestConsent(t *testing.T) {
	hydra := &accessHydra{sessions: []map[string]any{
		consentFixture("dcr-claude", "Claude Code", false, accessNow.Add(-3*time.Hour), "openid"),
		consentFixture("dcr-claude", "Claude Code", false, accessNow.Add(-time.Minute), "openid", "offline_access", "bex.read"),
		consentFixture("dcr-other", "Other", false, accessNow.Add(-time.Minute), "bex.write"),
	}}
	rows := &trail{}
	recorded, err := newAccess(hydra.server(t), rows).RecordGrant(context.Background(), "human-1", "dcr-claude")
	if err != nil || !recorded {
		t.Fatalf("RecordGrant = %v, %v; want recorded", recorded, err)
	}
	if len(rows.rows) != 2 {
		t.Fatalf("rows = %+v, want one per workspace", rows.rows)
	}
	handled := accessNow.Add(-time.Minute)
	for i, workspace := range []string{"tea-a", "tea-b"} {
		ev := rows.rows[i]
		if ev.Verb != core.AuditVerbGrantOAuthAccess || ev.Caller != "human-1" || ev.CallerMethod != "session" ||
			ev.Resource != core.WorkspaceObject(workspace) || ev.Target != "oauth-client:dcr-claude" ||
			ev.TargetName != "Claude Code" || ev.Outcome != core.AuditAllowed || !ev.At.Equal(handled) {
			t.Errorf("row %d = %+v", i, ev)
		}
		// The newest consent's scopes, not the union of every consent's.
		if !reflect.DeepEqual(ev.OAuthGrantScopes, []string{"openid", "offline_access", "bex.read"}) {
			t.Errorf("row %d scopes = %v", i, ev.OAuthGrantScopes)
		}
		if !strings.HasPrefix(ev.ID, "aud-") {
			t.Errorf("row %d id = %q, want a derived aud- id", i, ev.ID)
		}
	}
	if rows.rows[0].ID == rows.rows[1].ID {
		t.Errorf("both workspaces got the id %q", rows.rows[0].ID)
	}

	// A repeated report of the same consent derives the same ids, which the
	// store's ON CONFLICT DO NOTHING turns into no new rows.
	again := &trail{}
	if _, err := newAccess(hydra.server(t), again).RecordGrant(context.Background(), "human-1", "dcr-claude"); err != nil {
		t.Fatal(err)
	}
	if again.rows[0].ID != rows.rows[0].ID || again.rows[1].ID != rows.rows[1].ID {
		t.Errorf("retry ids = %s, %s; want %s, %s", again.rows[0].ID, again.rows[1].ID, rows.rows[0].ID, rows.rows[1].ID)
	}
}

func TestRecordGrantRecordsNothingHydraDoesNotVouchFor(t *testing.T) {
	cases := map[string][]map[string]any{
		// A forged report: the caller never consented to this client.
		"no consent for the client": {consentFixture("dcr-other", "Other", false, accessNow.Add(-time.Minute), "openid")},
		// An old consent re-reported, or a remembered one Hydra skipped.
		"consent older than the window": {consentFixture("dcr-claude", "Claude Code", false, accessNow.Add(-GrantWindow-time.Second), "openid")},
		"consent dated in the future":   {consentFixture("dcr-claude", "Claude Code", false, accessNow.Add(2*grantClockSkew), "openid")},
		// The platform's own sign-in clients: a bex CLI login is not an agent
		// being given access.
		"skip_consent platform client": {consentFixture("dcr-claude", "bex CLI", true, accessNow.Add(-time.Minute), "openid")},
	}
	for name, sessions := range cases {
		t.Run(name, func(t *testing.T) {
			rows := &trail{}
			recorded, err := newAccess((&accessHydra{sessions: sessions}).server(t), rows).
				RecordGrant(context.Background(), "human-1", "dcr-claude")
			if err != nil || recorded || len(rows.rows) != 0 {
				t.Fatalf("RecordGrant = %v, %v, rows %+v; want nothing recorded", recorded, err, rows.rows)
			}
		})
	}
}

func TestRecordGrantFailsClosedWhenHydraOrTheMembershipsCannotAnswer(t *testing.T) {
	rows := &trail{}
	_, err := newAccess((&accessHydra{listStatus: http.StatusBadGateway}).server(t), rows).
		RecordGrant(context.Background(), "human-1", "dcr-claude")
	if err == nil || len(rows.rows) != 0 {
		t.Fatalf("Hydra down: err %v, rows %+v; want an error and nothing recorded", err, rows.rows)
	}

	access := newAccess((&accessHydra{sessions: []map[string]any{
		consentFixture("dcr-claude", "Claude Code", false, accessNow.Add(-time.Minute), "openid"),
	}}).server(t), rows)
	access.Workspaces = memberships{err: errors.New("store down")}
	if _, err := access.RecordGrant(context.Background(), "human-1", "dcr-claude"); err == nil || len(rows.rows) != 0 {
		t.Fatalf("store down: err %v, rows %+v", err, rows.rows)
	}
}

func TestRecordRevokeWritesOneRowPerWorkspaceNamingTheClient(t *testing.T) {
	rows := &trail{}
	if err := newAccess((&accessHydra{clients: map[string]string{"dcr-claude": "Claude Code"}}).server(t), rows).
		RecordRevoke(context.Background(), "human-1", "dcr-claude"); err != nil {
		t.Fatal(err)
	}
	if len(rows.rows) != 2 {
		t.Fatalf("rows = %+v, want one per workspace", rows.rows)
	}
	for i, workspace := range []string{"tea-a", "tea-b"} {
		ev := rows.rows[i]
		if ev.Verb != core.AuditVerbRevokeOAuthAccess || ev.Caller != "human-1" || ev.CallerMethod != "session" ||
			ev.Resource != core.WorkspaceObject(workspace) || ev.Target != "oauth-client:dcr-claude" ||
			ev.TargetName != "Claude Code" || !ev.At.Equal(accessNow) || ev.OAuthGrantScopes != nil || ev.ID != "" {
			t.Errorf("row %d = %+v", i, ev)
		}
	}
}

func TestRecordRevokeOfAClientHydraDoesNotKnowRecordsNothing(t *testing.T) {
	rows := &trail{}
	hydra := &accessHydra{clients: map[string]string{}}
	if err := newAccess(hydra.server(t), rows).RecordRevoke(context.Background(), "human-1", "never-registered"); err != nil {
		t.Fatal(err)
	}
	if hydra.clientCalls != 1 || len(rows.rows) != 0 {
		t.Fatalf("lookups %d, rows %+v; want one lookup and nothing recorded", hydra.clientCalls, rows.rows)
	}
}

func TestRecordRevokeStillRecordsWhenHydraCannotNameTheClient(t *testing.T) {
	rows := &trail{}
	err := newAccess((&accessHydra{}).server(t), rows).RecordRevoke(context.Background(), "human-1", "dcr-claude")
	if err == nil {
		t.Fatal("want the lookup failure reported for the log")
	}
	if len(rows.rows) != 2 || rows.rows[0].TargetName != "" || rows.rows[0].Target != "oauth-client:dcr-claude" {
		t.Fatalf("rows = %+v, want both workspaces recorded without the name", rows.rows)
	}
}
