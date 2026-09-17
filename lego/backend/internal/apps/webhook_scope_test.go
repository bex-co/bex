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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeInstallations is a test InstallationResolver: a static
// installation→workspaces map (ADR078 §2 — one installation may serve several).
type fakeInstallations map[int64][]string

func (f fakeInstallations) WorkspacesForInstallation(_ context.Context, id int64) ([]string, error) {
	return f[id], nil
}

// erroringInstallations models a resolver whose lookup fails (store down). The
// fail-closed rule treats it exactly like an unknown installation.
type erroringInstallations struct{}

func (erroringInstallations) WorkspacesForInstallation(_ context.Context, _ int64) ([]string, error) {
	return nil, errors.New("control-plane store unavailable")
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

// TestScopesForConfinesGitHubAppDelivery pins the confinement decision (codex
// #7, restated for N:N in ADR078 §4a): an app-signed delivery is scoped to every
// workspace that PROVED a binding; an unbound installation acts on nothing; the
// manual key and the unwired-resolver single-tenant case stay global.
func TestScopesForConfinesGitHubAppDelivery(t *testing.T) {
	h := &GitWebhook{Installations: fakeInstallations{7: {"tea-a"}}}
	if scopes, proceed := h.scopesFor(context.Background(), keyGitHubApp, 7); !proceed || len(scopes) != 1 || scopes[0] != "tea-a" {
		t.Errorf("bound app delivery => (%v,%v), want ([tea-a],true)", scopes, proceed)
	}
	if scopes, proceed := h.scopesFor(context.Background(), keyGitHubApp, 99); proceed || len(scopes) != 0 {
		t.Errorf("unbound app delivery => (%v,%v), want (nil,false)", scopes, proceed)
	}
	if scopes, proceed := h.scopesFor(context.Background(), keyManual, 7); !proceed || len(scopes) != 1 || scopes[0] != "" {
		t.Errorf(`manual delivery => (%v,%v), want ([""],true)`, scopes, proceed)
	}
	bare := &GitWebhook{} // resolver unwired, single-tenant => pre-#7 global behavior
	if scopes, proceed := bare.scopesFor(context.Background(), keyGitHubApp, 7); !proceed || len(scopes) != 1 || scopes[0] != "" {
		t.Errorf(`app delivery without resolver => (%v,%v), want ([""],true)`, scopes, proceed)
	}
}

// TestScopesForFansOutToEveryProvedBinding is the N:N half (ADR078 §2/§4a): one
// installation bound by two workspaces yields BOTH scopes, and an empty workspace
// id is never emitted — that would silently mean "global".
func TestScopesForFansOutToEveryProvedBinding(t *testing.T) {
	h := &GitWebhook{Multitenant: true, Installations: fakeInstallations{7: {"tea-a", "tea-b"}}}
	scopes, proceed := h.scopesFor(context.Background(), keyGitHubApp, 7)
	if !proceed || len(scopes) != 2 {
		t.Fatalf("shared installation => (%v,%v), want both bindings", scopes, proceed)
	}
	for _, s := range scopes {
		if s == "" {
			t.Fatal("an empty scope would be a GLOBAL match; it must never be emitted")
		}
	}

	// A resolver that returns only empty ids must fail closed, not emit a global.
	empty := &GitWebhook{Multitenant: true, Installations: fakeInstallations{7: {""}}}
	if scopes, proceed := empty.scopesFor(context.Background(), keyGitHubApp, 7); proceed || len(scopes) != 0 {
		t.Fatalf("empty-id binding => (%v,%v), want (nil,false)", scopes, proceed)
	}
}

// TestScopesForFailsClosedInMultitenantPartialConfig pins codex-security
// round-6 #9: in multitenant operation an app-signed delivery must NEVER fall
// back to the manual key's global scope. Every unresolvable case — partial
// configuration, missing installation id, unknown installation, lookup error —
// exits with proceed=false. N:N changes how many non-empty scopes there can be,
// never what an unresolvable one means.
func TestScopesForFailsClosedInMultitenantPartialConfig(t *testing.T) {
	// Resolver unwired (the partial deployment configuration).
	unwired := &GitWebhook{Multitenant: true}
	if scopes, proceed := unwired.scopesFor(context.Background(), keyGitHubApp, 7); proceed || len(scopes) != 0 {
		t.Errorf("multitenant app delivery without resolver => (%v,%v), want (nil,false)", scopes, proceed)
	}
	// Resolver wired but the signed payload carries no installation id.
	wired := &GitWebhook{Multitenant: true, Installations: fakeInstallations{7: {"tea-a"}}}
	if scopes, proceed := wired.scopesFor(context.Background(), keyGitHubApp, 0); proceed || len(scopes) != 0 {
		t.Errorf("multitenant app delivery without installation id => (%v,%v), want (nil,false)", scopes, proceed)
	}
	// Unknown installation.
	if scopes, proceed := wired.scopesFor(context.Background(), keyGitHubApp, 99); proceed || len(scopes) != 0 {
		t.Errorf("unknown installation => (%v,%v), want (nil,false)", scopes, proceed)
	}
	// Lookup error — indistinguishable from unknown, by design.
	broken := &GitWebhook{Multitenant: true, Installations: erroringInstallations{}}
	if scopes, proceed := broken.scopesFor(context.Background(), keyGitHubApp, 7); proceed || len(scopes) != 0 {
		t.Errorf("resolver error => (%v,%v), want (nil,false)", scopes, proceed)
	}
	// The confined path still works.
	if scopes, proceed := wired.scopesFor(context.Background(), keyGitHubApp, 7); !proceed || len(scopes) != 1 || scopes[0] != "tea-a" {
		t.Errorf("multitenant bound app delivery => (%v,%v), want ([tea-a],true)", scopes, proceed)
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
	h := &GitWebhook{Svc: svc, GitHubSecret: secret, Installations: fakeInstallations{7: {"tea-a"}}}

	ev := newPush(repo, []string{"main.go"})
	ev.After = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
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
