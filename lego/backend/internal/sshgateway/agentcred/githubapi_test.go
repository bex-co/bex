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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// githubAPIUpstream records what reached the fake api.github.com.
type githubAPIUpstream struct {
	calls         int
	method        string
	authorization string
	userAgent     string
	path          string
	query         string
	body          string
}

// githubAPIHarness is a broker whose github-api route mints through real
// GitHubReadHandler and GitHubWriteHandler verbs and forwards to a fake GitHub
// API answering handler. The session is ags-one on octo/repo, its Pod
// 10.0.0.1.
func githubAPIHarness(t *testing.T, handler http.HandlerFunc) (*Broker, string, *credentialGitHub, *githubAPIUpstream) {
	t.Helper()
	return githubAPIHarnessWithSetting(t, &githubCLISetting{}, handler)
}

// githubCLISetting is the harness workspace's GitHub CLI setting (w10/m524),
// which a test turns off as an admin would while the session runs.
type githubCLISetting struct{ off bool }

func (s *githubCLISetting) GetAgentEnvironment(_ context.Context, workspaceID string) (store.AgentEnvironment, error) {
	if !s.off {
		return store.AgentEnvironment{}, store.ErrNotFound
	}
	return store.AgentEnvironment{WorkspaceID: workspaceID, GitHubCLIDisabled: true}, nil
}

func githubAPIHarnessWithSetting(t *testing.T, setting *githubCLISetting, handler http.HandlerFunc) (*Broker, string, *credentialGitHub, *githubAPIUpstream) {
	t.Helper()
	secret := []byte("github-api-secret")
	gh := &credentialGitHub{}
	minter := &agentsession.Minter{GitHub: gh, Connections: credentialConnections{}, Sessions: credentialSessions{}, Environments: setting}
	gitMint := httptest.NewServer(&agentsession.Handler{Secret: secret, Minter: minter})
	t.Cleanup(gitMint.Close)
	readMint := httptest.NewServer(&agentsession.GitHubReadHandler{Secret: secret, Minter: minter})
	t.Cleanup(readMint.Close)
	writeMint := httptest.NewServer(&agentsession.GitHubWriteHandler{Secret: secret, Minter: minter})
	t.Cleanup(writeMint.Close)
	seen := &githubAPIUpstream{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.calls++
		seen.method, seen.authorization, seen.userAgent = r.Method, r.Header.Get("Authorization"), r.Header.Get("User-Agent")
		seen.path, seen.query = r.URL.EscapedPath(), r.URL.RawQuery
		raw, _ := io.ReadAll(r.Body)
		seen.body = string(raw)
		handler(w, r)
	}))
	t.Cleanup(upstream.Close)
	pod := sshgateway.SessionPod{Name: "sbx-a-0", UID: "uid-a", Labels: credentialLabels(t, "tea-a", "ags-one", "bex-agent/task-1")}
	broker := &Broker{
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Pods: credentialPodResolver{pods: map[string]sshgateway.SessionPod{
			"10.0.0.1": pod,
			"10.0.0.2": {Name: "sbx-b-0", UID: "uid-b", Labels: credentialLabels(t, "tea-a", "ags-two", "bex-agent/task-2")},
		}},
		API:             &agentsession.Client{URL: gitMint.URL, Secret: secret, HTTP: gitMint.Client()},
		GitHubRead:      &agentsession.Client{URL: readMint.URL, Secret: secret, HTTP: readMint.Client()},
		GitHubWrite:     &agentsession.Client{URL: writeMint.URL, Secret: secret, HTTP: writeMint.Client()},
		GitHubAPIOrigin: upstream.URL, UpstreamOrigin: upstream.URL, HTTP: upstream.Client(),
	}
	repoURL, err := agentsession.ProxyRepositoryURL("http://gateway", "tea-a-sandbox", "ags-one", "octo/repo", "bex-agent/task-1")
	if err != nil {
		t.Fatal(err)
	}
	return broker, repoURL, gh, seen
}

func githubAPIRequest(broker *Broker, method, target, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req.RemoteAddr = "10.0.0.1:43210"
	req.Header.Set("Authorization", "token bex-gateway-placeholder")
	req.Header.Set("Accept", "application/vnd.github+json")
	rec := httptest.NewRecorder()
	broker.Handler().ServeHTTP(rec, req)
	return rec
}

func assertNoToken(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if strings.Contains(rec.Body.String(), "ghs_") || strings.Contains(fmt.Sprint(rec.Header()), "ghs_") {
		t.Fatalf("a GitHub token reached the sandbox: %v %q", rec.Header(), rec.Body.String())
	}
}

func githubAPIMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Message == "" {
		t.Fatalf("refusal is not GitHub's error shape: %q", rec.Body.String())
	}
	return body.Message
}

// w10/m513: a read of the session's repository reaches GitHub with a
// read-only token the gateway minted and injected, never the sandbox's
// placeholder; GitHub's answer comes back with its pagination rewritten to
// the owner/name form the route serves, and the token never does.
func TestGitHubAPIRouteForwardsAReadWithTheInjectedToken(t *testing.T) {
	broker, repoURL, gh, seen := githubAPIHarness(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `<https://api.github.com/repositories/793852040/actions/runs?per_page=1&page=2>; rel="next", <https://api.github.com/repositories/793852040/actions/runs?page=9>; rel="last"`)
		w.Header().Set("X-OAuth-Scopes", "repo")
		w.Header().Set("Github-Authentication-Token-Expiration", "2026-08-01 13:00:00 UTC")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"total_count":1}`)
	})
	rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/octo/repo/actions/runs?per_page=1", "")
	if rec.Code != http.StatusOK || rec.Body.String() != `{"total_count":1}` {
		t.Fatalf("read = %d %q", rec.Code, rec.Body.String())
	}
	if seen.authorization != "Bearer ghs_gateway_api_read" || seen.path != "/repos/octo/repo/actions/runs" || seen.query != "per_page=1" || seen.userAgent != githubAPIUserAgent {
		t.Fatalf("upstream saw auth=%q path=%q query=%q ua=%q", seen.authorization, seen.path, seen.query, seen.userAgent)
	}
	if gh.apiCalls != 1 || gh.calls != 0 {
		t.Fatalf("mints: read-only %d, write %d", gh.apiCalls, gh.calls)
	}
	want := `<https://api.github.com/repos/octo/repo/actions/runs?per_page=1&page=2>; rel="next", <https://api.github.com/repos/octo/repo/actions/runs?page=9>; rel="last"`
	if got := rec.Header().Get("Link"); got != want {
		t.Fatalf("Link = %q, want %q", got, want)
	}
	if rec.Header().Get("X-OAuth-Scopes") != "" || rec.Header().Get("Github-Authentication-Token-Expiration") != "" {
		t.Fatalf("token metadata reached the sandbox: %v", rec.Header())
	}
	assertNoToken(t, rec)

	// The repository's own root and HEAD reads are reads too; GitHub's
	// status and error body pass through unchanged.
	broker, repoURL, _, seen = githubAPIHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	if rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/Octo/Repo", ""); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "Not Found") || seen.path != "/repos/Octo/Repo" {
		t.Fatalf("repository root = %d %q (upstream %q)", rec.Code, rec.Body.String(), seen.path)
	}
	if rec := githubAPIRequest(broker, http.MethodHead, repoURL+"/github-api/repos/octo/repo/pulls", ""); rec.Code != http.StatusNotFound || seen.calls != 2 {
		t.Fatalf("HEAD = %d (upstream calls %d)", rec.Code, seen.calls)
	}
}

// Another repository, and every other part of the API, is refused before any
// mint, whichever spelling reaches for it.
func TestGitHubAPIRouteRefusesOtherRepositoriesAndPaths(t *testing.T) {
	broker, repoURL, gh, seen := githubAPIHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "{}")
	})
	for _, path := range []string{
		"/repos/other/repo/actions/runs",
		"/repos/octo/repository",
		"/repos/octo",
		"/repos/octo/repo.git",
		"/repositories/793852040/actions/runs",
		"/user",
		"/orgs/octo/repos",
		"/search/code?q=secret+repo:other/repo",
		"/rate_limit",
		"/repos/octo/repo/../../other/repo",
		"/repos/octo/repo/./x",
		"/repos/octo/repo/%2e%2e/%2e%2e/other/repo",
		"/repos/octo%2Frepo/actions/runs",
		"/repos/octo/repo%2F..%2F..%2Fother%2Frepo",
		"//repos/octo/repo",
		"/repos/octo/repo%5c..%5cother",
		"/repos/octo/repo%00",
		"",
		"x/repos/octo/repo",
	} {
		rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api"+path, "")
		if rec.Code != http.StatusForbidden && rec.Code != http.StatusBadRequest {
			t.Errorf("%q = %d, want 403 or 400", path, rec.Code)
		}
	}
	if gh.apiCalls != 0 || seen.calls != 0 {
		t.Fatalf("a refused path reached GitHub (mints %d, upstream %d)", gh.apiCalls, seen.calls)
	}
	rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/other/repo", "")
	if msg := githubAPIMessage(t, rec); !strings.Contains(msg, "reads only octo/repo") {
		t.Fatalf("other-repository refusal = %q", msg)
	}
}

// assertWriteRefusal checks a refusal says what gh can write here and names
// the bex tools that stay the pull-request path.
func assertWriteRefusal(t *testing.T, label string, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("%s = %d %q, want refused", label, rec.Code, rec.Body.String())
		return
	}
	msg := githubAPIMessage(t, rec)
	for _, want := range []string{"open_or_update_draft_pr", "reply_on_pull_request", "re-run or cancel Actions runs", "review without approving", "session branch"} {
		if !strings.Contains(msg, want) {
			t.Errorf("%s refusal lacks %q: %q", label, want, msg)
		}
	}
}

// graphQLBody is a GraphQL request body for query with variables.
func graphQLBody(t *testing.T, query string, variables map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// w10/m522 DoD 3: opening or changing a pull request, approving one, merging,
// dispatching a workflow on another ref, and every code, ref, release, status or settings
// write, on this repository or another, is refused before any mint with a
// message saying what gh can write and naming the bex tools. A GraphQL
// document that mixes an admitted mutation with a refused one is refused
// whole.
func TestGitHubAPIRouteRefusesWritesNamingTheBexTools(t *testing.T) {
	broker, repoURL, gh, seen := githubAPIHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "{}")
	})
	approve := `mutation PullRequestReviewAdd($input:AddPullRequestReviewInput!){addPullRequestReview(input:$input){clientMutationId}}`
	for _, write := range []struct{ method, path, body string }{
		{http.MethodPost, "/repos/octo/repo/pulls", `{"title":"x","head":"bex-agent/task-1","base":"main"}`},
		{http.MethodPatch, "/repos/octo/repo/pulls/1", `{"state":"closed"}`},
		{http.MethodPut, "/repos/octo/repo/pulls/1/merge", `{}`},
		{http.MethodPut, "/repos/octo/repo/pulls/1/update-branch", `{}`},
		{http.MethodPut, "/repos/octo/repo/pulls/1/reviews/2/dismissals", `{"message":"x"}`},
		{http.MethodPost, "/repos/octo/repo/pulls/1/reviews", `{"event":"APPROVE"}`},
		{http.MethodPost, "/repos/octo/repo/pulls/1/reviews", `{"body":"x","event":" approve "}`},
		{http.MethodPost, "/repos/octo/repo/pulls/1/reviews", `{"event":7}`},
		{http.MethodPost, "/repos/octo/repo/pulls/1/reviews/2/events", `{"event":"APPROVE"}`},
		{http.MethodPatch, "/repos/octo/repo/issues/1", `{"title":"x"}`},
		{http.MethodPut, "/repos/octo/repo/issues/1/lock", ""},
		{http.MethodPost, "/repos/octo/repo/labels", `{"name":"x"}`},
		{http.MethodPut, "/repos/octo/repo/contents/README.md", `{}`},
		{http.MethodDelete, "/repos/octo/repo/git/refs/heads/main", ""},
		{http.MethodPost, "/repos/octo/repo/git/refs", `{}`},
		{http.MethodPost, "/repos/octo/repo/releases", `{}`},
		{http.MethodPost, "/repos/octo/repo/statuses/0000000000000000000000000000000000000000", `{"state":"success"}`},
		{http.MethodPost, "/repos/octo/repo/check-runs", `{}`},
		{http.MethodPost, "/repos/octo/repo/actions/workflows/1/dispatches", `{"ref":"main"}`},
		{http.MethodPost, "/repos/octo/repo/actions/runs/1/approve", ""},
		{http.MethodDelete, "/repos/octo/repo/actions/runs/1", ""},
		{http.MethodPost, "/repos/octo/repo/actions/variables", `{}`},
		{http.MethodPost, "/repos/octo/repo/hooks", `{}`},
		{http.MethodPatch, "/repos/octo/repo", `{"private":false}`},
		{http.MethodPost, "/repos/octo/repo/actions/runs/x/rerun", ""},
		{http.MethodPost, "/repos/octo/repo/issues/", `{}`},
		{http.MethodPost, "/repos/other/repo/issues/1/comments", `{"body":"x"}`},
		{http.MethodPost, "/repos/other/repo/actions/runs/1/rerun", ""},
		{http.MethodPost, "/user/repos", `{}`},
		{http.MethodGet, "/graphql?query=%7Bviewer%7Blogin%7D%7D", ""},
		{http.MethodPost, "/graphql", graphQLBody(t, "mutation PullRequestCreate($input: CreatePullRequestInput!) { createPullRequest(input: $input) { pullRequest { url } } }", map[string]any{"input": map[string]any{}})},
		{http.MethodPost, "/graphql", graphQLBody(t, "mutation($input:UpdatePullRequestInput!){updatePullRequest(input: $input){__typename}}", map[string]any{"input": map[string]any{"pullRequestId": "PR_1", "title": "x"}})},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{updatePullRequest(input: {pullRequestId: "PR_1", baseRefName: "main"}){__typename}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{updatePullRequest(input: {pullRequestId: "PR_1", state: CLOSED}){__typename}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, approve, map[string]any{"input": map[string]any{"pullRequestId": "PR_1", "event": "APPROVE"}})},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{addPullRequestReview(input: {pullRequestId: "PR_1", event: APPROVE}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation($e: PullRequestReviewEvent){addPullRequestReview(input: {pullRequestId: "PR_1", event: $e}){clientMutationId}}`, map[string]any{"e": "APPROVE"})},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation($e: PullRequestReviewEvent = APPROVE){submitPullRequestReview(input: {pullRequestReviewId: "PRR_1", event: $e}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{addPullRequestReview(input: {pullRequestId: "PR_1", event: APPROVE, event: COMMENT}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{addPullRequestReview(input: {pullRequestId: "PR_1", event: APPROVE}, input: {pullRequestId: "PR_1"}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{addPullRequestReview(input: {pullRequestId: "PR_1"}, input: {pullRequestId: "PR_1", event: APPROVE}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, approve, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{mergePullRequest(input: {pullRequestId: "PR_1"}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{enablePullRequestAutoMerge(input: {pullRequestId: "PR_1"}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{markPullRequestReadyForReview(input: {pullRequestId: "PR_1"}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{closePullRequest(input: {pullRequestId: "PR_1"}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{dismissPullRequestReview(input: {pullRequestReviewId: "PRR_1", message: "x"}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{resolveReviewThread(input: {threadId: "PRRT_1"}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{createCommitOnBranch(input: {}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{addComment(input: {subjectId: "PR_1", body: "x"}){clientMutationId} mergePullRequest(input: {pullRequestId: "PR_1"}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation A {addComment(input: {subjectId: "PR_1", body: "x"}){clientMutationId}} mutation B {closePullRequest(input: {pullRequestId: "PR_1"}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `fragment f on Mutation { mergePullRequest(input: {pullRequestId: "PR_1"}){clientMutationId} } mutation { ...f }`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation { ... on Mutation { mergePullRequest(input: {pullRequestId: "PR_1"}){clientMutationId} } }`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `subscription S { x }`, nil)},
	} {
		rec := githubAPIRequest(broker, write.method, repoURL+"/github-api"+write.path, write.body)
		assertWriteRefusal(t, write.method+" "+write.path+" "+write.body, rec)
	}
	if seen.calls != 0 {
		t.Fatalf("a refused write reached GitHub (upstream %d)", seen.calls)
	}
	// A GraphQL document and a REST write's body are checked after admission,
	// so they are the only refusals that may have charged the session's
	// budget; none minted a credential.
	if gh.apiCalls != 0 || gh.apiWriteCalls != 0 {
		t.Fatalf("a refused write minted a credential (read %d, write %d)", gh.apiCalls, gh.apiWriteCalls)
	}
}

// w10/m522 DoD 1 and 2: each admitted write of the session's repository
// reaches GitHub as sent, with a write token the gateway minted for that call,
// while a read beside it still carries the read-only token.
func TestGitHubAPIRouteForwardsAdmittedWritesWithTheWriteToken(t *testing.T) {
	broker, repoURL, gh, seen := githubAPIHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":1}`)
	})
	writes := []struct{ method, path, body string }{
		{http.MethodPost, "/repos/octo/repo/issues/7/comments", `{"body":"Re-ran the flaky job."}`},
		{http.MethodPatch, "/repos/Octo/Repo/issues/comments/99", `{"body":"edited"}`},
		{http.MethodDelete, "/repos/octo/repo/issues/comments/99", ""},
		{http.MethodPost, "/repos/octo/repo/pulls/7/comments/123/replies", `{"body":"Fixed in the next commit."}`},
		{http.MethodPost, "/repos/octo/repo/pulls/7/reviews", `{"body":"x","event":"REQUEST_CHANGES"}`},
		{http.MethodPost, "/repos/octo/repo/pulls/7/reviews", `{"body":"pending review"}`},
		{http.MethodPost, "/repos/octo/repo/pulls/7/reviews/5/events", `{"event":"COMMENT"}`},
		{http.MethodPost, "/repos/octo/repo/issues/7/labels", `["flaky"]`},
		{http.MethodDelete, "/repos/octo/repo/issues/7/labels/needs%20triage", ""},
		{http.MethodPost, "/repos/octo/repo/pulls/7/requested_reviewers", `{"reviewers":["octocat"]}`},
		{http.MethodPost, "/repos/octo/repo/issues", `{"title":"Found while fixing","body":"x","labels":["bug"]}`},
		{http.MethodPost, "/repos/octo/repo/actions/runs/44652252248/rerun", `{"enable_debug_logging":true}`},
		{http.MethodPost, "/repos/octo/repo/actions/runs/44652252248/rerun-failed-jobs", ""},
		{http.MethodPost, "/repos/octo/repo/actions/jobs/136009738176/rerun", ""},
		{http.MethodPost, "/repos/octo/repo/actions/runs/44652252248/cancel", ""},
		{http.MethodPost, "/graphql", graphQLBody(t, "mutation CommentCreate($input:AddCommentInput!){addComment(input: $input){commentEdge{node{url}}}}", map[string]any{"input": map[string]any{"subjectId": "PR_1", "body": "x"}})},
		{http.MethodPost, "/graphql", graphQLBody(t, "mutation PullRequestReviewAdd($input:AddPullRequestReviewInput!){addPullRequestReview(input:$input){clientMutationId}}", map[string]any{"input": map[string]any{"pullRequestId": "PR_1", "body": "x", "event": "REQUEST_CHANGES"}})},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{addPullRequestReview(input: {pullRequestId: "PR_1", body: "x", event: COMMENT}){clientMutationId}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, "mutation PullRequestUpdate($input:UpdatePullRequestInput!){updatePullRequest(input: $input){__typename}}", map[string]any{"input": map[string]any{"pullRequestId": "PR_1"}})},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation{updatePullRequest(input: {pullRequestId: "PR_1", labelIds: ["LA_1"], milestoneId: "MI_1"}){__typename}}`, nil)},
		{http.MethodPost, "/graphql", graphQLBody(t, `mutation($t: ID!){addPullRequestReviewThreadReply(input: {pullRequestReviewThreadId: $t, body: "x"}){comment{url}}}`, map[string]any{"t": "PRRT_1"})},
		{http.MethodPost, "/graphql", graphQLBody(t, `query($owner: String!, $name: String!) { repository(owner: $owner, name: $name) { id } } mutation M($input: CreateIssueInput!) { createIssue(input: $input) { issue { url } } }`, map[string]any{"owner": "octo", "name": "repo", "input": map[string]any{"repositoryId": "R_1", "title": "x"}})},
	}
	for i, write := range writes {
		rec := githubAPIRequest(broker, write.method, repoURL+"/github-api"+write.path, write.body)
		if rec.Code != http.StatusCreated || seen.calls != i+1 {
			t.Fatalf("%s %s = %d %q (upstream %d)", write.method, write.path, rec.Code, rec.Body.String(), seen.calls)
		}
		if seen.method != write.method || seen.path != write.path || seen.body != write.body || seen.authorization != "Bearer ghs_gateway_api_write" {
			t.Fatalf("%s %s reached GitHub as %s %s %q with %q", write.method, write.path, seen.method, seen.path, seen.body, seen.authorization)
		}
		assertNoToken(t, rec)
	}
	if gh.apiWriteCalls != len(writes) || gh.apiCalls != 0 || gh.calls != 0 {
		t.Fatalf("mints: write %d, read %d, git %d; want one write token per write", gh.apiWriteCalls, gh.apiCalls, gh.calls)
	}
	// A read, REST or GraphQL, never gets the write token.
	if rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/octo/repo/actions/runs/1", ""); rec.Code != http.StatusCreated || seen.authorization != "Bearer ghs_gateway_api_read" {
		t.Fatalf("REST read = %d with %q", rec.Code, seen.authorization)
	}
	if rec := githubAPIRequest(broker, http.MethodPost, repoURL+"/github-api/graphql", graphQLBody(t, `{ repository(owner: "octo", name: "repo") { id } }`, nil)); rec.Code != http.StatusCreated || seen.authorization != "Bearer ghs_gateway_api_read" {
		t.Fatalf("GraphQL read = %d with %q", rec.Code, seen.authorization)
	}
	if gh.apiCalls != 2 || gh.apiWriteCalls != len(writes) {
		t.Fatalf("after reads: read %d, write %d", gh.apiCalls, gh.apiWriteCalls)
	}
}

// A write's body is checked as GitHub will read it, and its parameters must
// be in that body: a repeated key, invalid JSON, a query string or an
// oversized body is refused before any mint.
func TestGitHubAPIRouteChecksAWriteBodyAsGitHubReadsIt(t *testing.T) {
	broker, repoURL, gh, seen := githubAPIHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "{}")
	})
	for _, bad := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPost, "/repos/octo/repo/pulls/1/reviews", `{"event":"COMMENT","event":"APPROVE"}`, http.StatusBadRequest},
		{http.MethodPost, "/repos/octo/repo/pulls/1/reviews", `{"comments":[{"path":"a","path":"b"}]}`, http.StatusBadRequest},
		{http.MethodPost, "/repos/octo/repo/issues/1/comments", `{"body":`, http.StatusBadRequest},
		{http.MethodPost, "/repos/octo/repo/pulls/1/reviews?event=APPROVE", `{"body":"x"}`, http.StatusBadRequest},
		{http.MethodPost, "/repos/octo/repo/pulls/1/reviews", `["APPROVE"]`, http.StatusForbidden},
		{http.MethodPost, "/graphql", `{"query":"mutation($input:AddPullRequestReviewInput!){addPullRequestReview(input:$input){clientMutationId}}","variables":{"input":{"event":"COMMENT","event":"APPROVE"}}}`, http.StatusBadRequest},
		{http.MethodPost, "/graphql", `{"query":"mutation{addComment(input:{subjectId:\"PR_1\",body:\"x\"}){clientMutationId}}","query":"mutation{mergePullRequest(input:{pullRequestId:\"PR_1\"}){clientMutationId}}"}`, http.StatusBadRequest},
		{http.MethodPost, "/repos/octo/repo/issues/1/comments", `{"body":"` + strings.Repeat("x", maxGitHubAPIRequestBytes) + `"}`, http.StatusRequestEntityTooLarge},
	} {
		if rec := githubAPIRequest(broker, bad.method, repoURL+"/github-api"+bad.path, bad.body); rec.Code != bad.status {
			t.Errorf("%s %s %.80s = %d %q, want %d", bad.method, bad.path, bad.body, rec.Code, rec.Body.String(), bad.status)
		}
	}
	if seen.calls != 0 || gh.apiWriteCalls != 0 || gh.apiCalls != 0 {
		t.Fatalf("a refused body reached GitHub (upstream %d, mints %d/%d)", seen.calls, gh.apiCalls, gh.apiWriteCalls)
	}
	for _, raw := range []string{`{"a":{"b":1},"b":[{"a":1},{"a":2}],"c":{"a":{"a":1}}}`, `[]`, `"x"`, `{}`, ` [1, {"a": null}] `} {
		if !uniqueKeyJSON([]byte(raw)) {
			t.Errorf("uniqueKeyJSON(%s) = false", raw)
		}
	}
	for _, raw := range []string{`{"a":1,"a":2}`, `{"a":1,"\u0061":2}`, `[{"x":{"y":1,"y":1}}]`, `{"a":[1,{"b":1}],"a":0}`, `{"a":1`, `{}{}`, `{} x`, `1 2`, ``, `{"a"}`} {
		if uniqueKeyJSON([]byte(raw)) {
			t.Errorf("uniqueKeyJSON(%s) = true", raw)
		}
	}
}

// w10/033: a workflow dispatch, by id or file name, whose ref is the session
// branch, by name or as refs/heads/<branch>, reaches GitHub as sent with the
// write token, as `gh workflow run <workflow> --ref <branch>` sends it. Any
// other ref, a missing ref, a repeated key or a field GitHub could read
// another way is refused before any mint, naming the command that works and
// the bex tools.
func TestGitHubAPIRouteDispatchesAWorkflowOnlyOnTheSessionBranch(t *testing.T) {
	broker, repoURL, gh, seen := githubAPIHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"workflow_run_id":1}`)
	})
	dispatches := repoURL + "/github-api/repos/octo/repo/actions/workflows/"
	for i, admitted := range []struct{ workflow, body string }{
		{"925645", `{"inputs":{},"ref":"bex-agent/task-1","return_run_details":true}`},
		{"925645", `{"inputs":{"reason":"flake"},"ref":"refs/heads/bex-agent/task-1","return_run_details":true}`},
		{"e2e.yml", `{"ref":"bex-agent/task-1"}`},
		{"e2e.yml", `{"ref":"bex-agent\/task-1","return_run_details":false,"inputs":{"suite":{"nested":[1]}}}`},
	} {
		rec := githubAPIRequest(broker, http.MethodPost, dispatches+admitted.workflow+"/dispatches", admitted.body)
		if rec.Code != http.StatusOK || seen.calls != i+1 || seen.body != admitted.body || seen.authorization != "Bearer ghs_gateway_api_write" ||
			seen.path != "/repos/octo/repo/actions/workflows/"+admitted.workflow+"/dispatches" {
			t.Fatalf("%s = %d %q; GitHub saw %s %q with %q", admitted.body, rec.Code, rec.Body.String(), seen.path, seen.body, seen.authorization)
		}
		assertNoToken(t, rec)
	}
	calls, mints := seen.calls, gh.apiWriteCalls
	for _, body := range []string{
		`{"ref":"main"}`,
		`{"inputs":{},"ref":"trunk","return_run_details":true}`,
		`{"ref":"bex-agent/task-2"}`,
		`{"ref":"refs/tags/bex-agent/task-1"}`,
		`{"ref":"heads/bex-agent/task-1"}`,
		`{"ref":"refs/heads/bex-agent/task-1/x"}`,
		`{"ref":"Bex-Agent/Task-1"}`,
		`{"ref":"bex-agent/task-1 "}`,
		`{"ref":"bex-agent/task-1","ref":"main"}`,
		`{"ref":"main","ref":"bex-agent/task-1"}`,
		`{"ref":"bex-agent/task-1","\u0072ef":"main"}`,
		`{"ref":"bex-agent/task-1","sha":"0000000000000000000000000000000000000000"}`,
		`{"ref":"bex-agent/task-1","inputs":"x"}`,
		`{"ref":"bex-agent/task-1","inputs":null}`,
		`{"ref":"bex-agent/task-1","inputs":{"a":1,"a":2}}`,
		`{"ref":"bex-agent/task-1","return_run_details":"yes"}`,
		`{"ref":["bex-agent/task-1"]}`,
		`{"ref":null}`,
		`{"inputs":{}}`,
		`{}`,
		`[{"ref":"bex-agent/task-1"}]`,
		`{"ref":"bex-agent/task-1"} {"ref":"main"}`,
		`null`,
		``,
	} {
		rec := githubAPIRequest(broker, http.MethodPost, dispatches+"925645/dispatches", body)
		assertWriteRefusal(t, body, rec)
		if msg := githubAPIMessage(t, rec); !strings.Contains(msg, "`gh workflow run <workflow> --ref bex-agent/task-1`") || !strings.Contains(msg, "dispatch a workflow on your session branch") {
			t.Errorf("%s refusal does not name the command that works: %q", body, msg)
		}
	}
	// A query string, another repository or another dispatch-like path is
	// refused before its body is read.
	for _, target := range []string{
		dispatches + "925645/dispatches?ref=main",
		repoURL + "/github-api/repos/other/repo/actions/workflows/925645/dispatches",
		dispatches + "925645/dispatches/1",
		dispatches + "/dispatches",
	} {
		rec := githubAPIRequest(broker, http.MethodPost, target, `{"ref":"bex-agent/task-1"}`)
		if rec.Code != http.StatusForbidden && rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d %q, want refused", target, rec.Code, rec.Body.String())
		}
	}
	if seen.calls != calls || gh.apiWriteCalls != mints || gh.apiCalls != 0 {
		t.Fatalf("a refused dispatch reached GitHub (upstream %d, write mints %d, read mints %d)", seen.calls-calls, gh.apiWriteCalls-mints, gh.apiCalls)
	}
}

// A gateway without the write verb still reads, and answers a write 503
// instead of minting a read token for it.
func TestGitHubAPIRouteWithoutTheWriteVerbStillReads(t *testing.T) {
	broker, repoURL, gh, seen := githubAPIHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "{}")
	})
	broker.GitHubWrite = nil
	if rec := githubAPIRequest(broker, http.MethodPost, repoURL+"/github-api/repos/octo/repo/actions/runs/1/rerun", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("a write without the verb = %d, want 503", rec.Code)
	}
	if rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/octo/repo", ""); rec.Code != http.StatusOK {
		t.Fatalf("a read without the write verb = %d, want 200", rec.Code)
	}
	if seen.calls != 1 || gh.apiCalls != 1 || gh.apiWriteCalls != 0 {
		t.Fatalf("upstream %d, mints read %d write %d", seen.calls, gh.apiCalls, gh.apiWriteCalls)
	}
}

// recordedGhCommand is one command of testdata/gh-2.105.0-requests.json: the
// requests gh 2.105.0 sent for it through http_unix_socket, recorded on
// 2026-11-19 (and its gh workflow run commands on the session branch on
// 2026-11-20, w10/033) against a socket that forwarded reads to GitHub and
// answered every write itself, with the repository renamed octo/repo and the
// session branch bex-agent/task-1, the harness's. want is read
// (admitted, read token), write (admitted, write token) or refused.
type recordedGhCommand struct {
	Command  string `json:"command"`
	Admitted bool   `json:"admitted"`
	Requests []struct {
		Method string `json:"method"`
		Path   string `json:"path"`
		Query  string `json:"query"`
		Body   string `json:"body"`
		Want   string `json:"want"`
	} `json:"requests"`
}

// w10/m522: every request the pinned gh sends for an admitted command reaches
// GitHub with the token its kind needs, and the write of every refused
// command is refused naming the bex tools.
func TestGitHubAPIRouteReplaysTheRecordedGhCommands(t *testing.T) {
	raw, err := os.ReadFile("testdata/gh-2.105.0-requests.json")
	if err != nil {
		t.Fatal(err)
	}
	var commands []recordedGhCommand
	if err := json.Unmarshal(raw, &commands); err != nil {
		t.Fatal(err)
	}
	broker, repoURL, _, seen := githubAPIHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":{}}`)
	})
	admitted, refused := 0, 0
	for _, command := range commands {
		for _, request := range command.Requests {
			target := repoURL + "/github-api" + request.Path
			if request.Query != "" {
				target += "?" + request.Query
			}
			before := seen.calls
			rec := githubAPIRequest(broker, request.Method, target, request.Body)
			label := command.Command + ": " + request.Method + " " + request.Path
			switch request.Want {
			case "read", "write":
				token := "Bearer ghs_gateway_api_" + request.Want
				if rec.Code != http.StatusOK || seen.calls != before+1 || seen.authorization != token {
					t.Errorf("%s = %d %q (upstream %d, %q), want forwarded with the %s token", label, rec.Code, rec.Body.String(), seen.calls-before, seen.authorization, request.Want)
				}
			case "refused":
				if seen.calls != before || (rec.Code != http.StatusForbidden && rec.Code != http.StatusBadRequest) {
					t.Errorf("%s = %d (upstream %d), want refused", label, rec.Code, seen.calls-before)
				}
			default:
				t.Fatalf("%s: unknown want %q", label, request.Want)
			}
		}
		last := command.Requests[len(command.Requests)-1]
		if command.Admitted != (last.Want == "write") {
			t.Fatalf("%s: admitted=%v but its write is %q", command.Command, command.Admitted, last.Want)
		}
		if command.Admitted {
			admitted++
			continue
		}
		refused++
		rec := githubAPIRequest(broker, last.Method, repoURL+"/github-api"+last.Path, last.Body)
		assertWriteRefusal(t, command.Command, rec)
	}
	if admitted < 25 || refused < 25 {
		t.Fatalf("the recording holds %d admitted and %d refused commands, want both sets", admitted, refused)
	}
}

// A GraphQL query reaches GitHub only when its top-level fields read the
// session's repository (literal, variable or variable default), a node by id,
// the rate limit or the schema: the reads gh's pr and run commands send.
func TestGitHubAPIRouteScopesGraphQLToTheRepository(t *testing.T) {
	broker, repoURL, gh, seen := githubAPIHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":{}}`)
	})
	allowed := []string{
		`{"query":"query PullRequestByNumber($owner: String!, $repo: String!, $pr_number: Int!) { repository(owner: $owner, name: $repo) { pullRequest(number: $pr_number) { number } } }","variables":{"owner":"octo","repo":"repo","pr_number":198}}`,
		`{"query":"fragment pr on PullRequest{number} query { repository(owner: \"Octo\", name: \"REPO\") { pullRequests(first: 1) { nodes { ...pr } } } }"}`,
		`{"query":"query($owner: String = \"octo\", $name: String = \"repo\") { repository(owner: $owner, name: $name) { id } rateLimit { remaining } }"}`,
		`{"query":"query PullRequestStatusChecks($id: ID!) { node(id: $id) { ...on PullRequest { id } } }","variables":{"id":"PR_kwDOL1E4iM8AAAABQVmvkg"}}`,
		`{"query":"query PullRequest_fields{PullRequest: __type(name: \"PullRequest\"){fields(includeDeprecated: true){name}}}"}`,
	}
	for i, body := range allowed {
		rec := githubAPIRequest(broker, http.MethodPost, repoURL+"/github-api/graphql", body)
		if rec.Code != http.StatusOK || seen.calls != i+1 || seen.body != body || seen.path != "/graphql" {
			t.Fatalf("allowed query %d = %d %q (upstream %d, body %q)", i, rec.Code, rec.Body.String(), seen.calls, seen.body)
		}
	}
	refused := []string{
		`{"query":"query($owner: String!, $repo: String!) { repository(owner: $owner, name: $repo) { id } }","variables":{"owner":"other","repo":"repo"}}`,
		`{"query":"{ repository(owner: \"octo\", name: \"other\") { id } }"}`,
		`{"query":"{ repository(owner: \"octo\") { id } }"}`,
		`{"query":"{ repository(owner: \"octo\", name: \"repo\", owner: \"other\") { id } }"}`,
		`{"query":"query($o: String!) { repository(owner: $o, name: \"repo\") { id } }","variables":{"o":7}}`,
		`{"query":"query($o: String!) { repository(owner: $o, name: \"repo\") { id } }"}`,
		`{"query":"{ viewer { login } }"}`,
		`{"query":"{ search(query: \"repo:other/repo\", type: REPOSITORY, first: 1) { repositoryCount } }"}`,
		`{"query":"{ organization(login: \"octo\") { repositories(first: 10) { nodes { name } } } }"}`,
		`{"query":"fragment f on Query { viewer { login } } query { ...f }"}`,
		`{"query":"query { ... on Query { viewer { login } } }"}`,
	}
	for _, body := range refused {
		rec := githubAPIRequest(broker, http.MethodPost, repoURL+"/github-api/graphql", body)
		if rec.Code != http.StatusForbidden {
			t.Errorf("query %s = %d, want 403", body, rec.Code)
		}
	}
	for _, body := range []string{`not json`, `[{"query":"{ rateLimit { remaining } }"}]`, `{"query":"{ rateLimit { remaining } }","extensions":{}}`, `{"query":"{ unclosed"}`, `{}`, `{"query":"{ rateLimit { remaining } }","variables":{"a":1,"a":2}}`} {
		if rec := githubAPIRequest(broker, http.MethodPost, repoURL+"/github-api/graphql", body); rec.Code != http.StatusBadRequest {
			t.Errorf("malformed %s = %d, want 400", body, rec.Code)
		}
	}
	if seen.calls != len(allowed) || gh.apiCalls != len(allowed) {
		t.Fatalf("a refused query reached GitHub (upstream %d, mints %d)", seen.calls, gh.apiCalls)
	}
}

// The route is bound to the source Pod like every other gateway verb: another
// session's Pod, or an unknown source, gets nothing; so does a repository
// environment build, which only ever clones.
func TestGitHubAPIRouteRefusesAnotherPodAndBuilds(t *testing.T) {
	broker, repoURL, gh, seen := githubAPIHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "{}")
	})
	for _, source := range []string{"10.0.0.2:1", "10.0.0.3:1"} {
		req := httptest.NewRequest(http.MethodGet, repoURL+"/github-api/repos/octo/repo", nil)
		req.RemoteAddr = source
		rec := httptest.NewRecorder()
		broker.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("from %s = %d, want 403", source, rec.Code)
		}
	}
	buildURL, err := agentsession.ProxyRepositoryURL("http://gateway", "tea-a-sandbox", proxyBuildID, "octo/repo", agentsession.BuildBranch(proxyBuildID))
	if err != nil {
		t.Fatal(err)
	}
	if rec := githubAPIRequest(broker, http.MethodGet, buildURL+"/github-api/repos/octo/repo", ""); rec.Code != http.StatusForbidden {
		t.Errorf("a build's request = %d, want 403", rec.Code)
	}
	if gh.apiCalls != 0 || seen.calls != 0 {
		t.Fatalf("a refused source reached GitHub (mints %d, upstream %d)", gh.apiCalls, seen.calls)
	}
	broker.GitHubRead = nil
	if rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/octo/repo", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("an unwired route = %d, want 503", rec.Code)
	}
}

// GitHub sends log and archive downloads to its storage with a redirect. The
// route follows it itself with a fresh request that carries no token, and
// follows nothing else.
func TestGitHubAPIRouteFollowsStorageRedirectsWithoutTheToken(t *testing.T) {
	var storageAuthorization string
	storageCalls := 0
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		storageCalls++
		storageAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "job log line\n")
	}))
	defer storage.Close()
	elsewhere := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a redirect off the storage hosts was followed")
	}))
	defer elsewhere.Close()
	target := storage.URL + "/actions-results/job-logs.txt?sig=x"
	broker, repoURL, _, _ := githubAPIHarness(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusFound)
	})
	storageURL, _ := url.Parse(storage.URL)
	broker.redirectAllowed = func(location *url.URL) bool { return location.Host == storageURL.Host }

	rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/octo/repo/actions/jobs/1/logs", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "job log line\n" || storageCalls != 1 {
		t.Fatalf("log download = %d %q (storage calls %d)", rec.Code, rec.Body.String(), storageCalls)
	}
	if storageAuthorization != "" {
		t.Fatalf("the token followed the redirect: %q", storageAuthorization)
	}
	assertNoToken(t, rec)

	target = elsewhere.URL + "/steal"
	if rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/octo/repo/actions/jobs/1/logs", ""); rec.Code != http.StatusBadGateway {
		t.Fatalf("a redirect elsewhere = %d, want 502", rec.Code)
	}
	if !broker.storageRedirectDefault(t, "https://productionresultssa7.blob.core.windows.net/actions-results/x") ||
		!broker.storageRedirectDefault(t, "https://codeload.github.com/octo/repo/legacy.zip/main") ||
		!broker.storageRedirectDefault(t, "https://objects.githubusercontent.com/x") ||
		broker.storageRedirectDefault(t, "http://productionresultssa7.blob.core.windows.net/x") ||
		broker.storageRedirectDefault(t, "https://bex-api.bex-system.svc:8091/v1/agent-session-credentials") ||
		broker.storageRedirectDefault(t, "https://github.com.evil.example/x") ||
		broker.storageRedirectDefault(t, "https://user@objects.githubusercontent.com/x") {
		t.Fatal("the default storage-host rule admits the wrong hosts")
	}
}

// storageRedirectDefault applies the production storage-host rule.
func (b *Broker) storageRedirectDefault(t *testing.T, raw string) bool {
	t.Helper()
	location, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return (&Broker{}).storageRedirect(location)
}

// The route has its own per-session budget: once it is spent gh gets 429,
// and the session's Git exchanges, which it needs to deliver, are untouched.
func TestGitHubAPIRouteBudgetIsSeparateFromGit(t *testing.T) {
	broker, repoURL, _, _ := githubAPIHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "0000")
	})
	broker.MaxGitHubAPIRequestsPerSession, broker.MaxRequestsPerSession = 3, 2
	for i := range 2 {
		if rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/octo/repo", ""); rec.Code != http.StatusOK {
			t.Fatalf("read %d = %d", i, rec.Code)
		}
	}
	// w10/m522: a write spends the same budget as a read.
	if rec := githubAPIRequest(broker, http.MethodPost, repoURL+"/github-api/repos/octo/repo/actions/runs/1/rerun", ""); rec.Code != http.StatusOK {
		t.Fatalf("write = %d", rec.Code)
	}
	if rec := githubAPIRequest(broker, http.MethodPost, repoURL+"/github-api/repos/octo/repo/issues/1/comments", `{"body":"x"}`); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a write over budget = %d, want 429", rec.Code)
	}
	rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/octo/repo", "")
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(githubAPIMessage(t, rec), "3 GitHub API requests") {
		t.Fatalf("over budget = %d %q", rec.Code, rec.Body.String())
	}
	if got := gitBudgetRequest(broker, repoURL); got != http.StatusOK {
		t.Fatalf("a Git exchange after the API budget = %d, want 200", got)
	}
}

// w10/m524: once an admin turns gh off, a running session's next call on the
// route, a REST or GraphQL read or an admitted write, answers 403 in GitHub's
// error shape naming the setting and how to turn it back on, with no token
// minted and nothing sent to GitHub; its Git exchanges keep working, and
// turning gh back on serves the next call again.
func TestGitHubAPIRouteRefusesEveryCallWhileGitHubCLIIsOff(t *testing.T) {
	setting := &githubCLISetting{}
	broker, repoURL, gh, seen := githubAPIHarnessWithSetting(t, setting, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "{}")
	})
	registry := prometheus.NewRegistry()
	broker.Metrics = sshgateway.NewMetrics(registry)
	if rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/octo/repo/actions/runs", ""); rec.Code != http.StatusOK || gh.apiCalls != 1 {
		t.Fatalf("a read with gh on = %d (read mints %d)", rec.Code, gh.apiCalls)
	}

	setting.off = true
	calls := []struct{ method, path, body string }{
		{http.MethodGet, "/repos/octo/repo/actions/runs", ""},
		{http.MethodPost, "/graphql", graphQLBody(t, `{ repository(owner: "octo", name: "repo") { pullRequests(first: 1) { totalCount } } }`, nil)},
		{http.MethodPost, "/repos/octo/repo/actions/runs/44652252248/rerun-failed-jobs", ""},
		{http.MethodPost, "/graphql", graphQLBody(t, "mutation CommentCreate($input:AddCommentInput!){addComment(input: $input){commentEdge{node{url}}}}", map[string]any{"input": map[string]any{"subjectId": "PR_1", "body": "x"}})},
	}
	for _, call := range calls {
		rec := githubAPIRequest(broker, call.method, repoURL+"/github-api"+call.path, call.body)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s with gh off = %d %q, want 403", call.method, call.path, rec.Code, rec.Body.String())
		}
		message := githubAPIMessage(t, rec)
		for _, want := range []string{"gh is turned off for this workspace's agent sessions", "githubCliEnabled", "Settings → Agents", "bex agent env set --github-cli on", "session branch", "open_or_update_draft_pr"} {
			if !strings.Contains(message, want) {
				t.Fatalf("%s %s refusal %q does not say %q", call.method, call.path, message, want)
			}
		}
		assertNoToken(t, rec)
	}
	if gh.apiCalls != 1 || gh.apiWriteCalls != 0 || seen.calls != 1 {
		t.Fatalf("with gh off: read mints %d, write mints %d, GitHub calls %d; want nothing past the first read", gh.apiCalls, gh.apiWriteCalls, seen.calls)
	}
	if got := gitBudgetRequest(broker, repoURL); got != http.StatusOK || gh.calls != 1 {
		t.Fatalf("a Git exchange with gh off = %d (Git mints %d), want it served", got, gh.calls)
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_authentications_total", map[string]string{"result": "github_cli_off"}); got != float64(len(calls)) {
		t.Fatalf("github_cli_off refusals counted = %v, want %d", got, len(calls))
	}

	setting.off = false
	if rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/octo/repo/actions/runs", ""); rec.Code != http.StatusOK || gh.apiCalls != 2 {
		t.Fatalf("a read after gh is back on = %d (read mints %d)", rec.Code, gh.apiCalls)
	}
}
