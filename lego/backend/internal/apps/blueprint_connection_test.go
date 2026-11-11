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
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// blueprint_connection_test.go covers what a create does when the workspace
// already has a Blueprint for the same repo and branch.
//
// w4/m125 made that a refusal: the storage key was UNIQUE (tenant, repo,
// branch), so a second create used to run the admission upsert's conflict arm
// and return the FIRST blueprint's id carrying the second one's name, path and
// manifest. w8/m153 removes the key instead. Render: "You can create multiple
// Blueprints from a single YAML file. Each Blueprint creates and manages a
// completely independent set of resources." A create is a new Blueprint with
// its own id; what stays guarded is the resource (one manager each), and
// w4/m125's other halves — claims released when a sync drops a resource, the
// managing Blueprint readable, a failed lookup failing closed — are unchanged.

const (
	m125SiteManifest = `services:
  - name: static-site
    type: web
    runtime: static
    staticPublishPath: dist
`
	m125CronManifest = `services:
  - name: cron-demo
    type: cron
    runtime: docker
    schedule: "0 0 * * *"
    dockerCommand: /bin/true
`
)

// multiPathFetcher serves a different manifest per path, which is the shape the
// hunt used: one repo+branch, two manifests, two intended blueprints.
type multiPathFetcher struct{ byPath map[string]string }

func (f multiPathFetcher) ResolveBlueprintCommit(context.Context, string, string, string) (string, error) {
	return testCommitSHA, nil
}

func (f multiPathFetcher) FetchBlueprintFileAtCommit(_ context.Context, _, _, _, filePath string) (string, error) {
	if contents, ok := f.byPath[filePath]; ok {
		return contents, nil
	}
	return "", errors.New("bad request: not found on main")
}

func connectionService(t *testing.T) (*Service, *fakeBlueprintStore) {
	t.Helper()
	fs := newFakeBlueprintStore()
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Blueprints: fs,
		GitFetcher: multiPathFetcher{byPath: map[string]string{
			"examples/static-site/render.yaml": m125SiteManifest,
			"examples/cron-demo/render.yaml":   m125CronManifest,
		}},
	}
	return svc, fs
}

const (
	repoBex   = "https://github.com/bex-co/bex"
	pathSite  = "examples/static-site/render.yaml"
	pathCron  = "examples/cron-demo/render.yaml"
	connOwner = "tea-a"
)

func connectA(t *testing.T, svc *Service) BlueprintView {
	t.Helper()
	a, err := svc.CreateBlueprint(ownershipCtx(), connOwner, CreateBlueprintRequest{
		Repo: repoBex, Branch: "main", Path: pathSite, Name: "bpA",
	})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	return a
}

// A second create on a tracked repo and branch, for another file, is a second
// Blueprint. The first is untouched: this is w4/m125's live bug ("A must still
// be A") kept as an assertion, now without a refusal.
func TestM153_SecondBlueprintOnATrackedSourceIsANewBlueprint(t *testing.T) {
	svc, fs := connectionService(t)
	ctx := ownershipCtx()
	a := connectA(t, svc)

	b, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{
		Repo: repoBex, Branch: "main", Path: pathCron, Name: "bpB",
	})
	if err != nil {
		t.Fatalf("second Blueprint on a tracked repo and branch: %v", err)
	}
	if b.ID == a.ID || b.Path != pathCron || b.Name != "bpB" {
		t.Fatalf("second Blueprint = %+v, want a new id beside %s, at the cron path", b, a.ID)
	}
	stored := fs.blueprints[a.ID]
	if stored.Name != "bpA" || stored.Path != pathSite || stored.Manifest != m125SiteManifest {
		t.Fatalf("A was changed by the second create: %+v", stored)
	}
	list, err := svc.ListBlueprints(ctx, connOwner)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	paths := map[string]string{}
	for _, v := range list {
		paths[v.ID] = v.Path
	}
	if len(list) != 2 || paths[a.ID] != pathSite || paths[b.ID] != pathCron {
		t.Fatalf("list = %+v, want A and B, each at its own path", list)
	}
	for id, want := range map[string]string{a.ID: "static-site", b.ID: "cron-demo"} {
		if owner, _ := fs.GetBlueprintResourceOwner(ctx, "tea-a", store.BlueprintClaimService, want); owner != id {
			t.Errorf("%s is managed by %q, want %s", want, owner, id)
		}
	}
}

// No create is answered with a connection conflict or acts on a connection
// takeover phrase any more: a phrase naming A, which used to replace A, now
// creates B and leaves A alone, with nothing to record on the run.
func TestM153_AConnectionTakeoverPhraseReplacesNothing(t *testing.T) {
	svc, fs := connectionService(t)
	ctx := ownershipCtx()
	a := connectA(t, svc)

	b, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{
		Repo: repoBex, Branch: "main", Path: pathCron, Name: "bpB",
		Confirm: BlueprintTakeoverConfirmation(a.ID),
	})
	if err != nil {
		t.Fatalf("create carrying A's phrase: %v", err)
	}
	if b.ID == a.ID {
		t.Fatalf("the create reused A's row %s", a.ID)
	}
	if got := fs.blueprints[a.ID]; got.Path != pathSite || got.Name != "bpA" || got.Status == "disconnected" {
		t.Fatalf("A after the create = %+v, want it live and unchanged", got)
	}
	for _, run := range fs.insertedSyncs {
		if run.Note != "" {
			t.Fatalf("a create recorded the note %q; nothing was replaced", run.Note)
		}
	}
}

// The same file again. Render: "Do not manage a particular resource with more
// than one Blueprint." So in adopt mode the second Blueprint is refused the
// resource A manages, with the resource takeover phrase and never a silent
// move; in create_new mode it gets its own suffixed resource. Either way A
// keeps its id.
func TestM153_SameFileAgainNeverMovesAManagedResourceSilently(t *testing.T) {
	svc, fs := connectionService(t)
	ctx := ownershipCtx()
	a := connectA(t, svc)
	owner := func(name string) string {
		t.Helper()
		o, _ := fs.GetBlueprintResourceOwner(ctx, "tea-a", store.BlueprintClaimService, name)
		return o
	}

	_, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathSite, Name: "bpA2"})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "BLUEPRINT_RESOURCE_CONFLICT" {
		t.Fatalf("adopt-mode second create of A's file = %v, want BLUEPRINT_RESOURCE_CONFLICT", err)
	}
	if strings.Contains(err.Error(), "BLUEPRINT_CONNECTION_CONFLICT") || owner("static-site") != a.ID {
		t.Fatalf("after the refusal static-site is managed by %q (want %s); err = %v", owner("static-site"), a.ID, err)
	}
	phrase, _ := coded.Params["confirm"].(string)
	if phrase != BlueprintTakeoverConfirmation(a.ID) {
		t.Fatalf("refusal phrase = %q, want the takeover of %s", phrase, a.ID)
	}

	// The refusal comes before anything is recorded: no second Blueprint is
	// left in error for every push to re-run into the same conflict.
	if n := len(fs.blueprints); n != 1 {
		t.Fatalf("%d Blueprint rows after a refused create, want only A", n)
	}
	// Confirming creates the second Blueprint and only then moves the resource.
	confirmed, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathSite, Name: "bpA2", Confirm: phrase})
	if err != nil {
		t.Fatalf("confirmed takeover: %v", err)
	}
	if confirmed.ID == a.ID || owner("static-site") != confirmed.ID {
		t.Fatalf("confirmed create = %s owning static-site = %q, want a second Blueprint that now manages it", confirmed.ID, owner("static-site"))
	}
	if n := len(fs.blueprints); n != 2 {
		t.Fatalf("%d Blueprint rows after a refused create and a confirmed one, want 2 (A and the second)", n)
	}

	// create_new: an independent, suffixed copy; nobody's resource moves.
	copyBP, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathSite, Name: "bpCopy",
		ExistingResources: store.BlueprintExistingCreateNew, ResourceSuffix: "c0py"})
	if err != nil {
		t.Fatalf("create_new copy: %v", err)
	}
	if copyBP.ID == a.ID || copyBP.ID == confirmed.ID || copyBP.ResourceSuffix != "c0py" {
		t.Fatalf("copy = %+v, want a third Blueprint carrying suffix c0py", copyBP)
	}
	if owner("static-site-c0py") != copyBP.ID || owner("static-site") != confirmed.ID {
		t.Fatalf("owners: static-site-c0py=%q static-site=%q, want the copy and the second Blueprint", owner("static-site-c0py"), owner("static-site"))
	}
	if _, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathCron, Name: "bad",
		ExistingResources: "replace"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("unknown existingResources = %v, want a 400", err)
	}
	if _, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathCron, Name: "bad",
		ResourceSuffix: "c0py"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("a suffix without create_new = %v, want a 400", err)
	}
}

// failingAppWrites fails every real App create while failing is set, the way
// an API server that goes away mid-apply does. Dry runs (the validation's)
// still answer.
type failingAppWrites struct {
	client.Client
	failing *atomic.Bool
}

func (c failingAppWrites) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if _, isApp := obj.(*appv1alpha1.App); isApp && c.failing.Load() {
		options := &client.CreateOptions{}
		options.ApplyOptions(opts)
		if len(options.DryRun) == 0 {
			return errors.New("apiserver unavailable")
		}
	}
	return c.Client.Create(ctx, obj, opts...)
}

// A create whose apply failed left its row; repeating the create continues on
// that Blueprint, with its suffix and its settings, instead of adding a second
// half-made one. The review agrees with what the create will do. Once the
// Blueprint has synced, the same create is a new Blueprint.
func TestM153_ARepeatedCreateRetriesAFailedOne(t *testing.T) {
	svc, fs := connectionService(t)
	ctx := ownershipCtx()
	failing := &atomic.Bool{}
	svc.Client = failingAppWrites{Client: svc.Client, failing: failing}
	req := CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathSite, Name: "retry-me",
		ExistingResources: store.BlueprintExistingCreateNew, ResourceSuffix: "f1rs"}

	failing.Store(true)
	if _, err := svc.CreateBlueprint(ctx, connOwner, req); err == nil {
		t.Fatal("the first attempt should fail while the API server refuses writes")
	}
	if len(fs.blueprints) != 1 {
		t.Fatalf("the failed create left %d rows, want its own Blueprint", len(fs.blueprints))
	}
	var failed store.Blueprint
	for _, b := range fs.blueprints {
		failed = b
	}
	if failed.Status != store.BlueprintStatusError || failed.ResourceSuffix != "f1rs" {
		t.Fatalf("the failed create left %+v, want it in error with suffix f1rs", failed)
	}

	// The review of the same create is made as that Blueprint: its suffix,
	// whatever a fresh review would have minted.
	failing.Store(false)
	p, err := svc.previewBlueprintWith(ctx, connOwner, repoBex, "main", pathSite, blueprintPreviewOptions{ExistingResources: store.BlueprintExistingCreateNew})
	if err != nil || p.ResourceSuffix != "f1rs" || p.Validation == nil || !p.Validation.Valid {
		t.Fatalf("preview before the retry = suffix %q validation %+v err %v, want the failed Blueprint's f1rs and a valid review", p.ResourceSuffix, p.Validation, err)
	}
	// A suffix that is not the retried Blueprint's is refused, not replaced.
	other := req
	other.ResourceSuffix = "0th3"
	if _, err := svc.CreateBlueprint(ctx, connOwner, other); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("retry with another suffix = %v, want a conflict naming the Blueprint's own", err)
	}

	// The user renamed the failed Blueprint in the meantime; the retry keeps it.
	renamed := "renamed-since"
	if _, err := svc.UpdateBlueprint(ctx, failed.ID, connOwner, UpdateBlueprintRequest{Name: &renamed}); err != nil {
		t.Fatalf("rename the failed Blueprint: %v", err)
	}
	retried, err := svc.CreateBlueprint(ctx, connOwner, req)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retried.ID != failed.ID || retried.ResourceSuffix != "f1rs" || retried.Name != renamed || len(fs.blueprints) != 1 {
		t.Fatalf("retry = %+v with %d rows, want the failed Blueprint %s continued under its suffix and name", retried, len(fs.blueprints), failed.ID)
	}
	if owner, _ := fs.GetBlueprintResourceOwner(ctx, "tea-a", store.BlueprintClaimService, "static-site-f1rs"); owner != failed.ID {
		t.Fatalf("static-site-f1rs is managed by %q, want the retried Blueprint", owner)
	}

	// It has synced now: the same create is a second copy with its own suffix.
	again, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathSite, Name: "copy",
		ExistingResources: store.BlueprintExistingCreateNew})
	if err != nil {
		t.Fatalf("a create_new copy after the Blueprint synced: %v", err)
	}
	if again.ID == failed.ID || again.ResourceSuffix == "" || again.ResourceSuffix == "f1rs" {
		t.Fatalf("copy = %+v, want a new Blueprint with its own minted suffix", again)
	}
}

// A Blueprint created after another on the same repo and branch was deleted is
// new. The deleted row is not revived (it used to be, w8/m37 and w4/120), so
// it keeps its own file and stays disconnected, and the new Blueprint starts
// with no inherited history to mark off.
func TestM153_CreateAfterADeleteIsANewBlueprint(t *testing.T) {
	svc, fs := connectionService(t)
	ctx := ownershipCtx()
	a := connectA(t, svc)
	if err := svc.DisconnectBlueprint(ctx, a.ID, connOwner); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	before := len(fs.insertedSyncs)

	b, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{
		Repo: repoBex, Branch: "main", Path: pathSite, Name: "bpA-again",
	})
	if err != nil {
		t.Fatalf("create after a delete: %v", err)
	}
	if b.ID == a.ID {
		t.Fatalf("create after a delete reused the deleted row %s", a.ID)
	}
	if got := fs.blueprints[a.ID]; got.Status != "disconnected" || got.Name != "bpA" {
		t.Fatalf("the deleted row = %+v, want it left disconnected and unchanged", got)
	}
	for _, run := range fs.insertedSyncs[before:] {
		if run.BlueprintID != b.ID || run.Note != "" {
			t.Fatalf("run after the create = %+v, want the new Blueprint's own run with no note", run)
		}
	}
}

// The review step no longer reports a connection conflict. For a Blueprint not
// yet created it reports which declared resources already exist, and reviews
// in create_new mode (a separate, suffixed set) when asked to.
func TestM153_PreviewReportsMatchesNotAConnectionConflict(t *testing.T) {
	svc, _ := connectionService(t)
	ctx := ownershipCtx()
	a := connectA(t, svc)

	// `/blueprints/new`: another file on a tracked repo and branch.
	p, err := svc.PreviewBlueprint(ctx, connOwner, repoBex, "main", pathCron, "")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if p.Validation == nil || !p.Validation.Valid || len(p.MatchedResources) != 0 || p.ExistingResources != store.BlueprintExistingAdopt {
		t.Fatalf("preview of a new file = validation %+v matched %+v mode %q, want valid, no matches, adopt", p.Validation, p.MatchedResources, p.ExistingResources)
	}

	// The same file A was made from: its resource exists and A manages it.
	p, err = svc.PreviewBlueprint(ctx, connOwner, repoBex, "main", pathSite, "")
	if err != nil {
		t.Fatalf("preview of A's file: %v", err)
	}
	if len(p.MatchedResources) != 1 || p.MatchedResources[0] != (BlueprintResourceRef{Kind: "service", Name: "static-site"}) {
		t.Fatalf("matched = %+v, want the service static-site", p.MatchedResources)
	}
	if p.Validation == nil || p.Validation.Valid {
		t.Fatalf("adopt-mode review of A's file must report the resource conflict: %+v", p.Validation)
	}
	for _, e := range p.Validation.Errors {
		if e.Code != "BLUEPRINT_RESOURCE_CONFLICT" {
			t.Fatalf("adopt-mode review errors = %+v, want only BLUEPRINT_RESOURCE_CONFLICT", p.Validation.Errors)
		}
	}

	// Render's creation flow for that case: a separate, suffixed set.
	p, err = svc.previewBlueprintWith(ctx, connOwner, repoBex, "main", pathSite, blueprintPreviewOptions{ExistingResources: store.BlueprintExistingCreateNew})
	if err != nil {
		t.Fatalf("create_new preview: %v", err)
	}
	if p.Validation == nil || !p.Validation.Valid || p.ExistingResources != store.BlueprintExistingCreateNew ||
		!store.ValidBlueprintResourceSuffix(p.ResourceSuffix) || len(p.MatchedResources) != 1 {
		t.Fatalf("create_new preview = validation %+v mode %q suffix %q matched %+v", p.Validation, p.ExistingResources, p.ResourceSuffix, p.MatchedResources)
	}
	var planned []string
	for _, action := range p.Validation.Plan.Actions {
		planned = append(planned, string(action.Operation)+" "+action.Name)
	}
	if want := "create static-site-" + p.ResourceSuffix; len(planned) != 1 || planned[0] != want {
		t.Fatalf("create_new plan = %v, want [%s]", planned, want)
	}
	// The reviewed suffix is what the create makes.
	created, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathSite, Name: "copy",
		ExistingResources: p.ExistingResources, ResourceSuffix: p.ResourceSuffix})
	if err != nil || created.ResourceSuffix != p.ResourceSuffix || created.ID == a.ID {
		t.Fatalf("create with the reviewed suffix = %+v err=%v", created, err)
	}

	// The detail page's pre-sync preview passes its own id: it stays clean,
	// reports no create-time fields, and refuses a mode.
	p, err = svc.PreviewBlueprint(ctx, connOwner, repoBex, "main", pathSite, a.ID)
	if err != nil || p.Validation == nil || !p.Validation.Valid || p.ExistingResources != "" || len(p.MatchedResources) != 0 {
		t.Fatalf("self preview = %+v err=%v, want valid with no create-time fields", p, err)
	}
	if _, err := svc.previewBlueprintWith(ctx, connOwner, repoBex, "main", pathSite,
		blueprintPreviewOptions{ForBlueprintID: a.ID, ExistingResources: store.BlueprintExistingCreateNew}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("a mode on an existing Blueprint's preview = %v, want a 400", err)
	}
}

// TestM125_ClaimsEqualResourcesAfterASyncDropsOne is t002's headline: the claim
// used to be released only by disconnect, so a resource a manifest stopped
// declaring stayed claimed by a blueprint whose resources[] no longer listed
// it — the same id saying "I manage it" and "I do not" at once.
func TestM125_ClaimsEqualResourcesAfterASyncDropsOne(t *testing.T) {
	svc, fs := connectionService(t)
	ctx := ownershipCtx()
	a := connectA(t, svc)

	if owner, _ := fs.GetBlueprintResourceOwner(ctx, "tea-a", store.BlueprintClaimService, "static-site"); owner != a.ID {
		t.Fatalf("static-site owner after create = %q, want %s", owner, a.ID)
	}

	// Sync A with a manifest declaring a different service. (Until w8/m153
	// this test replaced A through a confirmed second create on its row.)
	if _, err := svc.SyncBlueprint(ctx, a.ID, connOwner, m125CronManifest, "", nil); err != nil {
		t.Fatalf("sync with the replaced manifest: %v", err)
	}

	if owner, _ := fs.GetBlueprintResourceOwner(ctx, "tea-a", store.BlueprintClaimService, "static-site"); owner != "" {
		t.Fatalf("static-site still claimed by %q after it was dropped", owner)
	}
	claims, err := fs.ListBlueprintResourceClaims(ctx, "tea-a", a.ID)
	if err != nil {
		t.Fatalf("list claims: %v", err)
	}
	for _, c := range claims {
		if c.Name == "static-site" {
			t.Fatalf("claim survived the drop: %+v", c)
		}
	}

	// The release frees the claim and nothing else — the orphan keeps running
	// (w4/119: not deleting it is deliberate), and its marker is cleared so the
	// label cannot outlive the claim it mirrors.
	var apps appv1alpha1.AppList
	if err := svc.Client.List(ctx, &apps); err != nil {
		t.Fatalf("list apps: %v", err)
	}
	var seen bool
	for i := range apps.Items {
		if core.AppPublicName(&apps.Items[i]) != "static-site" {
			continue
		}
		seen = true
		if got := apps.Items[i].Labels[core.LabelBlueprint]; got != "" {
			t.Fatalf("dropped service still marked as blueprint %q", got)
		}
	}
	if !seen {
		t.Fatal("the dropped service must still exist")
	}

	// And it is adoptable again: the preview that used to report a conflict
	// with a blueprint that disclaimed it now comes back clean.
	p, err := svc.PreviewBlueprint(ctx, connOwner, "https://github.com/other/repo", "main", pathSite, "")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	for _, e := range p.Validation.Errors {
		if e.Code == "BLUEPRINT_RESOURCE_CONFLICT" {
			t.Fatalf("released resource still conflicts: %+v", e)
		}
	}
}

// TestM125_ManagingBlueprintIsReadable is t003: the claim was invisible on
// every read, so the only way to learn who owned a resource was to trigger the
// conflict and read the error.
func TestM125_ManagingBlueprintIsReadable(t *testing.T) {
	svc, _ := connectionService(t)
	ctx := ownershipCtx()
	a := connectA(t, svc)

	views, err := svc.List(ctx, connOwner)
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	var managed *AppView
	for i := range views {
		if views[i].Name == "static-site" {
			managed = &views[i]
		}
	}
	if managed == nil {
		t.Fatal("static-site not found")
	}
	if managed.BlueprintID != a.ID {
		t.Fatalf("blueprintId = %q, want %s — and on the LIST read, deliberately", managed.BlueprintID, a.ID)
	}
	one, err := svc.Get(ctx, managed.Name)
	if err != nil {
		t.Fatalf("get service: %v", err)
	}
	if one.BlueprintID != a.ID {
		t.Fatalf("by-id blueprintId = %q, want %s", one.BlueprintID, a.ID)
	}
	if rendered := svc.restService(ctx, one); rendered.BlueprintID != a.ID {
		t.Fatalf("REST/MCP projection blueprintId = %q, want %s", rendered.BlueprintID, a.ID)
	}
}

// TestM125_ConnectionLookupFailureFailsClosed: an unreadable blueprint table
// must not read as "this is a new Blueprint" — the lookup now decides whether
// a create retries a failed one, and guessing would leave a duplicate.
func TestM125_ConnectionLookupFailureFailsClosed(t *testing.T) {
	svc, fs := connectionService(t)
	ctx := ownershipCtx()
	connectA(t, svc)
	fs.createRetryErr = errors.New("control plane unavailable")

	if _, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{
		Repo: repoBex, Branch: "main", Path: pathCron, Name: "bpB",
	}); err == nil {
		t.Fatal("want an error when the connection lookup fails")
	} else if errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a lookup failure must not be read as not-found: %v", err)
	}
}

// --- w4/118: workspace-resolvable references are checked at validate --------
//
// fromService names a service in the SAME FILE, so it is fully checkable from
// the manifest and always was. fromDatabase and fromService→KeyValue resolve
// against the workspace, and their checks ran only at apply — so a manifest
// naming a database that exists nowhere came back `valid: true` with a clean
// plan, while the same manifest's dangling fromService was rejected.

const m118DanglingDatabase = `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
    envVars:
      - key: REF
        fromDatabase:
          name: qa-no-such-database-anywhere
          property: connectionString
`

const m118DanglingKeyValue = `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
    envVars:
      - key: REF
        fromService:
          name: qa-no-such-keyvalue-anywhere
          type: keyvalue
          property: connectionString
`

func TestM118_DanglingWorkspaceReferencesFailValidation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest string
		want     string
	}{
		{
			name:     "fromDatabase",
			manifest: m118DanglingDatabase,
			want:     `fromDatabase references unknown database "qa-no-such-database-anywhere" in this workspace`,
		},
		{
			name:     "fromService to a Key Value",
			manifest: m118DanglingKeyValue,
			want:     `fromService references unknown Key Value "qa-no-such-keyvalue-anywhere" in this workspace`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := connectionService(t)
			v, err := svc.ValidateBlueprint(ownershipCtx(), connOwner, tc.manifest, "")
			if err != nil {
				t.Fatalf("ValidateBlueprint: %v", err)
			}
			if v.Valid {
				t.Fatalf("a reference that resolves to nothing must not validate: %+v", v)
			}
			// The wording is apply's, verbatim — validate and apply must not
			// have two vocabularies for one refusal.
			var found bool
			for _, e := range v.Errors {
				if strings.Contains(e.Error, tc.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("validation errors = %+v, want one containing %q", v.Errors, tc.want)
			}
		})
	}
}

// TestM118_AResolvableReferenceStillValidates keeps the guard above from being
// satisfied by refusing every workspace reference — the superset over Render's
// same-blueprint-only rule is deliberate and has to keep working.
func TestM118_AResolvableReferenceStillValidates(t *testing.T) {
	svc, _ := connectionService(t)
	ctx := ownershipCtx()

	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dpg-real",
			Namespace: "default",
			Labels:    map[string]string{core.LabelTenant: "tea-a"},
		},
		Spec: appv1alpha1.DatabaseSpec{Name: "qa-real-database"},
	}
	if err := svc.Client.Create(ctx, db); err != nil {
		t.Fatalf("create database: %v", err)
	}

	manifest := strings.Replace(m118DanglingDatabase, "qa-no-such-database-anywhere", "qa-real-database", 1)
	v, err := svc.ValidateBlueprint(ctx, connOwner, manifest, "")
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if !v.Valid {
		t.Fatalf("a database that exists in the workspace must validate: %+v", v.Errors)
	}
}

// w4/532: a Public Git URL with no branch previewed nothing and created
// nothing ("repo and branch are required"), while a service create resolves
// the repository's default branch. Preview and create now resolve it the same
// way and report the branch they read; a failed lookup is the preview's own
// reason, and with no opinion (GitHub unconfigured) a branch stays required.
func TestBlueprintWithoutBranchUsesTheRepositoryDefault(t *testing.T) {
	ctx := ownershipCtx()

	svc, _ := connectionService(t)
	gh := &fakeCloneTokens{defaultBranch: "main"}
	svc.GitHub = gh
	p, err := svc.PreviewBlueprint(ctx, connOwner, repoBex, "", pathCron, "")
	if err != nil || !p.Found || p.Branch != "main" || gh.validateCalls != 1 {
		t.Fatalf("preview = %+v, %v after %d lookups; want found on main after one", p, err, gh.validateCalls)
	}
	if _, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Path: pathCron, Name: "bpDefault"}); err != nil {
		t.Fatalf("create without branch: %v", err)
	}

	// An explicit branch asks GitHub nothing and is reported as given.
	gh.validateCalls = 0
	if p, err := svc.PreviewBlueprint(ctx, connOwner, repoBex, "main", pathCron, ""); err != nil || p.Branch != "main" || gh.validateCalls != 0 {
		t.Fatalf("explicit preview = %+v, %v after %d lookups", p, err, gh.validateCalls)
	}

	for name, tc := range map[string]struct {
		err  error
		want BlueprintPreviewReason
	}{
		"not accessible": {fmt.Errorf("%w: repository is not accessible", core.ErrBadRequest), BlueprintFetchRepoNotFound},
		"rate limited":   {core.NewUnavailableError("GITHUB_RATE_LIMITED", "GitHub is rate-limiting bex right now. Try again in a few minutes.", nil), BlueprintFetchRateLimited},
		"unreachable":    {core.NewUnavailableError("GITHUB_UNAVAILABLE", "Could not reach GitHub to check the repository. Try again in a few minutes.", nil), BlueprintFetchUnavailable},
	} {
		svc.GitHub = &fakeCloneTokens{validateErr: tc.err}
		if p, err := svc.PreviewBlueprint(ctx, connOwner, repoBex, "", pathCron, ""); err != nil || p.Found || p.Reason != tc.want {
			t.Errorf("%s: preview = %+v, %v; want reason %s", name, p, err, tc.want)
		}
	}

	svc.GitHub = &fakeCloneTokens{} // no opinion: not a github.com repo, or GitHub off
	if _, err := svc.PreviewBlueprint(ctx, connOwner, repoBex, "", pathCron, ""); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("no default to offer = %v, want a 400", err)
	}
}
