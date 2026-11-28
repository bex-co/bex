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
	"fmt"
	"log"
	"time"
)

// organizations.go is the kernel's part of Render's organizations (w7/m464,
// docs/ADR115-organizations.md): the OpenFGA object and tuples an org is
// made of, its audit verbs and recorders, and its named refusals. The verbs
// live in internal/workspaces and internal/members; the rows in
// internal/store.

// OrganizationObjectPrefix is the OpenFGA type of an org
// (deploy/gitops/authz/model.fga's `org`).
const OrganizationObjectPrefix = "org:"

// OrganizationObject is the OpenFGA object for an org id, e.g. org:org-abc.
// Its relations reuse the workspace verbs' names: can_manage (an Owner's org
// settings, members and audit log), can_create (an Owner creating a workspace
// in the org) and can_view (a Member or Guest reading the org).
func OrganizationObject(orgID string) string { return OrganizationObjectPrefix + orgID }

// The org roles (ADR115 D4).
const (
	OrgRoleOwner  = "owner"
	OrgRoleMember = "member"
	OrgRoleGuest  = "guest"
)

// The privacy settings of an org workspace (tenants.privacy). Render:
// "Newly added workspaces are invite-only by default."
const (
	PrivacyInviteOnly = "invite_only"
	PrivacyPublic     = "public"
)

// Tuple is one OpenFGA relationship tuple.
type Tuple struct {
	User     string
	Relation string
	Object   string
}

// OrganizationTupleWriter applies org tuple changes to OpenFGA: deletes first,
// then writes, each idempotent. *authz.openfgaChecker satisfies it, so the
// feature packages keep no dependency on the authz client.
type OrganizationTupleWriter interface {
	WriteOrganizationTuples(ctx context.Context, writes, deletes []Tuple) error
}

// WriteOrgTuples applies org tuple changes through w; a nil writer (authz
// off) and an empty change write nothing.
func WriteOrgTuples(ctx context.Context, w OrganizationTupleWriter, writes, deletes []Tuple) error {
	if w == nil || (len(writes) == 0 && len(deletes) == 0) {
		return nil
	}
	return w.WriteOrganizationTuples(ctx, writes, deletes)
}

// OrgRoleTuple is a person's org role: user:<subject> owner|member|guest
// org:<id>.
func OrgRoleTuple(orgID, subject, role string) Tuple {
	return Tuple{User: "user:" + subject, Relation: role, Object: OrganizationObject(orgID)}
}

// OrgWorkspaceTuple links a workspace to its org (workspace#org), which an
// Owner's can_join_as_admin derives through.
func OrgWorkspaceTuple(orgID, workspaceID string) Tuple {
	return Tuple{User: OrganizationObject(orgID), Relation: "org", Object: WorkspaceObject(workspaceID)}
}

// OrgPublicTuple marks an org workspace Public (workspace#public_org), which a
// Member's can_join derives through. It exists only while the workspace is
// Public.
func OrgPublicTuple(orgID, workspaceID string) Tuple {
	return Tuple{User: OrganizationObject(orgID), Relation: "public_org", Object: WorkspaceObject(workspaceID)}
}

// The org audit verbs (ADR115 D7). Each is spelled as callerVerb derives the
// verb that records it, so a refused attempt's row and the success row name
// the same action. AddToOrganization (an invite redemption or a transfer
// adds the person) and AcceptOrganizationInvite (redeemed at sign-in) have no
// verb of their own and are recorded explicitly.
const (
	AuditVerbOrgInvited              = "members.InviteToOrganization"
	AuditVerbOrgInviteAccepted       = "members.AcceptOrganizationInvite"
	AuditVerbOrgMemberAdded          = "members.AddToOrganization"
	AuditVerbOrgMemberRemoved        = "members.RemoveFromOrganization"
	AuditVerbOrgRoleChanged          = "members.ChangeOrganizationRole"
	AuditVerbJoinWorkspace           = "members.JoinWorkspace"
	AuditVerbJoinWorkspaceAsAdmin    = "members.JoinWorkspaceAsAdmin"
	AuditVerbOrgRenamed              = "workspaces.RenameOrganization"
	AuditVerbOrgWorkspaceCreated     = "workspaces.CreateOrganizationWorkspace"
	AuditVerbOrgWorkspaceDeleted     = "workspaces.Delete"
	AuditVerbWorkspacePrivacyChanged = "workspaces.SetWorkspacePrivacy"
	// The org's login settings (w7/m466): Render's
	// ChangeOrg2FAEnforcementEvent and ChangeOrgAllowedLoginMethodsEvent.
	AuditVerbOrgSetRequireTwoFactor   = "workspaces.SetOrganizationRequireTwoFactor"
	AuditVerbOrgSetRequireGoogleLogin = "workspaces.SetOrganizationRequireGoogleLogin"
)

// OrganizationAudit is one org event's facts beyond its verb: the person or
// workspace it is about, that one's display name, and a from/to pair (a role
// change, a rename, a privacy change).
type OrganizationAudit struct {
	Target     string
	TargetName string
	RoleFrom   string
	RoleTo     string
	From       string
	To         string
}

// withOrganizationDetail copies an org event's facts onto its row. The row's
// resource is the org object, so it is keyed by the org id and appears only
// in the org's log.
func withOrganizationDetail(ev *AuditEvent, a OrganizationAudit) {
	ev.TargetName = a.TargetName
	if a.RoleFrom != "" {
		ev.RoleFrom = &a.RoleFrom
	}
	if a.RoleTo != "" {
		ev.RoleTo = &a.RoleTo
	}
	ev.DisplayNameFrom, ev.DisplayNameTo = a.From, a.To
}

// RecordOrganizationEvent records one org event by the caller, once its
// action has landed (the verb authorized under WithDeferredAllowedWriteAudit).
func (b *Base) RecordOrganizationEvent(ctx context.Context, verb, orgID string, a OrganizationAudit) {
	ev := b.verbAuditEvent(ctx, verb, OrganizationObject(orgID), a.Target)
	withOrganizationDetail(&ev, a)
	b.recordAudit(ctx, ev)
}

// RecordOrganizationEventAs writes one org event through sink by caller: for
// the recorders with no feature Base, the auth gate's sign-in redemption and
// the operator's transfer (the RecordInviteAccepted precedent).
func RecordOrganizationEventAs(ctx context.Context, sink AuditSink, at time.Time, verb, orgID, caller, callerMethod string, a OrganizationAudit) {
	ev := AuditEvent{Verb: verb, Resource: OrganizationObject(orgID), Target: a.Target, Outcome: AuditAllowed, At: at, Caller: caller, CallerMethod: callerMethod}
	withOrganizationDetail(&ev, a)
	RecordAuditEvent(ctx, sink, ev)
}

// JoinedOrganization finishes a workspace invite whose redemption made subject
// a new Member of the workspace's org (Render without SSO: invitees
// "automatically become standard members of the org"): their org role tuple,
// best effort since the reconcile writes it again, and AddOrgMemberEvent.
func JoinedOrganization(ctx context.Context, w OrganizationTupleWriter, sink AuditSink, at time.Time, orgID, subject, callerMethod string) {
	if err := WriteOrgTuples(ctx, w, []Tuple{OrgRoleTuple(orgID, subject, OrgRoleMember)}, nil); err != nil {
		log.Printf("organizations: writing %s's membership of %s: %v", subject, orgID, err)
	}
	RecordOrganizationEventAs(ctx, sink, at, AuditVerbOrgMemberAdded, orgID, subject, callerMethod,
		OrganizationAudit{Target: MemberTarget(subject), RoleTo: OrgRoleMember})
}

// The org refusal codes (ADR115). Each surface answers the same code.
const (
	CodeOrganizationNotFound             = "ORGANIZATION_NOT_FOUND"
	CodeOrganizationPlanOnPrimary        = "ORGANIZATION_PLAN_ON_PRIMARY"
	CodeOrganizationDowngradeBlocked     = "ORGANIZATION_DOWNGRADE_BLOCKED"
	CodeOrganizationPrimaryHasWorkspaces = "ORGANIZATION_PRIMARY_HAS_WORKSPACES"
	CodeOrganizationLastOwner            = "ORGANIZATION_LAST_OWNER"
	CodeOrganizationMemberLastAdmin      = "ORGANIZATION_MEMBER_LAST_ADMIN"
	CodeOrganizationGuestSingleWorkspace = "ORGANIZATION_GUEST_SINGLE_WORKSPACE"
	CodeOrganizationBillingLimit         = "ORGANIZATION_BILLING_LIMIT"
	CodeOrganizationWorkspaceRequired    = "ORGANIZATION_WORKSPACE_REQUIRED"
	CodeOrganizationAlreadyMember        = "ORGANIZATION_ALREADY_MEMBER"
	CodeWorkspaceInOrganization          = "WORKSPACE_IN_ORGANIZATION"
	CodeInvalidPrivacy                   = "INVALID_PRIVACY"
)

// NewOrganizationWorkspaceRequiredError refuses an org-only verb (privacy, a
// join) on a standalone workspace.
func NewOrganizationWorkspaceRequiredError(workspaceID string) *CodedError {
	return NewBadRequestError(CodeOrganizationWorkspaceRequired, "this workspace is not in an organization", map[string]any{"workspaceId": workspaceID})
}

// NewWorkspaceInOrganizationError refuses moving or upgrading into an org a
// workspace that already belongs to one.
func NewWorkspaceInOrganizationError(workspaceID string) *CodedError {
	return NewConflictError(CodeWorkspaceInOrganization, "this workspace already belongs to an organization", map[string]any{"workspaceId": workspaceID})
}

// OrganizationPlan reports whether a workspace on plan belongs to an
// organization (Scale and Enterprise): an upgrade into it creates the org,
// and a downgrade out of it dissolves the org.
func OrganizationPlan(plan string) bool { return PlanAllows(plan, FeatureOrganizations) }

// OrganizationRoles are the org roles a plan offers: Render's pricing table
// reads "Organization user roles: — | — | Owner, Member, Guest | Owner,
// Member, Guest". None below Scale, where there is no org.
func OrganizationRoles(plan string) []string {
	if !OrganizationPlan(plan) {
		return nil
	}
	return []string{OrgRoleOwner, OrgRoleMember, OrgRoleGuest}
}

// OrganizationBillingLimit is Render's cap: "Each org supports up to two total
// members with the Billing role" (render.com/docs/organizations).
const OrganizationBillingLimit = 2

// NewOrganizationNotFoundError is the one answer for an org that does not
// exist and for one the caller is not in (403, an ErrForbidden), so an org id
// reveals nothing to someone outside it.
func NewOrganizationNotFoundError(orgID string) *CodedError {
	return NewForbiddenError(CodeOrganizationNotFound, "organization not found, or you are not in it", map[string]any{"organizationId": orgID})
}

// NewOrganizationPlanOnPrimaryError refuses a plan change on a secondary
// workspace: an org's plan is managed on its primary.
func NewOrganizationPlanOnPrimaryError(orgID, primaryID string) *CodedError {
	return NewBadRequestError(CodeOrganizationPlanOnPrimary,
		"this workspace's plan is its organization's; change the plan on the organization's primary workspace",
		map[string]any{"organizationId": orgID, "primaryWorkspaceId": primaryID})
}

// The kinds of DowngradePrecondition.
const (
	PreconditionSecondaryWorkspaces = "secondary_workspaces"
	PreconditionMemberRoles         = "member_roles"
	PreconditionInviteRoles         = "invite_roles"
	PreconditionInboundIPRules      = "inbound_ip_rules"
	// PreconditionMemberLimit names, on a target plan with a member cap
	// (Hobby seats one), every member and pending invite beyond it.
	PreconditionMemberLimit = "member_limit"
	// PreconditionServiceLimit and PreconditionWorkspaceLimit are the plan
	// page's blockers for a plan's service cap and the cap on a person's
	// workspaces of that plan (Hobby: 25 and 5); no org refusal carries them
	// (w3/m708).
	PreconditionServiceLimit   = "service_limit"
	PreconditionWorkspaceLimit = "workspace_limit"
)

// DowngradePrecondition is one of Render's preconditions for downgrading an
// org's primary below Scale that does not hold yet (ADR115 D6), and the shape
// of each blocker the plan page draws before a plan change (w3/m708).
type DowngradePrecondition struct {
	Kind string `json:"kind"`
	// Items name what to clear: workspace names, "email:role" pairs (a
	// member's sign-in subject only when no email resolves), or the
	// resources with inbound IP rules.
	Items []string `json:"items"`
	// Limit is the target plan's own number the kind is about: its seats
	// (member_limit), services (service_limit) or workspaces per person
	// (workspace_limit); zero for the other kinds.
	Limit int `json:"limit,omitempty"`
}

// NewOrganizationDowngradeBlockedError lists every unmet precondition.
func NewOrganizationDowngradeBlockedError(orgID, plan string, unmet []DowngradePrecondition) *CodedError {
	pre := make([]map[string]any, 0, len(unmet))
	kinds := make([]string, 0, len(unmet))
	for _, p := range unmet {
		entry := map[string]any{"kind": p.Kind, "items": p.Items}
		if p.Limit > 0 {
			entry["limit"] = p.Limit
		}
		pre = append(pre, entry)
		kinds = append(kinds, p.Kind)
	}
	return NewBadRequestError(CodeOrganizationDowngradeBlocked,
		fmt.Sprintf("downgrading to %s deletes the organization; first clear: %v", plan, kinds),
		map[string]any{"organizationId": orgID, "plan": plan, "preconditions": pre})
}
