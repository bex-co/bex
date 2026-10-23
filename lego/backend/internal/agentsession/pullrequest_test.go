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
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeForge models just enough of GitHub for the PR verb: whether the head is
// pushed and ahead, and the open PRs by head.
type fakeForge struct {
	mu        sync.Mutex
	notPushed bool
	notAhead  bool
	err       error
	calls     int
	last      struct{ owner, repo, head, title, body string }
	prs       map[string]*github.PullRequest
	bodies    map[int]string
}

func (f *fakeForge) SyncDraftPullRequest(_ context.Context, _ int64, owner, repo, head, title, body string) (github.DraftPullRequestResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.last.owner, f.last.repo, f.last.head, f.last.title, f.last.body = owner, repo, head, title, body
	switch {
	case f.err != nil:
		return github.DraftPullRequestResult{}, f.err
	case f.notPushed:
		return github.DraftPullRequestResult{}, github.ErrHeadBranchNotFound
	case f.notAhead:
		return github.DraftPullRequestResult{}, github.ErrHeadNotAhead
	}
	if f.prs == nil {
		f.prs, f.bodies = map[string]*github.PullRequest{}, map[int]string{}
	}
	created := false
	pr, ok := f.prs[head]
	if !ok {
		pr = &github.PullRequest{Number: 40 + len(f.prs) + 1, State: "open", Draft: true}
		pr.HTMLURL = "https://github.com/" + owner + "/" + repo + "/pull/" + strconv.Itoa(pr.Number)
		f.prs[head] = pr
		created = true
	}
	f.bodies[pr.Number] = body
	return github.DraftPullRequestResult{PullRequest: *pr, Created: created, HeadSHA: "0123456789abcdef0123456789abcdef01234567"}, nil
}

type prSessions struct {
	mu       sync.Mutex
	session  store.AgentSession
	recorded int
}

func (s *prSessions) GetAgentSession(_ context.Context, id string) (store.AgentSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != s.session.ID {
		return store.AgentSession{}, store.ErrNotFound
	}
	return s.session, nil
}

func (s *prSessions) RecordAgentSessionPullRequest(_ context.Context, id, prURL string, prNumber int, headSHA string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != s.session.ID {
		return store.ErrNotFound
	}
	s.recorded++
	s.session.PRURL, s.session.PRNumber, s.session.HeadSHA = prURL, prNumber, headSHA
	return nil
}

func prSession() store.AgentSession {
	session := activeSession()
	session.Repo, session.Branch = "octo/repo", "bex-agent/task-1"
	session.AgentConfig = []byte(`{"agent":"claude","task":"t"}`)
	return session
}

func prRequest(title, body string) PullRequestRequest {
	return PullRequestRequest{MintRequest: validMintRequest(), PullRequestContent: PullRequestContent{Title: title, Body: body}}
}

func newPullRequester(forge *fakeForge, sessions *prSessions, audit core.AuditSink) *PullRequester {
	return &PullRequester{
		GitHub: forge, Sessions: sessions, Audit: audit,
		Connections:  fakeConnections{row: store.GitConnection{WorkspaceID: "tea-a", InstallationID: 42, AccountLogin: "octo"}},
		DashboardURL: "https://dashboard.bex.co",
	}
}

func wantPRCode(t *testing.T, err error, code string, status int) {
	t.Helper()
	var typed *PullRequestError
	if !errors.As(err, &typed) || typed.Code != code || typed.Status != status {
		t.Fatalf("error = %#v, want %s (%d)", err, code, status)
	}
	if !strings.Contains(typed.Hint, "Do not try another route") {
		t.Fatalf("hint %q does not steer the agent away from other routes", typed.Hint)
	}
}

func TestPullRequesterCreatesThenUpdatesTheSameDraftWithOneFooter(t *testing.T) {
	forge, sessions, audit := &fakeForge{}, &prSessions{session: prSession()}, &auditRecorder{}
	p := newPullRequester(forge, sessions, audit)

	first, err := p.Sync(context.Background(), prRequest("Add retries", "## Summary\nfirst"))
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || first.Number == 0 || first.URL == "" || first.HeadSHA == "" {
		t.Fatalf("create = %+v", first)
	}
	if forge.last.owner != "octo" || forge.last.repo != "repo" || forge.last.head != "bex-agent/task-1" {
		t.Fatalf("GitHub target = %+v, want the session's own repo + bound branch", forge.last)
	}
	// The agent copies the PR's current body (footer included) and edits it,
	// twice: the footer must still appear exactly once.
	body := forge.bodies[first.Number] + "\n\nsecond edit"
	if _, err := p.Sync(context.Background(), prRequest("Add retries", body)); err != nil {
		t.Fatal(err)
	}
	third, err := p.Sync(context.Background(), prRequest("Add retries (v2)", forge.bodies[first.Number]+"\nthird edit"))
	if err != nil {
		t.Fatal(err)
	}
	if third.Created || third.Number != first.Number || third.URL != first.URL {
		t.Fatalf("update = %+v, want the same PR as %+v", third, first)
	}
	final := forge.bodies[first.Number]
	if n := strings.Count(final, PullRequestFooterMarker); n != 1 {
		t.Fatalf("footer appears %d times:\n%s", n, final)
	}
	if strings.Count(final, "Opened by a bex agent session") != 1 {
		t.Fatalf("footer text duplicated:\n%s", final)
	}
	wantFooter := "\n\n" + PullRequestFooterMarker + "\n---\n🤖 Opened by a bex agent session (claude) · [view session](https://dashboard.bex.co/agents/ags-one)"
	if !strings.HasSuffix(final, wantFooter) {
		t.Fatalf("body does not end with the footer:\n%s", final)
	}
	if !strings.HasPrefix(final, "## Summary\nfirst") || !strings.Contains(final, "third edit") {
		t.Fatalf("agent body lost: %s", final)
	}
	if sessions.recorded != 3 || sessions.session.PRNumber != first.Number || sessions.session.PRURL != first.URL || sessions.session.Phase != "running" {
		t.Fatalf("session record = %+v (recorded %d)", sessions.session, sessions.recorded)
	}
	if len(audit.events) != 3 || audit.events[0].Verb != AuditVerbPullRequest || audit.events[0].Outcome != core.AuditAllowed || audit.events[0].TargetName != "octo/repo" {
		t.Fatalf("audit = %+v", audit.events)
	}
}

func TestPullRequestFooterFallsBackToSessionID(t *testing.T) {
	got := PullRequestBodyWithFooter("body\n\n", "codex", "", "ags-one")
	want := "body\n\n" + PullRequestFooterMarker + "\n---\n🤖 Opened by a bex agent session (codex) · session `ags-one`"
	if got != want {
		t.Fatalf("footer = %q, want %q", got, want)
	}
	if got := PullRequestBodyWithFooter("", "codex", "", "ags-one"); !strings.HasPrefix(got, PullRequestFooterMarker) {
		t.Fatalf("empty body footer = %q", got)
	}
	// Text the agent wrote below an old footer survives; the old footer does not.
	edited := PullRequestBodyWithFooter(want+"\n\n## Testing\nran it", "codex", "https://d.example/agents/ags-one", "ags-one")
	if strings.Count(edited, PullRequestFooterMarker) != 1 || strings.Contains(edited, "session `ags-one`") || !strings.Contains(edited, "body\n\n## Testing\nran it\n\n"+PullRequestFooterMarker) {
		t.Fatalf("re-footered body = %q", edited)
	}
}

func TestPullRequesterTypedRefusals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*fakeForge, *prSessions, *PullRequestRequest)
		code   string
		status int
	}{
		{"branch not pushed", func(f *fakeForge, _ *prSessions, _ *PullRequestRequest) { f.notPushed = true }, PullRequestBranchNotPushed, http.StatusConflict},
		{"not ahead", func(f *fakeForge, _ *prSessions, _ *PullRequestRequest) { f.notAhead = true }, PullRequestNotAhead, http.StatusConflict},
		{"github failure", func(f *fakeForge, _ *prSessions, _ *PullRequestRequest) {
			f.err = &github.APIError{Status: 500, Body: "secret upstream text"}
		}, PullRequestUpstreamError, http.StatusBadGateway},
		{"stale pod generation", func(_ *fakeForge, _ *prSessions, r *PullRequestRequest) { r.PodName = "sbx-0-0" }, PullRequestForbidden, http.StatusForbidden},
		{"terminal session", func(_ *fakeForge, s *prSessions, _ *PullRequestRequest) { s.session.Phase = "completed" }, PullRequestForbidden, http.StatusForbidden},
		{"other session", func(_ *fakeForge, _ *prSessions, r *PullRequestRequest) { r.SessionID = "ags-two" }, PullRequestForbidden, http.StatusForbidden},
		{"branch differs from the record", func(_ *fakeForge, _ *prSessions, r *PullRequestRequest) { r.Branch = "bex-agent/other" }, PullRequestForbidden, http.StatusForbidden},
		{"repo differs from the record", func(_ *fakeForge, _ *prSessions, r *PullRequestRequest) { r.Repository = "octo/other" }, PullRequestForbidden, http.StatusForbidden},
		{"multi-line title", func(_ *fakeForge, _ *prSessions, r *PullRequestRequest) { r.Title = "one\ntwo" }, PullRequestInvalidRequest, http.StatusUnprocessableEntity},
		{"empty title", func(_ *fakeForge, _ *prSessions, r *PullRequestRequest) { r.Title = "  " }, PullRequestInvalidRequest, http.StatusUnprocessableEntity},
		{"long title", func(_ *fakeForge, _ *prSessions, r *PullRequestRequest) {
			r.Title = strings.Repeat("é", MaxPullRequestTitleRunes+1)
		}, PullRequestTooLarge, http.StatusRequestEntityTooLarge},
		{"long body", func(_ *fakeForge, _ *prSessions, r *PullRequestRequest) {
			r.Body = strings.Repeat("x", MaxPullRequestBodyBytes+1)
		}, PullRequestTooLarge, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			forge, sessions, audit := &fakeForge{}, &prSessions{session: prSession()}, &auditRecorder{}
			req := prRequest("Title", "Body")
			tc.mutate(forge, sessions, &req)
			_, err := newPullRequester(forge, sessions, audit).Sync(context.Background(), req)
			wantPRCode(t, err, tc.code, tc.status)
			if strings.Contains(err.Error(), "secret upstream text") {
				t.Fatal("upstream GitHub text leaked into the refusal")
			}
			if sessions.recorded != 0 {
				t.Fatal("a refused request recorded a PR on the session")
			}
			if len(audit.events) != 1 || audit.events[0].Outcome != core.AuditDenied {
				t.Fatalf("denied audit = %+v", audit.events)
			}
		})
	}
	// The not-pushed hint names the exact push the overlay prescribes.
	forge, sessions := &fakeForge{notPushed: true}, &prSessions{session: prSession()}
	_, err := newPullRequester(forge, sessions, nil).Sync(context.Background(), prRequest("Title", "Body"))
	var refusal *PullRequestError
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Hint, "git push origin HEAD:"+sessions.session.Branch+"`") {
		t.Fatalf("branch_not_pushed refusal = %v", err)
	}
	// A title at exactly the cap passes (runes, not bytes).
	forge, sessions = &fakeForge{}, &prSessions{session: prSession()}
	if _, err := newPullRequester(forge, sessions, nil).Sync(context.Background(), prRequest(strings.Repeat("é", MaxPullRequestTitleRunes), "b")); err != nil {
		t.Fatalf("title at the cap refused: %v", err)
	}
}

// The signed hop end to end: the gateway client sees bex-api's typed refusal
// (code, status, hint), and a success carries no credential.
func TestPullRequestHandlerAndClientRoundTrip(t *testing.T) {
	forge, sessions := &fakeForge{}, &prSessions{session: prSession()}
	api := httptest.NewServer(&PullRequestHandler{Secret: []byte("secret"), PullRequests: newPullRequester(forge, sessions, nil)})
	defer api.Close()
	client := &PullRequestClient{URL: api.URL + InternalPullRequestPath, Secret: []byte("secret"), HTTP: api.Client()}

	resp, err := client.Sync(context.Background(), prRequest("T", "B"))
	if err != nil || !resp.Created || resp.Number == 0 {
		t.Fatalf("sync = %+v err=%v", resp, err)
	}
	forge.notAhead = true
	_, err = client.Sync(context.Background(), prRequest("T", "B"))
	wantPRCode(t, err, PullRequestNotAhead, http.StatusConflict)

	// A wrong secret is refused before any GitHub call.
	calls := forge.calls
	bad := &PullRequestClient{URL: client.URL, Secret: []byte("wrong"), HTTP: api.Client()}
	_, err = bad.Sync(context.Background(), prRequest("T", "B"))
	wantPRCode(t, err, PullRequestForbidden, http.StatusForbidden)
	if forge.calls != calls {
		t.Fatal("an unsigned request reached GitHub")
	}

	// The raw wire body of a success names no token field.
	forge.notAhead = false
	raw := rawSync(t, client, prRequest("T", "B"))
	if strings.Contains(strings.ToLower(raw), "token") || !strings.Contains(raw, `"headSha"`) {
		t.Fatalf("success body = %s", raw)
	}
}

// rawSync posts a signed request and returns the response body verbatim.
func rawSync(t *testing.T, c *PullRequestClient, req PullRequestRequest) string {
	t.Helper()
	req.Nonce = newNonce()
	resp, err := signedPost(context.Background(), c.URL, c.Secret, c.now(), c.HTTP, req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return string(raw)
}
