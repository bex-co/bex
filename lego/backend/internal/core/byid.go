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

// ResourceOwner reads the workspace a by-id resource belongs to, unscoped:
// found=false for an id that exists nowhere. It is a routing read only — the
// verb still authorizes and loads the resource through its usual scoped path.
type ResourceOwner func(ctx context.Context) (workspace string, found bool, err error)

// ScopeByID picks the workspace a by-id verb acts in (w4/m172). An explicit
// ownerID, or a workspace the request already names, decides exactly as
// before. Otherwise the resource's OWN workspace does — so a member of several
// workspaces reaches their resource by id without guessing ?ownerId=, as
// Render's by-id endpoints take no owner (services, env groups, Blueprints
// (w4/m169) and API keys (w4/194) already resolve this way).
//
// A caller who is not a member of the owning workspace gets notFound — the
// answer a nonexistent id gets, so the id is no existence oracle. A member
// lacking the verb's relation keeps the 403 the caller's own Authorize gives.
// An unknown id, or no caller identity, stays on the default path, whose
// scoped lookup answers its own not-found. The store being off (Workspace nil)
// means one workspace and nothing to route.
func (b *Base) ScopeByID(ctx context.Context, ownerID string, owner ResourceOwner, notFound error) (context.Context, error) {
	if ownerID != "" {
		return WithWorkspace(ctx, ownerID), nil
	}
	if _, named := WorkspaceFrom(ctx); named || b == nil || b.Workspace == nil || owner == nil {
		return ctx, nil
	}
	caller, ok := IdentityFrom(ctx)
	if !ok {
		return ctx, nil
	}
	workspace, found, err := owner(ctx)
	if err != nil {
		return ctx, err
	}
	if !found || workspace == "" {
		return ctx, nil
	}
	if err := b.requireMember(ctx, caller, workspace); err != nil {
		if err == ErrForbidden {
			return ctx, notFound
		}
		return ctx, err
	}
	return WithWorkspace(ctx, workspace), nil
}
