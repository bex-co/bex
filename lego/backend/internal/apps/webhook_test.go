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
	"github.com/bex-co/bex/lego/backend/internal/store"
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
	got, _, err := h.redeployMatching(context.Background(), ignored, "main", "")
	if err != nil {
		t.Fatalf("redeployMatching: %v", err)
	}
	if contains(got, "api") {
		t.Errorf("an in-rootDir push matching only ignoredPaths must not redeploy: %v", got)
	}

	// Inside rootDir and not ignored => redeploy.
	included := newPush(repo, []string{"services/api/main.go"})
	got2, _, err := h.redeployMatching(context.Background(), included, "main", "")
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

type recordingFactWriter struct {
	facts map[string]store.ServiceEventFact
}

func (w *recordingFactWriter) InsertServiceEventFact(_ context.Context, fact store.ServiceEventFact) (bool, error) {
	if w.facts == nil {
		w.facts = map[string]store.ServiceEventFact{}
	}
	if _, exists := w.facts[fact.SourceKey]; exists {
		return false, nil
	}
	w.facts[fact.SourceKey] = fact
	return true, nil
}

func TestIgnoredCommitFactIsTypedAndDeliveryIdempotent(t *testing.T) {
	const repo = "https://github.com/x/mono"
	app := autoDeployApp("api", repo)
	app.Spec.RootDir = "services/api"
	app.Labels = map[string]string{
		store.LabelManagedBy: store.ManagedByValue,
		store.LabelAppID:     "srv-api",
	}
	svc, _ := newService(nil, app)
	writer := &recordingFactWriter{}
	svc.EventFacts = writer
	h := &GitWebhook{Svc: svc, Secret: "shh"}
	ev := newPush(repo, []string{"services/web/index.js"})
	ev.DeliveryKey = "delivery-1"
	ev.After = "abc123"
	ev.HeadCommit.ID = "abc123"
	ev.HeadCommit.URL = "https://github.com/x/mono/commit/abc123"

	for range 2 {
		if redeployed, _, err := h.redeployMatching(context.Background(), ev, "main", ""); err != nil || len(redeployed) != 0 {
			t.Fatalf("redeploy = %v, err = %v, want ignored", redeployed, err)
		}
	}
	if len(writer.facts) != 1 {
		t.Fatalf("fact count = %d, want one across delivery retry", len(writer.facts))
	}
	for _, fact := range writer.facts {
		if fact.Type != store.EventFactCommitIgnored || fact.ReasonCode != store.EventReasonRootDirectory || fact.CommitID != "abc123" {
			t.Fatalf("ignored fact = %+v", fact)
		}
	}
}

// TestWebhookBranchDeleteRecordsFactAndDisablesAutoDeploy proves w7/m66's
// branch_deleted: a `delete` event for a tracked branch records the typed fact
// once (idempotent across a delivery retry) and disables the service's
// auto-deploy — the branch it built from is gone.
func TestWebhookBranchDeleteRecordsFactAndDisablesAutoDeploy(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/mono"
	app := autoDeployApp("api", repo)
	app.Labels = map[string]string{
		store.LabelManagedBy: store.ManagedByValue,
		store.LabelAppID:     "srv-api",
	}
	svc, cl := newService(nil, app)
	writer := &recordingFactWriter{}
	svc.EventFacts = writer
	h := &GitWebhook{Svc: svc, Secret: secret}

	body, err := json.Marshal(map[string]any{
		"ref":        "main",
		"ref_type":   "branch",
		"repository": map[string]string{"clone_url": repo},
	})
	if err != nil {
		t.Fatal(err)
	}
	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
		req.Header.Set("X-Hub-Signature-256", sign(secret, body))
		req.Header.Set("X-GitHub-Event", "delete")
		req.Header.Set("X-GitHub-Delivery", "del-1")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for i := range 2 { // deliver twice: the fact must be idempotent
		if rec := send(); rec.Code != http.StatusOK {
			t.Fatalf("delivery %d => 200, got %d: %s", i, rec.Code, rec.Body)
		}
	}
	if len(writer.facts) != 1 {
		t.Fatalf("fact count = %d, want 1 across delivery retry", len(writer.facts))
	}
	for _, fact := range writer.facts {
		if fact.Type != store.EventFactBranchDeleted || fact.BranchFrom != "main" {
			t.Fatalf("branch_deleted fact = %+v", fact)
		}
	}
	if getApp(t, cl, "api").Spec.AutoDeploy {
		t.Error("branch delete must disable auto-deploy for a service tracking the deleted branch")
	}
}

// TestWebhookBranchDeleteVariants covers the push-with-deleted signal (git push
// --delete) and the tag-delete no-op.
func TestWebhookBranchDeleteVariants(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/mono"

	t.Run("push with deleted=true records the fact", func(t *testing.T) {
		app := autoDeployApp("api", repo)
		app.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-api"}
		svc, cl := newService(nil, app)
		writer := &recordingFactWriter{}
		svc.EventFacts = writer
		h := &GitWebhook{Svc: svc, Secret: secret}

		body, _ := json.Marshal(map[string]any{
			"ref":        "refs/heads/main",
			"deleted":    true,
			"after":      "0000000000000000000000000000000000000000",
			"repository": map[string]string{"clone_url": repo},
		})
		req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
		req.Header.Set("X-Hub-Signature-256", sign(secret, body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("=> 200, got %d: %s", rec.Code, rec.Body)
		}
		if len(writer.facts) != 1 {
			t.Fatalf("fact count = %d, want 1", len(writer.facts))
		}
		if getApp(t, cl, "api").Spec.AutoDeploy {
			t.Error("push --delete of the tracked branch must disable auto-deploy")
		}
	})

	t.Run("tag delete is a no-op", func(t *testing.T) {
		app := autoDeployApp("api", repo)
		app.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-api"}
		svc, cl := newService(nil, app)
		writer := &recordingFactWriter{}
		svc.EventFacts = writer
		h := &GitWebhook{Svc: svc, Secret: secret}

		body, _ := json.Marshal(map[string]any{
			"ref":        "v1.0.0",
			"ref_type":   "tag",
			"repository": map[string]string{"clone_url": repo},
		})
		req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
		req.Header.Set("X-Hub-Signature-256", sign(secret, body))
		req.Header.Set("X-GitHub-Event", "delete")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("=> 200, got %d: %s", rec.Code, rec.Body)
		}
		if len(writer.facts) != 0 {
			t.Fatalf("tag delete recorded %d facts, want 0", len(writer.facts))
		}
		if !getApp(t, cl, "api").Spec.AutoDeploy {
			t.Error("tag delete must not touch auto-deploy")
		}
	})
}

func TestCommitSkipPhrases(t *testing.T) {
	for _, message := range []string{"release [skip render]", "[RENDER SKIP] docs"} {
		if !commitHasSkipPhrase(message) {
			t.Errorf("%q did not match", message)
		}
	}
	if commitHasSkipPhrase("ordinary release") {
		t.Error("ordinary commit message matched a skip phrase")
	}
}

// --- redeployMatching end-to-end: in-root vs out-of-root push ---

func newPush(cloneURL string, changed []string) pushEvent {
	ev := pushEvent{Ref: "refs/heads/main", After: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
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
	redeployed, _, err := h.redeployMatching(context.Background(), outside, "main", "")
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
	redeployed2, _, err := h.redeployMatching(context.Background(), inside, "main", "")
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
		"after":      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
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

// TestWebhookRejectsHeaderSwappedDeleteBodyAsPush pins codex-security round 12,
// finding 10: the HMAC covers the body, not the X-GitHub-Event header, so a
// captured signed delete delivery replayed with the header stripped (the
// pre-fix Gitea fallback) or forced to "push" must NOT be reinterpreted as a
// push — it is a 400 before scope resolution, never redeploys, and the
// legitimate delete delivery afterwards still processes (the replays were
// rejected before claiming anything).
func TestWebhookRejectsHeaderSwappedDeleteBodyAsPush(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/mono"
	app := autoDeployApp("api", repo)
	app.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-api"}
	svc, cl := newService(nil, app)
	writer := &recordingFactWriter{}
	svc.EventFacts = writer
	h := &GitWebhook{Svc: svc, Secret: secret}

	deleteBody, err := json.Marshal(map[string]any{
		"ref":        "main",
		"ref_type":   "branch",
		"repository": map[string]string{"clone_url": repo},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"", "push"} {
		rec := postPush(t, h, secret, event, deleteBody)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("header %q on delete body => 400, got %d: %s", event, rec.Code, rec.Body)
		}
		if getApp(t, cl, "api").Spec.RestartedAt != "" {
			t.Fatalf("header %q reinterpreted a delete body as a push", event)
		}
	}
	// A body with no ref at all (e.g. a ping payload) is likewise not a push.
	pingBody, _ := json.Marshal(map[string]any{"zen": "Keep it logically awesome.", "hook_id": 1})
	if rec := postPush(t, h, secret, "push", pingBody); rec.Code != http.StatusBadRequest {
		t.Fatalf("ping body under push header => 400, got %d: %s", rec.Code, rec.Body)
	}

	// The legitimate delete delivery still processes after the replays.
	req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(deleteBody)))
	req.Header.Set("X-Hub-Signature-256", sign(secret, deleteBody))
	req.Header.Set("X-GitHub-Event", "delete")
	req.Header.Set("X-GitHub-Delivery", "del-legit")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("legitimate delete => 200, got %d: %s", rec.Code, rec.Body)
	}
	if getApp(t, cl, "api").Spec.AutoDeploy {
		t.Error("legitimate branch delete must disable auto-deploy")
	}
}

// A push-shaped body replayed under the delete header is a signed no-op, not a
// branch deletion (the symmetric half of the shape binding).
func TestWebhookDeleteHeaderOnPushBodyIsNoOp(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/mono"
	app := autoDeployApp("api", repo)
	app.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-api"}
	svc, cl := newService(nil, app)
	writer := &recordingFactWriter{}
	svc.EventFacts = writer
	h := &GitWebhook{Svc: svc, Secret: secret}

	rec := postPush(t, h, secret, "delete", pushBody(t, repo, "refs/heads/main"))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete header on push body => 200 no-op, got %d: %s", rec.Code, rec.Body)
	}
	if len(writer.facts) != 0 {
		t.Fatalf("push body under delete header recorded %+v, want none", writer.facts)
	}
	if !getApp(t, cl, "api").Spec.AutoDeploy {
		t.Error("push body under delete header must not disable auto-deploy")
	}
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

// --- push-to-deploy opens a deploy row (trigger "new_commit") ---

// TestWebhookPushOpensDeployRow proves a push on a store-managed App opens a
// deploys row through the real wire shape (raw JSON with head_commit, HMAC
// signature, ServeHTTP end to end): trigger "new_commit", the requested
// release generation, and the pushed head commit as provenance — plus the
// release-generation annotation on the CR, the same stamp deploys.Trigger
// writes. The row is what puts push-to-deploy in the deploy history and what
// replaces (via CreateDeploy's latest-pending slot) any queued deploy the push
// supersedes, without preempting the release already executing.
func TestWebhookPushOpensDeployRow(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/app"
	app := autoDeployApp("api", repo)
	app.Generation = 4
	// Both labels: the app-id alone names a public id; only managed-by proves it
	// is a Postgres source row (managedAppID's own gate).
	app.Labels = map[string]string{
		core.LabelAppID:      "srv-x1",
		store.LabelManagedBy: store.ManagedByValue,
	}
	st := &recordingStore{}
	svc, cl := newService(st, app)
	h := &GitWebhook{Svc: svc, Secret: secret}

	body, err := json.Marshal(map[string]any{
		"ref":        "refs/heads/main",
		"after":      "3597548b2424b9dee1ce4b07820b0f8cb007f747",
		"repository": map[string]string{"clone_url": repo},
		"commits":    []map[string]any{{"modified": []string{"main.go"}}},
		"head_commit": map[string]string{
			"id":        "3597548b2424b9dee1ce4b07820b0f8cb007f747",
			"message":   "fix(open-ledger): isolate registry from client bundle",
			"timestamp": "2026-07-19T06:58:44Z",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := postPush(t, h, secret, "push", body); rec.Code != http.StatusOK {
		t.Fatalf("push => %d: %s", rec.Code, rec.Body)
	}

	if len(st.deployCalls) != 1 {
		t.Fatalf("deploy rows opened = %d, want exactly 1: %+v", len(st.deployCalls), st.deployCalls)
	}
	d := st.deployCalls[0]
	if d.AppID != "srv-x1" || d.Trigger != store.TriggerNewCommit {
		t.Errorf("deploy row = {AppID:%s Trigger:%s}, want {srv-x1 new_commit}", d.AppID, d.Trigger)
	}
	if d.Generation != 5 {
		t.Errorf("deploy row generation = %d, want previous generation + 1 = 5", d.Generation)
	}
	if d.Commit != "3597548b2424b9dee1ce4b07820b0f8cb007f747" || d.CommitMessage != "fix(open-ledger): isolate registry from client bundle" {
		t.Errorf("deploy row commit = {%s %q}, want the pushed head commit", d.Commit, d.CommitMessage)
	}
	a := getApp(t, cl, "api")
	if got := a.Annotations[appv1alpha1.AnnotationReleaseGeneration]; got != "5" {
		t.Errorf("release-generation annotation = %q, want 5", got)
	}
	if a.Spec.RestartedAt == "" {
		t.Error("push must still bump restartedAt")
	}
}

func TestWebhookPushAfterRepoRollbackClearsImageOverride(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/app"
	app := autoDeployApp("api", repo)
	app.Spec.Image = "zot.test/api:gen-4" // exact-image override left by Rollback
	app.Labels = map[string]string{
		core.LabelAppID:      "srv-x1",
		store.LabelManagedBy: store.ManagedByValue,
	}
	st := &recordingStore{}
	svc, cl := newService(st, app)
	h := &GitWebhook{Svc: svc, Secret: secret}

	body := pushBody(t, repo, "refs/heads/main")
	if rec := postPush(t, h, secret, "push", body); rec.Code != http.StatusOK {
		t.Fatalf("push => %d: %s", rec.Code, rec.Body)
	}
	if len(st.imageCalls) != 1 || st.imageCalls[0].id != "srv-x1" || st.imageCalls[0].image != "" {
		t.Fatalf("stored image clears = %+v, want one empty-image write for srv-x1", st.imageCalls)
	}
	if got := getApp(t, cl, "api").Spec.Image; got != "" {
		t.Errorf("spec.image after push = %q, want cleared", got)
	}
	if len(st.deployCalls) != 1 || st.deployCalls[0].Image != "" {
		t.Fatalf("source deploy rows = %+v, want one row with empty image", st.deployCalls)
	}
}

// TestWebhookPushCROnlyAppSkipsDeployRow: a hand-applied App (no bex.co/app-id
// label) keeps the plain restartedAt bump — redeployed, no row, no error.
func TestWebhookPushCROnlyAppSkipsDeployRow(t *testing.T) {
	const repo = "https://github.com/x/app"
	st := &recordingStore{}
	svc, cl := newService(st, autoDeployApp("api", repo))
	h := &GitWebhook{Svc: svc, Secret: "shh"}

	redeployed, _, err := h.redeployMatching(context.Background(), newPush(repo, []string{"main.go"}), "main", "")
	if err != nil {
		t.Fatalf("redeployMatching: %v", err)
	}
	if !contains(redeployed, "api") {
		t.Fatalf("CR-only app must still redeploy: %v", redeployed)
	}
	if len(st.deployCalls) != 0 {
		t.Errorf("CR-only app must open no deploy row: %+v", st.deployCalls)
	}
	if getApp(t, cl, "api").Spec.RestartedAt == "" {
		t.Error("CR-only app must still bump restartedAt")
	}
}

// TestPushEventCommitInfo pins the wire→provenance lift: head_commit becomes
// the deploy row's CommitInfo, degrading field-by-field — absent head_commit
// is the zero value (the resolver fallback kicks in), a bad timestamp keeps
// hash+message and just drops authorAt. Offsets normalize to UTC.
func TestPushEventCommitInfo(t *testing.T) {
	var ev pushEvent
	payload := `{"ref":"refs/heads/main","head_commit":{"id":"abc123","message":"automated blog post","timestamp":"2026-07-19T08:58:44+02:00"}}`
	payload = `{"ref":"refs/heads/main","after":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","head_commit":{"id":"abc123","message":"automated blog post","timestamp":"2026-07-19T08:58:44+02:00"}}`
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		t.Fatal(err)
	}
	info := ev.commitInfo()
	if info.Hash != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || info.Message != "automated blog post" {
		t.Errorf("commitInfo = {%s %q}, want the head commit", info.Hash, info.Message)
	}
	want := time.Date(2026, 7, 19, 6, 58, 44, 0, time.UTC)
	if info.AuthorAt == nil || !info.AuthorAt.Equal(want) {
		t.Errorf("authorAt = %v, want %v", info.AuthorAt, want)
	}

	if got := (pushEvent{}).commitInfo(); got.Hash != "" || got.Message != "" || got.AuthorAt != nil {
		t.Errorf("absent head_commit => zero CommitInfo, got %+v", got)
	}

	ev.HeadCommit.Timestamp = "not-a-time"
	info = ev.commitInfo()
	if info.Hash != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || info.AuthorAt != nil {
		t.Errorf("bad timestamp must keep hash and drop authorAt, got %+v", info)
	}
}
