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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// blueprint_connection_test.go is w4/m125: creating a second Blueprint for a
// repo+branch another blueprint already tracks used to run the admission
// upsert's conflict arm and report a successful create — returning the FIRST
// blueprint's id carrying the second one's name, path and manifest. Live, that
// destroyed a blueprint, orphaned its running stack, and left the orphan
// holding the claim of a blueprint whose resources[] disclaimed it.

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

func TestM125_SecondConnectIsRefusedAndChangesNothing(t *testing.T) {
	svc, fs := connectionService(t)
	ctx := ownershipCtx()
	a := connectA(t, svc)

	_, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{
		Repo: repoBex, Branch: "main", Path: pathCron, Name: "bpB",
	})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "BLUEPRINT_CONNECTION_CONFLICT" {
		t.Fatalf("second connect = %v, want BLUEPRINT_CONNECTION_CONFLICT", err)
	}
	// The refusal has to name the blueprint being protected and the phrase, or
	// the caller has no way to proceed deliberately.
	if coded.Params["blueprintId"] != a.ID || coded.Params["confirm"] != BlueprintTakeoverConfirmation(a.ID) {
		t.Fatalf("conflict params = %+v, want blueprintId=%s and its takeover phrase", coded.Params, a.ID)
	}
	if !strings.Contains(err.Error(), "updateBlueprint") {
		t.Errorf("the refusal should name the verb that repoints a blueprint: %v", err)
	}

	// The live bug in one assertion: A must still be A.
	stored := fs.blueprints[a.ID]
	if stored.Name != "bpA" || stored.Path != pathSite || stored.Manifest != m125SiteManifest {
		t.Fatalf("A was mutated by a refused create: %+v", stored)
	}
	list, err := svc.ListBlueprints(ctx, connOwner)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ID != a.ID || list[0].Path != pathSite {
		t.Fatalf("list after refusal = %+v, want only A at its own path", list)
	}
}

func TestM125_ConfirmedTakeoverProceedsAndSaysSo(t *testing.T) {
	svc, fs := connectionService(t)
	ctx := ownershipCtx()
	a := connectA(t, svc)

	b, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{
		Repo: repoBex, Branch: "main", Path: pathCron, Name: "bpB",
		Confirm: BlueprintTakeoverConfirmation(a.ID),
	})
	if err != nil {
		t.Fatalf("confirmed takeover: %v", err)
	}
	if b.Path != pathCron || b.Name != "bpB" {
		t.Fatalf("takeover result = %+v, want bpB at the cron path", b)
	}
	// The row is reused, so sync history is the only place the replacement can
	// be recorded — and it must be, or a blueprint disappears with no trace.
	var note string
	for _, run := range fs.insertedSyncs {
		if run.Note != "" {
			note = run.Note
		}
	}
	if !strings.Contains(note, a.ID) || !strings.Contains(note, pathSite) {
		t.Fatalf("sync note = %q, want it to name the replaced blueprint and its path", note)
	}
}

func TestM125_ReconnectingTheSamePathNeedsNoConfirmation(t *testing.T) {
	svc, _ := connectionService(t)
	ctx := ownershipCtx()
	a := connectA(t, svc)

	// Same repo, same branch, same manifest: a re-connect, not a replacement.
	// This is also the retry path after a create whose apply failed — refusing
	// it would deadlock, since one Confirm cannot carry two phrases.
	again, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{
		Repo: repoBex, Branch: "main", Path: pathSite, Name: "bpA",
	})
	if err != nil {
		t.Fatalf("re-connecting the same path: %v", err)
	}
	if again.ID != a.ID {
		t.Fatalf("re-connect id = %s, want the same row %s", again.ID, a.ID)
	}
}

func TestM125_ConnectingOverADisconnectedRowNeedsNoConfirmation(t *testing.T) {
	svc, fs := connectionService(t)
	ctx := ownershipCtx()
	a := connectA(t, svc)
	if err := svc.DisconnectBlueprint(ctx, a.ID, connOwner); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	// w8/m37's "a disconnected row is deliberately re-established under a fresh
	// generation" is unchanged: there is no live blueprint to protect.
	if _, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{
		Repo: repoBex, Branch: "main", Path: pathCron, Name: "bpB",
	}); err != nil {
		t.Fatalf("create over a disconnected row: %v", err)
	}
	if got := fs.blueprints[a.ID].Path; got != pathCron {
		t.Fatalf("re-established path = %q, want %q", got, pathCron)
	}
}

func TestM125_PreviewReportsTheConflictButNotAgainstItself(t *testing.T) {
	svc, _ := connectionService(t)
	ctx := ownershipCtx()
	a := connectA(t, svc)

	// `/blueprints/new`: a different manifest on an occupied repo+branch.
	p, err := svc.PreviewBlueprint(ctx, connOwner, repoBex, "main", pathCron, "")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if p.Validation == nil || p.Validation.Valid {
		t.Fatalf("preview must not report valid: %+v", p.Validation)
	}
	var found bool
	for _, e := range p.Validation.Errors {
		if e.Code == "BLUEPRINT_CONNECTION_CONFLICT" && strings.Contains(e.Error, a.ID) {
			found = true
		}
	}
	if !found {
		t.Fatalf("preview errors = %+v, want a BLUEPRINT_CONNECTION_CONFLICT naming %s", p.Validation.Errors, a.ID)
	}

	// The detail page's pre-sync preview passes its own id and must stay clean.
	p, err = svc.PreviewBlueprint(ctx, connOwner, repoBex, "main", pathSite, a.ID)
	if err != nil || p.Validation == nil || !p.Validation.Valid {
		t.Fatalf("self preview must stay valid: %+v err=%v", p.Validation, err)
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

	if owner, _ := fs.GetBlueprintResourceOwner(ctx, "tea-a", "service", "static-site"); owner != a.ID {
		t.Fatalf("static-site owner after create = %q, want %s", owner, a.ID)
	}

	// Replace the manifest with one declaring a different service.
	if _, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{
		Repo: repoBex, Branch: "main", Path: pathCron, Name: "bpB",
		Confirm: BlueprintTakeoverConfirmation(a.ID),
	}); err != nil {
		t.Fatalf("takeover: %v", err)
	}

	if owner, _ := fs.GetBlueprintResourceOwner(ctx, "tea-a", "service", "static-site"); owner != "" {
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
		if appServiceName(&apps.Items[i]) != "static-site" {
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
// must not read as "nothing is connected", which is exactly the overwrite the
// guard exists to stop.
func TestM125_ConnectionLookupFailureFailsClosed(t *testing.T) {
	svc, fs := connectionService(t)
	ctx := ownershipCtx()
	connectA(t, svc)
	fs.getByRepoErr = errors.New("control plane unavailable")

	if _, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{
		Repo: repoBex, Branch: "main", Path: pathCron, Name: "bpB",
	}); err == nil {
		t.Fatal("want an error when the connection lookup fails")
	} else if errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a lookup failure must not be read as not-found: %v", err)
	}
}
