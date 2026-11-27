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
)

// A Blueprint plan refuses a group move its apply would refuse: moving an
// env group into an environment while a linked service would still sit
// outside it is a located refusal, not an "environment" update the sync then
// fails. The apply moves services before groups (applyStackEnvGroupMoves runs
// after the service writes), so a service the file moves along reads in its
// target environment by the time the group moves: only a service still
// sitting elsewhere strands the move. The apply side is pinned by
// TestGroupMoveFollowsTheStoredServiceEnvironment (envgroups).

const linkedWebService = `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
    plan: starter
    envVars:
      - {fromGroup: cfg}
`

const movedGroup = `envVarGroups:
  - name: cfg
    envVars:
      - {key: LOG_LEVEL, value: info}
`

// splitPlacement nests services under one shop environment and the group
// under another, so the plan judges a move of exactly one of them.
func splitPlacement(svcEnv, groupEnv string) string {
	indent := func(s, prefix string) string {
		lines := strings.SplitAfter(s, "\n")
		for i, line := range lines {
			if line != "" {
				lines[i] = prefix + line
			}
		}
		return strings.Join(lines, "")
	}
	var b strings.Builder
	b.WriteString("projects:\n  - name: shop\n    environments:\n")
	envs := []string{"staging", "production"}
	if svcEnv != "" && svcEnv != "staging" && svcEnv != "production" {
		envs = append(envs, svcEnv)
	}
	if groupEnv != "staging" && groupEnv != "production" && groupEnv != svcEnv {
		envs = append(envs, groupEnv)
	}
	for _, env := range envs {
		b.WriteString("      - name: " + env + "\n")
		if svcEnv != "" && env == svcEnv {
			b.WriteString(indent(linkedWebService, "        "))
		}
		if env == groupEnv {
			b.WriteString(indent(movedGroup, "        "))
		}
	}
	return b.String()
}

func strandTestStack(t *testing.T) (*Service, context.Context) {
	t.Helper()
	svc, base, _, ctx, _ := groupingStackService(t)
	groups := newFakeEnvGroups()
	svc.EnvGroups = groups
	svc.EnvGroupPlacements = func(context.Context) (map[string]string, error) { return groups.environments, nil }
	svc.EnvGroupWritesOwed = groups.writesOwed
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: splitPlacement("staging", "staging")}); err != nil {
		t.Fatalf("deploy linked stack: %v", err)
	}
	// The real service commits the link's secret ref onto the App; the fake
	// only logs it, so store what a linked service reads back as.
	app := getTenantApp(t, base, "tea-a", "web")
	app.Spec.EnvFromSecrets = []string{"evg-cfg-env"}
	if err := base.Update(ctx, app); err != nil {
		t.Fatalf("stage the stored link: %v", err)
	}
	return svc, ctx
}

func strandRefusal(t *testing.T, svc *Service, ctx context.Context, manifest string) string {
	t.Helper()
	v, err := svc.ValidateBlueprint(ctx, "tea-a", manifest, "")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if v.Valid || len(v.Errors) != 1 {
		t.Fatalf("validate = valid=%v errors=%+v, want one refusal", v.Valid, v.Errors)
	}
	// The plan carries the apply's code, so callers match one refusal on
	// both sides (the apply side is pinned by
	// TestGroupMoveFollowsTheStoredServiceEnvironment).
	if v.Errors[0].Code != "ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES" {
		t.Fatalf("refusal code = %q, want the apply's ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES", v.Errors[0].Code)
	}
	return v.Errors[0].Error
}

func TestValidateBlueprintRefusesStrandedEnvGroupMove(t *testing.T) {
	svc, ctx := strandTestStack(t)

	// The repro: only the group moves, its linked service stays behind.
	// First with the service re-declared where it is, then with the service
	// out of the file entirely: both strand the same link.
	msg := strandRefusal(t, svc, ctx, splitPlacement("staging", "production"))
	if !strings.Contains(msg, "web") {
		t.Fatalf("stranded move refused with %q, want it to name the stranded service", msg)
	}
	msg = strandRefusal(t, svc, ctx, splitPlacement("", "production"))
	if !strings.Contains(msg, "web") {
		t.Fatalf("stranded move refused with %q, want it to name the stranded service", msg)
	}

	// A service the file moves into the same environment moves along: the
	// apply lands both together, so the plan still names the move.
	v, err := svc.ValidateBlueprint(ctx, "tea-a", splitPlacement("production", "production"), "")
	if err != nil {
		t.Fatalf("validate co-move: %v", err)
	}
	if !v.Valid {
		t.Fatalf("co-move refused with %+v, want the move planned", v.Errors)
	}
	if got := plannedUpdates(v.Plan.Actions); got != "update env_var_group cfg: environment; update service web: environment" &&
		got != "update service web: environment; update env_var_group cfg: environment" {
		t.Fatalf("co-move planned %q, want both moves", got)
	}
}

func TestValidateBlueprintRefusesEnvGroupMovePastItsService(t *testing.T) {
	svc, ctx := strandTestStack(t)

	// The service moves to another environment while the group moves to
	// production: they still land apart.
	msg := strandRefusal(t, svc, ctx, `projects:
  - name: shop
    environments:
      - name: staging
      - name: production
        envVarGroups:
          - name: cfg
            envVars:
              - {key: LOG_LEVEL, value: info}
  - name: blog
    environments:
      - name: production
        services:
          - name: web
            type: web
            runtime: image
            image: {url: nginx:1}
            plan: starter
            envVars:
              - {fromGroup: cfg}
`)
	if !strings.Contains(msg, "web") {
		t.Fatalf("divergent move refused with %q, want it to name the stranded service", msg)
	}
}

func TestValidateBlueprintRefusesGroupMoveIntoNewEnvironment(t *testing.T) {
	svc, ctx := strandTestStack(t)

	// The target environment does not exist yet: the apply creates it and
	// then refuses the group onto it while the service stays. The plan must
	// refuse, not list a create plus a move the sync cannot finish.
	msg := strandRefusal(t, svc, ctx, splitPlacement("staging", "qa"))
	if !strings.Contains(msg, "web") {
		t.Fatalf("move into a new environment refused with %q, want it to name the stranded service", msg)
	}
}

func TestValidateBlueprintGroupMoveWithoutLinksStillPlans(t *testing.T) {
	svc, _, _, ctx, _ := groupingStackService(t)
	groups := newFakeEnvGroups()
	svc.EnvGroups = groups
	svc.EnvGroupPlacements = func(context.Context) (map[string]string, error) { return groups.environments, nil }
	svc.EnvGroupWritesOwed = groups.writesOwed
	deploy := func(manifest string) {
		t.Helper()
		if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest}); err != nil {
			t.Fatalf("deploy: %v", err)
		}
	}
	// No service links the group: the move plans as before, and the guard
	// adds no refusal.
	deploy(groupedResources(movedGroup, "shop", "staging"))
	v, err := svc.ValidateBlueprint(ctx, "tea-a", groupedResources(movedGroup, "shop", "production"), "")
	if err != nil || !v.Valid || v.Plan == nil {
		t.Fatalf("unlinked move = %+v err=%v, want a valid plan", v, err)
	}
	if got := plannedUpdates(v.Plan.Actions); got != "update env_var_group cfg: environment" {
		t.Fatalf("unlinked move planned %q, want the environment move", got)
	}
	// A brand-new link the file adds from a service staying behind strands
	// the move the same way a stored link does.
	deploy(groupedResources(`services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
    plan: starter
`, "shop", "staging"))
	msg := strandRefusal(t, svc, ctx, splitPlacement("staging", "production"))
	if !strings.Contains(msg, "web") {
		t.Fatalf("newly linked move refused with %q, want it to name the stranded service", msg)
	}
}
