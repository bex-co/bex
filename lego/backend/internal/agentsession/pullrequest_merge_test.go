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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const recordedHead = "0123456789abcdef0123456789abcdef01234567"

// mergeSession is commentSession with the head bex recorded for PR #7.
func mergeSession() store.AgentSession {
	session := commentSession()
	session.HeadSHA = recordedHead
	return session
}

// sessionPR is GitHub's PR #7 of mergeSession: open, a draft, at the
// recorded head.
func sessionPR() github.PullRequestDetail {
	return github.PullRequestDetail{
		PullRequest: github.PullRequest{Number: 7, HTMLURL: "https://github.com/octo/repo/pull/7", State: "open", Draft: true},
		NodeID:      "PR_kwDOnode7", HeadRef: "bex-agent/task-1", HeadSHA: recordedHead,
		HeadRepo: "octo/repo", BaseRepo: "octo/repo", BaseRef: "main", DefaultBranch: "main",
	}
}

func readyPR() github.PullRequestDetail {
	pr := sessionPR()
	pr.Draft = false
	return pr
}

func readyRequest() PullRequestReadyRequest {
	return PullRequestReadyRequest{MintRequest: validMintRequest()}
}

func mergeRequest(content PullRequestMergeContent) PullRequestMergeRequest {
	return PullRequestMergeRequest{MintRequest: validMintRequest(), PullRequestMergeContent: content}
}

func wantMergeCode(t *testing.T, err error, code string, status int) *PullRequestError {
	t.Helper()
	var typed *PullRequestError
	if !errors.As(err, &typed) || typed.Code != code || typed.Status != status {
		t.Fatalf("error = %#v, want %s (%d)", err, code, status)
	}
	if !strings.Contains(typed.Hint, "mark_pull_request_ready and merge_pull_request are the only way") {
		t.Fatalf("hint %q does not steer the agent away from other routes", typed.Hint)
	}
	return typed
}

func wantAudit(t *testing.T, audit *auditRecorder, verb string, outcome core.AuditOutcome) {
	t.Helper()
	if len(audit.events) != 1 || audit.events[0].Verb != verb || audit.events[0].Outcome != outcome ||
		audit.events[0].TargetName != "octo/repo" || audit.events[0].Target != "agent-session:ags-one" || audit.events[0].Caller != "sbx-1-0" {
		t.Fatalf("audit = %+v, want one %s %s naming the repository", audit.events, verb, outcome)
	}
}

func TestPullRequestReadyUndraftsTheSessionsOwnPROnce(t *testing.T) {
	forge, sessions, audit := &fakeForge{existing: sessionPR()}, &prSessions{session: mergeSession()}, &auditRecorder{}
	p := newPullRequester(forge, sessions, audit)

	got, err := p.MarkReady(context.Background(), readyRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 7 || got.URL != "https://github.com/octo/repo/pull/7" || got.AlreadyReady {
		t.Fatalf("response = %+v", got)
	}
	if len(forge.readied) != 1 || forge.readied[0] != "PR_kwDOnode7" {
		t.Fatalf("readied = %v, want the recorded PR's node", forge.readied)
	}
	wantAudit(t, audit, AuditVerbPullRequestReady, core.AuditAllowed)

	// Ready already: nothing is sent to GitHub.
	again, err := p.MarkReady(context.Background(), readyRequest())
	if err != nil || !again.AlreadyReady || again.Number != 7 || len(forge.readied) != 1 {
		t.Fatalf("second ready = %+v err=%v (readied %v)", again, err, forge.readied)
	}
	if len(forge.merges) != 0 || len(sessions.states) != 0 {
		t.Fatal("marking ready merged or recorded a state")
	}
}

func TestPullRequestMergeMergesAtTheRecordedHeadAndRecordsMerged(t *testing.T) {
	forge, sessions, audit := &fakeForge{existing: readyPR()}, &prSessions{session: mergeSession()}, &auditRecorder{}
	p := newPullRequester(forge, sessions, audit)

	got, err := p.Merge(context.Background(), mergeRequest(PullRequestMergeContent{}))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Merged || got.Number != 7 || got.URL != "https://github.com/octo/repo/pull/7" || got.SHA == "" || got.Method != github.MergeMethodSquash || got.HeadSHA != recordedHead {
		t.Fatalf("response = %+v", got)
	}
	if len(forge.merges) != 1 {
		t.Fatalf("merges = %+v", forge.merges)
	}
	merge := forge.merges[0]
	if merge.installation != 42 || merge.owner != "octo" || merge.repo != "repo" || merge.number != 7 {
		t.Fatalf("GitHub target = %+v, want the session's own repo and recorded PR", merge)
	}
	if merge.opts != (github.MergeOptions{SHA: recordedHead, Base: "main"}) {
		t.Fatalf("merge options = %+v, want the recorded head pinned, no method and the base branch", merge.opts)
	}
	// The merged state is on the session in the same request.
	if len(sessions.states) != 1 || sessions.states[0] != store.PRStateMerged || sessions.session.PRState != store.PRStateMerged {
		t.Fatalf("recorded states = %v", sessions.states)
	}
	wantAudit(t, audit, AuditVerbPullRequestMerge, core.AuditAllowed)

	// A named method and commit text pass through as the agent wrote them.
	forge, sessions = &fakeForge{existing: readyPR()}, &prSessions{session: mergeSession()}
	named, err := newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{
		Method: github.MergeMethodRebase, CommitTitle: "  Add retries (#7) ", CommitMessage: "Retries the flaky call.\n",
	}))
	if err != nil || named.Method != github.MergeMethodRebase {
		t.Fatalf("named merge = %+v err=%v", named, err)
	}
	if opts := forge.merges[0].opts; opts.Method != github.MergeMethodRebase || opts.CommitTitle != "Add retries (#7)" || opts.CommitMessage != "Retries the flaky call.\n" || opts.SHA != recordedHead {
		t.Fatalf("named merge options = %+v", opts)
	}
}

// A merged state that cannot be recorded does not undo or hide the merge:
// the PR-state poller records it later.
func TestPullRequestMergeSucceedsWhenTheStateRecordFails(t *testing.T) {
	forge, sessions := &fakeForge{existing: readyPR()}, &prSessions{session: mergeSession(), stateErr: errors.New("db down")}
	got, err := newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{}))
	if err != nil || !got.Merged || len(forge.merges) != 1 {
		t.Fatalf("merge = %+v err=%v", got, err)
	}
}

// A takeover session's recorded pull request is the person's it took over:
// it may merge that one, at the head its fast-forward recorded.
func TestPullRequestMergeOfATakenOverPR(t *testing.T) {
	session := takeoverSession()
	session.HeadSHA = sessionHead
	pr := takenOverPR()
	pr.HeadSHA, pr.BaseRef = sessionHead, "main"
	forge, sessions := &fakeForge{existing: pr}, &prSessions{session: session}
	got, err := newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{}))
	if err != nil || !got.Merged || len(forge.merges) != 1 || forge.merges[0].opts.SHA != sessionHead {
		t.Fatalf("takeover merge = %+v err=%v (%+v)", got, err, forge.merges)
	}
	// Its own bex-agent branch is not the pull request's head: a pull request
	// on it is not the one the session took over.
	pr.HeadRef = session.Branch
	forge = &fakeForge{existing: pr}
	_, err = newPullRequester(forge, &prSessions{session: session}, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{}))
	wantMergeCode(t, err, PullRequestUpstreamError, http.StatusBadGateway)
}

func TestPullRequestMergeTypedRefusals(t *testing.T) {
	apiError := func(status int, message string) error {
		raw, _ := json.Marshal(map[string]string{"message": message, "documentation_url": "https://docs.github.com"})
		return &github.APIError{Status: status, Body: string(raw)}
	}
	cases := []struct {
		name    string
		mutate  func(*fakeForge, *prSessions, *PullRequestMergeRequest)
		code    string
		status  int
		message string
		// sent is whether the merge reached GitHub before it was refused.
		sent bool
	}{
		{name: "session has no pull request", mutate: func(_ *fakeForge, s *prSessions, _ *PullRequestMergeRequest) { s.session.PRNumber = 0 }, code: PullRequestNoPullRequest, status: http.StatusConflict},
		{name: "still a draft", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) { f.existing.Draft = true }, code: PullRequestDraft, status: http.StatusConflict},
		{name: "head moved past the record", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) {
			f.existing.HeadSHA = "99999999999999999999999999999999deadbeef"
		}, code: PullRequestHeadChanged, status: http.StatusConflict, message: "999999999999"},
		{name: "no head recorded", mutate: func(_ *fakeForge, s *prSessions, _ *PullRequestMergeRequest) { s.session.HeadSHA = "" }, code: PullRequestHeadChanged, status: http.StatusConflict, message: "nothing"},
		{name: "closed", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) { f.existing.State = "closed" }, code: PullRequestClosed, status: http.StatusConflict, message: "already closed"},
		{name: "merged by someone", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) {
			f.existing.State, f.existing.Merged = "closed", true
		}, code: PullRequestClosed, status: http.StatusConflict, message: "already merged"},
		{name: "GitHub returns another head branch", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) { f.existing.HeadRef = "main" }, code: PullRequestUpstreamError, status: http.StatusBadGateway},
		{name: "GitHub returns a fork's pull request", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) { f.existing.HeadRepo = "fork/repo" }, code: PullRequestUpstreamError, status: http.StatusBadGateway},
		{name: "GitHub returns another repository's", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) { f.existing.BaseRepo = "octo/other" }, code: PullRequestUpstreamError, status: http.StatusBadGateway},
		{name: "the PR cannot be read", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) { f.err = &github.APIError{Status: 500} }, code: PullRequestUpstreamError, status: http.StatusBadGateway},
		{name: "stale pod generation", mutate: func(_ *fakeForge, _ *prSessions, r *PullRequestMergeRequest) { r.PodName = "sbx-0-0" }, code: PullRequestForbidden, status: http.StatusForbidden},
		{name: "terminal session", mutate: func(_ *fakeForge, s *prSessions, _ *PullRequestMergeRequest) { s.session.Phase = "completed" }, code: PullRequestForbidden, status: http.StatusForbidden},
		{name: "other session", mutate: func(_ *fakeForge, _ *prSessions, r *PullRequestMergeRequest) { r.SessionID = "ags-two" }, code: PullRequestForbidden, status: http.StatusForbidden},
		{name: "other workspace", mutate: func(_ *fakeForge, _ *prSessions, r *PullRequestMergeRequest) { r.Workspace = "tea-b" }, code: PullRequestForbidden, status: http.StatusForbidden},
		{name: "repo differs from the record", mutate: func(_ *fakeForge, _ *prSessions, r *PullRequestMergeRequest) { r.Repository = "octo/other" }, code: PullRequestForbidden, status: http.StatusForbidden},
		{name: "incomplete identity", mutate: func(_ *fakeForge, _ *prSessions, r *PullRequestMergeRequest) { r.PodUID = "" }, code: PullRequestForbidden, status: http.StatusForbidden},
		{name: "unknown method", mutate: func(_ *fakeForge, _ *prSessions, r *PullRequestMergeRequest) { r.Method = "fast-forward" }, code: PullRequestInvalidRequest, status: http.StatusUnprocessableEntity},
		{name: "multi-line commit title", mutate: func(_ *fakeForge, _ *prSessions, r *PullRequestMergeRequest) { r.CommitTitle = "a\nb" }, code: PullRequestInvalidRequest, status: http.StatusUnprocessableEntity},
		{name: "NUL in commit message", mutate: func(_ *fakeForge, _ *prSessions, r *PullRequestMergeRequest) { r.CommitMessage = "a\x00b" }, code: PullRequestInvalidRequest, status: http.StatusUnprocessableEntity},
		{name: "oversized commit title", mutate: func(_ *fakeForge, _ *prSessions, r *PullRequestMergeRequest) {
			r.CommitTitle = strings.Repeat("é", MaxMergeCommitTitleRunes+1)
		}, code: PullRequestTooLarge, status: http.StatusRequestEntityTooLarge},
		{name: "oversized commit message", mutate: func(_ *fakeForge, _ *prSessions, r *PullRequestMergeRequest) {
			r.CommitMessage = strings.Repeat("x", MaxMergeCommitMessageBytes+1)
		}, code: PullRequestTooLarge, status: http.StatusRequestEntityTooLarge},
		{name: "required check pending", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) {
			f.mergeErr = apiError(http.StatusMethodNotAllowed, "Required status check \"test\" is expected.")
		}, code: PullRequestMergeBlocked, status: http.StatusConflict, message: `GitHub: Required status check "test" is expected.`, sent: true},
		{name: "required review", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) {
			f.mergeErr = apiError(http.StatusMethodNotAllowed, "Repository rule violations found\n\nAt least 1 approving review is required by reviewers with write access.\n\n")
		}, code: PullRequestMergeBlocked, status: http.StatusConflict, message: "GitHub: Repository rule violations found At least 1 approving review is required", sent: true},
		{name: "conflict", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) {
			f.mergeErr = apiError(http.StatusMethodNotAllowed, "Pull Request is not mergeable")
		}, code: PullRequestMergeBlocked, status: http.StatusConflict, message: "GitHub: Pull Request is not mergeable", sent: true},
		{name: "GitHub's head pin refuses", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) {
			f.mergeErr = apiError(http.StatusConflict, "Head branch was modified. Review and try the merge again.")
		}, code: PullRequestHeadChanged, status: http.StatusConflict, message: "GitHub: Head branch was modified", sent: true},
		{name: "GitHub refuses the method", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) {
			f.mergeErr = apiError(http.StatusMethodNotAllowed, "Merge commits are not allowed on this repository.")
		}, code: PullRequestMergeMethodNotAllowed, status: http.StatusUnprocessableEntity, message: "GitHub: Merge commits are not allowed", sent: true},
		{name: "repository settings refuse the method", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) {
			f.mergeErr = &github.MergeMethodError{Method: "merge", Allowed: []string{"squash", "rebase"}}
		}, code: PullRequestMergeMethodNotAllowed, status: http.StatusUnprocessableEntity, message: "does not allow merge; it allows squash, rebase", sent: true},
		{name: "the repository allows no method", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) {
			f.mergeErr = &github.MergeMethodError{}
		}, code: PullRequestMergeMethodNotAllowed, status: http.StatusUnprocessableEntity, message: "allows no merge method", sent: true},
		{name: "App not permitted", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) {
			f.mergeErr = apiError(http.StatusForbidden, "Resource not accessible by integration")
		}, code: PullRequestNotPermitted, status: http.StatusForbidden, message: "GitHub: Resource not accessible by integration", sent: true},
		{name: "installation lacks the permission", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) {
			f.mergeErr = github.ErrPermissionNotGranted
		}, code: PullRequestNotPermitted, status: http.StatusForbidden, message: "has not granted", sent: true},
		{name: "rate limited", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) {
			f.mergeErr = &github.APIError{Status: http.StatusForbidden, RateLimited: true}
		}, code: PullRequestRateLimited, status: http.StatusTooManyRequests, sent: true},
		{name: "GitHub fails", mutate: func(f *fakeForge, _ *prSessions, _ *PullRequestMergeRequest) {
			f.mergeErr = &github.APIError{Status: http.StatusInternalServerError, Body: "<html>secret upstream text</html>"}
		}, code: PullRequestUpstreamError, status: http.StatusBadGateway, sent: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			forge, sessions, audit := &fakeForge{existing: readyPR()}, &prSessions{session: mergeSession()}, &auditRecorder{}
			req := mergeRequest(PullRequestMergeContent{})
			tc.mutate(forge, sessions, &req)
			_, err := newPullRequester(forge, sessions, audit).Merge(context.Background(), req)
			typed := wantMergeCode(t, err, tc.code, tc.status)
			if !strings.Contains(typed.Message, tc.message) {
				t.Fatalf("message %q, want GitHub's reason %q", typed.Message, tc.message)
			}
			if strings.Contains(typed.Message, "secret upstream text") || strings.Contains(typed.Message, "docs.github.com") {
				t.Fatalf("message %q relays more than GitHub's own words", typed.Message)
			}
			if tc.sent != (forge.mergeCalls > 0) {
				t.Fatalf("GitHub was asked to merge %d times, want sent=%v", forge.mergeCalls, tc.sent)
			}
			if len(forge.merges) != 0 {
				t.Fatalf("a refused merge was merged: %+v", forge.merges)
			}
			merged := forge.existing.Merged && tc.code == PullRequestClosed
			if merged != (len(sessions.states) == 1) {
				t.Fatalf("recorded states = %v, want merged recorded only for a merged PR", sessions.states)
			}
			if len(audit.events) != 1 || audit.events[0].Verb != AuditVerbPullRequestMerge || audit.events[0].Outcome != core.AuditDenied {
				t.Fatalf("denied audit = %+v", audit.events)
			}
		})
	}
	// Every refusal of a pending, failing, reviewed or conflicting merge
	// tells the agent what to wait for or fix, and not to loop.
	hint := NewPullRequestMergeError(PullRequestMergeBlocked, "").Hint
	for _, want := range []string{"gh pr checks --watch", "fix the failure", "you cannot give it", "merge the base branch", "Do not call merge_pull_request in a loop"} {
		if !strings.Contains(hint, want) {
			t.Errorf("merge_blocked hint lacks %q: %q", want, hint)
		}
	}
	if hint := NewPullRequestMergeError(PullRequestHeadChanged, "").Hint; !strings.Contains(hint, "open_or_update_draft_pr") {
		t.Errorf("head_changed hint %q does not say how to record the new head", hint)
	}
}

func TestPullRequestReadyTypedRefusals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*fakeForge, *prSessions, *PullRequestReadyRequest)
		code   string
		status int
	}{
		{"session has no pull request", func(_ *fakeForge, s *prSessions, _ *PullRequestReadyRequest) { s.session.PRNumber = 0 }, PullRequestNoPullRequest, http.StatusConflict},
		{"closed", func(f *fakeForge, _ *prSessions, _ *PullRequestReadyRequest) { f.existing.State = "closed" }, PullRequestClosed, http.StatusConflict},
		{"another session", func(_ *fakeForge, _ *prSessions, r *PullRequestReadyRequest) { r.SessionID = "ags-two" }, PullRequestForbidden, http.StatusForbidden},
		{"stale pod generation", func(_ *fakeForge, _ *prSessions, r *PullRequestReadyRequest) { r.PodName = "sbx-0-0" }, PullRequestForbidden, http.StatusForbidden},
		{"no node id", func(f *fakeForge, _ *prSessions, _ *PullRequestReadyRequest) { f.existing.NodeID = "" }, PullRequestUpstreamError, http.StatusBadGateway},
		{"App not permitted", func(f *fakeForge, _ *prSessions, _ *PullRequestReadyRequest) {
			f.readyErr = &github.APIError{Status: http.StatusForbidden, Body: `{"message":"Resource not accessible by integration"}`}
		}, PullRequestNotPermitted, http.StatusForbidden},
		{"installation lacks the permission", func(f *fakeForge, _ *prSessions, _ *PullRequestReadyRequest) {
			f.readyErr = github.ErrPermissionNotGranted
		}, PullRequestNotPermitted, http.StatusForbidden},
		{"GitHub fails", func(f *fakeForge, _ *prSessions, _ *PullRequestReadyRequest) {
			f.readyErr = &github.APIError{Status: 502}
		}, PullRequestUpstreamError, http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			forge, sessions, audit := &fakeForge{existing: sessionPR()}, &prSessions{session: mergeSession()}, &auditRecorder{}
			req := readyRequest()
			tc.mutate(forge, sessions, &req)
			_, err := newPullRequester(forge, sessions, audit).MarkReady(context.Background(), req)
			wantMergeCode(t, err, tc.code, tc.status)
			if len(forge.readied) != 0 {
				t.Fatalf("a refused request marked %v ready", forge.readied)
			}
			if len(audit.events) != 1 || audit.events[0].Verb != AuditVerbPullRequestReady || audit.events[0].Outcome != core.AuditDenied {
				t.Fatalf("denied audit = %+v", audit.events)
			}
		})
	}
}

// The signed hops end to end: the gateway clients see bex-api's typed
// refusals, each verb's MAC domain keeps its signatures to itself, an
// envelope naming a pull request is refused, and a captured request cannot
// be replayed.
func TestPullRequestReadyAndMergeHandlersAndClientsRoundTrip(t *testing.T) {
	secret := []byte("secret")
	forge, sessions := &fakeForge{existing: sessionPR()}, &prSessions{session: mergeSession()}
	pullRequests := newPullRequester(forge, sessions, nil)
	mux := http.NewServeMux()
	mux.Handle(InternalPullRequestReadyPath, &PullRequestReadyHandler{Secret: secret, PullRequests: pullRequests, Nonce: &fakeNonceClaimer{}})
	mux.Handle(InternalPullRequestMergePath, &PullRequestMergeHandler{Secret: secret, PullRequests: pullRequests, Nonce: &fakeNonceClaimer{}})
	mux.Handle(InternalPullRequestCommentPath, &PullRequestCommentHandler{Secret: secret, PullRequests: pullRequests, Nonce: &fakeNonceClaimer{}})
	api := httptest.NewServer(mux)
	defer api.Close()
	ready := &PullRequestReadyClient{URL: api.URL + InternalPullRequestReadyPath, Secret: secret, HTTP: api.Client()}
	merger := &PullRequestMergeClient{URL: api.URL + InternalPullRequestMergePath, Secret: secret, HTTP: api.Client()}

	// A draft refuses the merge with its own code, through the gateway's vocabulary.
	_, err := merger.Merge(context.Background(), mergeRequest(PullRequestMergeContent{}))
	wantMergeCode(t, err, PullRequestDraft, http.StatusConflict)
	if got, err := ready.MarkReady(context.Background(), readyRequest()); err != nil || got.Number != 7 || got.AlreadyReady {
		t.Fatalf("ready = %+v err=%v", got, err)
	}
	merged, err := merger.Merge(context.Background(), mergeRequest(PullRequestMergeContent{Method: github.MergeMethodSquash}))
	if err != nil || !merged.Merged || merged.Method != github.MergeMethodSquash || merged.SHA == "" {
		t.Fatalf("merge = %+v err=%v", merged, err)
	}
	// GitHub's reason reaches the gateway as GitHub said it.
	forge.mergeErr = &github.APIError{Status: http.StatusMethodNotAllowed, Body: `{"message":"Required status check \"test\" is in progress."}`}
	_, err = merger.Merge(context.Background(), mergeRequest(PullRequestMergeContent{}))
	if typed := wantMergeCode(t, err, PullRequestMergeBlocked, http.StatusConflict); typed.Message != `GitHub: Required status check "test" is in progress.` {
		t.Fatalf("relayed message = %q", typed.Message)
	}
	forge.mergeErr = nil

	bad := &PullRequestMergeClient{URL: merger.URL, Secret: []byte("wrong"), HTTP: api.Client()}
	_, err = bad.Merge(context.Background(), mergeRequest(PullRequestMergeContent{}))
	wantMergeCode(t, err, PullRequestForbidden, http.StatusForbidden)

	envelope := mergeRequest(PullRequestMergeContent{})
	envelope.Kind, envelope.Nonce = PullRequestMergeKind, newNonce()
	raw, _ := json.Marshal(envelope)
	merges := len(forge.merges)
	// Neither a bare-body signature nor another verb's domain verifies here.
	for name, mac := range map[string][]byte{"bare body": raw, "ready domain": kindMACInput(PullRequestReadyKind, raw), "comment domain": kindMACInput(PullRequestCommentKind, raw)} {
		if r := postSigned(t, merger.URL, secret, raw, mac); r.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s MAC status = %d, want 401", name, r.StatusCode)
		}
	}
	// This verb's envelope is refused by the ready and comment verbs.
	for _, path := range []string{InternalPullRequestReadyPath, InternalPullRequestCommentPath} {
		for _, kind := range []string{PullRequestReadyKind, PullRequestCommentKind} {
			if r := postSigned(t, api.URL+path, secret, raw, kindMACInput(kind, raw)); r.StatusCode == http.StatusOK {
				t.Fatalf("%s accepted a merge envelope signed as %s", path, kind)
			}
		}
	}
	// A correctly signed envelope merges once; its replay is refused.
	if r := postSigned(t, merger.URL, secret, raw, kindMACInput(PullRequestMergeKind, raw)); r.StatusCode != http.StatusOK {
		t.Fatalf("signed envelope status = %d", r.StatusCode)
	}
	if r := postSigned(t, merger.URL, secret, raw, kindMACInput(PullRequestMergeKind, raw)); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("replay status = %d, want 401", r.StatusCode)
	}
	// An envelope naming a pull request, an owner or a head is not this verb's shape.
	for _, field := range []string{`"number":8,`, `"owner":"octo",`, `"sha":"abc",`} {
		named := bytes.Replace(raw, []byte(`"kind":`), []byte(field+`"kind":`), 1)
		if r := postSigned(t, merger.URL, secret, named, kindMACInput(PullRequestMergeKind, named)); r.StatusCode != http.StatusBadRequest {
			t.Fatalf("envelope with %s status = %d, want 400", field, r.StatusCode)
		}
	}
	if len(forge.merges) != merges+1 {
		t.Fatalf("merges = %d, want only the one valid envelope past %d", len(forge.merges), merges)
	}

	// The nonce store is not optional.
	unguarded := httptest.NewServer(&PullRequestMergeHandler{Secret: secret, PullRequests: pullRequests})
	defer unguarded.Close()
	_, err = (&PullRequestMergeClient{URL: unguarded.URL, Secret: secret, HTTP: unguarded.Client()}).Merge(context.Background(), mergeRequest(PullRequestMergeContent{}))
	wantMergeCode(t, err, PullRequestUnavailable, http.StatusServiceUnavailable)
}

// armedForge serves readyPR on a base branch without a queue whose required checks
// are still running.
func armedForge() *fakeForge {
	return &fakeForge{existing: readyPR(), state: github.MergeState{Status: "BLOCKED"}}
}

// w10/m577 DoD 1: auto arms GitHub auto-merge on the session's recorded pull
// request, pinned to the recorded head, with the method rule and the commit
// text, and answers armed within the call: not merged, and no merged state
// on the session, which the PR-state poller records once GitHub merges it.
func TestPullRequestMergeAutoArmsAutoMergeAtTheRecordedHead(t *testing.T) {
	forge, sessions, audit := armedForge(), &prSessions{session: mergeSession()}, &auditRecorder{}
	got, err := newPullRequester(forge, sessions, audit).Merge(context.Background(), mergeRequest(PullRequestMergeContent{
		Auto: true, Method: github.MergeMethodSquash, CommitTitle: " Add retries (#7) ", CommitMessage: "Retries.",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := PullRequestMergeResponse{URL: "https://github.com/octo/repo/pull/7", Number: 7, AutoMerge: true, Method: github.MergeMethodSquash, HeadSHA: recordedHead}
	if got != want {
		t.Fatalf("response = %+v, want %+v", got, want)
	}
	if len(forge.armed) != 1 || forge.armed[0].node != "PR_kwDOnode7" || forge.armed[0].owner != "octo" || forge.armed[0].repo != "repo" || forge.armed[0].installation != 42 {
		t.Fatalf("armed = %+v, want the recorded PR's node in the session's repository", forge.armed)
	}
	if opts := forge.armed[0].opts; opts != (github.MergeOptions{SHA: recordedHead, Method: github.MergeMethodSquash, CommitTitle: "Add retries (#7)", CommitMessage: "Retries.", Base: "main"}) {
		t.Fatalf("arm options = %+v, want the recorded head pinned", opts)
	}
	if forge.mergeCalls != 0 || len(sessions.states) != 0 || sessions.session.PRState == store.PRStateMerged {
		t.Fatalf("an armed pull request was merged (%d) or recorded %v", forge.mergeCalls, sessions.states)
	}
	wantAudit(t, audit, AuditVerbPullRequestMerge, core.AuditAllowed)

	// Armed already, by the session or a person: the same answer with the
	// armed method, and GitHub is not asked again.
	forge = armedForge()
	forge.state.AutoMerge, forge.state.AutoMergeMethod = true, github.MergeMethodRebase
	again, err := newPullRequester(forge, &prSessions{session: mergeSession()}, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{Auto: true}))
	if err != nil || !again.AutoMerge || again.Merged || again.Method != github.MergeMethodRebase || len(forge.armed) != 0 || forge.mergeCalls != 0 {
		t.Fatalf("armed again = %+v err=%v (armed %d, merges %d)", again, err, len(forge.armed), forge.mergeCalls)
	}
}

// w10/m577 DoD 3: a pull request GitHub would merge now is merged now, as
// gh pr merge --auto does (its isImmediatelyMergeable), whether the state
// read says so or GitHub's refusal to arm it does.
func TestPullRequestMergeAutoMergesAPullRequestMergeableNow(t *testing.T) {
	for name, mutate := range map[string]func(*fakeForge){
		"the state reads clean":    func(f *fakeForge) { f.state.Status = "CLEAN" },
		"the state reads unstable": func(f *fakeForge) { f.state.Status = "UNSTABLE" },
		"GitHub says it is clean": func(f *fakeForge) {
			f.armErr = fmt.Errorf("%w: %w", github.ErrMergeableNow, errors.New("clean status"))
		},
		"armed but a plain merge asked": func(f *fakeForge) { f.state.AutoMerge = true },
	} {
		t.Run(name, func(t *testing.T) {
			forge, sessions := armedForge(), &prSessions{session: mergeSession()}
			mutate(forge)
			content := PullRequestMergeContent{Auto: !forge.state.AutoMerge}
			got, err := newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(content))
			if err != nil || !got.Merged || got.AutoMerge || got.SHA == "" || len(forge.merges) != 1 || forge.merges[0].opts.SHA != recordedHead {
				t.Fatalf("merge = %+v err=%v merges=%+v", got, err, forge.merges)
			}
			if len(forge.armed) != 0 || len(sessions.states) != 1 || sessions.states[0] != store.PRStateMerged {
				t.Fatalf("armed=%+v states=%v, want a merge recorded merged", forge.armed, sessions.states)
			}
		})
	}
}

// w10/m577 DoD 2: a base branch that requires a merge queue takes the pull
// request into the queue, pinned to the recorded head, with or without auto,
// and never asks GitHub for a direct merge, which a queue refuses. A queued
// pull request answers the same and is left as it is.
func TestPullRequestMergeJoinsTheMergeQueue(t *testing.T) {
	for _, content := range []PullRequestMergeContent{{}, {Auto: true, Method: github.MergeMethodRebase, CommitTitle: "ignored by the queue"}} {
		forge, sessions := armedForge(), &prSessions{session: mergeSession()}
		forge.state.MergeQueueEnabled = true
		got, err := newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(content))
		want := PullRequestMergeResponse{URL: "https://github.com/octo/repo/pull/7", Number: 7, MergeQueue: true, HeadSHA: recordedHead}
		if err != nil || got != want {
			t.Fatalf("%+v: queue = %+v err=%v, want %+v", content, got, err, want)
		}
		if strings.Join(forge.enqueued, ",") != "PR_kwDOnode7@"+recordedHead || forge.mergeCalls != 0 || len(forge.armed) != 0 || len(sessions.states) != 0 {
			t.Fatalf("%+v: enqueued=%v merges=%d armed=%d states=%v", content, forge.enqueued, forge.mergeCalls, len(forge.armed), sessions.states)
		}
	}
	forge := armedForge()
	forge.state.MergeQueueEnabled, forge.state.InMergeQueue = true, true
	got, err := newPullRequester(forge, &prSessions{session: mergeSession()}, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{Auto: true}))
	if err != nil || !got.MergeQueue || len(forge.enqueued) != 0 || forge.mergeCalls != 0 {
		t.Fatalf("queued again = %+v err=%v enqueued=%v", got, err, forge.enqueued)
	}
	// GitHub's refusal to queue it comes back in GitHub's words.
	forge = armedForge()
	forge.state.MergeQueueEnabled = true
	forge.enqueueErr = &github.APIError{Status: http.StatusUnprocessableEntity, Body: `{"message":"Pull request is in dirty status"}`}
	_, err = newPullRequester(forge, &prSessions{session: mergeSession()}, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{}))
	if typed := wantMergeCode(t, err, PullRequestMergeBlocked, http.StatusConflict); typed.Message != "GitHub: Pull request is in dirty status" {
		t.Fatalf("queue refusal message = %q", typed.Message)
	}
}

// w10/m577 DoD 3: auto's refusals are typed, and a draft or a head bex has
// not recorded is refused before any GitHub call, the state read included.
func TestPullRequestMergeAutoTypedRefusals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*fakeForge, *prSessions)
		code   string
		status int
		// read is whether the merge state was read before the refusal.
		read    bool
		message string
	}{
		{name: "a draft", mutate: func(f *fakeForge, _ *prSessions) { f.existing.Draft = true }, code: PullRequestDraft, status: http.StatusConflict},
		{name: "a head bex has not recorded", mutate: func(f *fakeForge, _ *prSessions) { f.existing.HeadSHA = "99999999999999999999999999999999deadbeef" }, code: PullRequestHeadChanged, status: http.StatusConflict},
		{name: "auto-merge off", mutate: func(f *fakeForge, _ *prSessions) { f.armErr = github.ErrAutoMergeNotAllowed }, code: PullRequestAutoMergeNotAllowed, status: http.StatusConflict, read: true, message: "the repository does not allow auto-merge"},
		{name: "auto-merge off, in GitHub's words", mutate: func(f *fakeForge, _ *prSessions) {
			f.armErr = fmt.Errorf("%w: %w", github.ErrAutoMergeNotAllowed, &github.APIError{Status: http.StatusUnprocessableEntity, Body: `{"message":"Auto merge is not allowed for this repository"}`})
		}, code: PullRequestAutoMergeNotAllowed, status: http.StatusConflict, read: true, message: "GitHub: Auto merge is not allowed for this repository"},
		{name: "a method the repository refuses", mutate: func(f *fakeForge, _ *prSessions) {
			f.armErr = &github.MergeMethodError{Method: "squash", Allowed: []string{"merge"}}
		}, code: PullRequestMergeMethodNotAllowed, status: http.StatusUnprocessableEntity, read: true, message: "does not allow squash"},
		{name: "GitHub's head pin refuses the arm", mutate: func(f *fakeForge, _ *prSessions) {
			f.armErr = &github.APIError{Status: http.StatusUnprocessableEntity, Body: `{"message":"The head of the pull request does not match the expected head OID"}`}
		}, code: PullRequestHeadChanged, status: http.StatusConflict, read: true, message: "does not match"},
		{name: "GitHub refuses the App", mutate: func(f *fakeForge, _ *prSessions) {
			f.armErr = &github.APIError{Status: http.StatusForbidden, Body: `{"message":"Resource not accessible by integration"}`}
		}, code: PullRequestNotPermitted, status: http.StatusForbidden, read: true, message: "Resource not accessible by integration"},
		{name: "the state cannot be read", mutate: func(f *fakeForge, _ *prSessions) { f.stateErr = &github.APIError{Status: http.StatusBadGateway} }, code: PullRequestUpstreamError, status: http.StatusBadGateway, read: true},
		{name: "no node id", mutate: func(f *fakeForge, _ *prSessions) { f.existing.NodeID = "" }, code: PullRequestUpstreamError, status: http.StatusBadGateway, message: "node id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			forge, sessions, audit := armedForge(), &prSessions{session: mergeSession()}, &auditRecorder{}
			tc.mutate(forge, sessions)
			_, err := newPullRequester(forge, sessions, audit).Merge(context.Background(), mergeRequest(PullRequestMergeContent{Auto: true}))
			typed := wantMergeCode(t, err, tc.code, tc.status)
			if !strings.Contains(typed.Message, tc.message) {
				t.Fatalf("message %q, want %q", typed.Message, tc.message)
			}
			if (forge.stateReads > 0) != tc.read || len(forge.armed) != 0 || forge.mergeCalls != 0 || len(sessions.states) != 0 {
				t.Fatalf("state reads=%d armed=%d merges=%d states=%v", forge.stateReads, len(forge.armed), forge.mergeCalls, sessions.states)
			}
			wantAudit(t, audit, AuditVerbPullRequestMerge, core.AuditDenied)
		})
	}
	hint := NewPullRequestMergeError(PullRequestAutoMergeNotAllowed, "").Hint
	for _, want := range []string{"Allow auto-merge", `without "auto"`, "tell the user"} {
		if !strings.Contains(hint, want) {
			t.Errorf("auto_merge_not_allowed hint lacks %q: %q", want, hint)
		}
	}
	if hint := NewPullRequestMergeError(PullRequestMergeBlocked, "").Hint; !strings.Contains(hint, `"auto": true so GitHub merges it once they pass, and end your turn`) {
		t.Errorf("merge_blocked hint %q does not offer auto for pending checks", hint)
	}
	// A plain merge does not need the state: an unreadable one merges as before.
	for name, mutate := range map[string]func(*fakeForge){
		"state unreadable": func(f *fakeForge) { f.stateErr = &github.APIError{Status: http.StatusBadGateway} },
		"no node id":       func(f *fakeForge) { f.existing.NodeID = "" },
	} {
		forge := armedForge()
		mutate(forge)
		got, err := newPullRequester(forge, &prSessions{session: mergeSession()}, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{}))
		if err != nil || !got.Merged || forge.mergeCalls != 1 {
			t.Fatalf("%s: plain merge = %+v err=%v", name, got, err)
		}
	}
}

// w10/m577 DoD 5: disableAuto stops a later merge, whatever the head and
// even on a draft: a queued pull request leaves the queue, an armed one is
// disarmed, and one with neither answers the same with no call.
func TestPullRequestMergeDisableAutoUndoesTheArm(t *testing.T) {
	for _, tc := range []struct {
		name      string
		state     github.MergeState
		disabled  string
		moveHead  bool
		makeDraft bool
	}{
		{name: "armed", state: github.MergeState{AutoMerge: true}, disabled: "[false]", moveHead: true},
		{name: "queued", state: github.MergeState{MergeQueueEnabled: true, InMergeQueue: true}, disabled: "[true]"},
		{name: "neither", state: github.MergeState{}, disabled: "[]", makeDraft: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forge, sessions, audit := armedForge(), &prSessions{session: mergeSession()}, &auditRecorder{}
			forge.state = tc.state
			if tc.moveHead {
				forge.existing.HeadSHA = "99999999999999999999999999999999deadbeef"
			}
			forge.existing.Draft = tc.makeDraft
			got, err := newPullRequester(forge, sessions, audit).Merge(context.Background(), mergeRequest(PullRequestMergeContent{DisableAuto: true}))
			want := PullRequestMergeResponse{URL: "https://github.com/octo/repo/pull/7", Number: 7, AutoMergeDisabled: true, HeadSHA: recordedHead}
			if err != nil || got != want {
				t.Fatalf("disable = %+v err=%v, want %+v", got, err, want)
			}
			if fmt.Sprint(forge.disabled) != tc.disabled || forge.mergeCalls != 0 || len(forge.armed) != 0 || len(sessions.states) != 0 {
				t.Fatalf("disabled=%v merges=%d armed=%d states=%v", forge.disabled, forge.mergeCalls, len(forge.armed), sessions.states)
			}
			wantAudit(t, audit, AuditVerbPullRequestMerge, core.AuditAllowed)
		})
	}
	forge := armedForge()
	forge.state.AutoMerge = true
	forge.disableErr = &github.APIError{Status: http.StatusForbidden, Body: `{"message":"Resource not accessible by integration"}`}
	_, err := newPullRequester(forge, &prSessions{session: mergeSession()}, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{DisableAuto: true}))
	wantMergeCode(t, err, PullRequestNotPermitted, http.StatusForbidden)
	for _, content := range []PullRequestMergeContent{
		{DisableAuto: true, Auto: true}, {DisableAuto: true, Method: github.MergeMethodSquash}, {DisableAuto: true, CommitTitle: "x"}, {DisableAuto: true, CommitMessage: "x"},
	} {
		forge := armedForge()
		_, err := newPullRequester(forge, &prSessions{session: mergeSession()}, nil).Merge(context.Background(), mergeRequest(content))
		if typed := wantMergeCode(t, err, PullRequestInvalidRequest, http.StatusUnprocessableEntity); forge.reads != 0 || !strings.Contains(typed.Hint, `"disableAuto": true alone`) {
			t.Fatalf("%+v: reads=%d hint=%q", content, forge.reads, typed.Hint)
		}
	}
}

// w10/m577 DoD 1 and 6: the gateway client accepts exactly one outcome, so
// an armed, queued or disarmed answer gets through and a malformed one does
// not.
func TestPullRequestMergeResponseWellFormed(t *testing.T) {
	base := PullRequestMergeResponse{URL: "https://github.com/octo/repo/pull/7", Number: 7, HeadSHA: recordedHead}
	for _, tc := range []struct {
		name   string
		mutate func(*PullRequestMergeResponse)
		ok     bool
	}{
		{"merged", func(r *PullRequestMergeResponse) { r.Merged, r.SHA = true, "fedcba" }, true},
		{"armed", func(r *PullRequestMergeResponse) { r.AutoMerge = true }, true},
		{"queued", func(r *PullRequestMergeResponse) { r.MergeQueue = true }, true},
		{"disarmed", func(r *PullRequestMergeResponse) { r.AutoMergeDisabled = true }, true},
		{"no outcome", func(*PullRequestMergeResponse) {}, false},
		{"merged without its commit", func(r *PullRequestMergeResponse) { r.Merged = true }, false},
		{"merged and armed", func(r *PullRequestMergeResponse) { r.Merged, r.SHA, r.AutoMerge = true, "fedcba", true }, false},
		{"armed and queued", func(r *PullRequestMergeResponse) { r.AutoMerge, r.MergeQueue = true, true }, false},
		{"no pull request", func(r *PullRequestMergeResponse) { r.AutoMerge, r.Number = true, 0 }, false},
	} {
		response := base
		tc.mutate(&response)
		if response.wellFormed() != tc.ok {
			t.Errorf("%s: wellFormed = %v, want %v", tc.name, !tc.ok, tc.ok)
		}
	}
}
