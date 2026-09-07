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

package opsrole

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const (
	testWorkspace = "tea-ops"
	testToken     = "ops-secret"
)

// fakeChecker answers from a tuples set keyed "subject/relation/object" and
// counts calls, so the tests can assert the bearer gate rejects BEFORE any
// authorization read happens.
type fakeChecker struct {
	tuples map[string]bool
	err    error
	calls  int
}

func (c *fakeChecker) Check(_ context.Context, subject, relation, object string) (bool, error) {
	c.calls++
	if c.err != nil {
		return false, c.err
	}
	return c.tuples[subject+"/"+relation+"/"+object], nil
}

// freshChecker layers CheckFresh over fakeChecker: the cached Check poisons the
// answer so a handler that ever falls back to it fails the test.
type freshChecker struct {
	fakeChecker
	freshCalls int
}

func (c *freshChecker) CheckFresh(ctx context.Context, subject, relation, object string) (bool, error) {
	c.freshCalls++
	return c.fakeChecker.Check(ctx, subject, relation, object)
}

func (c *freshChecker) Check(context.Context, string, string, string) (bool, error) {
	return false, errors.New("cached Check used; the verb must prefer CheckFresh")
}

func tuple(subject, role string) string {
	return "user:" + subject + "/" + role + "/" + core.WorkspaceObject(testWorkspace)
}

func identityFixture(email, name string) IdentityLookup {
	return func(_ context.Context, _ string) (string, string, bool) {
		return email, name, true
	}
}

func handlerWith(authz core.Checker, id IdentityLookup) *Handler {
	return &Handler{Workspace: testWorkspace, Token: testToken, Authz: authz, Identity: id}
}

// get performs GET /internal/ops-role through a mux the handler was Registered
// on — the same routing production uses on the internal listener.
func get(t *testing.T, h *Handler, bearer, subject string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	Register(mux, h)
	target := Path
	if subject != "" {
		target += "?subject=" + subject
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

// TestRegisterAbsentWhenUnconfigured pins the fixed contract's off state: a nil
// or partially configured handler leaves the mux untouched, so the path answers
// the router's normal 404 — no distinguishable "disabled" behavior.
func TestRegisterAbsentWhenUnconfigured(t *testing.T) {
	cases := map[string]*Handler{
		"nil handler":   nil,
		"no token":      {Workspace: testWorkspace},
		"no workspace":  {Token: testToken},
		"neither field": {},
	}
	for name, h := range cases {
		mux := http.NewServeMux()
		Register(mux, h)
		req := httptest.NewRequest(http.MethodGet, Path+"?subject=x", nil)
		req.Header.Set("Authorization", "Bearer "+testToken)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("%s: GET %s = %d, want the router's normal 404", name, Path, rr.Code)
		}
	}
}

// TestBearerRejectedBeforeAnyBackendWork: wrong or missing token is 401, and
// the checker/identity backends are never consulted — the bearer is the gate,
// not a hint.
func TestBearerRejectedBeforeAnyBackendWork(t *testing.T) {
	for _, bearer := range []string{"", "Bearer wrong", "Bearer " + testToken + "x", "Basic " + testToken, testToken} {
		chk := &fakeChecker{tuples: map[string]bool{tuple("sub-1", "admin"): true}}
		identityCalled := false
		h := handlerWith(chk, func(context.Context, string) (string, string, bool) {
			identityCalled = true
			return "", "", false
		})
		rr := get(t, h, bearer, "sub-1")
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("bearer %q: status = %d, want 401", bearer, rr.Code)
		}
		if chk.calls != 0 || identityCalled {
			t.Errorf("bearer %q reached the backends (checks=%d identity=%t); 401 must come first", bearer, chk.calls, identityCalled)
		}
	}
}

func TestMissingSubjectIs400(t *testing.T) {
	chk := &fakeChecker{}
	h := handlerWith(chk, identityFixture("op@example.com", "Op"))
	if rr := get(t, h, "Bearer "+testToken, ""); rr.Code != http.StatusBadRequest {
		t.Fatalf("missing subject = %d, want 400", rr.Code)
	}
	if chk.calls != 0 {
		t.Fatalf("missing subject still hit OpenFGA (%d checks)", chk.calls)
	}
}

// TestEveryRoleAnswersRaw: each of ADR024's five roles is reported verbatim —
// including contributor and billing, whose deny policy deliberately lives in
// the consent acceptor, not here.
func TestEveryRoleAnswersRaw(t *testing.T) {
	for _, role := range []string{"admin", "developer", "contributor", "billing", "viewer"} {
		chk := &fakeChecker{tuples: map[string]bool{tuple("sub-1", role): true}}
		h := handlerWith(chk, identityFixture("op@example.com", "Op Erator"))
		rr := get(t, h, "Bearer "+testToken, "sub-1")
		if rr.Code != http.StatusOK {
			t.Fatalf("role %s: status = %d, want 200 (body %s)", role, rr.Code, rr.Body)
		}
		var got struct {
			Member bool   `json:"member"`
			Role   string `json:"role"`
			Email  string `json:"email"`
			Name   string `json:"name"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatalf("role %s: decode: %v", role, err)
		}
		if !got.Member || got.Role != role || got.Email != "op@example.com" || got.Name != "Op Erator" {
			t.Errorf("role %s: answer = %+v", role, got)
		}
	}
}

// TestHighestRoleWins: a subject with several direct role tuples reports the
// highest rung of the ladder.
func TestHighestRoleWins(t *testing.T) {
	chk := &fakeChecker{tuples: map[string]bool{
		tuple("sub-1", "viewer"):    true,
		tuple("sub-1", "developer"): true,
	}}
	h := handlerWith(chk, identityFixture("op@example.com", ""))
	rr := get(t, h, "Bearer "+testToken, "sub-1")
	var got struct {
		Role string `json:"role"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Role != "developer" {
		t.Fatalf("role = %q, want developer (highest held)", got.Role)
	}
	// An identity that never set the display-name trait still answers with the
	// key present and empty (the fixed member shape).
	if !strings.Contains(rr.Body.String(), `"name":""`) {
		t.Fatalf("member body must carry an explicit empty name, got %s", rr.Body)
	}
}

// TestNonMemberAnswerIsExactlyMemberFalse: no role key, no email leak — the
// pinned {"member":false} shape.
func TestNonMemberAnswerIsExactlyMemberFalse(t *testing.T) {
	identityCalled := false
	h := handlerWith(&fakeChecker{}, func(context.Context, string) (string, string, bool) {
		identityCalled = true
		return "x@example.com", "X", true
	})
	rr := get(t, h, "Bearer "+testToken, "sub-outsider")
	if rr.Code != http.StatusOK {
		t.Fatalf("non-member = %d, want 200", rr.Code)
	}
	if body := strings.TrimSpace(rr.Body.String()); body != `{"member":false}` {
		t.Fatalf("non-member body = %s, want exactly {\"member\":false}", body)
	}
	if identityCalled {
		t.Fatal("non-member answer consulted Kratos; it must not")
	}
}

// TestFailClosed503: an unreachable OpenFGA, an unreachable Kratos (member
// path), a nil identity reader, and a nil checker all answer 503 — never a
// false non-member.
func TestFailClosed503(t *testing.T) {
	cases := map[string]*Handler{
		"openfga error": handlerWith(&fakeChecker{err: errors.New("dial tcp: refused")}, identityFixture("a@b", "")),
		"kratos miss": handlerWith(&fakeChecker{tuples: map[string]bool{tuple("sub-1", "admin"): true}},
			func(context.Context, string) (string, string, bool) { return "", "", false }),
		"nil identity reader": handlerWith(&fakeChecker{tuples: map[string]bool{tuple("sub-1", "admin"): true}}, nil),
		"nil checker":         handlerWith(nil, identityFixture("a@b", "")),
	}
	for name, h := range cases {
		if rr := get(t, h, "Bearer "+testToken, "sub-1"); rr.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: status = %d, want 503 (body %s)", name, rr.Code, rr.Body)
		}
	}
}

// TestPrefersFreshChecker: a checker with CheckFresh must be read fresh — this
// verb admits operators into an admin UI, so a just-revoked membership must
// not ride the positive cache.
func TestPrefersFreshChecker(t *testing.T) {
	chk := &freshChecker{fakeChecker: fakeChecker{tuples: map[string]bool{tuple("sub-1", "viewer"): true}}}
	h := handlerWith(chk, identityFixture("op@example.com", "Op"))
	rr := get(t, h, "Bearer "+testToken, "sub-1")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rr.Code, rr.Body)
	}
	if chk.freshCalls == 0 {
		t.Fatal("CheckFresh never called")
	}
}

// TestNonGETMethodNotAllowed: the mount is method-qualified, so a POST answers
// 405 before the handler runs.
func TestNonGETMethodNotAllowed(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, handlerWith(&fakeChecker{}, identityFixture("a@b", "")))
	req := httptest.NewRequest(http.MethodPost, Path+"?subject=x", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d, want 405", rr.Code)
	}
}
