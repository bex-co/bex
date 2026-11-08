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
	placements, err := s.blueprintEnvironmentPlacements(ctx, tenantID, st.groupings)
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
// placement check needs them: protection by id, and each declared grouping's
// target (its id, "" while the apply has yet to create it) and protection.
type blueprintPlacements struct {
	protected map[string]bool
	targets   map[string]groupingTarget
}

type groupingTarget struct {
	environmentID string
	protected     bool
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

// blueprintEnvironmentPlacements reads every environment in tenantID and
// resolves each grouping the manifest declares against them, as
// applyBlueprintGroupingsWith will.
func (s *Service) blueprintEnvironmentPlacements(ctx context.Context, tenantID string, groupings []parsedGrouping) (blueprintPlacements, error) {
	out := blueprintPlacements{protected: map[string]bool{}, targets: map[string]groupingTarget{}}
	projects, err := s.BlueprintGroups.ListProjects(ctx, tenantID)
	if err != nil {
		return out, err
	}
	// One query for the whole workspace when the store offers it, rather than
	// one per project.
	var environments []store.Environment
	if all, ok := s.BlueprintGroups.(interface {
		ListWorkspaceEnvironments(ctx context.Context, tenantID string) ([]store.Environment, error)
	}); ok {
		if environments, err = all.ListWorkspaceEnvironments(ctx, tenantID); err != nil {
			return out, err
		}
	} else {
		for _, project := range projects {
			listed, err := s.BlueprintGroups.ListEnvironments(ctx, project.ID)
			if err != nil {
				return out, err
			}
			environments = append(environments, listed...)
		}
	}
	byName := make(map[string]map[string]store.Environment, len(projects))
	projectName := make(map[string]string, len(projects))
	for _, project := range projects {
		projectName[project.ID] = project.Name
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
		target := groupingTarget{}
		if exists {
			target.environmentID = e.ID
			target.protected = out.protected[e.ID]
		}
		if g.protectionSpecified && g.protectedStatus == core.ProtectedStatusProtected {
			target.protected = true
		}
		out.targets[blueprintGroupingKey(g.projectName, g.environmentName)] = target
	}
	return out, nil
}
