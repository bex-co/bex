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
