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

// Package members is the workspace team-membership feature (w4/m12): invite a
// teammate by email, list and change roles, remove members — Render's workspace
// Members settings. It is the one place three things move together for a
// membership change: the `tenant_members` row (the source of truth), the
// workspace:<id> OpenFGA tuple (what the auth gate enforces), and — for an
// invite — a pending `tenant_invites` row plus its email. Every verb targets an
// EXPLICIT workspace id (like the workspaces lifecycle verbs) and gates on that
// workspace, not the caller's own: managing members is an admin acting ON a
// workspace it administers. The rest/graphql/mcp files are thin fragments over
// this Service; Render's role enum is UPPERCASE on the wire, lowercase as the
// FGA relation / stored role (docs/ADR012-auth.md), converted at the view boundary.
package members

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Roles is Render's workspace role ladder — the five FGA relations a member may
// hold (docs/ADR012-auth.md, deploy/gitops/authz/model.fga). Stored lowercase in
// tenant_members.role and written as the OpenFGA relation verbatim; surfaced
// UPPERCASE on the wire (Render's enum).
var Roles = []string{"viewer", "contributor", "developer", "admin", "billing"}

// DefaultInviteTTL is how long an invite stays redeemable — Render's 7 days
// (docs/render-artifacts/team-members.graphql: "pendingInvites expire in 7 days").
const DefaultInviteTTL = 7 * 24 * time.Hour

// ErrMembersUnavailable is returned when the control-plane store isn't wired
// (bex-api without BEX_CP_DB_URI); adapters surface it as 503.
var ErrMembersUnavailable = errors.New("workspace members store not configured")

// Service holds the membership logic once. It embeds *core.Base for the
// authorization gate + caller Identity and writes through the Postgres source of
// truth (Store) with OpenFGA membership kept in lockstep (Granter/Revoker). The
// Mailer delivers invites (nil => invites are still recorded, just not mailed —
// a self-hoster without SMTP, or the authz sweep).
type Service struct {
	*core.Base
	// Store is the control-plane source of truth. Nil (DB-less mode / the authz
	// sweep) leaves verbs answering ErrMembersUnavailable AFTER the Authorize gate.
	Store MembersStore
	// Granter/Revoker keep OpenFGA membership in step with tenant_members: a role
	// tuple written on invite-accept / role change, removed on remove / downgrade.
	// Both nil when authz is disabled — the row is still the source of truth.
	Granter RoleGranter
	Revoker RoleRevoker
	// Mailer delivers the invite email. Nil => the invite row is written but no
	// mail is sent (the recipient can still be told out-of-band, and the invite
	// redeems on their next login by email match).
	Mailer Mailer
	// InviteTTL overrides DefaultInviteTTL (tests). Zero => DefaultInviteTTL.
	InviteTTL time.Duration
	// InviteBaseURL is the dashboard origin the invite email links to (e.g.
	// https://dashboard.bex.co). Empty => the email carries instructions without a
	// deep link (acceptance is by email match regardless).
	InviteBaseURL string
	// Identities resolves a member's email from the identity provider — the same
	// enrichment the owners API performs (w6/m2). Nil (BEX_KRATOS_ADMIN_URL
	// unset) => Email omitted (honest subset); List still succeeds.
	Identities EmailLookup
}

// MembersStore is the slice of the source of truth this feature writes through —
// narrow, like workspaces.WorkspaceStore. *store.PGStore satisfies it.
type MembersStore interface {
	GetTenant(ctx context.Context, id string) (store.Tenant, error)
	ListTenantMembers(ctx context.Context, tenantID string) ([]store.TenantMember, error)
	GetTenantMember(ctx context.Context, tenantID, subject string) (store.TenantMember, error)
	CountTenantMembers(ctx context.Context, tenantID string) (int, error)
	CountTenantAdmins(ctx context.Context, tenantID string) (int, error)
	CountInvites(ctx context.Context, tenantID string) (int, error)
	UpdateMemberRole(ctx context.Context, tenantID, subject, role string) error
	RemoveMember(ctx context.Context, tenantID, subject string) error
	CreateInvite(ctx context.Context, tenantID, email, role, token, invitedBy string, expiresAt time.Time) (store.Invite, error)
	ListInvites(ctx context.Context, tenantID string) ([]store.Invite, error)
	GetInvite(ctx context.Context, tenantID, id string) (store.Invite, error)
	DeleteInvite(ctx context.Context, tenantID, id string) error
	// OwnerIDForSubject resolves a subject's stable opaque "own-" id, minting one
	// on first sight — the same identity enrichment the owners API performs
	// (workspaces.WorkspaceStore, w6/m7). *store.PGStore already satisfies it.
	OwnerIDForSubject(ctx context.Context, subject string) (string, error)
}

// EmailLookup resolves a subject's email from the identity provider (Kratos
// admin API) — the members-package seam for the same enrichment
// workspaces.IdentityReader performs for the owners API (named return type
// differs across packages, so this feature keeps its own narrow interface;
// the composition root adapts workspaces.IdentityReader to it). Nil
// (BEX_KRATOS_ADMIN_URL unset) or a lookup miss => Email omitted (honest
// subset) — List still succeeds.
type EmailLookup interface {
	LookupEmail(ctx context.Context, subject string) (string, bool)
}

// RoleGranter writes a member's OpenFGA role tuple on a workspace (the authz
// write side of a role change / invite accept). *authz.openfgaChecker satisfies
// it structurally, so this package keeps no dependency on the authz client.
type RoleGranter interface {
	GrantWorkspaceRole(ctx context.Context, tenantID, subject, relation string) error
}

// RoleRevoker removes a member's OpenFGA role tuple (the authz side of remove /
// downgrade). Same signature the workspaces feature revokes through.
type RoleRevoker interface {
	RevokeWorkspaceMember(ctx context.Context, tenantID, subject, relation string) error
}

// Mailer delivers a plain-text email — the seam over SMTP so this feature stays
// transport-free and testable. Injected by the composition root.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// MemberView is the neutral projection of an accepted member. Subject is the
// OpenFGA user (Kratos identity id or Hydra client id) — kept as the mutation
// key (ChangeRole/Remove take `subject`; a bex-native contract, not Render's
// `userId` surface, docs/render-artifacts/owners-api.md). UserID is the same
// opaque "own-" id the owners API reports as `userId` (w6/m7); Email is
// resolved via Identities, same as the owners API — both honest-omit
// (""/unset) when the identity reader is nil or a lookup misses (w6/m10).
// Role is UPPERCASE (Render enum).
type MemberView struct {
	Subject   string `json:"subject"`
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
}

// InviteView is the neutral projection of a pending invite — Render's
// pendingInvites shape (id, email, role, expiresAt), plus createdAt.
type InviteView struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expiresAt"`
	CreatedAt string `json:"createdAt"`
}

func memberView(m store.TenantMember) MemberView {
	return MemberView{Subject: m.Subject, Role: wireRole(m.Role), CreatedAt: rfc3339(m.CreatedAt)}
}

func inviteView(inv store.Invite) InviteView {
	return InviteView{
		ID: inv.ID, Email: inv.Email, Role: wireRole(inv.Role),
		ExpiresAt: rfc3339(inv.ExpiresAt), CreatedAt: rfc3339(inv.CreatedAt),
	}
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// canonicalRole lowercases + validates a caller-supplied role against Render's
// ladder; ok=false is a bad request. Case-insensitive so both the wire's
// UPPERCASE enum and a lowercase relation are accepted.
func canonicalRole(s string) (string, bool) {
	r := strings.ToLower(strings.TrimSpace(s))
	for _, valid := range Roles {
		if r == valid {
			return r, true
		}
	}
	return "", false
}

// wireRole renders a stored role as Render's UPPERCASE enum.
func wireRole(role string) string { return strings.ToUpper(role) }

// List returns a workspace's accepted members. Viewer-and-up (can_view on the
// named workspace) — Render shows the members list to every role.
func (s *Service) List(ctx context.Context, workspaceID string) ([]MemberView, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanView, core.WorkspaceObject(workspaceID)); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, ErrMembersUnavailable
	}
	ms, err := s.Store.ListTenantMembers(ctx, workspaceID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	out := make([]MemberView, 0, len(ms))
	for _, m := range ms {
		mv := memberView(m)
		// Resolve the opaque own- id (minted on first sight) — the same
		// enrichment workspaces.Service.ListMembers performs for the owners API.
		// A store error surfaces to the caller (5xx) rather than a silent blank.
		ownID, err := s.Store.OwnerIDForSubject(ctx, m.Subject)
		if err != nil {
			return nil, err
		}
		mv.UserID = ownID
		if s.Identities != nil {
			if email, ok := s.Identities.LookupEmail(ctx, m.Subject); ok {
				mv.Email = email
			}
		}
		out = append(out, mv)
	}
	return out, nil
}

// ListInvites returns a workspace's outstanding invites. Admin-only (managing
// members) — the pending list can reveal who was invited, an org-settings view.
func (s *Service) ListInvites(ctx context.Context, workspaceID string) ([]InviteView, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID)); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, ErrMembersUnavailable
	}
	invs, err := s.Store.ListInvites(ctx, workspaceID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	out := make([]InviteView, 0, len(invs))
	for _, inv := range invs {
		out = append(out, inviteView(inv))
	}
	return out, nil
}

// Invite records a pending invite to email at role and emails it. Admin-only
// (can_manage). Refuses the plan's per-workspace member cap (Hobby is
// single-member) counting accepted members + outstanding invites, and a bad
// email/role. The invite redeems on the recipient's first login by email match
// (internal/api/tenancy.go); the token backs a direct-accept link.
func (s *Service) Invite(ctx context.Context, workspaceID, email, role string) (InviteView, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID)); err != nil {
		return InviteView{}, err
	}
	if s.Store == nil {
		return InviteView{}, ErrMembersUnavailable
	}
	role, ok := canonicalRole(role)
	if !ok {
		return InviteView{}, fmt.Errorf("%w: role must be one of %s", core.ErrBadRequest, strings.Join(Roles, "|"))
	}
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return InviteView{}, fmt.Errorf("%w: invalid email address", core.ErrBadRequest)
	}
	email = strings.ToLower(addr.Address)

	tenant, err := s.Store.GetTenant(ctx, workspaceID)
	if err != nil {
		return InviteView{}, mapStoreErr(err)
	}
	if err := guardPlanRole(tenant.Plan, role); err != nil {
		return InviteView{}, err
	}
	// Seat cap: accepted members + outstanding invites both consume a seat, so a
	// single-member (Hobby) workspace can't invite a second person until upgraded.
	members, err := s.Store.CountTenantMembers(ctx, workspaceID)
	if err != nil {
		return InviteView{}, err
	}
	pending, err := s.Store.CountInvites(ctx, workspaceID)
	if err != nil {
		return InviteView{}, err
	}
	if !store.CanAddMember(tenant.Plan, members+pending) {
		lim := store.LimitsFor(tenant.Plan)
		return InviteView{}, core.NewPlanLimitError(
			fmt.Sprintf("the %s plan is limited to %d workspace member(s); upgrade to invite more", tenant.Plan, lim.MaxMembers),
			tenant.Plan, lim.MaxMembers,
		)
	}

	inviter, _ := core.IdentityFrom(ctx)
	inv, err := s.Store.CreateInvite(ctx, workspaceID, email, role, newToken(), inviter.Subject, s.Now().Add(s.inviteTTL()))
	if err != nil {
		return InviteView{}, mapStoreErr(err)
	}
	// Best-effort delivery: a failed send does NOT unwind the invite (it's
	// recoverable — an admin can resend, and acceptance is by email match, not the
	// mail). Surfaced only in logs, not to the caller, so a flaky relay doesn't
	// look like a failed invite.
	s.sendInvite(ctx, inv, tenant)
	return inviteView(inv), nil
}

// ChangeRole changes an existing member's role. Admin-only. Refuses demoting the
// last admin (a workspace nobody can administer). No-op when the role is
// unchanged. Writes the row (source of truth) then reconciles OpenFGA: grants the
// new relation, revokes the old — so the auth gate follows the row.
func (s *Service) ChangeRole(ctx context.Context, workspaceID, subject, role string) (MemberView, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID)); err != nil {
		return MemberView{}, err
	}
	if s.Store == nil {
		return MemberView{}, ErrMembersUnavailable
	}
	role, ok := canonicalRole(role)
	if !ok {
		return MemberView{}, fmt.Errorf("%w: role must be one of %s", core.ErrBadRequest, strings.Join(Roles, "|"))
	}
	m, err := s.Store.GetTenantMember(ctx, workspaceID, subject)
	if err != nil {
		return MemberView{}, mapStoreErr(err)
	}
	if m.Role == role {
		return memberView(m), nil // already there — nothing to write
	}
	tenant, err := s.Store.GetTenant(ctx, workspaceID)
	if err != nil {
		return MemberView{}, mapStoreErr(err)
	}
	if err := guardPlanRole(tenant.Plan, role); err != nil {
		return MemberView{}, err
	}
	if err := s.guardLastAdmin(ctx, workspaceID, m.Role, role); err != nil {
		return MemberView{}, err
	}
	if err := s.Store.UpdateMemberRole(ctx, workspaceID, subject, role); err != nil {
		return MemberView{}, mapStoreErr(err)
	}
	// Reconcile tuples: grant the new relation (idempotent via grantRole's
	// check-before-write), then remove the old. A role UPGRADE takes effect
	// immediately (a fresh positive check on the new relation isn't cached); a
	// DOWNGRADE is subject to the auth gate's ≤30s positive-check TTL on the old
	// relation, an acceptable staleness for a revoke.
	if err := s.grantRole(ctx, workspaceID, "user:"+subject, role); err != nil {
		return MemberView{}, fmt.Errorf("workspace %s: granting role failed: %w", workspaceID, err)
	}
	s.revokeRole(ctx, workspaceID, "user:"+subject, m.Role)
	m.Role = role
	return memberView(m), nil
}

// Remove drops a member from a workspace: the row then its OpenFGA tuple.
// Admin-only. Refuses removing the last admin. The revoke is best-effort (an
// already-gone tuple is not an error), so a retried remove completes.
func (s *Service) Remove(ctx context.Context, workspaceID, subject string) error {
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID)); err != nil {
		return err
	}
	if s.Store == nil {
		return ErrMembersUnavailable
	}
	m, err := s.Store.GetTenantMember(ctx, workspaceID, subject)
	if err != nil {
		return mapStoreErr(err)
	}
	if err := s.guardLastAdmin(ctx, workspaceID, m.Role, ""); err != nil {
		return err
	}
	if err := s.Store.RemoveMember(ctx, workspaceID, subject); err != nil {
		return mapStoreErr(err)
	}
	s.revokeRole(ctx, workspaceID, "user:"+subject, m.Role)
	return nil
}

// RevokeInvite deletes a pending invite before it's redeemed. Admin-only.
func (s *Service) RevokeInvite(ctx context.Context, workspaceID, inviteID string) error {
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID)); err != nil {
		return err
	}
	if s.Store == nil {
		return ErrMembersUnavailable
	}
	if err := s.Store.DeleteInvite(ctx, workspaceID, inviteID); err != nil {
		return mapStoreErr(err)
	}
	return nil
}

// guardPlanRole refuses a role Render doesn't offer on the workspace's plan
// (RESEARCH-workspaces.md finding 5: Pro is Admin+Developer only; Contributor,
// Viewer, and Billing are Scale+) — the same rule ChangePlan's downgrade guard
// enforces in the other direction (internal/workspaces/service.go).
func guardPlanRole(plan, role string) error {
	if store.RoleAllowedOnPlan(plan, role) {
		return nil
	}
	lim := store.LimitsFor(plan)
	return core.NewPlanLimitError(
		fmt.Sprintf("the %s plan only allows roles %s", plan, strings.Join(lim.AllowedRoles, "|")),
		plan, 0,
	)
}

// guardLastAdmin refuses a change that would leave a workspace with zero admins:
// demoting (newRole != "admin") or removing (newRole == "") its only admin. A
// change that keeps or adds an admin is always allowed.
func (s *Service) guardLastAdmin(ctx context.Context, workspaceID, currentRole, newRole string) error {
	if currentRole != "admin" || newRole == "admin" {
		return nil
	}
	admins, err := s.Store.CountTenantAdmins(ctx, workspaceID)
	if err != nil {
		return err
	}
	if admins <= 1 {
		return fmt.Errorf("%w: cannot remove or demote the last admin of a workspace", core.ErrBadRequest)
	}
	return nil
}

// grantRole writes a member's role tuple, skipping the write when it already
// exists (OpenFGA errors on a duplicate) — the same check-before-grant that
// keeps tenancy.ensureGranted idempotent, so a retried role change converges.
func (s *Service) grantRole(ctx context.Context, tenantID, subject, relation string) error {
	if s.Granter == nil {
		return nil
	}
	if s.Base != nil && s.Base.Authz != nil {
		if ok, err := s.Base.Authz.Check(ctx, subject, relation, core.WorkspaceObject(tenantID)); err == nil && ok {
			return nil
		}
	}
	return s.Granter.GrantWorkspaceRole(ctx, tenantID, subject, relation)
}

// revokeRole removes a member's role tuple, best-effort — an absent tuple is not
// an error to OpenFGA, so this is safe to call on a role that may already be gone.
func (s *Service) revokeRole(ctx context.Context, tenantID, subject, relation string) {
	if s.Revoker == nil {
		return
	}
	_ = s.Revoker.RevokeWorkspaceMember(ctx, tenantID, subject, relation)
}

func (s *Service) inviteTTL() time.Duration {
	if s.InviteTTL > 0 {
		return s.InviteTTL
	}
	return DefaultInviteTTL
}

// newToken mints an unguessable invite token (128 bits, hex). rand.Read from
// crypto/rand never fails on the platforms bex runs; a read error degrades to an
// empty token rather than panicking (acceptance is by email match regardless).
func newToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// mapStoreErr translates the store's error taxonomy into the kernel sentinels the
// adapters map to status codes — the same mapping the workspaces feature uses.
func mapStoreErr(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return fmt.Errorf("%w: %v", core.ErrNotFound, err)
	case errors.Is(err, store.ErrConflict), errors.Is(err, store.ErrInvalid):
		return fmt.Errorf("%w: %v", core.ErrBadRequest, err)
	default:
		return err
	}
}
