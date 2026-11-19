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

package workspaces

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// organizations.go is the workspace side of Render's organizations (w7/m464,
// docs/ADR115-organizations.md): reading the orgs a caller belongs to, the
// org's workspaces (create, privacy, the primary's delete guard), its name,
// and the plan lifecycle ChangePlan runs through (an upgrade creates the org,
// a secondary's plan is its org's, a downgrade below Scale dissolves it once
// Render's preconditions hold). Org membership verbs live in internal/members.

// OrganizationStore is the store slice the org verbs read and write.
// *store.PGStore satisfies it; a WorkspaceStore without it has no
// organizations, and ChangePlan then changes one workspace as before.
type OrganizationStore interface {
	GetOrganization(ctx context.Context, id string) (store.Organization, error)
	ListOrganizationsForSubject(ctx context.Context, subject string) ([]store.OrganizationMembership, error)
	OrganizationRole(ctx context.Context, orgID, subject string) (store.Organization, string, error)
	ListOrganizationMembers(ctx context.Context, orgID string) ([]store.OrganizationMember, error)
	ListOrganizationWorkspaces(ctx context.Context, orgID string) ([]store.Tenant, error)
	// ListMembersOfTenants lists the people of several workspaces in one read.
	ListMembersOfTenants(ctx context.Context, tenantIDs []string) ([]store.TenantMember, error)
	UpgradeToOrganization(ctx context.Context, workspaceID, plan string, owners []string) (store.Tenant, store.Organization, []store.OrganizationMember, error)
	ChangeOrganizationPlan(ctx context.Context, orgID, plan string) ([]store.Tenant, error)
	DissolveOrganization(ctx context.Context, orgID, plan string) (store.Tenant, error)
	CreateOrganizationWorkspace(ctx context.Context, orgID, name, privacy, creator string) (store.Tenant, error)
	AttachWorkspaceToOrganization(ctx context.Context, workspaceID, orgID string) (store.Tenant, []string, error)
	SetWorkspacePrivacy(ctx context.Context, workspaceID, privacy string) (store.Tenant, bool, error)
	RenameOrganization(ctx context.Context, orgID, name string) (store.Organization, error)
	SetOrganizationRequireTwoFactor(ctx context.Context, orgID string, enabled bool) (changed bool, err error)
	SetOrganizationRequireGoogleLogin(ctx context.Context, orgID string, enabled bool) (changed bool, err error)
	ListWorkspaceEnvironments(ctx context.Context, tenantID string) ([]store.Environment, error)
}

func (s *Service) organizations() (OrganizationStore, bool) {
	o, ok := s.Store.(OrganizationStore)
	return o, ok
}

// OrganizationView is an org as the caller sees it.
type OrganizationView struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Plan               string `json:"plan"`
	PrimaryWorkspaceID string `json:"primaryWorkspaceId"`
	// ViewerRole is the caller's org role, OWNER, MEMBER or GUEST (the
	// uppercase wire form workspace roles use).
	ViewerRole string `json:"viewerRole"`
	CreatedAt  string `json:"createdAt"`
	// RequireTwoFactor and RequireGoogleLogin are the org's login settings
	// (w7/m466): every org workspace enforces each beside its own. Every
	// person in the org reads them; only Owners change them.
	RequireTwoFactor   bool `json:"requireTwoFactor"`
	RequireGoogleLogin bool `json:"requireGoogleLogin"`
	// Workspaces are the org's workspaces, the primary first. A Guest sees
	// only the one they belong to.
	Workspaces []OrganizationWorkspaceView `json:"workspaces"`
}

// OrganizationWorkspaceView is one workspace of an org.
type OrganizationWorkspaceView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Plan    string `json:"plan"`
	Privacy string `json:"privacy"`
	Primary bool   `json:"primary"`
	// ViewerIsMember is whether the caller belongs to the workspace, which
	// decides between opening it and offering Join.
	ViewerIsMember bool `json:"viewerIsMember"`
}

// parsePrivacy reads a privacy argument: public or invite_only, either case,
// and "" as Render's default, Invite-only.
func parsePrivacy(privacy string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(privacy)) {
	case "", core.PrivacyInviteOnly, "invite-only":
		return core.PrivacyInviteOnly, nil
	case core.PrivacyPublic:
		return core.PrivacyPublic, nil
	}
	return "", core.NewBadRequestError(core.CodeInvalidPrivacy, "privacy must be public or invite_only", map[string]any{"privacy": privacy})
}

func organizationView(o store.Organization, role string, workspaces []store.Tenant, memberOf map[string]bool) OrganizationView {
	v := OrganizationView{
		ID: o.ID, Name: o.Name, Plan: o.Plan, PrimaryWorkspaceID: o.PrimaryWorkspaceID,
		ViewerRole: strings.ToUpper(role), CreatedAt: o.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		RequireTwoFactor: o.RequireTwoFactor, RequireGoogleLogin: o.RequireGoogleLogin,
		Workspaces: []OrganizationWorkspaceView{},
	}
	for _, t := range workspaces {
		// A Guest "can't access any org resources except those in the single
		// workspace they're invited to" (render.com/docs/organizations).
		if role == core.OrgRoleGuest && !memberOf[t.ID] {
			continue
		}
		v.Workspaces = append(v.Workspaces, OrganizationWorkspaceView{
			ID: t.ID, Name: t.Label(), Slug: t.Name, Plan: t.Plan, Privacy: t.Privacy,
			Primary: t.ID == o.PrimaryWorkspaceID, ViewerIsMember: memberOf[t.ID],
		})
	}
	return v
}

// callerWorkspaceSet is the set of workspaces the caller belongs to.
func (s *Service) callerWorkspaceSet(ctx context.Context, subject string) (map[string]bool, error) {
	ts, err := s.Store.ListTenantsForSubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ts))
	for _, t := range ts {
		out[t.ID] = true
	}
	return out, nil
}

// ListOrganizations returns the orgs the caller belongs to (the switcher's org
// entries), each with the workspaces the caller may see.
func (s *Service) ListOrganizations(ctx context.Context) ([]OrganizationView, error) {
	ctx = core.WithTwoFactorExempt(ctx)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	orgs, ok := s.organizations()
	if !ok {
		return nil, core.ErrWorkspacesUnavailable
	}
	id, _ := core.IdentityFrom(ctx)
	memberships, err := orgs.ListOrganizationsForSubject(ctx, id.Subject)
	if err != nil {
		return nil, err
	}
	memberOf, err := s.callerWorkspaceSet(ctx, id.Subject)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationView, 0, len(memberships))
	for _, m := range memberships {
		workspaces, err := orgs.ListOrganizationWorkspaces(ctx, m.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, organizationView(m.Organization, m.Role, workspaces, memberOf))
	}
	return out, nil
}

// GetOrganization reads one org the caller belongs to. An org that does not
// exist and one the caller is not in answer the same ORGANIZATION_NOT_FOUND.
func (s *Service) GetOrganization(ctx context.Context, orgID string) (OrganizationView, error) {
	ctx = core.WithTwoFactorExempt(ctx)
	if err := s.AuthorizeOn(ctx, core.RelCanView, core.OrganizationObject(orgID)); err != nil {
		if errors.Is(err, core.ErrForbidden) {
			return OrganizationView{}, core.NewOrganizationNotFoundError(orgID)
		}
		return OrganizationView{}, err
	}
	orgs, ok := s.organizations()
	if !ok {
		return OrganizationView{}, core.ErrWorkspacesUnavailable
	}
	id, _ := core.IdentityFrom(ctx)
	o, role, err := organizationOf(ctx, orgs, orgID, id.Subject)
	if err != nil {
		return OrganizationView{}, err
	}
	workspaces, err := orgs.ListOrganizationWorkspaces(ctx, orgID)
	if err != nil {
		return OrganizationView{}, err
	}
	memberOf, err := s.callerWorkspaceSet(ctx, id.Subject)
	if err != nil {
		return OrganizationView{}, err
	}
	return organizationView(o, role, workspaces, memberOf), nil
}

// organizationOf reads an org and subject's role in it from the source of
// truth. Not being in the org is ORGANIZATION_NOT_FOUND whatever OpenFGA
// answered, so an authz-off deployment still reveals no one else's org.
func organizationOf(ctx context.Context, orgs OrganizationStore, orgID, subject string) (store.Organization, string, error) {
	o, role, err := orgs.OrganizationRole(ctx, orgID, subject)
	if errors.Is(err, store.ErrNotFound) {
		return store.Organization{}, "", core.NewOrganizationNotFoundError(orgID)
	}
	return o, role, err
}

// requireOwner reads the org and refuses a caller who is not its Owner.
func requireOwner(ctx context.Context, orgs OrganizationStore, orgID string) (store.Organization, error) {
	id, _ := core.IdentityFrom(ctx)
	o, role, err := organizationOf(ctx, orgs, orgID, id.Subject)
	if err == nil && role != core.OrgRoleOwner {
		err = core.ErrForbidden
	}
	return o, err
}

// CreateOrganizationWorkspace creates a workspace in the org on the org's plan
// with no plan fee and no payment step, Public or Invite-only (Invite-only by
// default), and makes the caller its first admin. Owner only (ADR115 D3:
// "Org Owners can create a new workspace").
func (s *Service) CreateOrganizationWorkspace(ctx context.Context, orgID, name, privacy string) (WorkspaceView, error) {
	// A new workspace is a durable administrator authority, as in Create.
	if err := s.AuthorizeMintClass(ctx); err != nil {
		return WorkspaceView{}, err
	}
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	if err := s.AuthorizeOn(ctx, core.RelCanCreate, core.OrganizationObject(orgID)); err != nil {
		return WorkspaceView{}, err
	}
	orgs, ok := s.organizations()
	if !ok {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	if err := validWorkspaceName(name); err != nil {
		return WorkspaceView{}, err
	}
	privacy, err := parsePrivacy(privacy)
	if err != nil {
		return WorkspaceView{}, err
	}
	if _, err := requireOwner(ctx, orgs, orgID); err != nil {
		return WorkspaceView{}, err
	}
	id, _ := core.IdentityFrom(ctx)
	t, err := orgs.CreateOrganizationWorkspace(ctx, orgID, name, privacy, id.Subject)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	if s.Granter != nil {
		if err := s.Granter.GrantWorkspaceAdmin(ctx, t.ID, "user:"+id.Subject); err != nil {
			_ = s.Store.DeleteTenant(ctx, t.ID)
			return WorkspaceView{}, fmt.Errorf("workspace %s: granting admin failed: %w", t.ID, err)
		}
	}
	s.writeOrgTuples(ctx, workspaceTuples(orgID, t), nil)
	s.RecordOrganizationEvent(ctx, core.AuditVerbOrgWorkspaceCreated, orgID, core.OrganizationAudit{Target: core.WorkspaceObject(t.ID), TargetName: t.Label()})
	return view(t, "admin"), nil
}

// workspaceTuples are the tuples an org workspace implies: its org link, and
// its public link while it is Public.
func workspaceTuples(orgID string, t store.Tenant) []core.Tuple {
	out := []core.Tuple{core.OrgWorkspaceTuple(orgID, t.ID)}
	if t.Privacy == core.PrivacyPublic {
		out = append(out, core.OrgPublicTuple(orgID, t.ID))
	}
	return out
}

// organizationTuples are every tuple an org's rows imply.
func organizationTuples(o store.OrganizationTuples) []core.Tuple {
	var out []core.Tuple
	for _, m := range o.Members {
		out = append(out, core.OrgRoleTuple(o.Organization.ID, m.Subject, m.Role))
	}
	for _, t := range o.Workspaces {
		out = append(out, workspaceTuples(o.Organization.ID, t)...)
	}
	return out
}

// writeOrgTuples applies an org change's tuples after its rows committed. A
// failure is logged, not returned: the rows are the source of truth, and the
// org tuple reconcile (OrganizationTupleReconciler) writes them again.
func (s *Service) writeOrgTuples(ctx context.Context, writes, deletes []core.Tuple) {
	if err := core.WriteOrgTuples(ctx, s.OrgTuples, writes, deletes); err != nil {
		log.Printf("workspaces: writing organization tuples: %v", err)
	}
}

// SetWorkspacePrivacy sets an org workspace's privacy, Public or Invite-only.
// A workspace admin's setting (Render: "Workspace admins can change a
// workspace's privacy setting"). A standalone workspace has none:
// ORGANIZATION_WORKSPACE_REQUIRED.
func (s *Service) SetWorkspacePrivacy(ctx context.Context, workspaceID, privacy string) (WorkspaceView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, workspaceID)
	if err != nil {
		return WorkspaceView{}, err
	}
	orgs, ok := s.organizations()
	if !ok {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	privacy, err = parsePrivacy(privacy)
	if err != nil {
		return WorkspaceView{}, err
	}
	t, err := s.Store.GetTenant(ctx, workspaceID)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	if t.OrganizationID == "" {
		return WorkspaceView{}, core.NewOrganizationWorkspaceRequiredError(workspaceID)
	}
	if t.Privacy == privacy {
		return view(t, "admin"), nil
	}
	public := core.OrgPublicTuple(t.OrganizationID, workspaceID)
	// Closing a workspace takes the join grant away before the row says so,
	// so no window lets a Member join an Invite-only workspace.
	if privacy == core.PrivacyInviteOnly {
		if err := core.WriteOrgTuples(ctx, s.OrgTuples, nil, []core.Tuple{public}); err != nil {
			return WorkspaceView{}, fmt.Errorf("workspace %s: removing its public link: %w", workspaceID, err)
		}
	}
	nt, changed, err := orgs.SetWorkspacePrivacy(ctx, workspaceID, privacy)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	if privacy == core.PrivacyPublic {
		s.writeOrgTuples(ctx, []core.Tuple{public}, nil)
	}
	if changed {
		s.RecordOrganizationEvent(ctx, core.AuditVerbWorkspacePrivacyChanged, t.OrganizationID, core.OrganizationAudit{
			Target: core.WorkspaceObject(workspaceID), TargetName: nt.Label(), From: t.Privacy, To: privacy,
		})
	}
	return view(nt, "admin"), nil
}

// RenameOrganization renames an org. Owner only; recorded as Render's
// ChangeOrgNameEvent.
func (s *Service) RenameOrganization(ctx context.Context, orgID, name string) (OrganizationView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.OrganizationObject(orgID)); err != nil {
		return OrganizationView{}, err
	}
	orgs, ok := s.organizations()
	if !ok {
		return OrganizationView{}, core.ErrWorkspacesUnavailable
	}
	name, err := core.TrimmedLabel(name)
	if err != nil {
		return OrganizationView{}, err
	}
	if name == "" {
		return OrganizationView{}, core.NameRequiredError()
	}
	o, err := requireOwner(ctx, orgs, orgID)
	if err != nil {
		return OrganizationView{}, err
	}
	if o.Name != name {
		if _, err := orgs.RenameOrganization(ctx, orgID, name); err != nil {
			return OrganizationView{}, mapStoreErr(err)
		}
		s.RecordOrganizationEvent(ctx, core.AuditVerbOrgRenamed, orgID, core.OrganizationAudit{From: o.Name, To: name})
	}
	return s.GetOrganization(ctx, orgID)
}

// SetOrganizationRequireTwoFactor turns the org's "require two-factor
// authentication" setting on or off (w7/m466, docs/ADR012-auth.md §9c;
// Render's ChangeOrg2FAEnforcementEvent). Owner only. While it is on, every
// workspace of the org requires 2FA beside its own setting, and so do the
// org's own objects; turning it off leaves each workspace's own setting as it
// was. It has the workspace setting's rules: the org's plan must allow it, a
// no-op writes and records nothing, and turning it on is refused while the
// acting Owner has no second factor (409 WORKSPACE_2FA_ADMIN_NOT_ENROLLED,
// naming the org) or the deployment cannot look factors up (409
// WORKSPACE_2FA_UNAVAILABLE).
func (s *Service) SetOrganizationRequireTwoFactor(ctx context.Context, orgID string, enabled bool) (OrganizationView, error) {
	return s.setOrganizationLoginRequirement(ctx, orgID, enabled, twoFactorSetting)
}

// SetOrganizationRequireGoogleLogin turns the org's "require Google login"
// setting on or off (Render's ChangeOrgAllowedLoginMethodsEvent, the org's
// allowed login methods without SAML SSO), with SetOrganizationRequireTwoFactor's
// rules; its lock-out refusal is 409 WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN.
func (s *Service) SetOrganizationRequireGoogleLogin(ctx context.Context, orgID string, enabled bool) (OrganizationView, error) {
	return s.setOrganizationLoginRequirement(ctx, orgID, enabled, googleLoginSetting)
}

// setOrganizationLoginRequirement is the body both org mutations share: the
// Owner's authorization on the org, setLoginRequirement's rules on the org's
// plan, and a change drops this replica's cached setting of the org and of
// each of its workspaces (other replicas within the gate's TTL).
func (s *Service) setOrganizationLoginRequirement(ctx context.Context, orgID string, enabled bool, setting loginSetting) (OrganizationView, error) {
	// Denials stay on the record; the allowed row waits for the write.
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.OrganizationObject(orgID)); err != nil {
		return OrganizationView{}, err
	}
	orgs, ok := s.organizations()
	if !ok {
		return OrganizationView{}, core.ErrWorkspacesUnavailable
	}
	o, err := requireOwner(ctx, orgs, orgID)
	if err != nil {
		return OrganizationView{}, err
	}
	if setting.orgCurrent(o) == enabled {
		return s.GetOrganization(ctx, orgID) // already there — nothing to write
	}
	if enabled {
		if err := s.guardLoginRequirement(ctx, o.Plan, setting, setting.ownerNotMet(orgID)); err != nil {
			return OrganizationView{}, err
		}
	}
	changed, err := setting.orgWrite(orgs, ctx, orgID, enabled)
	if err != nil {
		return OrganizationView{}, mapStoreErr(err)
	}
	v, err := s.GetOrganization(ctx, orgID)
	if err != nil || !changed {
		return v, err // !changed: another Owner made this same change first
	}
	if s.TwoFactorChanged != nil {
		// An Owner's view lists every org workspace.
		s.TwoFactorChanged(core.OrganizationObject(orgID))
		for _, w := range v.Workspaces {
			s.TwoFactorChanged(w.ID)
		}
	}
	setting.orgRecord(s, ctx, orgID, enabled)
	return v, nil
}

// changeOrganizationPlan is ChangePlan for a workspace in an org (ADR115 D2,
// D6). A secondary's plan is its org's, changed on the primary. On the primary
// a change between Scale and Enterprise moves every org workspace; a change
// below Scale is refused until Render's preconditions hold, and then deletes
// the org.
func (s *Service) changeOrganizationPlan(ctx context.Context, orgs OrganizationStore, t store.Tenant, plan string) (WorkspaceView, error) {
	o, err := orgs.GetOrganization(ctx, t.OrganizationID)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	if t.ID != o.PrimaryWorkspaceID {
		return WorkspaceView{}, core.NewOrganizationPlanOnPrimaryError(o.ID, o.PrimaryWorkspaceID)
	}
	if core.OrganizationPlan(plan) {
		if err := s.RequirePlanBilling(ctx, t.ID, plan); err != nil {
			return WorkspaceView{}, err
		}
		moved, err := orgs.ChangeOrganizationPlan(ctx, o.ID, plan)
		if err != nil {
			return WorkspaceView{}, mapStoreErr(err)
		}
		for _, w := range moved {
			if w.ID == t.ID {
				return view(w, "admin"), nil
			}
		}
		return WorkspaceView{}, fmt.Errorf("organization %s: its primary %s was not moved", o.ID, t.ID)
	}
	workspaces, err := orgs.ListOrganizationWorkspaces(ctx, o.ID)
	if err != nil {
		return WorkspaceView{}, err
	}
	unmet, err := s.downgradePreconditions(ctx, orgs, o, workspaces, plan)
	if err != nil {
		return WorkspaceView{}, err
	}
	if len(unmet) > 0 {
		return WorkspaceView{}, core.NewOrganizationDowngradeBlockedError(o.ID, plan, unmet)
	}
	if err := s.guardPlanCapacity(ctx, t.ID, plan); err != nil {
		return WorkspaceView{}, err
	}
	members, err := orgs.ListOrganizationMembers(ctx, o.ID)
	if err != nil {
		return WorkspaceView{}, err
	}
	tuples := organizationTuples(store.OrganizationTuples{Organization: o, Members: members, Workspaces: workspaces})
	// The org's grants go before its rows: a failure leaves an org whose
	// Owners lost org rights until the reconcile restores them, never a
	// dissolved org whose Owners keep them.
	if err := core.WriteOrgTuples(ctx, s.OrgTuples, nil, tuples); err != nil {
		return WorkspaceView{}, fmt.Errorf("organization %s: removing its grants: %w", o.ID, err)
	}
	nt, err := orgs.DissolveOrganization(ctx, o.ID, plan)
	if err != nil {
		s.writeOrgTuples(ctx, tuples, nil)
		if errors.Is(err, store.ErrOrganizationHasWorkspaces) {
			return WorkspaceView{}, core.NewOrganizationDowngradeBlockedError(o.ID, plan, []core.DowngradePrecondition{{Kind: core.PreconditionSecondaryWorkspaces}})
		}
		return WorkspaceView{}, mapStoreErr(err)
	}
	return view(nt, "admin"), nil
}

// downgradePreconditions lists Render's preconditions for downgrading the
// org's primary below Scale that do not hold (ADR115 D6, render.com/docs/
// organizations): the secondary workspaces; Billing, Viewer and Contributor
// members and pending invites (the roles only an org offers, judged against
// Pro whatever the target, so a Hobby target does not list a Developer
// under them); and inbound IP rules on the workspace, its environments and
// its web services and static sites. Rules on Postgres and Key Value are not
// affected. A target with a member cap (Hobby seats one) also names everyone
// beyond it. People are named by email (w7/087).
func (s *Service) downgradePreconditions(ctx context.Context, orgs OrganizationStore, o store.Organization, workspaces []store.Tenant, plan string) ([]core.DowngradePrecondition, error) {
	var unmet []core.DowngradePrecondition
	var secondaries []string
	for _, w := range workspaces {
		if w.ID != o.PrimaryWorkspaceID {
			secondaries = append(secondaries, w.Label())
		}
	}
	if len(secondaries) > 0 {
		unmet = append(unmet, core.DowngradePrecondition{Kind: core.PreconditionSecondaryWorkspaces, Items: secondaries})
	}
	members, err := s.Store.ListTenantMembers(ctx, o.PrimaryWorkspaceID)
	if err != nil {
		return nil, err
	}
	if blocked := rolesOutsidePlan(members, store.PlanPro); len(blocked) > 0 {
		unmet = append(unmet, core.DowngradePrecondition{Kind: core.PreconditionMemberRoles, Items: s.memberRoles(ctx, blocked)})
	}
	invites, err := s.Store.ListInvites(ctx, o.PrimaryWorkspaceID)
	if err != nil {
		return nil, err
	}
	if blocked := invitesOutsidePlan(invites, store.PlanPro); len(blocked) > 0 {
		unmet = append(unmet, core.DowngradePrecondition{Kind: core.PreconditionInviteRoles, Items: invitedRoles(blocked)})
	}
	if limit := store.LimitsFor(plan).MaxMembers; limit > 0 {
		caller, _ := core.IdentityFrom(ctx)
		over, overInvites := unseated(members, invites, limit, caller.Subject)
		if items := append(s.memberRoles(ctx, over), invitedRoles(overInvites)...); len(items) > 0 {
			unmet = append(unmet, core.DowngradePrecondition{Kind: core.PreconditionMemberLimit, Items: items})
		}
	}
	rules, err := s.inboundIPRules(ctx, orgs, o.PrimaryWorkspaceID)
	if err != nil {
		return nil, err
	}
	if len(rules) > 0 {
		unmet = append(unmet, core.DowngradePrecondition{Kind: core.PreconditionInboundIPRules, Items: rules})
	}
	return unmet, nil
}

// unseated returns, in the order they joined, the members and pending
// invites a plan seating limit people cannot keep (w7/087; Hobby seats one).
// The caller, who manages the workspace and so is its admin, keeps their seat
// first, then the other admins and the rest by when they joined; pending
// invites take the seats left over.
func unseated(members []store.TenantMember, invites []store.Invite, limit int, caller string) ([]store.TenantMember, []store.Invite) {
	if len(members) < limit {
		if seats := limit - len(members); len(invites) > seats {
			return nil, invites[seats:]
		}
		return nil, nil
	}
	rank := func(m store.TenantMember) int {
		switch {
		case m.Subject == caller:
			return 0
		case m.Role == "admin":
			return 1
		default:
			return 2
		}
	}
	seated := slices.Clone(members)
	slices.SortStableFunc(seated, func(a, b store.TenantMember) int { return rank(a) - rank(b) })
	seated = seated[:limit]
	var over []store.TenantMember
	for _, m := range members {
		if !slices.ContainsFunc(seated, func(k store.TenantMember) bool { return k.Subject == m.Subject }) {
			over = append(over, m)
		}
	}
	return over, invites
}

// inboundIPRules names the restricting inbound IP rules a workspace holds
// outside its datastores: "workspace", "environment:<name>" and
// "service:<name>" for each web service or static site.
func (s *Service) inboundIPRules(ctx context.Context, orgs OrganizationStore, workspaceID string) ([]string, error) {
	var out []string
	if st, ok := s.Store.(ipRulesStore); ok {
		entries, set, err := st.WorkspaceIPAllowList(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		if set && core.RestrictsInbound(entries) {
			out = append(out, "workspace")
		}
	}
	envs, err := orgs.ListWorkspaceEnvironments(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, e := range envs {
		if core.RestrictsInbound(e.IPAllowList) {
			out = append(out, "environment:"+e.Name)
		}
	}
	if s.Client == nil {
		return out, nil
	}
	var apps appv1alpha1.AppList
	if err := s.ListByTenant(ctx, &apps, workspaceID); err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}
	for _, a := range core.ServiceApps(apps.Items) {
		if serviceRestrictsInbound(a.Spec) {
			out = append(out, "service:"+core.DisplayedAppName(&a))
		}
	}
	slices.Sort(out)
	return out, nil
}

// serviceRestrictsInbound reports whether a web service or static site has
// its own restricting inbound IP rules; an empty list is open (Render's
// default for a service).
func serviceRestrictsInbound(spec appv1alpha1.AppSpec) bool {
	if spec.Type != appv1alpha1.TypeWebService && spec.Type != appv1alpha1.TypeStaticSite {
		return false
	}
	entries := spec.EffectiveIPAllowListEntries()
	return len(entries) > 0 && core.RestrictsServiceInbound(core.AllowListFromSpec(entries))
}

// upgradeToOrganization is ChangePlan's upgrade of a standalone workspace to
// Scale or Enterprise: the plan write and the new org land in one
// transaction, with the caller as Owner (ADR115 D2).
func (s *Service) upgradeToOrganization(ctx context.Context, orgs OrganizationStore, workspaceID, plan string) (WorkspaceView, error) {
	id, _ := core.IdentityFrom(ctx)
	t, o, members, err := orgs.UpgradeToOrganization(ctx, workspaceID, plan, []string{id.Subject})
	if err != nil {
		if errors.Is(err, store.ErrInOrganization) {
			return WorkspaceView{}, core.NewWorkspaceInOrganizationError(workspaceID)
		}
		return WorkspaceView{}, mapStoreErr(err)
	}
	s.writeOrgTuples(ctx, organizationTuples(store.OrganizationTuples{Organization: o, Members: members, Workspaces: []store.Tenant{t}}), nil)
	return view(t, "admin"), nil
}

// OrganizationTupleReconciler writes every org's tuples into OpenFGA, which
// is idempotent: bex-api runs it at start and every ten minutes, so the
// migration's backfill gets its tuples and a tuple write lost to a crash
// comes back (ADR115 D5). It only writes; every removal deletes its tuples
// before its rows.
type OrganizationTupleReconciler struct {
	Store interface {
		AllOrganizationTuples(ctx context.Context) ([]store.OrganizationTuples, error)
	}
	Tuples core.OrganizationTupleWriter
}

// ReconcileOnce writes every org's tuples once.
func (r OrganizationTupleReconciler) ReconcileOnce(ctx context.Context) error {
	if r.Tuples == nil {
		return nil
	}
	all, err := r.Store.AllOrganizationTuples(ctx)
	if err != nil {
		return err
	}
	var writes []core.Tuple
	for _, o := range all {
		writes = append(writes, organizationTuples(o)...)
	}
	return r.Tuples.WriteOrganizationTuples(ctx, writes, nil)
}

// OrganizationTransfer is Render support's transfer of an existing workspace
// into an org (ADR115 D3), run by an operator through bex-api's
// move-workspace-to-organization command. It carries no caller identity, so it
// is not a Service verb and never mounts on a public surface.
type OrganizationTransfer struct{ Service *Service }

// TransferPlan is what a transfer does or would do.
type TransferPlan struct {
	WorkspaceID    string   `json:"workspaceId"`
	WorkspaceName  string   `json:"workspaceName"`
	OrganizationID string   `json:"organizationId"`
	Organization   string   `json:"organization"`
	PlanFrom       string   `json:"planFrom"`
	PlanTo         string   `json:"planTo"`
	NewMembers     []string `json:"newMembers"`
	Applied        bool     `json:"applied"`
}

// Move attaches a standalone workspace to the org; without apply it only
// reports what it would do. A workspace already in an org is refused.
func (x OrganizationTransfer) Move(ctx context.Context, workspaceID, orgID string, apply bool) (TransferPlan, error) {
	s := x.Service
	if s == nil {
		return TransferPlan{}, core.ErrWorkspacesUnavailable
	}
	orgs, ok := s.organizations()
	if !ok {
		return TransferPlan{}, core.ErrWorkspacesUnavailable
	}
	o, err := orgs.GetOrganization(ctx, orgID)
	if err != nil {
		return TransferPlan{}, mapStoreErr(err)
	}
	t, err := s.Store.GetTenant(ctx, workspaceID)
	if err != nil {
		return TransferPlan{}, mapStoreErr(err)
	}
	if t.OrganizationID != "" {
		return TransferPlan{}, core.NewWorkspaceInOrganizationError(workspaceID)
	}
	plan := TransferPlan{WorkspaceID: t.ID, WorkspaceName: t.Label(), OrganizationID: o.ID, Organization: o.Name, PlanFrom: t.Plan, PlanTo: o.Plan, NewMembers: []string{}}
	members, err := s.Store.ListTenantMembers(ctx, workspaceID)
	if err != nil {
		return TransferPlan{}, err
	}
	inOrg, err := orgs.ListOrganizationMembers(ctx, orgID)
	if err != nil {
		return TransferPlan{}, err
	}
	for _, m := range members {
		if !slices.ContainsFunc(inOrg, func(om store.OrganizationMember) bool { return om.Subject == m.Subject }) {
			plan.NewMembers = append(plan.NewMembers, m.Subject)
		}
	}
	if !apply {
		return plan, nil
	}
	moved, added, err := orgs.AttachWorkspaceToOrganization(ctx, workspaceID, orgID)
	if err != nil {
		if errors.Is(err, store.ErrInOrganization) {
			return TransferPlan{}, core.NewWorkspaceInOrganizationError(workspaceID)
		}
		return TransferPlan{}, mapStoreErr(err)
	}
	plan.NewMembers = added
	var writes []core.Tuple
	writes = append(writes, workspaceTuples(orgID, moved)...)
	for _, subject := range added {
		writes = append(writes, core.OrgRoleTuple(orgID, subject, core.OrgRoleMember))
	}
	if err := core.WriteOrgTuples(ctx, s.OrgTuples, writes, nil); err != nil {
		return plan, fmt.Errorf("workspace %s moved, but writing its org tuples failed (the reconcile writes them again): %w", workspaceID, err)
	}
	for _, subject := range added {
		core.RecordOrganizationEventAs(ctx, s.Audit, s.Now(), core.AuditVerbOrgMemberAdded, orgID, "bex-operator", "system",
			core.OrganizationAudit{Target: core.MemberTarget(subject), RoleTo: core.OrgRoleMember})
	}
	plan.Applied = true
	return plan, nil
}

// organizationDelete is what deleting an org workspace changes in its org:
// the org it leaves, and the tuples that go with it.
type organizationDelete struct {
	org     store.Organization
	deletes []core.Tuple
}

// guardOrganizationDelete refuses deleting an org's primary while it has
// secondaries (ADR115 D3): the primary holds the org's plan, and Render's
// downgrade deletes the secondaries first. Deleting a primary that is alone
// deletes the org (the rows cascade), so its members' org tuples are read
// here, before they are gone. A standalone workspace answers a zero value.
func (s *Service) guardOrganizationDelete(ctx context.Context, t store.Tenant) (organizationDelete, error) {
	orgs, ok := s.organizations()
	if !ok || t.OrganizationID == "" {
		return organizationDelete{}, nil
	}
	o, err := orgs.GetOrganization(ctx, t.OrganizationID)
	if err != nil {
		return organizationDelete{}, mapStoreErr(err)
	}
	d := organizationDelete{org: o, deletes: []core.Tuple{core.OrgWorkspaceTuple(o.ID, t.ID), core.OrgPublicTuple(o.ID, t.ID)}}
	if t.ID != o.PrimaryWorkspaceID {
		return d, nil
	}
	workspaces, err := orgs.ListOrganizationWorkspaces(ctx, o.ID)
	if err != nil {
		return organizationDelete{}, err
	}
	if len(workspaces) > 1 {
		return organizationDelete{}, core.NewConflictError(core.CodeOrganizationPrimaryHasWorkspaces,
			"delete the organization's other workspaces before its primary workspace",
			map[string]any{"organizationId": o.ID, "workspaces": len(workspaces) - 1})
	}
	members, err := orgs.ListOrganizationMembers(ctx, o.ID)
	if err != nil {
		return organizationDelete{}, err
	}
	for _, m := range members {
		d.deletes = append(d.deletes, core.OrgRoleTuple(o.ID, m.Subject, m.Role))
	}
	return d, nil
}

// recordOrganizationDelete records Render's DeleteWorkspaceEvent in the org's
// log and drops the deleted workspace's org tuples.
func (s *Service) recordOrganizationDelete(ctx context.Context, d organizationDelete, t store.Tenant) {
	if d.org.ID == "" {
		return
	}
	s.RecordOrganizationEvent(ctx, core.AuditVerbOrgWorkspaceDeleted, d.org.ID, core.OrganizationAudit{Target: core.WorkspaceObject(t.ID), TargetName: t.Label()})
	s.writeOrgTuples(ctx, nil, d.deletes)
}
