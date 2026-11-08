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

package core

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EnvironmentAssignment is the non-secret environment state a resource create
// needs at birth. Keeping the neutral shape in core lets apps, postgres, and
// keyvalue share one resolver without importing environments (which already
// depends on the datastore feature interfaces).
type EnvironmentAssignment struct {
	ID                      string
	ProjectID               string
	WorkspaceID             string
	NetworkIsolationEnabled bool
	IPAllowList             []IPAllowListEntry
	// Protected is the environment's protectedStatus=protected, which the
	// placement gate (EnvironmentPlacementGuard) reads (w3/m281).
	Protected bool
}

// EnvironmentResolver validates that environmentID belongs to workspaceID and
// returns the project/environment controls a newborn resource must inherit.
// ServiceDeleter is the apps service's normal delete verb (authorization,
// protected-service gate, env-group unlink, secrets purge, store row and CR),
// which environment and project deletes call for each contained service
// (w4/m264).
type ServiceDeleter interface {
	Delete(ctx context.Context, name string) error
}

// PostgresDeleter and KeyValueDeleter are the datastores' normal delete verbs
// (authorization, protected-environment gate, fresh can_create, CR delete),
// which the cascading environment and project deletes call for each contained
// datastore (w6/m290).
type PostgresDeleter interface {
	DeletePostgres(ctx context.Context, name string) error
}

// KeyValueDeleter is PostgresDeleter's Key Value counterpart.
type KeyValueDeleter interface {
	DeleteKeyValue(ctx context.Context, name string) error
}

// environments.Service is the single implementation wired by the composition
// root. Unknown ids return ErrNotFound; ids in another workspace return
// ErrForbidden.
type EnvironmentResolver interface {
	ResolveForCreate(ctx context.Context, environmentID, workspaceID string) (EnvironmentAssignment, error)
}

type blueprintSyncKey struct{}

// WithBlueprintSync marks ctx as a Blueprint's own sync: the auto-sync worker
// applying a pushed render.yaml (w3/m292). Render applies such an update in a
// protected environment whoever pushed it ("non-Admin workspace members can
// still modify those resources by publishing an update to the corresponding
// render.yaml file", render.com/docs/projects), so the protected placement
// gates admit it. Only platform code reading a durable Blueprint row may set
// it, as with WithActingTenant.
func WithBlueprintSync(ctx context.Context) context.Context {
	return context.WithValue(ctx, blueprintSyncKey{}, true)
}

// BlueprintSync reports a Blueprint's own sync (WithBlueprintSync). A context
// carrying an Identity is a member's call — a manual sync stays gated — so the
// mark counts only without one.
func BlueprintSync(ctx context.Context) bool {
	if _, ok := IdentityFrom(ctx); ok {
		return false
	}
	marked, _ := ctx.Value(blueprintSyncKey{}).(bool)
	return marked
}

type placementClearedKey struct{}

// WithPlacementCleared records that this request already passed a fresh
// workspace-manage check for workspaceID's protected placements — a Blueprint
// apply's whole-manifest pre-check — so the per-resource creates that follow
// need not repeat it (w3/m292).
func WithPlacementCleared(ctx context.Context, workspaceID string) context.Context {
	return context.WithValue(ctx, placementClearedKey{}, workspaceID)
}

// PlacementCleared reports whether WithPlacementCleared covers workspaceID.
func PlacementCleared(ctx context.Context, workspaceID string) bool {
	cleared, _ := ctx.Value(placementClearedKey{}).(string)
	return cleared != "" && cleared == workspaceID
}

// EnvironmentPlacementGuard is the optional half of an EnvironmentResolver
// that authorizes placing a resource in the environment it resolved: Render
// lets only workspace Admins create resources in a protected environment or
// move them into it (render.com/docs/projects; w3/m281). environments.Service's
// resolver implements it; a resolver without it (test fixtures) places freely.
type EnvironmentPlacementGuard interface {
	RequirePlacement(ctx context.Context, assignment EnvironmentAssignment) error
}

// ResolveEnvironmentForCreate is the create-time assignment lookup shared by
// every resource kind that can join an environment (services, Postgres, Key
// Value, environment groups, recoveries, Blueprint services). An empty
// environmentID means unassigned and resolves to the zero assignment; a
// non-empty one needs both a wired resolver and a known workspace, since an
// environment is only meaningful inside one. A resolved environment must
// then admit the placement (EnvironmentPlacementGuard), so every create and
// move-in that resolves here inherits the protected-environment Admin gate.
func ResolveEnvironmentForCreate(ctx context.Context, resolver EnvironmentResolver, environmentID, workspaceID string) (EnvironmentAssignment, error) {
	assignment, err := ResolveEnvironmentMembership(ctx, resolver, environmentID, workspaceID)
	if err != nil || assignment.ID == "" {
		return assignment, err
	}
	if guard, ok := resolver.(EnvironmentPlacementGuard); ok {
		if err := guard.RequirePlacement(ctx, assignment); err != nil {
			return EnvironmentAssignment{}, err
		}
	}
	return assignment, nil
}

// ResolveEnvironmentMembership is ResolveEnvironmentForCreate without the
// placement gate, for re-reading the environment a resource already belongs
// to: a Blueprint re-apply that leaves a service where it is places nothing,
// and Render lets non-Admins update Blueprint-managed resources in a protected
// environment.
func ResolveEnvironmentMembership(ctx context.Context, resolver EnvironmentResolver, environmentID, workspaceID string) (EnvironmentAssignment, error) {
	if environmentID == "" {
		return EnvironmentAssignment{}, nil
	}
	if resolver == nil || workspaceID == "" {
		return EnvironmentAssignment{}, ErrWorkspacesUnavailable
	}
	assignment, err := resolver.ResolveForCreate(ctx, environmentID, workspaceID)
	if err != nil {
		return EnvironmentAssignment{}, fmt.Errorf("resolving environment: %w", err)
	}
	return assignment, nil
}

// ApplyGrouping stamps (or clears) the project/environment labels on a resource
// and returns the environment's projected inbound-IP layer (w4/m28) for the
// caller's Spec.EnvironmentIPAllowList — a grouped resource inherits the layer,
// an ungrouped one sheds both labels and layer.
func ApplyGrouping(obj client.Object, assignment EnvironmentAssignment) []string {
	labels := obj.GetLabels()
	if assignment.ID == "" {
		delete(labels, LabelProject)
		delete(labels, LabelEnvironment)
		return nil
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels[LabelProject] = assignment.ProjectID
	labels[LabelEnvironment] = assignment.ID
	obj.SetLabels(labels)
	return EnvironmentLayerCIDRs(assignment.IPAllowList)
}

// TenantLabels is the dual stamp every tenant-owned CR carries: the tenant
// label scopes ownerId reads, and the workspace label is what the controllers
// propagate onto pod metadata for same-workspace NetworkPolicy selectors
// (docs/ADR022-tenant-isolation.md). One definition so the pair cannot drift.
func TenantLabels(workspaceID string) map[string]string {
	return map[string]string{LabelTenant: workspaceID, LabelWorkspace: workspaceID}
}
