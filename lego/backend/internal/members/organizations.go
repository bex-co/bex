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

package members

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/email"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// organizations.go is the membership side of Render's organizations (w7/m464,
// docs/ADR115-organizations.md D4): who is in an org and as what (Owner,
// Member, Guest), org invites, removing someone from the whole org, and an
// org member joining a workspace. The Billing cap and the Guest's single
// workspace hook into the workspace verbs (Invite, ChangeRole).

// OrganizationStore is the store slice the org membership verbs use.
// *store.PGStore satisfies it; a MembersStore without it has no orgs.
type OrganizationStore interface {
	OrganizationRole(ctx context.Context, orgID, subject string) (store.Organization, string, error)
	GetOrganizationMember(ctx context.Context, orgID, subject string) (store.OrganizationMember, error)
	ListOrganizationMembers(ctx context.Context, orgID string) ([]store.OrganizationMember, error)
	SubjectOrganizationWorkspaces(ctx context.Context, orgID, subject string) ([]string, error)
	OrganizationMemberWorkspaces(ctx context.Context, orgID string) (map[string][]string, error)
	SetOrganizationMemberRole(ctx context.Context, orgID, subject, role string) (string, error)
	RemoveOrganizationMember(ctx context.Context, orgID, subject string) error
	CreateOrganizationInvite(ctx context.Context, orgID, email, role, token, invitedBy string, expiresAt time.Time) (store.OrganizationInvite, error)
	ListOrganizationInvites(ctx context.Context, orgID string) ([]store.OrganizationInvite, error)
	DeleteOrganizationInvite(ctx context.Context, orgID, id string) error
	OrganizationBillingHolders(ctx context.Context, orgID string) (subjects, inviteEmails []string, err error)
	JoinOrganizationWorkspace(ctx context.Context, workspaceID, subject, role string) (bool, error)
}

func (s *Service) organizations() (OrganizationStore, bool) {
	o, ok := s.Store.(OrganizationStore)
	return o, ok
}

// OrganizationMemberView is a person in an org.
type OrganizationMemberView struct {
	Subject   string `json:"subject"`
	UserID    string `json:"userId"`
	Email     string `json:"email,omitempty"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
	// Workspaces are the ids of the org workspaces the person belongs to: a
	// Guest's one workspace, and what removing them from the org ends.
	Workspaces []string `json:"workspaces"`
	IsSelf     bool     `json:"isSelf"`
}

// OrganizationInviteView is a pending org invite.
type OrganizationInviteView struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expiresAt"`
	CreatedAt string `json:"createdAt"`
}

// JoinedWorkspaceView is the membership a join created.
type JoinedWorkspaceView struct {
	WorkspaceID string `json:"workspaceId"`
	Role        string `json:"role"`
	Joined      bool   `json:"joined"`
}

func organizationInviteView(inv store.OrganizationInvite) OrganizationInviteView {
	return OrganizationInviteView{ID: inv.ID, Email: inv.Email, Role: wireRole(inv.Role), ExpiresAt: rfc3339(inv.ExpiresAt), CreatedAt: rfc3339(inv.CreatedAt)}
}

// organizationNotFound is the one answer for an org that does not exist and
// for one the caller is not in.
func organizationNotFound(orgID string, err error) error {
	if core.IsLoginRefusal(err) {
		return err // a member the org's login setting refuses is told why (w7/m466)
	}
	if err == nil || errors.Is(err, core.ErrForbidden) || errors.Is(err, store.ErrNotFound) {
		return core.NewOrganizationNotFoundError(orgID)
	}
	return err
}

// callerOrgRole reads the caller's org role from the source of truth; not
// being in the org is ORGANIZATION_NOT_FOUND whatever OpenFGA answered.
func (s *Service) callerOrgRole(ctx context.Context, orgs OrganizationStore, orgID string) (store.Organization, string, error) {
	id, _ := core.IdentityFrom(ctx)
	o, role, err := orgs.OrganizationRole(ctx, orgID, id.Subject)
	if err != nil {
		return store.Organization{}, "", organizationNotFound(orgID, err)
	}
	return o, role, nil
}

// requireOrgOwner refuses a caller who is not the org's Owner.
func (s *Service) requireOrgOwner(ctx context.Context, orgs OrganizationStore, orgID string) (store.Organization, error) {
	o, role, err := s.callerOrgRole(ctx, orgs, orgID)
	if err != nil {
		return store.Organization{}, err
	}
	if role != core.OrgRoleOwner {
		return store.Organization{}, core.ErrForbidden
	}
	return o, nil
}

// canonicalOrgRole validates an org role argument, either case.
func canonicalOrgRole(role string, allowed ...string) (string, error) {
	r := strings.ToLower(strings.TrimSpace(role))
	if slices.Contains(allowed, r) {
		return r, nil
	}
	wire := make([]string, len(allowed))
	for i, a := range allowed {
		wire[i] = wireRole(a)
	}
	return "", fmt.Errorf("%w: role must be one of %s", core.ErrBadRequest, strings.Join(wire, "|"))
}

// writeOrgTuples applies org tuple changes, failing on error: every caller
// that removes a grant must not report success over a grant it left.
func (s *Service) writeOrgTuples(ctx context.Context, writes, deletes []core.Tuple) error {
	return core.WriteOrgTuples(ctx, s.OrgTuples, writes, deletes)
}

// ListOrganizationMembers lists an org's people with their org roles and the
// org workspaces each belongs to. Owners and Members see it; a Guest does not
// (they "can't access any org resources except those in the single workspace
// they're invited to").
func (s *Service) ListOrganizationMembers(ctx context.Context, orgID string) ([]OrganizationMemberView, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanView, core.OrganizationObject(orgID)); err != nil {
		return nil, organizationNotFound(orgID, err)
	}
	orgs, ok := s.organizations()
	if !ok {
		return nil, ErrMembersUnavailable
	}
	_, role, err := s.callerOrgRole(ctx, orgs, orgID)
	if err != nil {
		return nil, err
	}
	if role == core.OrgRoleGuest {
		return nil, core.ErrForbidden
	}
	members, err := orgs.ListOrganizationMembers(ctx, orgID)
	if err != nil {
		return nil, err
	}
	subjects := make([]string, 0, len(members))
	for _, m := range members {
		subjects = append(subjects, m.Subject)
	}
	ownIDs, err := s.Store.OwnerIDsForSubjects(ctx, subjects)
	if err != nil {
		return nil, err
	}
	var emails map[string]string
	if s.Identities != nil {
		emails, _ = s.Identities.LookupEmails(ctx, subjects)
	}
	memberOf, err := orgs.OrganizationMemberWorkspaces(ctx, orgID)
	if err != nil {
		return nil, err
	}
	caller, _ := core.IdentityFrom(ctx)
	out := make([]OrganizationMemberView, 0, len(members))
	for _, m := range members {
		workspaces := memberOf[m.Subject]
		if workspaces == nil {
			workspaces = []string{}
		}
		out = append(out, OrganizationMemberView{
			Subject: m.Subject, UserID: ids.UserPublicID(ownIDs[m.Subject]), Email: emails[m.Subject],
			Role: wireRole(m.Role), CreatedAt: rfc3339(m.CreatedAt), Workspaces: workspaces,
			IsSelf: m.Subject == caller.Subject,
		})
	}
	return out, nil
}

// InviteToOrganization invites email to the org as an Owner or a Member
// (Render's Org Owners "Invite or remove other org members (including other
// Org Owners)"). bex has no SSO, so this and a workspace invite are how
// people enter an org (ADR115 D4). The invite redeems on the invitee's next
// sign-in with that verified email.
func (s *Service) InviteToOrganization(ctx context.Context, orgID, emailAddr, role string) (OrganizationInviteView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	// An invite is a redeemable membership that outlives the inviter's
	// credential, as a workspace invite is (round 24, H-1).
	if err := s.AuthorizeMintClass(ctx); err != nil {
		return OrganizationInviteView{}, err
	}
	object := core.OrganizationObject(orgID)
	if err := s.AuthorizeOn(ctx, core.RelCanManage, object); err != nil {
		return OrganizationInviteView{}, err
	}
	if err := s.AuthorizeFreshOn(ctx, core.RelCanManage, object); err != nil {
		return OrganizationInviteView{}, err
	}
	orgs, ok := s.organizations()
	if !ok {
		return OrganizationInviteView{}, ErrMembersUnavailable
	}
	role, err := canonicalOrgRole(role, core.OrgRoleOwner, core.OrgRoleMember)
	if err != nil {
		return OrganizationInviteView{}, err
	}
	addr, err := mail.ParseAddress(emailAddr)
	if err != nil {
		return OrganizationInviteView{}, fmt.Errorf("%w: invalid email address", core.ErrBadRequest)
	}
	emailAddr = strings.ToLower(addr.Address)
	o, err := s.requireOrgOwner(ctx, orgs, orgID)
	if err != nil {
		return OrganizationInviteView{}, err
	}
	if member, err := s.orgMemberWithEmail(ctx, orgs, orgID, emailAddr); err != nil {
		return OrganizationInviteView{}, err
	} else if member != nil {
		return OrganizationInviteView{}, core.NewConflictError(core.CodeOrganizationAlreadyMember,
			fmt.Sprintf("%s is already in this organization; change their org role instead", emailAddr),
			map[string]any{"email": emailAddr})
	}
	token, err := s.newInviteToken()
	if err != nil {
		return OrganizationInviteView{}, err
	}
	inviter, _ := core.IdentityFrom(ctx)
	inv, err := orgs.CreateOrganizationInvite(ctx, orgID, emailAddr, role, token, inviter.Subject, s.Now().Add(s.inviteTTL()))
	if err != nil {
		return OrganizationInviteView{}, mapStoreErr(err)
	}
	s.RecordOrganizationEvent(ctx, core.AuditVerbOrgInvited, orgID, core.OrganizationAudit{
		Target: core.InviteTarget(inv.ID), TargetName: inv.Email, RoleTo: inv.Role,
	})
	s.sendOrganizationInvite(ctx, inv, o)
	return organizationInviteView(inv), nil
}

// orgMemberWithEmail returns the org member whose email is emailAddr, nil
// when none (or no identity reader to tell).
func (s *Service) orgMemberWithEmail(ctx context.Context, orgs OrganizationStore, orgID, emailAddr string) (*store.OrganizationMember, error) {
	if s.Identities == nil {
		return nil, nil
	}
	members, err := orgs.ListOrganizationMembers(ctx, orgID)
	if err != nil {
		return nil, err
	}
	subjects := make([]string, 0, len(members))
	for _, m := range members {
		subjects = append(subjects, m.Subject)
	}
	subject := s.subjectWithEmail(ctx, subjects, emailAddr)
	for i := range members {
		if subject != "" && members[i].Subject == subject {
			return &members[i], nil
		}
	}
	return nil, nil
}

func (s *Service) sendOrganizationInvite(ctx context.Context, inv store.OrganizationInvite, o store.Organization) {
	if s.Mailer == nil {
		return
	}
	m := email.Message{
		Title: "Organization invitation",
		Paragraphs: []string{
			fmt.Sprintf("You've been invited to join the %q organization on bex as %s.", o.Name, strings.ToLower(wireRole(inv.Role))),
			fmt.Sprintf("Signing in to bex with %s joins it.", inv.Email),
		},
		Footer: []string{fmt.Sprintf("This invitation expires on %s.", inv.ExpiresAt.UTC().Format("2006-01-02 15:04 UTC"))},
	}
	if base := strings.TrimSuffix(s.InviteBaseURL, "/"); base != "" {
		m.CTA = &email.CTA{Lead: fmt.Sprintf("Sign up or log in with %s to accept", inv.Email), Label: "Open bex", URL: base}
	}
	if err := s.Mailer.Send(ctx, inv.Email, fmt.Sprintf("You've been invited to the %q organization on bex", o.Name), m.Text(), m.HTML()); err != nil {
		log.Printf("members: sending organization invite %s to %s: %v", inv.ID, inv.Email, err)
	}
}

// ListOrganizationInvites lists an org's pending invites. Owner only.
func (s *Service) ListOrganizationInvites(ctx context.Context, orgID string) ([]OrganizationInviteView, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.OrganizationObject(orgID)); err != nil {
		return nil, err
	}
	orgs, ok := s.organizations()
	if !ok {
		return nil, ErrMembersUnavailable
	}
	if _, err := s.requireOrgOwner(ctx, orgs, orgID); err != nil {
		return nil, err
	}
	invs, err := orgs.ListOrganizationInvites(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationInviteView, 0, len(invs))
	for _, inv := range invs {
		out = append(out, organizationInviteView(inv))
	}
	return out, nil
}

// RevokeOrganizationInvite revokes a pending org invite. Owner only.
func (s *Service) RevokeOrganizationInvite(ctx context.Context, orgID, inviteID string) error {
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.OrganizationObject(orgID)); err != nil {
		return err
	}
	orgs, ok := s.organizations()
	if !ok {
		return ErrMembersUnavailable
	}
	if _, err := s.requireOrgOwner(ctx, orgs, orgID); err != nil {
		return err
	}
	return mapStoreErr(orgs.DeleteOrganizationInvite(ctx, orgID, inviteID))
}

// ChangeOrganizationRole sets a person's org role: Owner, Member or Guest.
// Owner only, never their own (as a workspace role change). The last Owner
// cannot be demoted, and a Guest belongs to at most one org workspace.
func (s *Service) ChangeOrganizationRole(ctx context.Context, orgID, subject, role string) (OrganizationMemberView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	if err := s.AuthorizeMintClass(ctx); err != nil {
		return OrganizationMemberView{}, err
	}
	object := core.OrganizationObject(orgID)
	if err := s.AuthorizeOnTarget(ctx, core.RelCanManage, object, core.MemberTarget(subject)); err != nil {
		return OrganizationMemberView{}, err
	}
	if err := s.AuthorizeFreshOn(ctx, core.RelCanManage, object); err != nil {
		return OrganizationMemberView{}, err
	}
	if err := guardSelf(ctx, subject, ErrorCannotChangeOwnRole, "you cannot change your own org role; ask another Owner to change it"); err != nil {
		return OrganizationMemberView{}, err
	}
	orgs, ok := s.organizations()
	if !ok {
		return OrganizationMemberView{}, ErrMembersUnavailable
	}
	role, err := canonicalOrgRole(role, store.OrgRoles...)
	if err != nil {
		return OrganizationMemberView{}, err
	}
	if _, err := s.requireOrgOwner(ctx, orgs, orgID); err != nil {
		return OrganizationMemberView{}, err
	}
	m, err := orgs.GetOrganizationMember(ctx, orgID, subject)
	if err != nil {
		return OrganizationMemberView{}, mapStoreErr(err)
	}
	if m.Role != role {
		// Revoke first, as a workspace role change does (round-19 #3): a
		// failed grant leaves the person with less, never with both roles.
		if err := s.writeOrgTuples(ctx, nil, []core.Tuple{core.OrgRoleTuple(orgID, subject, m.Role)}); err != nil {
			return OrganizationMemberView{}, fmt.Errorf("organization %s: revoking %s's %s role: %w", orgID, subject, m.Role, err)
		}
		if _, err := orgs.SetOrganizationMemberRole(ctx, orgID, subject, role); err != nil {
			_ = s.writeOrgTuples(ctx, []core.Tuple{core.OrgRoleTuple(orgID, subject, m.Role)}, nil)
			return OrganizationMemberView{}, mapOrganizationErr(err, orgID, subject)
		}
		if err := s.writeOrgTuples(ctx, []core.Tuple{core.OrgRoleTuple(orgID, subject, role)}, nil); err != nil {
			return OrganizationMemberView{}, fmt.Errorf("organization %s: granting %s the %s role: %w", orgID, subject, role, err)
		}
		s.RecordOrganizationEvent(ctx, core.AuditVerbOrgRoleChanged, orgID, core.OrganizationAudit{
			Target: core.MemberTarget(subject), RoleFrom: m.Role, RoleTo: role,
		})
	}
	workspaces, err := orgs.SubjectOrganizationWorkspaces(ctx, orgID, subject)
	if err != nil {
		return OrganizationMemberView{}, err
	}
	return OrganizationMemberView{Subject: subject, Role: wireRole(role), CreatedAt: rfc3339(m.CreatedAt), Workspaces: append([]string{}, workspaces...)}, nil
}

// mapOrganizationErr names the store's org refusals.
func mapOrganizationErr(err error, orgID, subject string) error {
	switch {
	case errors.Is(err, store.ErrLastOwner):
		return core.NewConflictError(core.CodeOrganizationLastOwner,
			"an organization needs at least one Owner; make someone else an Owner first", map[string]any{"organizationId": orgID})
	case errors.Is(err, store.ErrGuestSingleWorkspace):
		return core.NewConflictError(core.CodeOrganizationGuestSingleWorkspace,
			"a Guest belongs to one workspace of the organization; make them a Member to give them more",
			map[string]any{"organizationId": orgID, "subject": subject})
	default:
		return mapStoreErr(err)
	}
}

// RemoveFromOrganization removes a person from the org and from every org
// workspace they belong to, through the same teardown a workspace removal
// runs (API keys, agent sessions, tuples). Owner only, never themselves. The
// last Owner cannot be removed, nor the last admin of an org workspace.
func (s *Service) RemoveFromOrganization(ctx context.Context, orgID, subject string) error {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	object := core.OrganizationObject(orgID)
	if err := s.AuthorizeOnTarget(ctx, core.RelCanManage, object, core.MemberTarget(subject)); err != nil {
		return err
	}
	if err := s.AuthorizeFreshOn(ctx, core.RelCanManage, object); err != nil {
		return err
	}
	if err := guardSelf(ctx, subject, ErrorCannotRemoveSelf, "you cannot remove yourself from an organization; ask another Owner"); err != nil {
		return err
	}
	orgs, ok := s.organizations()
	if !ok {
		return ErrMembersUnavailable
	}
	if _, err := s.requireOrgOwner(ctx, orgs, orgID); err != nil {
		return err
	}
	m, err := orgs.GetOrganizationMember(ctx, orgID, subject)
	if err != nil {
		return mapStoreErr(err)
	}
	if m.Role == core.OrgRoleOwner {
		owners := 0
		members, err := orgs.ListOrganizationMembers(ctx, orgID)
		if err != nil {
			return err
		}
		for _, om := range members {
			if om.Role == core.OrgRoleOwner {
				owners++
			}
		}
		if owners <= 1 {
			return mapOrganizationErr(store.ErrLastOwner, orgID, subject)
		}
	}
	workspaces, err := orgs.SubjectOrganizationWorkspaces(ctx, orgID, subject)
	if err != nil {
		return err
	}
	// Every refusal is decided before anything is torn down.
	memberships := make([]store.TenantMember, 0, len(workspaces))
	for _, ws := range workspaces {
		wm, err := s.Store.GetTenantMember(ctx, ws, subject)
		if err != nil {
			return mapStoreErr(err)
		}
		if err := s.guardLastAdmin(ctx, ws, wm.Role, ""); err != nil {
			return core.NewConflictError(core.CodeOrganizationMemberLastAdmin,
				"they are the last admin of an organization workspace; make someone else its admin first",
				map[string]any{"organizationId": orgID, "workspaceId": ws})
		}
		memberships = append(memberships, wm)
	}
	for _, wm := range memberships {
		revokedKeys, err := s.endMembership(ctx, wm.TenantID, subject, wm.Role)
		if err != nil {
			return err
		}
		s.RecordMemberRemoved(ctx, wm.TenantID, subject, revokedKeys)
	}
	if err := s.writeOrgTuples(ctx, nil, []core.Tuple{core.OrgRoleTuple(orgID, subject, m.Role)}); err != nil {
		return fmt.Errorf("organization %s: revoking %s's org role: %w", orgID, subject, err)
	}
	if err := orgs.RemoveOrganizationMember(ctx, orgID, subject); err != nil {
		_ = s.writeOrgTuples(ctx, []core.Tuple{core.OrgRoleTuple(orgID, subject, m.Role)}, nil)
		return mapOrganizationErr(err, orgID, subject)
	}
	s.RecordOrganizationEvent(ctx, core.AuditVerbOrgMemberRemoved, orgID, core.OrganizationAudit{Target: core.MemberTarget(subject), RoleFrom: m.Role})
	return nil
}

// JoinWorkspace adds the caller to a Public workspace of their org as a
// Developer (audit-logs, JoinTeamEvent: "Other org members can add themselves
// to public workspaces only (they receive the Developer role)").
func (s *Service) JoinWorkspace(ctx context.Context, workspaceID string) (JoinedWorkspaceView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanJoin, workspaceID)
	if err != nil {
		return JoinedWorkspaceView{}, err
	}
	return s.join(ctx, workspaceID, "developer", core.AuditVerbJoinWorkspace, func(t store.Tenant, role string) bool {
		return t.Privacy == core.PrivacyPublic && (role == core.OrgRoleMember || role == core.OrgRoleOwner)
	})
}

// JoinWorkspaceAsAdmin adds the caller, an Owner of the workspace's org, to it
// as admin ("Add themselves to any workspace as an admin"). An Owner already
// in the workspace at another role becomes its admin.
func (s *Service) JoinWorkspaceAsAdmin(ctx context.Context, workspaceID string) (JoinedWorkspaceView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanJoinAsAdmin, workspaceID)
	if err != nil {
		return JoinedWorkspaceView{}, err
	}
	return s.join(ctx, workspaceID, "admin", core.AuditVerbJoinWorkspaceAsAdmin, func(_ store.Tenant, role string) bool {
		return role == core.OrgRoleOwner
	})
}

func (s *Service) join(ctx context.Context, workspaceID, role, verb string, allowed func(store.Tenant, string) bool) (JoinedWorkspaceView, error) {
	orgs, ok := s.organizations()
	if !ok {
		return JoinedWorkspaceView{}, ErrMembersUnavailable
	}
	t, err := s.Store.GetTenant(ctx, workspaceID)
	if err != nil {
		return JoinedWorkspaceView{}, mapStoreErr(err)
	}
	if t.OrganizationID == "" {
		return JoinedWorkspaceView{}, core.NewOrganizationWorkspaceRequiredError(workspaceID)
	}
	_, orgRole, err := s.callerOrgRole(ctx, orgs, t.OrganizationID)
	if err != nil {
		return JoinedWorkspaceView{}, err
	}
	if !allowed(t, orgRole) {
		return JoinedWorkspaceView{}, core.ErrForbidden
	}
	id, _ := core.IdentityFrom(ctx)
	view := JoinedWorkspaceView{WorkspaceID: workspaceID, Role: wireRole(role)}
	current, err := s.Store.GetTenantMember(ctx, workspaceID, id.Subject)
	switch {
	case err == nil && (current.Role == role || role != "admin"):
		view.Role = wireRole(current.Role)
		return view, nil
	case err == nil:
		// An Owner already in the workspace takes its admin role; the
		// reconcile below converges OpenFGA onto exactly that role.
		if err := s.Store.UpdateMemberRole(ctx, workspaceID, id.Subject, role); err != nil {
			return JoinedWorkspaceView{}, mapStoreErr(err)
		}
	case errors.Is(err, store.ErrNotFound):
		if _, err := orgs.JoinOrganizationWorkspace(ctx, workspaceID, id.Subject, role); err != nil {
			return JoinedWorkspaceView{}, mapStoreErr(err)
		}
	default:
		return JoinedWorkspaceView{}, err
	}
	if _, err := s.reconcileQueuedRole(ctx, workspaceID, id.Subject, role); err != nil {
		log.Printf("members: reconciling %s's %s role on %s: %v", id.Subject, role, workspaceID, err)
	}
	s.invalidateResolution(id.Subject, workspaceID)
	view.Joined = true
	s.RecordOrganizationEvent(ctx, verb, t.OrganizationID, core.OrganizationAudit{
		Target: core.WorkspaceObject(workspaceID), TargetName: t.Label(), RoleTo: role,
	})
	return view, nil
}

// guardOrganizationBilling refuses a third person with the Billing role in an
// org's workspaces: "Each org supports up to two total members with the
// Billing role" (render.com/docs/organizations). Pending Billing invites count
// as people. subject or emailAddr names who is getting the role; someone who
// already holds it anywhere in the org takes no new place.
func (s *Service) guardOrganizationBilling(ctx context.Context, t store.Tenant, subject, emailAddr string) error {
	orgs, ok := s.organizations()
	if !ok || t.OrganizationID == "" {
		return nil
	}
	subjects, inviteEmails, err := orgs.OrganizationBillingHolders(ctx, t.OrganizationID)
	if err != nil {
		return err
	}
	if subject != "" && slices.Contains(subjects, subject) {
		return nil
	}
	people := map[string]bool{}
	for _, e := range inviteEmails {
		people["email:"+strings.ToLower(e)] = true
	}
	var emails map[string]string
	if s.Identities != nil {
		lookup := subjects
		if subject != "" {
			lookup = append(slices.Clone(subjects), subject)
		}
		emails, _ = s.Identities.LookupEmails(ctx, lookup)
	}
	for _, sub := range subjects {
		if e := emails[sub]; e != "" {
			people["email:"+strings.ToLower(e)] = true
		} else {
			people["subject:"+sub] = true
		}
	}
	if subject != "" && emailAddr == "" {
		emailAddr = emails[subject]
	}
	if emailAddr != "" && people["email:"+strings.ToLower(emailAddr)] {
		return nil
	}
	if len(people) >= core.OrganizationBillingLimit {
		return core.NewConflictError(core.CodeOrganizationBillingLimit,
			fmt.Sprintf("an organization supports up to %d members with the Billing role", core.OrganizationBillingLimit),
			map[string]any{"limit": core.OrganizationBillingLimit, "organizationId": t.OrganizationID})
	}
	return nil
}

// guardGuestInvite refuses a workspace invite to a Guest of the workspace's
// org who already belongs to another of its workspaces (ADR115 D4).
func (s *Service) guardGuestInvite(ctx context.Context, t store.Tenant, emailAddr string) error {
	orgs, ok := s.organizations()
	if !ok || t.OrganizationID == "" || s.Identities == nil {
		return nil
	}
	m, err := s.orgMemberWithEmail(ctx, orgs, t.OrganizationID, emailAddr)
	if err != nil || m == nil || m.Role != core.OrgRoleGuest {
		return err
	}
	workspaces, err := orgs.SubjectOrganizationWorkspaces(ctx, t.OrganizationID, m.Subject)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(workspaces, func(ws string) bool { return ws != t.ID }) {
		return mapOrganizationErr(store.ErrGuestSingleWorkspace, t.OrganizationID, m.Subject)
	}
	return nil
}

// joinedOrganization finishes a workspace invite that made the caller a new
// Member of the workspace's org.
func (s *Service) joinedOrganization(ctx context.Context, inv store.Invite, id core.Identity) {
	if inv.JoinedOrganizationID == "" || s.Base == nil {
		return
	}
	core.JoinedOrganization(ctx, s.OrgTuples, s.Base.Audit, s.Now(), inv.JoinedOrganizationID, id.Subject, id.Method)
}
