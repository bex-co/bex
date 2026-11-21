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

package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// fakeMergeAPI is GitHub's surface for one repository (octo/repo) as the
// mark-ready and merge calls use it: the token mint, the repository's merge
// settings, its base branch's rules, the merge and the GraphQL mutation.
type fakeMergeAPI struct {
	mu sync.Mutex
	// noWorkflows refuses a mint that asks for workflows (an installation
	// that has not granted Workflows); noContents refuses every mint.
	noWorkflows, noContents bool
	settings                string
	rules                   string
	mergeStatus             int
	mergeBody               string
	graphql                 string
	scopes                  []map[string]any
	merges                  []map[string]any
	mutations               []map[string]any
}

func (f *fakeMergeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	decode := func() map[string]any {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		return body
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/app/installations/42/access_tokens":
		scope := decode()
		f.scopes = append(f.scopes, scope)
		perms, _ := scope["permissions"].(map[string]any)
		if f.noContents || (f.noWorkflows && perms["workflows"] != nil) {
			http.Error(w, `{"message":"The permissions requested are not granted to this installation."}`, http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"token":"ghs_merge","expires_at":"2026-11-20T13:00:00Z"}`)
	case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/repo":
		fmt.Fprint(w, f.settings)
	case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/repo/rules/branches/main":
		if f.rules == "" {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}
		fmt.Fprint(w, f.rules)
	case r.Method == http.MethodPut && r.URL.Path == "/repos/octo/repo/pulls/7/merge":
		f.merges = append(f.merges, decode())
		if f.mergeStatus != 0 {
			w.WriteHeader(f.mergeStatus)
			fmt.Fprint(w, f.mergeBody)
			return
		}
		fmt.Fprint(w, `{"sha":"fedcba9876543210fedcba9876543210fedcba98","merged":true,"message":"Pull Request successfully merged"}`)
	case r.Method == http.MethodPost && r.URL.Path == "/graphql":
		f.mutations = append(f.mutations, decode())
		fmt.Fprint(w, f.graphql)
	default:
		http.Error(w, "unexpected "+r.Method+" "+r.URL.String(), http.StatusTeapot)
	}
}

func (f *fakeMergeAPI) lastPermissions() map[string]any {
	perms, _ := f.scopes[len(f.scopes)-1]["permissions"].(map[string]any)
	return perms
}

const allMethods = `{"full_name":"octo/repo","allow_merge_commit":true,"allow_squash_merge":true,"allow_rebase_merge":true}`

func TestMergePullRequestPinsTheHeadWithTheFirstAllowedMethod(t *testing.T) {
	api := &fakeMergeAPI{settings: `{"allow_merge_commit":false,"allow_squash_merge":true,"allow_rebase_merge":true}`}
	c := pullsClient(t, api)
	got, err := c.MergePullRequest(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != MergeMethodSquash || got.SHA != "fedcba9876543210fedcba9876543210fedcba98" {
		t.Fatalf("merge = %+v, want squash, the first method the repository allows", got)
	}
	if len(api.merges) != 1 || api.merges[0]["sha"] != pullsTestHead || api.merges[0]["merge_method"] != "squash" || len(api.merges[0]) != 2 {
		t.Fatalf("merge body = %+v, want the recorded head pinned and no commit text", api.merges)
	}
	perms := api.lastPermissions()
	repos, _ := api.scopes[0]["repositories"].([]any)
	if perms["contents"] != "write" || perms["pull_requests"] != "write" || perms["workflows"] != "write" || perms["metadata"] != "read" || len(perms) != 4 || len(repos) != 1 || repos[0] != "repo" {
		t.Fatalf("token scope = %+v", api.scopes)
	}

	// A named method and commit text go to GitHub as given.
	api = &fakeMergeAPI{settings: allMethods}
	c = pullsClient(t, api)
	if _, err := c.MergePullRequest(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Method: MergeMethodRebase, CommitTitle: "t", CommitMessage: "m", Base: "main"}); err != nil {
		t.Fatal(err)
	}
	if body := api.merges[0]; body["merge_method"] != "rebase" || body["commit_title"] != "t" || body["commit_message"] != "m" {
		t.Fatalf("named merge body = %+v", body)
	}
}

// The base branch's rules narrow the default: a ruleset's allowed merge
// methods and linear history drop what they refuse, and a named method they
// refuse never reaches the merge.
func TestMergePullRequestHonorsTheBaseBranchRules(t *testing.T) {
	api := &fakeMergeAPI{settings: allMethods, rules: `[{"type":"required_linear_history"},{"type":"pull_request","parameters":{"required_approving_review_count":0,"allowed_merge_methods":["merge","rebase"]}}]`}
	c := pullsClient(t, api)
	got, err := c.MergePullRequest(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"})
	if err != nil || got.Method != MergeMethodRebase {
		t.Fatalf("merge = %+v err=%v, want rebase, the only method both rules allow", got, err)
	}
	api.merges = nil
	_, err = c.MergePullRequest(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Method: MergeMethodSquash, Base: "main"})
	var methodErr *MergeMethodError
	if !errors.As(err, &methodErr) || methodErr.Method != "squash" || strings.Join(methodErr.Allowed, ",") != "rebase" || len(api.merges) != 0 {
		t.Fatalf("refused method err=%v merges=%d", err, len(api.merges))
	}
	// Settings that allow nothing name no method to fall back on.
	api = &fakeMergeAPI{settings: `{"allow_merge_commit":false,"allow_squash_merge":false,"allow_rebase_merge":false}`}
	_, err = pullsClient(t, api).MergePullRequest(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"})
	if !errors.As(err, &methodErr) || methodErr.Method != "" || len(api.merges) != 0 {
		t.Fatalf("no allowed method err=%v", err)
	}
	// Settings GitHub left out, and rules it would not show, leave GitHub to decide.
	api = &fakeMergeAPI{settings: `{"full_name":"octo/repo"}`}
	if got, err := pullsClient(t, api).MergePullRequest(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"}); err != nil || got.Method != MergeMethodMerge {
		t.Fatalf("unknown settings merge = %+v err=%v", got, err)
	}
}

// An installation that has not granted Workflows still merges, with a token
// without it; one that has not granted Contents write is not permitted.
func TestMergePullRequestTokenFallsBackWithoutWorkflows(t *testing.T) {
	api := &fakeMergeAPI{noWorkflows: true, settings: allMethods}
	c := pullsClient(t, api)
	if _, err := c.MergePullRequest(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"}); err != nil {
		t.Fatal(err)
	}
	if len(api.scopes) != 2 || api.lastPermissions()["workflows"] != nil || api.lastPermissions()["contents"] != "write" || api.lastPermissions()["pull_requests"] != "write" {
		t.Fatalf("mints = %+v, want the workflows set then the set without it", api.scopes)
	}
	api = &fakeMergeAPI{noContents: true, settings: allMethods}
	_, err := pullsClient(t, api).MergePullRequest(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"})
	if !errors.Is(err, ErrPermissionNotGranted) || len(api.merges) != 0 {
		t.Fatalf("no contents err=%v merges=%d", err, len(api.merges))
	}
}

// GitHub's refusal comes back with its status, and APIErrorMessage reads
// GitHub's own words from it, not Error().
func TestMergePullRequestRefusalCarriesGitHubsMessage(t *testing.T) {
	for _, tc := range []struct {
		status  int
		body    string
		message string
	}{
		{http.StatusMethodNotAllowed, `{"message":"Required status check \"test\" is expected.","documentation_url":"https://docs.github.com/rest"}`, `Required status check "test" is expected.`},
		{http.StatusConflict, `{"message":"Head branch was modified. Review and try the merge again."}`, "Head branch was modified. Review and try the merge again."},
		{http.StatusUnprocessableEntity, `{"message":"Validation Failed","errors":[{"resource":"PullRequest","code":"custom","message":"sha is invalid"}, "plain"]}`, "Validation Failed sha is invalid plain"},
		{http.StatusBadGateway, `<html>bad gateway</html>`, ""},
	} {
		api := &fakeMergeAPI{settings: allMethods, mergeStatus: tc.status, mergeBody: tc.body}
		_, err := pullsClient(t, api).MergePullRequest(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"})
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status != tc.status {
			t.Fatalf("%d: err = %v", tc.status, err)
		}
		if got := APIErrorMessage(err); got != tc.message {
			t.Fatalf("%d: message = %q, want %q", tc.status, got, tc.message)
		}
		if strings.Contains(err.Error(), "test") || strings.Contains(err.Error(), "html") {
			t.Fatalf("Error() relays GitHub's text: %q", err.Error())
		}
	}
	unmerged := &fakeMergeAPI{settings: allMethods, mergeStatus: http.StatusOK, mergeBody: `{"merged":false,"message":"?"}`}
	if _, err := pullsClient(t, unmerged).MergePullRequest(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"}); err == nil {
		t.Fatal("a 200 that did not merge read as merged")
	}
	long := &APIError{Status: 405, Body: `{"message":"` + strings.Repeat("é", 2*maxAPIErrorMessageRunes) + `"}`}
	if got := []rune(APIErrorMessage(long)); len(got) != maxAPIErrorMessageRunes+1 {
		t.Fatalf("long message kept %d runes", len(got))
	}
	if APIErrorMessage(errors.New("plain")) != "" {
		t.Fatal("a non-GitHub error has a message")
	}
}

func TestMarkPullRequestReadySendsTheMutationForTheNode(t *testing.T) {
	api := &fakeMergeAPI{graphql: `{"data":{"markPullRequestReadyForReview":{"pullRequest":{"isDraft":false}}}}`}
	c := pullsClient(t, api)
	if err := c.MarkPullRequestReady(context.Background(), 42, "repo", "PR_kwDOnode7"); err != nil {
		t.Fatal(err)
	}
	if len(api.mutations) != 1 {
		t.Fatalf("mutations = %+v", api.mutations)
	}
	query, _ := api.mutations[0]["query"].(string)
	variables, _ := api.mutations[0]["variables"].(map[string]any)
	if !strings.Contains(query, "markPullRequestReadyForReview(input: {pullRequestId: $id})") || variables["id"] != "PR_kwDOnode7" {
		t.Fatalf("mutation = %+v", api.mutations[0])
	}
	if perms := api.lastPermissions(); perms["pull_requests"] != "write" || perms["contents"] != "write" || len(perms) != 3 {
		t.Fatalf("token scope = %+v", api.scopes)
	}

	for _, tc := range []struct {
		graphql string
		status  int
		message string
	}{
		{`{"data":{"markPullRequestReadyForReview":null},"errors":[{"type":"FORBIDDEN","message":"Resource not accessible by integration"}]}`, http.StatusForbidden, "Resource not accessible by integration"},
		{`{"errors":[{"type":"NOT_FOUND","message":"Could not resolve to a node"}]}`, http.StatusNotFound, "Could not resolve to a node"},
		{`{"errors":[{"type":"UNPROCESSABLE","message":"Pull request is closed"}]}`, http.StatusUnprocessableEntity, "Pull request is closed"},
	} {
		api := &fakeMergeAPI{graphql: tc.graphql}
		err := pullsClient(t, api).MarkPullRequestReady(context.Background(), 42, "repo", "PR_kwDOnode7")
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status != tc.status || APIErrorMessage(err) != tc.message {
			t.Fatalf("%s: err = %v (%q)", tc.graphql, err, APIErrorMessage(err))
		}
	}
	// GitHub answering without undrafting is a failure, not a success.
	still := &fakeMergeAPI{graphql: `{"data":{"markPullRequestReadyForReview":{"pullRequest":{"isDraft":true}}}}`}
	if err := pullsClient(t, still).MarkPullRequestReady(context.Background(), 42, "repo", "PR_kwDOnode7"); err == nil {
		t.Fatal("a pull request still a draft read as marked ready")
	}
	err := pullsClient(t, &fakeMergeAPI{noContents: true}).MarkPullRequestReady(context.Background(), 42, "repo", "PR_kwDOnode7")
	if !errors.Is(err, ErrPermissionNotGranted) {
		t.Fatalf("no contents err = %v", err)
	}
}
