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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m38: resource-independent Blueprint auto-sync — discovery, path gate,
// durable enqueue before ack, and no-target early return without replay claim.

func TestBlueprintManifestPathChanged(t *testing.T) {
	cases := []struct {
		name    string
		bpPath  string
		changed []string
		want    bool
	}{
		{"empty paths fail open", "render.yaml", nil, true},
		{"exact match", "render.yaml", []string{"README.md", "render.yaml"}, true},
		{"custom path match", "infra/bex.yml", []string{"infra/bex.yml"}, true},
		{"unrelated complete evidence skips", "render.yaml", []string{"README.md", "src/main.go"}, false},
		{"path.Clean normalizes", "./render.yaml", []string{"render.yaml"}, true},
		{"empty bpPath defaults to render.yaml", "", []string{"render.yaml"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := blueprintManifestPathChanged(c.bpPath, c.changed); got != c.want {
				t.Errorf("blueprintManifestPathChanged(%q, %v) = %v, want %v", c.bpPath, c.changed, got, c.want)
			}
		})
	}
}

func TestWebhookSelectsBlueprintWithoutApp(t *testing.T) {
	secret := "shh"
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-db-only", TenantID: "tea-a", Name: "db-only",
		Repo: "https://github.com/acme/infra.git", Branch: "main",
		Path: "render.yaml", AutoSync: true, Status: store.BlueprintStatusInSync,
	})
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default"},
		Blueprints: fs,
	}
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Svc: svc, Secret: secret, Replays: replays}

	body := pushBodyWithPaths("https://github.com/acme/infra.git", "main",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", []string{"render.yaml"})
	rec := postSignedPush(t, h, secret, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	if replays.claimed() != 1 {
		t.Fatalf("replay claims = %d, want 1 (blueprint-only still claims)", replays.claimed())
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if len(fs.autoSyncIntents) != 1 {
		t.Fatalf("intents = %d, want 1", len(fs.autoSyncIntents))
	}
	for _, intent := range fs.autoSyncIntents {
		if intent.BlueprintID != "blp-db-only" {
			t.Errorf("blueprint = %q, want blp-db-only", intent.BlueprintID)
		}
		if intent.CommitSHA != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
			t.Errorf("commit = %q", intent.CommitSHA)
		}
		if intent.Path != "render.yaml" {
			t.Errorf("path = %q", intent.Path)
		}
	}
}

func TestWebhookExcludesWrongBranchAndAutoSyncOff(t *testing.T) {
	secret := "shh"
	fs := newFakeBlueprintStore(
		store.Blueprint{
			ID: "blp-wrong-branch", TenantID: "tea-a", Name: "x",
			Repo: "https://github.com/acme/infra", Branch: "develop",
			Path: "render.yaml", AutoSync: true, Status: store.BlueprintStatusInSync,
		},
		store.Blueprint{
			ID: "blp-autosync-off", TenantID: "tea-a", Name: "y",
			Repo: "https://github.com/acme/infra", Branch: "main",
			Path: "render.yaml", AutoSync: false, Status: store.BlueprintStatusPaused,
		},
		store.Blueprint{
			ID: "blp-disconnected", TenantID: "tea-a", Name: "z",
			Repo: "https://github.com/acme/infra", Branch: "main",
			Path: "render.yaml", AutoSync: true, Status: "disconnected",
		},
	)
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}, Blueprints: fs}
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Svc: svc, Secret: secret, Replays: replays}

	body := pushBodyWithPaths("https://github.com/acme/infra.git", "main",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", []string{"render.yaml"})
	rec := postSignedPush(t, h, secret, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if replays.claimed() != 0 {
		t.Fatalf("replay claims = %d, want 0 (no targets)", replays.claimed())
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if len(fs.autoSyncIntents) != 0 {
		t.Fatalf("intents = %d, want 0", len(fs.autoSyncIntents))
	}
}

func TestWebhookPathFilterSkipsUnrelatedCompleteEvidence(t *testing.T) {
	secret := "shh"
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Name: "app",
		Repo: "https://github.com/acme/app", Branch: "main",
		Path: "infra/render.yaml", AutoSync: true, Status: store.BlueprintStatusInSync,
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}, Blueprints: fs}
	h := &GitWebhook{Svc: svc, Secret: secret, Replays: &fakeReplayGuard{}}

	body := pushBodyWithPaths("https://github.com/acme/app.git", "main",
		"cccccccccccccccccccccccccccccccccccccccc", []string{"README.md", "src/main.go"})
	rec := postSignedPush(t, h, secret, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	fs.mu.Lock()
	n := len(fs.autoSyncIntents)
	fs.mu.Unlock()
	if n != 0 {
		t.Fatalf("intents = %d, want 0 for unrelated complete paths", n)
	}
}

func TestWebhookEmptyPathsStillEnqueueBlueprint(t *testing.T) {
	secret := "shh"
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Name: "app",
		Repo: "https://github.com/acme/app", Branch: "main",
		Path: "render.yaml", AutoSync: true, Status: store.BlueprintStatusInSync,
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}, Blueprints: fs}
	h := &GitWebhook{Svc: svc, Secret: secret, Replays: &fakeReplayGuard{}}

	// No commits array → empty changedPaths → fail open.
	body := `{"ref":"refs/heads/main","after":"dddddddddddddddddddddddddddddddddddddddd","repository":{"clone_url":"https://github.com/acme/app.git","html_url":"https://github.com/acme/app"},"commits":[]}`
	rec := postSignedPush(t, h, secret, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	fs.mu.Lock()
	n := len(fs.autoSyncIntents)
	fs.mu.Unlock()
	if n != 1 {
		t.Fatalf("intents = %d, want 1 (incomplete path evidence must not suppress)", n)
	}
}

func TestWebhookNoTargetsSkipsReplayClaim(t *testing.T) {
	secret := "shh"
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default"},
		Blueprints: newFakeBlueprintStore(),
	}
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Svc: svc, Secret: secret, Replays: replays}

	body := pushBodyWithPaths("https://github.com/nobody/nothing.git", "main",
		"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", []string{"render.yaml"})
	rec := postSignedPush(t, h, secret, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if replays.claimed() != 0 {
		t.Fatalf("replay claims = %d, want 0", replays.claimed())
	}
}

func TestWebhookEnqueueFailureReturns500AndReleasesClaim(t *testing.T) {
	secret := "shh"
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Name: "app",
		Repo: "https://github.com/acme/app", Branch: "main",
		Path: "render.yaml", AutoSync: true, Status: store.BlueprintStatusInSync,
	})
	fs.enqueueIntentErr = errors.New("store down")
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}, Blueprints: fs}
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Svc: svc, Secret: secret, Replays: replays}

	body := pushBodyWithPaths("https://github.com/acme/app.git", "main",
		"ffffffffffffffffffffffffffffffffffffffff", []string{"render.yaml"})
	rec := postSignedPush(t, h, secret, body)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if replays.claimed() != 0 {
		t.Fatalf("replay claims = %d after 5xx, want 0 (released for retry)", replays.claimed())
	}
}

func TestWebhookIntentDedupeOnReplay(t *testing.T) {
	secret := "shh"
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Name: "app",
		Repo: "https://github.com/acme/app", Branch: "main",
		Path: "render.yaml", AutoSync: true, Status: store.BlueprintStatusInSync,
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}, Blueprints: fs}
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Svc: svc, Secret: secret, Replays: replays}

	body := pushBodyWithPaths("https://github.com/acme/app.git", "main",
		"1111111111111111111111111111111111111111", []string{"render.yaml"})
	if rec := postSignedPush(t, h, secret, body); rec.Code != http.StatusOK {
		t.Fatalf("first: %d", rec.Code)
	}
	// Second delivery: replay claim rejects before enqueue; intents stay at 1.
	if rec := postSignedPush(t, h, secret, body); rec.Code != http.StatusOK {
		t.Fatalf("second: %d", rec.Code)
	}
	fs.mu.Lock()
	n := len(fs.autoSyncIntents)
	fs.mu.Unlock()
	if n != 1 {
		t.Fatalf("intents = %d, want 1 after duplicate delivery", n)
	}
}

func TestWebhookBranchDeleteSkipsBlueprintIntents(t *testing.T) {
	secret := "shh"
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelTenant: "tea-a"}
	app.Spec.Repo = "https://github.com/acme/app.git"
	app.Spec.Branch = "main"
	app.Spec.AutoDeploy = true
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Name: "app",
		Repo: "https://github.com/acme/app", Branch: "main",
		Path: "render.yaml", AutoSync: true, Status: store.BlueprintStatusInSync,
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(app), Namespace: "default"}, Blueprints: fs}
	h := &GitWebhook{Svc: svc, Secret: secret, Replays: &fakeReplayGuard{}}

	zero := strings.Repeat("0", 40)
	body := `{"ref":"refs/heads/main","after":"` + zero + `","deleted":true,"repository":{"clone_url":"https://github.com/acme/app.git"},"commits":[]}`
	rec := postSignedPush(t, h, secret, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	fs.mu.Lock()
	n := len(fs.autoSyncIntents)
	fs.mu.Unlock()
	if n != 0 {
		t.Fatalf("intents = %d, want 0 on branch-delete", n)
	}
}

func TestFakeEnqueueBlueprintAutoSyncIntentDedupe(t *testing.T) {
	fs := newFakeBlueprintStore()
	intent := store.BlueprintAutoSyncIntent{
		TenantID: "tea-a", BlueprintID: "blp-1",
		DeliveryDigest: "digest-1", CommitSHA: strings.Repeat("a", 40), Path: "render.yaml",
	}
	ok, err := fs.EnqueueBlueprintAutoSyncIntent(context.Background(), intent)
	if err != nil || !ok {
		t.Fatalf("first insert: ok=%v err=%v", ok, err)
	}
	ok, err = fs.EnqueueBlueprintAutoSyncIntent(context.Background(), intent)
	if err != nil || ok {
		t.Fatalf("duplicate: ok=%v err=%v, want ok=false", ok, err)
	}
	if len(fs.autoSyncIntents) != 1 {
		t.Fatalf("stored = %d, want 1", len(fs.autoSyncIntents))
	}
}

func TestBlueprintAutoSyncWorkerPinsCommit(t *testing.T) {
	commit := "2222222222222222222222222222222222222222"
	manifest := `
services:
  - name: web
    type: web
    runtime: image
    image: {url: pinned:latest}
`
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Name: "app",
		Repo: "https://github.com/acme/app", Branch: "main",
		Path: CanonicalBlueprintFilename, Manifest: "stale",
		Status: store.BlueprintStatusInSync, AutoSync: true,
	})
	if _, err := fs.EnqueueBlueprintAutoSyncIntent(context.Background(), store.BlueprintAutoSyncIntent{
		TenantID: "tea-a", BlueprintID: "blp-1",
		DeliveryDigest: "d1", CommitSHA: commit, Path: CanonicalBlueprintFilename,
	}); err != nil {
		t.Fatal(err)
	}
	fetcher := fakeBlueprintFetcher{contents: manifest, sha: commit}
	svc := &Service{
		Base:            &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"u": "tea-a"}},
		Blueprints:      fs,
		DomainOwnership: allowDomainOwnership{},
		GitFetcher:      fetcher,
	}
	w := &BlueprintAutoSyncWorker{Svc: svc}
	if err := w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	var terminal store.BlueprintAutoSyncIntent
	for _, intent := range fs.autoSyncIntents {
		terminal = intent
	}
	if terminal.State != store.BlueprintAutoSyncIntentCompleted {
		t.Fatalf("intent state = %q, want completed (sync err may have failed)", terminal.State)
	}
	var a appv1alpha1.App
	key := client.ObjectKey{Namespace: "tea-a", Name: core.CRName("tea-a", "web")}
	if err := svc.Client.Get(context.Background(), key, &a); err != nil {
		t.Fatalf("expected synced app: %v", err)
	}
}

// --- helpers ---

func pushBodyWithPaths(cloneURL, branch, after string, paths []string) string {
	type commit struct {
		Modified []string `json:"modified"`
	}
	ev := map[string]any{
		"ref":   "refs/heads/" + branch,
		"after": after,
		"repository": map[string]string{
			"clone_url": cloneURL,
			"html_url":  strings.TrimSuffix(cloneURL, ".git"),
		},
		"head_commit": map[string]string{"id": after, "message": "msg"},
		"commits":     []commit{{Modified: paths}},
	}
	b, _ := json.Marshal(ev)
	return string(b)
}

func postSignedPush(t *testing.T, h *GitWebhook, secret, body string) *httptest.ResponseRecorder {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/git", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "del-"+hex.EncodeToString(mac.Sum(nil)[:8]))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
