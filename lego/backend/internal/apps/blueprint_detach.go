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
	"slices"
	"strings"
)

// Only durable claims belonging to this blueprint can become detach actions.
// Missing resources are not advertised as continuing to run, and resources
// declared by other blueprints never enter this diff.
func (s *Service) blueprintDetachments(ctx context.Context, tenantID, blueprintID string, st parsedStack, resolver *blueprintActionResolver) ([]BlueprintResource, error) {
	out := []BlueprintResource{}
	if blueprintID == "" || s.Blueprints == nil {
		return out, nil
	}
	claims, err := s.Blueprints.ListBlueprintResourceClaims(ctx, tenantID, blueprintID)
	if err != nil {
		return nil, fmt.Errorf("listing Blueprint detach candidates: %w", err)
	}
	declared := blueprintDeclaredClaims(st)
	for _, claim := range claims {
		if declared[claim.Kind+"/"+claim.Name] {
			continue
		}
		if resolver == nil {
			resolver, err = newBlueprintActionResolver(ctx, s, parsedStack{})
			if err != nil {
				return nil, err
			}
		}
		kind := BlueprintResourceKind(claim.Kind)
		if claim.Kind == "database" {
			kind = BlueprintResourcePostgres
		}
		current, exists, err := resolver.ResolveBlueprintResource(ctx, kind, claim.Name)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		resourceType := string(kind)
		if kind == BlueprintResourceService {
			resourceType = effectiveType(resolver.services[claim.Name].Spec.Type)
		}
		out = append(out, BlueprintResource{ID: current.ID, Name: claim.Name, Type: resourceType})
	}
	slices.SortFunc(out, func(a, b BlueprintResource) int {
		if c := strings.Compare(a.Type, b.Type); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}
