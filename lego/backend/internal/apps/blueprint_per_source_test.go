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
	"errors"
	"net/http"
	"sort"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// blueprint_per_source_test.go is w8/m153: a workspace may hold several
// Blueprints for one repo, branch and file. Render: "You can create multiple
// Blueprints from a single YAML file. Each Blueprint creates and manages a
// completely independent set of resources." and "Render appends a suffix to
// the name of each new resource to prevent collisions with your existing
// resources." A second `ea blueprints create` used to answer
// 409 BLUEPRINT_CONNECTION_CONFLICT.

const perSourceManifest = `
services:
  - type: web
    name: api
    runtime: image
    image:
      url: docker.io/library/nginx:1.27
`

func perSourceCreate(name, mode string) RenderCreateBlueprintRequest {
	return RenderCreateBlueprintRequest{Name: name, OwnerID: "tea-a", ExistingResources: &mode,
		Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}}
}

// The milestone's Definition of done 1 and 2 against the service: two
// create_new Blueprints from one file have their own ids, suffixes, plans,
// services and histories, and deleting one leaves the other alone.
func TestM153_TwoBlueprintsFromOneFileAreIndependent(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, perSourceManifest)

	a, err := svc.CreatePlannedBlueprint(ctx, perSourceCreate("copy-a", store.BlueprintExistingCreateNew))
	if err != nil {
		t.Fatalf("copy-a: %v", err)
	}
	b, err := svc.CreatePlannedBlueprint(ctx, perSourceCreate("copy-b", store.BlueprintExistingCreateNew))
	if err != nil {
		t.Fatalf("copy-b, a second Blueprint from the same file: %v", err)
	}
	if a.Blueprint.ID == b.Blueprint.ID {
		t.Fatalf("both creates answered the Blueprint %s", a.Blueprint.ID)
	}
	suffixA := st.blueprints[ids.BlueprintStorageID(a.Blueprint.ID)].ResourceSuffix
	suffixB := st.blueprints[ids.BlueprintStorageID(b.Blueprint.ID)].ResourceSuffix
	if suffixA == "" || suffixB == "" || suffixA == suffixB {
		t.Fatalf("suffixes = %q and %q, want two different ones", suffixA, suffixB)
	}
	if got, want := plannedActions(a), "create service api-"+suffixA; got != want {
		t.Fatalf("copy-a plans %q, want %q", got, want)
	}
	if got, want := plannedActions(b), "create service api-"+suffixB; got != want {
		t.Fatalf("copy-b plans %q, want %q", got, want)
	}

	for _, id := range []string{a.Blueprint.ID, b.Blueprint.ID} {
		if _, err := svc.ApproveBlueprint(ctx, id, "tea-a", "", nil); err != nil {
			t.Fatalf("approve %s: %v", id, err)
		}
	}
	for _, name := range []string{"api-" + suffixA, "api-" + suffixB} {
		if err := svc.Client.Get(ctx, appKey(t, svc, name), &appv1alpha1.App{}); err != nil {
			t.Fatalf("service %s was not created: %v", name, err)
		}
	}
	list, err := svc.ListBlueprints(ctx, "tea-a")
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v err=%v, want both Blueprints", list, err)
	}
	resources := func(id string) []string {
		t.Helper()
		v, err := svc.GetBlueprintByID(ctx, id, "tea-a")
		if err != nil {
			t.Fatalf("get %s: %v", id, err)
		}
		var names []string
		for _, r := range v.Resources {
			names = append(names, r.Name)
		}
		sort.Strings(names)
		return names
	}
	if got := resources(a.Blueprint.ID); len(got) != 1 || got[0] != "api-"+suffixA {
		t.Fatalf("copy-a lists %v, want only its own api-%s", got, suffixA)
	}
	if got := resources(b.Blueprint.ID); len(got) != 1 || got[0] != "api-"+suffixB {
		t.Fatalf("copy-b lists %v, want only its own api-%s", got, suffixB)
	}

	// Deleting copy-b leaves copy-a, its history and its service untouched.
	// (The fake keeps runs in insertedSyncs; its list read is canned.)
	history := func(id string) (n int) {
		for _, run := range st.insertedSyncs {
			if run.BlueprintID == ids.BlueprintStorageID(id) {
				n++
			}
		}
		return n
	}
	historyBefore := history(a.Blueprint.ID)
	if historyBefore == 0 || history(b.Blueprint.ID) == 0 {
		t.Fatalf("runs recorded: copy-a %d, copy-b %d; want each Blueprint to have its own", historyBefore, history(b.Blueprint.ID))
	}
	if err := svc.DisconnectBlueprint(ctx, b.Blueprint.ID, "tea-a"); err != nil {
		t.Fatalf("delete copy-b: %v", err)
	}
	after, err := svc.GetBlueprintByID(ctx, a.Blueprint.ID, "tea-a")
	if err != nil || after.Status != store.BlueprintStatusInSync {
		t.Fatalf("copy-a after deleting copy-b = %+v err=%v, want it in sync", after, err)
	}
	if got := history(a.Blueprint.ID); got != historyBefore {
		t.Fatalf("copy-a history went from %d to %d runs", historyBefore, got)
	}
	if got := resources(a.Blueprint.ID); len(got) != 1 || got[0] != "api-"+suffixA {
		t.Fatalf("copy-a lists %v after copy-b was deleted", got)
	}
	if owner, _ := st.GetBlueprintResourceOwner(ctx, "tea-a", store.BlueprintClaimService, "api-"+suffixA); owner != ids.BlueprintStorageID(a.Blueprint.ID) {
		t.Fatalf("api-%s is managed by %q after copy-b was deleted", suffixA, owner)
	}
	if list, err := svc.ListBlueprints(ctx, "tea-a"); err != nil || len(list) != 1 || list[0].ID != a.Blueprint.ID {
		t.Fatalf("list after the delete = %+v err=%v, want only copy-a", list, err)
	}
}

// Definition of done 4. Render: "Do not manage a particular resource with more
// than one Blueprint." A second create in adopt mode is still a new Blueprint
// and is never refused for its source, but it cannot take the resource the
// first one manages: its plan fails naming BLUEPRINT_RESOURCE_CONFLICT, and a
// failed plan cannot be approved.
func TestM153_AnAdoptModeSecondCreateNeverTakesAManagedResource(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, perSourceManifest)
	first, err := svc.CreatePlannedBlueprint(ctx, perSourceCreate("first", store.BlueprintExistingAdopt))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveBlueprint(ctx, first.Blueprint.ID, "tea-a", "", nil); err != nil {
		t.Fatal(err)
	}
	firstID := ids.BlueprintStorageID(first.Blueprint.ID)

	second, err := svc.CreatePlannedBlueprint(ctx, perSourceCreate("second", store.BlueprintExistingAdopt))
	if err != nil {
		t.Fatalf("an adopt-mode second create must not be refused for its source: %v", err)
	}
	if second.Blueprint.ID == first.Blueprint.ID {
		t.Fatalf("the second create answered the first Blueprint %s", first.Blueprint.ID)
	}
	if second.PlanningOutcome != PlanningFailed || second.Sync == nil {
		t.Fatalf("the adopt-mode second create planned %q (sync %+v), want a failed plan", second.PlanningOutcome, second.Sync)
	}
	var conflict bool
	for _, n := range second.Notices {
		t.Logf("notice: %s %s", n.Severity, n.Message)
		conflict = conflict || (n.Severity == "error" && strings.Contains(n.Message, `service "api" is managed by blueprint `+first.Blueprint.ID))
	}
	if !conflict {
		t.Fatalf("the failed plan's notices = %+v, want an error naming api and the Blueprint that manages it", second.Notices)
	}
	if _, err := svc.ApproveBlueprint(ctx, second.Blueprint.ID, "tea-a", "", nil); err == nil {
		t.Fatal("a plan that failed on a resource conflict was approved")
	}
	if owner, _ := st.GetBlueprintResourceOwner(ctx, "tea-a", store.BlueprintClaimService, "api"); owner != firstID {
		t.Fatalf("api is managed by %q, want the first Blueprint %s", owner, firstID)
	}
}

// Definition of done 3: one push reaches every Blueprint tracking the branch,
// each syncs against its own resources, and one failing does not stop the
// other.
func TestM153_APushReachesEveryBlueprintOnTheBranch(t *testing.T) {
	const repo, commit = "https://github.com/acme/app", "3333333333333333333333333333333333333333"
	manifest := `
services:
  - name: web
    type: web
    runtime: image
    image: {url: docker.io/library/nginx:1.27}
`
	fs := newFakeBlueprintStore(
		store.Blueprint{ID: "blp-plain", TenantID: "tea-a", Name: "plain", Repo: repo, Branch: "main",
			Path: CanonicalBlueprintFilename, Manifest: "stale", Status: store.BlueprintStatusInSync, AutoSync: true},
		store.Blueprint{ID: "blp-copy", TenantID: "tea-a", Name: "copy", Repo: repo, Branch: "main",
			Path: CanonicalBlueprintFilename, Manifest: "stale", Status: store.BlueprintStatusInSync, AutoSync: true,
			ExistingResources: store.BlueprintExistingCreateNew, ResourceSuffix: "ab12"},
	)
	svc := &Service{
		Base:          &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"u": "tea-a"}},
		Blueprints:    fs,
		DomainTraffic: allowDomainTraffic{},
		GitFetcher:    fakeBlueprintFetcher{contents: manifest, sha: commit},
	}
	// `web` already belongs to a third Blueprint, so the plain one's sync is
	// refused (a push never takes a resource over); the copy's `web-ab12` is
	// its own.
	if err := fs.ClaimBlueprintResource(context.Background(), "tea-a", store.BlueprintClaimService, "web", "blp-other", ""); err != nil {
		t.Fatalf("seed the foreign claim: %v", err)
	}

	const secret = "shh"
	h := &GitWebhook{Svc: svc, Secret: secret, Replays: &fakeReplayGuard{}}
	rec := postSignedPush(t, h, secret, pushBodyWithPaths(repo+".git", "main", commit, []string{CanonicalBlueprintFilename}))
	if rec.Code != http.StatusOK {
		t.Fatalf("push status = %d body=%s", rec.Code, rec.Body.String())
	}
	fs.mu.Lock()
	queued := map[string]bool{}
	for _, intent := range fs.autoSyncIntents {
		queued[intent.BlueprintID] = true
	}
	fs.mu.Unlock()
	if len(queued) != 2 || !queued["blp-plain"] || !queued["blp-copy"] {
		t.Fatalf("one push queued syncs for %v, want one for each Blueprint on the branch", queued)
	}

	w := &BlueprintAutoSyncWorker{Svc: svc}
	if err := w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	fs.mu.Lock()
	states := map[string]string{}
	for _, intent := range fs.autoSyncIntents {
		states[intent.BlueprintID] = intent.State
	}
	fs.mu.Unlock()
	if states["blp-copy"] != store.BlueprintAutoSyncIntentCompleted {
		t.Fatalf("the copy's sync = %q, want completed although the other Blueprint's failed (%v)", states["blp-copy"], states)
	}
	if states["blp-plain"] == store.BlueprintAutoSyncIntentCompleted || states["blp-plain"] == "" {
		t.Fatalf("the plain Blueprint's sync = %q, want it refused: web belongs to another Blueprint", states["blp-plain"])
	}
	var app appv1alpha1.App
	if err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "tea-a", Name: core.CRName("tea-a", "web-ab12")}, &app); err != nil {
		t.Fatalf("the copy's service web-ab12 was not synced: %v", err)
	}
	if err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "tea-a", Name: core.CRName("tea-a", "web")}, &app); err == nil {
		t.Fatal("the refused sync still created web")
	}
	if owner, _ := fs.GetBlueprintResourceOwner(context.Background(), "tea-a", store.BlueprintClaimService, "web"); owner != "blp-other" {
		t.Fatalf("web is managed by %q after the push, want it left with blp-other", owner)
	}
}

// The create-conflict code is gone from every create; nothing asks for a
// confirm phrase merely to make a second Blueprint (Definition of done 5).
func TestM153_NoCreateSurfaceAnswersAConnectionConflict(t *testing.T) {
	svc, _, ctx := newOnboardingService(t, perSourceManifest)
	for i, name := range []string{"one", "two", "three"} {
		res, err := svc.CreatePlannedBlueprint(ctx, perSourceCreate(name, store.BlueprintExistingCreateNew))
		if err != nil {
			var c *core.CodedError
			if errors.As(err, &c) {
				t.Fatalf("planned create %d answered %s: %v", i+1, c.Code, err)
			}
			t.Fatalf("planned create %d: %v", i+1, err)
		}
		if res.Blueprint == nil || res.PlanningOutcome != PlanningPlanned {
			t.Fatalf("planned create %d = %+v, want a planned Blueprint", i+1, res)
		}
	}
	// The one-step create, in both modes, beside the three above.
	for _, mode := range []string{"", store.BlueprintExistingCreateNew} {
		if _, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/a/app", Branch: "main", Name: "legacy" + mode, ExistingResources: mode}); err != nil {
			t.Fatalf("one-step create (mode %q) beside three Blueprints of the same file: %v", mode, err)
		}
	}
	list, err := svc.ListBlueprints(ctx, "tea-a")
	if err != nil || len(list) != 5 {
		t.Fatalf("list = %d Blueprints (err %v), want five from one file", len(list), err)
	}
}
