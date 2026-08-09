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

	"github.com/bex-co/bex/lego/backend/internal/core"
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
func (p *WorkspacePurger) PurgeWorkspace(ctx context.Context, tenantID string) error {
	if p.Store == nil {
		return nil
	}
	ids, err := p.Store.List(ctx, "env-groups")
	if err != nil {
		return err
	}
	for _, gid := range ids {
		raw, err := p.Store.Get(ctx, metaPath(gid))
		if err != nil {
			return err
		}
		owner := raw["workspace"]
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
		for _, path := range []string{envPath(gid), filesPath(gid), metaPath(gid)} {
			if err := p.Store.Delete(ctx, path); err != nil {
				return err
			}
		}
	}
	return nil
}
