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
	"encoding/json"
	"errors"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// commit_test.go covers w9/001: a trigger resolves its ref to the exact
// commit through the CommitResolver seam and stamps it onto the deploy row;
// rollback copies the target's commit instead of re-resolving; the REST shape
// nests Render's commit object only when a commit was actually resolved.

// fakeResolver records the resolution request and returns a canned answer.
type fakeResolver struct {
	commit store.CommitInfo
	ok     bool
	err    error

	gotWorkspace, gotRepo, gotRef string
	calls                         int
}

func (f *fakeResolver) ResolveCommit(_ context.Context, workspaceID, repoURL, ref string) (store.CommitInfo, bool, error) {
	f.calls++
	f.gotWorkspace, f.gotRepo, f.gotRef = workspaceID, repoURL, ref
	return f.commit, f.ok, f.err
}

// repoApp is a store-managed, repo-backed App carrying a tenant label — the
// shape whose deploys have a commit to resolve.
func repoApp(name, storeID, branch string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "default",
			Labels: map[string]string{store.LabelAppID: storeID, core.LabelTenant: "tea-acme"},
		},
		Spec: appv1alpha1.AppSpec{Repo: "https://github.com/acme/web", Branch: branch},
	}
}

func TestTriggerStampsResolvedCommit(t *testing.T) {
	ds := newFakeStore()
	resolver := &fakeResolver{commit: store.CommitInfo{Hash: "abc1234def5678", Message: "fix: header"}, ok: true}
	svc, _ := newService(ds, repoApp("web", "srv-1", "dev"))
	svc.Commits = resolver

	d, err := svc.Trigger(context.Background(), "web", TriggerParams{})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if d.CommitID != "abc1234def5678" || d.CommitMessage != "fix: header" {
		t.Errorf("triggered deploy commit = %q/%q, want the resolved commit", d.CommitID, d.CommitMessage)
	}
	// The resolution request names the App's OWN workspace (its tenant label)
	// and the branch, since no commitId pinned a ref.
	if resolver.gotWorkspace != "tea-acme" || resolver.gotRepo != "https://github.com/acme/web" || resolver.gotRef != "dev" {
		t.Errorf("resolved (%q, %q, %q), want (tea-acme, the repo, dev)",
			resolver.gotWorkspace, resolver.gotRepo, resolver.gotRef)
	}

	// An explicit commitId is the ref that gets resolved.
	if _, err := svc.Trigger(context.Background(), "web", TriggerParams{CommitID: "abc1234"}); err != nil {
		t.Fatalf("Trigger with commitId: %v", err)
	}
	if resolver.gotRef != "abc1234" {
		t.Errorf("resolved ref = %q, want the explicit commitId", resolver.gotRef)
	}
}

// TestTriggerCommitResolutionIsBestEffort: commit metadata is provenance —
// a resolver failure (or nothing to resolve) must never block the deploy.
func TestTriggerCommitResolutionIsBestEffort(t *testing.T) {
	ds := newFakeStore()
	resolver := &fakeResolver{err: errors.New("github is down")}
	svc, _ := newService(ds, repoApp("web", "srv-1", "main"))
	svc.Commits = resolver

	d, err := svc.Trigger(context.Background(), "web", TriggerParams{})
	if err != nil {
		t.Fatalf("Trigger must survive a resolver failure, got %v", err)
	}
	if d.CommitID != "" || d.CommitMessage != "" {
		t.Errorf("failed resolution must leave commit empty, got %q/%q", d.CommitID, d.CommitMessage)
	}
}

// TestTriggerSkipsResolutionForImageBackedApp: nothing to resolve without a
// repo — the resolver must not even be consulted.
func TestTriggerSkipsResolutionForImageBackedApp(t *testing.T) {
	ds := newFakeStore()
	resolver := &fakeResolver{commit: store.CommitInfo{Hash: "abc"}, ok: true}
	svc, _ := newService(ds, sampleApp("web", "srv-1")) // image-backed
	svc.Commits = resolver

	if _, err := svc.Trigger(context.Background(), "web", TriggerParams{}); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if resolver.calls != 0 {
		t.Errorf("resolver consulted %d times for an image-backed app, want 0", resolver.calls)
	}
}

// TestRollbackCopiesCommitFromTarget: a rollback re-runs what the target
// built, so its commit is copied from the target row, never re-resolved
// against a branch that has since moved.
func TestRollbackCopiesCommitFromTarget(t *testing.T) {
	ds := newFakeStore()
	target, _ := ds.CreateDeploy(context.Background(), "srv-1", "api", "web:v1", 1,
		store.CommitInfo{Hash: "abc1234def5678", Message: "fix: header"})
	if _, err := ds.CloseDeploy(context.Background(), target.ID, store.DeployLive, "web@sha256:aa"); err != nil {
		t.Fatalf("close target: %v", err)
	}
	// A resolver that would return a DIFFERENT commit — rollback must not use it.
	resolver := &fakeResolver{commit: store.CommitInfo{Hash: "newer000"}, ok: true}
	svc, _ := newService(ds, repoApp("web", "srv-1", "main"))
	svc.Commits = resolver

	rb, err := svc.Rollback(context.Background(), "web", target.ID)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rb.CommitID != "abc1234def5678" || rb.CommitMessage != "fix: header" {
		t.Errorf("rollback commit = %q/%q, want the target's own", rb.CommitID, rb.CommitMessage)
	}
	if resolver.calls != 0 {
		t.Errorf("rollback consulted the resolver %d times, want 0 (copy, don't re-resolve)", resolver.calls)
	}
}

// TestRenderDeployNestsCommitObject: REST/MCP emit Render's nested
// commit{id,message} only when a commit was resolved — omitted, not faked.
func TestRenderDeployNestsCommitObject(t *testing.T) {
	with, _ := json.Marshal(toRenderDeploy(DeployView{ID: "dep-1", CommitID: "abc123", CommitMessage: "feat: x"}))
	if !strings.Contains(string(with), `"commit":{"id":"abc123","message":"feat: x"}`) {
		t.Errorf("render deploy = %s, want the nested commit object", with)
	}
	without, _ := json.Marshal(toRenderDeploy(DeployView{ID: "dep-1"}))
	if strings.Contains(string(without), `"commit"`) {
		t.Errorf("render deploy = %s, want no commit key when unresolved", without)
	}
}
