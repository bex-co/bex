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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const pullsTestHead = "0123456789abcdef0123456789abcdef01234567"

// fakePullsAPI is GitHub's REST surface for one repository (octo/repo, default
// branch main) as SyncDraftPullRequest uses it.
type fakePullsAPI struct {
	mu        sync.Mutex
	notPushed bool
	aheadBy   int
	open      []map[string]any
	created   []map[string]any
	patched   []map[string]any
	scope     map[string]any
}

func (f *fakePullsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	decode := func() map[string]any {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		return body
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/app/installations/42/access_tokens":
		f.scope = decode()
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"token":"ghs_pr","expires_at":"2026-10-22T13:00:00Z"}`)
	case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/repo":
		fmt.Fprint(w, `{"full_name":"octo/repo","default_branch":"main"}`)
	case r.Method == http.MethodGet && r.URL.EscapedPath() == "/repos/octo/repo/branches/bex-agent%2Ftask-1":
		if f.notPushed {
			http.Error(w, `{"message":"Branch not found"}`, http.StatusNotFound)
			return
		}
		fmt.Fprintf(w, `{"commit":{"sha":%q}}`, pullsTestHead)
	case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/repo/compare/main..."+pullsTestHead:
		if r.URL.Query().Get("per_page") != "1" {
			http.Error(w, "compare must page to one commit", http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, `{"ahead_by":%d,"commits":[]}`, f.aheadBy)
	case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/repo/pulls":
		if r.URL.Query().Get("head") != "octo:bex-agent/task-1" || r.URL.Query().Get("state") != "open" {
			http.Error(w, "bad list query", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(f.open)
	case r.Method == http.MethodPost && r.URL.Path == "/repos/octo/repo/pulls":
		body := decode()
		f.created = append(f.created, body)
		pr := map[string]any{"number": 7, "html_url": "https://github.com/octo/repo/pull/7", "state": "open", "draft": body["draft"]}
		f.open = append(f.open, pr)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(pr)
	case r.Method == http.MethodPatch && r.URL.Path == "/repos/octo/repo/pulls/7":
		f.patched = append(f.patched, decode())
		fmt.Fprint(w, `{"number":7,"html_url":"https://github.com/octo/repo/pull/7","state":"open","draft":true}`)
	default:
		http.Error(w, "unexpected "+r.Method+" "+r.URL.String(), http.StatusTeapot)
	}
}

func pullsClient(t *testing.T, api http.Handler) *Client {
	t.Helper()
	keyPEM, _ := testKeyPEM(t)
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	c, err := NewClient(Config{AppID: "1", PrivateKey: keyPEM, Slug: "bex"})
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = server.URL
	return c
}

func TestSyncDraftPullRequestCreatesThenUpdatesWithoutUndrafting(t *testing.T) {
	api := &fakePullsAPI{aheadBy: 2}
	c := pullsClient(t, api)
	ctx := context.Background()

	first, err := c.SyncDraftPullRequest(ctx, 42, "octo", "repo", "bex-agent/task-1", "Add retries", "body one")
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || first.Number != 7 || first.HeadSHA != pullsTestHead || !first.Draft {
		t.Fatalf("create = %+v", first)
	}
	if len(api.created) != 1 || api.created[0]["draft"] != true || api.created[0]["head"] != "bex-agent/task-1" || api.created[0]["base"] != "main" {
		t.Fatalf("create payload = %+v", api.created)
	}
	perms, _ := api.scope["permissions"].(map[string]any)
	repos, _ := api.scope["repositories"].([]any)
	if perms["pull_requests"] != "write" || perms["contents"] != "read" || len(repos) != 1 || repos[0] != "repo" {
		t.Fatalf("token scope = %+v", api.scope)
	}

	second, err := c.SyncDraftPullRequest(ctx, 42, "octo", "repo", "bex-agent/task-1", "Add retries v2", "body two")
	if err != nil {
		t.Fatal(err)
	}
	if second.Created || second.Number != 7 || len(api.created) != 1 {
		t.Fatalf("update = %+v (creates %d)", second, len(api.created))
	}
	if len(api.patched) != 1 {
		t.Fatalf("patches = %d, want 1", len(api.patched))
	}
	patch := api.patched[0]
	if patch["title"] != "Add retries v2" || patch["body"] != "body two" || len(patch) != 2 {
		t.Fatalf("PATCH payload = %+v, want only title + body (never draft/base/state)", patch)
	}
}

func TestSyncDraftPullRequestTypedRefusals(t *testing.T) {
	c := pullsClient(t, &fakePullsAPI{notPushed: true})
	if _, err := c.SyncDraftPullRequest(context.Background(), 42, "octo", "repo", "bex-agent/task-1", "t", "b"); !errors.Is(err, ErrHeadBranchNotFound) {
		t.Fatalf("unpushed head err = %v, want ErrHeadBranchNotFound", err)
	}
	api := &fakePullsAPI{aheadBy: 0}
	c = pullsClient(t, api)
	if _, err := c.SyncDraftPullRequest(context.Background(), 42, "octo", "repo", "bex-agent/task-1", "t", "b"); !errors.Is(err, ErrHeadNotAhead) {
		t.Fatalf("level head err = %v, want ErrHeadNotAhead", err)
	}
	if len(api.created) != 0 {
		t.Fatal("a head with nothing to review opened a PR")
	}
}

// Upstream error text never reaches Error() (w6/005).
func TestSyncDraftPullRequestHidesUpstreamText(t *testing.T) {
	c := pullsClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		http.Error(w, "internal secret detail", http.StatusInternalServerError)
	}))
	_, err := c.SyncDraftPullRequest(context.Background(), 42, "octo", "repo", "bex-agent/task-1", "t", "b")
	if err == nil || strings.Contains(err.Error(), "secret detail") {
		t.Fatalf("err = %v", err)
	}
}
