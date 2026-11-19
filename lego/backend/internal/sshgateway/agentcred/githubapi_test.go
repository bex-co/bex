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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

// githubAPIUpstream records what reached the fake api.github.com.
type githubAPIUpstream struct {
	calls         int
	authorization string
	userAgent     string
	path          string
	query         string
	body          string
}

// githubAPIHarness is a broker whose github-api route mints through a real
// GitHubReadHandler and forwards to a fake GitHub API answering handler. The
// session is ags-one on octo/repo, its Pod 10.0.0.1.
func githubAPIHarness(t *testing.T, handler http.HandlerFunc) (*Broker, string, *credentialGitHub, *githubAPIUpstream) {
	t.Helper()
	secret := []byte("github-api-secret")
	gh := &credentialGitHub{}
	minter := &agentsession.Minter{GitHub: gh, Connections: credentialConnections{}, Sessions: credentialSessions{}}
	gitMint := httptest.NewServer(&agentsession.Handler{Secret: secret, Minter: minter})
	t.Cleanup(gitMint.Close)
	readMint := httptest.NewServer(&agentsession.GitHubReadHandler{Secret: secret, Minter: minter})
	t.Cleanup(readMint.Close)
	seen := &githubAPIUpstream{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.calls++
		seen.authorization, seen.userAgent = r.Header.Get("Authorization"), r.Header.Get("User-Agent")
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

// Every write is refused, before any mint, with a message naming the bex
// tools: they stay the session's one pull-request path.
func TestGitHubAPIRouteRefusesWritesNamingTheBexTools(t *testing.T) {
	broker, repoURL, gh, seen := githubAPIHarness(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "{}")
	})
	for _, write := range []struct{ method, path, body string }{
		{http.MethodPost, "/repos/octo/repo/pulls", `{"title":"x","head":"bex-agent/task-1","base":"main"}`},
		{http.MethodPost, "/repos/octo/repo/issues/1/comments", `{"body":"x"}`},
		{http.MethodPatch, "/repos/octo/repo/pulls/1", `{"state":"closed"}`},
		{http.MethodPut, "/repos/octo/repo/contents/README.md", `{}`},
		{http.MethodDelete, "/repos/octo/repo/git/refs/heads/main", ""},
		{http.MethodPost, "/repos/octo/repo/actions/runs/1/rerun", ""},
		{http.MethodPost, "/repos/other/repo/pulls", `{}`},
		{http.MethodGet, "/graphql?query=%7Bviewer%7Blogin%7D%7D", ""},
		{http.MethodPost, "/graphql", `{"query":"mutation PullRequestCreate($input: CreatePullRequestInput!) { createPullRequest(input: $input) { pullRequest { url } } }","variables":{"input":{}}}`},
		{http.MethodPost, "/graphql", `{"query":"query Q { repository(owner: \"octo\", name: \"repo\") { id } } mutation M { addComment(input: {}) { clientMutationId } }"}`},
		{http.MethodPost, "/graphql", `{"query":"subscription S { x }"}`},
	} {
		rec := githubAPIRequest(broker, write.method, repoURL+"/github-api"+write.path, write.body)
		if rec.Code != http.StatusForbidden && rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want refused", write.method, write.path, rec.Code)
			continue
		}
		if msg := githubAPIMessage(t, rec); !strings.Contains(msg, "open_or_update_draft_pr") || !strings.Contains(msg, "reply_on_pull_request") {
			t.Errorf("%s %s refusal does not name the bex tools: %q", write.method, write.path, msg)
		}
	}
	if seen.calls != 0 {
		t.Fatalf("a write reached GitHub (upstream %d)", seen.calls)
	}
	// GraphQL is refused after admission, so it is the only refusal that may
	// have charged the session's budget; none minted a credential.
	if gh.apiCalls != 0 {
		t.Fatalf("a refused write minted a credential (%d)", gh.apiCalls)
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
	for _, body := range []string{`not json`, `[{"query":"{ rateLimit { remaining } }"}]`, `{"query":"{ rateLimit { remaining } }","extensions":{}}`, `{"query":"{ unclosed"}`, `{}`} {
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
	broker.MaxGitHubAPIRequestsPerSession, broker.MaxRequestsPerSession = 2, 2
	for i := range 2 {
		if rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/octo/repo", ""); rec.Code != http.StatusOK {
			t.Fatalf("read %d = %d", i, rec.Code)
		}
	}
	rec := githubAPIRequest(broker, http.MethodGet, repoURL+"/github-api/repos/octo/repo", "")
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(githubAPIMessage(t, rec), "2 GitHub API requests") {
		t.Fatalf("over budget = %d %q", rec.Code, rec.Body.String())
	}
	if got := gitBudgetRequest(broker, repoURL); got != http.StatusOK {
		t.Fatalf("a Git exchange after the API budget = %d, want 200", got)
	}
}
