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
	"io"
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
var ErrMembersUnavailable = core.Unavailable("workspace members store not configured")

// Stable GraphQL extensions.code values for direct invite redemption. Mobile
// uses these to clear terminal bearer capabilities while retaining only
// retryable transport failures; human error strings are not an API.
const (
	InviteErrorInvalid         = "INVITE_INVALID"
	InviteErrorExpired         = "INVITE_EXPIRED"
	InviteErrorAlreadyAccepted = "INVITE_ALREADY_ACCEPTED"
	InviteErrorPlanLimit       = "INVITE_PLAN_LIMIT"
	// InviteErrorAlreadyMember refuses inviting someone who already belongs to
	// the workspace (Render answers the same way). Their role is changed with
	// ChangeRole — an invite must never be a second, unaudited way to re-role a
	// member, which is how inviting an existing admin at the default role used to
	// demote them (w1/m82).
	InviteErrorAlreadyMember = "MEMBER_ALREADY_EXISTS"
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
	// InviteRandom supplies entropy for the 128-bit direct-accept capability.
	// Nil uses crypto/rand.Reader. Tests may inject a deterministic or failing
	// reader; production callers should leave this nil.
	InviteRandom io.Reader
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

// RoleReconciliationStore is the durable Postgres outbox used to converge an
// already-committed membership role into OpenFGA. It stays optional so DB-less
// deployments and the feature's narrow unit-test fakes retain their historical
// behavior; *store.PGStore implements it.
type RoleReconciliationStore interface {
	ClaimRoleReconciliations(ctx context.Context, limit int) ([]store.RoleReconciliation, error)
	CompleteRoleReconciliation(ctx context.Context, tenantID, subject, role string) error
	FailRoleReconciliation(ctx context.Context, tenantID, subject, role, message string) error
	// HasPendingRoleReconciliation backs the can_manage fail-closed veto
	// (round-19 #3): true while the caller's own role intent in this workspace
	// has not converged into OpenFGA yet.
	HasPendingRoleReconciliation(ctx context.Context, tenantID, subject string) (bool, error)
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
	WorkspaceID          string `json:"workspaceId"`
	WorkspaceName        string `json:"workspaceName"`
	Role                 string `json:"role"`
	AuthorizationPending bool   `json:"authorizationPending,omitempty"`
}

// Capabilities is the calling user's effective authorization in ONE workspace —
// the read-only projection the dashboard reads to gate controls the server would
// refuse (w9/m84, docs/ADR024-members.md § the contributor boundary). A member
// lacking a permission then sees a disabled control with a reason instead of an
// editable field that 403s on save. Role is the caller's UPPERCASE workspace role
// (omitted when the store is off or the membership can't be resolved); each
// boolean is a non-auditing probe of the matching OpenFGA relation
// (core.Base.Can), so the UI can never disagree with a real refusal and asking
// "what can I do" records nothing. The server stays authoritative — this is a
// courtesy hint, never a security boundary.
type Capabilities struct {
	Role             string `json:"role"`
	CanView          bool   `json:"canView"`
	CanViewLogs      bool   `json:"canViewLogs"`
	CanOperate       bool   `json:"canOperate"`
	CanCreate        bool   `json:"canCreate"`
	CanViewSensitive bool   `json:"canViewSensitive"`
	CanManageKeys    bool   `json:"canManageKeys"`
	CanManage        bool   `json:"canManage"`
	CanManageBilling bool   `json:"canManageBilling"`
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

// Capabilities returns the caller's effective authorization in workspaceID — the
// read-only projection the dashboard consumes to disable controls the server
// would refuse (w9/m84). Empty workspaceID resolves the caller's default
// workspace, exactly like a create. Requires can_view (any member); a non-member
// is ErrForbidden, like every workspace-scoped verb. The booleans are
// non-auditing Can-probes, so this read records nothing even though it touches
// every relation. The role string is best-effort: a nil store (DB-less mode) or
// an unresolvable membership just omits it — the booleans are the authoritative
// part the UI gates on.
func (s *Service) Capabilities(ctx context.Context, workspaceID string) (Capabilities, error) {
	ctx = core.WithWorkspace(ctx, workspaceID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return Capabilities{}, err
	}
	caps := Capabilities{
		CanView:          true, // the gate above proved it
		CanViewLogs:      s.Can(ctx, core.RelCanViewLogs),
		CanOperate:       s.Can(ctx, core.RelCanOperate),
		CanCreate:        s.Can(ctx, core.RelCanCreate),
		CanViewSensitive: s.Can(ctx, core.RelCanViewSensitive),
		CanManageKeys:    s.Can(ctx, core.RelCanManageKeys),
		CanManage:        s.Can(ctx, core.RelCanManage),
		CanManageBilling: s.Can(ctx, core.RelCanManageBilling),
	}
	if s.Store != nil {
		if tenantID, ok := s.Tenant(ctx); ok {
			if idn, ok := core.IdentityFrom(ctx); ok {
				if m, err := s.Store.GetTenantMember(ctx, tenantID, idn.Subject); err == nil {
					caps.Role = wireRole(m.Role)
				}
			}
		}
	}
	return caps, nil
}

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

// memberWithEmail reports whether email already belongs to an accepted member of
// the workspace — the invite verb's "already a member" guard (w1/m82).
//
// Membership is keyed by subject; the address lives in the identity provider, so
// this resolves the workspace's members through the same Identities seam List
// uses. Member counts are small and inviting is a rare admin action, so the
// per-member lookup is affordable here.
//
// Without an identity reader (BEX_KRATOS_ADMIN_URL unset) no address can be
// resolved, so the answer is an honest "not known to be a member" and the invite
// proceeds — redemption is the backstop that keeps an established member's role
// unchanged either way (store.redeemInvite), so the failure mode is a redundant
// invite, never a silent re-role.
func (s *Service) memberWithEmail(ctx context.Context, workspaceID, email string) (bool, error) {
	if s.Identities == nil {
		return false, nil
	}
	ms, err := s.Store.ListTenantMembers(ctx, workspaceID)
	if err != nil {
		return false, mapStoreErr(err)
	}
	for _, m := range ms {
		attrs, ok := s.Identities.LookupIdentity(ctx, m.Subject)
		if ok && strings.EqualFold(strings.TrimSpace(attrs.Email), email) {
			return true, nil
		}
	}
	return false, nil
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
	// round-5 finding 4: creating an admin invite is a durable-capability
	// issuance, so re-assert can_manage against the source of truth (uncached).
	// A caller demoted/removed within the last PositiveTTL must not ride a stale
	// cached positive to mint an invite that outlives the cache window.
	if err := s.AuthorizeFreshOn(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID)); err != nil {
		return InviteView{}, err
	}
	// round-19 #3: an invite is durable-capability issuance — refuse it while
	// the caller's own role reconciliation is pending (a stale higher tuple is
	// exactly what a demoted admin would invite through).
	if err := s.guardCallerRoleSettled(ctx, workspaceID); err != nil {
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

	// Someone who already belongs here cannot be "invited" again: their role is
	// ChangeRole's business. Checked BEFORE the plan/seat guards so the caller is
	// told the real reason rather than a cap error. (w1/m82)
	member, err := s.memberWithEmail(ctx, workspaceID, email)
	if err != nil {
		return InviteView{}, err
	}
	if member {
		return InviteView{}, core.NewConflictError(InviteErrorAlreadyMember,
			fmt.Sprintf("%s is already a member of this workspace; change their role instead of inviting them again", email),
			map[string]any{"email": email})
	}

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

	token, err := s.newInviteToken()
	if err != nil {
		return InviteView{}, err
	}
	inviter, _ := core.IdentityFrom(ctx)
	inv, err := s.Store.CreateInvite(ctx, workspaceID, email, role, token, inviter.Subject, s.Now().Add(s.inviteTTL()))
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
	// round-19 #3: resending re-issues a redeemable capability — the members
	// can_manage verbs fail closed while the caller's reconciliation pends.
	if err := s.guardCallerRoleSettled(ctx, workspaceID); err != nil {
		return InviteView{}, err
	}
	if s.Store == nil {
		return InviteView{}, ErrMembersUnavailable
	}
	token, err := s.newInviteToken()
	if err != nil {
		return InviteView{}, err
	}
	// Refresh first: the tenant row is only needed for the email body, so a 404
	// invite (accepted/unknown/cross-workspace) doesn't pay a wasted read.
	inv, err := s.Store.RefreshInvite(ctx, workspaceID, inviteID, token, s.Now().Add(s.inviteTTL()))
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
	pending, err := s.reconcileQueuedRole(ctx, inv.TenantID, id.Subject, inv.Role)
	if err != nil {
		// The membership and its reconciliation row committed atomically. Surface
		// the degraded state explicitly while the background worker retries; the
		// caller must never infer that OpenFGA already reflects the returned role.
		log.Printf("members: reconciling %s on workspace %s to %s: %v", inv.Role, inv.TenantID, id.Subject, err)
	}
	s.recordAccepted(ctx, inv, id)
	view := AcceptedInviteView{WorkspaceID: inv.TenantID, Role: wireRole(inv.Role), AuthorizationPending: pending}
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
// unchanged. Writes the row (source of truth) then reconciles OpenFGA: revokes
// the old relation, grants the new — so the auth gate follows the row, failing
// CLOSED on a partial demotion (round-19 #3).
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
	// round-5 finding 4: a role change (including self-promotion back to admin)
	// writes a durable membership tuple, so re-assert can_manage uncached — a
	// caller demoted within the last PositiveTTL must not ride a stale positive.
	if err := s.AuthorizeFreshOn(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID)); err != nil {
		return MemberView{}, err
	}
	// round-19 #3: a fresh check still reads OpenFGA's UNION of role tuples, so
	// a caller whose own demotion has not converged yet answers can_manage from
	// the stale higher tuple. Refuse while the caller's reconciliation is
	// pending — this is the re-promotion path that ordering alone cannot close.
	if err := s.guardCallerRoleSettled(ctx, workspaceID); err != nil {
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
		// Already at the target role in the row — but a prior partial failure may
		// have left OpenFGA out of sync. Converge to the EXACT role rather than
		// only repairing the grant (codex round-4 #4): the row is committed before
		// the tuple writes below, so a failed REVOKE strands the old, possibly
		// HIGHER, role tuple and lands every retry here. A grant-only repair could
		// never remove it, and the model ORs the five role relations, so a
		// downgraded admin would stay admin forever. reconcileExactRole grants the
		// target and revokes every other role tuple that actually exists, both
		// check-before-write. Without a checker (authz off) it degrades to a bare
		// idempotent grant — the historical no-op, since a missing tuple can't be
		// told from a present one. No audit row — nothing changed.
		_, queued := s.Store.(RoleReconciliationStore)
		if queued || (s.Base != nil && s.Base.Authz != nil) {
			if _, err := s.reconcileQueuedRole(ctx, workspaceID, subject, role); err != nil {
				return MemberView{}, err
			}
		}
		return memberView(m), nil
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
	// The row update and durable reconciliation intent committed together. Apply
	// the known old->new transition synchronously (this also works in the legacy
	// granter-without-checker mode), but retain the outbox row on either failure.
	//
	// round-19 #3: REVOKE the prior tuple before granting the new one. The model
	// ORs role relations, so the old grant-then-revoke order could leave a
	// demoted member holding BOTH tuples on a revoke failure — the union keeps
	// the higher authority effective until the reconciler converges. Revoke-first
	// fails closed instead: a grant failure strands the member with NO tuple
	// (under-privileged, repaired by the outbox), and a revoke failure leaves
	// exactly the pre-request state (never a new union of old+new).
	if err := s.revokeRoleErr(ctx, workspaceID, "user:"+subject, m.Role); err != nil {
		s.failQueuedRole(ctx, workspaceID, subject, role, err)
		return MemberView{}, fmt.Errorf("workspace %s: revoking prior role %q: %w", workspaceID, m.Role, err)
	}
	if err := s.grantRole(ctx, workspaceID, "user:"+subject, role); err != nil {
		s.failQueuedRole(ctx, workspaceID, subject, role, err)
		return MemberView{}, fmt.Errorf("workspace %s: granting role %q: %w", workspaceID, role, err)
	}
	if err := s.completeQueuedRole(ctx, workspaceID, subject, role); err != nil {
		return MemberView{}, err
	}
	s.RecordMemberRoleChanged(ctx, workspaceID, subject, m.Role, role)
	m.Role = role
	return memberView(m), nil
}

// reconcileQueuedRole applies the exact source-of-truth role, then acknowledges
// the matching outbox row. pending is true only when convergence failed and the
// durable row remains queued. A role changed again concurrently is safe: the
// conditional acknowledgement cannot delete the newer desired role.
func (s *Service) reconcileQueuedRole(ctx context.Context, tenantID, subject, role string) (pending bool, err error) {
	if err := s.reconcileExactRole(ctx, tenantID, "user:"+subject, role); err != nil {
		if saveErr := s.failQueuedRole(ctx, tenantID, subject, role, err); saveErr != nil {
			return true, errors.Join(err, fmt.Errorf("record role reconciliation failure: %w", saveErr))
		}
		return true, err
	}
	if err := s.completeQueuedRole(ctx, tenantID, subject, role); err != nil {
		return true, err
	}
	return false, nil
}

func (s *Service) completeQueuedRole(ctx context.Context, tenantID, subject, role string) error {
	if queue, ok := s.Store.(RoleReconciliationStore); ok {
		if err := queue.CompleteRoleReconciliation(ctx, tenantID, subject, role); err != nil {
			return fmt.Errorf("acknowledge role reconciliation: %w", err)
		}
	}
	return nil
}

func (s *Service) failQueuedRole(ctx context.Context, tenantID, subject, role string, reconcileErr error) error {
	if queue, ok := s.Store.(RoleReconciliationStore); ok {
		return queue.FailRoleReconciliation(ctx, tenantID, subject, role, reconcileErr.Error())
	}
	return nil
}

// ReconcilePendingRoles claims and repairs one bounded batch of durable role
// intents. Claim leases plus conditional acknowledgement make this safe across
// multiple bex-api replicas and crashes between the OpenFGA write and DB ack.
func (s *Service) reconcilePendingRoles(ctx context.Context, limit int) error {
	queue, ok := s.Store.(RoleReconciliationStore)
	if !ok {
		return nil
	}
	rows, err := queue.ClaimRoleReconciliations(ctx, limit)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := s.reconcileQueuedRole(ctx, row.TenantID, row.Subject, row.Role); err != nil {
			log.Printf("members: queued role reconciliation %s/%s=%s: %v", row.TenantID, row.Subject, row.Role, err)
			if errors.Is(err, context.Canceled) {
				return err
			}
			continue
		}
	}
	return nil
}

// RunRoleReconciler continuously drains the membership-role outbox.
func (s *Service) RunRoleReconciler(ctx context.Context) {
	if _, ok := s.Store.(RoleReconciliationStore); !ok {
		return
	}
	core.Poll(ctx, "members: role reconciliation sweep", 15*time.Second,
		func(ctx context.Context) error { return s.reconcilePendingRoles(ctx, 100) })
}

// Remove drops a member from a workspace: the row then its OpenFGA tuple.
// Admin-only. Refuses removing the last admin. The revoke is best-effort (an
// already-gone tuple is not an error), so a retried remove completes.
func (s *Service) Remove(ctx context.Context, workspaceID, subject string) error {
	if err := s.AuthorizeOnTarget(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID), core.MemberTarget(subject)); err != nil {
		return err
	}
	// codex round-8 #8: removing a member revokes their access irreversibly —
	// re-assert can_manage uncached (the Invite/ChangeRole pattern) so a caller
	// demoted or revoked inside PositiveTTL cannot drop one last member.
	if err := s.AuthorizeFreshOn(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID)); err != nil {
		return err
	}
	// round-19 #3: the members can_manage verbs fail closed while the caller's
	// own role reconciliation is pending (see ChangeRole).
	if err := s.guardCallerRoleSettled(ctx, workspaceID); err != nil {
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
	// Revoke the member's role tuple BEFORE deleting the row (F7). Two reasons: the
	// model ORs roles, so leaving the tuple keeps the removed member authorized;
	// and revoking first leaves a fail-closed intermediate on partial failure (row
	// present, no privilege) that a retry converges — whereas deleting the row
	// first would strand the stale tuple a Remove retry can no longer reach (the
	// member is gone, GetTenantMember 404s). The revoke error is now SURFACED, not
	// discarded, so the caller can retry to convergence.
	if err := s.revokeRoleErr(ctx, workspaceID, "user:"+subject, m.Role); err != nil {
		return err
	}
	if err := s.Store.RemoveMember(ctx, workspaceID, subject); err != nil {
		return mapStoreErr(err)
	}
	return nil
}

// RevokeInvite deletes a pending invite before it's redeemed. Admin-only.
func (s *Service) RevokeInvite(ctx context.Context, workspaceID, inviteID string) error {
	if err := s.AuthorizeOnTarget(ctx, core.RelCanManage, core.WorkspaceObject(workspaceID), core.InviteTarget(inviteID)); err != nil {
		return err
	}
	// round-19 #3: the members can_manage verbs fail closed while the caller's
	// own role reconciliation is pending (see ChangeRole).
	if err := s.guardCallerRoleSettled(ctx, workspaceID); err != nil {
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
		if ok, err := s.checkRoleFresh(ctx, subject, relation, tenantID); err == nil && ok {
			return nil
		}
	}
	return s.Granter.GrantWorkspaceRole(ctx, tenantID, subject, relation)
}

// reconcileExactRole revokes every OTHER role tuple the subject currently holds
// and grants `role`, converging OpenFGA to a single role (subject is already
// FGA-prefixed, "user:<id>"). It backs the invite-redemption paths, where the
// caller does NOT know the member's prior role: the model ORs the five role
// relations, so redeeming an invite for an existing member must drop the old
// tuple or both stay effective (w1/m65 F7).
//
// round-19 #3: the revokes run BEFORE the grant, so a grant failure strands the
// subject with NO tuple (fail-closed, repaired by the outbox) rather than a
// union of old+new. A failed revoke does not skip the grant: granting the
// desired role never raises the union above what the un-revoked tuple already
// grants, and it keeps convergence moving — the surfaced error keeps the outbox
// row pending so the retry (plus the callers' pending-role veto) closes the
// window. Without a checker (OpenFGA off) it degrades to a bare grant —
// byte-identical to the pre-fix behavior — because role existence can't be
// probed to revoke precisely.
func (s *Service) reconcileExactRole(ctx context.Context, tenantID, subject, role string) error {
	var errs []error
	if s.Base != nil && s.Base.Authz != nil {
		for _, other := range Roles {
			if other == role {
				continue
			}
			// Only skip a role tuple that is DEFINITIVELY absent (check-before-revoke):
			// keeps a no-op from masquerading as a failure and avoids asking OpenFGA to
			// delete a missing tuple. round-5 finding 16: a Check ERROR is NOT absence —
			// the old `err != nil || !ok` swallowed a transient failure and left a stale
			// higher-role tuple the OR-based model keeps effective. On an indeterminate
			// check, attempt the revoke (deleting an absent tuple is idempotent) and
			// surface any real error via errs so it isn't lost.
			ok, err := s.checkRoleFresh(ctx, subject, other, tenantID)
			if err == nil && !ok {
				continue
			}
			if err := s.revokeRoleErr(ctx, tenantID, subject, other); err != nil {
				errs = append(errs, fmt.Errorf("revoke %q: %w", other, err))
			}
		}
	}
	if err := s.grantRole(ctx, tenantID, subject, role); err != nil {
		errs = append(errs, fmt.Errorf("grant %q: %w", role, err))
	}
	return errors.Join(errs...)
}

// guardCallerRoleSettled refuses a can_manage verb while the CALLER's own role
// reconciliation for this workspace is still pending (round-19 #3). The
// tenant_members row commits before the OpenFGA tuples converge, and the model
// ORs role relations, so a fresh check during that window can answer from the
// caller's OLD, possibly higher tuple — the exact path a demoted administrator
// would use to re-promote themselves (or invite a colluding admin) before the
// 15s reconciler catches up. Failing closed here is bounded: the synchronous
// path deletes the outbox row before returning, so this only fires while
// convergence genuinely has not finished. A store without the outbox (DB-less
// mode, narrow fakes) keeps its historical behavior; a caller with no identity
// (background sweeps) was already decided by the Authorize gate above.
func (s *Service) guardCallerRoleSettled(ctx context.Context, workspaceID string) error {
	queue, ok := s.Store.(RoleReconciliationStore)
	if !ok {
		return nil
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok || id.Subject == "" {
		return nil
	}
	pending, err := queue.HasPendingRoleReconciliation(ctx, workspaceID, id.Subject)
	if err != nil {
		// Fail closed on the check itself: an unreadable outbox cannot vouch
		// that the caller's authorization has converged.
		return fmt.Errorf("workspace %s: check pending role reconciliation: %w", workspaceID, err)
	}
	if pending {
		return fmt.Errorf("%w: your workspace role change is still being applied; retry shortly", core.ErrForbidden)
	}
	return nil
}

func (s *Service) checkRoleFresh(ctx context.Context, subject, relation, tenantID string) (bool, error) {
	object := core.WorkspaceObject(tenantID)
	if fresh, ok := s.Base.Authz.(core.FreshChecker); ok {
		return fresh.CheckFresh(ctx, subject, relation, object)
	}
	return s.Base.Authz.Check(ctx, subject, relation, object)
}

// revokeRoleErr removes a member's role tuple, SURFACING the error (unlike the
// old best-effort revoke that discarded it, w1/m65 F7). No revoker => no-op
// success.
func (s *Service) revokeRoleErr(ctx context.Context, tenantID, subject, relation string) error {
	if s.Revoker == nil {
		return nil
	}
	return s.Revoker.RevokeWorkspaceMember(ctx, tenantID, subject, relation)
}

func (s *Service) inviteTTL() time.Duration {
	if s.InviteTTL > 0 {
		return s.InviteTTL
	}
	return DefaultInviteTTL
}

// newInviteToken mints an unguessable invite token (128 bits, hex). Entropy
// failure and short reads are fatal before CreateInvite/RefreshInvite: a blank
// or partially random bearer capability must never be stored or emailed.
func (s *Service) newInviteToken() (string, error) {
	reader := s.InviteRandom
	if reader == nil {
		reader = rand.Reader
	}
	var b [16]byte
	if _, err := io.ReadFull(reader, b[:]); err != nil {
		return "", fmt.Errorf("mint invite capability: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
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
