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

package projects

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w5/086: a project read by id answers like every by-id family. A workspace
// the request names decides: a member of it is told the project is not found
// there, a non-member of it is refused, and an unnamed request reaches the
// project in its own workspace.
func TestProjectByIDAnswersNotFoundInANamedWorkspaceThatDoesNotHoldIt(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-b", TenantID: "tea-b", Name: "bravo"})
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	svc := &Service{Base: &core.Base{Authz: allowChecker{}, Workspace: coretest.Workspaces{"tea-a", "tea-b"}}, Store: st}

	if _, err := svc.Get(core.WithWorkspace(ctx, "tea-a"), "prj-b"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Get naming tea-a = %v, want not found", err)
	}
	for name, c := range map[string]context.Context{"named owner": core.WithWorkspace(ctx, "tea-b"), "unnamed": ctx} {
		if got, err := svc.Get(c, "prj-b"); err != nil || got.ID != "prj-b" {
			t.Fatalf("Get %s = %+v, %v", name, got, err)
		}
	}

	// As for Apps and datastores, membership of the named workspace decides:
	// a member without the relation there is still told "not found", and a
	// non-member of it is refused.
	noRoleInA := &Service{Base: &core.Base{Authz: denyObjectChecker(core.WorkspaceObject("tea-a")), Workspace: coretest.Workspaces{"tea-a", "tea-b"}}, Store: st}
	if _, err := noRoleInA.Get(core.WithWorkspace(ctx, "tea-a"), "prj-b"); !errors.Is(err, core.ErrNotInNamedWorkspace) {
		t.Fatalf("Get naming a workspace the caller belongs to = %v, want not found there", err)
	}
	outsider := &Service{Base: &core.Base{Authz: allowChecker{}, Workspace: coretest.Workspaces{"tea-b"}}, Store: st}
	if _, err := outsider.Get(core.WithWorkspace(ctx, "tea-a"), "prj-b"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Get naming a workspace the caller is not a member of = %v, want forbidden", err)
	}
}
