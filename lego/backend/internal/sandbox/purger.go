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

package sandbox

import "context"

// PurgeKeyLookup resolves a workspace's existing OpenSandbox tenant key WITHOUT
// minting one — *store.PGStore's SandboxKeyLookup satisfies it structurally. The
// purger needs a lookup-only seam, never the mint-on-miss KeyProvider: minting a
// key for a workspace being torn down would write during teardown and, since the
// tenant-row cascade (migration 0056) is about to drop it, be pointless.
// found=false means no key was ever minted ⇒ no sandbox was ever created (the
// first create is what mints the key), so there is nothing to purge.
type PurgeKeyLookup interface {
	SandboxKeyLookup(ctx context.Context, workspaceID string) (string, bool, error)
}

// WorkspacePurger adapts the OpenSandbox client to the workspaces feature's
// WorkspacePurger seam (satisfied structurally — this package never imports
// workspaces), closing a w1/m61 gap: a deleted workspace's running sandboxes
// were never stopped through the API. It runs PRE-cascade in workspaces.Delete
// because it needs the workspace's tenant key, which the tenant-row cascade
// drops. The `<ws>-sandbox` namespace prune eventually reaps the pods at the
// Kubernetes layer too, but that path is best-effort; terminating through
// OpenSandbox is the always-on clean shutdown and also clears the OpenSandbox
// control plane's own records.
type WorkspacePurger struct {
	Client *Client
	Keys   PurgeKeyLookup
}

// PurgeWorkspace stops every sandbox in the deleted workspace's `<ws>-sandbox`
// namespace. The tenant key scopes the OpenSandbox list to exactly that
// namespace, so this is a workspace-wide teardown — no owner/admin filtering,
// because it is an internal system operation after the deleting caller was
// already authorized, not a user-facing list. Idempotent: no key ⇒ nothing was
// ever created ⇒ no-op; Client.Terminate already tolerates a missing sandbox
// (NotFound), so a retried delete (or one racing the namespace prune) completes
// cleanly. Never authorizes here — there is no Identity in ctx to check against.
func (p *WorkspacePurger) PurgeWorkspace(ctx context.Context, tenantID string) error {
	if p.Client == nil || p.Keys == nil {
		return nil
	}
	key, found, err := p.Keys.SandboxKeyLookup(ctx, tenantID)
	if err != nil {
		return err
	}
	if !found || key == "" {
		return nil
	}
	list, err := p.Client.List(ctx, key)
	if err != nil {
		return err
	}
	for _, sb := range list {
		if err := p.Client.Terminate(ctx, key, sb.ID); err != nil {
			return err
		}
	}
	return nil
}
