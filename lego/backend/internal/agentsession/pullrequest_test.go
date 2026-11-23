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
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

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
	last      struct {
		owner, repo, head, title, body string
		draft                          bool
	}
	// noDrafts refuses a draft as a repository whose plan has none.
	noDrafts bool
	prs      map[string]*github.PullRequest
	bodies   map[int]string
	comments []forgeComment
	// existing is a person's pull request a takeover session works on;
	// sessionHead is its session branch's head on GitHub ("" when not
	// pushed), diverged makes the fast-forward refuse, and moved records the
	// branch and commit it fast-forwarded.
	existing    github.PullRequestDetail
	sessionHead string
	diverged    bool
	protected   bool
	// ruleProtected is GitHub's answer to BranchProtected, and protectErr
	// fails that read.
	ruleProtected bool
	protectErr    error
	reads         int
	moved         []string
	// readied records the node ids marked ready, and readyErr fails that
	// call; merges records each merge, and mergeErr fails it.
	readied    []string
	readyErr   error
	merges     []forgeMerge
	mergeErr   error
	mergeCalls int
	// state is the pull request's merge state and stateErr fails its read;
	// armed records each auto-merge armed (armErr fails it), enqueued each
	// head queued (enqueueErr fails it), and disabled each disarm by the
	// state it was given: queued, stack or armed (disableErr fails it).
	state      github.MergeState
	stateErr   error
	stateReads int
	// rereadErr fails every state read after the first, as GitHub's read
	// back after an arm, a queue or a disarm may.
	rereadErr  error
	armed      []forgeMerge
	armErr     error
	enqueued   []string
	enqueueErr error
	disabled   []string
	disableErr error
	// other is a second pull request of the repository, a person's or
	// another session's, which a named number reaches (w10/041).
	other *github.PullRequestDetail
	// asyncMerges records each asynchronous merge of a stack (w10/053),
	// asyncResult is GitHub's result (merged by default) and asyncErr fails it.
	asyncMerges []forgeMerge
	asyncResult github.AsyncMergeResult
	asyncErr    error
}

// byNode is the pull request with GraphQL node id node, nil for none.
func (f *fakeForge) byNode(node string) *github.PullRequestDetail {
	switch {
	case node == f.existing.NodeID:
		return &f.existing
	case f.other != nil && node == f.other.NodeID:
		return f.other
	}
	return nil
}

// forgeMerge is one merge, or one auto-merge armed (by node), the merge verb
// asked for.
type forgeMerge struct {
	installation int64
	owner, repo  string
	number       int
	node         string
	opts         github.MergeOptions
}

func (f *fakeForge) PullRequestMergeState(_ context.Context, _ int64, _, nodeID string) (github.MergeState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stateReads++
	if f.byNode(nodeID) == nil {
		return github.MergeState{}, &github.APIError{Status: http.StatusNotFound}
	}
	if f.stateReads > 1 && f.rereadErr != nil {
		return github.MergeState{}, f.rereadErr
	}
	return f.state, f.stateErr
}

func (f *fakeForge) EnablePullRequestAutoMerge(_ context.Context, installation int64, owner, repo, nodeID string, opts github.MergeOptions) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.armErr != nil {
		return "", f.armErr
	}
	f.armed = append(f.armed, forgeMerge{installation: installation, owner: owner, repo: repo, node: nodeID, opts: opts})
	method := cmp.Or(opts.Method, github.MergeMethodSquash)
	armedAt := time.Date(2026, 11, 22, 9, 30, 0, 0, time.UTC)
	f.state.AutoMerge = &store.PRAutoMerge{Method: method, EnabledBy: forgeAppLogin, EnabledAt: &armedAt}
	f.state.StackAutoMerge = opts.IncludeStack
	return method, nil
}

// forgeAppLogin is the login GitHub names the App by when it arms auto-merge.
const forgeAppLogin = "bex-co[bot]"

func (f *fakeForge) EnqueuePullRequest(_ context.Context, _ int64, _, nodeID, headSHA string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.enqueueErr != nil {
		return f.enqueueErr
	}
	f.enqueued = append(f.enqueued, nodeID+"@"+headSHA)
	f.state.InMergeQueue = true
	return nil
}

func (f *fakeForge) DisablePullRequestAutoMerge(_ context.Context, _ int64, _, nodeID string, state github.MergeState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.disableErr != nil {
		return f.disableErr
	}
	if f.byNode(nodeID) == nil {
		return &github.APIError{Status: http.StatusNotFound}
	}
	disarm := "armed"
	switch {
	case state.InMergeQueue:
		disarm = "queued"
	case state.StackAutoMerge:
		disarm = "stack"
	}
	f.disabled = append(f.disabled, disarm)
	f.state.AutoMerge, f.state.InMergeQueue, f.state.StackAutoMerge = nil, false, false
	return nil
}

func (f *fakeForge) MarkPullRequestReady(_ context.Context, _ int64, _, nodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readyErr != nil {
		return f.readyErr
	}
	f.readied = append(f.readied, nodeID)
	if pr := f.byNode(nodeID); pr != nil {
		pr.Draft = false
	}
	return nil
}

func (f *fakeForge) MergePullRequest(_ context.Context, installation int64, owner, repo string, number int, opts github.MergeOptions) (github.MergeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mergeCalls++
	if f.mergeErr != nil {
		return github.MergeResult{}, f.mergeErr
	}
	f.merges = append(f.merges, forgeMerge{installation: installation, owner: owner, repo: repo, number: number, opts: opts})
	method := opts.Method
	if method == "" {
		method = github.MergeMethodSquash
	}
	return github.MergeResult{SHA: "fedcba9876543210fedcba9876543210fedcba98", Method: method}, nil
}

func (f *fakeForge) MergePullRequestAsync(_ context.Context, installation int64, owner, repo string, number int, opts github.MergeOptions) (github.AsyncMergeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asyncMerges = append(f.asyncMerges, forgeMerge{installation: installation, owner: owner, repo: repo, number: number, opts: opts})
	if f.asyncErr != nil {
		return github.AsyncMergeResult{}, f.asyncErr
	}
	if f.asyncResult.Status == github.AsyncMergeEnqueued {
		f.state.InMergeQueue = true
	}
	if f.asyncResult.Status != "" {
		return f.asyncResult, nil
	}
	return github.AsyncMergeResult{Status: github.AsyncMergeMerged, SHA: "fedcba9876543210fedcba9876543210fedcba98", Method: cmp.Or(opts.Method, github.MergeMethodSquash)}, nil
}

func (f *fakeForge) GetPullRequest(_ context.Context, _ int64, _, _ string, number int) (github.PullRequestDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	if f.err != nil {
		return github.PullRequestDetail{}, f.err
	}
	if f.other != nil && number == f.other.Number {
		return *f.other, nil
	}
	if number != f.existing.Number {
		return github.PullRequestDetail{}, &github.APIError{Status: http.StatusNotFound}
	}
	return f.existing, nil
}

func (f *fakeForge) FastForwardBranch(_ context.Context, _ int64, _, _, branch, from string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case f.sessionHead == "":
		return "", github.ErrHeadBranchNotFound
	case f.diverged:
		return "", github.ErrNotFastForward
	case f.protected:
		return "", github.ErrBranchUpdateRefused
	}
	f.moved = append(f.moved, branch+"<-"+from+"@"+f.sessionHead)
	return f.sessionHead, nil
}

func (f *fakeForge) BranchProtected(_ context.Context, _ int64, _, _, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ruleProtected, f.protectErr
}

// forgeComment is one comment the comment verb posted.
type forgeComment struct {
	installation int64
	owner, repo  string
	number       int
	body         string
}

func (f *fakeForge) CreateIssueComment(_ context.Context, installation int64, owner, repo string, number int, body string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	f.comments = append(f.comments, forgeComment{installation: installation, owner: owner, repo: repo, number: number, body: body})
	return "https://github.com/" + owner + "/" + repo + "/pull/" + strconv.Itoa(number) + "#issuecomment-" + strconv.Itoa(900+len(f.comments)), nil
}

func (f *fakeForge) SyncPullRequest(_ context.Context, _ int64, owner, repo, head, title, body string, draft bool) (github.PullRequestSyncResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.last.owner, f.last.repo, f.last.head, f.last.title, f.last.body, f.last.draft = owner, repo, head, title, body, draft
	switch {
	case f.err != nil:
		return github.PullRequestSyncResult{}, f.err
	case f.notPushed:
		return github.PullRequestSyncResult{}, github.ErrHeadBranchNotFound
	case f.notAhead:
		return github.PullRequestSyncResult{}, github.ErrHeadNotAhead
	}
	if f.prs == nil {
		f.prs, f.bodies = map[string]*github.PullRequest{}, map[int]string{}
	}
	created := false
	pr, ok := f.prs[head]
	if !ok {
		if draft && f.noDrafts {
			return github.PullRequestSyncResult{}, github.ErrDraftNotSupported
		}
		pr = &github.PullRequest{Number: 40 + len(f.prs) + 1, State: "open", Draft: draft}
		pr.HTMLURL = "https://github.com/" + owner + "/" + repo + "/pull/" + strconv.Itoa(pr.Number)
		f.prs[head] = pr
		created = true
	}
	f.bodies[pr.Number] = body
	return github.PullRequestSyncResult{PullRequest: *pr, Created: created, HeadSHA: "0123456789abcdef0123456789abcdef01234567"}, nil
}

type prSessions struct {
	mu       sync.Mutex
	session  store.AgentSession
	recorded int
	// states records each pull request state written, and stateErr fails it;
	// repoStates records each state written on every session of a workspace
	// that recorded a pull request ("<workspace> <repo>#<n> <state>").
	states     []string
	stateErr   error
	repoStates []string
	// marks records each auto-merge and merge queue written on every session
	// of a workspace that recorded a pull request ("<workspace> <repo>#<n>
	// <marks>", w10/047), and marksErr fails it.
	marks    []string
	marksErr error
}

// mergeMarks spells an auto-merge and a merge queue for the marks record:
// "squash by octocat", "none", with " queued" when it is in the queue.
func mergeMarks(armed *store.PRAutoMerge, queued bool) string {
	out := "none"
	if armed != nil {
		out = armed.Method + " by " + cmp.Or(armed.EnabledBy, "nobody")
	}
	if queued {
		out += " queued"
	}
	return out
}

func (s *prSessions) RecordPullRequestMergeMarks(_ context.Context, workspaceIDs []string, repo string, number int, armed *store.PRAutoMerge, queued bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.marksErr != nil {
		return 0, s.marksErr
	}
	s.marks = append(s.marks, fmt.Sprintf("%s %s#%d %s", strings.Join(workspaceIDs, ","), repo, number, mergeMarks(armed, queued)))
	return 1, nil
}

func (s *prSessions) RecordPullRequestState(_ context.Context, workspaceIDs []string, repo string, number int, state string, _ time.Time, next *time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stateErr != nil {
		return 0, s.stateErr
	}
	if next != nil {
		return 0, errors.New("a merged pull request is never read again")
	}
	s.repoStates = append(s.repoStates, fmt.Sprintf("%s %s#%d %s", strings.Join(workspaceIDs, ","), repo, number, state))
	return 1, nil
}

func (s *prSessions) RecordAgentSessionPRState(_ context.Context, id, state string, _ time.Time, next *time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stateErr != nil {
		return false, s.stateErr
	}
	if id != s.session.ID || next != nil {
		return false, store.ErrNotFound
	}
	s.states = append(s.states, state)
	changed := s.session.PRState != state
	s.session.PRState = state
	return changed, nil
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

func TestPullRequesterCreatesThenUpdatesTheSamePullRequestWithOneFooter(t *testing.T) {
	forge, sessions, audit := &fakeForge{}, &prSessions{session: prSession()}, &auditRecorder{}
	p := newPullRequester(forge, sessions, audit)

	first, err := p.Sync(context.Background(), prRequest("Add retries", "## Summary\nfirst"))
	if err != nil {
		t.Fatal(err)
	}
	// Devin's default: a pull request ready for review, not a draft (w10/040).
	if !first.Created || first.Number == 0 || first.URL == "" || first.HeadSHA == "" || first.Draft {
		t.Fatalf("create = %+v", first)
	}
	if forge.last.owner != "octo" || forge.last.repo != "repo" || forge.last.head != "bex-agent/task-1" || forge.last.draft {
		t.Fatalf("GitHub target = %+v, want the session's own repo + bound branch, not a draft", forge.last)
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

// A request that asks for a draft opens one and answers draft: true; a
// repository whose plan has no drafts refuses it with its own code, and the
// same call without draft then opens the pull request (w10/040).
func TestPullRequesterOpensADraftOnlyWhenAsked(t *testing.T) {
	forge, sessions := &fakeForge{}, &prSessions{session: prSession()}
	p := newPullRequester(forge, sessions, &auditRecorder{})
	req := prRequest("WIP: retries", "body")
	req.Draft = true
	got, err := p.Sync(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Created || !got.Draft || !forge.last.draft {
		t.Fatalf("draft request = %+v, GitHub saw draft=%v", got, forge.last.draft)
	}

	forge, sessions = &fakeForge{noDrafts: true}, &prSessions{session: prSession()}
	p = newPullRequester(forge, sessions, &auditRecorder{})
	_, err = p.Sync(context.Background(), req)
	wantPRCode(t, err, PullRequestDraftNotSupported, http.StatusUnprocessableEntity)
	if sessions.recorded != 0 {
		t.Fatal("a refused draft was recorded on the session")
	}
	if got, err := p.Sync(context.Background(), prRequest("Retries", "body")); err != nil || !got.Created || got.Draft {
		t.Fatalf("the same call without draft = %+v, %v", got, err)
	}
}

// The signed hop carries draft only when it is set, so a request that does
// not ask for one is the same bytes it was before the option.
func TestPullRequestContentOmitsAnUnsetDraft(t *testing.T) {
	plain, _ := json.Marshal(PullRequestContent{Title: "t", Body: "b"})
	if string(plain) != `{"title":"t","body":"b"}` {
		t.Fatalf("content without a draft = %s", plain)
	}
	draft, _ := json.Marshal(PullRequestContent{Title: "t", Body: "b", Draft: true})
	if string(draft) != `{"title":"t","body":"b","draft":true}` {
		t.Fatalf("content with a draft = %s", draft)
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
