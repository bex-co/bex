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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// blueprintPlanLimitErrors reports, at each authored environment, every
// workspace-plan refusal applyBlueprintGroupingsWith would hit: an environment
// past the plan's per-project cap (Hobby: two) and network isolation turned on
// below Pro (w8/m138 t013), each with the refusal's machine code. It counts
// existing environments first and the manifest's new ones in document order,
// as the apply does. A store read failure leaves the check to the apply, as
// validateWorkspaceReferences does.
func (s *Service) blueprintPlanLimitErrors(ctx context.Context, source *BlueprintSource, groupings []parsedGrouping) []BlueprintValidationError {
	if len(groupings) == 0 || s.BlueprintGroups == nil {
		return nil
	}
	tenantID, ok := s.Tenant(ctx)
	if !ok {
		return nil
	}
	plan, planned, err := s.WorkspacePlanOf(ctx, tenantID)
	if err != nil || !planned ||
		(core.MaxEnvironmentsPerProject(plan) == 0 && core.PlanAllows(plan, core.FeatureNetworkIsolation)) {
		return nil
	}
	projects, err := s.BlueprintGroups.ListProjects(ctx, tenantID)
	if err != nil {
		return nil
	}
	projectIDs := make(map[string]string, len(projects))
	for _, project := range projects {
		projectIDs[project.Name] = project.ID
	}
	environments := map[string]map[string]store.Environment{}
	var out []BlueprintValidationError
	refuse := func(err error, pointer string) {
		entry := blueprintLocatedError(source, blueprintValidationMessage(err), pointer)
		entry.Code = blueprintRefusalCode(err)
		out = append(out, entry)
	}
	for _, grouping := range groupings {
		existing, listed := environments[grouping.projectName]
		if !listed {
			existing = map[string]store.Environment{}
			if id := projectIDs[grouping.projectName]; id != "" {
				rows, err := s.BlueprintGroups.ListEnvironments(ctx, id)
				if err != nil {
					return nil
				}
				for _, row := range rows {
					existing[row.Name] = row
				}
			}
			environments[grouping.projectName] = existing
		}
		current, exists := existing[grouping.environmentName]
		if !exists {
			if err := core.EnvironmentCapacityRefusal(plan, len(existing)); err != nil {
				refuse(err, grouping.pointer)
				continue
			}
			existing[grouping.environmentName] = store.Environment{Name: grouping.environmentName}
		}
		if err := core.NetworkIsolationRefusal(plan, grouping.desiredIsolation(current.NetworkIsolationEnabled), current.NetworkIsolationEnabled); err != nil {
			refuse(err, grouping.pointer+"/networking/isolation")
		}
	}
	return out
}
