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
	"log"
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

// Stable GraphQL extensions.code values for direct invite redemption. Mobile
// uses these to clear terminal bearer capabilities while retaining only
// retryable transport failures; human error strings are not an API.
const (
	InviteErrorInvalid         = "INVITE_INVALID"
	InviteErrorExpired         = "INVITE_EXPIRED"
	InviteErrorAlreadyAccepted = "INVITE_ALREADY_ACCEPTED"
	InviteErrorPlanLimit       = "INVITE_PLAN_LIMIT"
)

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
	// Identities resolves a member's email + MFA state from the identity
	// provider — the same enrichment the owners API performs (w6/m2, w1/m33).
	// Nil (BEX_KRATOS_ADMIN_URL unset) => Email/MFAEnabled omitted (honest
	// subset); List still succeeds.
	Identities IdentityLookup
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
	// RefreshInvite pushes an unaccepted invite's expiry forward and installs a
	// freshly minted token (the resend verb, w1/m33; token rotates since w1/041 —
	// only its hash is at rest, so the old plaintext cannot be re-emailed);
	// accepted/unknown is ErrNotFound.
	RefreshInvite(ctx context.Context, tenantID, id, token string, expiresAt time.Time) (store.Invite, error)
	// AcceptInviteByToken redeems one invite by its emailed token for subject —
	// the direct-accept path (w1/m33); plan/seat guards run at redemption
	// exactly like the login-time email match.
	AcceptInviteByToken(ctx context.Context, token, subject string) (store.Invite, error)
	// OwnerIDForSubject resolves a subject's stable opaque "own-" id, minting one
	// on first sight — the same identity enrichment the owners API performs
	// (workspaces.WorkspaceStore, w6/m7). *store.PGStore already satisfies it.
	OwnerIDForSubject(ctx context.Context, subject string) (string, error)
}

// IdentityAttrs is the slice of identity-provider state a member row is
// enriched with: the email (w6/m10) plus the MFA-enrollment flag (w1/m33 —
// Render's Team Members query carries user.otpEnabled; bex spells it
// mfaEnabled, matching its own owners read API).
type IdentityAttrs struct {
	Email      string
	MFAEnabled bool
}

// IdentityLookup resolves a subject's identity attributes from the identity
// provider (Kratos admin API) — the members-package seam for the same
// enrichment workspaces.IdentityReader performs for the owners API (named
// return type differs across packages, so this feature keeps its own narrow
// interface; the composition root adapts workspaces.IdentityReader to it).
// Nil (BEX_KRATOS_ADMIN_URL unset) or a lookup miss => Email/MFAEnabled
// omitted (honest subset) — List still succeeds.
type IdentityLookup interface {
	LookupIdentity(ctx context.Context, subject string) (IdentityAttrs, bool)
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

// Mailer delivers an email with a plain-text body and an optional HTML
// alternative — the seam over SMTP so this feature stays transport-free and
// testable. Injected by the composition root; mailer.SMTP satisfies it.
type Mailer interface {
	Send(ctx context.Context, to, subject, text, html string) error
}

// MemberView is the neutral projection of an accepted member. Subject is the
// OpenFGA user (Kratos identity id or Hydra client id) — kept as the mutation
// key (ChangeRole/Remove take `subject`; a bex-native contract, not Render's
// `userId` surface, docs/render-artifacts/owners-api.md). UserID is the same
// opaque "own-" id the owners API reports as `userId` (w6/m7); Email is
// resolved via Identities, same as the owners API — both honest-omit
// (""/unset) when the identity reader is nil or a lookup misses (w6/m10).
// Role is UPPERCASE (Render enum). MFAEnabled mirrors Render's per-member
// otpEnabled under bex's own owners-API spelling — honest-false when the
// identity reader is nil or the lookup misses, like Email.
type MemberView struct {
	Subject    string `json:"subject"`
	UserID     string `json:"userId"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	CreatedAt  string `json:"createdAt"`
	MFAEnabled bool   `json:"mfaEnabled"`
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

// SeatUsageView is the workspace's seat consumption — Render's
// owner.usage.users { used, limit } (docs/render-artifacts/team-members.graphql).
// Used counts accepted members PLUS outstanding invites, the same formula the
// invite verb's cap enforces (store.CanAddMember), so what the UI displays and
// what the guard refuses can never disagree. Limit 0 means unlimited (the
// paid plans), mirroring store.PlanLimits.MaxMembers.
type SeatUsageView struct {
	Used  int `json:"used"`
	Limit int `json:"limit"`
}

// AcceptedInviteView is what redeeming an invite token returns: the workspace
// joined, its display name (for the dashboard's toast), and the role granted.
type AcceptedInviteView struct {
	WorkspaceID   string `json:"workspaceId"`
	WorkspaceName string `json:"workspaceName"`
	Role          string `json:"role"`
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
			if attrs, ok := s.Identities.LookupIdentity(ctx, m.Subject); ok {
				mv.Email = attrs.Email
				mv.MFAEnabled = attrs.MFAEnabled
			}
		}
		out = append(out, mv)
	}
	return out, nil
}

// SeatUsage returns the workspace's seat consumption (used/limit) —
// viewer-and-up like List: Render shows the seat bar to every role, and the
// numbers reveal nothing the members list doesn't.
func (s *Service) SeatUsage(ctx context.Context, workspaceID string) (SeatUsageView, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanView, core.WorkspaceObject(workspaceID)); err != nil {
		return SeatUsageView{}, err
	}
	if s.Store == nil {
		return SeatUsageView{}, ErrMembersUnavailable
	}
	tenant, err := s.Store.GetTenant(ctx, workspaceID)
	if err != nil {
		return SeatUsageView{}, mapStoreErr(err)
	}
	used, err := s.seatsUsed(ctx, workspaceID)
	if err != nil {
		return SeatUsageView{}, err
	}
	return SeatUsageView{Used: used, Limit: store.LimitsFor(tenant.Plan).MaxMembers}, nil
}

// seatsUsed is the ONE seat-consumption formula — accepted members plus
// outstanding invites — shared by the Invite verb's cap guard and the
// SeatUsage read, so what the UI displays and what the guard refuses are
// structurally the same number.
func (s *Service) seatsUsed(ctx context.Context, workspaceID string) (int, error) {
	members, err := s.Store.CountTenantMembers(ctx, workspaceID)
	if err != nil {
		return 0, err
	}
	pending, err := s.Store.CountInvites(ctx, workspaceID)
	if err != nil {
		return 0, err
	}
	return members + pending, nil
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
	// The allowed-write audit row is deferred to after the invite row exists —
	// its target (the invite id) is minted by the store, so recording at
	// authorize time could carry no target at all. Denials record immediately.
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
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
	used, err := s.seatsUsed(ctx, workspaceID)
	if err != nil {
		return InviteView{}, err
	}
	if !store.CanAddMember(tenant.Plan, used) {
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
	s.RecordMemberInvited(ctx, workspaceID, inv.ID, inv.Email, inv.Role)
	// Best-effort delivery: a failed send does NOT unwind the invite (it's
	// recoverable — an admin can resend, and acceptance is by email match, not the
	// mail). Surfaced only in logs, not to the caller, so a flaky relay doesn't
	// look like a failed invite.
	s.sendInvite(ctx, inv, tenant)
	return inviteView(inv), nil
}

// ResendInvite re-delivers a pending invite's email and pushes its expiry
// forward — how an admin recovers a lost or lapsed invite without revoke +
// re-invite (which would churn the invite id). Admin-only. The token is
// re-minted (w1/041: only its hash is at rest, so the original plaintext
// cannot be reproduced for the new mail) — the freshly emailed link
// supersedes the original, which stops redeeming. An accepted or unknown
// invite is a 404 on every surface.
func (s *Service) ResendInvite(ctx context.Context, workspaceID, inviteID string) (InviteView, error) {
	if err := s.AuthorizeOnTarget(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID), core.InviteTarget(inviteID)); err != nil {
		return InviteView{}, err
	}
	if s.Store == nil {
		return InviteView{}, ErrMembersUnavailable
	}
	// Refresh first: the tenant row is only needed for the email body, so a 404
	// invite (accepted/unknown/cross-workspace) doesn't pay a wasted read.
	inv, err := s.Store.RefreshInvite(ctx, workspaceID, inviteID, newToken(), s.Now().Add(s.inviteTTL()))
	if err != nil {
		return InviteView{}, mapStoreErr(err)
	}
	tenant, err := s.Store.GetTenant(ctx, workspaceID)
	if err != nil {
		return InviteView{}, mapStoreErr(err)
	}
	s.sendInvite(ctx, inv, tenant)
	return inviteView(inv), nil
}

// AcceptInvite redeems an invite by its emailed token for the CALLER — the
// direct-accept path that works when the recipient signed up under a different
// email than the one invited (the login-time email match can never redeem
// those). The token is the capability; the gate here is only "an authenticated,
// onboarded caller" (can_view on their own workspace — every fresh signup's
// personal workspace grants it), not membership of the joined workspace, which
// is exactly what's being established. Plan/seat guards run at redemption in
// the store, same as the login path.
func (s *Service) AcceptInvite(ctx context.Context, token string) (AcceptedInviteView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return AcceptedInviteView{}, err
	}
	if s.Store == nil {
		return AcceptedInviteView{}, ErrMembersUnavailable
	}
	if strings.TrimSpace(token) == "" {
		return AcceptedInviteView{}, core.NewBadRequestError(InviteErrorInvalid, "missing invite token", nil)
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok {
		return AcceptedInviteView{}, core.ErrForbidden
	}
	inv, err := s.Store.AcceptInviteByToken(ctx, token, id.Subject)
	if err != nil {
		return AcceptedInviteView{}, mapAcceptInviteErr(err)
	}
	if err := s.grantRole(ctx, inv.TenantID, "user:"+id.Subject, inv.Role); err != nil {
		// Row is the source of truth; the tuple is re-driven on the next login
		// (tenancy's ensureGranted path), same best-effort model as the email path.
		log.Printf("members: granting %s on workspace %s to %s: %v", inv.Role, inv.TenantID, id.Subject, err)
	}
	s.recordAccepted(ctx, inv, id)
	view := AcceptedInviteView{WorkspaceID: inv.TenantID, Role: wireRole(inv.Role)}
	if tenant, err := s.Store.GetTenant(ctx, inv.TenantID); err == nil {
		view.WorkspaceName = tenant.Name
	}
	return view, nil
}

func mapAcceptInviteErr(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return core.NewNotFoundError(InviteErrorInvalid, "invite token is invalid", nil)
	case errors.Is(err, store.ErrInviteAlreadyAccepted):
		return core.NewBadRequestError(InviteErrorAlreadyAccepted, "invite has already been accepted", nil)
	case errors.Is(err, store.ErrInviteExpired):
		return core.NewBadRequestError(InviteErrorExpired, "invite has expired", nil)
	case errors.Is(err, store.ErrInvitePlanLimit):
		return core.NewBadRequestError(InviteErrorPlanLimit, "the workspace plan cannot seat this invite", nil)
	default:
		return mapStoreErr(err)
	}
}

// recordAccepted writes the members.AcceptInvite audit row for a token
// redemption — the caller is the accepting identity, mirroring what the login
// path records (internal/api/tenancy.go).
func (s *Service) recordAccepted(ctx context.Context, inv store.Invite, id core.Identity) {
	if s.Base == nil {
		return
	}
	core.RecordInviteAccepted(ctx, s.Base.Audit, s.Now(),
		inv.TenantID, inv.ID, inv.Email, inv.Role, id.Subject, id.Method)
}

// ChangeRole changes an existing member's role. Admin-only. Refuses demoting the
// last admin (a workspace nobody can administer). No-op when the role is
// unchanged. Writes the row (source of truth) then reconciles OpenFGA: grants the
// new relation, revokes the old — so the auth gate follows the row.
func (s *Service) ChangeRole(ctx context.Context, workspaceID, subject, role string) (MemberView, error) {
	// Deferred allowed-write recording: the success row carries the typed
	// old→new role pair (RecordMemberRoleChanged), which isn't known until the
	// member is fetched — a refused change (bad role, plan gate, last admin)
	// leaves only the denial/attempt trail. Denials record immediately, with
	// the member target the caller asked to change.
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	if err := s.AuthorizeOnTarget(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID), core.MemberTarget(subject)); err != nil {
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
	s.RecordMemberRoleChanged(ctx, workspaceID, subject, m.Role, role)
	m.Role = role
	return memberView(m), nil
}

// Remove drops a member from a workspace: the row then its OpenFGA tuple.
// Admin-only. Refuses removing the last admin. The revoke is best-effort (an
// already-gone tuple is not an error), so a retried remove completes.
func (s *Service) Remove(ctx context.Context, workspaceID, subject string) error {
	if err := s.AuthorizeOnTarget(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID), core.MemberTarget(subject)); err != nil {
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
	if err := s.AuthorizeOnTarget(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID), core.InviteTarget(inviteID)); err != nil {
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
