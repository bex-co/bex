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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// WorkspacePurger adapts Service to the workspaces feature's WorkspacePurger
// seam (satisfied structurally — this package never imports workspaces),
// closing a gap w6/m11's live verification found: an App created through the
// public REST/GraphQL/MCP surface (Service.create, the hand-applied path) is
// stamped with core.LabelTenant only, never store.LabelManagedBy — so the
// control-plane row cascade + reconciler prune that tears down *row-backed*
// Apps on workspace delete never sees it (the reconciler's own prune sweep
// filters by LabelManagedBy). Without this purger, such an App survives its
// workspace's deletion forever: still running (billable compute), and
// permanently unreachable — its LabelTenant now names a deleted tenant, so
// core.Base's tenant gate forbids everyone, including its own creator, from
// ever touching it again. It also permanently squats its name (Apps share one
// k8s namespace platform-wide), unlike the failure mode of a resource that's
// merely leaked.
type WorkspacePurger struct {
	*Service
}

// PurgeWorkspace deletes every App CR labeled with the given tenant id
// (core.LabelTenant). Safe to run alongside the row-backed cascade/reconciler
// path: deleting an App the reconciler already pruned is a no-op (NotFound is
// ignored by core.PurgeByTenant), so this purger's only observable effect is
// catching the Apps that path never reaches. Idempotent: no matching App is a
// no-op, not an error. Never call s.Authorize/AuthorizeOn here — this runs as
// an internal system operation after the deleting caller's own authorization
// already passed, with no Identity in ctx to check against.
func (p *WorkspacePurger) PurgeWorkspace(ctx context.Context, tenantID string) error {
	return p.PurgeByTenant(ctx, &appv1alpha1.AppList{}, tenantID)
}
