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
	"slices"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
)

// previewStackCreates runs, for each resource the apply would create, the plan
// that create runs before its first write (w5/m126), so a preview refuses what
// the apply would: a taken name, a claimed host, a registry credential that
// cannot apply, the create-time secret limits, and admission (the CRD's rules
// and the plan's count cap). plan is the current-state plan; its create actions
// are the resources the apply creates. A refusal comes back located at its
// resource; any other failure (a store or cluster read) is returned.
func (s *Service) previewStackCreates(ctx context.Context, st parsedStack, plan BlueprintPlan) (blueprintResourceErrors, error) {
	creates := map[BlueprintResourceKind]map[string]bool{}
	for _, action := range plan.Actions {
		if action.Operation != BlueprintPlanCreate {
			continue
		}
		if creates[action.Kind] == nil {
			creates[action.Kind] = map[string]bool{}
		}
		creates[action.Kind][action.Name] = true
	}
	// planned is each kind's creates in the order the apply makes them.
	planned := map[BlueprintResourceKind][]string{}
	var refused blueprintResourceErrors
	// record notes a planned create and its plan's answer: a manifest problem
	// is a refusal at the resource, any other error ends the preview.
	record := func(kind BlueprintResourceKind, name string, err error) error {
		planned[kind] = append(planned[kind], name)
		if err == nil {
			return nil
		}
		if !blueprintManifestProblem(err) {
			return err
		}
		refused = append(refused, blueprintResourceError{kind: kind, name: name, err: err})
		return nil
	}
	for _, svc := range st.services {
		if !creates[BlueprintResourceService][svc.req.Name] {
			continue
		}
		desired, err := specFromCreate(svc.req)
		if err == nil {
			_, err = s.planStackApp(ctx, svc.req, desired)
		}
		if fatal := record(BlueprintResourceService, svc.req.Name, err); fatal != nil {
			return nil, fatal
		}
	}
	// The apply's grouping assignment needs environments it creates, so a
	// preview plans each datastore ungrouped; grouping adds only environment
	// membership (labels and an allow-list), which admission does not judge.
	for _, db := range st.databases {
		if !creates[BlueprintResourcePostgres][db.name] {
			continue
		}
		d, err := s.planStackDatabase(ctx, db, core.EnvironmentAssignment{})
		if err == nil {
			err = postgres.CountCap.CreateError(s.DryRunCreate(ctx, d))
		}
		if fatal := record(BlueprintResourcePostgres, db.name, err); fatal != nil {
			return nil, fatal
		}
	}
	for _, kv := range st.keyValues {
		if !creates[BlueprintResourceKeyValue][kv.name] {
			continue
		}
		err := keyvalue.CountCap.CreateError(s.DryRunCreate(ctx, s.planStackKeyValue(ctx, kv, core.EnvironmentAssignment{})))
		if fatal := record(BlueprintResourceKeyValue, kv.name, err); fatal != nil {
			return nil, fatal
		}
	}
	overCap, err := s.previewStackCountCaps(ctx, planned, refused)
	if err != nil {
		return nil, err
	}
	return append(refused, overCap...), nil
}

// previewStackCountCaps refuses the first new resource of each kind past the
// workspace's count cap. Each create's own dry-run sees today's count, so a
// stack creating several can pass every one while the apply's later creates
// are refused. A workspace with no quota yet has no cap to bind.
func (s *Service) previewStackCountCaps(ctx context.Context, planned map[BlueprintResourceKind][]string, refused blueprintResourceErrors) (blueprintResourceErrors, error) {
	// One create of a kind is its own dry-run's to judge.
	several := false
	for _, names := range planned {
		several = several || len(names) > 1
	}
	if !several {
		return nil, nil
	}
	tenantID, _ := s.Tenant(ctx)
	quota := &corev1.ResourceQuota{}
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.TenantNamespace(tenantID), Name: core.TenantQuotaName}, quota); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	var overCap blueprintResourceErrors
	for _, kind := range []struct {
		kind     BlueprintResourceKind
		countCap core.CountCap
	}{
		{BlueprintResourceService, serviceCountCap},
		{BlueprintResourcePostgres, postgres.CountCap},
		{BlueprintResourceKeyValue, keyvalue.CountCap},
	} {
		names := planned[kind.kind]
		room, limit, ok := kind.countCap.Room(quota)
		// With no room, every create's own dry-run has already been refused.
		if !ok || room <= 0 || int64(len(names)) <= room {
			continue
		}
		first := names[room]
		if slices.ContainsFunc(refused, func(r blueprintResourceError) bool { return r.kind == kind.kind && r.name == first }) {
			continue
		}
		overCap = append(overCap, blueprintResourceError{kind: kind.kind, name: first, err: kind.countCap.Exceeded(limit)})
	}
	return overCap, nil
}

// blueprintManifestProblem reports a refusal the manifest can fix, which a
// preview reports at its resource rather than failing on.
func blueprintManifestProblem(err error) bool {
	return errors.Is(err, core.ErrBadRequest) || errors.Is(err, core.ErrConflict)
}
