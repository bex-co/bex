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

package savedsearch

import (
	"context"
	"fmt"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
)

// TooManyResourcesCode is Render's structured 400 error code when a logs or
// metrics query covers more resources than the endpoint's cap — whether named
// directly via `resource` or expanded from `project` / `environment`.
const TooManyResourcesCode = "too_many_resources"

// ScopeLister resolves the resources of a project or an environment to a caller
// who may view it; projects.Service satisfies it.
type ScopeLister interface {
	ProjectScopeResources(ctx context.Context, projectID, environmentID string) (workspaceID string, resources []Resource, err error)
}

// ScopeRequest is the Render-shaped resource/project/environment/region
// targeting shared by logs and metrics reads (w4/m596).
type ScopeRequest struct {
	Project     string
	Environment string
	Region      string
	Resources   []string
}

// ScopePolicy parametrizes ResolveScope for logs vs metrics vs bandwidth-sources.
type ScopePolicy struct {
	// RequireProjectForEnvironment is true for Render's logs endpoints
	// (`environment` requires `project`) and false for metrics endpoints
	// (`environment` may be given alone).
	RequireProjectForEnvironment bool
	// MaxResources caps the resolved resource count with 400 `too_many_resources`
	// (20 for logs, 50 for metrics, 30 for bandwidth-sources).
	MaxResources int
	// Keep filters project/environment resources to the kinds applicable to
	// the operation when no explicit `resource` list was provided. Nil keeps all.
	Keep func(Resource) bool
	// Unavailable is the error returned when project/environment scoping is
	// requested but no ScopeLister is wired.
	Unavailable error
}

// ResolvedScope is the outcome of ResolveScope.
type ResolvedScope struct {
	WorkspaceID string
	Resources   []Resource // populated only when Scoped is true
	IDs         []string   // deduplicated resource ids to read
	Scoped      bool       // true when project or environment scoping was used
}

// ResolveScope validates project/environment/region/resource combinations and
// returns the deduplicated resource IDs to query (w4/m596).
//
// Rules:
//   - `environment` requires `project` when policy.RequireProjectForEnvironment.
//   - `region` requires `project` (or `project`/`environment` when environment
//     is allowed alone) and must be one of Render's five regions (w8/415).
//     Because bex runs one physical region (BEX_REGION), any valid Render region
//     is accepted without dropping resources.
//   - When neither `project` nor `environment` is set, at least one `resource`
//     is required; raw length is checked against MaxResources before deduping.
//   - When `project` or `environment` is set, explicit `resource` IDs narrow
//     the scope and every named resource must belong to that scope (else 400).
//     When `resource` is omitted, policy.Keep filters out resource kinds the
//     operation does not apply to.
//   - Exceeding policy.MaxResources returns 400 with code `too_many_resources`.
func ResolveScope(ctx context.Context, lister ScopeLister, req ScopeRequest, policy ScopePolicy) (ResolvedScope, error) {
	if req.Environment != "" && policy.RequireProjectForEnvironment && req.Project == "" {
		return ResolvedScope{}, fmt.Errorf("%w: environment requires project", core.ErrBadRequest)
	}
	if req.Region != "" {
		if req.Project == "" && (policy.RequireProjectForEnvironment || req.Environment == "") {
			if policy.RequireProjectForEnvironment {
				return ResolvedScope{}, fmt.Errorf("%w: region requires project", core.ErrBadRequest)
			}
			return ResolvedScope{}, fmt.Errorf("%w: region requires project or environment", core.ErrBadRequest)
		}
		if err := resourcemeta.CheckRequestedRegion(req.Region); err != nil {
			return ResolvedScope{}, err
		}
	}

	if req.Project == "" && req.Environment == "" {
		if len(req.Resources) == 0 {
			if policy.RequireProjectForEnvironment {
				return ResolvedScope{}, fmt.Errorf("%w: resource is required unless project is provided", core.ErrBadRequest)
			}
			return ResolvedScope{}, fmt.Errorf("%w: resource is required unless project or environment is provided", core.ErrBadRequest)
		}
		if policy.MaxResources > 0 && len(req.Resources) > policy.MaxResources {
			return ResolvedScope{}, tooManyResourcesError(len(req.Resources), policy.MaxResources)
		}
		return ResolvedScope{IDs: dedupeStrings(req.Resources)}, nil
	}

	if lister == nil {
		if policy.Unavailable != nil {
			return ResolvedScope{}, policy.Unavailable
		}
		return ResolvedScope{}, core.Unavailable("project scope listing needs the projects store")
	}

	workspace, all, err := lister.ProjectScopeResources(ctx, req.Project, req.Environment)
	if err != nil {
		return ResolvedScope{}, err
	}

	var selected []Resource
	if len(req.Resources) > 0 {
		if policy.MaxResources > 0 && len(req.Resources) > policy.MaxResources {
			return ResolvedScope{}, tooManyResourcesError(len(req.Resources), policy.MaxResources)
		}
		byID := make(map[string]Resource, len(all))
		for _, r := range all {
			byID[r.ID] = r
		}
		want := dedupeStrings(req.Resources)
		selected = make([]Resource, 0, len(want))
		for _, id := range want {
			r, ok := byID[id]
			if !ok {
				return ResolvedScope{}, fmt.Errorf("%w: resource %q does not belong to the requested scope", core.ErrBadRequest, id)
			}
			selected = append(selected, r)
		}
	} else {
		selected = make([]Resource, 0, len(all))
		for _, r := range all {
			if policy.Keep != nil && !policy.Keep(r) {
				continue
			}
			selected = append(selected, r)
		}
	}

	if policy.MaxResources > 0 && len(selected) > policy.MaxResources {
		return ResolvedScope{}, tooManyResourcesError(len(selected), policy.MaxResources)
	}
	ids := make([]string, len(selected))
	for i, r := range selected {
		ids[i] = r.ID
	}
	return ResolvedScope{
		WorkspaceID: workspace,
		Resources:   selected,
		IDs:         ids,
		Scoped:      true,
	}, nil
}

func tooManyResourcesError(count, limit int) error {
	return core.NewBadRequestError(
		TooManyResourcesCode,
		fmt.Sprintf("query covers %d resources; at most %d resources are allowed (limit %d) — provide resource to narrow the query", count, limit, limit),
		map[string]any{"resources": count, "maxResources": limit},
	)
}

func dedupeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}
