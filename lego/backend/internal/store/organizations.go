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
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// organizations.go is the source of truth for Render's organizations (w7/m464,
// docs/ADR115-organizations.md): an org holds its primary workspace and any
// number of secondaries on the primary's plan, and gives each person one org
// role. Every write locks the rows it reads (the organization row, or the
// tenant when no org exists yet) the way UpdateTenantPlan locks its tenant,
// so two changes at once cannot both pass a check the other invalidates. The
// OpenFGA tuples follow these rows; the callers write them.

// The org roles (organization_members.role), core's vocabulary.
const (
	OrgRoleOwner  = core.OrgRoleOwner
	OrgRoleMember = core.OrgRoleMember
	OrgRoleGuest  = core.OrgRoleGuest
)

// OrgRoles is every org role in rank order.
var OrgRoles = []string{OrgRoleOwner, OrgRoleMember, OrgRoleGuest}

// The workspace privacy settings (tenants.privacy). Render: "Newly added
// workspaces are invite-only by default."
const (
	PrivacyInviteOnly = "invite_only"
	PrivacyPublic     = "public"
)

// The org refusals the store decides under its locks. Each wraps ErrConflict
// so an unmapped caller still answers 409.
var (
	// ErrLastOwner refuses removing or demoting an org's last Owner.
	ErrLastOwner = fmt.Errorf("organization would have no owner: %w", ErrConflict)
	// ErrInOrganization refuses attaching or upgrading a workspace that
	// already belongs to an organization.
	ErrInOrganization = fmt.Errorf("workspace already belongs to an organization: %w", ErrConflict)
	// ErrOrganizationHasWorkspaces refuses dissolving an org that still holds
	// a workspace besides its primary.
	ErrOrganizationHasWorkspaces = fmt.Errorf("organization still has secondary workspaces: %w", ErrConflict)
	// ErrGuestSingleWorkspace refuses a Guest a membership of a second org
	// workspace (Render: a Guest "can access only the single workspace they're
	// invited to").
	ErrGuestSingleWorkspace = fmt.Errorf("a guest belongs to one organization workspace: %w", ErrConflict)
)

// Organization is a row of organizations with its plan, which is its
// primary workspace's.
type Organization struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	PrimaryWorkspaceID string    `json:"primaryWorkspaceId"`
	Plan               string    `json:"plan"`
	CreatedAt          time.Time `json:"createdAt"`
	AuditVisibleFrom   time.Time `json:"auditVisibleFrom"`
	// RequireTwoFactor and RequireGoogleLogin are the org's login settings
	// (w7/m466, migration 0295): every org workspace enforces each beside its
	// own, and so do the org's own objects.
	RequireTwoFactor   bool `json:"requireTwoFactor"`
	RequireGoogleLogin bool `json:"requireGoogleLogin"`
}

// OrganizationMember is a person's org role.
type OrganizationMember struct {
	OrganizationID string    `json:"organizationId"`
	Subject        string    `json:"subject"`
	Role           string    `json:"role"`
	CreatedAt      time.Time `json:"createdAt"`
}

// OrganizationMembership is one org a subject belongs to, with their role.
type OrganizationMembership struct {
	Organization
	Role string `json:"role"`
}

// OrganizationInvite is an Owner's invite to the org by email. Token is the
// plaintext capability in flight only (set by CreateOrganizationInvite); the
// row stores its sha256.
type OrganizationInvite struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Email          string     `json:"email"`
	Role           string     `json:"role"`
	Token          string     `json:"-"`
	InvitedBy      string     `json:"invitedBy,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	AcceptedAt     *time.Time `json:"acceptedAt,omitempty"`
}

// OrganizationTuples is the OpenFGA state one org implies: each member's role
// tuple, each workspace's org link, and the public link of each Public one.
type OrganizationTuples struct {
	Organization Organization
	Members      []OrganizationMember
	Workspaces   []Tenant
}

const organizationColumns = `o.id, o.name, o.primary_workspace_id, p.plan, o.created_at, o.audit_visible_from, o.require_two_factor, o.require_google_login`

const organizationFrom = ` FROM organizations o JOIN tenants p ON p.id = o.primary_workspace_id`

func scanOrganization(row pgx.Row, extra ...any) (Organization, error) {
	var o Organization
	dest := append([]any{&o.ID, &o.Name, &o.PrimaryWorkspaceID, &o.Plan, &o.CreatedAt, &o.AuditVisibleFrom, &o.RequireTwoFactor, &o.RequireGoogleLogin}, extra...)
	return o, row.Scan(dest...)
}

// GetOrganization reads one org (ErrNotFound when absent).
func (s *PGStore) GetOrganization(ctx context.Context, id string) (Organization, error) {
	o, err := scanOrganization(s.Pool.QueryRow(ctx, `SELECT `+organizationColumns+organizationFrom+` WHERE o.id = $1`, id))
	if err != nil {
		return Organization{}, classify("organization", err)
	}
	return o, nil
}

// ListOrganizationsForSubject returns the orgs subject belongs to, oldest
// first, each with subject's role.
func (s *PGStore) ListOrganizationsForSubject(ctx context.Context, subject string) ([]OrganizationMembership, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+organizationColumns+`, m.role`+organizationFrom+`
		JOIN organization_members m ON m.organization_id = o.id
		WHERE m.subject = $1 ORDER BY o.created_at, o.id`, subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OrganizationMembership
	for rows.Next() {
		var m OrganizationMembership
		if m.Organization, err = scanOrganization(rows, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetOrganizationMember reads subject's org role (ErrNotFound when they are
// not in the org).
func (s *PGStore) GetOrganizationMember(ctx context.Context, orgID, subject string) (OrganizationMember, error) {
	var m OrganizationMember
	err := s.Pool.QueryRow(ctx, `SELECT organization_id, subject, role, created_at FROM organization_members
		WHERE organization_id = $1 AND subject = $2`, orgID, subject).Scan(&m.OrganizationID, &m.Subject, &m.Role, &m.CreatedAt)
	if err != nil {
		return OrganizationMember{}, classify("organization member", err)
	}
	return m, nil
}

// ListOrganizationMembers returns an org's people, Owners first, then by when
// they joined.
func (s *PGStore) ListOrganizationMembers(ctx context.Context, orgID string) ([]OrganizationMember, error) {
	return listOrganizationMembers(ctx, s.Pool, orgID)
}

func listOrganizationMembers(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, orgID string) ([]OrganizationMember, error) {
	rows, err := q.Query(ctx, `SELECT organization_id, subject, role, created_at FROM organization_members
		WHERE organization_id = $1
		ORDER BY CASE role WHEN 'owner' THEN 0 WHEN 'member' THEN 1 ELSE 2 END, created_at, subject`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OrganizationMember
	for rows.Next() {
		var m OrganizationMember
		if err := rows.Scan(&m.OrganizationID, &m.Subject, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListOrganizationWorkspaces returns an org's workspaces, the primary first.
func (s *PGStore) ListOrganizationWorkspaces(ctx context.Context, orgID string) ([]Tenant, error) {
	return listOrganizationWorkspaces(ctx, s.Pool, orgID)
}

func listOrganizationWorkspaces(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, orgID string) ([]Tenant, error) {
	rows, err := q.Query(ctx, `SELECT `+tenantColumnsT+` FROM tenants t
		JOIN organizations o ON o.id = t.organization_id
		WHERE t.organization_id = $1
		ORDER BY (t.id = o.primary_workspace_id) DESC, t.created_at, t.id`, orgID)
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

// SubjectOrganizationWorkspaces returns the ids of the org's workspaces
// subject is a member of: a Guest's one workspace, and what an org removal
// has to end.
func (s *PGStore) SubjectOrganizationWorkspaces(ctx context.Context, orgID, subject string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT t.id FROM tenants t JOIN tenant_members m ON m.tenant_id = t.id
		WHERE t.organization_id = $1 AND m.subject = $2 AND m.kind = 'user' ORDER BY t.created_at, t.id`, orgID, subject)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// OrganizationMemberWorkspaces maps each person in the org's workspaces to
// the ids of the org workspaces they belong to, in one read.
func (s *PGStore) OrganizationMemberWorkspaces(ctx context.Context, orgID string) (map[string][]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT m.subject, t.id FROM tenant_members m JOIN tenants t ON t.id = m.tenant_id
		WHERE t.organization_id = $1 AND m.kind = 'user' ORDER BY t.created_at, t.id`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var subject, id string
		if err := rows.Scan(&subject, &id); err != nil {
			return nil, err
		}
		out[subject] = append(out[subject], id)
	}
	return out, rows.Err()
}

// OrganizationRole reads an org and subject's role in it in one read:
// ErrNotFound when the org does not exist or subject is not in it.
func (s *PGStore) OrganizationRole(ctx context.Context, orgID, subject string) (Organization, string, error) {
	var role string
	o, err := scanOrganization(s.Pool.QueryRow(ctx, `SELECT `+organizationColumns+`, m.role`+organizationFrom+`
		JOIN organization_members m ON m.organization_id = o.id AND m.subject = $2
		WHERE o.id = $1`, orgID, subject), &role)
	if err != nil {
		return Organization{}, "", classify("organization", err)
	}
	return o, role, nil
}

// lockOrganization takes the org row's lock for the rest of the transaction
// and returns the org (ErrNotFound when absent).
func lockOrganization(ctx context.Context, tx pgx.Tx, orgID string) (Organization, error) {
	if _, err := tx.Exec(ctx, `SELECT 1 FROM organizations WHERE id = $1 FOR UPDATE`, orgID); err != nil {
		return Organization{}, err
	}
	return scanOrganization(tx.QueryRow(ctx, `SELECT `+organizationColumns+organizationFrom+` WHERE o.id = $1`, orgID))
}

// UpgradeToOrganization moves a standalone workspace onto plan (Scale or
// Enterprise) and makes it the primary of a new org in one transaction
// (ADR115 D2). owners become Owners; every other person in the workspace
// becomes a Member ("all other workspace members become org members",
// render.com/docs/organizations). An owners list that names no person in the
// workspace leaves every user admin as Owner instead. A workspace already in
// an org is ErrInOrganization.
func (s *PGStore) UpgradeToOrganization(ctx context.Context, workspaceID, plan string, owners []string) (Tenant, Organization, []OrganizationMember, error) {
	var t Tenant
	var o Organization
	var members []OrganizationMember
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var orgID *string
		var name string
		if err := tx.QueryRow(ctx, `SELECT organization_id, COALESCE(NULLIF(display_name, ''), name) FROM tenants WHERE id = $1 FOR UPDATE`, workspaceID).Scan(&orgID, &name); err != nil {
			return err
		}
		if orgID != nil {
			return ErrInOrganization
		}
		var err error
		if t, err = updateTenantPlanTx(ctx, tx, workspaceID, plan); err != nil {
			return err
		}
		id := ids.New(ids.Organization)
		if _, err := tx.Exec(ctx, `INSERT INTO organizations (id, name, primary_workspace_id) VALUES ($1, $2, $3)`, id, name, workspaceID); err != nil {
			return err
		}
		if t, err = scanTenant(tx.QueryRow(ctx, `UPDATE tenants SET organization_id = $2, updated_at = now() WHERE id = $1 RETURNING `+tenantColumns, workspaceID, id)); err != nil {
			return err
		}
		people, err := tx.Query(ctx, `SELECT subject, role FROM tenant_members WHERE tenant_id = $1 AND kind = 'user' ORDER BY created_at, subject`, workspaceID)
		if err != nil {
			return err
		}
		type person struct{ subject, role string }
		var inWorkspace []person
		for people.Next() {
			var p person
			if err := people.Scan(&p.subject, &p.role); err != nil {
				people.Close()
				return err
			}
			inWorkspace = append(inWorkspace, p)
		}
		people.Close()
		if err := people.Err(); err != nil {
			return err
		}
		ownerSet := map[string]bool{}
		for _, p := range inWorkspace {
			if slices.Contains(owners, p.subject) {
				ownerSet[p.subject] = true
			}
		}
		if len(ownerSet) == 0 {
			for _, p := range inWorkspace {
				if p.role == "admin" {
					ownerSet[p.subject] = true
				}
			}
		}
		for _, p := range inWorkspace {
			role := OrgRoleMember
			if ownerSet[p.subject] {
				role = OrgRoleOwner
			}
			if _, err := tx.Exec(ctx, `INSERT INTO organization_members (organization_id, subject, role) VALUES ($1, $2, $3)`, id, p.subject, role); err != nil {
				return err
			}
		}
		if o, err = lockOrganization(ctx, tx, id); err != nil {
			return err
		}
		members, err = listOrganizationMembers(ctx, tx, id)
		return err
	})
	if err != nil {
		return Tenant{}, Organization{}, nil, classify("workspace", err)
	}
	return t, o, members, nil
}

// ChangeOrganizationPlan writes plan on every workspace of the org in one
// transaction (ADR115 D2: a change on the primary between Scale and
// Enterprise moves the whole org). It returns the org's workspaces after it.
func (s *PGStore) ChangeOrganizationPlan(ctx context.Context, orgID, plan string) ([]Tenant, error) {
	var out []Tenant
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := lockOrganization(ctx, tx, orgID); err != nil {
			return err
		}
		workspaces, err := listOrganizationWorkspaces(ctx, tx, orgID)
		if err != nil {
			return err
		}
		for _, w := range workspaces {
			t, err := updateTenantPlanTx(ctx, tx, w.ID, plan)
			if err != nil {
				return err
			}
			out = append(out, t)
		}
		return nil
	})
	if err != nil {
		return nil, classify("organization", err)
	}
	return out, nil
}

// DissolveOrganization deletes the org and moves its primary onto plan in one
// transaction (ADR115 D6: "Downgrading removes your organization"; the primary
// "continues as a standalone workspace on the new plan"). It refuses an org
// that still holds a secondary (ErrOrganizationHasWorkspaces). The primary's
// own members are unchanged; the caller removed the org's tuples first.
func (s *PGStore) DissolveOrganization(ctx context.Context, orgID, plan string) (Tenant, error) {
	var t Tenant
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		o, err := lockOrganization(ctx, tx, orgID)
		if err != nil {
			return err
		}
		var others int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM tenants WHERE organization_id = $1 AND id <> $2`, orgID, o.PrimaryWorkspaceID).Scan(&others); err != nil {
			return err
		}
		if others > 0 {
			return ErrOrganizationHasWorkspaces
		}
		if _, err := tx.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, orgID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE tenants SET privacy = $2 WHERE id = $1`, o.PrimaryWorkspaceID, PrivacyInviteOnly); err != nil {
			return err
		}
		t, err = updateTenantPlanTx(ctx, tx, o.PrimaryWorkspaceID, plan)
		return err
	})
	if err != nil {
		return Tenant{}, classify("organization", err)
	}
	return t, nil
}

// CreateOrganizationWorkspace inserts a workspace in the org on the org's
// plan with privacy, and makes creator its first admin (ADR115 D3: "Render
// creates the workspace and adds you as its first admin").
func (s *PGStore) CreateOrganizationWorkspace(ctx context.Context, orgID, name, privacy, creator string) (Tenant, error) {
	var t Tenant
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockSubjectMembership(ctx, tx, creator); err != nil {
			return err
		}
		if err := refuseDeletingSubject(ctx, tx, creator); err != nil {
			return err
		}
		o, err := lockOrganization(ctx, tx, orgID)
		if err != nil {
			return err
		}
		if t, err = scanTenant(tx.QueryRow(ctx,
			`INSERT INTO tenants (id, name, plan, organization_id, privacy) VALUES ($1, $2, $3, $4, $5) RETURNING `+tenantColumns,
			ids.New(ids.Workspace), name, o.Plan, orgID, privacy)); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, 'admin')`, t.ID, creator)
		return err
	})
	if err != nil {
		return Tenant{}, classify("workspace", err)
	}
	return t, nil
}

// AttachWorkspaceToOrganization moves a standalone workspace into the org:
// Render support's transfer ("Each member of each transferred workspace
// becomes a member of your org"). Its plan becomes the org's and its privacy
// Invite-only; each of its people not yet in the org becomes a Member, and
// their subjects are returned. A workspace already in an org (including a
// primary) is ErrInOrganization.
func (s *PGStore) AttachWorkspaceToOrganization(ctx context.Context, workspaceID, orgID string) (Tenant, []string, error) {
	var t Tenant
	var added []string
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		o, err := lockOrganization(ctx, tx, orgID)
		if err != nil {
			return err
		}
		var current *string
		if err := tx.QueryRow(ctx, `SELECT organization_id FROM tenants WHERE id = $1 FOR UPDATE`, workspaceID).Scan(&current); err != nil {
			return err
		}
		if current != nil {
			return ErrInOrganization
		}
		if _, err := updateTenantPlanTx(ctx, tx, workspaceID, o.Plan); err != nil {
			return err
		}
		if t, err = scanTenant(tx.QueryRow(ctx, `UPDATE tenants SET organization_id = $2, privacy = $3, updated_at = now() WHERE id = $1 RETURNING `+tenantColumns,
			workspaceID, orgID, PrivacyInviteOnly)); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `INSERT INTO organization_members (organization_id, subject, role)
			SELECT $1, subject, 'member' FROM tenant_members WHERE tenant_id = $2 AND kind = 'user'
			ON CONFLICT (organization_id, subject) DO NOTHING RETURNING subject`, orgID, workspaceID)
		if err != nil {
			return err
		}
		added, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	if err != nil {
		return Tenant{}, nil, classify("workspace", err)
	}
	return t, added, nil
}

// SetWorkspacePrivacy sets an org workspace's privacy. changed is false when
// it already held that value. A standalone workspace is ErrInvalid: privacy is
// an org setting.
func (s *PGStore) SetWorkspacePrivacy(ctx context.Context, workspaceID, privacy string) (t Tenant, changed bool, err error) {
	t, err = scanTenant(s.Pool.QueryRow(ctx, `UPDATE tenants SET privacy = $2, updated_at = now()
		WHERE id = $1 AND organization_id IS NOT NULL AND privacy <> $2 RETURNING `+tenantColumns, workspaceID, privacy))
	if errors.Is(err, pgx.ErrNoRows) {
		t, err = s.GetTenant(ctx, workspaceID)
		if err == nil && t.OrganizationID == "" {
			err = fmt.Errorf("workspace %s is not in an organization: %w", workspaceID, ErrInvalid)
		}
		return t, false, err
	}
	if err != nil {
		return Tenant{}, false, classify("workspace", err)
	}
	return t, true, nil
}

// RenameOrganization sets an org's name.
func (s *PGStore) RenameOrganization(ctx context.Context, orgID, name string) (Organization, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE organizations SET name = $2 WHERE id = $1`, orgID, name)
	if err != nil {
		return Organization{}, classify("organization", err)
	}
	if tag.RowsAffected() == 0 {
		return Organization{}, fmt.Errorf("organization %s: %w", orgID, ErrNotFound)
	}
	return s.GetOrganization(ctx, orgID)
}

// SetOrganizationRequireTwoFactor sets the org's "require two-factor
// authentication" setting (w7/m466). changed is false when it already held
// that value (or the org does not exist: the caller has read it), so a no-op
// writes and records nothing, as SetTenantRequireTwoFactor's conditional write
// does for a workspace.
func (s *PGStore) SetOrganizationRequireTwoFactor(ctx context.Context, orgID string, enabled bool) (changed bool, err error) {
	return s.setOrganizationLoginRequirement(ctx, `UPDATE organizations SET require_two_factor = $2
		 WHERE id = $1 AND require_two_factor <> $2`, orgID, enabled)
}

// SetOrganizationRequireGoogleLogin is SetOrganizationRequireTwoFactor for
// the "require Google login" setting.
func (s *PGStore) SetOrganizationRequireGoogleLogin(ctx context.Context, orgID string, enabled bool) (changed bool, err error) {
	return s.setOrganizationLoginRequirement(ctx, `UPDATE organizations SET require_google_login = $2
		 WHERE id = $1 AND require_google_login <> $2`, orgID, enabled)
}

func (s *PGStore) setOrganizationLoginRequirement(ctx context.Context, update, orgID string, enabled bool) (bool, error) {
	tag, err := s.Pool.Exec(ctx, update, orgID, enabled)
	if err != nil {
		return false, classify("organization", err)
	}
	return tag.RowsAffected() > 0, nil // false: already there, or no such org
}

// OrganizationLoginRequirements reads only an org's own login settings: the
// enforcement point's question for the org's own objects (org:<id>), asked
// at most once a minute per org (internal/twofactor). An org that does not
// exist requires nothing: the zero value, no error.
func (s *PGStore) OrganizationLoginRequirements(ctx context.Context, orgID string) (core.LoginRequirements, error) {
	var name string
	var twoFactor, google bool
	err := s.Pool.QueryRow(ctx, `SELECT name, require_two_factor, require_google_login FROM organizations WHERE id = $1`, orgID).
		Scan(&name, &twoFactor, &google)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.LoginRequirements{}, nil
	}
	if err != nil {
		return core.LoginRequirements{}, classify("organization", err)
	}
	return core.LoginRequirements{}.WithOrganization(orgID, name, twoFactor, google), nil
}

// SetOrganizationMemberRole changes subject's org role and returns the old
// one. It refuses demoting the last Owner (ErrLastOwner) and making a Guest of
// someone in more than one org workspace (ErrGuestSingleWorkspace).
func (s *PGStore) SetOrganizationMemberRole(ctx context.Context, orgID, subject, role string) (string, error) {
	var old string
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := lockOrganization(ctx, tx, orgID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT role FROM organization_members WHERE organization_id = $1 AND subject = $2 FOR UPDATE`, orgID, subject).Scan(&old); err != nil {
			return err
		}
		if old == role {
			return nil
		}
		if old == OrgRoleOwner {
			if err := refuseLastOwner(ctx, tx, orgID); err != nil {
				return err
			}
		}
		if role == OrgRoleGuest {
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM tenant_members m JOIN tenants t ON t.id = m.tenant_id
				WHERE t.organization_id = $1 AND m.subject = $2 AND m.kind = 'user'`, orgID, subject).Scan(&n); err != nil {
				return err
			}
			if n > 1 {
				return ErrGuestSingleWorkspace
			}
		}
		_, err := tx.Exec(ctx, `UPDATE organization_members SET role = $3 WHERE organization_id = $1 AND subject = $2`, orgID, subject, role)
		return err
	})
	if err != nil {
		return "", classify("organization member", err)
	}
	return old, nil
}

func refuseLastOwner(ctx context.Context, tx pgx.Tx, orgID string) error {
	var owners int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM organization_members WHERE organization_id = $1 AND role = 'owner'`, orgID).Scan(&owners); err != nil {
		return err
	}
	if owners <= 1 {
		return ErrLastOwner
	}
	return nil
}

// RemoveOrganizationMember deletes subject's org role (ErrNotFound when they
// have none), refusing the last Owner. The caller ends their workspace
// memberships first, through the members teardown.
func (s *PGStore) RemoveOrganizationMember(ctx context.Context, orgID, subject string) error {
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := lockOrganization(ctx, tx, orgID); err != nil {
			return err
		}
		var role string
		if err := tx.QueryRow(ctx, `SELECT role FROM organization_members WHERE organization_id = $1 AND subject = $2 FOR UPDATE`, orgID, subject).Scan(&role); err != nil {
			return err
		}
		if role == OrgRoleOwner {
			if err := refuseLastOwner(ctx, tx, orgID); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `DELETE FROM organization_members WHERE organization_id = $1 AND subject = $2`, orgID, subject)
		return err
	})
	return classify("organization member", err)
}

// OrganizationBillingHolders returns the people holding the Billing role in
// any of the org's workspaces and the emails of pending Billing invites to
// them: Render's "up to two total members with the Billing role" counts both.
func (s *PGStore) OrganizationBillingHolders(ctx context.Context, orgID string) (subjects, inviteEmails []string, err error) {
	rows, err := s.Pool.Query(ctx, `SELECT DISTINCT m.subject FROM tenant_members m JOIN tenants t ON t.id = m.tenant_id
		WHERE t.organization_id = $1 AND m.role = 'billing' AND m.kind = 'user' ORDER BY m.subject`, orgID)
	if err != nil {
		return nil, nil, err
	}
	if subjects, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
		return nil, nil, err
	}
	rows, err = s.Pool.Query(ctx, `SELECT DISTINCT i.email FROM tenant_invites i JOIN tenants t ON t.id = i.tenant_id
		WHERE t.organization_id = $1 AND i.role = 'billing' AND i.accepted_at IS NULL AND i.expires_at > now() ORDER BY i.email`, orgID)
	if err != nil {
		return nil, nil, err
	}
	inviteEmails, err = pgx.CollectRows(rows, pgx.RowTo[string])
	return subjects, inviteEmails, err
}

// AllOrganizationTuples returns every org with its members and workspaces:
// the state bex-api's startup reconcile writes into OpenFGA.
func (s *PGStore) AllOrganizationTuples(ctx context.Context) ([]OrganizationTuples, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+organizationColumns+organizationFrom+` ORDER BY o.created_at, o.id`)
	if err != nil {
		return nil, err
	}
	var orgs []Organization
	for rows.Next() {
		o, err := scanOrganization(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		orgs = append(orgs, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]OrganizationTuples, 0, len(orgs))
	for _, o := range orgs {
		members, err := listOrganizationMembers(ctx, s.Pool, o.ID)
		if err != nil {
			return nil, err
		}
		workspaces, err := listOrganizationWorkspaces(ctx, s.Pool, o.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, OrganizationTuples{Organization: o, Members: members, Workspaces: workspaces})
	}
	return out, nil
}

// CreateOrganizationInvite records a pending org invite (ErrConflict when one
// already targets the same email). Only sha256(token) is stored.
func (s *PGStore) CreateOrganizationInvite(ctx context.Context, orgID, email, role, token, invitedBy string, expiresAt time.Time) (OrganizationInvite, error) {
	inv := OrganizationInvite{
		ID: ids.New(ids.Invite), OrganizationID: orgID, Email: email, Role: role,
		Token: token, InvitedBy: invitedBy, ExpiresAt: expiresAt,
	}
	err := s.Pool.QueryRow(ctx, `INSERT INTO organization_invites (id, organization_id, email, role, token_hash, invited_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING created_at`,
		inv.ID, orgID, email, role, hashInviteToken(token), invitedBy, expiresAt).Scan(&inv.CreatedAt)
	if err != nil {
		return OrganizationInvite{}, classify("organization invite", err)
	}
	return inv, nil
}

const organizationInviteColumns = `id, organization_id, email, role, invited_by, created_at, expires_at, accepted_at`

func scanOrganizationInvite(row pgx.Row) (OrganizationInvite, error) {
	var inv OrganizationInvite
	err := row.Scan(&inv.ID, &inv.OrganizationID, &inv.Email, &inv.Role, &inv.InvitedBy, &inv.CreatedAt, &inv.ExpiresAt, &inv.AcceptedAt)
	return inv, err
}

// ListOrganizationInvites returns an org's outstanding invites, oldest first.
func (s *PGStore) ListOrganizationInvites(ctx context.Context, orgID string) ([]OrganizationInvite, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+organizationInviteColumns+` FROM organization_invites
		WHERE organization_id = $1 AND accepted_at IS NULL AND expires_at > now() ORDER BY created_at, id`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OrganizationInvite
	for rows.Next() {
		inv, err := scanOrganizationInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// DeleteOrganizationInvite revokes a pending org invite (ErrNotFound when it
// is not this org's outstanding invite).
func (s *PGStore) DeleteOrganizationInvite(ctx context.Context, orgID, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM organization_invites WHERE organization_id = $1 AND id = $2 AND accepted_at IS NULL`, orgID, id)
	if err != nil {
		return classify("organization invite", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("organization invite %s: %w", id, ErrNotFound)
	}
	return nil
}

// GetOrganizationInviteByToken reads an org invite by its emailed token
// without redeeming it (ErrNotFound when no invite has it). Accepted rows stay
// readable, so someone already in the org can reopen it from the link.
func (s *PGStore) GetOrganizationInviteByToken(ctx context.Context, token string) (OrganizationInvite, error) {
	inv, err := scanOrganizationInvite(s.Pool.QueryRow(ctx, `SELECT `+organizationInviteColumns+` FROM organization_invites
		WHERE token_hash = $1`, hashInviteToken(token)))
	if err != nil {
		return OrganizationInvite{}, classify("organization invite", err)
	}
	return inv, nil
}

// AcceptOrganizationInviteByToken redeems one org invite by its emailed token
// for subject, whatever email they signed in with (w7/083): the token is the
// capability, as a workspace invite's is (AcceptInviteByToken). An unknown
// token is ErrNotFound, a used one ErrInviteAlreadyAccepted, a lapsed one
// ErrInviteExpired. Someone already in the org keeps their role; the returned
// invite carries the role they hold, and joined says whether they are new.
func (s *PGStore) AcceptOrganizationInviteByToken(ctx context.Context, token, subject string) (inv OrganizationInvite, joined bool, err error) {
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockSubjectMembership(ctx, tx, subject); err != nil {
			return err
		}
		if err := refuseDeletingSubject(ctx, tx, subject); err != nil {
			return err
		}
		if inv, err = scanOrganizationInvite(tx.QueryRow(ctx, `SELECT `+organizationInviteColumns+` FROM organization_invites
			WHERE token_hash = $1 FOR UPDATE`, hashInviteToken(token))); err != nil {
			return err
		}
		switch {
		case inv.AcceptedAt != nil:
			return ErrInviteAlreadyAccepted
		case !inv.ExpiresAt.After(time.Now()):
			return ErrInviteExpired
		}
		tag, err := tx.Exec(ctx, `INSERT INTO organization_members (organization_id, subject, role) VALUES ($1, $2, $3)
			ON CONFLICT (organization_id, subject) DO NOTHING`, inv.OrganizationID, subject, inv.Role)
		if err != nil {
			return err
		}
		if joined = tag.RowsAffected() == 1; !joined {
			if err := tx.QueryRow(ctx, `SELECT role FROM organization_members WHERE organization_id = $1 AND subject = $2`,
				inv.OrganizationID, subject).Scan(&inv.Role); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE organization_invites SET accepted_at = now() WHERE id = $1`, inv.ID)
		return err
	})
	if err != nil {
		return OrganizationInvite{}, false, classify("organization invite", err)
	}
	return inv, joined, nil
}

// joinOrganizationOnRedeem is a workspace invite's org half, inside its
// redemption transaction (ADR115 D4). Render without SSO: "workspace admins
// can invite any Render user to any org-managed workspace. These users
// automatically become standard members of the org." It returns the org id
// when subject became a new Member, "" otherwise (a standalone workspace, or
// someone already in the org, whose role is unchanged).
func joinOrganizationOnRedeem(ctx context.Context, tx pgx.Tx, tenantID, subject string) (string, error) {
	var orgID string
	err := tx.QueryRow(ctx, `INSERT INTO organization_members (organization_id, subject, role)
		SELECT organization_id, $2, 'member' FROM tenants WHERE id = $1 AND organization_id IS NOT NULL
		ON CONFLICT (organization_id, subject) DO NOTHING RETURNING organization_id`, tenantID, subject).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return orgID, err
}

// guestElsewhere reports whether subject is a Guest of the workspace's org
// who already belongs to another of its workspaces: redeeming an invite would
// give a Guest a second workspace (ErrGuestSingleWorkspace).
func guestElsewhere(ctx context.Context, tx pgx.Tx, tenantID, subject string) (bool, error) {
	var blocked bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM tenants w
		JOIN organization_members g ON g.organization_id = w.organization_id AND g.subject = $2 AND g.role = 'guest'
		JOIN tenants t ON t.organization_id = w.organization_id AND t.id <> w.id
		JOIN tenant_members m ON m.tenant_id = t.id AND m.subject = $2 AND m.kind = 'user'
		WHERE w.id = $1)`, tenantID, subject).Scan(&blocked)
	return blocked, err
}

// JoinOrganizationWorkspace adds subject to an org workspace at role: a
// Member joining a Public workspace as Developer, or an Owner joining any as
// admin (ADR115 D3). The caller has authorized the join. A subject already in
// the workspace keeps their role, and joined is false.
func (s *PGStore) JoinOrganizationWorkspace(ctx context.Context, workspaceID, subject, role string) (joined bool, err error) {
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockSubjectMembership(ctx, tx, subject); err != nil {
			return err
		}
		if err := refuseDeletingSubject(ctx, tx, subject); err != nil {
			return err
		}
		if err := refuseDeletingTenant(ctx, tx, workspaceID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, $3)
			ON CONFLICT (tenant_id, subject) DO NOTHING`, workspaceID, subject, role)
		if err != nil {
			return err
		}
		if joined = tag.RowsAffected() == 1; !joined {
			return nil
		}
		return enqueueRoleReconciliation(ctx, tx, workspaceID, subject, role)
	})
	if err != nil {
		return false, classify("tenant_member", err)
	}
	return joined, nil
}

// AcceptOrganizationInvitesForEmail redeems every outstanding org invite
// addressed to email for subject, on their sign-in (the workspace invites'
// login-time match, AcceptInvitesForEmail). Someone already in an org keeps
// their role. It returns the invites that made subject a new org member,
// each carrying the role they got.
func (s *PGStore) AcceptOrganizationInvitesForEmail(ctx context.Context, email, subject string) ([]OrganizationInvite, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+organizationInviteColumns+` FROM organization_invites
		WHERE email = $1 AND accepted_at IS NULL AND expires_at > now()`, email)
	if err != nil {
		return nil, classify("organization invite", err)
	}
	pending, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (OrganizationInvite, error) { return scanOrganizationInvite(row) })
	if err != nil {
		return nil, classify("organization invite", err)
	}
	if len(pending) == 0 {
		return nil, nil
	}
	var joined []OrganizationInvite
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockSubjectMembership(ctx, tx, subject); err != nil {
			return err
		}
		if err := refuseDeletingSubject(ctx, tx, subject); err != nil {
			return err
		}
		for _, inv := range pending {
			tag, err := tx.Exec(ctx, `INSERT INTO organization_members (organization_id, subject, role) VALUES ($1, $2, $3)
				ON CONFLICT (organization_id, subject) DO NOTHING`, inv.OrganizationID, subject, inv.Role)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE organization_invites SET accepted_at = now() WHERE id = $1`, inv.ID); err != nil {
				return err
			}
			if tag.RowsAffected() == 1 {
				joined = append(joined, inv)
			}
		}
		return nil
	})
	if err != nil {
		return nil, classify("organization invite", err)
	}
	return joined, nil
}
