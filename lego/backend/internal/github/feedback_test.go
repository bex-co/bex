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
	"testing"
)

// The PR feedback reads and the reply (ADR047 D11) use an installation token
// and the documented endpoints; a non-2xx answer is an *APIError.
func TestFeedbackEndpoints(t *testing.T) {
	var posted map[string]string
	var scopes []map[string]any
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app/installations/42/access_tokens" && r.Header.Get("Authorization") != "token ghs_fb" {
			http.Error(w, "no installation token", http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/42/access_tokens":
			var scope map[string]any
			_ = json.NewDecoder(r.Body).Decode(&scope)
			scopes = append(scopes, scope)
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"token":"ghs_fb","expires_at":"2026-11-12T13:00:00Z"}`)
		case r.URL.Path == "/repos/octo/repo/collaborators/alice/permission":
			fmt.Fprint(w, `{"permission":"write","role_name":"maintain"}`)
		case r.URL.Path == "/repos/octo/repo/collaborators/bob/permission":
			fmt.Fprint(w, `{"permission":"read"}`)
		case r.URL.Path == "/repos/octo/repo/collaborators/ghost/permission":
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		case r.URL.Path == "/repos/octo/repo/pulls/7/reviews/55/comments":
			fmt.Fprint(w, `[{"path":"main.go","line":12,"body":"use a const","html_url":"https://github.com/c/1"}]`)
		case r.URL.Path == "/repos/octo/repo/check-suites/77/check-runs":
			fmt.Fprint(w, `{"total_count":1,"check_runs":[{"name":"lint","conclusion":"failure","html_url":"https://github.com/r/1","output":{"title":"2 errors","summary":"x"}}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/octo/repo/issues/7/comments":
			_ = json.NewDecoder(r.Body).Decode(&posted)
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"html_url":"https://github.com/octo/repo/pull/7#issuecomment-1"}`)
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.String(), http.StatusTeapot)
		}
	})
	c := pullsClient(t, api)
	ctx := context.Background()
	if p, err := c.CollaboratorPermission(ctx, 42, "octo", "repo", "alice"); err != nil || p != "write" {
		t.Fatalf("alice = %q %v, want the legacy permission, not the role name", p, err)
	}
	if p, err := c.CollaboratorPermission(ctx, 42, "octo", "repo", "bob"); err != nil || p != "read" {
		t.Fatalf("bob = %q %v", p, err)
	}
	if p, err := c.CollaboratorPermission(ctx, 42, "octo", "repo", "ghost"); err != nil || p != "none" {
		t.Fatalf("a non-collaborator = %q %v, want none", p, err)
	}
	if comments, err := c.ReviewComments(ctx, 42, "octo", "repo", 7, 55); err != nil || len(comments) != 1 || comments[0].Line != 12 || comments[0].URL == "" {
		t.Fatalf("review comments = %+v %v", comments, err)
	}
	if runs, err := c.SuiteCheckRuns(ctx, 42, "octo", "repo", 77); err != nil || len(runs) != 1 || runs[0].Output.Title != "2 errors" {
		t.Fatalf("check runs = %+v %v", runs, err)
	}
	if url, err := c.CreateIssueComment(ctx, 42, "octo", "repo", 7, "done"); err != nil || url == "" || posted["body"] != "done" {
		t.Fatalf("comment = %q %v posted=%v", url, err, posted)
	}
	// Every call mints a token for the one repository with only what it needs.
	want := []string{`{"metadata":"read"}`, `{"metadata":"read"}`, `{"metadata":"read"}`,
		`{"metadata":"read","pull_requests":"read"}`, `{"checks":"read","metadata":"read"}`, `{"metadata":"read","pull_requests":"write"}`}
	if len(scopes) != len(want) {
		t.Fatalf("minted %d tokens, want %d", len(scopes), len(want))
	}
	for i, scope := range scopes {
		perms, _ := json.Marshal(scope["permissions"])
		if string(perms) != want[i] || fmt.Sprint(scope["repositories"]) != "[repo]" {
			t.Errorf("token %d = %s for %v, want %s for [repo]", i, perms, scope["repositories"], want[i])
		}
	}
}

// A `/bex` takeover reads the pull request and the review a comment
// belongs to, and reacts to the comment; a repeat reaction is not an error.
func TestTakeoverEndpoints(t *testing.T) {
	var reactions []string
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/42/access_tokens":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"token":"ghs_fb","expires_at":"2026-11-12T13:00:00Z"}`)
		case r.Header.Get("Authorization") != "token ghs_fb":
			http.Error(w, "no installation token", http.StatusUnauthorized)
		case r.URL.Path == "/repos/octo/repo/pulls/7":
			fmt.Fprint(w, `{"number":7,"title":"Fix it","html_url":"https://github.com/octo/repo/pull/7","state":"open","merged":false,
				"head":{"ref":"feature/x","sha":"0123456789abcdef0123456789abcdef01234567","repo":{"full_name":"octo/repo","default_branch":"main"}},
				"base":{"ref":"main","repo":{"full_name":"octo/repo","default_branch":"main"}}}`)
		case r.URL.Path == "/repos/octo/repo/pulls/8":
			fmt.Fprint(w, `{"number":8,"state":"open","head":{"ref":"patch-1","sha":"ab","repo":null},"base":{"repo":{"full_name":"octo/repo","default_branch":"main"}}}`)
		case r.URL.Path == "/repos/octo/repo/pulls/7/reviews/55":
			fmt.Fprint(w, `{"id":55,"body":"two nits","state":"CHANGES_REQUESTED","html_url":"https://github.com/r/55"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/octo/repo/issues/comments/9001/reactions":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			reactions = append(reactions, "issue:"+body["content"])
			if len(reactions) > 1 {
				w.WriteHeader(http.StatusOK) // GitHub's answer to a repeat reaction
			} else {
				w.WriteHeader(http.StatusCreated)
			}
			fmt.Fprint(w, `{"id":1,"content":"eyes"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/octo/repo/pulls/comments/9002/reactions":
			reactions = append(reactions, "inline")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"id":2}`)
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.String(), http.StatusTeapot)
		}
	})
	c := pullsClient(t, api)
	ctx := context.Background()
	pr, err := c.GetPullRequest(ctx, 42, "octo", "repo", 7)
	if err != nil || pr.Number != 7 || pr.HTMLURL != "https://github.com/octo/repo/pull/7" || pr.HeadRef != "feature/x" || pr.HeadRepo != "octo/repo" || pr.BaseRepo != "octo/repo" ||
		pr.DefaultBranch != "main" || pr.State != "open" || pr.Merged || pr.Title != "Fix it" || len(pr.HeadSHA) != 40 {
		t.Fatalf("pull request = %+v %v", pr, err)
	}
	if fork, err := c.GetPullRequest(ctx, 42, "octo", "repo", 8); err != nil || fork.HeadRepo != "" {
		t.Fatalf("a deleted fork's head repo = %+v %v, want empty", fork, err)
	}
	if review, err := c.GetReview(ctx, 42, "octo", "repo", 7, 55); err != nil || review.State != "changes_requested" || review.Body != "two nits" {
		t.Fatalf("review = %+v %v, want the webhook's lowercase state", review, err)
	}
	for range 2 {
		if err := c.AddCommentReaction(ctx, 42, "octo", "repo", 9001, false, ReactionEyes); err != nil {
			t.Fatalf("reaction: %v", err)
		}
	}
	if err := c.AddCommentReaction(ctx, 42, "octo", "repo", 9002, true, ReactionEyes); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(reactions) != "[issue:eyes issue:eyes inline]" {
		t.Fatalf("reactions = %v", reactions)
	}
	if _, err := c.GetPullRequest(ctx, 42, "octo", "repo", 9); err == nil {
		t.Fatal("a missing pull request read as found")
	}
}

// The takeover fast-forward compares first and moves the pull request's
// branch only when the session head is ahead of it, with force:false, under
// one contents:write token. Behind or diverged is ErrNotFastForward without a
// write; a 422 on the write itself is a branch rule, ErrBranchUpdateRefused.
func TestFastForwardBranch(t *testing.T) {
	const head = "0123456789abcdef0123456789abcdef01234567"
	var patched []map[string]any
	var scopes []string
	status, refuse := "ahead", false
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/42/access_tokens":
			var scope map[string]any
			_ = json.NewDecoder(r.Body).Decode(&scope)
			perms, _ := json.Marshal(scope["permissions"])
			scopes = append(scopes, string(perms))
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"token":"ghs_ff","expires_at":"2026-11-12T13:00:00Z"}`)
		case r.Header.Get("Authorization") != "token ghs_ff":
			http.Error(w, "no installation token", http.StatusUnauthorized)
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/repos/octo/repo/branches/bex-agent%2Fags-1":
			fmt.Fprintf(w, `{"name":"bex-agent/ags-1","commit":{"sha":%q}}`, head)
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/repos/octo/repo/compare/feature%2Ffix-login..."+head:
			fmt.Fprintf(w, `{"status":%q,"ahead_by":1,"files":[]}`, status)
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/repos/octo/repo/branches/feature%2Ffix-login":
			fmt.Fprint(w, `{"name":"feature/fix-login","protected":true}`)
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/octo/repo/git/refs/heads/feature/fix-login":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			patched = append(patched, body)
			if refuse {
				http.Error(w, `{"message":"Protected branch update failed"}`, http.StatusUnprocessableEntity)
				return
			}
			fmt.Fprintf(w, `{"ref":"refs/heads/feature/fix-login","object":{"sha":%q}}`, head)
		default:
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		}
	})
	c := pullsClient(t, api)
	ctx := context.Background()
	ff := func() (string, error) {
		return c.FastForwardBranch(ctx, 42, "octo", "repo", "feature/fix-login", "bex-agent/ags-1")
	}
	if sha, err := ff(); err != nil || sha != head {
		t.Fatalf("fast-forward = %q %v", sha, err)
	}
	if len(patched) != 1 || patched[0]["sha"] != head || patched[0]["force"] != false {
		t.Fatalf("PATCH bodies = %v, want {sha, force:false}", patched)
	}
	if len(scopes) != 1 || scopes[0] != `{"contents":"write","metadata":"read"}` {
		t.Fatalf("tokens = %v, want one contents:write token", scopes)
	}
	// Already there: nothing to move, nothing written.
	status = "identical"
	if sha, err := ff(); err != nil || sha != head || len(patched) != 1 {
		t.Fatalf("an identical head = %q %v after %d writes", sha, err, len(patched))
	}
	for _, s := range []string{"behind", "diverged"} {
		status = s
		if _, err := ff(); !errors.Is(err, ErrNotFastForward) || len(patched) != 1 {
			t.Fatalf("%s = %v after %d writes, want ErrNotFastForward and no write", s, err, len(patched))
		}
	}
	status, refuse = "ahead", true
	if _, err := ff(); !errors.Is(err, ErrBranchUpdateRefused) || errors.Is(err, ErrNotFastForward) {
		t.Fatalf("a refused fast-forward = %v, want ErrBranchUpdateRefused", err)
	}
	if _, err := c.FastForwardBranch(ctx, 42, "octo", "repo", "feature/fix-login", "bex-agent/missing"); !errors.Is(err, ErrHeadBranchNotFound) {
		t.Fatalf("an unpushed session branch = %v, want ErrHeadBranchNotFound", err)
	}
	if protected, err := c.BranchProtected(ctx, 42, "octo", "repo", "feature/fix-login"); err != nil || !protected {
		t.Fatalf("branch protection = %v %v", protected, err)
	}
	if sha, err := c.BranchHead(ctx, 42, "octo", "repo", "bex-agent/ags-1"); err != nil || sha != head {
		t.Fatalf("branch head = %q %v", sha, err)
	}
	if _, err := c.BranchHead(ctx, 42, "octo", "repo", "bex-agent/missing"); !errors.Is(err, ErrHeadBranchNotFound) {
		t.Fatalf("a missing branch's head = %v, want ErrHeadBranchNotFound", err)
	}
}

func TestBexCommandReadsOnlyALeadingBexWord(t *testing.T) {
	for body, want := range map[string]string{
		"/bex fix the failing test": "fix the failing test",
		"/BEX Fix it":               "Fix it",
		"  /Bex\n\nfix\nthe test":   "fix\nthe test",
		"/bex":                      "",
		"\t/bex   rename foo  ":     "rename foo",
		"/bex\u00a0non-breaking":    "non-breaking",
	} {
		if got, ok := BexCommand(body); !ok || got != want {
			t.Errorf("BexCommand(%q) = %q %v, want %q", body, got, ok, want)
		}
	}
	for _, body := range []string{"please /bex fix it", "/bexify it", "/be x", "bex fix", "", "> /bex quoted", "/bexé"} {
		if _, ok := BexCommand(body); ok {
			t.Errorf("BexCommand(%q) read as the command", body)
		}
	}
}
