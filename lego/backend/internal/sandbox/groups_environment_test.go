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

package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// w22/m412: a sandbox group can be placed in a project environment, as in
// Render's dashboard. The surfaces' shared behavior is in
// TestGroupBehaviorsAgreeAcrossRESTGraphQLAndMCP; this file holds what that
// table cannot say: the exact refusal bodies, what a refusal writes, and how a
// group outlives its environment.

// managerChecker is a workspace member whose role decides can_manage, as the
// protected-environment placement gate asks it (w3/m281): a Developer (admin
// false) holds every other relation, an Admin all of them.
type managerChecker struct{ admin bool }

func (c managerChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return c.admin || relation != core.RelCanManage, nil
}

// memberOfAll makes the caller a member of every workspace listed, the first
// being its default.
type memberOfAll []string

func (m memberOfAll) Tenant(context.Context, core.Identity) (string, bool) { return m[0], true }

func (m memberOfAll) IsMember(_ context.Context, _ core.Identity, workspace string) (bool, error) {
	return slices.Contains(m, workspace), nil
}

// environmentOf creates a project and an environment in a workspace (protected
// when asked) and returns the environment's public id, as the API shows it.
func (f *groupFixture) environmentOf(t *testing.T, workspace, name string, protected bool) string {
	t.Helper()
	ctx := context.Background()
	project, err := f.st.CreateProject(ctx, workspace, name)
	if err != nil {
		t.Fatal(err)
	}
	env, err := f.st.CreateEnvironment(ctx, project.ID, workspace, name)
	if err != nil {
		t.Fatal(err)
	}
	if protected {
		if err := f.st.SetEnvironmentACL(ctx, env.ID, core.ProtectedStatusProtected, false, env.IPAllowList); err != nil {
			t.Fatal(err)
		}
	}
	f.lastEnvironment = ids.EnvironmentPublicID(env.ID)
	return f.lastEnvironment
}

// environment is environmentOf the fixture's own workspace.
func (f *groupFixture) environment(t *testing.T, name string, protected bool) string {
	t.Helper()
	return f.environmentOf(t, f.ws, name, protected)
}

// createGroupIn creates a group over REST in an environment and returns the
// raw status and body.
func (f *groupFixture) createGroupIn(t *testing.T, name, environmentID string) (int, string) {
	t.Helper()
	return f.do(t, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"name":%q,"environmentId":%q}`, name, environmentID))
}

func (f *groupFixture) mustCreateGroupIn(t *testing.T, name, environmentID string) Group {
	t.Helper()
	code, body := f.createGroupIn(t, name, environmentID)
	var g Group
	if err := json.Unmarshal([]byte(body), &g); code != http.StatusCreated || err != nil {
		t.Fatalf("create %q in %s = %d %s", name, environmentID, code, body)
	}
	return g
}

func environmentOfGroup(g Group) string {
	if g.EnvironmentID == nil {
		return "<null>"
	}
	return *g.EnvironmentID
}

// A group placed in an environment of its workspace reads it back as the
// environment's public evm- id on create, get and list, and a legacy env- id
// places it in the same environment. A group in none reads null.
func TestGroupEchoesItsEnvironment(t *testing.T) {
	f := newGroupFixture(t)
	env := f.environment(t, "staging", false)

	placed := f.mustCreateGroupIn(t, "placed", env)
	if environmentOfGroup(placed) != env || !strings.HasPrefix(env, "evm-") {
		t.Fatalf("create answered environmentId %s, want the public id %s", environmentOfGroup(placed), env)
	}
	legacy := f.mustCreateGroupIn(t, "legacy", ids.EnvironmentStorageID(env))
	if environmentOfGroup(legacy) != env {
		t.Fatalf("a legacy env- id placed the group in %s, want %s", environmentOfGroup(legacy), env)
	}
	plain := f.createGroup(t, "plain")
	if plain.EnvironmentID != nil {
		t.Fatalf("a group created with no environment reads %s, want null", *plain.EnvironmentID)
	}

	code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+placed.ID, "")
	var got Group
	if err := json.Unmarshal([]byte(body), &got); code != http.StatusOK || err != nil || environmentOfGroup(got) != env {
		t.Fatalf("get = %d %s, want environmentId %s", code, body, env)
	}
	byName := map[string]string{}
	for _, g := range f.listGroups(t) {
		byName[g.Name] = environmentOfGroup(g)
	}
	if byName["placed"] != env || byName["legacy"] != env || byName["plain"] != "<null>" {
		t.Fatalf("list environments = %v, want placed and legacy in %s and plain in none", byName, env)
	}

	// The other group verbs answer with the group as it is: an update and a
	// default swap neither move nor clear the placement.
	if g := f.setLimit(t, placed, 3); environmentOfGroup(g) != env {
		t.Errorf("update answered environmentId %s, want %s", environmentOfGroup(g), env)
	}
	code, body = f.do(t, http.MethodPost, "/v1/sandbox-groups/"+placed.ID+"/set-default", "")
	var swapped Group
	if err := json.Unmarshal([]byte(body), &swapped); code != http.StatusOK || err != nil || environmentOfGroup(swapped) != env {
		t.Errorf("set-default = %d %s, want environmentId %s", code, body, env)
	}
}

// Another workspace's environment answers exactly as an id that does not exist:
// the same status, code and message, so a probe learns nothing about whether
// the id names an environment elsewhere. Neither writes a group.
func TestAnotherWorkspacesEnvironmentIsNotFoundLikeAnUnknownOne(t *testing.T) {
	f := newGroupFixture(t)
	wsB := f.addWorkspace(t, "id-b")
	foreign := f.environmentOf(t, wsB, "theirs", false)
	unknown := ids.EnvironmentPublicID("env-" + strings.Repeat("a", 20))

	foreignCode, foreignBody := f.createGroupIn(t, "mine", foreign)
	unknownCode, unknownBody := f.createGroupIn(t, "mine", unknown)
	if foreignCode != http.StatusNotFound || errCode(foreignBody) != core.NotFoundCode {
		t.Fatalf("another workspace's environment = %d %s, want 404 %s", foreignCode, foreignBody, core.NotFoundCode)
	}
	if unknownCode != foreignCode || unknownBody != foreignBody {
		t.Fatalf("an unknown environment answered %d %s, another workspace's answered %d %s; they must be identical", unknownCode, unknownBody, foreignCode, foreignBody)
	}
	// The member of the other workspace places the group there, so the refusal
	// is the caller's workspace, not the environment.
	code, body := f.doAs(t, "id-b", http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"theirs","environmentId":%q}`, wsB, foreign))
	if code != http.StatusCreated {
		t.Fatalf("the owning workspace's own create = %d %s", code, body)
	}
	if n := len(f.listGroups(t)); n != 0 {
		t.Fatalf("a refused create left %d groups in the workspace", n)
	}
}

// A protected environment needs an Admin to place a group in it (Render: only
// workspace Admins create resources in a protected environment, w3/m281). The
// refusal is the shared coded 403, writes no group, and an Admin succeeds.
func TestProtectedEnvironmentNeedsAnAdminToPlaceAGroup(t *testing.T) {
	f := newGroupFixture(t)
	prod := f.environment(t, "production", true)
	staging := f.environment(t, "staging", false)

	f.svc.Base.Authz = managerChecker{admin: false}
	code, body := f.createGroupIn(t, "in-prod", prod)
	if code != http.StatusForbidden || errCode(body) != core.ProtectedPlacementCode {
		t.Fatalf("developer into a protected environment = %d %s, want 403 %s", code, body, core.ProtectedPlacementCode)
	}
	if n := len(f.listGroups(t)); n != 0 {
		t.Fatalf("a refused placement left %d groups", n)
	}
	// An unprotected environment, and none, stay open to a Developer.
	f.mustCreateGroupIn(t, "in-staging", staging)
	f.createGroup(t, "in-none")

	f.svc.Base.Authz = managerChecker{admin: true}
	if g := f.mustCreateGroupIn(t, "in-prod", prod); environmentOfGroup(g) != prod {
		t.Fatalf("admin placed the group in %s, want %s", environmentOfGroup(g), prod)
	}
}

// Deleting the environment keeps the group, clears the field on every read and
// changes nothing else about it, including the sandboxes in it.
func TestDeletingTheEnvironmentKeepsTheGroupAndItsSandboxes(t *testing.T) {
	f := newGroupFixture(t)
	env := f.environment(t, "doomed", false)
	g := f.mustCreateGroupIn(t, "survivor", env)
	box := f.mustCreate(t, g.ID)

	if err := f.st.DeleteEnvironment(context.Background(), ids.EnvironmentStorageID(env)); err != nil {
		t.Fatal(err)
	}
	code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+g.ID, "")
	var got Group
	if err := json.Unmarshal([]byte(body), &got); code != http.StatusOK || err != nil {
		t.Fatalf("get after the environment's delete = %d %s, want the group", code, body)
	}
	if got.EnvironmentID != nil || got.Name != "survivor" || got.ID != g.ID || !got.IsDefault {
		t.Fatalf("group after the delete = %+v, want the same default group in no environment", got)
	}
	if listed := f.listIDs(t, "&sandboxGroupId="+g.ID); len(listed) != 1 || listed[0] != box.ID {
		t.Fatalf("the group's sandboxes after the delete = %v, want %s", listed, box.ID)
	}
}

// A deployment that cannot resolve environments refuses a create that names
// one, rather than storing an id nothing validated, and still creates a group
// that names none.
func TestPlacementWithoutAnEnvironmentResolverIsRefused(t *testing.T) {
	f := newGroupFixture(t)
	env := f.environment(t, "staging", false)
	f.svc.Environments = nil

	code, body := f.createGroupIn(t, "needs-resolver", env)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("create naming an environment with no resolver = %d %s, want 503", code, body)
	}
	if n := len(f.listGroups(t)); n != 0 {
		t.Fatalf("the refused create left %d groups", n)
	}
	f.createGroup(t, "no-environment")
}

// The environment is bound by the workspace the group is created in, not the
// caller's default: a member of two workspaces places a group in the
// environment of the workspace named by ownerId.
func TestPlacementUsesTheNamedWorkspace(t *testing.T) {
	f := newGroupFixture(t)
	wsB := f.addWorkspace(t, "id-b")
	f.svc.Base.Workspace = memberOfAll{f.ws, wsB}
	inB := f.environmentOf(t, wsB, "b-staging", false)
	inA := f.environment(t, "a-staging", false)

	code, body := f.do(t, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"in-b","environmentId":%q}`, wsB, inB))
	var g Group
	if err := json.Unmarshal([]byte(body), &g); code != http.StatusCreated || err != nil || environmentOfGroup(g) != inB || g.OwnerID != wsB {
		t.Fatalf("create in workspace B with B's environment = %d %s", code, body)
	}
	code, body = f.do(t, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"cross","environmentId":%q}`, wsB, inA))
	if code != http.StatusNotFound || errCode(body) != core.NotFoundCode {
		t.Fatalf("workspace B with workspace A's environment = %d %s, want 404 %s", code, body, core.NotFoundCode)
	}
}
