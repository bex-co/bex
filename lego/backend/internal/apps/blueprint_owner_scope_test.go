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
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/m169: a Blueprint id resolves its own workspace, like a service or
// env-group id, so a caller who belongs to several workspaces needs no ownerId.
// A workspace the caller is not in answers ADR072's typed-id 403, and only an
// id that exists nowhere is a 404 (w5/m115).
func TestBlueprintByIDVerbsResolveTheOwningWorkspace(t *testing.T) {
	fs := newFakeBlueprintStore(store.Blueprint{ID: "blp-b", TenantID: "tea-b", Name: "bravo", Repo: "https://github.com/acme/app", Branch: "main", Status: "active"})
	svc := newBlueprintService(fs, coretest.Members{"dana": {"tea-a", "tea-b"}, "eve": {"tea-a"}})
	dana := ctxAs("dana") // default workspace tea-a; the Blueprint lives in tea-b

	got, err := svc.GetBlueprintByID(dana, "blp-b", "")
	if err != nil || got.ID != "blp-b" {
		t.Fatalf("GetBlueprintByID without ownerId = (%+v, %v), want the tea-b Blueprint", got, err)
	}
	if _, err := svc.ListBlueprintSyncs(dana, "blp-b", "", "", 20); err != nil {
		t.Fatalf("ListBlueprintSyncs without ownerId = %v", err)
	}
	name := "renamed"
	if v, err := svc.UpdateBlueprint(dana, "blp-b", "", UpdateBlueprintRequest{Name: &name}); err != nil || v.Name != "renamed" {
		t.Fatalf("UpdateBlueprint without ownerId = (%+v, %v)", v, err)
	}

	// An explicit ownerId still decides: the wrong workspace is not found.
	if _, err := svc.GetBlueprintByID(dana, "blp-b", "tea-a"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("GetBlueprintByID with a mismatched ownerId = %v, want not found", err)
	}

	_, foreign := svc.GetBlueprintByID(ctxAs("eve"), "blp-b", "")
	_, missing := svc.GetBlueprintByID(ctxAs("eve"), "blp-missing", "")
	if !errors.Is(foreign, core.ErrForbidden) || !errors.Is(missing, core.ErrNotFound) {
		t.Fatalf("non-member = %v, missing = %v; want the typed-id 403 and a 404", foreign, missing)
	}
	if err := svc.DisconnectBlueprint(ctxAs("eve"), "blp-b", ""); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-member DisconnectBlueprint = %v, want forbidden", err)
	}
	if b := fs.blueprints["blp-b"]; b.Status == "disconnected" {
		t.Fatal("a non-member disconnected another workspace's Blueprint")
	}
}

// w4/m172: validating a manifest on behalf of an existing Blueprint resolves
// that Blueprint's own workspace too, so the dry run of a non-default
// workspace's Blueprint no longer 404s without ownerId.
func TestValidateBlueprintForAnIDResolvesTheOwningWorkspace(t *testing.T) {
	fs := newFakeBlueprintStore(store.Blueprint{ID: "blp-b", TenantID: "tea-b", Name: "bravo", Repo: "https://github.com/acme/app", Branch: "main", Status: "active"})
	svc := newBlueprintService(fs, coretest.Members{"dana": {"tea-a", "tea-b"}, "eve": {"tea-a"}})
	const manifest = "services:\n  - type: web\n    name: web\n    runtime: image\n    image:\n      url: nginx:1\n"

	if _, err := svc.ValidateBlueprint(ctxAs("dana"), "", manifest, "blp-b"); err != nil {
		t.Fatalf("ValidateBlueprint without ownerId = %v, want the tea-b Blueprint found", err)
	}
	if _, err := svc.ValidateBlueprint(ctxAs("dana"), "tea-a", manifest, "blp-b"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("mismatched ownerId = %v, want not found", err)
	}
	_, foreign := svc.ValidateBlueprint(ctxAs("eve"), "", manifest, "blp-b")
	_, missing := svc.ValidateBlueprint(ctxAs("eve"), "", manifest, "blp-missing")
	if !errors.Is(foreign, core.ErrForbidden) || !errors.Is(missing, core.ErrNotFound) {
		t.Fatalf("non-member = %v, missing = %v; want the typed-id 403 and a 404", foreign, missing)
	}
	// A new manifest (no id) still validates in the caller's workspace.
	if _, err := svc.ValidateBlueprint(ctxAs("eve"), "", manifest, ""); err != nil {
		t.Fatalf("ValidateBlueprint without an id = %v", err)
	}
}
