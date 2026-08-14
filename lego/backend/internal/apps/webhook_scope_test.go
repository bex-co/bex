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

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeInstallations is a test InstallationResolver: a static installation→workspace map.
type fakeInstallations map[int64]string

func (f fakeInstallations) WorkspaceForInstallation(_ context.Context, id int64) (string, bool, error) {
	ws, ok := f[id]
	return ws, ok, nil
}

// TestVerifyKeyIdentifiesTheMatchingKey pins codex #7's precondition: verification
// preserves WHICH configured key accepted the delivery (so the GitHub App path can
// be confined) while still 401ing an unsigned/forged one.
func TestVerifyKeyIdentifiesTheMatchingKey(t *testing.T) {
	h := &GitWebhook{Secret: "manual-secret", GitHubSecret: "app-secret"}
	body := []byte(`{"ref":"refs/heads/main"}`)
	if got := h.verifyKey(sign("manual-secret", body), body); got != keyManual {
		t.Errorf("manual signature => %v, want keyManual", got)
	}
	if got := h.verifyKey(sign("app-secret", body), body); got != keyGitHubApp {
		t.Errorf("app signature => %v, want keyGitHubApp", got)
	}
	if got := h.verifyKey(sign("wrong", body), body); got != keyNone {
		t.Errorf("bad signature => %v, want keyNone", got)
	}
}

// TestScopeForConfinesGitHubAppDelivery pins the confinement decision (codex #7):
// only an app-signed delivery with a bound installation is scoped to a workspace;
// an unbound installation acts on nothing; the manual key and the unwired-resolver
// case stay global.
func TestScopeForConfinesGitHubAppDelivery(t *testing.T) {
	h := &GitWebhook{Installations: fakeInstallations{7: "tea-a"}}
	if scope, proceed := h.scopeFor(context.Background(), keyGitHubApp, 7); !proceed || scope != "tea-a" {
		t.Errorf("bound app delivery => (%q,%v), want (tea-a,true)", scope, proceed)
	}
	if scope, proceed := h.scopeFor(context.Background(), keyGitHubApp, 99); proceed || scope != "" {
		t.Errorf(`unbound app delivery => (%q,%v), want ("",false)`, scope, proceed)
	}
	if scope, proceed := h.scopeFor(context.Background(), keyManual, 7); !proceed || scope != "" {
		t.Errorf(`manual delivery => (%q,%v), want ("",true)`, scope, proceed)
	}
	bare := &GitWebhook{} // resolver unwired, single-tenant => pre-#7 global behavior
	if scope, proceed := bare.scopeFor(context.Background(), keyGitHubApp, 7); !proceed || scope != "" {
		t.Errorf(`app delivery without resolver => (%q,%v), want ("",true)`, scope, proceed)
	}
}

// TestScopeForFailsClosedInMultitenantPartialConfig pins codex-security
// round-6 #9: in multitenant operation an app-signed delivery must NEVER fall
// back to the manual key's global scope — a partial configuration (GitHub
// webhook secret + store, no GitHub service) or a payload with no installation
// id acts on nothing instead of scanning and mutating every workspace's Apps.
func TestScopeForFailsClosedInMultitenantPartialConfig(t *testing.T) {
	// Resolver unwired (the partial deployment configuration).
	unwired := &GitWebhook{Multitenant: true}
	if scope, proceed := unwired.scopeFor(context.Background(), keyGitHubApp, 7); proceed || scope != "" {
		t.Errorf(`multitenant app delivery without resolver => (%q,%v), want ("",false)`, scope, proceed)
	}
	// Resolver wired but the signed payload carries no installation id.
	wired := &GitWebhook{Multitenant: true, Installations: fakeInstallations{7: "tea-a"}}
	if scope, proceed := wired.scopeFor(context.Background(), keyGitHubApp, 0); proceed || scope != "" {
		t.Errorf(`multitenant app delivery without installation id => (%q,%v), want ("",false)`, scope, proceed)
	}
	// The confined path still works.
	if scope, proceed := wired.scopeFor(context.Background(), keyGitHubApp, 7); !proceed || scope != "tea-a" {
		t.Errorf("multitenant bound app delivery => (%q,%v), want (tea-a,true)", scope, proceed)
	}
}

// TestRedeployMatchingConfinesToWorkspace proves the scope actually filters: a
// non-empty scope redeploys only that workspace's Apps; an empty scope (the manual
// key) redeploys every repo match, as before.
func TestRedeployMatchingConfinesToWorkspace(t *testing.T) {
	const repo = "https://github.com/octo/app"
	appA := &appv1alpha1.App{}
	appA.Name, appA.Namespace = "web", "default"
	appA.Labels = map[string]string{core.LabelTenant: "tea-a"}
	appA.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", AutoDeploy: true}
	appB := &appv1alpha1.App{}
	appB.Name, appB.Namespace = "api", "default"
	appB.Labels = map[string]string{core.LabelTenant: "tea-b"}
	appB.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", AutoDeploy: true}

	svc, _ := newService(nil, appA, appB)
	h := &GitWebhook{Svc: svc, Secret: "shh"}
	ev := newPush(repo, []string{"main.go"})

	got, _, err := h.redeployMatching(context.Background(), ev, "main", "tea-a")
	if err != nil {
		t.Fatalf("redeployMatching: %v", err)
	}
	if !contains(got, "web") || contains(got, "api") {
		t.Errorf("scope=tea-a redeployed %v, want only [web]", got)
	}

	all, _, err := h.redeployMatching(context.Background(), ev, "main", "")
	if err != nil {
		t.Fatalf("redeployMatching global: %v", err)
	}
	if !contains(all, "web") || !contains(all, "api") {
		t.Errorf("global scope redeployed %v, want [web api]", all)
	}
}

// TestServeHTTPConfinesGitHubAppPush is the end-to-end codex #7 proof: an
// app-signed push whose installation is bound to tea-a redeploys tea-a's App and
// leaves another workspace's App untouched, even though both track the same repo.
func TestServeHTTPConfinesGitHubAppPush(t *testing.T) {
	const repo = "https://github.com/octo/app"
	const secret = "app-secret"
	appA := &appv1alpha1.App{}
	appA.Name, appA.Namespace = "web", "default"
	appA.Labels = map[string]string{core.LabelTenant: "tea-a"}
	appA.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", AutoDeploy: true}
	appB := &appv1alpha1.App{}
	appB.Name, appB.Namespace = "api", "default"
	appB.Labels = map[string]string{core.LabelTenant: "tea-b"}
	appB.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", AutoDeploy: true}

	svc, cl := newService(nil, appA, appB)
	h := &GitWebhook{Svc: svc, GitHubSecret: secret, Installations: fakeInstallations{7: "tea-a"}}

	ev := newPush(repo, []string{"main.go"})
	ev.After = "abc123"
	ev.Installation.ID = 7
	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", sign(secret, body))
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("=> 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if getApp(t, cl, "web").Spec.RestartedAt == "" {
		t.Error("tea-a App (installation 7's workspace) should have redeployed")
	}
	if getApp(t, cl, "api").Spec.RestartedAt != "" {
		t.Error("tea-b App must NOT redeploy from another workspace's installation push (codex #7)")
	}
}
