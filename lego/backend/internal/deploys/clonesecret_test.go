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

package deploys

import (
	"context"
	"errors"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// clonesecret_test.go pins the trigger-time clone-credential refresh: GitHub
// App installation tokens live one hour, so every deploy-initiating seam must
// remint — found live on prod 2026-07-17, where every trigger=api build of a
// private repo ran with the previous deploy's expired token and failed at git
// clone with the misleading "could not read Username" (git's 401-then-prompt
// fallback), while webhook-triggered siblings built fine.

// fakeCloneSecreter records the mint request and returns a canned Secret name.
type fakeCloneSecreter struct {
	name string
	err  error

	gotNamespace, gotApp, gotWorkspace, gotRepo string
	calls                                       int
}

func (f *fakeCloneSecreter) EnsureCloneSecret(_ context.Context, namespace, appName, workspaceID, repo string) (string, error) {
	f.calls++
	f.gotNamespace, f.gotApp, f.gotWorkspace, f.gotRepo = namespace, appName, workspaceID, repo
	return f.name, f.err
}

func TestTriggerRefreshesCloneSecret(t *testing.T) {
	ds := newFakeStore()
	minter := &fakeCloneSecreter{name: "web-clone"}
	svc, cl := newService(ds, repoApp("web", "srv-1", "main"))
	svc.CloneSecrets = minter

	if _, err := svc.Trigger(context.Background(), "web", TriggerParams{}); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	// The mint request names the App's own namespace/name, its OWN workspace
	// (tenant label — a deploy hook carries no caller identity), and the repo.
	if minter.calls != 1 || minter.gotNamespace != "default" || minter.gotApp != "web" ||
		minter.gotWorkspace != "tea-acme" || minter.gotRepo != "https://github.com/acme/web" {
		t.Errorf("mint request = %+v, want one call with the App's namespace/name/tenant/repo", minter)
	}
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &a); err != nil {
		t.Fatalf("get app: %v", err)
	}
	if a.Spec.CloneSecret != "web-clone" {
		t.Errorf("spec.cloneSecret = %q, want the freshly minted Secret riding the trigger patch", a.Spec.CloneSecret)
	}
}

// A mint failure fails the trigger loudly BEFORE any generation bump or deploy
// row — a private repo must never build with a stale token (the create path's
// rule, apps/clonesecret.go).
func TestTriggerFailsWhenCloneMintFails(t *testing.T) {
	ds := newFakeStore()
	minter := &fakeCloneSecreter{err: errors.New("github is down")}
	svc, cl := newService(ds, repoApp("web", "srv-1", "main"))
	svc.CloneSecrets = minter

	if _, err := svc.Trigger(context.Background(), "web", TriggerParams{}); err == nil {
		t.Fatal("Trigger must fail when the clone mint fails, got nil")
	}
	if rows, _ := ds.ListDeploys(context.Background(), "srv-1", store.DeployFilter{}); len(rows) != 0 {
		t.Errorf("a refused trigger must open no deploy row, got %d", len(rows))
	}
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &a); err != nil {
		t.Fatalf("get app: %v", err)
	}
	if a.Spec.RestartedAt != "" {
		t.Errorf("a refused trigger must not bump RestartedAt, got %q", a.Spec.RestartedAt)
	}
}

// An empty mint result (public/unconnected repo, or GitHub off) leaves the
// existing spec.cloneSecret untouched rather than clearing a hand-managed one.
func TestTriggerKeepsCloneSecretWhenMintDeclines(t *testing.T) {
	ds := newFakeStore()
	minter := &fakeCloneSecreter{name: ""}
	app := repoApp("web", "srv-1", "main")
	app.Spec.CloneSecret = "hand-managed-clone"
	svc, cl := newService(ds, app)
	svc.CloneSecrets = minter

	if _, err := svc.Trigger(context.Background(), "web", TriggerParams{}); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &a); err != nil {
		t.Fatalf("get app: %v", err)
	}
	if a.Spec.CloneSecret != "hand-managed-clone" {
		t.Errorf("spec.cloneSecret = %q, a declined mint must leave the existing Secret alone", a.Spec.CloneSecret)
	}
}

// Image-backed apps have nothing to clone — the minter must not be consulted.
func TestTriggerSkipsCloneMintForImageBackedApp(t *testing.T) {
	ds := newFakeStore()
	minter := &fakeCloneSecreter{name: "unused"}
	svc, _ := newService(ds, sampleApp("web", "srv-1")) // image-backed
	svc.CloneSecrets = minter

	if _, err := svc.Trigger(context.Background(), "web", TriggerParams{}); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if minter.calls != 0 {
		t.Errorf("minter consulted %d times for an image-backed app, want 0", minter.calls)
	}
}
