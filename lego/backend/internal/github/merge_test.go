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
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
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
	// answers is GitHub's GraphQL answer by the field a document calls
	// (graphQLFields), graphql the answer to any other.
	answers       map[string]string
	settingsReads int
	scopes        []map[string]any
	merges        []map[string]any
	mutations     []map[string]any
	// asyncStatus and asyncBody are GitHub's answer to an asynchronous merge
	// (w10/053), asyncMerges its bodies; asyncPolls are the answers to the
	// reads of its result, in order, the last one repeating ("" a 403), and
	// polled the result paths read.
	asyncStatus int
	asyncBody   string
	asyncMerges []map[string]any
	asyncPolls  []string
	polled      []string
	// onPoll, when set, runs at each read of the result.
	onPoll func()
}

// graphQLFields are the top-level fields the merge calls send, in the order
// fakeMergeAPI looks for them.
var graphQLFields = []string{"node(id", "enablePullRequestAutoMerge", "enqueuePullRequest", "disablePullRequestAutoMerge", "dequeuePullRequest"}

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
		f.settingsReads++
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
	case r.Method == http.MethodPut && r.URL.Path == "/repos/octo/repo/pulls/7/merge-async":
		f.asyncMerges = append(f.asyncMerges, decode())
		w.WriteHeader(f.asyncStatus)
		fmt.Fprint(w, f.asyncBody)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/repos/octo/repo/pulls/7/merge-async/"):
		f.polled = append(f.polled, r.URL.Path)
		if f.onPoll != nil {
			f.onPoll()
		}
		answer := f.asyncPolls[min(len(f.polled), len(f.asyncPolls))-1]
		if answer == "" {
			http.Error(w, `{"message":"Resource not accessible by integration"}`, http.StatusForbidden)
			return
		}
		fmt.Fprint(w, answer)
	case r.Method == http.MethodPost && r.URL.Path == "/graphql":
		body := decode()
		f.mutations = append(f.mutations, body)
		query, _ := body["query"].(string)
		answer := f.graphql
		for _, field := range graphQLFields {
			if strings.Contains(query, field) {
				if found, ok := f.answers[field]; ok {
					answer = found
				}
				break
			}
		}
		fmt.Fprint(w, answer)
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

// mutationInput is the input variable of the index-th GraphQL document sent.
func (f *fakeMergeAPI) mutationInput(t *testing.T, index int) (string, map[string]any) {
	t.Helper()
	if len(f.mutations) <= index {
		t.Fatalf("GraphQL documents = %+v, want at least %d", f.mutations, index+1)
	}
	query, _ := f.mutations[index]["query"].(string)
	variables, _ := f.mutations[index]["variables"].(map[string]any)
	input, _ := variables["input"].(map[string]any)
	return query, input
}

const armed = `{"data":{"enablePullRequestAutoMerge":{"clientMutationId":null}}}`

// graphQLRefusal is GitHub's GraphQL answer refusing a mutation.
func graphQLRefusal(field, kind, message string) string {
	raw, _ := json.Marshal(map[string]any{"data": map[string]any{field: nil}, "errors": []map[string]string{{"type": kind, "message": message}}})
	return string(raw)
}

// w10/m577: auto-merge is armed with gh pr merge --auto's mutation, pinned to
// the recorded head (expectedHeadOid, which gh leaves out), with the merge
// method rule and the commit text, and the merge's token.
func TestEnablePullRequestAutoMergePinsTheHeadWithTheMethodRule(t *testing.T) {
	api := &fakeMergeAPI{settings: `{"allow_merge_commit":false,"allow_squash_merge":true,"allow_rebase_merge":true,"allow_auto_merge":true}`, answers: map[string]string{"enablePullRequestAutoMerge": armed}}
	c := pullsClient(t, api)
	method, err := c.EnablePullRequestAutoMerge(context.Background(), 42, "octo", "repo", "PR_kwDOnode7", MergeOptions{SHA: pullsTestHead, CommitTitle: "Add retries (#7)", CommitMessage: "Retries.", Base: "main"})
	if err != nil || method != MergeMethodSquash {
		t.Fatalf("arm = %q err=%v, want squash, the first method the repository allows", method, err)
	}
	query, input := api.mutationInput(t, 0)
	if !strings.Contains(query, "enablePullRequestAutoMerge(input: $input)") || len(api.mutations) != 1 {
		t.Fatalf("documents = %+v", api.mutations)
	}
	want := map[string]any{"pullRequestId": "PR_kwDOnode7", "expectedHeadOid": pullsTestHead, "mergeMethod": "SQUASH", "commitHeadline": "Add retries (#7)", "commitBody": "Retries."}
	if fmt.Sprint(input) != fmt.Sprint(want) {
		t.Fatalf("input = %v, want %v", input, want)
	}
	if len(api.merges) != 0 {
		t.Fatal("arming auto-merge merged the pull request")
	}
	if perms := api.lastPermissions(); perms["contents"] != "write" || perms["pull_requests"] != "write" || perms["workflows"] != "write" || len(perms) != 4 {
		t.Fatalf("token scope = %+v, want the merge's", api.scopes)
	}
	// A named method goes to GitHub's enum; no commit text sends none.
	api = &fakeMergeAPI{settings: allMethods, answers: map[string]string{"enablePullRequestAutoMerge": armed}}
	if method, err := pullsClient(t, api).EnablePullRequestAutoMerge(context.Background(), 42, "octo", "repo", "PR_kwDOnode7", MergeOptions{SHA: pullsTestHead, Method: MergeMethodRebase, Base: "main"}); err != nil || method != MergeMethodRebase {
		t.Fatalf("named arm = %q err=%v", method, err)
	}
	if _, input := api.mutationInput(t, 0); len(input) != 3 || input["mergeMethod"] != "REBASE" || input["expectedHeadOid"] != pullsTestHead {
		t.Fatalf("named input = %v", input)
	}
	// w10/048: a stacked pull request arms its stack (includeStack), under
	// the method rule of Base, the stack's base; without IncludeStack (above)
	// the input has no such field.
	api = &fakeMergeAPI{settings: allMethods, rules: `[{"type":"required_linear_history"}]`, answers: map[string]string{"enablePullRequestAutoMerge": armed}}
	if method, err := pullsClient(t, api).EnablePullRequestAutoMerge(context.Background(), 42, "octo", "repo", "PR_kwDOnode7", MergeOptions{SHA: pullsTestHead, Base: "main", IncludeStack: true}); err != nil || method != MergeMethodSquash {
		t.Fatalf("stack arm = %q err=%v, want squash: main's linear history refuses a merge commit", method, err)
	}
	if _, input := api.mutationInput(t, 0); len(input) != 4 || input["includeStack"] != true || input["expectedHeadOid"] != pullsTestHead || input["mergeMethod"] != "SQUASH" {
		t.Fatalf("stack input = %v", input)
	}
	// The method rule refuses before any mutation, as for a merge.
	api = &fakeMergeAPI{settings: `{"allow_merge_commit":true,"allow_squash_merge":false,"allow_rebase_merge":false}`}
	_, err = pullsClient(t, api).EnablePullRequestAutoMerge(context.Background(), 42, "octo", "repo", "PR_kwDOnode7", MergeOptions{SHA: pullsTestHead, Method: MergeMethodSquash, Base: "main"})
	var methodErr *MergeMethodError
	if !errors.As(err, &methodErr) || len(api.mutations) != 0 {
		t.Fatalf("refused method err=%v documents=%d", err, len(api.mutations))
	}
}

// A repository whose "Allow auto-merge" is off is refused before the
// mutation; GitHub's own words for it and for a pull request it would merge
// now are named, and every other refusal keeps its status and message.
func TestEnablePullRequestAutoMergeNamesGitHubsRefusals(t *testing.T) {
	api := &fakeMergeAPI{settings: `{"allow_merge_commit":true,"allow_auto_merge":false}`}
	_, err := pullsClient(t, api).EnablePullRequestAutoMerge(context.Background(), 42, "octo", "repo", "PR_kwDOnode7", MergeOptions{SHA: pullsTestHead, Base: "main"})
	if !errors.Is(err, ErrAutoMergeNotAllowed) || len(api.mutations) != 0 {
		t.Fatalf("allow_auto_merge false: err=%v documents=%d", err, len(api.mutations))
	}
	for _, tc := range []struct {
		answer  string
		want    error
		status  int
		message string
	}{
		{graphQLRefusal("enablePullRequestAutoMerge", "UNPROCESSABLE", "Auto merge is not allowed for this repository"), ErrAutoMergeNotAllowed, http.StatusUnprocessableEntity, "Auto merge is not allowed for this repository"},
		{graphQLRefusal("enablePullRequestAutoMerge", "UNPROCESSABLE", "Pull request Pull request is in clean status"), ErrMergeableNow, http.StatusUnprocessableEntity, "Pull request Pull request is in clean status"},
		{graphQLRefusal("enablePullRequestAutoMerge", "UNPROCESSABLE", "Pull request Pull request is in unstable status"), ErrMergeableNow, http.StatusUnprocessableEntity, "Pull request Pull request is in unstable status"},
		{graphQLRefusal("enablePullRequestAutoMerge", "FORBIDDEN", "Resource not accessible by integration"), nil, http.StatusForbidden, "Resource not accessible by integration"},
		{graphQLRefusal("enablePullRequestAutoMerge", "UNPROCESSABLE", "Pull request is in dirty status"), nil, http.StatusUnprocessableEntity, "Pull request is in dirty status"},
		{graphQLRefusal("enablePullRequestAutoMerge", "RATE_LIMITED", "API rate limit exceeded"), nil, http.StatusForbidden, "API rate limit exceeded"},
	} {
		api := &fakeMergeAPI{settings: allMethods, answers: map[string]string{"enablePullRequestAutoMerge": tc.answer}}
		_, err := pullsClient(t, api).EnablePullRequestAutoMerge(context.Background(), 42, "octo", "repo", "PR_kwDOnode7", MergeOptions{SHA: pullsTestHead, Base: "main"})
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status != tc.status || APIErrorMessage(err) != tc.message {
			t.Fatalf("%s: err = %v (%q)", tc.message, err, APIErrorMessage(err))
		}
		if tc.want != nil && !errors.Is(err, tc.want) {
			t.Fatalf("%s: err = %v, want %v", tc.message, err, tc.want)
		}
		if tc.want == nil && (errors.Is(err, ErrAutoMergeNotAllowed) || errors.Is(err, ErrMergeableNow)) {
			t.Fatalf("%s: err = %v is named as a refusal it is not", tc.message, err)
		}
		if apiErr.RateLimited != strings.Contains(tc.answer, "RATE_LIMITED") {
			t.Fatalf("%s: rate limited = %v", tc.message, apiErr.RateLimited)
		}
	}
}

// w10/m577: a merge-queue branch takes the pull request through gh's queue
// call, pinned to the head, with no method or commit text (the queue sets
// them) and no read of the repository's merge settings; where GitHub will not
// arm it, it is enqueued directly with the same pin.
func TestEnqueuePullRequestPinsTheHeadAndLeavesTheMethodToTheQueue(t *testing.T) {
	api := &fakeMergeAPI{settings: allMethods, answers: map[string]string{"enablePullRequestAutoMerge": armed}}
	if err := pullsClient(t, api).EnqueuePullRequest(context.Background(), 42, "repo", "PR_kwDOnode7", pullsTestHead); err != nil {
		t.Fatal(err)
	}
	query, input := api.mutationInput(t, 0)
	if !strings.Contains(query, "enablePullRequestAutoMerge(input: $input)") || len(api.mutations) != 1 || len(input) != 2 ||
		input["pullRequestId"] != "PR_kwDOnode7" || input["expectedHeadOid"] != pullsTestHead || api.settingsReads != 0 {
		t.Fatalf("queue documents = %+v, settings reads %d", api.mutations, api.settingsReads)
	}
	if perms := api.lastPermissions(); perms["contents"] != "write" || perms["workflows"] != "write" {
		t.Fatalf("token scope = %+v, want the merge's", api.scopes)
	}
	for _, refusal := range []string{"Auto merge is not allowed for this repository", "Pull request Pull request is in clean status"} {
		api := &fakeMergeAPI{answers: map[string]string{
			"enablePullRequestAutoMerge": graphQLRefusal("enablePullRequestAutoMerge", "UNPROCESSABLE", refusal),
			"enqueuePullRequest":         `{"data":{"enqueuePullRequest":{"clientMutationId":null}}}`,
		}}
		if err := pullsClient(t, api).EnqueuePullRequest(context.Background(), 42, "repo", "PR_kwDOnode7", pullsTestHead); err != nil {
			t.Fatalf("%s: %v", refusal, err)
		}
		query, input := api.mutationInput(t, 1)
		if !strings.Contains(query, "enqueuePullRequest(input: $input)") || len(input) != 2 || input["expectedHeadOid"] != pullsTestHead {
			t.Fatalf("%s: documents = %+v", refusal, api.mutations)
		}
	}
	// Any other refusal is GitHub's answer; nothing else is tried.
	api = &fakeMergeAPI{answers: map[string]string{"enablePullRequestAutoMerge": graphQLRefusal("enablePullRequestAutoMerge", "UNPROCESSABLE", "Pull request is in dirty status")}}
	err := pullsClient(t, api).EnqueuePullRequest(context.Background(), 42, "repo", "PR_kwDOnode7", pullsTestHead)
	if APIErrorMessage(err) != "Pull request is in dirty status" || len(api.mutations) != 1 {
		t.Fatalf("dirty: err=%v documents=%d", err, len(api.mutations))
	}
}

// w10/m577: the undo disables auto-merge, or takes a queued pull request out
// of the queue, with the mark-ready token.
//
// w10/048: the stack's auto-merge request covering a stacked pull request is
// disarmed with includeStack, for every pull request it covers; a pull
// request's own auto-merge, stacked or not, is disarmed as before.
func TestDisablePullRequestAutoMergeOrLeaveTheQueue(t *testing.T) {
	armedAlone := &store.PRAutoMerge{Method: MergeMethodSquash}
	for _, tc := range []struct {
		name     string
		state    MergeState
		mutation string
		answer   string
	}{
		{"armed", MergeState{AutoMerge: armedAlone}, "disablePullRequestAutoMerge(input: {pullRequestId: $id})", `{"data":{"disablePullRequestAutoMerge":{"clientMutationId":null}}}`},
		{"armed alone in a stack", MergeState{AutoMerge: armedAlone, Stacked: true, StackBase: "main"}, "disablePullRequestAutoMerge(input: {pullRequestId: $id})", `{"data":{"disablePullRequestAutoMerge":{"clientMutationId":null}}}`},
		{"the stack armed", MergeState{AutoMerge: armedAlone, Stacked: true, StackBase: "main", StackAutoMerge: true}, "disablePullRequestAutoMerge(input: {pullRequestId: $id, includeStack: true})", `{"data":{"disablePullRequestAutoMerge":{"clientMutationId":null}}}`},
		{"queued", MergeState{MergeQueueEnabled: true, InMergeQueue: true}, "dequeuePullRequest(input: {id: $id})", `{"data":{"dequeuePullRequest":{"clientMutationId":null}}}`},
		{"a queued stack", MergeState{InMergeQueue: true, AutoMerge: armedAlone, Stacked: true, StackAutoMerge: true}, "dequeuePullRequest(input: {id: $id})", `{"data":{"dequeuePullRequest":{"clientMutationId":null}}}`},
	} {
		api := &fakeMergeAPI{graphql: tc.answer}
		if err := pullsClient(t, api).DisablePullRequestAutoMerge(context.Background(), 42, "repo", "PR_kwDOnode7", tc.state); err != nil {
			t.Fatal(err)
		}
		query, _ := api.mutations[0]["query"].(string)
		variables, _ := api.mutations[0]["variables"].(map[string]any)
		if len(api.mutations) != 1 || !strings.Contains(query, tc.mutation) || variables["id"] != "PR_kwDOnode7" {
			t.Fatalf("%s: documents = %+v", tc.name, api.mutations)
		}
		if strings.Contains(query, "includeStack") != (tc.name == "the stack armed") {
			t.Fatalf("%s: query %q", tc.name, query)
		}
		if perms := api.lastPermissions(); perms["contents"] != "write" || perms["pull_requests"] != "write" || len(perms) != 3 {
			t.Fatalf("token scope = %+v", api.scopes)
		}
	}
	api := &fakeMergeAPI{graphql: graphQLRefusal("disablePullRequestAutoMerge", "FORBIDDEN", "Resource not accessible by integration")}
	err := pullsClient(t, api).DisablePullRequestAutoMerge(context.Background(), 42, "repo", "PR_kwDOnode7", MergeState{AutoMerge: armedAlone})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusForbidden {
		t.Fatalf("forbidden err = %v", err)
	}
}

// w10/m577: the state read asks for what gh pr merge reads, with the PR
// read's token, and tells the queue, an armed pull request and one GitHub
// would merge now apart; w10/047: an armed one says who armed it and when.
func TestPullRequestMergeStateReadsWhatGhReads(t *testing.T) {
	api := &fakeMergeAPI{answers: map[string]string{"node(id": `{"data":{"node":{"isMergeQueueEnabled":true,"isInMergeQueue":false,"mergeStateStatus":"BLOCKED","autoMergeRequest":{"mergeMethod":"SQUASH","enabledAt":"2026-11-22T09:30:00Z","enabledBy":{"login":"octocat"}}}}}`}}
	state, err := pullsClient(t, api).PullRequestMergeState(context.Background(), 42, "repo", "PR_kwDOnode7")
	if err != nil {
		t.Fatal(err)
	}
	armed := state.AutoMerge
	state.AutoMerge = nil
	if state != (MergeState{MergeQueueEnabled: true, Status: "BLOCKED"}) || state.MergeableNow() || armed == nil ||
		armed.Method != MergeMethodSquash || armed.EnabledBy != "octocat" || armed.EnabledAt == nil ||
		!armed.EnabledAt.Equal(time.Date(2026, 11, 22, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("state = %+v, auto-merge %+v", state, armed)
	}
	query, _ := api.mutations[0]["query"].(string)
	variables, _ := api.mutations[0]["variables"].(map[string]any)
	for _, field := range []string{"isMergeQueueEnabled", "isInMergeQueue", "mergeStateStatus", "autoMergeRequest { mergeMethod enabledAt enabledBy { login } }",
		"stack { baseRefName entries(first: 100) { nodes { position pullRequest { state } } } }", "stackEntry { position }",
		"stackAutoMergeRequest: autoMergeRequest(includeStack: true) { mergeMethod enabledAt enabledBy { login } }"} {
		if !strings.Contains(query, field) {
			t.Fatalf("query %q does not read %s", query, field)
		}
	}
	if variables["id"] != "PR_kwDOnode7" || strings.Contains(query, "mutation") {
		t.Fatalf("query = %+v", api.mutations[0])
	}
	if perms := api.lastPermissions(); perms["pull_requests"] != "read" || len(perms) != 2 {
		t.Fatalf("token scope = %+v, want the PR read's", api.scopes)
	}
	for status, now := range map[string]bool{"CLEAN": true, "HAS_HOOKS": true, "UNSTABLE": true, "BLOCKED": false, "BEHIND": false, "DIRTY": false, "UNKNOWN": false, "DRAFT": false} {
		if (MergeState{Status: status}).MergeableNow() != now {
			t.Errorf("%s mergeable now = %v, want %v (gh's isImmediatelyMergeable)", status, !now, now)
		}
	}
	// A ghost (deleted) account that armed it reads as nobody, and a pull
	// request with nothing armed as no auto-merge.
	for answer, want := range map[string]*store.PRAutoMerge{
		`{"data":{"node":{"isInMergeQueue":true,"mergeStateStatus":"CLEAN","autoMergeRequest":{"mergeMethod":"MERGE","enabledAt":null,"enabledBy":null}}}}`: {Method: MergeMethodMerge},
		`{"data":{"node":{"isInMergeQueue":true,"mergeStateStatus":"CLEAN","autoMergeRequest":null}}}`:                                                      nil,
	} {
		api := &fakeMergeAPI{answers: map[string]string{"node(id": answer}}
		state, err := pullsClient(t, api).PullRequestMergeState(context.Background(), 42, "repo", "PR_kwDOnode7")
		if err != nil || !state.InMergeQueue || (want == nil) != (state.AutoMerge == nil) || (want != nil && *state.AutoMerge != *want) {
			t.Fatalf("%s = %+v %+v / %v", answer, state, state.AutoMerge, err)
		}
	}
	// w10/048: a stacked pull request names its stack's base, and the stack's
	// auto-merge request covering it, armed on it or on one above it, is its
	// auto-merge; a request GitHub reports for a pull request in no stack is
	// left to its own.
	for _, tc := range []struct {
		name, answer string
		want         MergeState
	}{
		{"stacked, nothing armed", `{"data":{"node":{"mergeStateStatus":"CLEAN","autoMergeRequest":null,"stack":{"baseRefName":"main"},"stackAutoMergeRequest":null}}}`,
			MergeState{Status: "CLEAN", Stacked: true, StackBase: "main"}},
		{"stacked, the stack armed above it", `{"data":{"node":{"mergeStateStatus":"BLOCKED","autoMergeRequest":null,"stack":{"baseRefName":"main"},"stackAutoMergeRequest":{"mergeMethod":"REBASE","enabledBy":{"login":"octocat"}}}}}`,
			MergeState{Status: "BLOCKED", Stacked: true, StackBase: "main", StackAutoMerge: true, AutoMerge: &store.PRAutoMerge{Method: MergeMethodRebase, EnabledBy: "octocat"}}},
		{"stacked, armed alone", `{"data":{"node":{"mergeStateStatus":"BLOCKED","autoMergeRequest":{"mergeMethod":"SQUASH"},"stack":{"baseRefName":"main"},"stackAutoMergeRequest":null}}}`,
			MergeState{Status: "BLOCKED", Stacked: true, StackBase: "main", AutoMerge: &store.PRAutoMerge{Method: MergeMethodSquash}}},
		{"in no stack", `{"data":{"node":{"mergeStateStatus":"BLOCKED","autoMergeRequest":{"mergeMethod":"SQUASH"},"stack":null,"stackEntry":null,"stackAutoMergeRequest":{"mergeMethod":"MERGE"}}}}`,
			MergeState{Status: "BLOCKED", AutoMerge: &store.PRAutoMerge{Method: MergeMethodSquash}}},
		// w10/053: what merging it lands, as Devin's merge button counts it:
		// it and every open pull request below it, not a merged or closed
		// one, nor one above it.
		{"third of four, the first merged", `{"data":{"node":{"mergeStateStatus":"BLOCKED","stackEntry":{"position":3},"stack":{"baseRefName":"main","entries":{"nodes":[` +
			`{"position":1,"pullRequest":{"state":"MERGED"}},{"position":2,"pullRequest":{"state":"OPEN"}},{"position":3,"pullRequest":{"state":"OPEN"}},{"position":4,"pullRequest":{"state":"OPEN"}}]}}}}}`,
			MergeState{Status: "BLOCKED", Stacked: true, StackBase: "main", StackLands: 2}},
		{"the bottom", `{"data":{"node":{"mergeStateStatus":"CLEAN","stackEntry":{"position":1},"stack":{"baseRefName":"main","entries":{"nodes":[{"position":2,"pullRequest":{"state":"OPEN"}},{"position":1,"pullRequest":{"state":"OPEN"}}]}}}}}`,
			MergeState{Status: "CLEAN", Stacked: true, StackBase: "main", StackLands: 1}},
		{"a position below it unread", `{"data":{"node":{"mergeStateStatus":"BLOCKED","stackEntry":{"position":3},"stack":{"baseRefName":"main","entries":{"nodes":[{"position":1,"pullRequest":{"state":"OPEN"}},{"position":3,"pullRequest":{"state":"OPEN"}}]}}}}}`,
			MergeState{Status: "BLOCKED", Stacked: true, StackBase: "main"}},
		{"a pull request below it hidden", `{"data":{"node":{"mergeStateStatus":"BLOCKED","stackEntry":{"position":2},"stack":{"baseRefName":"main","entries":{"nodes":[{"position":1,"pullRequest":null},{"position":2,"pullRequest":{"state":"OPEN"}}]}}}}}`,
			MergeState{Status: "BLOCKED", Stacked: true, StackBase: "main"}},
		{"no place read", `{"data":{"node":{"mergeStateStatus":"BLOCKED","stackEntry":null,"stack":{"baseRefName":"main","entries":{"nodes":[{"position":1,"pullRequest":{"state":"OPEN"}}]}}}}}`,
			MergeState{Status: "BLOCKED", Stacked: true, StackBase: "main"}},
	} {
		api := &fakeMergeAPI{answers: map[string]string{"node(id": tc.answer}}
		state, err := pullsClient(t, api).PullRequestMergeState(context.Background(), 42, "repo", "PR_kwDOnode7")
		got, want := state, tc.want
		got.AutoMerge, want.AutoMerge = nil, nil
		if err != nil || got != want || (tc.want.AutoMerge == nil) != (state.AutoMerge == nil) || (state.AutoMerge != nil && *state.AutoMerge != *tc.want.AutoMerge) {
			t.Fatalf("%s: state = %+v %+v / %v, want %+v %+v", tc.name, state, state.AutoMerge, err, tc.want, tc.want.AutoMerge)
		}
	}
	for _, answer := range []string{`{"data":{"node":null}}`, graphQLRefusal("node", "NOT_FOUND", "Could not resolve to a node")} {
		api := &fakeMergeAPI{answers: map[string]string{"node(id": answer}}
		if _, err := pullsClient(t, api).PullRequestMergeState(context.Background(), 42, "repo", "PR_kwDOnode7"); err == nil {
			t.Fatalf("%s read as a state", answer)
		}
	}
}

// asyncMergeAPI is fakeMergeAPI answering an asynchronous merge with status
// and body, and the reads of its result with polls.
func asyncMergeAPI(status int, body string, polls ...string) *fakeMergeAPI {
	return &fakeMergeAPI{settings: allMethods, rules: `[{"type":"pull_request","parameters":{"allowed_merge_methods":["squash","rebase"]}}]`,
		asyncStatus: status, asyncBody: body, asyncPolls: polls}
}

// asyncMergeClient is pullsClient waiting at most wait for an asynchronous
// merge, polling every millisecond.
func asyncMergeClient(t *testing.T, api *fakeMergeAPI, wait time.Duration) *Client {
	c := pullsClient(t, api)
	c.mergeWait, c.mergePause = wait, time.Millisecond
	return c
}

const (
	asyncUUID    = "630b9d5e-3f2a-4f7e-8b0c-2d5f9a8c1e42"
	asyncPending = `{"status":"pending","details":{"message":"Merge request enqueued.","uuid":"` + asyncUUID + `","merge_method":"squash","merge_action":"default","expected_head_sha":"` + pullsTestHead + `"}}`
	asyncMerged  = `{"status":"merged","details":{"message":"Pull request was merged.","sha":"6dcb09b5b57875f334f61aebed695e2e4193db5e"}}`
	asyncQueued  = `{"status":"enqueued","details":{"message":"Pull request is in the merge queue."}}`
)

// w10/053: a pull request in a GitHub stack lands only through GitHub's
// asynchronous merge, which lands it with every open pull request below it
// or queues them where the stack's base requires a merge queue. The call is
// pinned to the head, uses the stack base's method rule with the commit text,
// leaves the choice of a queue to GitHub (merge_action default) and never
// bypasses a rule, and its result is polled within the bound, with the
// merge's own token (Contents write, as GitHub lists both calls).
func TestMergePullRequestAsyncLandsTheStackAndWaitsForItsResult(t *testing.T) {
	api := asyncMergeAPI(http.StatusAccepted, asyncPending, asyncPending, asyncMerged)
	got, err := asyncMergeClient(t, api, time.Minute).MergePullRequestAsync(context.Background(), 42, "octo", "repo", 7,
		MergeOptions{SHA: pullsTestHead, CommitTitle: "Part 2 (#7)", CommitMessage: "The second part.", Base: "main"})
	if err != nil || got != (AsyncMergeResult{Status: AsyncMergeMerged, SHA: "6dcb09b5b57875f334f61aebed695e2e4193db5e", Method: MergeMethodSquash}) {
		t.Fatalf("merge = %+v err=%v, want merged with squash, the first method the stack's base allows", got, err)
	}
	wantBody := map[string]any{"sha": pullsTestHead, "merge_method": "squash", "merge_action": "default", "commit_title": "Part 2 (#7)", "commit_message": "The second part."}
	if len(api.asyncMerges) != 1 || fmt.Sprint(api.asyncMerges[0]) != fmt.Sprint(wantBody) {
		t.Fatalf("merge body = %+v, want %+v", api.asyncMerges, wantBody)
	}
	if want := "/repos/octo/repo/pulls/7/merge-async/" + asyncUUID; len(api.polled) != 2 || api.polled[0] != want || api.polled[1] != want {
		t.Fatalf("polled = %v, want its result read until it merged", api.polled)
	}
	if perms := api.lastPermissions(); perms["contents"] != "write" || perms["pull_requests"] != "write" || perms["workflows"] != "write" || len(api.merges) != 0 {
		t.Fatalf("token scope = %+v, synchronous merges %d", api.scopes, len(api.merges))
	}

	// The stack joins the merge queue its base requires, now or once polled.
	for name, api := range map[string]*fakeMergeAPI{
		"already queued":     asyncMergeAPI(http.StatusOK, asyncQueued),
		"queued once polled": asyncMergeAPI(http.StatusAccepted, asyncPending, asyncQueued),
	} {
		got, err := asyncMergeClient(t, api, time.Minute).MergePullRequestAsync(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Method: MergeMethodRebase, Base: "main"})
		if err != nil || got != (AsyncMergeResult{Status: AsyncMergeEnqueued, Method: MergeMethodRebase}) {
			t.Fatalf("%s: merge = %+v err=%v", name, got, err)
		}
	}

	// A result read that fails is read again: GitHub has accepted the merge.
	api = asyncMergeAPI(http.StatusAccepted, asyncPending, "", asyncMerged)
	if got, err := asyncMergeClient(t, api, time.Minute).MergePullRequestAsync(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"}); err != nil || got.Status != AsyncMergeMerged || len(api.polled) != 2 {
		t.Fatalf("merge after a failed read = %+v err=%v polled=%v", got, err, api.polled)
	}

	// Still landing at the bound: pending, which only the bound ends.
	api = asyncMergeAPI(http.StatusAccepted, asyncPending, asyncPending)
	start := time.Now()
	got, err = asyncMergeClient(t, api, 50*time.Millisecond).MergePullRequestAsync(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"})
	if err != nil || got != (AsyncMergeResult{Status: AsyncMergePending, Method: MergeMethodSquash}) || len(api.polled) < 2 || time.Since(start) > 5*time.Second {
		t.Fatalf("pending merge = %+v err=%v polled=%d", got, err, len(api.polled))
	}
	// A request given up on while it waits stops waiting, with no answer.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api = asyncMergeAPI(http.StatusAccepted, asyncPending, asyncPending)
	api.onPoll = cancel
	if got, err := asyncMergeClient(t, api, time.Minute).MergePullRequestAsync(ctx, 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"}); !errors.Is(err, context.Canceled) || len(api.polled) != 1 {
		t.Fatalf("a request given up on = %+v err=%v polled=%d", got, err, len(api.polled))
	}
	if AsyncMergeWait != 15*time.Second {
		t.Fatalf("AsyncMergeWait = %s; ADR090 D3 states 15 seconds, inside bex-api's 30-second write timeout", AsyncMergeWait)
	}

	// Another merge of it already pending at the same head is waited for; at
	// another head it is GitHub's 409, unpolled.
	api = asyncMergeAPI(http.StatusConflict, strings.Replace(asyncPending, "Merge request enqueued.", "A merge request already exists for this pull request.", 1), asyncMerged)
	if got, err := asyncMergeClient(t, api, time.Minute).MergePullRequestAsync(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: strings.ToUpper(pullsTestHead), Base: "main"}); err != nil || got.Status != AsyncMergeMerged || len(api.polled) != 1 {
		t.Fatalf("pending at the same head = %+v err=%v polled=%v", got, err, api.polled)
	}
	api.polled = nil
	_, err = asyncMergeClient(t, api, time.Minute).MergePullRequestAsync(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: "0000000000000000000000000000000000000000", Base: "main"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict || APIErrorMessage(err) != "A merge request already exists for this pull request." || len(api.polled) != 0 {
		t.Fatalf("pending at another head err=%v (%q) polled=%v", err, APIErrorMessage(err), api.polled)
	}
}

// w10/053: a merge GitHub fails, at once or once it ran, is an
// *AsyncMergeFailedError whose message APIErrorMessage reads and Error()
// never carries; GitHub's other refusals are *APIErrors, and a method the
// stack's base refuses never reaches GitHub.
func TestMergePullRequestAsyncRefusals(t *testing.T) {
	for name, api := range map[string]*fakeMergeAPI{
		"failed at once":     asyncMergeAPI(http.StatusBadRequest, `{"status":"failed","details":{"message":"Pull request is closed."}}`),
		"failed once it ran": asyncMergeAPI(http.StatusAccepted, asyncPending, `{"status":"failed","details":{"message":"Pull request is closed."}}`),
	} {
		_, err := asyncMergeClient(t, api, time.Minute).MergePullRequestAsync(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"})
		var failed *AsyncMergeFailedError
		if !errors.As(err, &failed) || APIErrorMessage(err) != "Pull request is closed." || strings.Contains(err.Error(), "closed") {
			t.Fatalf("%s: err = %v (%q)", name, err, APIErrorMessage(err))
		}
	}
	for _, tc := range []struct {
		status  int
		body    string
		message string
	}{
		{http.StatusUnprocessableEntity, `{"message":"Validation Failed","errors":[{"resource":"PullRequest","code":"custom","message":"sha is invalid"}]}`, "Validation Failed sha is invalid"},
		{http.StatusForbidden, `{"message":"Resource not accessible by integration"}`, "Resource not accessible by integration"},
		{http.StatusBadRequest, `{"message":"Problems parsing JSON"}`, "Problems parsing JSON"},
		{http.StatusNotFound, `{"message":"Not Found"}`, "Not Found"},
	} {
		_, err := asyncMergeClient(t, asyncMergeAPI(tc.status, tc.body), time.Minute).MergePullRequestAsync(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"})
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status != tc.status || APIErrorMessage(err) != tc.message {
			t.Fatalf("%d: err = %v (%q)", tc.status, err, APIErrorMessage(err))
		}
	}
	for name, body := range map[string]string{
		"merged without its commit": `{"status":"merged","details":{"message":"Pull request was merged."}}`,
		"an unknown status":         `{"status":"rejected","details":{"message":"?"}}`,
		"no result":                 `{}`,
	} {
		if _, err := asyncMergeClient(t, asyncMergeAPI(http.StatusOK, body), time.Minute).MergePullRequestAsync(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Base: "main"}); err == nil {
			t.Fatalf("%s read as a result", name)
		}
	}
	api := asyncMergeAPI(http.StatusAccepted, asyncPending, asyncMerged)
	_, err := asyncMergeClient(t, api, time.Minute).MergePullRequestAsync(context.Background(), 42, "octo", "repo", 7, MergeOptions{SHA: pullsTestHead, Method: MergeMethodMerge, Base: "main"})
	var methodErr *MergeMethodError
	if !errors.As(err, &methodErr) || methodErr.Method != MergeMethodMerge || strings.Join(methodErr.Allowed, ",") != "squash,rebase" || len(api.asyncMerges) != 0 {
		t.Fatalf("refused method err=%v merges=%d", err, len(api.asyncMerges))
	}
}
