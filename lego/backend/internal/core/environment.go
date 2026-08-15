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
}

// EnvironmentResolver validates that environmentID belongs to workspaceID and
// returns the project/environment controls a newborn resource must inherit.
// environments.Service is the single implementation wired by the composition
// root. Unknown ids return ErrNotFound; ids in another workspace return
// ErrForbidden.
type EnvironmentResolver interface {
	ResolveForCreate(ctx context.Context, environmentID, workspaceID string) (EnvironmentAssignment, error)
}

// ResolveEnvironmentForCreate is the create-time assignment lookup shared by
// every resource kind that can join an environment (services, Postgres, Key
// Value). An empty environmentID means unassigned and resolves to the zero
// assignment; a non-empty one needs both a wired resolver and a known
// workspace, since an environment is only meaningful inside one.
func ResolveEnvironmentForCreate(ctx context.Context, resolver EnvironmentResolver, environmentID, workspaceID string) (EnvironmentAssignment, error) {
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
