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

package cliauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// fakeAccess records the reports the dashboard's two calls make (w4/m454).
type fakeAccess struct {
	calls     []string
	recorded  bool
	grantErr  error
	revokeErr error
}

func (f *fakeAccess) RecordGrant(_ context.Context, subject, clientID string) (bool, error) {
	f.calls = append(f.calls, "grant "+subject+" "+clientID)
	return f.recorded, f.grantErr
}

func (f *fakeAccess) RecordRevoke(_ context.Context, subject, clientID string) error {
	f.calls = append(f.calls, "revoke "+subject+" "+clientID)
	return f.revokeErr
}

// markerLog is the revocation marker store, failing on demand.
type markerLog struct {
	bumps []string
	err   error
}

func (m *markerLog) BumpOAuthRevocation(_ context.Context, subject, clientID string) error {
	m.bumps = append(m.bumps, subject+" "+clientID)
	return m.err
}

var memberSession = core.Identity{Subject: "human-1", Method: "session", Human: true}

// postAccess sends one dashboard report through the REST router.
func postAccess(t *testing.T, svc *Service, path, body string, id *core.Identity) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if id != nil {
		req = req.WithContext(core.WithIdentity(req.Context(), *id))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestGrantReportRecordsWhatHydraVouchesFor(t *testing.T) {
	access := &fakeAccess{recorded: true}
	svc := New("", nil, nil)
	svc.Access = access
	rec := postAccess(t, svc, "/v1/oauth/grants", `{"clientId":" dcr-claude "}`, &memberSession)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("grant report => %d %s", rec.Code, rec.Body.String())
	}
	if len(access.calls) != 1 || access.calls[0] != "grant human-1 dcr-claude" {
		t.Fatalf("calls = %v, want one grant for the session's subject", access.calls)
	}

	// Hydra holds no fresh grant for the pair (a forged or late report).
	access.recorded = false
	if rec := postAccess(t, svc, "/v1/oauth/grants", `{"clientId":"dcr-claude"}`, &memberSession); rec.Code != http.StatusNotFound {
		t.Fatalf("unvouched report => %d %s, want 404", rec.Code, rec.Body.String())
	}

	access.grantErr = errors.New("hydra down")
	if rec := postAccess(t, svc, "/v1/oauth/grants", `{"clientId":"dcr-claude"}`, &memberSession); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("Hydra down => %d %s, want 503", rec.Code, rec.Body.String())
	}
}

func TestGrantReportAdmitsOnlyAMembersSession(t *testing.T) {
	cases := map[string]*core.Identity{
		"no identity":       nil,
		"API key":           {Subject: "key-1", Method: "oauth2", ClientID: "key-1"},
		"a human's OAuth":   {Subject: "human-1", Method: "oauth2", ClientID: "dcr-claude", Human: true},
		"a machine session": {Subject: "human-1", Method: "session"},
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			access := &fakeAccess{recorded: true}
			svc := New("", nil, nil)
			svc.Access = access
			if rec := postAccess(t, svc, "/v1/oauth/grants", `{"clientId":"dcr-claude"}`, id); rec.Code != http.StatusForbidden {
				t.Fatalf("=> %d %s, want 403", rec.Code, rec.Body.String())
			}
			if len(access.calls) != 0 {
				t.Fatalf("calls = %v, want none", access.calls)
			}
		})
	}
}

func TestGrantReportRefusesABadBodyAndAnUnwiredRecorder(t *testing.T) {
	access := &fakeAccess{recorded: true}
	svc := New("", nil, nil)
	svc.Access = access
	for _, body := range []string{`{}`, `{"clientId":"  "}`, `{"clientId":"` + strings.Repeat("c", 256) + `"}`, `not json`} {
		if rec := postAccess(t, svc, "/v1/oauth/grants", body, &memberSession); rec.Code != http.StatusBadRequest {
			t.Errorf("body %.40q => %d, want 400", body, rec.Code)
		}
	}
	if len(access.calls) != 0 {
		t.Fatalf("calls = %v, want none", access.calls)
	}
	if rec := postAccess(t, New("", nil, nil), "/v1/oauth/grants", `{"clientId":"dcr-claude"}`, &memberSession); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no recorder => %d, want 503", rec.Code)
	}
}

func TestRevocationIsAuditedAfterItsMarkerAndNeverFailsOnTheAudit(t *testing.T) {
	access := &fakeAccess{revokeErr: errors.New("hydra down")}
	markers := &markerLog{}
	svc := New("", nil, nil)
	svc.Access, svc.Revocations = access, markers
	rec := postAccess(t, svc, "/v1/oauth/revocations", `{"clientId":"dcr-claude"}`, &memberSession)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revocation => %d %s, want 204 despite the audit failure", rec.Code, rec.Body.String())
	}
	if len(markers.bumps) != 1 || len(access.calls) != 1 || access.calls[0] != "revoke human-1 dcr-claude" {
		t.Fatalf("bumps %v, audits %v", markers.bumps, access.calls)
	}

	// A marker that did not move is a failed revocation: nothing is audited.
	markers.err, access.calls = errors.New("db down"), nil
	if rec := postAccess(t, svc, "/v1/oauth/revocations", `{"clientId":"dcr-claude"}`, &memberSession); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("marker failure => %d, want 503", rec.Code)
	}
	if len(access.calls) != 0 {
		t.Fatalf("audits = %v after a failed marker, want none", access.calls)
	}

	// Without a recorder the revocation still lands.
	markers.err = nil
	svc.Access = nil
	if rec := postAccess(t, svc, "/v1/oauth/revocations", `{"clientId":"dcr-claude"}`, &memberSession); rec.Code != http.StatusNoContent {
		t.Fatalf("no recorder => %d, want 204", rec.Code)
	}
}
