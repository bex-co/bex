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
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// blueprintProtectedPlacementProblems is every declared resource a Blueprint
// apply would create in, or move into or out of, a protected environment while
// the caller is not a workspace Admin (render.com/docs/projects; w3/m281). The
// apply runs it before its first write, so a refusal writes nothing, and
// validation reports each as a located entry, so the plan shows it. A resource
// the apply leaves in place is never listed: Render lets non-Admins update
// Blueprint-managed resources in a protected environment, and a Blueprint's
// own (auto-)sync applies whoever pushed (core.BlueprintSync, w3/m292).
//
// A grouping counts as protected when its environment is protected now or the
// manifest arms it, so a new protected environment admits no non-Admin
// creates either — the same answer the create path's resolver gives the apply.
func (s *Service) blueprintProtectedPlacementProblems(ctx context.Context, st parsedStack, snap *workspaceSnapshot) (refused blueprintResourceErrors, cleared bool, err error) {
	tenantID, scoped := s.Tenant(ctx)
	if !scoped || s.BlueprintGroups == nil || core.BlueprintSync(ctx) {
		return nil, false, nil
	}
	placements, err := s.blueprintEnvironmentPlacements(ctx, snap, st.groupings)
	if err != nil || !placements.anyProtected() {
		return nil, false, err
	}
	var candidates []blueprintResourceError
	refuse := func(kind BlueprintResourceKind, name, environmentID string) {
		candidates = append(candidates, blueprintResourceError{kind: kind, name: name, err: core.ProtectedPlacementRefusal(environmentID)})
	}
	services, err := snap.services(ctx)
	if err != nil {
		return nil, false, err
	}
	for _, svc := range st.services {
		current := placementOf{}
		if existing := services[svc.req.Name]; existing != nil {
			current = placedIn(existing.Labels)
		}
		if envID, refused := placements.check(svc.grouping, svc.ungrouped, current); refused {
			refuse(BlueprintResourceService, svc.req.Name, envID)
		}
	}
	if len(st.databases) > 0 {
		databases, err := snap.databases(ctx)
		if err != nil {
			return nil, false, err
		}
		for _, db := range st.databases {
			current := placementOf{}
			if existing, _ := uniqueDatabaseByDisplayName(databases.Items, scoped, tenantID, db.name); existing != nil {
				current = placedIn(existing.Labels)
			}
			if envID, refused := placements.check(db.grouping, db.ungrouped, current); refused {
				refuse(BlueprintResourcePostgres, db.name, envID)
			}
		}
	}
	if len(st.keyValues) > 0 {
		keyValues, err := snap.keyValues(ctx)
		if err != nil {
			return nil, false, err
		}
		for _, kv := range st.keyValues {
			current := placementOf{}
			if existing, _ := uniqueKeyValueByDisplayName(keyValues.Items, scoped, tenantID, kv.name); existing != nil {
				current = placedIn(existing.Labels)
			}
			if envID, refused := placements.check(kv.grouping, kv.ungrouped, current); refused {
				refuse(BlueprintResourceKeyValue, kv.name, envID)
			}
		}
	}
	if len(st.envGroups) > 0 && s.EnvGroups != nil {
		var current map[string]string
		if s.EnvGroupPlacements != nil {
			if current, err = s.EnvGroupPlacements(ctx); err != nil {
				return nil, false, err
			}
		}
		for _, g := range st.envGroups {
			// A group declared outside any environment keeps its scope: the
			// apply only ever assigns one (applyStackEnvGroups).
			if g.grouping == "" {
				continue
			}
			envID, exists := current[g.name]
			if envID, refused := placements.check(g.grouping, false, placementOf{exists: exists, environmentID: envID, unknown: current == nil}); refused {
				refuse(BlueprintResourceEnvVarGroup, g.name, envID)
			}
		}
	}
	if len(candidates) == 0 {
		return nil, false, nil
	}
	err = s.AuthorizeFreshOn(ctx, core.RelCanManage, core.WorkspaceObject(tenantID))
	if err == nil {
		return nil, true, nil
	}
	if !errors.Is(err, core.ErrForbidden) {
		return nil, false, err
	}
	return candidates, false, nil
}

// requireBlueprintPlacement is blueprintProtectedPlacementProblems as the
// apply's refusal: the first resource it names, as a 403. When an Admin passed
// the check it returns ctx marked cleared, so the apply's creates skip the
// per-resource fresh check (w3/m292).
func (s *Service) requireBlueprintPlacement(ctx context.Context, st parsedStack, snap *workspaceSnapshot) (context.Context, error) {
	refused, cleared, err := s.blueprintProtectedPlacementProblems(ctx, st, snap)
	if cleared {
		tenantID, _ := s.Tenant(ctx)
		return core.WithPlacementCleared(ctx, tenantID), nil
	}
	if err != nil || len(refused) == 0 {
		return ctx, err
	}
	return ctx, fmt.Errorf("%s %q: %w", refused[0].kind, refused[0].name, refused[0].err)
}

// placementOf is a declared resource's current placement: whether it exists,
// and the environment it is in ("" for none). unknown marks a current
// environment the check could not read, which counts as a move.
type placementOf struct {
	exists        bool
	environmentID string
	unknown       bool
}

// placedIn is the placement of an existing resource carrying labels.
func placedIn(labels map[string]string) placementOf {
	return placementOf{exists: true, environmentID: labels[core.LabelEnvironment]}
}

// blueprintPlacements is the workspace's environments as the protected
// placement check and the plan need them: protection by id, and each declared
// grouping's target (its environment and project ids, each "" while the apply
// has yet to create it), protection, network isolation and the settings of
// its own the apply changes.
type blueprintPlacements struct {
	protected map[string]bool
	targets   map[string]groupingTarget
}

type groupingTarget struct {
	environmentID string
	projectID     string
	protected     bool
	// isolated is the network isolation the apply leaves the environment
	// with (parsedGrouping.desiredIsolation).
	isolated bool
	// changed are the render.yaml paths of the existing environment's own
	// settings the apply changes (environmentControls.changed).
	changed []string
}

func (p blueprintPlacements) anyProtected() bool {
	for _, protected := range p.protected {
		if protected {
			return true
		}
	}
	for _, target := range p.targets {
		if target.protected {
			return true
		}
	}
	return false
}

// check reports whether placing a resource declared under grouping (or at the
// top level, ungrouped) from current is a protected placement, and the
// protected environment it names. A resource declared without a placement
// stays where it is.
func (p blueprintPlacements) check(grouping string, ungrouped bool, current placementOf) (string, bool) {
	var target groupingTarget
	switch {
	case grouping != "":
		target = p.targets[grouping]
	case ungrouped:
		// Declared ungrouped: the apply takes it out of any environment.
	default:
		return "", false
	}
	if !current.exists && !current.unknown {
		// A create.
		if target.protected {
			return target.environmentID, true
		}
		return "", false
	}
	if !current.unknown && current.environmentID == target.environmentID && target.environmentID != "" {
		return "", false // stays put
	}
	if !current.unknown && current.environmentID == "" && target.environmentID == "" && grouping == "" {
		return "", false // stays unassigned
	}
	if target.protected {
		return target.environmentID, true
	}
	if p.protected[current.environmentID] {
		return current.environmentID, true
	}
	return "", false
}

// moves names the placement paths applying a resource declared under grouping
// (or in the ungrouped: list) changes, from the placement its labels hold:
// "environment" when it lands in another environment or in none, "project"
// when in another project or in none, and, for a service (isolation), its
// "networkIsolation" when it stays in an environment whose isolation the
// apply turns on or off (in another environment the move already says so). A
// grouping the apply has yet to create is always a move; a resource declared
// without a placement stays where it is. It is diffPlacement, the comparison
// the apply makes, so a plan lists every change its approval applies (w9/094).
func (p blueprintPlacements) moves(grouping string, ungrouped, isolation bool, labels map[string]string) []string {
	var target groupingTarget
	switch {
	case grouping != "":
		target = p.targets[grouping]
	case !ungrouped:
		return nil
	}
	diff := diffPlacement(labels, core.EnvironmentAssignment{ID: target.environmentID, ProjectID: target.projectID, NetworkIsolationEnabled: target.isolated})
	if grouping != "" {
		// No label can hold the id of a grouping the apply has yet to create.
		diff.environment = diff.environment || target.environmentID == ""
		diff.project = diff.project || target.projectID == ""
	}
	var paths []string
	if diff.environment {
		paths = append(paths, "environment")
	}
	if diff.project {
		paths = append(paths, "project")
	}
	if isolation && diff.isolation && !diff.environment {
		paths = append(paths, "networkIsolation")
	}
	return paths
}

// blueprintEnvironmentPlacements resolves each grouping the manifest declares
// against the environments in snap's workspace, as applyBlueprintGroupingsWith
// will.
func (s *Service) blueprintEnvironmentPlacements(ctx context.Context, snap *workspaceSnapshot, groupings []parsedGrouping) (blueprintPlacements, error) {
	out := blueprintPlacements{protected: map[string]bool{}, targets: map[string]groupingTarget{}}
	rows, err := snap.groupings(ctx)
	if err != nil {
		return out, err
	}
	projects, environments := rows.projects, rows.environments
	byName := make(map[string]map[string]store.Environment, len(projects))
	projectName := make(map[string]string, len(projects))
	projectID := make(map[string]string, len(projects))
	for _, project := range projects {
		projectName[project.ID] = project.Name
		projectID[project.Name] = project.ID
		byName[project.Name] = map[string]store.Environment{}
	}
	for _, e := range environments {
		name, ok := projectName[e.ProjectID]
		if !ok {
			continue
		}
		out.protected[e.ID] = e.ProtectedStatus == core.ProtectedStatusProtected
		byName[name][e.Name] = e
	}
	for _, g := range groupings {
		e, exists := byName[g.projectName][g.environmentName]
		target := groupingTarget{projectID: projectID[g.projectName], isolated: g.desiredIsolation(e.NetworkIsolationEnabled)}
		if exists {
			target.environmentID = e.ID
			target.protected = out.protected[e.ID]
			target.changed = g.controls(e).changed(e)
		}
		if g.protectionSpecified && g.protectedStatus == core.ProtectedStatusProtected {
			target.protected = true
		}
		out.targets[blueprintGroupingKey(g.projectName, g.environmentName)] = target
	}
	return out, nil
}
