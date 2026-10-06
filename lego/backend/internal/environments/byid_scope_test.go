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

package environments

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/m172: an environment id resolves its own workspace, so the verb's leading
// check runs there — a member whose role lives only in that workspace is not
// refused by their default workspace — and a non-member's typed id is a 403
// while a missing one is a 404 (ADR072 #8, w4/199).
func TestEnvironmentByIDVerbsResolveTheOwningWorkspace(t *testing.T) {
	st := newFakeStore()
	env, err := st.CreateEnvironment(context.Background(), "prj-b", "tea-b", "staging")
	if err != nil {
		t.Fatal(err)
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	onlyB := denyObjectChecker(core.WorkspaceObject("tea-a")) // dana holds no role in her default
	member := &Service{Base: &core.Base{Authz: onlyB, Workspace: coretest.Workspaces{"tea-a", "tea-b"}}, Store: st}
	if got, err := member.Get(ctx, env.ID); err != nil || got.ID == "" {
		t.Fatalf("Get by a tea-b member = %+v, %v", got, err)
	}

	outsider := &Service{Base: &core.Base{Authz: allowChecker{}, Workspace: coretest.Workspaces{"tea-c"}}, Store: st}
	_, foreign := outsider.Get(ctx, env.ID)
	_, missing := outsider.Get(ctx, "evm-d0000000000000000000")
	if !errors.Is(foreign, core.ErrForbidden) || !errors.Is(missing, core.ErrNotFound) {
		t.Fatalf("non-member Get = %v, missing = %v; want forbidden vs not found (ADR072 #8)", foreign, missing)
	}
	if err := outsider.Delete(ctx, env.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-member Delete = %v, want forbidden", err)
	}
}

// w5/m115: an environment is created in its project's workspace, authorized
// once there. A caller who administers the project's workspace but holds no
// role in their default is allowed, and a refusal is recorded in the
// project's workspace, not the caller's default.
func TestCreateEnvironmentAuthorizesInTheProjectsWorkspace(t *testing.T) {
	st := newFakeStore()
	st.projs["prj-b"] = store.Project{ID: "prj-b", TenantID: "tea-b", Name: "bravo"}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})

	audit := &recordingSink{}
	onlyB := denyObjectChecker(core.WorkspaceObject("tea-a")) // dana holds no role in her default
	member := &Service{Base: &core.Base{Authz: onlyB, Audit: audit, Workspace: coretest.Workspaces{"tea-a", "tea-b"}}, Store: st}
	got, err := member.Create(ctx, "prj-b", "staging")
	if err != nil || got.ID == "" {
		t.Fatalf("Create in a tea-b project = %+v, %v", got, err)
	}
	if want := []string{core.WorkspaceObject("tea-b")}; !slices.Equal(audit.resources(), want) {
		t.Fatalf("audit objects = %v, want one row on %v", audit.resources(), want)
	}

	audit.events = nil
	viewer := &Service{Base: &core.Base{Authz: denyObjectChecker(core.WorkspaceObject("tea-b")), Audit: audit, Workspace: coretest.Workspaces{"tea-a", "tea-b"}}, Store: st}
	if _, err := viewer.CreateWithACL(ctx, CreateEnvironmentRequest{ProjectID: "prj-b", Name: "prod"}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Create without a role in the project's workspace = %v, want forbidden", err)
	}
	if want := []string{core.WorkspaceObject("tea-b")}; !slices.Equal(audit.resources(), want) {
		t.Fatalf("refusal audit objects = %v, want one row on %v", audit.resources(), want)
	}

	// A named workspace that does not hold the project reads not-found, even
	// for a caller who may create there.
	admin := &Service{Base: &core.Base{Authz: allowChecker{}, Workspace: coretest.Workspaces{"tea-a", "tea-b"}}, Store: st}
	if _, err := admin.Create(core.WithWorkspace(ctx, "tea-a"), "prj-b", "dev"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Create naming tea-a for a tea-b project = %v, want not found", err)
	}
}

// An environment write is one decision, recorded once in the environment's
// workspace: routing makes the verb's own gate the environment's, so the fetch
// must not authorize it a second time. SetACL's gate is can_manage itself.
func TestEnvironmentWritesRecordOneDecision(t *testing.T) {
	st := newFakeStore()
	env, err := st.CreateEnvironment(context.Background(), "prj-b", "tea-b", "staging")
	if err != nil {
		t.Fatal(err)
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	member := func(authz core.Checker) (*Service, *recordingSink) {
		audit := &recordingSink{}
		return &Service{Base: &core.Base{Authz: authz, Audit: audit, Workspace: coretest.Workspaces{"tea-a", "tea-b"}}, Store: st}, audit
	}
	once := func(audit *recordingSink, relation string, outcome core.AuditOutcome) {
		t.Helper()
		if len(audit.events) != 1 {
			t.Fatalf("audit = %+v, want one row", audit.events)
		}
		if ev := audit.events[0]; ev.Resource != core.WorkspaceObject("tea-b") || ev.Relation != relation || ev.Outcome != outcome {
			t.Fatalf("audit row = %s %s %s, want %s %s on %s", ev.Relation, ev.Outcome, ev.Resource, relation, outcome, core.WorkspaceObject("tea-b"))
		}
	}

	admin, audit := member(allowChecker{})
	if _, err := admin.Rename(ctx, env.ID, "stage"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	once(audit, core.RelCanCreate, core.AuditAllowed)

	admin, audit = member(allowChecker{})
	if _, err := admin.SetACL(ctx, env.ID, ProtectedStatusProtected, false, nil); err != nil {
		t.Fatalf("SetACL as admin: %v", err)
	}
	once(audit, core.RelCanManage, core.AuditAllowed)

	developer, audit := member(developerChecker{})
	if _, err := developer.SetACL(ctx, env.ID, ProtectedStatusUnprotected, false, nil); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("SetACL as developer = %v, want forbidden", err)
	}
	once(audit, core.RelCanManage, core.AuditDenied)

	// A patch that renames AND arms an ACL takes can_manage before anything
	// is written, so a developer's attempt leaves the name alone.
	developer, audit = member(developerChecker{})
	name, protected := "renamed-by-developer", ProtectedStatusProtected
	if _, err := developer.Update(ctx, env.ID, EnvironmentPatch{Name: &name, ProtectedStatus: &protected}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Update(name+ACL) as developer = %v, want forbidden", err)
	}
	once(audit, core.RelCanManage, core.AuditDenied)
	if got, _ := st.GetEnvironment(context.Background(), env.ID); got.Name != "stage" {
		t.Fatalf("refused Update renamed the environment to %q", got.Name)
	}

	admin, audit = member(allowChecker{})
	if _, err := admin.Update(ctx, env.ID, EnvironmentPatch{Name: &name, ProtectedStatus: &protected}); err != nil {
		t.Fatalf("Update(name+ACL) as admin: %v", err)
	}
	once(audit, core.RelCanManage, core.AuditAllowed)
}

// The matrix's "named workspace without the id → 404" for environment verbs,
// and create without a workspace resolver: a grant on the default object must
// not reach another workspace's project (w5/m115 review).
func TestEnvironmentVerbsStayInTheActingWorkspace(t *testing.T) {
	st := newFakeStore()
	st.projs["prj-b"] = store.Project{ID: "prj-b", TenantID: "tea-b", Name: "bravo"}
	env, err := st.CreateEnvironment(context.Background(), "prj-b", "tea-b", "staging")
	if err != nil {
		t.Fatal(err)
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})

	admin := &Service{Base: &core.Base{Authz: allowChecker{}, Workspace: coretest.Workspaces{"tea-a", "tea-b"}}, Store: st}
	if _, err := admin.Get(core.WithWorkspace(ctx, "tea-a"), env.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Get naming tea-a for a tea-b environment = %v, want not found", err)
	}

	unresolved := &Service{Base: &core.Base{Authz: denyObjectChecker(core.WorkspaceObject("tea-b"))}, Store: st}
	if _, err := unresolved.Create(ctx, "prj-b", "dev"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Create with no workspace resolver = %v, want the project's own workspace to refuse", err)
	}

	// Arming an ACL raises the gate to can_manage, but a named workspace that
	// does not hold the project still reads not-found, and nothing is checked
	// or recorded in the project's workspace.
	audit := &recordingSink{}
	mallory := &Service{Base: &core.Base{Authz: denyObjectChecker(core.WorkspaceObject("tea-b")), Audit: audit, Workspace: coretest.Workspaces{"tea-a"}}, Store: st}
	if _, err := mallory.CreateWithACL(core.WithWorkspace(ctx, "tea-a"), CreateEnvironmentRequest{ProjectID: "prj-b", Name: "prod", ProtectedStatus: ProtectedStatusProtected}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("CreateWithACL naming tea-a for a tea-b project = %v, want not found", err)
	}
	if slices.Contains(audit.resources(), core.WorkspaceObject("tea-b")) {
		t.Fatalf("audit objects = %v; the project's workspace was consulted", audit.resources())
	}
}
