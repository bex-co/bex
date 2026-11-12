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

package agentsession

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// commentSession is prSession after its draft PR #7 was opened.
func commentSession() store.AgentSession {
	session := prSession()
	session.PRNumber, session.PRURL = 7, "https://github.com/octo/repo/pull/7"
	return session
}

func commentRequest(body string) PullRequestCommentRequest {
	return PullRequestCommentRequest{MintRequest: validMintRequest(), PullRequestCommentContent: PullRequestCommentContent{Body: body}}
}

func wantCommentCode(t *testing.T, err error, code string, status int) {
	t.Helper()
	var typed *PullRequestError
	if !errors.As(err, &typed) || typed.Code != code || typed.Status != status {
		t.Fatalf("error = %#v, want %s (%d)", err, code, status)
	}
	if !strings.Contains(typed.Hint, "the only way this session can comment on its pull request") {
		t.Fatalf("hint %q does not steer the agent away from other routes", typed.Hint)
	}
}

func TestPullRequestCommentPostsOnTheSessionsOwnPRWithFooter(t *testing.T) {
	forge, sessions, audit := &fakeForge{}, &prSessions{session: commentSession()}, &auditRecorder{}
	p := newPullRequester(forge, sessions, audit)

	got, err := p.Comment(context.Background(), commentRequest("Fixed the nil check in `retry.go`.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 7 || got.URL != "https://github.com/octo/repo/pull/7#issuecomment-901" {
		t.Fatalf("response = %+v", got)
	}
	if len(forge.comments) != 1 {
		t.Fatalf("comments = %+v", forge.comments)
	}
	posted := forge.comments[0]
	if posted.installation != 42 || posted.owner != "octo" || posted.repo != "repo" || posted.number != 7 {
		t.Fatalf("GitHub target = %+v, want the session's own repo and recorded PR", posted)
	}
	want := "Fixed the nil check in `retry.go`.\n\n" + PullRequestFooterMarker +
		"\n---\n🤖 Written by a bex agent session (claude) · [view session](https://dashboard.bex.co/agents/ags-one)"
	if posted.body != want {
		t.Fatalf("comment body = %q, want %q", posted.body, want)
	}
	if sessions.recorded != 0 {
		t.Fatal("a comment rewrote the session's PR record")
	}
	if len(audit.events) != 1 || audit.events[0].Verb != AuditVerbPullRequestComment || audit.events[0].Outcome != core.AuditAllowed ||
		audit.events[0].TargetName != "octo/repo" || audit.events[0].Target != "agent-session:ags-one" || audit.events[0].Caller != "sbx-1-0" {
		t.Fatalf("audit = %+v", audit.events)
	}

	// A comment quoting the PR description's footer still carries exactly one,
	// the comment's own.
	quoted := PullRequestBodyWithFooter("## Summary", "claude", "https://dashboard.bex.co/agents/ags-one", "ags-one")
	if _, err := p.Comment(context.Background(), commentRequest("Updated the description:\n\n"+quoted)); err != nil {
		t.Fatal(err)
	}
	second := forge.comments[1].body
	if strings.Count(second, PullRequestFooterMarker) != 1 || strings.Contains(second, "Opened by a bex agent session") || !strings.HasSuffix(second, "Written by a bex agent session (claude) · [view session](https://dashboard.bex.co/agents/ags-one)") {
		t.Fatalf("re-footered comment = %q", second)
	}
}

func TestPullRequestCommentTypedRefusals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*fakeForge, *prSessions, *PullRequestCommentRequest)
		code   string
		status int
	}{
		{"session has no pull request", func(_ *fakeForge, s *prSessions, _ *PullRequestCommentRequest) { s.session.PRNumber = 0 }, PullRequestNoPullRequest, http.StatusConflict},
		{"stale pod generation", func(_ *fakeForge, _ *prSessions, r *PullRequestCommentRequest) { r.PodName = "sbx-0-0" }, PullRequestForbidden, http.StatusForbidden},
		{"terminal session", func(_ *fakeForge, s *prSessions, _ *PullRequestCommentRequest) { s.session.Phase = "completed" }, PullRequestForbidden, http.StatusForbidden},
		{"other session", func(_ *fakeForge, _ *prSessions, r *PullRequestCommentRequest) { r.SessionID = "ags-two" }, PullRequestForbidden, http.StatusForbidden},
		{"other workspace", func(_ *fakeForge, _ *prSessions, r *PullRequestCommentRequest) { r.Workspace = "tea-b" }, PullRequestForbidden, http.StatusForbidden},
		{"branch differs from the record", func(_ *fakeForge, _ *prSessions, r *PullRequestCommentRequest) { r.Branch = "bex-agent/other" }, PullRequestForbidden, http.StatusForbidden},
		{"repo differs from the record", func(_ *fakeForge, _ *prSessions, r *PullRequestCommentRequest) { r.Repository = "octo/other" }, PullRequestForbidden, http.StatusForbidden},
		{"incomplete identity", func(_ *fakeForge, _ *prSessions, r *PullRequestCommentRequest) { r.PodUID = "" }, PullRequestForbidden, http.StatusForbidden},
		{"no installation for the owner", func(_ *fakeForge, s *prSessions, r *PullRequestCommentRequest) {
			s.session.Repo, r.Repository = "elsewhere/repo", "elsewhere/repo"
		}, PullRequestForbidden, http.StatusForbidden},
		{"empty body", func(_ *fakeForge, _ *prSessions, r *PullRequestCommentRequest) { r.Body = " \n\t" }, PullRequestInvalidRequest, http.StatusUnprocessableEntity},
		{"NUL in body", func(_ *fakeForge, _ *prSessions, r *PullRequestCommentRequest) { r.Body = "a\x00b" }, PullRequestInvalidRequest, http.StatusUnprocessableEntity},
		{"invalid UTF-8", func(_ *fakeForge, _ *prSessions, r *PullRequestCommentRequest) { r.Body = "a\xffb" }, PullRequestInvalidRequest, http.StatusUnprocessableEntity},
		{"oversized body", func(_ *fakeForge, _ *prSessions, r *PullRequestCommentRequest) {
			r.Body = strings.Repeat("x", MaxPullRequestCommentBytes+1)
		}, PullRequestTooLarge, http.StatusRequestEntityTooLarge},
		{"github failure", func(f *fakeForge, _ *prSessions, _ *PullRequestCommentRequest) {
			f.err = &github.APIError{Status: 403, Body: "secret upstream text"}
		}, PullRequestUpstreamError, http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			forge, sessions, audit := &fakeForge{}, &prSessions{session: commentSession()}, &auditRecorder{}
			req := commentRequest("Fixed.")
			tc.mutate(forge, sessions, &req)
			_, err := newPullRequester(forge, sessions, audit).Comment(context.Background(), req)
			wantCommentCode(t, err, tc.code, tc.status)
			if strings.Contains(err.Error(), "secret upstream text") {
				t.Fatal("upstream GitHub text leaked into the refusal")
			}
			if len(forge.comments) != 0 {
				t.Fatalf("a refused request posted %+v", forge.comments)
			}
			if len(audit.events) != 1 || audit.events[0].Verb != AuditVerbPullRequestComment || audit.events[0].Outcome != core.AuditDenied {
				t.Fatalf("denied audit = %+v", audit.events)
			}
		})
	}
	// A body at exactly the cap passes.
	forge, sessions := &fakeForge{}, &prSessions{session: commentSession()}
	if _, err := newPullRequester(forge, sessions, nil).Comment(context.Background(), commentRequest(strings.Repeat("x", MaxPullRequestCommentBytes))); err != nil {
		t.Fatalf("body at the cap refused: %v", err)
	}
}

// postSigned posts raw bytes signed over macInput, as an on-path caller of the
// internal listener would.
func postSigned(t *testing.T, url string, secret, raw, macInput []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	signHeaders(req.Header, secret, macInput, time.Now())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp
}

// The signed hop end to end: the gateway client sees bex-api's typed refusal,
// the MAC domain keeps the PR verb's and this verb's signatures apart, and a
// captured request cannot be replayed into a second comment.
func TestPullRequestCommentHandlerAndClientRoundTrip(t *testing.T) {
	secret := []byte("secret")
	forge, sessions := &fakeForge{}, &prSessions{session: commentSession()}
	pullRequests := newPullRequester(forge, sessions, nil)
	mux := http.NewServeMux()
	mux.Handle(InternalPullRequestCommentPath, &PullRequestCommentHandler{Secret: secret, PullRequests: pullRequests, Nonce: &fakeNonceClaimer{}})
	mux.Handle(InternalPullRequestPath, &PullRequestHandler{Secret: secret, PullRequests: pullRequests, Nonce: &fakeNonceClaimer{}})
	api := httptest.NewServer(mux)
	defer api.Close()
	client := &PullRequestCommentClient{URL: api.URL + InternalPullRequestCommentPath, Secret: secret, HTTP: api.Client()}

	resp, err := client.Comment(context.Background(), commentRequest("Fixed."))
	if err != nil || resp.Number != 7 || !strings.HasPrefix(resp.URL, "https://github.com/octo/repo/pull/7#") {
		t.Fatalf("comment = %+v err=%v", resp, err)
	}

	sessions.session.PRNumber = 0
	_, err = client.Comment(context.Background(), commentRequest("Fixed."))
	wantCommentCode(t, err, PullRequestNoPullRequest, http.StatusConflict)
	sessions.session.PRNumber = 7

	calls := forge.calls
	bad := &PullRequestCommentClient{URL: client.URL, Secret: []byte("wrong"), HTTP: api.Client()}
	_, err = bad.Comment(context.Background(), commentRequest("Fixed."))
	wantCommentCode(t, err, PullRequestForbidden, http.StatusForbidden)

	envelope := commentRequest("Fixed.")
	envelope.Kind, envelope.Nonce = PullRequestCommentKind, newNonce()
	raw, _ := json.Marshal(envelope)
	// A bare-body (PR-verb) signature does not verify under this verb's domain.
	if r := postSigned(t, client.URL, secret, raw, raw); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bare-body MAC status = %d, want 401", r.StatusCode)
	}
	// This verb's signed envelope is refused by the PR verb.
	if r := postSigned(t, api.URL+InternalPullRequestPath, secret, raw, pullRequestCommentMACInput(raw)); r.StatusCode == http.StatusOK {
		t.Fatal("the PR verb accepted a comment envelope")
	}
	// A correctly signed envelope posts once; its replay is refused.
	if r := postSigned(t, client.URL, secret, raw, pullRequestCommentMACInput(raw)); r.StatusCode != http.StatusOK {
		t.Fatalf("signed envelope status = %d", r.StatusCode)
	}
	if r := postSigned(t, client.URL, secret, raw, pullRequestCommentMACInput(raw)); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("replay status = %d, want 401", r.StatusCode)
	}
	// An envelope naming a PR number is not this verb's shape.
	numbered := bytes.Replace(raw, []byte(`"body":`), []byte(`"number":8,"body":`), 1)
	if r := postSigned(t, client.URL, secret, numbered, pullRequestCommentMACInput(numbered)); r.StatusCode != http.StatusBadRequest {
		t.Fatalf("numbered envelope status = %d, want 400", r.StatusCode)
	}
	if forge.calls != calls+1 || len(forge.comments) != 2 {
		t.Fatalf("GitHub calls = %d (comments %d), want only the one valid envelope past the first", forge.calls, len(forge.comments))
	}

	// The nonce store is not optional.
	unguarded := httptest.NewServer(&PullRequestCommentHandler{Secret: secret, PullRequests: pullRequests})
	defer unguarded.Close()
	_, err = (&PullRequestCommentClient{URL: unguarded.URL, Secret: secret, HTTP: unguarded.Client()}).Comment(context.Background(), commentRequest("Fixed."))
	wantCommentCode(t, err, PullRequestUnavailable, http.StatusServiceUnavailable)
}
