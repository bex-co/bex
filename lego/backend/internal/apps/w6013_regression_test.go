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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w6013_regression_test.go is the permanent regression test for w6/013,
// promoted to w6/m17: a resource-scoped verb used to run TWO gates —
// s.Authorize(relation) against the CALLER'S DEFAULT workspace, then the
// shared fetch against the RESOURCE'S workspace (w6/m14) — so effective
// permission was their INTERSECTION. Because EnsureTenant redeems a pending
// invite before minting the invitee's own personal tenant, an invited
// viewer's default workspace is often the one they were invited into, not the
// one they own — so they could not restart, suspend, delete, or set env vars
// on services in their OWN admin-owned workspace. core.Base.AuthorizeApp
// collapsed the two gates into one, authorized against the resource's own
// workspace, closing the bug at its root.
//
// The exact scenario from the source note (.pm/w6/done/013.md):
//
//	bob: default = tea-team (invited VIEWER), own = tea-mine (admin),
//	     service "mine-web" in tea-mine
//	svc.Restart(bob, "mine-web") => forbidden        # was: checked can_operate
//	                                                  # against tea-team
func TestW6013_InvitedViewerCanOperateTheirOwnWorkspacesService(t *testing.T) {
	cl := fakeClient(tenantApp("mine-web", "tea-mine"))
	svc := &Service{Base: &core.Base{
		Client:    cl,
		Namespace: "default",
		// bob's oldest (default) membership is tea-team — where he was
		// invited as a viewer; he separately owns tea-mine, where "mine-web"
		// actually lives.
		Workspace: twoWorkspaces{"bob": {"tea-team", "tea-mine"}},
		// bob is a viewer of tea-team (can_operate denied there) and admin
		// of tea-mine (can_operate allowed there).
		Authz: &fakeChecker{deny: core.WorkspaceObject("tea-team")},
	}}

	if _, err := svc.Restart(ctxAs("bob"), "mine-web"); err != nil {
		t.Fatalf("Restart on bob's own service: got %v, want it served — "+
			"the service's own workspace (tea-mine) must decide, not bob's default (tea-team)", err)
	}
}

// The sibling verbs w6/013 names explicitly (suspend, delete, set env vars)
// hit the exact same bug, through the exact same seam — swept here rather
// than duplicated per verb.
func TestW6013_InvitedViewerCanSuspendDeleteAndConfigureTheirOwnWorkspacesService(t *testing.T) {
	newBobService := func(t *testing.T) *Service {
		t.Helper()
		cl := fakeClient(tenantApp("mine-web", "tea-mine"))
		return &Service{Base: &core.Base{
			Client:    cl,
			Namespace: "default",
			Workspace: twoWorkspaces{"bob": {"tea-team", "tea-mine"}},
			Authz:     &fakeChecker{deny: core.WorkspaceObject("tea-team")},
		}}
	}

	t.Run("Suspend", func(t *testing.T) {
		svc := newBobService(t)
		if _, err := svc.Suspend(ctxAs("bob"), "mine-web"); err != nil {
			t.Errorf("Suspend on bob's own service: got %v, want it served", err)
		}
	})
	t.Run("SetIdleTTL", func(t *testing.T) {
		svc := newBobService(t)
		if _, err := svc.SetIdleTTL(ctxAs("bob"), "mine-web", 3600); err != nil {
			t.Errorf("SetIdleTTL on bob's own service: got %v, want it served", err)
		}
	})
	t.Run("Delete", func(t *testing.T) {
		svc := newBobService(t)
		if err := svc.Delete(ctxAs("bob"), "mine-web"); err != nil {
			t.Errorf("Delete on bob's own service: got %v, want it served", err)
		}
	})
}
