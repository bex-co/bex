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

// wantAudit asserts one audit event of verb and outcome naming the session's
// pull request #7 (w10/041: the audit names which pull request).
func wantAudit(t *testing.T, audit *auditRecorder, verb string, outcome core.AuditOutcome) {
	t.Helper()
	wantAuditOf(t, audit, verb, outcome, "octo/repo#7")
}

func wantAuditOf(t *testing.T, audit *auditRecorder, verb string, outcome core.AuditOutcome, targetName string) {
	t.Helper()
	if len(audit.events) != 1 || audit.events[0].Verb != verb || audit.events[0].Outcome != outcome ||
		audit.events[0].TargetName != targetName || audit.events[0].Target != "agent-session:ags-one" || audit.events[0].Caller != "sbx-1-0" {
		t.Fatalf("audit = %+v, want one %s %s naming %s", audit.events, verb, outcome, targetName)
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
	// tells the agent what to wait for or fix, and not to loop. A session's
	// origin is the bex Git proxy, so gh finds no pull request for the
	// current branch: the wait names the number (w10/042).
	hint := NewPullRequestMergeError(PullRequestMergeBlocked, "").Hint
	for _, want := range []string{"gh pr checks <number> --watch", "the `number` open_or_update_draft_pr returned", "fix the failure", "you cannot give it", "merge the base branch", "Do not call merge_pull_request in a loop"} {
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
	// An envelope naming an owner, a repository or a head is not this verb's
	// shape; a pull request number is (w10/041).
	for _, field := range []string{`"owner":"octo",`, `"sha":"abc",`, `"pullRequest":8,`} {
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
	// w10/047: the arm is recorded at once, as GitHub reads it back: by the
	// App, on every session of the workspace that recorded the pull request.
	if got := strings.Join(sessions.marks, ","); got != "tea-a octo/repo#7 squash by "+forgeAppLogin {
		t.Fatalf("auto-merge recorded = %q", got)
	}
	wantAudit(t, audit, AuditVerbPullRequestMerge, core.AuditAllowed)

	// Armed already, by the session or a person: the same answer with the
	// armed method, GitHub is not asked again, and who armed it is recorded.
	forge, sessions = armedForge(), &prSessions{session: mergeSession()}
	forge.state.AutoMerge = &store.PRAutoMerge{Method: github.MergeMethodRebase, EnabledBy: "octocat"}
	again, err := newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{Auto: true}))
	if err != nil || !again.AutoMerge || again.Merged || again.Method != github.MergeMethodRebase || len(forge.armed) != 0 || forge.mergeCalls != 0 {
		t.Fatalf("armed again = %+v err=%v (armed %d, merges %d)", again, err, len(forge.armed), forge.mergeCalls)
	}
	if got := strings.Join(sessions.marks, ","); got != "tea-a octo/repo#7 rebase by octocat" {
		t.Fatalf("a person's auto-merge recorded = %q", got)
	}

	// The read back fails: the arm is recorded as the answer says, armed by
	// nobody named, for the PR-state poller to complete; a failed record does
	// not fail the arm, which stands on GitHub.
	forge, sessions = armedForge(), &prSessions{session: mergeSession()}
	forge.rereadErr = &github.APIError{Status: http.StatusBadGateway}
	if got, err := newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{Auto: true})); err != nil || !got.AutoMerge ||
		strings.Join(sessions.marks, ",") != "tea-a octo/repo#7 squash by nobody" {
		t.Fatalf("unread arm = %+v err=%v marks=%v", got, err, sessions.marks)
	}
	forge, sessions = armedForge(), &prSessions{session: mergeSession(), marksErr: errors.New("db down")}
	if got, err := newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{Auto: true})); err != nil || !got.AutoMerge {
		t.Fatalf("an arm whose record failed = %+v err=%v", got, err)
	}
}

// w10/048: a taken-over pull request in a GitHub stack targets the branch of
// the one below it, so auto arms the stack, as Devin Review's auto-merge on a
// stacked pull request does: GitHub lands it with every unmerged pull request
// below it, on the stack's base, under that branch's method rule. Its own
// merge state (clean against the branch below) does not merge it alone, a
// stack already armed above it is left as it is, and disableAuto disarms the
// stack's request.
func TestPullRequestMergeAutoArmsTheStackOfAStackedPullRequest(t *testing.T) {
	stacked := func() (*fakeForge, *prSessions) {
		session := takeoverSession()
		session.HeadSHA = sessionHead
		pr := takenOverPR()
		pr.NodeID, pr.HeadSHA, pr.BaseRef = "PR_kwDOnode7", sessionHead, "alice/part-1"
		return &fakeForge{existing: pr, state: github.MergeState{Status: "CLEAN", Stacked: true, StackBase: "main", StackLands: 2}}, &prSessions{session: session}
	}
	forge, sessions := stacked()
	got, err := newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{Auto: true, Method: github.MergeMethodRebase}))
	want := PullRequestMergeResponse{URL: "https://github.com/octo/repo/pull/7", Number: 7, AutoMerge: true, Method: github.MergeMethodRebase, Stack: true, PullRequestCount: 2, HeadSHA: sessionHead}
	if err != nil || got != want {
		t.Fatalf("stacked auto = %+v err=%v, want %+v", got, err, want)
	}
	if len(forge.armed) != 1 || forge.armed[0].opts != (github.MergeOptions{SHA: sessionHead, Method: github.MergeMethodRebase, Base: "main", IncludeStack: true}) {
		t.Fatalf("armed = %+v, want the stack armed at the recorded head on the stack's base", forge.armed)
	}
	if forge.mergeCalls != 0 || len(sessions.states) != 0 {
		t.Fatalf("a stacked pull request clean against the branch below was merged alone (%d) or recorded %v", forge.mergeCalls, sessions.states)
	}

	// The stack armed already, on it or above it: the same answer, no call,
	// and no count, since a request armed above it lands more than it.
	forge, sessions = stacked()
	forge.state.AutoMerge, forge.state.StackAutoMerge = &store.PRAutoMerge{Method: github.MergeMethodSquash, EnabledBy: "octocat"}, true
	got, err = newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{Auto: true}))
	if err != nil || !got.AutoMerge || !got.Stack || got.PullRequestCount != 0 || got.Method != github.MergeMethodSquash || len(forge.armed) != 0 || forge.mergeCalls != 0 {
		t.Fatalf("stack armed already = %+v err=%v (armed %d, merges %d)", got, err, len(forge.armed), forge.mergeCalls)
	}

	// disableAuto disarms the stack's request, for every pull request it covers.
	got, err = newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{DisableAuto: true}))
	if err != nil || !got.AutoMergeDisabled || !got.Stack || fmt.Sprint(forge.disabled) != "[stack]" {
		t.Fatalf("stack disarm = %+v err=%v (disabled %v)", got, err, forge.disabled)
	}

	// GitHub says the whole stack would merge now: it is merged now, as gh
	// pr merge --auto does, and a stack lands only through the asynchronous
	// merge, with every open pull request below it, on the stack's base
	// (w10/053).
	forge, sessions = stacked()
	forge.armErr = fmt.Errorf("%w: %w", github.ErrMergeableNow, errors.New("clean status"))
	got, err = newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{Auto: true, Method: github.MergeMethodRebase}))
	want = PullRequestMergeResponse{URL: "https://github.com/octo/repo/pull/7", Number: 7, Merged: true, SHA: "fedcba9876543210fedcba9876543210fedcba98", Method: github.MergeMethodRebase, Stack: true, PullRequestCount: 2, HeadSHA: sessionHead}
	if err != nil || got != want || forge.mergeCalls != 0 || len(forge.asyncMerges) != 1 ||
		forge.asyncMerges[0].opts != (github.MergeOptions{SHA: sessionHead, Method: github.MergeMethodRebase, Base: "main"}) {
		t.Fatalf("stack ready now = %+v err=%v (sync merges %d, async %+v), want %+v", got, err, forge.mergeCalls, forge.asyncMerges, want)
	}
	if len(sessions.states) != 1 || sessions.states[0] != store.PRStateMerged {
		t.Fatalf("stack ready now recorded %v, want merged", sessions.states)
	}
}

// w10/053: a pull request in a GitHub stack merged now lands with every open
// pull request below it, as Devin's merge button does, through GitHub's
// asynchronous merge, the only call that lands a stack: pinned, under the
// stack base's method rule, with the commit text, and with how many pull
// requests it landed. Its own base needs no merge queue of its own: one the
// stack's base requires is GitHub's to use (merge_action default), and the
// answer says the stack joined it. A merge GitHub is still landing at the
// bound answers mergePending and records nothing. A pull request in no stack
// keeps the synchronous merge.
func TestPullRequestMergeLandsAStackedPullRequestWithItsStack(t *testing.T) {
	stacked := func(mutate func(*fakeForge)) (*fakeForge, *prSessions) {
		session := takeoverSession()
		session.HeadSHA = sessionHead
		pr := takenOverPR()
		pr.NodeID, pr.HeadSHA, pr.BaseRef = "PR_kwDOnode7", sessionHead, "alice/part-2"
		forge := &fakeForge{existing: pr, state: github.MergeState{Status: "BLOCKED", Stacked: true, StackBase: "main", StackLands: 3}}
		if mutate != nil {
			mutate(forge)
		}
		return forge, &prSessions{session: session}
	}
	content := PullRequestMergeContent{Method: github.MergeMethodSquash, CommitTitle: " Part 2 (#7) ", CommitMessage: "The second part."}
	wantOpts := github.MergeOptions{SHA: sessionHead, Method: github.MergeMethodSquash, CommitTitle: "Part 2 (#7)", CommitMessage: "The second part.", Base: "main"}
	base := PullRequestMergeResponse{URL: "https://github.com/octo/repo/pull/7", Number: 7, HeadSHA: sessionHead, Stack: true, PullRequestCount: 3}
	for _, tc := range []struct {
		name   string
		mutate func(*fakeForge)
		want   func(*PullRequestMergeResponse)
		states string
		marks  string
	}{
		{"merged", nil, func(r *PullRequestMergeResponse) {
			r.Merged, r.SHA, r.Method = true, "fedcba9876543210fedcba9876543210fedcba98", github.MergeMethodSquash
		}, store.PRStateMerged, ""},
		{"the bottom pull request on a merge-queue base", func(f *fakeForge) { f.state.MergeQueueEnabled, f.state.StackLands = true, 1 }, func(r *PullRequestMergeResponse) {
			r.Merged, r.SHA, r.Method, r.PullRequestCount = true, "fedcba9876543210fedcba9876543210fedcba98", github.MergeMethodSquash, 1
		}, store.PRStateMerged, ""},
		{"queued", func(f *fakeForge) {
			f.asyncResult = github.AsyncMergeResult{Status: github.AsyncMergeEnqueued, Method: github.MergeMethodSquash}
		},
			func(r *PullRequestMergeResponse) { r.MergeQueue = true }, "", "tea-a octo/repo#7 none queued"},
		{"still landing at the bound", func(f *fakeForge) {
			f.asyncResult = github.AsyncMergeResult{Status: github.AsyncMergePending, Method: github.MergeMethodSquash}
		},
			func(r *PullRequestMergeResponse) { r.MergePending, r.Method = true, github.MergeMethodSquash }, "", ""},
		{"a stack the read could not count", func(f *fakeForge) { f.state.StackLands = 0 }, func(r *PullRequestMergeResponse) {
			r.Merged, r.SHA, r.Method, r.PullRequestCount = true, "fedcba9876543210fedcba9876543210fedcba98", github.MergeMethodSquash, 0
		}, store.PRStateMerged, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forge, sessions := stacked(tc.mutate)
			audit := &auditRecorder{}
			got, err := newPullRequester(forge, sessions, audit).Merge(context.Background(), mergeRequest(content))
			want := base
			tc.want(&want)
			if err != nil || got != want || !got.wellFormed() {
				t.Fatalf("merge = %+v err=%v, want %+v", got, err, want)
			}
			if len(forge.asyncMerges) != 1 || forge.asyncMerges[0].opts != wantOpts || forge.asyncMerges[0].number != 7 || forge.asyncMerges[0].owner != "octo" || forge.asyncMerges[0].repo != "repo" {
				t.Fatalf("async merges = %+v, want one of #7 pinned on the stack's base", forge.asyncMerges)
			}
			if forge.mergeCalls != 0 || len(forge.enqueued) != 0 || len(forge.armed) != 0 {
				t.Fatalf("a stacked pull request went to the synchronous merge (%d), the queue (%v) or an arm (%v)", forge.mergeCalls, forge.enqueued, forge.armed)
			}
			if got := strings.Join(sessions.states, ","); got != tc.states {
				t.Fatalf("recorded states = %q, want %q", got, tc.states)
			}
			if got := strings.Join(sessions.marks, ","); got != tc.marks {
				t.Fatalf("recorded marks = %q, want %q", got, tc.marks)
			}
			wantAudit(t, audit, AuditVerbPullRequestMerge, core.AuditAllowed)
		})
	}

	// A merge GitHub failed is refused in GitHub's words under the codes of
	// any merge.
	for _, tc := range []struct {
		message string
		code    string
		status  int
	}{
		{`Required status check "test" is expected.`, PullRequestMergeBlocked, http.StatusConflict},
		{"Pull request is closed.", PullRequestMergeBlocked, http.StatusConflict},
		{"Head branch was modified. Review and try the merge again.", PullRequestHeadChanged, http.StatusConflict},
		{"Squash merges are not allowed on this repository.", PullRequestMergeMethodNotAllowed, http.StatusUnprocessableEntity},
	} {
		forge, sessions := stacked(func(f *fakeForge) { f.asyncErr = &github.AsyncMergeFailedError{Message: tc.message} })
		_, err := newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(content))
		if typed := wantMergeCode(t, err, tc.code, tc.status); typed.Message != "GitHub: "+tc.message {
			t.Fatalf("%s: message = %q", tc.message, typed.Message)
		}
		if len(sessions.states) != 0 || len(sessions.marks) != 0 {
			t.Fatalf("%s: a failed merge recorded %v %v", tc.message, sessions.states, sessions.marks)
		}
	}
	// Another merge of it pending at another head is GitHub's 409.
	forge, sessions := stacked(func(f *fakeForge) {
		f.asyncErr = &github.APIError{Status: http.StatusConflict, Body: `{"status":"pending","details":{"message":"A merge request already exists for this pull request."}}`}
	})
	_, err := newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(content))
	if typed := wantMergeCode(t, err, PullRequestHeadChanged, http.StatusConflict); typed.Message != "GitHub: A merge request already exists for this pull request." {
		t.Fatalf("pending at another head message = %q", typed.Message)
	}

	// In no stack, the same merge is the synchronous one, as before.
	forge, sessions = stacked(func(f *fakeForge) { f.state = github.MergeState{Status: "CLEAN"} })
	got, err := newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(content))
	if err != nil || !got.Merged || got.Stack || got.PullRequestCount != 0 || forge.mergeCalls != 1 || len(forge.asyncMerges) != 0 || forge.merges[0].opts.Base != "alice/part-2" {
		t.Fatalf("unstacked merge = %+v err=%v (sync %d, async %d)", got, err, forge.mergeCalls, len(forge.asyncMerges))
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
		"armed but a plain merge asked": func(f *fakeForge) { f.state.AutoMerge = &store.PRAutoMerge{Method: github.MergeMethodSquash} },
	} {
		t.Run(name, func(t *testing.T) {
			forge, sessions := armedForge(), &prSessions{session: mergeSession()}
			mutate(forge)
			content := PullRequestMergeContent{Auto: forge.state.AutoMerge == nil}
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
		if got := strings.Join(sessions.marks, ","); got != "tea-a octo/repo#7 none queued" {
			t.Fatalf("%+v: queue recorded = %q", content, got)
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
	for _, want := range []string{"Allow auto-merge", "gh pr checks <number> --watch", `without "auto"`, "tell the user"} {
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
		{name: "armed", state: github.MergeState{AutoMerge: &store.PRAutoMerge{Method: github.MergeMethodSquash, EnabledBy: "octocat"}}, disabled: "[armed]", moveHead: true},
		{name: "queued", state: github.MergeState{MergeQueueEnabled: true, InMergeQueue: true}, disabled: "[queued]"},
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
			// w10/047: the disarm clears the recorded auto-merge and queue.
			if got := strings.Join(sessions.marks, ","); got != "tea-a octo/repo#7 none" {
				t.Fatalf("disarm recorded = %q", got)
			}
			wantAudit(t, audit, AuditVerbPullRequestMerge, core.AuditAllowed)
		})
	}
	forge := armedForge()
	forge.state.AutoMerge = &store.PRAutoMerge{Method: github.MergeMethodSquash}
	forge.disableErr = &github.APIError{Status: http.StatusForbidden, Body: `{"message":"Resource not accessible by integration"}`}
	_, err := newPullRequester(forge, &prSessions{session: mergeSession()}, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{DisableAuto: true}))
	wantMergeCode(t, err, PullRequestNotPermitted, http.StatusForbidden)
	for _, content := range []PullRequestMergeContent{
		{DisableAuto: true, Auto: true}, {DisableAuto: true, Method: github.MergeMethodSquash}, {DisableAuto: true, CommitTitle: "x"}, {DisableAuto: true, CommitMessage: "x"},
	} {
		forge := armedForge()
		_, err := newPullRequester(forge, &prSessions{session: mergeSession()}, nil).Merge(context.Background(), mergeRequest(content))
		if typed := wantMergeCode(t, err, PullRequestInvalidRequest, http.StatusUnprocessableEntity); forge.reads != 0 || !strings.Contains(typed.Hint, `"disableAuto": true with at most a "number"`) {
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
		{"still landing", func(r *PullRequestMergeResponse) { r.MergePending = true }, true},
		{"merged and still landing", func(r *PullRequestMergeResponse) { r.Merged, r.SHA, r.MergePending = true, "fedcba", true }, false},
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

// otherHead is the head GitHub reports for a pull request the session did not
// open, which it never recorded.
const otherHead = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// otherPRs are the pull requests of the session's repository w10/041 names
// by number: a person's, from a fork, and another bex session's.
func otherPRs() map[string]github.PullRequestDetail {
	return map[string]github.PullRequestDetail{
		"a person's": {
			PullRequest: github.PullRequest{Number: 42, HTMLURL: "https://github.com/octo/repo/pull/42", State: "open"},
			NodeID:      "PR_kwDOnode42", HeadRef: "fix-typo", HeadSHA: otherHead, HeadRepo: "alice/repo", BaseRepo: "octo/repo", BaseRef: "main",
		},
		"another session's": {
			PullRequest: github.PullRequest{Number: 42, HTMLURL: "https://github.com/octo/repo/pull/42", State: "open"},
			NodeID:      "PR_kwDOnode42", HeadRef: "bex-agent/task-2", HeadSHA: otherHead, HeadRepo: "octo/repo", BaseRepo: "octo/repo", BaseRef: "release",
		},
	}
}

// w10/041: a number names another pull request of the session's repository,
// a person's or another session's, as Devin's credentials can merge any pull
// request there. It is marked ready and merged pinned to the head GitHub
// reports for it now, its merged state lands on every session of the
// workspace that recorded it (never on this session, whose own pull request
// is still open), and the audit names which pull request.
func TestPullRequestReadyAndMergeOfAnotherPullRequestByNumber(t *testing.T) {
	for name, other := range otherPRs() {
		t.Run(name, func(t *testing.T) {
			draft := other
			draft.Draft = true
			forge, sessions, audit := &fakeForge{existing: readyPR(), other: &draft}, &prSessions{session: mergeSession()}, &auditRecorder{}
			p := newPullRequester(forge, sessions, audit)

			ready, err := p.MarkReady(context.Background(), PullRequestReadyRequest{MintRequest: validMintRequest(), PullRequestReadyContent: PullRequestReadyContent{Number: 42}})
			if err != nil {
				t.Fatal(err)
			}
			if ready != (PullRequestReadyResponse{URL: "https://github.com/octo/repo/pull/42", Number: 42, OtherPullRequest: true}) {
				t.Fatalf("ready = %+v", ready)
			}
			if len(forge.readied) != 1 || forge.readied[0] != "PR_kwDOnode42" {
				t.Fatalf("readied = %v, want the named pull request's node", forge.readied)
			}
			wantAuditOf(t, audit, AuditVerbPullRequestReady, core.AuditAllowed, "octo/repo#42")

			audit.events = nil
			got, err := p.Merge(context.Background(), mergeRequest(PullRequestMergeContent{Number: 42, Method: github.MergeMethodSquash}))
			if err != nil {
				t.Fatal(err)
			}
			if !got.Merged || got.Number != 42 || got.URL != "https://github.com/octo/repo/pull/42" || got.HeadSHA != otherHead || !got.OtherPullRequest || got.SHA == "" {
				t.Fatalf("merge = %+v", got)
			}
			if len(forge.merges) != 1 {
				t.Fatalf("merges = %+v", forge.merges)
			}
			merge := forge.merges[0]
			if merge.owner != "octo" || merge.repo != "repo" || merge.number != 42 || merge.opts.SHA != otherHead || merge.opts.Base != other.BaseRef {
				t.Fatalf("GitHub merge = %+v, want #42 of the session's repository at the head GitHub reported", merge)
			}
			if len(sessions.states) != 0 || sessions.session.PRState == store.PRStateMerged {
				t.Fatalf("the session's own pull request was recorded %v", sessions.states)
			}
			if len(sessions.repoStates) != 1 || sessions.repoStates[0] != "tea-a octo/repo#42 merged" {
				t.Fatalf("repository states = %v, want #42 merged on the workspace's sessions", sessions.repoStates)
			}
			wantAuditOf(t, audit, AuditVerbPullRequestMerge, core.AuditAllowed, "octo/repo#42")
		})
	}
}

// A session with no pull request of its own can still act on one it names,
// and naming its own number is the same as naming none: pinned to the head
// bex recorded, and recorded merged on the session itself.
func TestPullRequestMergeNamingNoneOrItsOwnNumber(t *testing.T) {
	other := otherPRs()["a person's"]
	session := mergeSession()
	session.PRNumber, session.PRURL, session.HeadSHA = 0, "", ""
	forge, sessions := &fakeForge{existing: readyPR(), other: &other}, &prSessions{session: session}
	got, err := newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{Number: 42}))
	if err != nil || !got.Merged || !got.OtherPullRequest || forge.merges[0].opts.SHA != otherHead {
		t.Fatalf("merge without a pull request of its own = %+v err=%v (%+v)", got, err, forge.merges)
	}

	forge, sessions = &fakeForge{existing: readyPR(), other: &other}, &prSessions{session: mergeSession()}
	got, err = newPullRequester(forge, sessions, nil).Merge(context.Background(), mergeRequest(PullRequestMergeContent{Number: 7}))
	if err != nil || !got.Merged || got.OtherPullRequest || got.HeadSHA != recordedHead || forge.merges[0].number != 7 || forge.merges[0].opts.SHA != recordedHead {
		t.Fatalf("merge naming its own number = %+v err=%v (%+v)", got, err, forge.merges)
	}
	if len(sessions.states) != 1 || len(sessions.repoStates) != 0 {
		t.Fatalf("states = %v repository states = %v, want the session's own record", sessions.states, sessions.repoStates)
	}
}

// w10/041: auto arms GitHub auto-merge on a named pull request pinned to the
// head GitHub reported, and disableAuto takes a number too.
func TestPullRequestMergeAutoAndDisableAutoOfANamedPullRequest(t *testing.T) {
	other := otherPRs()["another session's"]
	forge := &fakeForge{existing: readyPR(), other: &other, state: github.MergeState{Status: "BLOCKED"}}
	sessions := &prSessions{session: mergeSession()}
	p := newPullRequester(forge, sessions, nil)
	got, err := p.Merge(context.Background(), mergeRequest(PullRequestMergeContent{Number: 42, Auto: true}))
	if err != nil || !got.AutoMerge || got.Merged || !got.OtherPullRequest || got.HeadSHA != otherHead {
		t.Fatalf("auto = %+v err=%v", got, err)
	}
	if len(forge.armed) != 1 || forge.armed[0].node != "PR_kwDOnode42" || forge.armed[0].opts.SHA != otherHead || len(forge.merges) != 0 {
		t.Fatalf("armed = %+v merges = %+v, want #42 armed at GitHub's head", forge.armed, forge.merges)
	}
	got, err = p.Merge(context.Background(), mergeRequest(PullRequestMergeContent{Number: 42, DisableAuto: true}))
	if err != nil || !got.AutoMergeDisabled || !got.OtherPullRequest || len(forge.disabled) != 1 {
		t.Fatalf("disableAuto = %+v err=%v (disabled %v)", got, err, forge.disabled)
	}
	if len(sessions.states) != 0 || len(sessions.repoStates) != 0 {
		t.Fatalf("an armed or disarmed pull request was recorded as merged or closed: %v %v", sessions.states, sessions.repoStates)
	}
	// w10/047: both land on the sessions that recorded #42, not on this one's own.
	if got := strings.Join(sessions.marks, ","); got != "tea-a octo/repo#42 squash by "+forgeAppLogin+",tea-a octo/repo#42 none" {
		t.Fatalf("named pull request's auto-merge recorded = %q", got)
	}
}

// w10/041: a named pull request is refused for what it is, and the denied
// audit names it.
func TestPullRequestReadyAndMergeOfANamedPullRequestRefusals(t *testing.T) {
	cases := []struct {
		name   string
		number int
		mutate func(*fakeForge)
		code   string
		status int
		// message is part of the refusal's message; recorded is the
		// repository-wide state written, "" for none.
		message, recorded string
	}{
		{name: "no such pull request", number: 99, code: PullRequestNotFound, status: http.StatusNotFound, message: "octo/repo has no pull request #99"},
		{name: "a negative number", number: -1, code: PullRequestInvalidRequest, status: http.StatusUnprocessableEntity},
		{name: "GitHub returns another repository's", number: 42, mutate: func(f *fakeForge) { f.other.BaseRepo = "octo/other" }, code: PullRequestUpstreamError, status: http.StatusBadGateway},
		{name: "closed", number: 42, mutate: func(f *fakeForge) { f.other.State = "closed" }, code: PullRequestClosed, status: http.StatusConflict, message: "#42 is already closed"},
		{name: "merged by someone", number: 42, mutate: func(f *fakeForge) { f.other.State, f.other.Merged = "closed", true }, code: PullRequestClosed, status: http.StatusConflict,
			message: "#42 is already merged", recorded: "tea-a octo/repo#42 merged"},
		{name: "a draft", number: 42, mutate: func(f *fakeForge) { f.other.Draft = true }, code: PullRequestDraft, status: http.StatusConflict},
		{name: "pushed to after the read", number: 42, mutate: func(f *fakeForge) {
			f.mergeErr = &github.APIError{Status: http.StatusConflict, Body: `{"message":"Head branch was modified. Review and try the merge again."}`}
		}, code: PullRequestHeadChanged, status: http.StatusConflict, message: "GitHub: Head branch was modified"},
		{name: "the PR cannot be read", number: 42, mutate: func(f *fakeForge) { f.err = &github.APIError{Status: http.StatusInternalServerError} }, code: PullRequestUpstreamError, status: http.StatusBadGateway},
		{name: "GitHub reports no head", number: 42, mutate: func(f *fakeForge) { f.other.HeadSHA = "" }, code: PullRequestUpstreamError, status: http.StatusBadGateway, message: "did not return pull request #42's head"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			other := otherPRs()["a person's"]
			forge, sessions, audit := &fakeForge{existing: readyPR(), other: &other}, &prSessions{session: mergeSession()}, &auditRecorder{}
			if tc.mutate != nil {
				tc.mutate(forge)
			}
			_, err := newPullRequester(forge, sessions, audit).Merge(context.Background(), mergeRequest(PullRequestMergeContent{Number: tc.number}))
			typed := wantMergeCode(t, err, tc.code, tc.status)
			if !strings.Contains(typed.Message, tc.message) {
				t.Fatalf("message %q, want %q", typed.Message, tc.message)
			}
			if len(forge.merges) != 0 || len(sessions.states) != 0 {
				t.Fatalf("merges = %+v states = %v", forge.merges, sessions.states)
			}
			if got := strings.Join(sessions.repoStates, ","); got != tc.recorded {
				t.Fatalf("repository states = %q, want %q", got, tc.recorded)
			}
			want := fmt.Sprintf("octo/repo#%d", tc.number)
			if tc.number < 0 {
				want = ""
			}
			wantAuditOf(t, audit, AuditVerbPullRequestMerge, core.AuditDenied, want)
		})
	}
	// Mark-ready refuses the same way.
	forge := &fakeForge{existing: readyPR()}
	_, err := newPullRequester(forge, &prSessions{session: mergeSession()}, nil).MarkReady(context.Background(),
		PullRequestReadyRequest{MintRequest: validMintRequest(), PullRequestReadyContent: PullRequestReadyContent{Number: 99}})
	wantMergeCode(t, err, PullRequestNotFound, http.StatusNotFound)
	if hint := NewPullRequestMergeError(PullRequestNotFound, "").Hint; !strings.Contains(hint, "gh pr list") {
		t.Errorf("pull_request_not_found hint %q does not say how to check the number", hint)
	}
	if hint := NewPullRequestMergeError(PullRequestHeadChanged, "").Hint; !strings.Contains(hint, "For another pull request") {
		t.Errorf("head_changed hint %q does not cover another pull request", hint)
	}
}

// w10/041: a number crosses the signed hop in both verbs' envelopes, and the
// gateway client accepts the answer naming another pull request.
func TestPullRequestReadyAndMergeClientsCarryANumber(t *testing.T) {
	secret := []byte("secret")
	other := otherPRs()["a person's"]
	other.Draft = true
	forge := &fakeForge{existing: readyPR(), other: &other}
	pullRequests := newPullRequester(forge, &prSessions{session: mergeSession()}, nil)
	mux := http.NewServeMux()
	mux.Handle(InternalPullRequestReadyPath, &PullRequestReadyHandler{Secret: secret, PullRequests: pullRequests, Nonce: &fakeNonceClaimer{}})
	mux.Handle(InternalPullRequestMergePath, &PullRequestMergeHandler{Secret: secret, PullRequests: pullRequests, Nonce: &fakeNonceClaimer{}})
	api := httptest.NewServer(mux)
	defer api.Close()
	ready := &PullRequestReadyClient{URL: api.URL + InternalPullRequestReadyPath, Secret: secret, HTTP: api.Client()}
	merger := &PullRequestMergeClient{URL: api.URL + InternalPullRequestMergePath, Secret: secret, HTTP: api.Client()}
	if got, err := ready.MarkReady(context.Background(), PullRequestReadyRequest{MintRequest: validMintRequest(), PullRequestReadyContent: PullRequestReadyContent{Number: 42}}); err != nil || got.Number != 42 || !got.OtherPullRequest {
		t.Fatalf("ready = %+v err=%v", got, err)
	}
	got, err := merger.Merge(context.Background(), mergeRequest(PullRequestMergeContent{Number: 42}))
	if err != nil || !got.Merged || got.Number != 42 || !got.OtherPullRequest || got.HeadSHA != otherHead {
		t.Fatalf("merge = %+v err=%v", got, err)
	}
	if len(forge.merges) != 1 || forge.merges[0].number != 42 {
		t.Fatalf("merges = %+v", forge.merges)
	}
}
