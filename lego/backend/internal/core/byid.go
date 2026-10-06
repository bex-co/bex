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
	"errors"
	"slices"
)

// ResourceOwner reads the workspaces a by-id resource belongs to, unscoped:
// none for an id that exists nowhere. Most resources live in one; a service
// event, a push notification or a GitHub installation can be recorded in
// several. It is a routing read only — the verb still authorizes and loads the
// resource through its usual scoped path.
type ResourceOwner func(ctx context.Context) ([]string, error)

// ScopeByID picks the workspace a by-id verb acts in (w4/m172), per ADR072's
// by-id matrix (w5/m115). An explicit ownerID, or a workspace the request
// already names, decides exactly as before. Otherwise the resource's OWN
// workspace does — so a member of several workspaces reaches their resource by
// id without guessing ?ownerId=, as Render's by-id endpoints take no owner.
//
// It only routes. The verb's own Authorize then decides membership and the
// relation, and writes the audit row: a caller outside the owning workspace
// gets the typed-id 403 ADR072 #8 keeps (w4/199), recorded once like any other
// refusal. An unknown id, or no caller identity, stays on the default path,
// whose scoped lookup answers its own not-found. The store being off
// (Workspace nil) means one workspace and nothing to route.
func (b *Base) ScopeByID(ctx context.Context, ownerID string, owner ResourceOwner) (context.Context, error) {
	return b.scopeByID(ctx, ownerID, owner, false)
}

// ScopeByVisibleID is ScopeByID for the matrix's named exemptions: only the
// caller's own workspaces count, so a resource held nowhere they belong stays
// on the default path and reads not-found. It is for ids a caller could
// enumerate (GitHub installation numbers), for resources whose owner is only
// discoverable in the caller's own workspaces (sandboxes), and for the
// caller's own rows in a workspace they have left (claim selections).
func (b *Base) ScopeByVisibleID(ctx context.Context, ownerID string, owner ResourceOwner) (context.Context, error) {
	return b.scopeByID(ctx, ownerID, owner, true)
}

func (b *Base) scopeByID(ctx context.Context, ownerID string, owner ResourceOwner, visibleOnly bool) (context.Context, error) {
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
	workspaces, err := owner(ctx)
	if err != nil {
		return ctx, err
	}
	workspace, err := b.pickOwner(ctx, caller, workspaces, visibleOnly)
	if err != nil || workspace == "" {
		return ctx, err
	}
	return WithWorkspace(ctx, workspace), nil
}

// pickOwner chooses the workspace among those holding a resource. One decides
// alone — membership is the verb's question, not the router's. Among several,
// the caller's default wins, then the only one they belong to; two or more of
// theirs is ambiguous (ErrConflict: name ownerId), and none of theirs routes
// to the first, where the verb refuses and audits. visibleOnly drops the
// workspaces the caller does not belong to before choosing.
func (b *Base) pickOwner(ctx context.Context, caller Identity, workspaces []string, visibleOnly bool) (string, error) {
	workspaces = slices.DeleteFunc(slices.Clone(workspaces), func(w string) bool { return w == "" })
	if len(workspaces) == 1 && !visibleOnly {
		return workspaces[0], nil
	}
	if len(workspaces) == 0 {
		return "", nil
	}
	if def, ok := b.Workspace.Tenant(ctx, caller); ok && slices.Contains(workspaces, def) {
		return def, nil
	}
	var mine []string
	for _, w := range workspaces {
		switch err := b.requireMember(ctx, caller, w); {
		case err == nil:
			mine = append(mine, w)
		case !errors.Is(err, ErrForbidden):
			return "", err
		}
		if len(mine) > 1 {
			return "", NewConflictError("OWNER_AMBIGUOUS",
				"the resource is in more than one of your workspaces; name the workspace (ownerId, or workspaceId on MCP)", nil)
		}
	}
	switch {
	case len(mine) == 1:
		return mine[0], nil
	case visibleOnly:
		return "", nil
	}
	return workspaces[0], nil
}
