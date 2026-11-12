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
	"net/http"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A session a `/bex` comment handed a person's pull request pushes only its
// own bex-agent/ branch; the PR verb fast-forwards the pull request's branch
// to it and changes nothing else on the pull request.

const (
	prHead      = "feature/fix-login"
	sessionHead = "89abcdef0123456789abcdef0123456789abcdef"
)

// takeoverSession is the durable row of a session that took over PR #7.
func takeoverSession() store.AgentSession {
	session := prSession()
	session.TakeoverPR, session.TakeoverHeadRef, session.PRNumber = 7, prHead, 7
	session.PRURL = "https://github.com/octo/repo/pull/7"
	return session
}

func takenOverPR() github.PullRequestDetail {
	return github.PullRequestDetail{PullRequest: github.PullRequest{Number: 7, HTMLURL: "https://github.com/octo/repo/pull/7", State: "open"},
		HeadRef: prHead, HeadRepo: "octo/repo", BaseRepo: "octo/repo", DefaultBranch: "main"}
}

// withPR is takenOverPR changed by mutate.
func withPR(mutate func(*github.PullRequestDetail)) github.PullRequestDetail {
	pr := takenOverPR()
	mutate(&pr)
	return pr
}

func TestPullRequesterFastForwardsATakenOverPRAndTouchesNothingElse(t *testing.T) {
	forge := &fakeForge{existing: takenOverPR(), sessionHead: sessionHead}
	sessions, audit := &prSessions{session: takeoverSession()}, &auditRecorder{}
	got, err := newPullRequester(forge, sessions, audit).Sync(context.Background(), prRequest("Agent title", "agent body"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Created || !got.TakenOver || got.Number != 7 || got.URL != "https://github.com/octo/repo/pull/7" || got.HeadSHA != sessionHead {
		t.Fatalf("takeover sync = %+v", got)
	}
	// The pull request's branch moved to the session branch's head; no draft
	// was opened or edited, so its title, body and draft state are the author's.
	if len(forge.moved) != 1 || forge.moved[0] != prHead+"<-bex-agent/task-1@"+sessionHead || forge.calls != 0 || len(forge.prs) != 0 {
		t.Fatalf("moved=%v sync calls=%d prs=%d", forge.moved, forge.calls, len(forge.prs))
	}
	if sessions.recorded != 1 || sessions.session.HeadSHA != sessionHead || sessions.session.PRNumber != 7 {
		t.Fatalf("session record = %+v (recorded %d)", sessions.session, sessions.recorded)
	}
	if len(audit.events) != 1 || audit.events[0].Verb != AuditVerbPullRequest {
		t.Fatalf("audit = %+v", audit.events)
	}
}

func TestPullRequesterTakeoverRefusals(t *testing.T) {
	cases := map[string]struct {
		forge  *fakeForge
		code   string
		status int
		says   string
	}{
		"the PR branch moved on": {&fakeForge{existing: takenOverPR(), sessionHead: sessionHead, diverged: true},
			PullRequestBranchDiverged, http.StatusConflict, "git fetch origin " + prHead + " && git merge FETCH_HEAD"},
		"the session branch is not pushed": {&fakeForge{existing: takenOverPR()},
			PullRequestBranchNotPushed, http.StatusConflict, "bex-agent/task-1"},
		"the PR was closed": {&fakeForge{existing: withPR(func(p *github.PullRequestDetail) { p.State = "closed" }), sessionHead: sessionHead},
			PullRequestClosed, http.StatusConflict, "no longer open"},
		"GitHub names another head branch": {&fakeForge{existing: withPR(func(p *github.PullRequestDetail) { p.HeadRef = "other" }), sessionHead: sessionHead},
			PullRequestUpstreamError, http.StatusBadGateway, "different pull request"},
		// A rule on the branch refused the fast-forward: not retryable.
		"the PR branch is protected": {&fakeForge{existing: takenOverPR(), sessionHead: sessionHead, protected: true},
			PullRequestBranchProtected, http.StatusForbidden, "is protected; bex cannot update it"},
		// Re-read at every call: the default branch or another repository's
		// branch is never updated.
		"the head became the default branch": {&fakeForge{existing: withPR(func(p *github.PullRequestDetail) { p.DefaultBranch = prHead }), sessionHead: sessionHead},
			PullRequestBranchProtected, http.StatusForbidden, "default branch or lives in another repository"},
		"the head is in another repository": {&fakeForge{existing: withPR(func(p *github.PullRequestDetail) { p.HeadRepo = "mallory/repo" }), sessionHead: sessionHead},
			PullRequestBranchProtected, http.StatusForbidden, "default branch or lives in another repository"},
		"GitHub fails the read": {&fakeForge{err: &github.APIError{Status: 500, Body: "secret upstream text"}},
			PullRequestUpstreamError, http.StatusBadGateway, "could not read"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sessions := &prSessions{session: takeoverSession()}
			_, err := newPullRequester(tc.forge, sessions, nil).Sync(context.Background(), prRequest("t", "b"))
			wantPRCode(t, err, tc.code, tc.status)
			if !strings.Contains(err.Error(), tc.says) || strings.Contains(err.Error(), "secret upstream text") {
				t.Fatalf("refusal = %v, want it to say %q", err, tc.says)
			}
			if len(tc.forge.moved) != 0 || sessions.recorded != 0 || tc.forge.calls != 0 {
				t.Fatalf("a refused takeover sync moved %v, recorded %d, synced %d", tc.forge.moved, sessions.recorded, tc.forge.calls)
			}
		})
	}
	// The prefix rule is whole for a takeover too: its row's branch must be a
	// bex-agent/ branch, whatever it took over.
	session := takeoverSession()
	session.Branch = prHead
	req := prRequest("t", "b")
	req.Branch = prHead
	forge := &fakeForge{existing: takenOverPR(), sessionHead: sessionHead}
	_, err := newPullRequester(forge, &prSessions{session: session}, nil).Sync(context.Background(), req)
	wantPRCode(t, err, PullRequestForbidden, http.StatusForbidden)
	if len(forge.moved) != 0 || forge.reads != 0 {
		t.Fatal("a non-prefixed session branch reached GitHub")
	}
}

func TestSessionReferenceLinksTheDashboardOrNamesTheID(t *testing.T) {
	if got := SessionReference("https://dashboard.bex.co", "ags-one"); got != "[view session](https://dashboard.bex.co/agents/ags-one)" {
		t.Fatalf("with a dashboard = %q", got)
	}
	if got := SessionReference("", "ags-one"); got != "session `ags-one`" {
		t.Fatalf("without one = %q", got)
	}
}
