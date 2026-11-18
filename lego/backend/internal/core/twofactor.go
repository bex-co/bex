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
	"log"
	"strings"
)

// Workspace login requirements: two-factor authentication (w22/m402,
// docs/ADR012-auth.md §9a) and Google login (w7/m465, §9b), Render's "Enforce
// 2FA & Google login". A workspace admin can require either or both; while one
// is on, a member who does not meet it — and any API key or SSH key that does
// not — cannot use the workspace's resources. The decision is one hook on
// Base, called by the authorization funnel after a positive OpenFGA decision,
// so REST, GraphQL and MCP share it and cannot drift. The policy itself (what
// counts as enrolled or as signed in with Google, whose sign-in an API key
// carries, the caches) lives in internal/twofactor; core only knows the seam,
// keeping it free of Kratos and of the store. The seam keeps its first name,
// TwoFactor, for both requirements.

// LoginRequirements is what a workspace requires of how its members sign in,
// in any combination: its own settings ORed with its organization's (w7/m466,
// §9c). For an org's own objects it is the org's settings alone.
type LoginRequirements struct {
	TwoFactor   bool
	GoogleLogin bool
	// Organization names the org whose own setting requires TwoFactor,
	// GoogleLogin or both; the zero value when no org requires anything.
	Organization LoginOrganization
}

// LoginOrganization is an org that requires something of how its members
// sign in, and what: the refusal names it, so a blocked member knows that
// the org, not the workspace, holds the requirement.
type LoginOrganization struct {
	ID          string
	Name        string
	TwoFactor   bool
	GoogleLogin bool
}

// Any reports whether the workspace requires anything of how its members sign
// in.
func (r LoginRequirements) Any() bool { return r.TwoFactor || r.GoogleLogin }

// WithOrganization ORs an org's own settings into r and names the org when it
// requires either. An org that requires nothing leaves r as it was.
func (r LoginRequirements) WithOrganization(id, name string, twoFactor, googleLogin bool) LoginRequirements {
	if !twoFactor && !googleLogin {
		return r
	}
	r.TwoFactor = r.TwoFactor || twoFactor
	r.GoogleLogin = r.GoogleLogin || googleLogin
	r.Organization = LoginOrganization{ID: id, Name: name, TwoFactor: twoFactor, GoogleLogin: googleLogin}
	return r
}

// TwoFactorGate decides whether the principal behind id may act in
// workspaceID, given the workspace's login requirements. A nil Base.TwoFactor
// is off.
type TwoFactorGate interface {
	// Check returns nil to allow, a coded 403 to refuse
	// (NewWorkspaceGoogleLoginRequiredError before
	// NewWorkspaceTwoFactorRequiredError when both fail), and
	// ErrAuthzUnavailable when the workspace requires something and the answer
	// cannot be established (fail closed, like an unreachable OpenFGA). A
	// workspace that requires nothing must cost no lookup at all.
	Check(ctx context.Context, workspaceID string, id Identity) error
}

// OrganizationTwoFactorGate is the optional capability of a TwoFactorGate
// that enforces an org's own login requirements on the org's own objects
// (org:<id>: its members, invites, settings and audit log), with no workspace
// involved (w7/m466, §9c). A gate without it does not enforce org objects:
// the SSH gateway's, which never authorizes one.
type OrganizationTwoFactorGate interface {
	CheckOrganization(ctx context.Context, orgID string, id Identity) error
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

// NewLoginTwoFactorRequiredError is the 2FA refusal for what r requires in
// workspaceID: NewWorkspaceTwoFactorRequiredError, or, when the workspace's
// org requires it, the same code naming the org (organizationId,
// organizationName) so the member knows which setting holds them. On an org's
// own objects workspaceID is "".
func NewLoginTwoFactorRequiredError(workspaceID string, r LoginRequirements) *CodedError {
	org := r.Organization
	if !org.TwoFactor {
		return NewWorkspaceTwoFactorRequiredError(workspaceID)
	}
	return NewForbiddenError(CodeWorkspaceTwoFactorRequired,
		"the organization "+org.Name+" requires two-factor authentication in all of its workspaces; enable it in your account settings to regain access",
		organizationRefusalDetails(workspaceID, org))
}

// NewLoginUnattributedKeyError refuses an API key whose creator's second
// factor cannot be checked where r requires 2FA, naming the org when its
// setting requires it.
func NewLoginUnattributedKeyError(workspaceID string, r LoginRequirements) *CodedError {
	who, details := "this workspace", map[string]any{"workspaceId": workspaceID}
	if org := r.Organization; org.TwoFactor {
		who, details = "the organization "+org.Name, organizationRefusalDetails(workspaceID, org)
	}
	return NewForbiddenError(CodeWorkspaceTwoFactorRequired,
		who+" requires two-factor authentication and this API key has no recorded creator whose second factor could be checked; create a new API key",
		details)
}

// organizationRefusalDetails are the details of a refusal an org's setting
// decided: the workspace (absent on the org's own objects) and the org.
func organizationRefusalDetails(workspaceID string, org LoginOrganization) map[string]any {
	d := map[string]any{"organizationId": org.ID, "organizationName": org.Name}
	if workspaceID != "" {
		d["workspaceId"] = workspaceID
	}
	return d
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

// The Google-login codes (w7/m465, docs/ADR012-auth.md §9b), bex-owned like
// the 2FA ones: the 403 a principal not signed in with Google gets on every
// surface, and the two 409s refusing the admin's mutation.
const (
	CodeWorkspaceGoogleLoginRequired         = "WORKSPACE_GOOGLE_LOGIN_REQUIRED"
	CodeWorkspaceGoogleLoginAdminNotSignedIn = "WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN"
	CodeWorkspaceGoogleLoginUnavailable      = "WORKSPACE_GOOGLE_LOGIN_UNAVAILABLE"
)

// NewWorkspaceGoogleLoginRequiredError is the shared refusal of a member whose
// sign-in is not Google, with the Google account of their own email, in a
// workspace that requires it. It wraps ErrForbidden, so every surface answers
// 403, and carries the workspace id in its details.
func NewWorkspaceGoogleLoginRequiredError(workspaceID string) *CodedError {
	return NewForbiddenError(CodeWorkspaceGoogleLoginRequired, "this workspace requires signing in with Google, with the Google account of your bex email address; sign in with Google to regain access",
		map[string]any{"workspaceId": workspaceID})
}

// NewWorkspaceGoogleLoginKeyError is the same refusal for an API key or SSH
// key that was not created while its creator was signed in with Google.
func NewWorkspaceGoogleLoginKeyError(workspaceID string) *CodedError {
	return NewForbiddenError(CodeWorkspaceGoogleLoginRequired, "this workspace requires signing in with Google, and this key was not created while signed in with Google; sign in with Google and create a new key",
		map[string]any{"workspaceId": workspaceID})
}

// NewLoginGoogleLoginRequiredError is the Google-login refusal for what r
// requires in workspaceID, for a member (key false) or for an API key or SSH
// key not created in a Google sign-in (key true). When the workspace's org
// requires it, it names the org, as NewLoginTwoFactorRequiredError does.
func NewLoginGoogleLoginRequiredError(workspaceID string, r LoginRequirements, key bool) *CodedError {
	org := r.Organization
	switch {
	case !org.GoogleLogin && key:
		return NewWorkspaceGoogleLoginKeyError(workspaceID)
	case !org.GoogleLogin:
		return NewWorkspaceGoogleLoginRequiredError(workspaceID)
	}
	msg := "the organization " + org.Name + " requires signing in with Google in all of its workspaces, with the Google account of your bex email address; sign in with Google to regain access"
	if key {
		msg = "the organization " + org.Name + " requires signing in with Google in all of its workspaces, and this key was not created while signed in with Google; sign in with Google and create a new key"
	}
	return NewForbiddenError(CodeWorkspaceGoogleLoginRequired, msg, organizationRefusalDetails(workspaceID, org))
}

// NewOrganizationTwoFactorOwnerNotEnrolledError is the org setting's lock-out
// refusal: an Owner with no second factor cannot require one of the org. It
// shares the workspace setting's code.
func NewOrganizationTwoFactorOwnerNotEnrolledError(orgID string) *CodedError {
	return NewConflictError(CodeWorkspaceTwoFactorAdminNotEnrolled,
		"enable two-factor authentication on your own account before requiring it in this organization",
		map[string]any{"organizationId": orgID})
}

// NewOrganizationGoogleLoginOwnerNotSignedInError is the org Google-login
// setting's lock-out refusal, with the workspace setting's code.
func NewOrganizationGoogleLoginOwnerNotSignedInError(orgID string) *CodedError {
	return NewConflictError(CodeWorkspaceGoogleLoginAdminNotSignedIn,
		"sign in with Google, with the Google account of your bex email address, before requiring Google login in this organization",
		map[string]any{"organizationId": orgID})
}

// NewWorkspaceGoogleLoginAdminNotSignedInError refuses turning the setting on
// for an admin who is not signed in with Google themselves: it would lock out
// its only actor.
func NewWorkspaceGoogleLoginAdminNotSignedInError() *CodedError {
	return NewConflictError(CodeWorkspaceGoogleLoginAdminNotSignedIn, "sign in with Google, with the Google account of your bex email address, before requiring Google login in this workspace", nil)
}

// NewWorkspaceGoogleLoginUnavailableError refuses turning the setting on when
// the deployment has no identity-provider lookup to enforce it with.
func NewWorkspaceGoogleLoginUnavailableError() *CodedError {
	return NewConflictError(CodeWorkspaceGoogleLoginUnavailable, "Google login enforcement is unavailable: this deployment cannot look up members' Google accounts", nil)
}

// NewWorkspaceGoogleLoginDefaultWorkspaceError refuses turning the setting on
// for the default workspace, which the gate never enforces.
func NewWorkspaceGoogleLoginDefaultWorkspaceError() *CodedError {
	return NewConflictError(CodeWorkspaceGoogleLoginUnavailable, "Google login enforcement is unavailable for the default workspace: it is the legacy workspace the requirement is never enforced in", nil)
}

// IsLoginRefusal reports whether err is the login gate's refusal, 2FA or
// Google login. A verb that answers a forbidden org or workspace as not found
// lets it through: the gate runs only after a positive decision, so the
// refusal reaches members alone and reveals nothing to anyone else.
func IsLoginRefusal(err error) bool {
	var coded *CodedError
	return errors.As(err, &coded) && (coded.Code == CodeWorkspaceTwoFactorRequired || coded.Code == CodeWorkspaceGoogleLoginRequired)
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
// a positive decision was just made on. An org object is checked against the
// org's own requirement; a workspace object names its workspace; any other
// object is handed to the gate's resolver, if it has one.
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
	if orgID, isOrg := strings.CutPrefix(object, OrganizationObjectPrefix); isOrg {
		// An org's own object is held to the org's own requirement (§9c).
		if g, ok := b.TwoFactor.(OrganizationTwoFactorGate); ok {
			return g.CheckOrganization(ctx, orgID, id)
		}
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

// CheckTwoFactor asks the gate whether the caller may read workspaceID's own
// configuration, ignoring WithTwoFactorExempt. The exempt reads (the workspaces
// list, currentUser) return only what a blocked member needs to see why; any
// workspace configuration that rides along on a workspace object, such as its
// inbound IP rules, is still refused to a blocked member and calls this.
func (b *Base) CheckTwoFactor(ctx context.Context, workspaceID string) error {
	if b == nil || b.TwoFactor == nil {
		return nil
	}
	id, ok := IdentityFrom(ctx)
	if !ok {
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

// RecordOrganizationTwoFactorEnforcement records one successful change of an
// org's two-factor setting in the org's own log (w7/m466, Render's
// ChangeOrg2FAEnforcementEvent), carrying the value it was set to as the
// workspace rows do. Paired with WithDeferredAllowedWriteAudit, so a no-op
// records nothing.
func (b *Base) RecordOrganizationTwoFactorEnforcement(ctx context.Context, orgID string, enabled bool) {
	ev := b.verbAuditEvent(ctx, AuditVerbOrgSetRequireTwoFactor, OrganizationObject(orgID), "")
	ev.Relation = RelCanManage
	ev.TwoFactorRequiredTo = &enabled
	b.recordAudit(ctx, ev)
}

// RecordOrganizationGoogleLoginEnforcement is RecordOrganizationTwoFactorEnforcement
// for the org's Google-login setting (ChangeOrgAllowedLoginMethodsEvent).
func (b *Base) RecordOrganizationGoogleLoginEnforcement(ctx context.Context, orgID string, enabled bool) {
	ev := b.verbAuditEvent(ctx, AuditVerbOrgSetRequireGoogleLogin, OrganizationObject(orgID), "")
	ev.Relation = RelCanManage
	ev.GoogleLoginRequiredTo = &enabled
	b.recordAudit(ctx, ev)
}

// RecordGoogleLoginEnforcement records one successful change of a workspace's
// Google-login setting (Render's ChangeTeamAllowedLoginMethodsEvent), paired
// with WithDeferredAllowedWriteAudit like RecordTwoFactorEnforcement.
func (b *Base) RecordGoogleLoginEnforcement(ctx context.Context, workspaceID string, enabled bool) {
	ev := b.verbAuditEvent(ctx, AuditVerbSetRequireGoogleLogin, WorkspaceObject(workspaceID), "")
	ev.Relation = RelCanManage
	ev.GoogleLoginRequiredTo = &enabled
	b.recordAudit(ctx, ev)
}
