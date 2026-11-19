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

package store

import (
	"context"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPGGitHubRepoIdentity pins w10/m490's row contract: a service and a
// Blueprint keep GitHub's repository id beside their repo, every write is a
// compare-and-set on the row's current repo, a source change to another
// repository forgets the old id, a move needs the id it names, and the gap
// listing is the rows the backfill still has to resolve.
func TestPGGitHubRepoIdentity(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	tenant, err := st.CreateWorkspace(ctx, "repo-identity-"+ids.New(ids.Workspace), PlanHobby, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
		_ = st.DeleteTenant(context.Background(), tenant.ID)
	})
	const oldName, newName = "https://github.com/octo/old-name", "https://github.com/octo/new-name"

	created, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "connected", Repo: oldName, GitHubRepoID: 42,
		Type: "web_service", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	public, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "public", Repo: "https://github.com/stranger/public",
		Type: "web_service", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	read := func(id string) App {
		t.Helper()
		a, err := st.GetApp(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	if got := read(created.ID).GitHubRepoID; got != 42 {
		t.Fatalf("created id = %d, want 42", got)
	}
	desired, err := st.ListDesiredApps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range desired {
		if d.ID == created.ID && d.GitHubRepoID != 42 {
			t.Errorf("projector reads id %d, want 42", d.GitHubRepoID)
		}
	}

	// A move names the id and the URL it moves from; a stale or id-less move
	// writes nothing, and a repeat is a no-op.
	for _, bad := range []struct {
		id   int64
		from string
	}{{99, oldName}, {42, newName}, {0, oldName}} {
		if moved, err := st.MoveAppRepo(ctx, created.ID, bad.id, bad.from, newName); err != nil || moved {
			t.Errorf("MoveAppRepo(id %d, from %s) = %v, %v; want no move", bad.id, bad.from, moved, err)
		}
	}
	if moved, err := st.MoveAppRepo(ctx, created.ID, 42, oldName, newName); err != nil || !moved {
		t.Fatalf("MoveAppRepo = %v, %v; want moved", moved, err)
	}
	if moved, _ := st.MoveAppRepo(ctx, created.ID, 42, oldName, newName); moved {
		t.Error("a repeated move must write nothing")
	}
	if got := read(created.ID); got.Repo != newName || got.GitHubRepoID != 42 {
		t.Errorf("after the move = %s id %d, want %s id 42", got.Repo, got.GitHubRepoID, newName)
	}

	// A source change to another repository forgets the id; a change that
	// keeps the repository keeps it.
	if _, err := st.SetAppSource(ctx, created.ID, newName, "", "develop", nil); err != nil {
		t.Fatal(err)
	}
	if got := read(created.ID).GitHubRepoID; got != 42 {
		t.Errorf("a branch change dropped the id (%d)", got)
	}
	if _, err := st.SetAppSource(ctx, created.ID, "https://github.com/octo/elsewhere", "", "main", nil); err != nil {
		t.Fatal(err)
	}
	if got := read(created.ID).GitHubRepoID; got != 0 {
		t.Errorf("another repository kept the old id %d", got)
	}
	// Recording an id is a compare-and-set on the current repo.
	if set, _ := st.SetAppGitHubRepoID(ctx, created.ID, newName, 77); set {
		t.Error("an id for a repo the row no longer names must not land")
	}
	if set, err := st.SetAppGitHubRepoID(ctx, created.ID, "https://github.com/octo/elsewhere", 77); err != nil || !set {
		t.Fatalf("SetAppGitHubRepoID = %v, %v; want set", set, err)
	}
	if set, _ := st.SetAppGitHubRepoID(ctx, created.ID, "https://github.com/octo/elsewhere", 77); set {
		t.Error("recording the same id twice must report no change")
	}

	// Blueprints follow the same rules.
	bp, err := st.UpsertBlueprint(ctx, Blueprint{TenantID: tenant.ID, Name: "stack", Repo: oldName, Branch: "main",
		Manifest: "services: []", Status: BlueprintStatusInSync, AutoSync: true})
	if err != nil {
		t.Fatal(err)
	}
	gaps, err := st.ListRepoIdentityGaps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !hasGap(gaps, RepoIdentityService, public.ID) || !hasGap(gaps, RepoIdentityBlueprint, bp.ID) || hasGap(gaps, RepoIdentityService, created.ID) {
		t.Errorf("gaps = %+v, want the public service and the Blueprint, not the service with an id", gaps)
	}
	if moved, _ := st.MoveBlueprintRepo(ctx, bp.ID, 0, oldName, newName); moved {
		t.Error("a Blueprint with no id must not move")
	}
	if set, err := st.SetBlueprintGitHubRepoID(ctx, bp.ID, oldName, 42); err != nil || !set {
		t.Fatalf("SetBlueprintGitHubRepoID = %v, %v", set, err)
	}
	if moved, err := st.MoveBlueprintRepo(ctx, bp.ID, 42, oldName, newName); err != nil || !moved {
		t.Fatalf("MoveBlueprintRepo = %v, %v", moved, err)
	}
	listed, err := st.ListAutoSyncBlueprints(ctx, "main", tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Repo != newName || listed[0].GitHubRepoID != 42 {
		t.Errorf("auto-sync listing = %+v, want the moved Blueprint with id 42", listed)
	}
	gaps, _ = st.ListRepoIdentityGaps(ctx)
	if hasGap(gaps, RepoIdentityBlueprint, bp.ID) {
		t.Error("a Blueprint with an id is no longer a gap")
	}

	// The move's audit row keeps both URLs.
	if err := st.Record(ctx, core.AuditEvent{Caller: core.RepoMoveCaller, CallerMethod: "internal",
		Verb: core.AuditVerbSetSourceAndRegistryCredential, Resource: core.WorkspaceObject(tenant.ID),
		Target: core.ServiceTarget(created.ID), Outcome: core.AuditAllowed, At: time.Now().UTC(),
		RepoFrom: oldName, RepoTo: newName}); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListAuditEvents(ctx, tenant.ID, AuditFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].RepoFrom != oldName || rows[0].RepoTo != newName {
		t.Fatalf("audit rows = %+v, want one naming both URLs", rows)
	}
}

func hasGap(gaps []RepoIdentityGap, kind, id string) bool {
	for _, g := range gaps {
		if g.Kind == kind && g.ID == id {
			return true
		}
	}
	return false
}

// TestProjectorStampsTheGitHubRepoID: the projector carries the row's
// repository id onto the App as core.AnnotationGitHubRepoID, which the push
// webhook matches by, removes it when the row forgets the id, and reports no
// change when the App already carries it (no write on every pass).
func TestProjectorStampsTheGitHubRepoID(t *testing.T) {
	var app appv1alpha1.App
	d := DesiredApp{App: App{ID: "srv-web", TenantID: "tea-a", Name: "web", GitHubRepoID: 42}}
	if !stampLabels(&app, d, "test", false) || core.GitHubRepoID(&app) != 42 {
		t.Fatalf("first stamp: annotations %v", app.Annotations)
	}
	if stampLabels(&app, d, "test", false) {
		t.Error("an App that already carries the id must not be rewritten")
	}
	d.GitHubRepoID = 0
	if !stampLabels(&app, d, "test", false) || core.GitHubRepoID(&app) != 0 {
		t.Errorf("a row without an id must clear the annotation: %v", app.Annotations)
	}
}
