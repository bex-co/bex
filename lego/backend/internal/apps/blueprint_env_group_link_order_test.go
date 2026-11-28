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
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Blueprint plan that approves a sync's end state must be executable: every
// fromGroup link the plan's end-state model blesses must pass the apply's
// link guards, which judge the group's environment at link time. The apply
// places services (and links them) before it moves env groups
// (applyStackEnvGroupMoves runs after the service writes), so a link the file
// adds to a group moving into the service's environment must land after the
// move, not before it — and a carrier link, which can never share an
// environment, must strand the move in the plan the way a stored one does.

// orderedEnvGroups is a fakeEnvGroups that enforces Render's scoping rule at
// link and move time exactly as the real envgroups.Service does: a link lands
// only when the group is workspace-scoped or the service sits in the group's
// current environment, judged at link time; a group move into an environment
// lands only while every linked service sits in the target environment,
// judged at move time. Group environments track SetGroupEnvironment;
// service environments are read live from the fake client's App labels, the
// same source the apply's guards read. The rule itself is pinned by
// real-verb tests in envgroups; this fake exists to pin the apply's
// link/move ORDER through the real DeployStack orchestration.
type orderedEnvGroups struct {
	*fakeEnvGroups
	base  client.Client
	calls []string // "link group->service" / "move group->env" in call order
}

func (g *orderedEnvGroups) groupEnv(name string) string {
	return g.environments[name]
}

func (g *orderedEnvGroups) serviceEnv(ref string) string {
	var list appv1alpha1.AppList
	if err := g.base.List(context.Background(), &list); err != nil {
		return ""
	}
	for i := range list.Items {
		if stackServiceRef(&list.Items[i]) == ref {
			return list.Items[i].Labels[core.LabelEnvironment]
		}
	}
	return ""
}

func scopingRefusal(code string) error {
	return core.NewConflictError(code, "test seam refuses what the real guard refuses", map[string]any{})
}

func (g *orderedEnvGroups) LinkEnvGroup(ctx context.Context, name, service string) error {
	g.calls = append(g.calls, "link "+name+"->"+service)
	if env := g.groupEnv(name); env != "" && g.serviceEnv(service) != env {
		return scopingRefusal("ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH")
	}
	return g.fakeEnvGroups.LinkEnvGroup(ctx, name, service)
}

func (g *orderedEnvGroups) WithInitialEnvGroups(ctx context.Context, names []string, a *appv1alpha1.App, create, complete func() error) error {
	for _, name := range names {
		g.calls = append(g.calls, "link "+name+"->"+stackServiceRef(a))
		if env := g.groupEnv(name); env != "" && a.Labels[core.LabelEnvironment] != env {
			return scopingRefusal("ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH")
		}
	}
	return g.fakeEnvGroups.WithInitialEnvGroups(ctx, names, a, create, complete)
}

func (g *orderedEnvGroups) SetGroupEnvironment(ctx context.Context, name, environmentID string) error {
	g.calls = append(g.calls, "move "+name+"->"+environmentID)
	if environmentID != "" {
		for _, link := range g.links {
			lname, ref, _ := strings.Cut(link, "->")
			if lname != name {
				continue
			}
			if got := g.serviceEnv(ref); got != environmentID {
				return scopingRefusal("ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES")
			}
		}
	}
	return g.fakeEnvGroups.SetGroupEnvironment(ctx, name, environmentID)
}

// linkOrderFixture holds web (unlinked) and cfg together in staging, with an
// empty production beside them.
const linkOrderFixture = `projects:
  - name: shop
    environments:
      - name: staging
        services:
          - name: web
            type: web
            runtime: image
            image: {url: nginx:1}
            plan: starter
        envVarGroups:
          - name: cfg
            envVars:
              - {key: LOG_LEVEL, value: info}
      - name: production
`

func linkOrderService(t *testing.T) (*Service, context.Context, *orderedEnvGroups, *blueprintGroupingTestStore) {
	t.Helper()
	svc, base, _, ctx, st := groupingStackService(t)
	groups := &orderedEnvGroups{fakeEnvGroups: newFakeEnvGroups(), base: base}
	svc.EnvGroups = groups
	svc.EnvGroupPlacements = func(context.Context) (map[string]string, error) { return groups.environments, nil }
	svc.EnvGroupWritesOwed = groups.writesOwed
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: linkOrderFixture}); err != nil {
		t.Fatalf("deploy link-order fixture: %v", err)
	}
	return svc, ctx, groups, st
}

func storageEnvID(t *testing.T, st *blueprintGroupingTestStore, project, environment string) string {
	t.Helper()
	parts := strings.SplitN(groupingAt(t, st, project, environment), "/", 2)
	if len(parts) != 2 {
		t.Fatalf("grouping %s/%s = %q, want project/env", project, environment, groupingAt(t, st, project, environment))
	}
	return parts[1]
}

func linkCallIndex(calls []string, prefix string) int {
	for i, c := range calls {
		if strings.HasPrefix(c, prefix) {
			return i
		}
	}
	return -1
}

// A sync that moves a service into an environment, moves a group into the
// same environment, and links the two is end-state valid: the plan approves
// it, so the apply must perform it. The link judges the group's environment
// at link time, so it must land after the group moves.
func TestBlueprintApplyLinksAfterItMovesTheGroup(t *testing.T) {
	svc, ctx, groups, st := linkOrderService(t)
	production := storageEnvID(t, st, "shop", "production")
	manifest := `projects:
  - name: shop
    environments:
      - name: staging
      - name: production
        services:
          - name: web
            type: web
            runtime: image
            image: {url: nginx:1}
            plan: starter
            envVars:
              - {fromGroup: cfg}
        envVarGroups:
          - name: cfg
            envVars:
              - {key: LOG_LEVEL, value: info}
`
	v, err := svc.ValidateBlueprint(ctx, "tea-a", manifest, "")
	if err != nil || !v.Valid {
		t.Fatalf("validate co-move with a new link = %+v err=%v, want the plan to approve the end state", v, err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest}); err != nil {
		t.Fatalf("deploy the approved co-move with a new link: %v", err)
	}
	if got := groups.environments["cfg"]; got != production {
		t.Fatalf("group env = %q, want the move to %q to have landed", got, production)
	}
	move := linkCallIndex(groups.calls, "move cfg->")
	link := linkCallIndex(groups.calls, "link cfg->")
	if link < 0 {
		t.Fatalf("calls = %v, want the new link to have landed", groups.calls)
	}
	if move < 0 || link < move {
		t.Fatalf("calls = %v, want the link to land after the group move", groups.calls)
	}
}

// A service the same sync creates links a co-moving group the same way: its
// initial links wait out the move, then land on its target environment.
func TestBlueprintApplyCreatedServiceLinksAfterTheGroupMoves(t *testing.T) {
	svc, ctx, groups, st := linkOrderService(t)
	production := storageEnvID(t, st, "shop", "production")
	manifest := `projects:
  - name: shop
    environments:
      - name: staging
      - name: production
        services:
          - name: api
            type: web
            runtime: image
            image: {url: nginx:1}
            plan: starter
            envVars:
              - {fromGroup: cfg}
        envVarGroups:
          - name: cfg
            envVars:
              - {key: LOG_LEVEL, value: info}
`
	v, err := svc.ValidateBlueprint(ctx, "tea-a", manifest, "")
	if err != nil || !v.Valid {
		t.Fatalf("validate create with a new link to a moving group = %+v err=%v, want the plan to approve the end state", v, err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest}); err != nil {
		t.Fatalf("deploy the approved create with a new link: %v", err)
	}
	if got := groups.environments["cfg"]; got != production {
		t.Fatalf("group env = %q, want the move to %q to have landed", got, production)
	}
	move := linkCallIndex(groups.calls, "move cfg->")
	link := linkCallIndex(groups.calls, "link cfg->")
	if link < 0 {
		t.Fatalf("calls = %v, want the created service's link to have landed", groups.calls)
	}
	if move < 0 || link < move {
		t.Fatalf("calls = %v, want the created service's link to land after the group move", groups.calls)
	}
}

// A build source's carrier never has an environment, so a fromGroup link the
// file adds from a build source strands a group move into an environment the
// same way a stored carrier link does: the plan must refuse it with the
// apply's code instead of approving a sync the apply fails.
func TestBlueprintPlanRefusesBuildSourceLinkStrandingAGroupMove(t *testing.T) {
	svc, ctx, _, _ := linkOrderService(t)
	manifest := `projects:
  - name: shop
    buildSources:
      - name: mono
        git:
          repo: https://github.com/acme/mono
          runtime: docker
          envVars:
            - {fromGroup: cfg}
    environments:
      - name: staging
      - name: production
        envVarGroups:
          - name: cfg
            envVars:
              - {key: LOG_LEVEL, value: info}
`
	v, err := svc.ValidateBlueprint(ctx, "tea-a", manifest, "")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if v.Valid || len(v.Errors) != 1 {
		t.Fatalf("validate = valid=%v errors=%+v, want one refusal for the stranded move", v.Valid, v.Errors)
	}
	if v.Errors[0].Code != "ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES" {
		t.Fatalf("refusal code = %q, want the apply's ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES", v.Errors[0].Code)
	}
	if !strings.Contains(v.Errors[0].Error, "mono") {
		t.Fatalf("refusal = %q, want it to name the stranding build source", v.Errors[0].Error)
	}
}
