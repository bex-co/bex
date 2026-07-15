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

import "context"

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
