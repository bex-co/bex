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
	"github.com/jackc/pgx/v5/pgconn"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Sandbox groups (w22/m311, docs/ADR042-sandbox-cluster-substrate.md
// § Sandbox groups): the unit a Render sandbox is created in and a concurrency
// limit applies to.

// DefaultSandboxGroupName is what a workspace's lazily created default group is
// called; Render's OpenAPI example names it "Default".
const DefaultSandboxGroupName = "Default"

// SandboxGroupRegion is the one region Render runs sandboxes in
// (render.com/docs/sandboxes-limits, read 2026-11-10). bex accepts it on a
// group create and reads it only on a deployment that names no region of its
// own (sandbox.Service.region, w32/016); placement ignores it.
const SandboxGroupRegion = "oregon"

var (
	// ErrSandboxAdmissionBusy is an admission that could not get its workspace's
	// lock in time: another create or resume of the workspace held it, which only
	// a hung OpenSandbox read makes long. The caller retries.
	ErrSandboxAdmissionBusy = errors.New("sandbox admission busy")
	// ErrSandboxGroupNameTaken is a create whose name another group of the
	// workspace already has (Render answers 409).
	ErrSandboxGroupNameTaken = errors.New("sandbox group name already taken")
	// ErrSandboxGroupLimit is a create past the workspace's group cap (Render's
	// `sandbox_group_limit_reached`).
	ErrSandboxGroupLimit = errors.New("sandbox group limit reached")
)

// SandboxGroup is one row of sandbox_groups. ConcurrencyLimit is the configured
// value; the effective limit the API reports is that capped by the platform's
// maximum, which the sandbox service applies.
type SandboxGroup struct {
	ID               string
	WorkspaceID      string
	Name             string
	Region           string
	ConcurrencyLimit int
	IsDefault        bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// SandboxGroupMember binds one sandbox to its group. LeaseUntil is the end of
// the slot reservation an admission wrote (nil when none).
type SandboxGroupMember struct {
	GroupID    string
	LeaseUntil *time.Time
}

const sandboxGroupColumns = `id, workspace_id, name, region, concurrency_limit, is_default, created_at, updated_at`

func scanSandboxGroup(row pgx.Row) (SandboxGroup, error) {
	var g SandboxGroup
	err := row.Scan(&g.ID, &g.WorkspaceID, &g.Name, &g.Region, &g.ConcurrencyLimit, &g.IsDefault, &g.CreatedAt, &g.UpdatedAt)
	return g, err
}

// lockSandboxGroups serializes a workspace's group creates, so the cap and the
// "first group is the default" rule are checked against a count no concurrent
// create moves. It is a different lock domain from the admission lock below.
func lockSandboxGroups(ctx context.Context, tx pgx.Tx, workspaceID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "sandbox-groups:"+workspaceID)
	return err
}

// CreateSandboxGroup adds a group. The workspace's first group becomes its
// default; a create past maxGroups (0 disables the cap) is ErrSandboxGroupLimit
// and a name another group has is ErrSandboxGroupNameTaken.
func (s *PGStore) CreateSandboxGroup(ctx context.Context, workspaceID, name, region string, concurrencyLimit, maxGroups int) (SandboxGroup, error) {
	var out SandboxGroup
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockSandboxGroups(ctx, tx, workspaceID); err != nil {
			return err
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM sandbox_groups WHERE workspace_id = $1`, workspaceID).Scan(&count); err != nil {
			return err
		}
		if maxGroups > 0 && count >= maxGroups {
			return ErrSandboxGroupLimit
		}
		g, err := insertSandboxGroup(ctx, tx, workspaceID, name, region, concurrencyLimit, count == 0)
		out = g
		return err
	})
	return out, err
}

func insertSandboxGroup(ctx context.Context, tx pgx.Tx, workspaceID, name, region string, concurrencyLimit int, isDefault bool) (SandboxGroup, error) {
	g, err := scanSandboxGroup(tx.QueryRow(ctx, `
		INSERT INTO sandbox_groups (id, workspace_id, name, region, concurrency_limit, is_default)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+sandboxGroupColumns,
		ids.New(ids.SandboxGroup), workspaceID, name, region, concurrencyLimit, isDefault))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "sandbox_groups_workspace_name_key" {
		return SandboxGroup{}, ErrSandboxGroupNameTaken
	}
	if err != nil {
		return SandboxGroup{}, classify("sandbox group", err)
	}
	return g, nil
}

// EnsureDefaultSandboxGroup returns the workspace's default group, creating it
// (named "Default") when the workspace has no group yet. This is how every
// workspace gets exactly one default without a creation hook or a backfill:
// Render does the same on a sandbox create before any group exists. A workspace
// with no group is below any group cap, so this never answers
// ErrSandboxGroupLimit.
func (s *PGStore) EnsureDefaultSandboxGroup(ctx context.Context, workspaceID string, concurrencyLimit int) (SandboxGroup, error) {
	// The common case is a read; take the lock only to create.
	if g, err := s.defaultSandboxGroup(ctx, workspaceID); err == nil {
		return g, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return SandboxGroup{}, err
	}
	var out SandboxGroup
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockSandboxGroups(ctx, tx, workspaceID); err != nil {
			return err
		}
		g, err := scanSandboxGroup(tx.QueryRow(ctx,
			`SELECT `+sandboxGroupColumns+` FROM sandbox_groups WHERE workspace_id = $1 AND is_default`, workspaceID))
		if err == nil {
			out = g
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM sandbox_groups WHERE workspace_id = $1`, workspaceID).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			// Groups without a default cannot arise from these writes; if a future
			// delete leaves that state, the oldest group takes the role.
			out, err = scanSandboxGroup(tx.QueryRow(ctx, `
				UPDATE sandbox_groups SET is_default = true, updated_at = now()
				WHERE id = (SELECT id FROM sandbox_groups WHERE workspace_id = $1 ORDER BY created_at, id LIMIT 1)
				RETURNING `+sandboxGroupColumns, workspaceID))
			return err
		}
		out, err = insertSandboxGroup(ctx, tx, workspaceID, DefaultSandboxGroupName, SandboxGroupRegion, concurrencyLimit, true)
		return err
	})
	return out, err
}

func (s *PGStore) defaultSandboxGroup(ctx context.Context, workspaceID string) (SandboxGroup, error) {
	return scanSandboxGroup(s.Pool.QueryRow(ctx,
		`SELECT `+sandboxGroupColumns+` FROM sandbox_groups WHERE workspace_id = $1 AND is_default`, workspaceID))
}

// maxSandboxGroupsListed bounds a list when the group cap is disabled.
const maxSandboxGroupsListed = 1000

// SandboxGroups returns one workspace's groups, newest first.
func (s *PGStore) SandboxGroups(ctx context.Context, workspaceID string) ([]SandboxGroup, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+sandboxGroupColumns+` FROM sandbox_groups WHERE workspace_id = $1
		ORDER BY created_at DESC, id DESC LIMIT $2`, workspaceID, maxSandboxGroupsListed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SandboxGroup{}
	for rows.Next() {
		g, err := scanSandboxGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// SandboxGroup returns one group of a workspace, or ErrNotFound. A group of
// another workspace is ErrNotFound too.
func (s *PGStore) SandboxGroup(ctx context.Context, workspaceID, id string) (SandboxGroup, error) {
	g, err := scanSandboxGroup(s.Pool.QueryRow(ctx,
		`SELECT `+sandboxGroupColumns+` FROM sandbox_groups WHERE workspace_id = $1 AND id = $2`, workspaceID, id))
	if err != nil {
		return SandboxGroup{}, classify("sandbox group", err)
	}
	return g, nil
}

// SandboxGroupWorkspace is the by-id routing read: the workspace a group
// belongs to, or ErrNotFound. It only routes; every verb still authorizes and
// loads the group through SandboxGroup in that workspace.
func (s *PGStore) SandboxGroupWorkspace(ctx context.Context, id string) (string, error) {
	var workspace string
	if err := s.Pool.QueryRow(ctx, `SELECT workspace_id FROM sandbox_groups WHERE id = $1`, id).Scan(&workspace); err != nil {
		return "", classify("sandbox group", err)
	}
	return workspace, nil
}

// SetSandboxGroupLimit changes a group's configured concurrency limit and
// returns the group. It never touches a sandbox: lowering the limit below the
// active count stops nothing (Render, pinned OpenAPI update-sandbox-group).
func (s *PGStore) SetSandboxGroupLimit(ctx context.Context, workspaceID, id string, limit int) (SandboxGroup, error) {
	g, err := scanSandboxGroup(s.Pool.QueryRow(ctx, `
		UPDATE sandbox_groups SET concurrency_limit = $3, updated_at = now()
		WHERE workspace_id = $1 AND id = $2
		RETURNING `+sandboxGroupColumns, workspaceID, id, limit))
	if err != nil {
		return SandboxGroup{}, classify("sandbox group", err)
	}
	return g, nil
}

// SandboxGroupMembers returns the group binding of every sandbox of one
// workspace that has one, by public sandbox id.
func (s *PGStore) SandboxGroupMembers(ctx context.Context, workspaceID string) (map[string]SandboxGroupMember, error) {
	return sandboxGroupMembers(ctx, s.Pool, workspaceID)
}

type sandboxQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func sandboxGroupMembers(ctx context.Context, q sandboxQuerier, workspaceID string) (map[string]SandboxGroupMember, error) {
	rows, err := q.Query(ctx,
		`SELECT sandbox_id, group_id, lease_until FROM sandbox_group_members WHERE workspace_id = $1`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]SandboxGroupMember{}
	for rows.Next() {
		var id string
		var m SandboxGroupMember
		if err := rows.Scan(&id, &m.GroupID, &m.LeaseUntil); err != nil {
			return nil, err
		}
		out[id] = m
	}
	return out, rows.Err()
}

// SetSandboxSlotLease replaces a sandbox's slot reservation: until nil clears
// it (a suspended sandbox holds no slot), a time renews it. No binding row, no
// effect.
func (s *PGStore) SetSandboxSlotLease(ctx context.Context, workspaceID, sandboxID string, until *time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE sandbox_group_members SET lease_until = $3 WHERE workspace_id = $1 AND sandbox_id = $2`,
		workspaceID, sandboxID, until)
	return err
}

// ReleaseSandboxSlot removes a sandbox's binding: its create failed, or it was
// terminated, so it holds no slot.
func (s *PGStore) ReleaseSandboxSlot(ctx context.Context, workspaceID, sandboxID string) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM sandbox_group_members WHERE workspace_id = $1 AND sandbox_id = $2`, workspaceID, sandboxID)
	return err
}

// PruneSandboxGroupMembers deletes the bindings of one workspace's sandboxes
// absent from a complete OpenSandbox list. A row younger than ten minutes, or
// one still holding a slot lease, is kept: it may belong to a create that has
// not reached the list yet.
func (s *PGStore) PruneSandboxGroupMembers(ctx context.Context, workspaceID string, live []string) error {
	if live == nil {
		live = []string{}
	}
	_, err := s.Pool.Exec(ctx, `
		DELETE FROM sandbox_group_members
		WHERE workspace_id = $1 AND NOT (sandbox_id = ANY($2::text[]))
		  AND created_at < now() - interval '10 minutes'
		  AND (lease_until IS NULL OR lease_until < now())`,
		workspaceID, live)
	return err
}

// SandboxAdmission is the view an admission decision gets while it holds a
// workspace's admission lock: the current bindings, and the write that takes a
// slot. Both run in the lock's own transaction, so an admission needs one
// connection, never two.
type SandboxAdmission interface {
	// Members returns the workspace's sandbox-to-group bindings and their leases.
	Members(ctx context.Context) (map[string]SandboxGroupMember, error)
	// Bind binds sandboxID to groupID (an existing binding keeps its group) and
	// reserves its slot until lease.
	Bind(ctx context.Context, sandboxID, groupID string, lease time.Time) error
}

// WithSandboxAdmission runs fn while holding the workspace's sandbox admission
// lock, a transaction-scoped Postgres advisory lock, and commits what fn bound
// when it returns nil. Counting active sandboxes and taking a slot are one
// critical section per workspace, across bex-api replicas, so parallel creates
// and resumes cannot all see room for one more. The lock is per workspace, not
// per group, because the workspace-wide cap spans groups.
func (s *PGStore) WithSandboxAdmission(ctx context.Context, workspaceID string, fn func(SandboxAdmission) error) error {
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		// A waiter holds a pooled connection, so the wait is bounded: a burst of
		// creates for one workspace must not starve the pool for everyone else.
		if _, err := tx.Exec(ctx, fmt.Sprintf(`SET LOCAL lock_timeout = %d`, sandboxAdmissionLockTimeout.Milliseconds())); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "sandbox-admission:"+workspaceID); err != nil {
			return err
		}
		return fn(admissionTx{tx: tx, workspaceID: workspaceID})
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "55P03" { // lock_not_available
		return ErrSandboxAdmissionBusy
	}
	return err
}

// sandboxAdmissionLockTimeout is how long an admission waits for its workspace's
// lock. The holder's longest step is the OpenSandbox list (bounded at 10 s by the
// caller), so a healthy workspace never waits near this.
var sandboxAdmissionLockTimeout = 15 * time.Second

type admissionTx struct {
	tx          pgx.Tx
	workspaceID string
}

func (a admissionTx) Members(ctx context.Context) (map[string]SandboxGroupMember, error) {
	return sandboxGroupMembers(ctx, a.tx, a.workspaceID)
}

func (a admissionTx) Bind(ctx context.Context, sandboxID, groupID string, lease time.Time) error {
	_, err := a.tx.Exec(ctx, `
		INSERT INTO sandbox_group_members (workspace_id, sandbox_id, group_id, lease_until) VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id, sandbox_id) DO UPDATE SET lease_until = EXCLUDED.lease_until`,
		a.workspaceID, sandboxID, groupID, lease)
	if err != nil {
		return fmt.Errorf("bind sandbox to group: %w", classify("sandbox group member", err))
	}
	return nil
}
