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
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// TenantMember is a row of `tenant_members` — a subject's membership of a
// workspace and their role. Subject is the OpenFGA user (Kratos identity id or
// Hydra client id) granted the matching relation on workspace:<tenant-id>.
type TenantMember struct {
	TenantID  string    `json:"tenantId"`
	Subject   string    `json:"subject"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	// Kind is MemberKindUser for a person and MemberKindMachine for an API
	// key's binding (w5/m103). Both rows authorize; only people are members.
	Kind string `json:"kind"`
}

// The two kinds of tenant_members row. A machine row is what makes a bound API
// key resolve to its workspace and authorize there — it is not a member of the
// workspace in any sense the Team surface or the seat cap should mean.
const (
	MemberKindUser    = "user"
	MemberKindMachine = "machine"
)

// CreateWorkspace inserts a tenant row and the owner's `admin` membership in one
// transaction — the atomic create the workspaces feature writes through, so a
// workspace never exists without an owner (the OpenFGA grant is a separate,
// caller-driven step, see workspaces.Service.Create). Name collisions and the
// plan CHECK surface as ErrConflict / ErrInvalid via classify.
func (s *PGStore) CreateWorkspace(ctx context.Context, name, plan, ownerSubject string) (Tenant, error) {
	t := Tenant{ID: ids.New(ids.Workspace), Name: name, Plan: plan}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockSubjectMembership(ctx, tx, ownerSubject); err != nil {
			return err
		}
		if err := refuseDeletingSubject(ctx, tx, ownerSubject); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO tenants (id, name, plan) VALUES ($1, $2, $3) RETURNING created_at`,
			t.ID, name, plan,
		).Scan(&t.CreatedAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, 'admin')`,
			t.ID, ownerSubject)
		return err
	})
	if err != nil {
		return Tenant{}, classify("workspace", err)
	}
	return t, nil
}

// TenantForOwner returns the personal tenant an identity OWNS (by
// owner_identity_id), or ErrNotFound — the cheap read the onboarding path tries
// before the upsert, so a returning caller's login is a single SELECT, not a
// write transaction. Unlike TenantForIdentity (a membership JOIN that can now
// return a workspace the caller was merely invited to), this returns only the
// workspace the caller actually owns (w4/m12).
func (s *PGStore) TenantForOwner(ctx context.Context, identityID string) (Tenant, error) {
	t, err := scanTenant(s.Pool.QueryRow(ctx,
		`SELECT `+tenantColumns+` FROM tenants WHERE owner_identity_id = $1`, identityID))
	if err != nil {
		return Tenant{}, classify("tenant", err)
	}
	return t, nil
}

// TenantOwnerSubject returns the identity a workspace's onboarding binding
// names (tenants.owner_identity_id), or "" when the binding is NULL — the shape
// every workspace minted through CreateWorkspace has, since only onboarding's
// CreateTenantWithMember sets it. ErrNotFound when the workspace doesn't exist.
//
// The members view reads it to mark the creator's row (information only). The
// binding is the key EnsureTenant resolves the personal workspace by, so the
// store ends it when the creator is removed, demoted or leaves (detachOwnerTx,
// w5/m372).
func (s *PGStore) TenantOwnerSubject(ctx context.Context, tenantID string) (string, error) {
	var owner *string
	if err := s.Pool.QueryRow(ctx,
		`SELECT owner_identity_id FROM tenants WHERE id = $1`, tenantID,
	).Scan(&owner); err != nil {
		return "", classify("workspace", err)
	}
	if owner == nil {
		return "", nil
	}
	return *owner, nil
}

// TenantContactSubjects returns, for each of tenantIDs, the subject whose email
// is the workspace's contact: its owner binding when set, else its oldest user
// admin, ties broken by subject (w5/126). One query serves a whole list.
// Unknown ids and workspaces with neither are absent.
func (s *PGStore) TenantContactSubjects(ctx context.Context, tenantIDs []string) (map[string]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT t.id, COALESCE(NULLIF(t.owner_identity_id, ''), (
			SELECT m.subject FROM tenant_members m
			WHERE m.tenant_id = t.id AND m.kind = 'user' AND m.role = 'admin'
			ORDER BY m.created_at, m.subject LIMIT 1))
		FROM tenants t WHERE t.id = ANY($1)`, tenantIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string, len(tenantIDs))
	for rows.Next() {
		var id string
		var subject *string
		if err := rows.Scan(&id, &subject); err != nil {
			return nil, err
		}
		if subject != nil {
			out[id] = *subject
		}
	}
	return out, rows.Err()
}

// GetTenant reads one tenant by id (ErrNotFound when absent).
func (s *PGStore) GetTenant(ctx context.Context, id string) (Tenant, error) {
	t, err := scanTenant(s.Pool.QueryRow(ctx,
		`SELECT `+tenantColumns+` FROM tenants WHERE id = $1`, id))
	if err != nil {
		return Tenant{}, classify("workspace", err)
	}
	return t, nil
}

// SetTenantDisplayName renames a workspace: it sets the freeform display name
// (w3/m246, Render's rename). The slug and id are untouched, so no reference
// breaks. ErrNotFound when the id doesn't exist.
func (s *PGStore) SetTenantDisplayName(ctx context.Context, id, name string) (Tenant, error) {
	t, err := scanTenant(s.Pool.QueryRow(ctx,
		`UPDATE tenants SET display_name = $2, updated_at = now() WHERE id = $1
		 RETURNING `+tenantColumns, id, name))
	if err != nil {
		return Tenant{}, classify("workspace", err)
	}
	return t, nil
}

// SetTenantRequireTwoFactor turns a workspace's "require two-factor
// authentication" setting on or off and returns the row (ErrNotFound when the
// id doesn't exist). Enforcement reads it through RequireTwoFactor; this write
// is the admin mutation's. changed is false when the row already held enabled:
// the write is conditional, so of two admins toggling at once exactly one gets
// changed=true and the other must not audit or announce a change it did not make.
func (s *PGStore) SetTenantRequireTwoFactor(ctx context.Context, id string, enabled bool) (t Tenant, changed bool, err error) {
	t, err = scanTenant(s.Pool.QueryRow(ctx,
		`UPDATE tenants SET require_two_factor = $2, updated_at = now()
		 WHERE id = $1 AND require_two_factor <> $2
		 RETURNING `+tenantColumns, id, enabled))
	if errors.Is(err, pgx.ErrNoRows) {
		t, err = s.GetTenant(ctx, id) // already there, or no such workspace (ErrNotFound)
		return t, false, err
	}
	if err != nil {
		return Tenant{}, false, classify("workspace", err)
	}
	return t, true, nil
}

// RequireTwoFactor reads only the setting — the enforcement point's question,
// asked at most once a minute per workspace (internal/twofactor). A workspace
// that does not exist requires nothing: false, no error.
func (s *PGStore) RequireTwoFactor(ctx context.Context, id string) (bool, error) {
	var required bool
	err := s.Pool.QueryRow(ctx, `SELECT require_two_factor FROM tenants WHERE id = $1`, id).Scan(&required)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, classify("workspace", err)
	}
	return required, nil
}

// SetTenantDisplayNameIfUnset gives a workspace its first display name and
// leaves one already set alone, so a signup workspace is named once (w3/m246)
// and a user's later rename is never overwritten.
func (s *PGStore) SetTenantDisplayNameIfUnset(ctx context.Context, id, displayName string) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE tenants SET display_name = $2, updated_at = now()
		 WHERE id = $1 AND display_name = ''`, id, displayName)
	if err != nil {
		return classify("workspace", err)
	}
	if tag.RowsAffected() == 0 {
		if _, err := s.GetTenant(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// UpdateTenantPlan changes a workspace's plan (ErrNotFound when the id doesn't
// exist). The plan is validated by the caller (workspaces.Service.ChangePlan
// via store.NormalizePlan) before this write; the `tenants.plan` CHECK
// constraint is the last-resort backstop, surfaced as ErrInvalid via classify.
//
// It is every plan change's one write, so it also keeps the workspace's log
// floor in step (w9/m389 t004): an upgrade stores the old plan's floor, so
// logs that had already aged out stay hidden, and a downgrade drops a kept
// floor its shorter window already covers (core.LogsVisibleFromAfterPlanChange).
// The row is locked first, so two changes at once cannot both read the old
// plan.
func (s *PGStore) UpdateTenantPlan(ctx context.Context, id, plan string) (Tenant, error) {
	var t Tenant
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var old string
		var visibleFrom *time.Time
		if err := tx.QueryRow(ctx, `SELECT plan, logs_visible_from FROM tenants WHERE id = $1 FOR UPDATE`, id).Scan(&old, &visibleFrom); err != nil {
			return err
		}
		var kept *time.Time
		if next := core.LogsVisibleFromAfterPlanChange(old, plan, timeOrZero(visibleFrom), time.Now()); !next.IsZero() {
			kept = &next
		}
		var err error
		t, err = scanTenant(tx.QueryRow(ctx,
			`UPDATE tenants SET plan = $2, logs_visible_from = $3, updated_at = now() WHERE id = $1
			 RETURNING `+tenantColumns, id, plan, kept))
		return err
	})
	if err != nil {
		return Tenant{}, classify("workspace", err)
	}
	return t, nil
}

// WorkspacePlan is a workspace's plan (core.WorkspacePlanReader). ErrNotFound
// when the workspace doesn't exist.
func (s *PGStore) WorkspacePlan(ctx context.Context, workspaceID string) (string, error) {
	plan, _, err := s.WorkspaceLogPlan(ctx, workspaceID)
	return plan, err
}

// WorkspaceLogPlan is a workspace's plan and the log floor its last plan
// change kept, in one read (core.WorkspaceLogPlanReader, w9/m389): the floor
// is zero when it kept none. ErrNotFound when the workspace doesn't exist.
func (s *PGStore) WorkspaceLogPlan(ctx context.Context, workspaceID string) (string, time.Time, error) {
	var plan string
	var visibleFrom *time.Time
	if err := s.Pool.QueryRow(ctx, `SELECT plan, logs_visible_from FROM tenants WHERE id = $1`, workspaceID).Scan(&plan, &visibleFrom); err != nil {
		return "", time.Time{}, classify("workspace", err)
	}
	return plan, timeOrZero(visibleFrom), nil
}

// timeOrZero reads a nullable timestamp: NULL is the zero time.
func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// DeleteTenant removes a workspace row. The FK cascades (ON DELETE CASCADE) drop
// its apps, their domains, and its tenant_members in the same statement; the
// projector then prunes the orphaned App CRs on its next pass. Returns
// ErrNotFound when the id doesn't exist.
func (s *PGStore) DeleteTenant(ctx context.Context, id string) error {
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		// Agent attachments have no tenant FK: retiring them here leaves the
		// sweep a row for every object to purge (ADR095).
		if err := retireWorkspaceAttachments(ctx, tx, id); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("workspace %s: %w", id, ErrNotFound)
		}
		return nil
	})
	if err != nil && !errors.Is(err, ErrNotFound) {
		return classify("workspace", err)
	}
	return err
}

// ListTenantsForSubject returns the workspaces the subject belongs to, oldest
// first — the caller's workspace list (the dashboard switcher / owners list).
func (s *PGStore) ListTenantsForSubject(ctx context.Context, subject string) ([]Tenant, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT t.id, t.name, t.plan, t.created_at, t.display_name, t.require_two_factor
		 FROM tenants t JOIN tenant_members m ON m.tenant_id = t.id
		 WHERE m.subject = $1 ORDER BY t.created_at`, subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tenant
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CountWorkspacesForSubjectPlan counts how many workspaces of a given plan the
// subject owns — the per-user plan cap check (Render's five-Hobby-workspace
// limit). Membership, not just ownership, is the unit Render caps on.
func (s *PGStore) CountWorkspacesForSubjectPlan(ctx context.Context, subject, plan string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM tenants t JOIN tenant_members m ON m.tenant_id = t.id
		 WHERE m.subject = $1 AND t.plan = $2`, subject, plan,
	).Scan(&n)
	return n, err
}

// CountAppsForTenant counts a workspace's apps (all of them, including suspended
// — the `apps` row exists whether or not spec.suspended) for the per-plan
// service cap.
func (s *PGStore) CountAppsForTenant(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM apps a WHERE a.tenant_id = $1 AND `+serviceAppsOnly, tenantID).Scan(&n)
	return n, err
}

// CountBuildSourcesForTenant counts a workspace's build source carriers, for
// their own ceiling (QuotaCaps.BuildSources).
func (s *PGStore) CountBuildSourcesForTenant(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM apps a WHERE a.tenant_id = $1 AND `+buildSourcesOnly, tenantID).Scan(&n)
	return n, err
}

// queryRower is the package's transaction-aware single-row query shape. It is
// satisfied by both *pgxpool.Pool and pgx.Tx, so shared query helpers can run
// either standalone or inside an in-flight transaction.
type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// CountTenantMembers counts a workspace's members — the guard w4/m12's
// invite verb consults before adding one (Hobby's single-member cap).
func (s *PGStore) CountTenantMembers(ctx context.Context, tenantID string) (int, error) {
	return countTenantMembers(ctx, s.Pool, tenantID)
}

// countTenantMembers counts PEOPLE (w5/m103) — the seat cap charges for
// teammates, not for the workspace's own API keys, and this count is both what
// SeatUsage displays and what CanAddMember refuses on, so the two cannot
// disagree.
func countTenantMembers(ctx context.Context, q queryRower, tenantID string) (int, error) {
	var n int
	err := q.QueryRow(ctx,
		`SELECT count(*) FROM tenant_members WHERE tenant_id = $1 AND kind = 'user'`, tenantID).Scan(&n)
	return n, err
}

// ListTenantMembers returns a workspace's PEOPLE, oldest first (w5/m103):
// machine bindings are excluded, because every consumer of this read — the Team
// page, the Render owners members API, seat usage — is answering "who belongs
// to this workspace", and an API key is not a who. The key's own row still
// exists and still authorizes; it is listed on the API-keys surface instead.
func (s *PGStore) ListTenantMembers(ctx context.Context, tenantID string) ([]TenantMember, error) {
	return s.ListMembersOfTenants(ctx, []string{tenantID})
}

// ListMembersOfTenants is ListTenantMembers for several workspaces in one
// query; each row names its TenantID.
func (s *PGStore) ListMembersOfTenants(ctx context.Context, tenantIDs []string) ([]TenantMember, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT tenant_id, subject, role, created_at, kind FROM tenant_members
		 WHERE tenant_id = ANY($1) AND kind = 'user' ORDER BY created_at`, tenantIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TenantMember
	for rows.Next() {
		var m TenantMember
		if err := rows.Scan(&m.TenantID, &m.Subject, &m.Role, &m.CreatedAt, &m.Kind); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// OwnerIDForSubject returns the stable opaque "own-" id for a subject, minting
// and persisting one on first sight (w6/m7): OwnerIDsForSubjects for one
// subject. The Render owners members surface reports userId as this id instead
// of leaking the raw Kratos/Hydra subject.
func (s *PGStore) OwnerIDForSubject(ctx context.Context, subject string) (string, error) {
	ownIDs, err := s.OwnerIDsForSubjects(ctx, []string{subject})
	if err != nil {
		return "", err
	}
	return ownIDs[subject], nil
}

// LookupOwnerIDs returns the own- ids already minted for subjects, minting
// none: a read surface that names machine callers (API-key client ids) beside
// people must not create user ids for them (w5/m277).
func (s *PGStore) LookupOwnerIDs(ctx context.Context, subjects []string) (map[string]string, error) {
	out := make(map[string]string, len(subjects))
	if len(subjects) == 0 {
		return out, nil
	}
	rows, err := s.Pool.Query(ctx, `SELECT subject, own_id FROM owner_ids WHERE subject = ANY($1)`, subjects)
	if err != nil {
		return nil, classify("owner_id", err)
	}
	defer rows.Close()
	for rows.Next() {
		var subject, ownID string
		if err := rows.Scan(&subject, &ownID); err != nil {
			return nil, err
		}
		out[subject] = ownID
	}
	return out, rows.Err()
}

// SubjectForOwnerID inverts OwnerIDsForSubjects: the subject an own- id was
// minted for (own_id is unique). An id never minted is ErrNotFound.
func (s *PGStore) SubjectForOwnerID(ctx context.Context, ownID string) (string, error) {
	var subject string
	err := s.Pool.QueryRow(ctx, `SELECT subject FROM owner_ids WHERE own_id = $1`, ownID).Scan(&subject)
	if err != nil {
		return "", classify("owner_id", err)
	}
	return subject, nil
}

// OwnerIDsForSubjects returns each subject's opaque own- id, minting one on
// first sight. The ids a list already has are read in one query, and only a
// subject without one is written (w5/104): the Team page polls its members
// every 30s, and an upsert writes a row version even when its DO UPDATE changes
// nothing. A miss still goes through the upsert, so a concurrent first sight of
// the same subject yields exactly one row (the subject is the primary key), and
// RETURNING surfaces the stored id rather than the freshly minted candidate.
func (s *PGStore) OwnerIDsForSubjects(ctx context.Context, subjects []string) (map[string]string, error) {
	out := make(map[string]string, len(subjects))
	if len(subjects) == 0 {
		return out, nil
	}
	rows, err := s.Pool.Query(ctx, `SELECT subject, own_id FROM owner_ids WHERE subject = ANY($1)`, subjects)
	if err != nil {
		return nil, classify("owner_id", err)
	}
	defer rows.Close()
	for rows.Next() {
		var subject, ownID string
		if err := rows.Scan(&subject, &ownID); err != nil {
			return nil, classify("owner_id", err)
		}
		out[subject] = ownID
	}
	if err := rows.Err(); err != nil {
		return nil, classify("owner_id", err)
	}
	for _, subject := range subjects {
		if _, ok := out[subject]; ok {
			continue
		}
		var ownID string
		if err := s.Pool.QueryRow(ctx,
			`INSERT INTO owner_ids (subject, own_id) VALUES ($1, $2)
			 ON CONFLICT (subject) DO UPDATE SET subject = EXCLUDED.subject
			 RETURNING own_id`,
			subject, ids.New(ids.Owner),
		).Scan(&ownID); err != nil {
			return nil, classify("owner_id", err)
		}
		out[subject] = ownID
	}
	return out, nil
}
