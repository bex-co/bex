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

package apps

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- rootDirMatches (the path-scoped auto-deploy filter, monorepo support) ---

func TestRootDirMatches(t *testing.T) {
	cases := []struct {
		name    string
		rootDir string
		paths   []string
		want    bool
	}{
		{"empty rootDir always matches (today's whole-repo behavior)", "", []string{"unrelated/file.go"}, true},
		{"no changed paths at all fails open (nothing to filter on)", "services/api", nil, true},
		{"file inside rootDir matches", "services/api", []string{"services/api/main.go"}, true},
		{"file outside rootDir does not match", "services/api", []string{"services/web/index.js"}, false},
		{"file at an unrelated top-level path does not match", "services/api", []string{"README.md"}, false},
		{"one matching path among several is enough", "services/api", []string{"README.md", "services/api/main.go"}, true},
		{"leading slash on the changed path is tolerated", "services/api", []string{"/services/api/main.go"}, true},
		{"trailing slash on rootDir is tolerated", "services/api/", []string{"services/api/main.go"}, true},
		{"sibling directory with a matching prefix string is not a match", "services/api", []string{"services/api-gateway/main.go"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rootDirMatches(c.rootDir, c.paths); got != c.want {
				t.Errorf("rootDirMatches(%q, %v) = %v, want %v", c.rootDir, c.paths, got, c.want)
			}
		})
	}
}

// --- buildFilterMatches (Render's Build Filters, glob-based auto-deploy gate) ---

func TestBuildFilterMatches(t *testing.T) {
	bf := func(paths, ignored []string) *appv1alpha1.BuildFilterSpec {
		return &appv1alpha1.BuildFilterSpec{Paths: paths, IgnoredPaths: ignored}
	}
	cases := []struct {
		name   string
		filter *appv1alpha1.BuildFilterSpec
		paths  []string
		want   bool
	}{
		{"nil filter always matches (today's behavior)", nil, []string{"anything.go"}, true},
		{"all-empty filter always matches", bf(nil, nil), []string{"anything.go"}, true},
		{"no changed paths fails open (nothing to filter on)", bf([]string{"src/**"}, nil), nil, true},

		// Included paths only: deploy iff a changed file matches an include glob.
		{"included: a matching file triggers", bf([]string{"src/**"}, nil), []string{"src/app/main.go"}, true},
		{"included: no matching file does not trigger", bf([]string{"src/**"}, nil), []string{"web/index.js"}, false},
		{"included: one match among several is enough", bf([]string{"src/**"}, nil), []string{"web/index.js", "src/app/main.go"}, true},
		{"included: globstar spans directories", bf([]string{"**/*.md"}, nil), []string{"docs/guide/readme.md"}, true},
		{"included: leading slash on the changed path is tolerated", bf([]string{"src/**"}, nil), []string{"/src/main.go"}, true},

		// Ignored paths only: skip iff ALL changed files are ignored.
		{"ignored: all changed files ignored => no deploy", bf(nil, []string{"docs/**"}), []string{"docs/a.md", "docs/b.md"}, false},
		{"ignored: one non-ignored file => deploy", bf(nil, []string{"docs/**"}), []string{"docs/a.md", "src/main.go"}, true},

		// Both: ignored wins over included, even for a file that matches both.
		{"both: a file that is included but also ignored does not trigger", bf([]string{"src/**"}, []string{"src/**/*.test.go"}), []string{"src/app/main.test.go"}, false},
		{"both: an included, non-ignored file triggers", bf([]string{"src/**"}, []string{"src/**/*.test.go"}), []string{"src/app/main.go"}, true},
		{"both: a mix with one triggering file deploys", bf([]string{"src/**"}, []string{"src/**/*.test.go"}), []string{"src/app/main.test.go", "src/app/main.go"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := buildFilterMatches(c.filter, c.paths); got != c.want {
				t.Errorf("buildFilterMatches(%+v, %v) = %v, want %v", c.filter, c.paths, got, c.want)
			}
		})
	}
}

// TestRedeployMatchingComposesBuildFilterWithRootDir proves buildFilter is ANDed
// after rootDir scoping: a change inside rootDir that the buildFilter ignores does
// not redeploy, while an included change inside rootDir does.
func TestRedeployMatchingComposesBuildFilterWithRootDir(t *testing.T) {
	const repo = "https://github.com/x/mono"
	app := &appv1alpha1.App{}
	app.Name, app.Namespace = "api", "default"
	app.Spec = appv1alpha1.AppSpec{
		Repo: repo, Branch: "main", RootDir: "services/api", AutoDeploy: true,
		BuildFilter: &appv1alpha1.BuildFilterSpec{IgnoredPaths: []string{"services/api/**/*.md"}},
	}
	svc, _ := newService(nil, app)
	h := &GitWebhook{Svc: svc, Secret: "shh"}

	// Inside rootDir but only a doc change the filter ignores => no redeploy.
	ignored := newPush(repo, []string{"services/api/README.md"})
	got, err := h.redeployMatching(context.Background(), ignored, "main")
	if err != nil {
		t.Fatalf("redeployMatching: %v", err)
	}
	if contains(got, "api") {
		t.Errorf("an in-rootDir push matching only ignoredPaths must not redeploy: %v", got)
	}

	// Inside rootDir and not ignored => redeploy.
	included := newPush(repo, []string{"services/api/main.go"})
	got2, err := h.redeployMatching(context.Background(), included, "main")
	if err != nil {
		t.Fatalf("redeployMatching: %v", err)
	}
	if !contains(got2, "api") {
		t.Errorf("an in-rootDir, non-ignored push must redeploy: %v", got2)
	}
}

func TestPushEventChangedPaths(t *testing.T) {
	var ev pushEvent
	ev.Commits = append(ev.Commits, struct {
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	}{Added: []string{"a.go"}, Removed: []string{"b.go"}, Modified: []string{"c.go"}})
	ev.Commits = append(ev.Commits, struct {
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	}{Modified: []string{"services/api/main.go"}})

	got := ev.changedPaths()
	want := []string{"a.go", "b.go", "c.go", "services/api/main.go"}
	if len(got) != len(want) {
		t.Fatalf("changedPaths() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("changedPaths()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// --- redeployMatching end-to-end: in-root vs out-of-root push ---

func newPush(cloneURL string, changed []string) pushEvent {
	ev := pushEvent{Ref: "refs/heads/main"}
	ev.Repository.CloneURL = cloneURL
	ev.Commits = []struct {
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	}{{Modified: changed}}
	return ev
}

func TestRedeployMatchingScopesToRootDir(t *testing.T) {
	const repo = "https://github.com/x/mono"
	scoped := &appv1alpha1.App{}
	scoped.Name, scoped.Namespace = "api", "default"
	scoped.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", RootDir: "services/api", AutoDeploy: true}

	unscoped := &appv1alpha1.App{}
	unscoped.Name, unscoped.Namespace = "whole-repo", "default"
	unscoped.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", AutoDeploy: true}

	svc, _ := newService(nil, scoped, unscoped)
	h := &GitWebhook{Svc: svc, Secret: "shh"}

	outside := newPush(repo, []string{"services/web/index.js"})
	redeployed, err := h.redeployMatching(context.Background(), outside, "main")
	if err != nil {
		t.Fatalf("redeployMatching: %v", err)
	}
	if contains(redeployed, "api") {
		t.Errorf("push outside rootDir must not redeploy the scoped App: redeployed=%v", redeployed)
	}
	if !contains(redeployed, "whole-repo") {
		t.Errorf("push outside rootDir must still redeploy an App with no rootDir: redeployed=%v", redeployed)
	}

	inside := newPush(repo, []string{"services/api/main.go"})
	redeployed2, err := h.redeployMatching(context.Background(), inside, "main")
	if err != nil {
		t.Fatalf("redeployMatching: %v", err)
	}
	if !contains(redeployed2, "api") {
		t.Errorf("push inside rootDir must redeploy the scoped App: redeployed=%v", redeployed2)
	}
	if !contains(redeployed2, "whole-repo") {
		t.Errorf("push inside rootDir must still redeploy an App with no rootDir: redeployed=%v", redeployed2)
	}
}

// TestWebhookHTTPParsesCommitPaths exercises the real JSON wire shape (a raw
// push payload's "commits[].modified", not a hand-built pushEvent) through
// ServeHTTP end to end, so a tag/parsing regression in pushEvent would fail
// here even though TestRedeployMatchingScopesToRootDir builds pushEvent directly.
func TestWebhookHTTPParsesCommitPaths(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/mono"
	scoped := &appv1alpha1.App{}
	scoped.Name, scoped.Namespace = "api", "default"
	scoped.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", RootDir: "services/api", AutoDeploy: true}
	svc, cl := newService(nil, scoped)
	h := &GitWebhook{Svc: svc, Secret: secret}

	body, err := json.Marshal(map[string]any{
		"ref":        "refs/heads/main",
		"repository": map[string]string{"clone_url": repo},
		"commits":    []map[string]any{{"modified": []string{"services/web/index.js"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", sign(secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("=> 200, got %d: %s", rec.Code, rec.Body)
	}
	if getApp(t, cl, "api").Spec.RestartedAt != "" {
		t.Error("a push whose commits[].modified sits outside rootDir must not redeploy")
	}
}

// autoDeployApp is a repo-backed app that opts into push redeploys.
func autoDeployApp(name, repo string) *appv1alpha1.App {
	a := &appv1alpha1.App{}
	a.Name, a.Namespace = name, "default"
	a.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", AutoDeploy: true}
	return a
}

type webhookStartedCall struct{ tenantID, appName string }

type blockingWebhookStartedNotifier struct {
	calls   chan webhookStartedCall
	release chan struct{}
}

func (n *blockingWebhookStartedNotifier) NotifyDeployStarted(_ context.Context, tenantID, appName, _ string) {
	n.calls <- webhookStartedCall{tenantID: tenantID, appName: appName}
	<-n.release
}

func postPush(t *testing.T, h *GitWebhook, sigSecret, event string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
	if sigSecret != "" {
		req.Header.Set("X-Hub-Signature-256", sign(sigSecret, body))
	}
	if event != "" {
		req.Header.Set("X-GitHub-Event", event)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestWebhookAcceptsEitherKey(t *testing.T) {
	const repo = "https://github.com/x/app"
	body := pushBody(t, repo, "refs/heads/main")

	// GitHub App key alone (BEX_WEBHOOK_SECRET unset): a GitHub-App-signed push
	// still redeploys.
	svc, cl := newService(nil, autoDeployApp("api", repo))
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
	if rec := postPush(t, h, "gh-key", "push", body); rec.Code != http.StatusOK {
		t.Fatalf("github key => %d: %s", rec.Code, rec.Body)
	}
	if getApp(t, cl, "api").Spec.RestartedAt == "" {
		t.Error("github-key push should redeploy")
	}

	// Both keys set: valid under EITHER key is accepted.
	for _, key := range []string{"manual-key", "gh-key"} {
		svc, cl := newService(nil, autoDeployApp("api", repo))
		h := &GitWebhook{Svc: svc, Secret: "manual-key", GitHubSecret: "gh-key"}
		if rec := postPush(t, h, key, "push", body); rec.Code != http.StatusOK {
			t.Fatalf("key %s => %d", key, rec.Code)
		}
		if getApp(t, cl, "api").Spec.RestartedAt == "" {
			t.Errorf("push signed with %s should redeploy", key)
		}
	}
}

func TestWebhookNotifiesDeployStartedOffRequestPath(t *testing.T) {
	const repo = "https://github.com/x/app"
	a := autoDeployApp("api", repo)
	a.Labels = map[string]string{core.LabelTenant: "tea-a"}
	svc, _ := newService(nil, a)
	notifier := &blockingWebhookStartedNotifier{
		calls:   make(chan webhookStartedCall, 1),
		release: make(chan struct{}),
	}
	defer close(notifier.release)
	svc.StartedNotifier = notifier
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
	body := pushBody(t, repo, "refs/heads/main")

	response := make(chan *httptest.ResponseRecorder, 1)
	go func() { response <- postPush(t, h, "gh-key", "push", body) }()

	select {
	case rec := <-response:
		if rec.Code != http.StatusOK {
			t.Fatalf("push => %d: %s", rec.Code, rec.Body)
		}
	case <-time.After(time.Second):
		t.Fatal("git webhook blocked on deploy-start notification delivery")
	}
	select {
	case got := <-notifier.calls:
		if got != (webhookStartedCall{tenantID: "tea-a", appName: "api"}) {
			t.Errorf("started notification = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("git webhook returned without issuing a deploy-start notification")
	}
}

func TestWebhookRejectsUnknownKey(t *testing.T) {
	body := pushBody(t, "https://github.com/x/app", "refs/heads/main")
	svc, _ := newService(nil, autoDeployApp("api", "https://github.com/x/app"))
	h := &GitWebhook{Svc: svc, Secret: "manual-key", GitHubSecret: "gh-key"}
	if rec := postPush(t, h, "attacker-key", "push", body); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-key push => %d, want 401", rec.Code)
	}
}

func TestWebhookGitHubLifecycleEventsAreNoOp(t *testing.T) {
	const repo = "https://github.com/x/app"
	body := pushBody(t, repo, "refs/heads/main")
	svc, cl := newService(nil, autoDeployApp("api", repo))
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}

	// A validly-signed ping (or installation) is a 200 no-op, not a 401 and not a redeploy.
	for _, event := range []string{"ping", "installation", "installation_repositories"} {
		rec := postPush(t, h, "gh-key", event, body)
		if rec.Code != http.StatusOK {
			t.Errorf("%s => %d, want 200", event, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "ignored") {
			t.Errorf("%s body = %s, want ignored", event, rec.Body.String())
		}
	}
	if getApp(t, cl, "api").Spec.RestartedAt != "" {
		t.Error("lifecycle events must not redeploy")
	}

	// An unsigned ping is still rejected — the signature gate runs first.
	if rec := postPush(t, h, "", "ping", body); rec.Code != http.StatusUnauthorized {
		t.Errorf("unsigned ping => %d, want 401", rec.Code)
	}
}

func TestWebhookAutoDeployFalseSuppressesGitHubPush(t *testing.T) {
	const repo = "https://github.com/x/app"
	// AutoDeploy off: a valid GitHub-App-signed push must NOT redeploy.
	app := &appv1alpha1.App{}
	app.Name, app.Namespace = "api", "default"
	app.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", AutoDeploy: false}
	svc, cl := newService(nil, app)
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}

	rec := postPush(t, h, "gh-key", "push", pushBody(t, repo, "refs/heads/main"))
	if rec.Code != http.StatusOK {
		t.Fatalf("push => %d, want 200", rec.Code)
	}
	if getApp(t, cl, "api").Spec.RestartedAt != "" {
		t.Error("autoDeploy:false must suppress the GitHub-key push redeploy")
	}
}

func TestWebhook503WhenNoKeys(t *testing.T) {
	svc, _ := newService(nil)
	h := &GitWebhook{Svc: svc} // neither key set
	body := pushBody(t, "https://github.com/x/app", "refs/heads/main")
	if rec := postPush(t, h, "anything", "push", body); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no keys => %d, want 503", rec.Code)
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
