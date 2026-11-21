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
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w8/382: a Blueprint remembers each env group it applied by id, under the
// name its file declares it as. Matched by name alone, a group renamed in the
// dashboard was lost to the next sync, which minted a second group of the
// declared name and linked the services to both, so the renamed original
// looked live while the copy's values won. The Blueprint is the source of
// truth ("if any such changes conflict with Blueprint-defined configuration,
// the next Blueprint sync overwrites them", render.com/docs/infrastructure-as-code),
// so the sync renames the claimed group back instead (ADR049 § Env groups
// renamed outside the file).

// blueprintEnvGroupRename is a declared env group whose claimed group has
// another name now: id is the group, current the name it has.
type blueprintEnvGroupRename struct{ id, current string }

// blueprintTenantID is the workspace a Blueprint flow acts in.
func (s *Service) blueprintTenantID(ctx context.Context) string {
	if tenantID, ok := s.Tenant(ctx); ok {
		return tenantID
	}
	return s.resolveTenantID(ctx)
}

// renamedBlueprintEnvGroups is each of groups that blueprintID's earlier syncs
// applied and that was renamed outside the file since, keyed by its declared
// name. byName is the workspace's groups by name (nil: read here). A claimed
// group that no longer exists is not listed: the sync recreates it by name,
// as Render recreates a deleted Blueprint resource. A declared name another
// group has taken meanwhile is refused, since renaming the claimed group back
// cannot take it and adopting the other would leave the services linked to
// both.
func (s *Service) renamedBlueprintEnvGroups(ctx context.Context, blueprintID string, groups []parsedEnvGroup, byName map[string]string) (map[string]blueprintEnvGroupRename, error) {
	if blueprintID == "" || len(groups) == 0 || s.Blueprints == nil || s.EnvGroups == nil {
		return nil, nil
	}
	claims, err := s.Blueprints.BlueprintEnvGroupClaims(ctx, s.blueprintTenantID(ctx), blueprintID)
	if err != nil {
		return nil, fmt.Errorf("reading the Blueprint's env group claims: %w", err)
	}
	if len(claims) == 0 {
		return nil, nil
	}
	if byName == nil {
		if byName, err = s.EnvGroups.GroupIDsByName(ctx); err != nil {
			return nil, err
		}
	}
	nameOf := make(map[string]string, len(byName))
	for name, groupID := range byName {
		nameOf[groupID] = name
	}
	renames := map[string]blueprintEnvGroupRename{}
	var refused blueprintResourceErrors
	for _, g := range groups {
		groupID, claimed := claims[g.name]
		current, exists := nameOf[groupID]
		if !claimed || !exists || current == g.name {
			continue
		}
		if holder, taken := byName[g.name]; taken {
			refused = append(refused, blueprintResourceError{kind: BlueprintResourceEnvVarGroup, name: g.name, err: fmt.Errorf(
				"%w: this Blueprint manages env group %s, which was renamed to %q outside the file, and another env group (%s) now has the name %q; rename or delete one of them, then sync again",
				core.ErrBadRequest, groupID, current, holder, g.name)})
			continue
		}
		renames[g.name] = blueprintEnvGroupRename{id: groupID, current: current}
	}
	if len(refused) > 0 {
		return nil, refused
	}
	return renames, nil
}

// renameBlueprintEnvGroupBack gives a claimed group its declared name again,
// before the sync applies it by that name.
func (s *Service) renameBlueprintEnvGroupBack(ctx context.Context, name string, renamed blueprintEnvGroupRename) error {
	if s.EnvGroupRename == nil {
		return fmt.Errorf("%w: env group %q was renamed to %q outside the file, and env group renames are unavailable", core.ErrBadRequest, name, renamed.current)
	}
	if err := s.EnvGroupRename(ctx, renamed.id, name); err != nil {
		return fmt.Errorf("renaming env group %s back to %q: %w", renamed.id, name, err)
	}
	return nil
}

// recordBlueprintEnvGroupClaims makes the groups this apply converged the
// whole of blueprintID's env group claims, so a group the file stopped
// declaring is no longer followed through a rename.
func (s *Service) recordBlueprintEnvGroupClaims(ctx context.Context, tenantID, blueprintID string, st parsedStack) error {
	if s.Blueprints == nil {
		return nil
	}
	groups := map[string]string{}
	if len(st.envGroups) > 0 && s.EnvGroups != nil {
		byName, err := s.EnvGroups.GroupIDsByName(ctx)
		if err != nil {
			return fmt.Errorf("resolving env groups for the Blueprint's claims: %w", err)
		}
		for _, g := range st.envGroups {
			if groupID, ok := byName[g.name]; ok {
				groups[g.name] = groupID
			}
		}
	}
	if err := s.Blueprints.RecordBlueprintEnvGroupClaims(ctx, tenantID, blueprintID, groups); err != nil {
		return fmt.Errorf("recording the Blueprint's env group claims: %w", err)
	}
	return nil
}
