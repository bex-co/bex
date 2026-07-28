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

// workspace.go carries the caller's EXPLICITLY named workspace through a request
// (w6/m14). A caller can belong to several workspaces (w6/m1, w4/m12 invites);
// with none named, core.Base resolves the caller's *default* workspace (the
// oldest membership — store.TenantForIdentity). Naming one overrides that for
// the whole request: Authorize checks the verb's relation against THAT
// workspace, Tenant reports it (so a create is stamped into it), and GetApp
// scopes to it.
//
// It travels in the context — the same seam core.Identity already uses — so the
// 80+ verb signatures stay untouched: the three adapters (REST ownerId, GraphQL
// ownerId, MCP's per-call workspaceId) each decode their own spelling of
// "which workspace" and put the resolved id here, and every verb inherits it
// through core.Base without knowing which surface it came from.
//
// The value is UNVERIFIED intent, not a grant: it is whatever the caller asked
// for. core.Base is the only place that acts on it, and only after checking the
// caller is a member of it (WorkspaceResolver.IsMember) — a non-member naming a
// workspace is ErrForbidden, never a silent fall-back to their own.

type workspaceKey struct{}

// WithWorkspace records the workspace (tenant id, "tea-<xid>") the caller named
// for this request. The adapters call it; nothing else should. Empty is a no-op
// (an absent ownerId means "my default workspace", not "no workspace").
func WithWorkspace(ctx context.Context, tenantID string) context.Context {
	if tenantID == "" {
		return ctx
	}
	return context.WithValue(ctx, workspaceKey{}, tenantID)
}

// WorkspaceFrom returns the workspace the caller named, if any. ok=false means
// the caller named none — resolve their default workspace instead.
func WorkspaceFrom(ctx context.Context) (string, bool) {
	tenantID, ok := ctx.Value(workspaceKey{}).(string)
	return tenantID, ok && tenantID != ""
}

// NamedWorkspace returns the explicitly named workspace or "" when the
// request intentionally relies on the caller's default workspace. Adapters use
// this after placing their canonical workspace parameter in the context; the
// domain layer still resolves and authorizes the value through Base.
func NamedWorkspace(ctx context.Context) string {
	tenantID, _ := WorkspaceFrom(ctx)
	return tenantID
}

// resolvedWorkspaceKey caches the result of Base.resolveWorkspace for the
// lifetime of one request context. resolveWorkspace depends only on the caller's
// identity and the workspace they named (both immutable for a given request), so
// memoizing it eliminates redundant Workspace.Tenant / IsMember round trips when
// collision loops call AuthorizeLabeled repeatedly (w4/027).
type resolvedWorkspaceKey struct{}

type resolvedWorkspace struct {
	acting string
	err    error
}

// resolvedWorkspaceFrom returns the cached resolved workspace, if any. A nil
// context simply reports a cache miss rather than panicking — some tests call
// Base methods with a nil context and a nil Workspace resolver, which is legal
// because resolveWorkspace's uncached path short-circuits on b.Workspace == nil.
func resolvedWorkspaceFrom(ctx context.Context) (resolvedWorkspace, bool) {
	if ctx == nil {
		return resolvedWorkspace{}, false
	}
	r, ok := ctx.Value(resolvedWorkspaceKey{}).(resolvedWorkspace)
	return r, ok
}

// withResolvedWorkspace caches the resolved workspace result on ctx. Callers
// should treat the returned context as read-only for this value.
func withResolvedWorkspace(ctx context.Context, acting string, err error) context.Context {
	return context.WithValue(ctx, resolvedWorkspaceKey{}, resolvedWorkspace{acting: acting, err: err})
}
