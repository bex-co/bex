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

package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// ErrLastAdmin is the typed refusal when a demotion or removal would leave a
// workspace with zero administrators (codex round-16 #3).
var ErrLastAdmin = fmt.Errorf("%w: cannot remove or demote the last admin of a workspace", ErrInvalid)

// Stable direct-invite redemption causes. Each wraps ErrConflict so existing
// REST/MCP status semantics remain unchanged while callers can classify the
// refusal without parsing human prose.
var (
	ErrInviteAlreadyAccepted = fmt.Errorf("invite already accepted: %w", ErrConflict)
	ErrInviteExpired         = fmt.Errorf("invite expired: %w", ErrConflict)
	ErrInvitePlanLimit       = fmt.Errorf("workspace plan cannot seat invite: %w", ErrConflict)
)

// members.go is the write side of workspace membership the w4/m12 team surface
// drives: role changes and removals on existing tenant_members rows, and the
// tenant_invites lifecycle (an email is invited, then redeemed into a
// tenant_members row on the recipient's first login). Reads
// (ListTenantMembers/CountTenantMembers) live in workspaces.go; this file is
// the mutating half plus invites.

// Invite is a row of `tenant_invites` — a pending workspace membership addressed
// to an email that has no OpenFGA subject yet. Redeemed on the recipient's first
// authenticated login (AcceptInvitesForEmail) into a tenant_members row + the
// matching workspace:<id> tuple.
type Invite struct {
	ID       string `json:"id"`
	TenantID string `json:"tenantId"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	// Token is the plaintext capability IN FLIGHT only: set by CreateInvite and
	// RefreshInvite (the two mints) so the caller can email the link, never
	// persisted — the row stores sha256(token) (w1/041), so reads return "".
	Token      string     `json:"token"`
	InvitedBy  string     `json:"invitedBy,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	AcceptedAt *time.Time `json:"acceptedAt,omitempty"`
}

// hashInviteToken is the at-rest form of an invite token (w1/041): the store
// writes and looks up sha256 hex, so a DB read yields no redeemable links. The
// token's 128-bit crypto/rand entropy makes the unsalted hash preimage- and
// table-resistant.
func hashInviteToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// GetTenantMember reads one membership row (ErrNotFound when the subject is not
// a member) — the read the role/remove verbs consult to learn the current role
// (the last-admin guard) before mutating.
func (s *PGStore) GetTenantMember(ctx context.Context, tenantID, subject string) (TenantMember, error) {
	var m TenantMember
	err := s.Pool.QueryRow(ctx,
		`SELECT tenant_id, subject, role, created_at FROM tenant_members
		 WHERE tenant_id = $1 AND subject = $2`, tenantID, subject,
	).Scan(&m.TenantID, &m.Subject, &m.Role, &m.CreatedAt)
	if err != nil {
		return TenantMember{}, classify("tenant_member", err)
	}
	return m, nil
}

// CountTenantAdmins counts a workspace's admin members — the guard the
// role-change/remove verbs consult so the last admin can't demote or remove
// itself, leaving a workspace nobody can administer.
func (s *PGStore) CountTenantAdmins(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM tenant_members WHERE tenant_id = $1 AND role = 'admin'`,
		tenantID).Scan(&n)
	return n, err
}

// SubjectIsWorkspaceAdmin reports whether subject currently holds the admin
// role in tenantID — the webhook failure-notice recipient gate (round-14 #6):
// a notice discloses (a redacted projection of) a destination that may have
// been configured by a different admin after this subject was removed or
// demoted, so CURRENT authorization state, not created_by provenance, decides
// the recipient. False for a non-member, a non-admin, and a workspace that
// does not exist — all equally "do not mail".
func (s *PGStore) SubjectIsWorkspaceAdmin(ctx context.Context, tenantID, subject string) (bool, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM tenant_members WHERE tenant_id = $1 AND subject = $2 AND role = 'admin')`,
		tenantID, subject,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// UpdateMemberRole changes an existing member's role (ErrNotFound when the
// subject is not a member of the workspace). The role CHECK is enforced by the
// API layer's validation, not the column, so an unknown role is a caller error
// mapped upstream, not a constraint violation here.
//
// SECURITY (codex round-16 #3): demoting an admin re-counts admins inside the
// same transaction under a tenant advisory lock so two concurrent demotions
// cannot both pass a standalone count and leave zero admins.
func (s *PGStore) UpdateMemberRole(ctx context.Context, tenantID, subject, role string) error {
	var affected int64
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, tenantID); err != nil {
			return err
		}
		var currentRole string
		err := tx.QueryRow(ctx,
			`SELECT role FROM tenant_members WHERE tenant_id = $1 AND subject = $2 FOR UPDATE`,
			tenantID, subject).Scan(&currentRole)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if currentRole == "admin" && role != "admin" {
			var admins int
			if err := tx.QueryRow(ctx,
				`SELECT count(*) FROM tenant_members WHERE tenant_id = $1 AND role = 'admin'`,
				tenantID).Scan(&admins); err != nil {
				return err
			}
			if admins <= 1 {
				return ErrLastAdmin
			}
		}
		tag, err := tx.Exec(ctx,
			`UPDATE tenant_members SET role = $3 WHERE tenant_id = $1 AND subject = $2`,
			tenantID, subject, role)
		if err != nil {
			return err
		}
		affected = tag.RowsAffected()
		if affected == 0 {
			return nil
		}
		return enqueueRoleReconciliation(ctx, tx, tenantID, subject, role)
	})
	if err != nil {
		if errors.Is(err, ErrLastAdmin) {
			return err
		}
		return classify("tenant_member", err)
	}
	if affected == 0 {
		return fmt.Errorf("tenant_member %s: %w", subject, ErrNotFound)
	}
	return nil
}

// RemoveMember deletes a membership row (ErrNotFound when absent). The FGA tuple
// removal is the caller's separate, best-effort step (members.Service.Remove).
//
// SECURITY (codex round-16 #3): removing an admin re-counts under the same
// tenant advisory lock as UpdateMemberRole so concurrent remove/demote cannot
// erase the last administrator.
func (s *PGStore) RemoveMember(ctx context.Context, tenantID, subject string) error {
	var affected int64
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, tenantID); err != nil {
			return err
		}
		var currentRole string
		err := tx.QueryRow(ctx,
			`SELECT role FROM tenant_members WHERE tenant_id = $1 AND subject = $2 FOR UPDATE`,
			tenantID, subject).Scan(&currentRole)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if currentRole == "admin" {
			var admins int
			if err := tx.QueryRow(ctx,
				`SELECT count(*) FROM tenant_members WHERE tenant_id = $1 AND role = 'admin'`,
				tenantID).Scan(&admins); err != nil {
				return err
			}
			if admins <= 1 {
				return ErrLastAdmin
			}
		}
		tag, err := tx.Exec(ctx,
			`DELETE FROM tenant_members WHERE tenant_id = $1 AND subject = $2`,
			tenantID, subject)
		if err != nil {
			return err
		}
		affected = tag.RowsAffected()
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrLastAdmin) {
			return err
		}
		return classify("tenant_member", err)
	}
	if affected == 0 {
		return fmt.Errorf("tenant_member %s: %w", subject, ErrNotFound)
	}
	return nil
}

// CountInvites counts a workspace's OUTSTANDING invites (unaccepted, unexpired)
// — the count the seat-cap check consults, so the Invite verb doesn't SELECT
// full rows just to length them. Mirrors CountTenantMembers.
func (s *PGStore) CountInvites(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM tenant_invites
		 WHERE tenant_id = $1 AND accepted_at IS NULL AND expires_at > now()`,
		tenantID).Scan(&n)
	return n, err
}

// CreateInvite records a pending invite (ErrConflict when an outstanding invite
// already targets the same (tenant, email) — the partial unique index). The id
// is minted here; expiresAt is computed by the caller so the TTL is one policy
// in the service layer. Only sha256(token) is stored (w1/041); the returned
// Invite carries the plaintext so the caller can email the link.
func (s *PGStore) CreateInvite(ctx context.Context, tenantID, email, role, token, invitedBy string, expiresAt time.Time) (Invite, error) {
	inv := Invite{
		ID: ids.New(ids.Invite), TenantID: tenantID, Email: email, Role: role,
		Token: token, InvitedBy: invitedBy, ExpiresAt: expiresAt,
	}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO tenant_invites (id, tenant_id, email, role, token_hash, invited_by, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING created_at`,
		inv.ID, tenantID, email, role, hashInviteToken(token), invitedBy, expiresAt,
	).Scan(&inv.CreatedAt)
	if err != nil {
		return Invite{}, classify("invite", err)
	}
	return inv, nil
}

const inviteColumns = `id, tenant_id, email, role, invited_by, created_at, expires_at, accepted_at`

func scanInvite(row pgx.Row) (Invite, error) {
	var inv Invite
	err := row.Scan(&inv.ID, &inv.TenantID, &inv.Email, &inv.Role,
		&inv.InvitedBy, &inv.CreatedAt, &inv.ExpiresAt, &inv.AcceptedAt)
	return inv, err
}

// ListInvites returns a workspace's OUTSTANDING invites — not yet accepted and
// not expired — oldest first. Accepted/expired rows are audit history, not
// pending work, so the Team page's pending list excludes them.
func (s *PGStore) ListInvites(ctx context.Context, tenantID string) ([]Invite, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+inviteColumns+` FROM tenant_invites
		 WHERE tenant_id = $1 AND accepted_at IS NULL AND expires_at > now()
		 ORDER BY created_at`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invite
	for rows.Next() {
		inv, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// GetInvite reads one invite scoped to its workspace (ErrNotFound otherwise) —
// the revoke verb's existence check so it can't delete another workspace's row.
func (s *PGStore) GetInvite(ctx context.Context, tenantID, id string) (Invite, error) {
	inv, err := scanInvite(s.Pool.QueryRow(ctx,
		`SELECT `+inviteColumns+` FROM tenant_invites WHERE tenant_id = $1 AND id = $2`,
		tenantID, id))
	if err != nil {
		return Invite{}, classify("invite", err)
	}
	return inv, nil
}

// RefreshInvite pushes an unaccepted invite's expiry forward and replaces its
// token — the resend verb's write half (w1/m33). The token rotates (w1/041):
// with only sha256(token) at rest the old plaintext cannot be re-emailed, so
// resend mints a fresh capability and the freshly emailed link supersedes the
// original, which stops redeeming. An expired-but-unaccepted invite is revived
// (resend is how an admin recovers a lapsed invite without churning the id);
// an accepted or unknown invite is ErrNotFound — accepted rows are audit
// history, not pending work. The returned Invite carries the plaintext token
// for the resent mail.
func (s *PGStore) RefreshInvite(ctx context.Context, tenantID, id, token string, expiresAt time.Time) (Invite, error) {
	inv, err := scanInvite(s.Pool.QueryRow(ctx,
		`UPDATE tenant_invites SET expires_at = $3, token_hash = $4
		 WHERE tenant_id = $1 AND id = $2 AND accepted_at IS NULL
		 RETURNING `+inviteColumns, tenantID, id, expiresAt, hashInviteToken(token)))
	if err != nil {
		return Invite{}, classify("invite", err)
	}
	inv.Token = token
	return inv, nil
}

// DeleteInvite revokes a pending invite (ErrNotFound when absent / another
// workspace's). Idempotency is the caller's concern; a missing row is a 404.
func (s *PGStore) DeleteInvite(ctx context.Context, tenantID, id string) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM tenant_invites WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return classify("invite", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("invite %s: %w", id, ErrNotFound)
	}
	return nil
}

// AcceptInvitesForEmail redeems every outstanding invite addressed to email into
// a tenant_members row for subject, marking each invite accepted — the whole set
// in ONE transaction so a signup either joins all its invited workspaces or none
// (a partial join with dangling invites would be confusing to re-drive). It
// returns the accepted invites so the caller can write the matching OpenFGA
// tuples (Postgres and OpenFGA aren't one transaction; the row is the source of
// truth, the tuple a best-effort follow-up the resolver re-drives). Idempotent:
// a second login finds no outstanding invites and returns an empty slice. An
// invite for someone who already belongs to the workspace is still marked
// accepted (so it stops lingering) but leaves their role untouched — see
// redeemInvite; each returned invite carries the EFFECTIVE membership role.
func (s *PGStore) AcceptInvitesForEmail(ctx context.Context, email, subject string) ([]Invite, error) {
	// Read the pending set outside any transaction — the overwhelmingly common
	// case (a login with no pending invite) then costs one SELECT, not a
	// BEGIN/COMMIT round trip on the auth hot path.
	rows, err := s.Pool.Query(ctx,
		`SELECT `+inviteColumns+` FROM tenant_invites
		 WHERE email = $1 AND accepted_at IS NULL AND expires_at > now()`, email)
	if err != nil {
		return nil, classify("invite", err)
	}
	defer rows.Close()
	var pending []Invite
	for rows.Next() {
		inv, err := scanInvite(rows)
		if err != nil {
			return nil, classify("invite", err)
		}
		pending = append(pending, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, classify("invite", err)
	}
	if len(pending) == 0 {
		return nil, nil
	}
	// Redeem the whole set in one transaction so a signup joins all its invited
	// workspaces or none. An invite whose workspace can no longer seat it under
	// its CURRENT plan is left pending rather than redeemed (planAllowsJoin) —
	// the plan caps are enforced here, at the moment membership is created,
	// because that is the only point every path to a tenant_members row passes
	// through. Invite-time and ChangePlan-time checks both look at state that
	// can change before the invitee ever logs in: an invite issued on Pro (2nd
	// member, or a `developer` role Hobby doesn't offer) outlives a downgrade to
	// Hobby, whose guards count accepted members only — so without this check a
	// Hobby workspace ends up over its member cap, or holding a role its plan
	// forbids (verified live on prod, w6/m13). A skipped invite stays pending and
	// self-heals: it redeems on the next login after the workspace upgrades again.
	accepted := make([]Invite, 0, len(pending))
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockSubjectMembership(ctx, tx, subject); err != nil {
			return err
		}
		if err := refuseDeletingSubject(ctx, tx, subject); err != nil {
			return err
		}
		for _, inv := range pending {
			ok, err := planAllowsJoin(ctx, tx, inv, subject)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			effective, err := redeemInvite(ctx, tx, inv, subject)
			if err != nil {
				return err
			}
			// Report the role the membership actually holds: redemption never
			// re-roles an established member, so the caller's OpenFGA tuple and
			// audit row describe the real membership, not the invite's wish.
			inv.Role = effective
			accepted = append(accepted, inv)
		}
		return nil
	})
	if err != nil {
		return nil, classify("invite", err)
	}
	return accepted, nil
}

// redeemInvite is the one redemption write both acceptance paths share: the
// membership upsert plus marking the invite accepted, inside the caller's
// transaction. It returns the role the membership actually holds afterwards.
//
// An invite NEVER changes an established member's role (w1/m82). It used to
// upsert `DO UPDATE SET role = EXCLUDED.role` so a re-invite could upgrade a
// member, but that same write silently DOWNGRADED one: inviting an existing
// admin at the default role demoted them on their next request, reported to the
// admin as "Invitation sent" and applied nowhere the UI showed. Changing a
// member's role has exactly one verb (ChangeRole), which is audited and refuses
// to strip the last admin; redemption only ever CREATES a membership.
//
// `DO UPDATE SET role = tenant_members.role` is a deliberate no-op update rather
// than DO NOTHING: it keeps the existing role while still firing RETURNING, so
// one statement yields the effective role on both the insert and the conflict
// path. The effective role — not the invited one — is what the reconciliation
// outbox and the caller's OpenFGA tuple are written from, so the row and the
// tuple can never disagree about what the member may do.
func redeemInvite(ctx context.Context, tx pgx.Tx, inv Invite, subject string) (string, error) {
	var effective string
	if err := tx.QueryRow(ctx,
		`INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, $3)
		 ON CONFLICT (tenant_id, subject) DO UPDATE SET role = tenant_members.role
		 RETURNING role`,
		inv.TenantID, subject, inv.Role).Scan(&effective); err != nil {
		return "", err
	}
	if err := enqueueRoleReconciliation(ctx, tx, inv.TenantID, subject, effective); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE tenant_invites SET accepted_at = now() WHERE id = $1`, inv.ID); err != nil {
		return "", err
	}
	return effective, nil
}

func enqueueRoleReconciliation(ctx context.Context, tx pgx.Tx, tenantID, subject, role string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO membership_role_reconciliations (tenant_id, subject, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, subject) DO UPDATE
		SET role = EXCLUDED.role, attempts = 0, next_attempt_at = now(), last_error = NULL, updated_at = now()`,
		tenantID, subject, role)
	return err
}

// AcceptInviteByToken redeems ONE invite by its emailed token for subject —
// the direct-accept path (w1/m33) that makes the invite link real: the
// recipient may have signed up under a DIFFERENT email than the one invited,
// which the login-time email match (AcceptInvitesForEmail) can never redeem.
// The token is the capability; possession of the link is the authorization.
// The lookup is by sha256(token) — only the hash is at rest (w1/041).
// Named refusals rather than a silent no-op: unknown token is ErrNotFound,
// an already-accepted or expired invite is ErrConflict (the caller can say
// WHY the link failed), and a workspace whose current plan cannot seat the
// invite refuses with ErrConflict exactly like the login path's silent skip —
// except here the caller is told, because they asked explicitly.
func (s *PGStore) AcceptInviteByToken(ctx context.Context, token, subject string) (Invite, error) {
	var accepted Invite
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockSubjectMembership(ctx, tx, subject); err != nil {
			return err
		}
		if err := refuseDeletingSubject(ctx, tx, subject); err != nil {
			return err
		}
		inv, err := scanInvite(tx.QueryRow(ctx,
			`SELECT `+inviteColumns+` FROM tenant_invites WHERE token_hash = $1 FOR UPDATE`,
			hashInviteToken(token)))
		if err != nil {
			return err // pgx.ErrNoRows → classify's ErrNotFound below
		}
		switch {
		case inv.AcceptedAt != nil:
			return ErrInviteAlreadyAccepted
		case time.Now().After(inv.ExpiresAt):
			return ErrInviteExpired
		}
		ok, err := planAllowsJoin(ctx, tx, inv, subject)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInvitePlanLimit
		}
		effective, err := redeemInvite(ctx, tx, inv, subject)
		if err != nil {
			return err
		}
		// See AcceptInvitesForEmail: the accepted invite reports the effective
		// membership role, which is what the caller reconciles into OpenFGA.
		inv.Role = effective
		accepted = inv
		return nil
	})
	if err != nil {
		return Invite{}, classify("invite", err)
	}
	return accepted, nil
}

// planAllowsJoin reports whether redeeming inv for subject keeps the workspace
// within its current plan — the accept-time half of the plan-limit enforcement
// (LimitsFor/RoleAllowedOnPlan are the same predicates invite and ChangePlan
// use). A subject who is ALREADY a member takes no new seat, so the cap does not
// apply to them (their role is left as-is by redemption, w1/m82).
func planAllowsJoin(ctx context.Context, tx pgx.Tx, inv Invite, subject string) (bool, error) {
	var plan string
	if err := tx.QueryRow(ctx, `SELECT plan FROM tenants WHERE id = $1`, inv.TenantID).Scan(&plan); err != nil {
		return false, err
	}
	if !RoleAllowedOnPlan(plan, inv.Role) {
		return false, nil
	}
	limits := LimitsFor(plan)
	if limits.MaxMembers <= 0 {
		return true, nil // unlimited seats on this plan
	}
	var alreadyMember bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM tenant_members WHERE tenant_id = $1 AND subject = $2)`,
		inv.TenantID, subject).Scan(&alreadyMember); err != nil {
		return false, err
	}
	if alreadyMember {
		return true, nil // role change, not a new seat
	}
	members, err := countTenantMembers(ctx, tx, inv.TenantID)
	if err != nil {
		return false, err
	}
	return members < limits.MaxMembers, nil
}
