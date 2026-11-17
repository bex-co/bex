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
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// journal records, in order, every write the revoker makes to Hydra, the marker
// and the audit trail, so a test can pin the order they happen in.
type journal struct {
	mu     sync.Mutex
	events []string
	audit  []core.AuditEvent
}

func (j *journal) add(event string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.events = append(j.events, event)
}

func (j *journal) log() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]string(nil), j.events...)
}

func (j *journal) BumpOAuthRevocation(_ context.Context, subject, client string) error {
	j.add("marker " + subject + " " + client)
	return nil
}

func (j *journal) Record(_ context.Context, ev core.AuditEvent) error {
	j.add("audit " + ev.Target)
	j.mu.Lock()
	defer j.mu.Unlock()
	j.audit = append(j.audit, ev)
	return nil
}

const listBody = `[
 {"grant_scope":["openid","offline_access"],"handled_at":"2026-11-15T10:00:00Z","consent_request":{"client":{"client_id":"cli"}}},
 {"grant_scope":["openid","bex.read"],"handled_at":"2026-11-16T10:00:00Z","consent_request":{"client":{"client_id":"cli"}}},
 {"grant_scope":["openid"],"handled_at":"2026-11-14T10:00:00Z","consent_request":{"client":{"client_id":"mobile"}}},
 {"grant_scope":["openid"],"consent_request":{"client":{}}}
]`

// hydraStub answers the two consent-session calls. deleteStatus is what the
// DELETE returns; link is a Link header on the list.
func hydraStub(t *testing.T, j *journal, deleteStatus int, link string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/oauth2/auth/sessions/consent" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		switch r.Method {
		case http.MethodGet:
			j.add("hydra list " + q.Get("subject"))
			if link != "" {
				w.Header().Set("Link", link)
			}
			_, _ = w.Write([]byte(listBody))
		case http.MethodDelete:
			j.add("hydra delete " + q.Get("subject") + " " + q.Get("client"))
			w.WriteHeader(deleteStatus)
		default:
			http.Error(w, "unexpected", http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newRevoker(srv *httptest.Server, j *journal) *Revoker {
	return &Revoker{
		HydraAdminURL: srv.URL + "/", Markers: j, Audit: j, AuditWorkspace: "tea-ops",
		Now: func() time.Time { return time.Unix(1_700_000_000, 0).UTC() },
	}
}

func TestDryRunListsTheGrantsAndWritesNothing(t *testing.T) {
	j := &journal{}
	res, err := newRevoker(hydraStub(t, j, http.StatusNoContent, ""), j).Run(context.Background(),
		Request{Subject: "human-1", ClientID: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied || len(res.Revoked) != 0 || !reflect.DeepEqual(res.Targets, []string{"cli"}) {
		t.Fatalf("dry run = %+v", res)
	}
	if got := j.log(); !reflect.DeepEqual(got, []string{"hydra list human-1"}) {
		t.Fatalf("a dry run did more than read: %v", got)
	}
	// One entry per client with the union of its scopes and its latest grant.
	if len(res.Grants) != 2 || res.Grants[0].ClientID != "cli" || res.Grants[1].ClientID != "mobile" {
		t.Fatalf("grants = %+v", res.Grants)
	}
	cli := res.Grants[0]
	if !reflect.DeepEqual(cli.Scopes, []string{"openid", "offline_access", "bex.read"}) ||
		cli.HandledAt == nil || !cli.HandledAt.Equal(time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("cli grant = %+v", cli)
	}
}

// TestRevokeDeletesAtHydraBeforeTheMarkerThenAudits pins the order the doc
// comment on revoke explains: a marker written before Hydra's revocation lets a
// replica cache "active" newer than the marker.
func TestRevokeDeletesAtHydraBeforeTheMarkerThenAudits(t *testing.T) {
	j := &journal{}
	res, err := newRevoker(hydraStub(t, j, http.StatusNoContent, ""), j).Run(context.Background(),
		Request{Subject: "human-1", ClientID: "cli", Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"hydra list human-1",
		"hydra delete human-1 cli",
		"marker human-1 cli",
		"audit oauth-grant:cli",
	}
	if got := j.log(); !reflect.DeepEqual(got, want) {
		t.Fatalf("writes = %v, want %v", got, want)
	}
	if !res.Applied || !reflect.DeepEqual(res.Revoked, []Revocation{{ClientID: "cli", Hydra: "deleted", Marker: true, Audited: true}}) {
		t.Fatalf("result = %+v", res)
	}
	ev := j.audit[0]
	if ev.Verb != core.AuditVerbRevokeOAuthAccess || ev.Caller != "operator" || ev.CallerMethod != "exec" || ev.Resource != "workspace:tea-ops" ||
		ev.Target != "oauth-grant:cli" || ev.TargetName != "human-1" || ev.Outcome != core.AuditAllowed || ev.At.IsZero() {
		t.Fatalf("audit event = %+v", ev)
	}
}

func TestRevokeAuditsUnderTheDefaultWorkspaceWithoutAnOpsWorkspace(t *testing.T) {
	j := &journal{}
	r := newRevoker(hydraStub(t, j, http.StatusNoContent, ""), j)
	r.AuditWorkspace = ""
	if _, err := r.Run(context.Background(), Request{Subject: "human-1", ClientID: "cli", Apply: true}); err != nil {
		t.Fatal(err)
	}
	if got := j.audit[0].Resource; got != "workspace:default" {
		t.Fatalf("resource = %q", got)
	}
}

func TestAllClientsRevokesEveryGrantTheSubjectHolds(t *testing.T) {
	j := &journal{}
	res, err := newRevoker(hydraStub(t, j, http.StatusNoContent, ""), j).Run(context.Background(),
		Request{Subject: "human-1", AllClients: true, Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"hydra list human-1",
		"hydra delete human-1 cli", "marker human-1 cli", "audit oauth-grant:cli",
		"hydra delete human-1 mobile", "marker human-1 mobile", "audit oauth-grant:mobile",
	}
	if got := j.log(); !reflect.DeepEqual(got, want) {
		t.Fatalf("writes = %v, want %v", got, want)
	}
	if len(res.Revoked) != 2 {
		t.Fatalf("revoked = %+v", res.Revoked)
	}
}

// A pair Hydra holds no consent session for (404) still gets its marker: a
// token Hydra answers active for must not outlive the request that follows.
func TestAPairHydraHoldsNoSessionForStillGetsItsMarker(t *testing.T) {
	j := &journal{}
	res, err := newRevoker(hydraStub(t, j, http.StatusNotFound, ""), j).Run(context.Background(),
		Request{Subject: "human-1", ClientID: "other", Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Revoked) != 1 || res.Revoked[0].Hydra != "none" || !res.Revoked[0].Marker {
		t.Fatalf("result = %+v", res)
	}
}

func TestHydraFailureWritesNoMarkerAndNoAuditRow(t *testing.T) {
	j := &journal{}
	res, err := newRevoker(hydraStub(t, j, http.StatusInternalServerError, ""), j).Run(context.Background(),
		Request{Subject: "human-1", ClientID: "cli", Apply: true})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("error = %v, want Hydra's 500", err)
	}
	if got := j.log(); !reflect.DeepEqual(got, []string{"hydra list human-1", "hydra delete human-1 cli"}) {
		t.Fatalf("writes after a Hydra failure = %v", got)
	}
	if len(res.Revoked) != 1 || res.Revoked[0].Marker || res.Revoked[0].Audited {
		t.Fatalf("result = %+v", res)
	}
}

type failingMarkers struct{}

func (failingMarkers) BumpOAuthRevocation(context.Context, string, string) error {
	return errors.New("connection refused")
}

func TestMarkerFailureIsReportedAfterHydraRevoked(t *testing.T) {
	j := &journal{}
	r := newRevoker(hydraStub(t, j, http.StatusNoContent, ""), j)
	r.Markers = failingMarkers{}
	res, err := r.Run(context.Background(), Request{Subject: "human-1", ClientID: "cli", Apply: true})
	if err == nil || !strings.Contains(err.Error(), "revoked at Hydra but the shared marker was not written") {
		t.Fatalf("error = %v", err)
	}
	if len(res.Revoked) != 1 || res.Revoked[0].Hydra != "deleted" || res.Revoked[0].Marker || len(j.audit) != 0 {
		t.Fatalf("result = %+v, audit = %v", res, j.audit)
	}
}

type failingAudit struct{}

func (failingAudit) Record(context.Context, core.AuditEvent) error { return errors.New("no database") }

func TestAuditFailureIsReportedAfterTheGrantIsRevoked(t *testing.T) {
	j := &journal{}
	r := newRevoker(hydraStub(t, j, http.StatusNoContent, ""), j)
	r.Audit = failingAudit{}
	res, err := r.Run(context.Background(), Request{Subject: "human-1", ClientID: "cli", Apply: true})
	if err == nil || !strings.Contains(err.Error(), "audit row was not written") {
		t.Fatalf("error = %v", err)
	}
	if len(res.Revoked) != 1 || !res.Revoked[0].Marker || res.Revoked[0].Audited {
		t.Fatalf("result = %+v", res)
	}
}

func TestRefusedRequestsNeverReachHydra(t *testing.T) {
	long := strings.Repeat("x", maxIDLen+1)
	for name, req := range map[string]Request{
		"no subject":            {ClientID: "cli"},
		"no client":             {Subject: "human-1"},
		"client and all":        {Subject: "human-1", ClientID: "cli", AllClients: true},
		"machine credential":    {Subject: "key-1", ClientID: "key-1"},
		"subject too long":      {Subject: long, ClientID: "cli"},
		"client id too long":    {Subject: "human-1", ClientID: long},
		"subject only, applied": {Subject: "human-1", Apply: true},
	} {
		t.Run(name, func(t *testing.T) {
			j := &journal{}
			if _, err := newRevoker(hydraStub(t, j, http.StatusNoContent, ""), j).Run(context.Background(), req); err == nil {
				t.Fatalf("%+v was accepted", req)
			}
			if got := j.log(); len(got) != 0 {
				t.Fatalf("a refused request called out: %v", got)
			}
		})
	}
}

// --all-clients on a listing Hydra pages would stop at the first page and
// report success, leaving grants live: refuse it.
func TestAPagedListingRefusesAllClients(t *testing.T) {
	j := &journal{}
	_, err := newRevoker(hydraStub(t, j, http.StatusNoContent, `<http://hydra/?page_token=abc>; rel="next"`), j).Run(
		context.Background(), Request{Subject: "human-1", AllClients: true, Apply: true})
	if err == nil || !strings.Contains(err.Error(), "more consent sessions") {
		t.Fatalf("error = %v", err)
	}
	if got := j.log(); len(got) != 1 {
		t.Fatalf("writes = %v, want only the list", got)
	}
}

func TestSubjectsAreEscapedInTheQuery(t *testing.T) {
	var gotRaw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw = r.URL.RawQuery
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()
	r := &Revoker{HydraAdminURL: srv.URL}
	if _, err := r.Grants(context.Background(), "a&client=victim b"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotRaw, "subject=a%26client%3Dvictim+b") {
		t.Fatalf("query = %q: the subject escaped its parameter", gotRaw)
	}
}

var _ Markers = (*journal)(nil)
var _ core.AuditSink = (*journal)(nil)
