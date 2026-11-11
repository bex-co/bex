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
	"sync/atomic"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// blueprint_name_unique_test.go is w8/250. Render keeps Blueprint names unique
// among approved Blueprints in a workspace. Render's create and the approval
// enforced that; the one-step create and a rename did not, so two copies of
// one file both took the repository's name.

const codeBlueprintNameTaken = "BLUEPRINT_NAME_TAKEN"

func TestOneStepCreateRefusesATakenBlueprintName(t *testing.T) {
	svc, fs := connectionService(t)
	ctx := ownershipCtx()
	connectA(t, svc) // "bpA", in sync

	for _, req := range []CreateBlueprintRequest{
		{Repo: repoBex, Branch: "main", Path: pathCron, Name: "bpA"},
		{Repo: repoBex, Branch: "main", Path: pathCron, Name: "BPA"}, // ignoring case
		{Repo: repoBex, Branch: "main", Path: pathSite, Name: "bpA", ExistingResources: store.BlueprintExistingCreateNew},
	} {
		_, err := svc.CreateBlueprint(ctx, connOwner, req)
		if got := coded(t, err); got != codeBlueprintNameTaken {
			t.Fatalf("create %+v answered %s, want %s", req, got, codeBlueprintNameTaken)
		}
		if len(fs.blueprints) != 1 {
			t.Fatalf("the refused create %+v left %d Blueprints, want the first alone", req, len(fs.blueprints))
		}
	}
	// A name is compared, and stored, without the spaces around it.
	if _, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathCron, Name: "  bpA "}); coded(t, err) != codeBlueprintNameTaken {
		t.Fatalf("create under a taken name with spaces around it = %v, want %s", err, codeBlueprintNameTaken)
	}
	created, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathCron, Name: " bpB "})
	if err != nil || created.Name != "bpB" {
		t.Fatalf("create under a free name = %q, err %v, want it stored trimmed", created.Name, err)
	}
}

// A create that sends no name is never refused for the default it was given:
// it gets the repository's name, and a copy gets that name with its suffix.
func TestOneStepCreateWithoutANameGetsAFreeDefault(t *testing.T) {
	svc, _ := connectionService(t)
	ctx := ownershipCtx()

	// While the repository's name is free there is nothing to suggest.
	if fresh, err := svc.previewBlueprintWith(ctx, connOwner, repoBex, "main", pathSite, blueprintPreviewOptions{}); err != nil || fresh.SuggestedName != "" {
		t.Fatalf("review in an empty workspace suggests %q (err %v), want nothing", fresh.SuggestedName, err)
	}
	first, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathSite})
	if err != nil || first.Name != "bex" {
		t.Fatalf("first unnamed create = %q, err %v, want the repository's name", first.Name, err)
	}
	review, err := svc.previewBlueprintWith(ctx, connOwner, repoBex, "main", pathSite, blueprintPreviewOptions{ExistingResources: store.BlueprintExistingCreateNew})
	if err != nil || review.ResourceSuffix == "" || review.SuggestedName != "bex-"+review.ResourceSuffix {
		t.Fatalf("review of a copy suggests %q (suffix %q, err %v), want the repository's name with the suffix", review.SuggestedName, review.ResourceSuffix, err)
	}
	copied, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathSite,
		ExistingResources: store.BlueprintExistingCreateNew, ResourceSuffix: review.ResourceSuffix})
	if err != nil || copied.Name != review.SuggestedName {
		t.Fatalf("unnamed copy = %q, err %v, want the reviewed %q", copied.Name, err, review.SuggestedName)
	}
	// A form that proposes its own default asks about that name.
	form, err := svc.previewBlueprintWith(ctx, connOwner, repoBex, "main", pathCron, blueprintPreviewOptions{Name: "my-stack"})
	if err != nil || form.SuggestedName != "" {
		t.Fatalf("review under a free form default suggests %q (err %v), want nothing", form.SuggestedName, err)
	}
	form, err = svc.previewBlueprintWith(ctx, connOwner, repoBex, "main", pathCron, blueprintPreviewOptions{Name: "BEX"})
	if err != nil || form.SuggestedName != "BEX-2" {
		t.Fatalf("review under a taken form default suggests %q (err %v), want BEX-2", form.SuggestedName, err)
	}
	// A Blueprint without a suffix is numbered.
	adopt, err := svc.previewBlueprintWith(ctx, connOwner, repoBex, "main", pathCron, blueprintPreviewOptions{})
	if err != nil || adopt.SuggestedName != "bex-2" {
		t.Fatalf("review of another file suggests %q (err %v), want bex-2", adopt.SuggestedName, err)
	}
	other, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathCron})
	if err != nil || other.Name != "bex-2" {
		t.Fatalf("unnamed create of another file = %q, err %v, want bex-2", other.Name, err)
	}
}

// Repeating a create whose apply failed continues that Blueprint. It is not
// refused by the name its own failed attempt took.
func TestARetriedCreateIsNotRefusedByItsOwnName(t *testing.T) {
	svc, fs := connectionService(t)
	ctx := ownershipCtx()
	failing := &atomic.Bool{}
	svc.Client = failingAppWrites{Client: svc.Client, failing: failing}
	req := CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathSite, Name: "again"}

	failing.Store(true)
	if _, err := svc.CreateBlueprint(ctx, connOwner, req); err == nil {
		t.Fatal("the first attempt should fail while the API server refuses writes")
	}
	failing.Store(false)
	retried, err := svc.CreateBlueprint(ctx, connOwner, req)
	if err != nil || retried.Name != "again" || len(fs.blueprints) != 1 {
		t.Fatalf("retry = %q, %d Blueprints, err %v, want the same Blueprint under its name", retried.Name, len(fs.blueprints), err)
	}
}

func TestRenameRefusesATakenBlueprintName(t *testing.T) {
	svc, _ := connectionService(t)
	ctx := ownershipCtx()
	a := connectA(t, svc)
	b, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathCron, Name: "bpB"})
	if err != nil {
		t.Fatal(err)
	}
	taken := "BPa"
	if _, err := svc.UpdateBlueprint(ctx, b.ID, connOwner, UpdateBlueprintRequest{Name: &taken}); coded(t, err) != codeBlueprintNameTaken {
		t.Fatalf("rename to a taken name = %v, want %s", err, codeBlueprintNameTaken)
	}
	// Its own name in another case is its own.
	if got, err := svc.UpdateBlueprint(ctx, a.ID, connOwner, UpdateBlueprintRequest{Name: &taken}); err != nil || got.Name != taken {
		t.Fatalf("rename to its own name in another case = %q, err %v", got.Name, err)
	}
	padded, blank := " bpa ", "   "
	if _, err := svc.UpdateBlueprint(ctx, b.ID, connOwner, UpdateBlueprintRequest{Name: &padded}); coded(t, err) != codeBlueprintNameTaken {
		t.Fatalf("rename to a taken name with spaces around it = %v, want %s", err, codeBlueprintNameTaken)
	}
	if _, err := svc.UpdateBlueprint(ctx, b.ID, connOwner, UpdateBlueprintRequest{Name: &blank}); err == nil {
		t.Fatal("rename to a blank name was accepted")
	}
	free := "bpC"
	if _, err := svc.UpdateBlueprint(ctx, b.ID, connOwner, UpdateBlueprintRequest{Name: &free}); err != nil {
		t.Fatalf("rename to a free name: %v", err)
	}
}
