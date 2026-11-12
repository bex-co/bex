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

package agentcred

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// commentSessions is credentialSessions after ags-one opened draft PR #prNumber
// (0: not yet).
type commentSessions struct{ prNumber int }

func (s commentSessions) GetAgentSession(ctx context.Context, id string) (store.AgentSession, error) {
	session, err := credentialSessions{}.GetAgentSession(ctx, id)
	if s.prNumber > 0 {
		session.PRNumber, session.PRURL = s.prNumber, "https://github.com/octo/repo/pull/7"
	}
	return session, err
}

func (commentSessions) RecordAgentSessionPullRequest(context.Context, string, string, int, string) error {
	return nil
}

// commentHarness is prHarness with the comment verb wired as well, over the
// same signed hop and the same Pod table.
func commentHarness(t *testing.T, prNumber int) (*Broker, *prForge, *credentialAudit) {
	t.Helper()
	secret := []byte("comment-secret")
	forge := &prForge{}
	pullRequests := &agentsession.PullRequester{GitHub: forge, Connections: credentialConnections{}, Sessions: commentSessions{prNumber: prNumber}}
	mux := http.NewServeMux()
	mux.Handle(agentsession.InternalPullRequestPath, &agentsession.PullRequestHandler{Secret: secret, PullRequests: pullRequests})
	mux.Handle(agentsession.InternalPullRequestCommentPath, &agentsession.PullRequestCommentHandler{
		Secret: secret, PullRequests: pullRequests, Nonce: &onceNonces{seen: map[string]bool{}},
	})
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)
	broker, _, audit := prHarness(t)
	broker.PullRequests = &agentsession.PullRequestClient{URL: api.URL + agentsession.InternalPullRequestPath, Secret: secret, HTTP: api.Client()}
	broker.PullRequestComments = &agentsession.PullRequestCommentClient{URL: api.URL + agentsession.InternalPullRequestCommentPath, Secret: secret, HTTP: api.Client()}
	broker.Metrics = sshgateway.NewMetrics(prometheus.NewRegistry())
	return broker, forge, audit
}

func commentURL(t *testing.T, namespace, session, branch string) string {
	t.Helper()
	repoURL, err := agentsession.ProxyRepositoryURL("http://gateway.internal:8082", namespace, session, "octo/repo", branch)
	if err != nil {
		t.Fatal(err)
	}
	return repoURL + "/" + agentsession.PullRequestCommentProxySegment
}

const okComment = `{"body":"Fixed the nil check; see the new commit."}`

func TestPullRequestCommentRouteCommentsOnTheSessionsPROnly(t *testing.T) {
	broker, forge, audit := commentHarness(t, 7)
	reply := postPR(t, broker, commentURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1"), "10.0.0.1", "application/json; charset=utf-8", okComment)
	if reply.code != http.StatusOK {
		t.Fatalf("status=%d body=%s", reply.code, reply.raw)
	}
	var got agentsession.PullRequestCommentResponse
	if err := json.Unmarshal([]byte(reply.raw), &got); err != nil || got.Number != 7 || got.URL != "https://github.com/octo/repo/pull/7#issuecomment-1" {
		t.Fatalf("response=%+v err=%v", got, err)
	}
	if len(forge.comments) != 1 || forge.comments[0] != 7 || len(forge.heads) != 0 {
		t.Fatalf("GitHub comments = %v heads = %v, want one comment on PR 7 and no PR sync", forge.comments, forge.heads)
	}
	if len(audit.events) != 1 || audit.events[0].Verb != agentsession.AuditVerbPullRequestComment || audit.events[0].Caller != "sbx-a-0" ||
		audit.events[0].Target != "agent-session:ags-one" || audit.events[0].TargetName != "octo/repo" {
		t.Fatalf("gateway audit = %+v", audit.events)
	}
}

func TestPullRequestCommentRouteRefusesForeignPodsAndBranches(t *testing.T) {
	cases := map[string]struct{ url, ip string }{
		"unresolved source pod":         {commentURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1"), "10.0.0.9"},
		"sibling session's pod":         {commentURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1"), "10.0.0.2"},
		"pod bound to another branch":   {commentURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1"), "10.0.0.3"},
		"other workspace's pod":         {commentURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1"), "10.0.0.4"},
		"URL claims another session":    {commentURL(t, "tea-a-sandbox", "ags-two", "bex-agent/task-1"), "10.0.0.1"},
		"labels match, record does not": {commentURL(t, "tea-a-sandbox", "ags-one", "bex-agent/other"), "10.0.0.3"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			broker, forge, _ := commentHarness(t, 7)
			reply := postPR(t, broker, tc.url, tc.ip, "application/json", okComment)
			if reply.code != http.StatusForbidden || reply.error.Code != agentsession.PullRequestForbidden {
				t.Fatalf("status=%d body=%s, want 403 forbidden", reply.code, reply.raw)
			}
			if forge.calls != 0 {
				t.Fatal("a refused request reached GitHub")
			}
			if !strings.Contains(reply.error.Hint, "comment on its pull request") {
				t.Fatalf("hint = %q", reply.error.Hint)
			}
		})
	}
}

// The sandbox says only what to write: a request naming the PR, the repository
// or anything else is refused before bex-api, as is a malformed body.
func TestPullRequestCommentRouteValidatesTheBody(t *testing.T) {
	url := commentURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1")
	cases := []struct {
		name, contentType, body string
		status                  int
		code                    string
	}{
		{"another PR number", "application/json", `{"body":"b","number":8}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"another repository", "application/json", `{"body":"b","repository":"octo/other"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"empty body", "application/json", `{"body":""}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"blank body", "application/json", `{"body":" \n "}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"no body", "application/json", `{}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"not json", "application/json", `body=b`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"wrong content type", "text/plain", okComment, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"body too long", "application/json", `{"body":"` + strings.Repeat("b", agentsession.MaxPullRequestCommentBytes+1) + `"}`, http.StatusRequestEntityTooLarge, agentsession.PullRequestTooLarge},
		{"request over 64 KiB", "application/json", `{"body":"` + strings.Repeat("b", agentsession.MaxPullRequestRequestBytes) + `"}`, http.StatusRequestEntityTooLarge, agentsession.PullRequestTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broker, forge, _ := commentHarness(t, 7)
			reply := postPR(t, broker, url, "10.0.0.1", tc.contentType, tc.body)
			if reply.code != tc.status || reply.error.Code != tc.code {
				t.Fatalf("status=%d code=%q, want %d %q (body %.200s)", reply.code, reply.error.Code, tc.status, tc.code, reply.raw)
			}
			if forge.calls != 0 {
				t.Fatal("an invalid request reached GitHub")
			}
		})
	}
}

func TestPullRequestCommentRouteRelaysBexAPIRefusals(t *testing.T) {
	url := commentURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1")
	broker, forge, audit := commentHarness(t, 0)
	reply := postPR(t, broker, url, "10.0.0.1", "application/json", okComment)
	if reply.code != http.StatusConflict || reply.error.Code != agentsession.PullRequestNoPullRequest || forge.calls != 0 {
		t.Fatalf("no PR yet: status=%d body=%s", reply.code, reply.raw)
	}
	if !strings.Contains(reply.error.Hint, "open the draft pull request first") {
		t.Fatalf("hint = %q", reply.error.Hint)
	}
	if len(audit.events) != 1 || audit.events[0].Verb != agentsession.AuditVerbPullRequestComment || audit.events[0].Outcome != core.AuditDenied {
		t.Fatalf("gateway audit = %+v", audit.events)
	}

	broker, _, _ = commentHarness(t, 7)
	broker.PullRequestComments = nil
	reply = postPR(t, broker, url, "10.0.0.1", "application/json", okComment)
	if reply.code != http.StatusServiceUnavailable || reply.error.Code != agentsession.PullRequestUnavailable {
		t.Fatalf("unwired: status=%d body=%s", reply.code, reply.raw)
	}
}

// The comment route charges the same per-session budget as Git and the PR verb.
func TestPullRequestCommentRouteSharesTheBudget(t *testing.T) {
	broker, forge, _ := commentHarness(t, 7)
	broker.MaxRequestsPerSession = 1
	if reply := postPR(t, broker, prURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1"), "10.0.0.1", "application/json", okBody); reply.code != http.StatusOK {
		t.Fatalf("PR request status=%d body=%s", reply.code, reply.raw)
	}
	reply := postPR(t, broker, commentURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1"), "10.0.0.1", "application/json", okComment)
	if reply.code != http.StatusTooManyRequests || reply.error.Code != agentsession.PullRequestRateLimited || len(forge.comments) != 0 {
		t.Fatalf("budget: status=%d body=%s", reply.code, reply.raw)
	}
}
