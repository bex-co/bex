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
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/m169: a Blueprint id resolves its own workspace, like a service or
// env-group id, so a caller who belongs to several workspaces needs no ownerId.
// A workspace the caller is not in answers exactly like a missing id.
func TestBlueprintByIDVerbsResolveTheOwningWorkspace(t *testing.T) {
	fs := newFakeBlueprintStore(store.Blueprint{ID: "blp-b", TenantID: "tea-b", Name: "bravo", Repo: "https://github.com/acme/app", Branch: "main", Status: "active"})
	svc := newBlueprintService(fs, memberships{"dana": {"tea-a", "tea-b"}, "eve": {"tea-a"}})
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

	// A non-member gets the same answer as for an id that does not exist.
	_, foreign := svc.GetBlueprintByID(ctxAs("eve"), "blp-b", "")
	_, missing := svc.GetBlueprintByID(ctxAs("eve"), "blp-missing", "")
	if !errors.Is(foreign, core.ErrNotFound) || foreign.Error() != missing.Error() {
		t.Fatalf("non-member = %v, missing = %v; want identical not-found answers", foreign, missing)
	}
	if err := svc.DisconnectBlueprint(ctxAs("eve"), "blp-b", ""); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("non-member DisconnectBlueprint = %v, want not found", err)
	}
	if b := fs.blueprints["blp-b"]; b.Status == "disconnected" {
		t.Fatal("a non-member disconnected another workspace's Blueprint")
	}
}
