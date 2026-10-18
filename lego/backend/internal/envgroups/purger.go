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
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// WorkspacePurger removes environment groups from the out-of-cascade OpenBao
// and Kubernetes stores when their workspace is deleted. It is an internal
// system operation: the deleting caller was authorized by workspaces.Service,
// so this adapter must not run a second request-scoped authorization check.
type WorkspacePurger struct {
	*Service
}

// PurgeWorkspace deletes tenantID's groups: their projection Secrets, every
// OpenBao path under the workspace's env-group prefix (a crashed create's
// unreferenced maps included), and their rows. Group links are constrained to
// same-workspace Apps; the Apps purger runs in the same delete sweep, so the
// group purger removes the projection Secrets and maps directly.
func (p *WorkspacePurger) PurgeWorkspace(ctx context.Context, tenantID string) error {
	if p.Store == nil {
		return nil // no value store: no group can exist
	}
	if err := p.requireGroups(); err != nil {
		return err // never report a purge that left the workspace's values behind
	}
	rows, err := p.Groups.ListEnvGroups(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, g := range rows {
		err := p.Groups.WithEnvGroupContentLock(ctx, g.ID, func(ctx context.Context) error {
			if err := p.deleteSecret(ctx, tenantID, envSecretName(g.ID)); err != nil {
				return err
			}
			if err := p.deleteSecret(ctx, tenantID, filesSecretName(g.ID)); err != nil {
				return err
			}
			if err := p.deleteGroupValues(ctx, tenantID, g.ID); err != nil {
				return err
			}
			// Keep the row as a retry anchor until external cleanup succeeds.
			_, err := p.Groups.DeleteEnvGroup(ctx, g.ID)
			if errors.Is(err, store.ErrNotFound) {
				return nil
			}
			return err
		})
		if err != nil {
			return err
		}
	}
	gids, err := p.Store.List(groupCtx(ctx, tenantID), "env-groups")
	if err != nil {
		return err
	}
	for _, gid := range gids {
		err := p.Groups.WithEnvGroupContentLock(ctx, gid, func(ctx context.Context) error {
			return p.deleteGroupValues(ctx, tenantID, gid)
		})
		if err != nil {
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
	service := core.AppPublicID(a)
	for _, gid := range mountedGroups(a) {
		g, err := p.readGroup(ctx, gid)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if g.TenantID != a.Labels[core.LabelTenant] {
			continue
		}
		if err := p.dropLinks(ctx, gid, service); err != nil && !errors.Is(err, core.ErrNotFound) {
			return err
		}
	}
	return nil
}
