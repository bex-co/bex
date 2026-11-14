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
	"log"
	"strings"
)

// Workspace-enforced two-factor authentication (w22/m402, docs/ADR012-auth.md
// §9a). A workspace admin can require 2FA; while the setting is on, a member
// with no enrolled second factor — and any API key or SSH key that member
// created — cannot use the workspace's resources. The decision is one hook on
// Base, called by the authorization funnel after a positive OpenFGA decision,
// so REST, GraphQL and MCP share it and cannot drift. The policy itself (what
// counts as enrolled, whose factor an API key borrows, the caches) lives in
// internal/twofactor; core only knows the seam, keeping it free of Kratos and
// of the store.

// TwoFactorGate decides whether the principal behind id may act in
// workspaceID. A nil Base.TwoFactor is off.
type TwoFactorGate interface {
	// Check returns nil to allow, the coded 403 NewWorkspaceTwoFactorRequiredError
	// to refuse, and ErrAuthzUnavailable when the workspace requires 2FA and the
	// answer cannot be established (fail closed, like an unreachable OpenFGA).
	// A workspace that does not require 2FA must cost no lookup at all.
	Check(ctx context.Context, workspaceID string, id Identity) error
}

// TwoFactorObjectResolver is the optional capability of a TwoFactorGate that
// can name the workspace of an authorization object whose name carries none
// (agent_session:<id>). "" means the object is not a workspace resource, so
// there is nothing to enforce. An error fails the check closed.
type TwoFactorObjectResolver interface {
	ObjectWorkspace(ctx context.Context, object string) (workspaceID string, err error)
}

// The bex-owned codes of the feature (Render documents none for this
// refusal). The first is the 403 a blocked principal gets on every surface;
// the other two refuse the admin's mutation with a 409.
const (
	CodeWorkspaceTwoFactorRequired         = "WORKSPACE_2FA_REQUIRED"
	CodeWorkspaceTwoFactorAdminNotEnrolled = "WORKSPACE_2FA_ADMIN_NOT_ENROLLED"
	CodeWorkspaceTwoFactorUnavailable      = "WORKSPACE_2FA_UNAVAILABLE"
)

// NewWorkspaceTwoFactorRequiredError is the shared refusal of a principal who
// has no second factor in a workspace that requires one. It wraps ErrForbidden,
// so every surface answers 403, and carries the workspace id in its details.
func NewWorkspaceTwoFactorRequiredError(workspaceID string) *CodedError {
	return NewForbiddenError(CodeWorkspaceTwoFactorRequired,
		"this workspace requires two-factor authentication; enable it in your account settings to regain access",
		map[string]any{"workspaceId": workspaceID})
}

// NewWorkspaceTwoFactorAdminNotEnrolledError refuses turning the setting on
// for an admin who has no second factor themselves: it would lock out its only
// actor.
func NewWorkspaceTwoFactorAdminNotEnrolledError() *CodedError {
	return NewConflictError(CodeWorkspaceTwoFactorAdminNotEnrolled,
		"enable two-factor authentication on your own account before requiring it in this workspace", nil)
}

// NewWorkspaceTwoFactorUnavailableError refuses turning the setting on when
// the deployment has no identity-provider lookup to enforce it with, rather
// than leaving it silently unenforced.
func NewWorkspaceTwoFactorUnavailableError() *CodedError {
	return NewConflictError(CodeWorkspaceTwoFactorUnavailable,
		"two-factor enforcement is unavailable: this deployment cannot look up members' second factors", nil)
}

// NewWorkspaceTwoFactorDefaultWorkspaceError refuses turning the setting on for
// the default workspace, the legacy tenant the gate never enforces: showing its
// members a requirement nothing enforces would be a lie. It shares the code of
// the other "cannot be enforced here" refusal.
func NewWorkspaceTwoFactorDefaultWorkspaceError() *CodedError {
	return NewConflictError(CodeWorkspaceTwoFactorUnavailable,
		"two-factor enforcement is unavailable for the default workspace: it is the legacy workspace the requirement is never enforced in", nil)
}

type twoFactorExemptKey struct{}

// WithTwoFactorExempt marks the operations a blocked member must still reach
// to see why and fix it — listing their workspaces, who they are, leaving a
// workspace — so the funnel skips the gate for them. Only those verbs call it.
func WithTwoFactorExempt(ctx context.Context) context.Context {
	return context.WithValue(ctx, twoFactorExemptKey{}, true)
}

func twoFactorExempt(ctx context.Context) bool {
	exempt, _ := ctx.Value(twoFactorExemptKey{}).(bool)
	return exempt
}

// requireTwoFactor is the funnel's call into the gate for the OpenFGA object
// a positive decision was just made on. A workspace object names its
// workspace; any other object is handed to the gate's resolver, if it has one.
// No gate, no identity (a platform background caller) and an exempt operation
// are all "nothing to ask".
func (b *Base) requireTwoFactor(ctx context.Context, object string) error {
	if b == nil || b.TwoFactor == nil || twoFactorExempt(ctx) {
		return nil
	}
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil
	}
	workspaceID, isWorkspace := strings.CutPrefix(object, WorkspaceObjectPrefix)
	if !isWorkspace {
		resolver, ok := b.TwoFactor.(TwoFactorObjectResolver)
		if !ok {
			return nil
		}
		var err error
		if workspaceID, err = resolver.ObjectWorkspace(ctx, object); err != nil {
			log.Printf("core: two-factor object resolution unavailable: %v", err)
			return ErrAuthzUnavailable
		}
	}
	if workspaceID == "" {
		return nil
	}
	return b.TwoFactor.Check(ctx, workspaceID, id)
}

// RecordTwoFactorEnforcement records one successful change of a workspace's
// two-factor setting: the verb's allowed row, carrying the value it was set
// to. Paired with WithDeferredAllowedWriteAudit so a no-op change, which
// writes nothing, records nothing.
func (b *Base) RecordTwoFactorEnforcement(ctx context.Context, workspaceID string, enabled bool) {
	ev := b.verbAuditEvent(ctx, AuditVerbSetRequireTwoFactor, WorkspaceObject(workspaceID), "")
	ev.Relation = RelCanManage
	ev.TwoFactorRequiredTo = &enabled
	b.recordAudit(ctx, ev)
}
