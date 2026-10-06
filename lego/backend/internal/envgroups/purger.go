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

package envgroups

import (
	"context"
	"errors"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// WorkspacePurger removes environment groups from the out-of-cascade OpenBao
// and Kubernetes stores when their workspace is deleted. It is an internal
// system operation: the deleting caller was authorized by workspaces.Service,
// so this adapter must not run a second request-scoped authorization check.
type WorkspacePurger struct {
	*Service
}

// PurgeWorkspace deletes groups attributed to tenantID. A legacy ownerless
// group is treated exactly as readMeta's deterministic migration would treat
// it: it belongs to core.DefaultTenant, so deleting that workspace removes it
// and deleting any other workspace leaves it alone. Group links are constrained
// to same-workspace Apps; the Apps purger runs in the same delete sweep, so the
// group purger removes the projection Secrets and source paths directly.
//
// w2/m80: the candidate id set is prefix-scoped (listGroupIDs) — tenantID's own
// workspace tenant (every migrated group it owns) unioned with the shared
// legacy tenant for the dual-read window (which may hold other workspaces'
// unmigrated groups too, so ownership is still confirmed via readMeta's own
// dual-read before anything is deleted). deleteGroupArtifacts removes both a
// migrated group's workspace-tenant copy and any legacy-tenant leftover
// (locator or still-unmigrated content) in one call.
func (p *WorkspacePurger) PurgeWorkspace(ctx context.Context, tenantID string) error {
	if p.Store == nil {
		return nil
	}
	ids, err := p.listGroupIDs(ctx, tenantID, true)
	if err != nil {
		return err
	}
	for _, gid := range ids {
		m, err := p.readMeta(ctx, gid)
		if errors.Is(err, core.ErrNotFound) {
			continue // create publishes metadata last; ignore an in-flight id
		}
		if err != nil {
			return err
		}
		owner := m.workspace
		if owner == "" {
			owner = core.DefaultTenant
		}
		if owner != tenantID {
			continue
		}
		if err := p.deleteSecret(ctx, tenantID, envSecretName(gid)); err != nil {
			return err
		}
		if err := p.deleteSecret(ctx, tenantID, filesSecretName(gid)); err != nil {
			return err
		}
		if err := p.deleteGroupArtifacts(ctx, tenantID, gid); err != nil {
			return err
		}
	}
	return nil
}

// UnlinkApp removes a service being deleted from every group its spec mounts
// (w5/m120). It is a step of a delete the caller was already authorized for,
// like secrets.WorkspacePurger.PurgeApp, so it checks nothing again. A group
// already gone, or another workspace's, is skipped; a group listing the
// service without mounting it is pruned by its next patch.
func (p *WorkspacePurger) UnlinkApp(ctx context.Context, a *appv1alpha1.App) error {
	if p.Store == nil {
		return nil
	}
	for _, gid := range mountedGroups(a) {
		m, err := p.readMeta(ctx, gid)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if m.workspace != a.Labels[core.LabelTenant] {
			continue
		}
		if err := p.dropLinks(ctx, gid, m.workspace, linkAliases(a)...); err != nil && !errors.Is(err, core.ErrNotFound) {
			return err
		}
	}
	return nil
}
