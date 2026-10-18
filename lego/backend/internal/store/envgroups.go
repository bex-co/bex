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
)

// Content states of an environment group (env_groups.content_state).
const (
	EnvGroupIdle           = "idle"
	EnvGroupBusy           = "busy"
	EnvGroupRepairRequired = "repair_required"
)

var (
	// ErrEnvGroupNameTaken is the (tenant_id, name) unique violation: another
	// group in the workspace already uses the name.
	ErrEnvGroupNameTaken = errors.New("environment group name taken")
	// ErrEnvGroupChanged is a lost compare-and-set: the row exists but its
	// version moved, or the operation it names is no longer the active one.
	ErrEnvGroupChanged = errors.New("environment group changed")
)

// EnvGroup is a row of env_groups (w8/m114): every non-secret fact about one
// environment group. Its env and files maps stay in OpenBao.
type EnvGroup struct {
	ID            string
	TenantID      string
	Name          string
	EnvironmentID string // storage id ("env-…"); "" for a workspace-scoped group
	ServiceLinks  []string
	EnvKeys       []string
	FileNames     []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	MetaVersion   int64
	Content       EnvGroupContent
}

// EnvGroupContent is the coordination state of the group's content: which
// generation is committed, whether a save holds it, and that save's operation.
type EnvGroupContent struct {
	State      string
	Generation uint64
	Version    int64
	Op         *EnvGroupOp
}

// EnvGroupOp is the in-flight (or stranded) content operation recorded with
// the claim, so another API instance can recover it after its lease.
type EnvGroupOp struct {
	ID           string
	Kind         string
	Phase        string
	LeaseUntil   time.Time
	EnvChanged   bool
	FilesChanged bool
}

// EnvGroupRelease ends a claim. By default it releases the group idle and
// clears the operation, with the committed content unchanged. RepairRequired
// keeps the operation as the evidence recovery needs instead. Commit records
// that the save changed content: the next generation, with these names.
type EnvGroupRelease struct {
	RepairRequired bool
	Commit         *EnvGroupCommit
}

// EnvGroupCommit is what a content-changing save leaves on the row.
type EnvGroupCommit struct {
	EnvKeys   []string
	FileNames []string
	UpdatedAt time.Time
}

const envGroupColumns = `id, tenant_id, name, COALESCE(environment_id, ''), service_links, env_keys, file_names,
	created_at, updated_at, meta_version, content_state, content_generation, content_version,
	op_id, op_kind, op_phase, op_lease_until, op_env_changed, op_files_changed`

func scanEnvGroup(row pgx.Row) (EnvGroup, error) {
	var g EnvGroup
	var generation int64
	var opID, opKind, opPhase *string
	var opLease *time.Time
	var envChanged, filesChanged bool
	err := row.Scan(&g.ID, &g.TenantID, &g.Name, &g.EnvironmentID, &g.ServiceLinks, &g.EnvKeys, &g.FileNames,
		&g.CreatedAt, &g.UpdatedAt, &g.MetaVersion, &g.Content.State, &generation, &g.Content.Version,
		&opID, &opKind, &opPhase, &opLease, &envChanged, &filesChanged)
	if err != nil {
		return EnvGroup{}, classifyEnvGroup(err)
	}
	g.Content.Generation = uint64(generation)
	if opID != nil {
		g.Content.Op = &EnvGroupOp{
			ID: *opID, Kind: *opKind, Phase: *opPhase, LeaseUntil: opLease.UTC(),
			EnvChanged: envChanged, FilesChanged: filesChanged,
		}
	}
	g.CreatedAt, g.UpdatedAt = g.CreatedAt.UTC(), g.UpdatedAt.UTC()
	return g, nil
}

// classifyEnvGroup maps a unique violation on the name constraint to
// ErrEnvGroupNameTaken and everything else through classify.
func classifyEnvGroup(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "env_groups_tenant_name_key" {
		return ErrEnvGroupNameTaken
	}
	return classify("environment group", err)
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// InsertEnvGroup creates a group row, idle at g.Content.Generation (1 when
// unset), and returns it.
func (s *PGStore) InsertEnvGroup(ctx context.Context, g EnvGroup) (EnvGroup, error) {
	generation := max(g.Content.Generation, 1)
	return scanEnvGroup(s.Pool.QueryRow(ctx, `INSERT INTO env_groups
		(id, tenant_id, name, environment_id, service_links, env_keys, file_names, created_at, updated_at, content_generation)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+envGroupColumns,
		g.ID, g.TenantID, g.Name, nullIfEmpty(g.EnvironmentID), nonNil(g.ServiceLinks), nonNil(g.EnvKeys), nonNil(g.FileNames),
		g.CreatedAt, g.UpdatedAt, int64(generation)))
}

// GetEnvGroup returns one group, or ErrNotFound.
func (s *PGStore) GetEnvGroup(ctx context.Context, id string) (EnvGroup, error) {
	return scanEnvGroup(s.Pool.QueryRow(ctx, `SELECT `+envGroupColumns+` FROM env_groups WHERE id = $1`, id))
}

// GetEnvGroupByName returns the workspace's group named name, or ErrNotFound.
func (s *PGStore) GetEnvGroupByName(ctx context.Context, tenantID, name string) (EnvGroup, error) {
	return scanEnvGroup(s.Pool.QueryRow(ctx,
		`SELECT `+envGroupColumns+` FROM env_groups WHERE tenant_id = $1 AND name = $2`, tenantID, name))
}

// ListEnvGroups returns one workspace's groups in id order.
func (s *PGStore) ListEnvGroups(ctx context.Context, tenantID string) ([]EnvGroup, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+envGroupColumns+` FROM env_groups WHERE tenant_id = $1 ORDER BY id`, tenantID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (EnvGroup, error) { return scanEnvGroup(row) })
}

// CountEnvGroups is the workspace's group count, for the create quota.
func (s *PGStore) CountEnvGroups(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM env_groups WHERE tenant_id = $1`, tenantID).Scan(&n)
	return n, err
}

// EnvGroupNameTaken reports whether a group in the workspace uses name. It
// lets a create fail before its side effects; the unique constraint still
// decides a race.
func (s *PGStore) EnvGroupNameTaken(ctx context.Context, tenantID, name string) (bool, error) {
	var taken bool
	err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM env_groups WHERE tenant_id = $1 AND name = $2)`, tenantID, name).Scan(&taken)
	return taken, err
}

// UpdateEnvGroupMeta writes g's name, Environment, links and updated_at if the
// row still carries g.MetaVersion, and returns the new row. A moved version is
// ErrEnvGroupChanged, a missing row ErrNotFound, a taken name
// ErrEnvGroupNameTaken.
func (s *PGStore) UpdateEnvGroupMeta(ctx context.Context, g EnvGroup) (EnvGroup, error) {
	updated, err := scanEnvGroup(s.Pool.QueryRow(ctx, `UPDATE env_groups
		SET name = $3, environment_id = $4, service_links = $5, updated_at = $6, meta_version = meta_version + 1
		WHERE id = $1 AND meta_version = $2
		RETURNING `+envGroupColumns,
		g.ID, g.MetaVersion, g.Name, nullIfEmpty(g.EnvironmentID), nonNil(g.ServiceLinks), g.UpdatedAt))
	return updated, s.envGroupCAS(ctx, g.ID, err)
}

// DeleteEnvGroup removes the row and returns it as it was, so the caller can
// detach its links and remove its Secrets and OpenBao maps.
func (s *PGStore) DeleteEnvGroup(ctx context.Context, id string) (EnvGroup, error) {
	return scanEnvGroup(s.Pool.QueryRow(ctx, `DELETE FROM env_groups WHERE id = $1 RETURNING `+envGroupColumns, id))
}

// ClaimEnvGroupContent marks the group busy for op if its content version is
// still expectedVersion, recording the operation in the same statement.
// Returns the claim's content version.
func (s *PGStore) ClaimEnvGroupContent(ctx context.Context, id string, expectedVersion int64, op EnvGroupOp) (int64, error) {
	var version int64
	err := s.Pool.QueryRow(ctx, `UPDATE env_groups
		SET content_state = 'busy', op_id = $3, op_kind = $4, op_phase = $5, op_lease_until = $6,
			op_env_changed = $7, op_files_changed = $8, content_version = content_version + 1
		WHERE id = $1 AND content_version = $2
		RETURNING content_version`,
		id, expectedVersion, op.ID, op.Kind, op.Phase, op.LeaseUntil, op.EnvChanged, op.FilesChanged).Scan(&version)
	return version, s.envGroupCAS(ctx, id, err)
}

// AdvanceEnvGroupOp records a later phase (and a renewed lease) for opID. It
// does not move the content version, so the claim's release still matches.
func (s *PGStore) AdvanceEnvGroupOp(ctx context.Context, id, opID, phase string, leaseUntil time.Time) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE env_groups SET op_phase = $3, op_lease_until = $4
		WHERE id = $1 AND op_id = $2`, id, opID, phase, leaseUntil)
	if err == nil && tag.RowsAffected() == 0 {
		err = pgx.ErrNoRows
	}
	return s.envGroupCAS(ctx, id, err)
}

// ReleaseEnvGroupContent ends the claim held at expectedVersion.
func (s *PGStore) ReleaseEnvGroupContent(ctx context.Context, id string, expectedVersion int64, rel EnvGroupRelease) error {
	set := "content_state = 'idle', op_id = NULL, op_kind = NULL, op_phase = NULL, op_lease_until = NULL, op_env_changed = false, op_files_changed = false"
	if rel.RepairRequired {
		set = "content_state = 'repair_required'"
	}
	args := []any{id, expectedVersion}
	if c := rel.Commit; c != nil {
		set += ", content_generation = content_generation + 1, env_keys = $3, file_names = $4, updated_at = $5"
		args = append(args, nonNil(c.EnvKeys), nonNil(c.FileNames), c.UpdatedAt)
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE env_groups SET `+set+`, content_version = content_version + 1
		WHERE id = $1 AND content_version = $2`, args...)
	if err == nil && tag.RowsAffected() == 0 {
		err = pgx.ErrNoRows
	}
	return s.envGroupCAS(ctx, id, err)
}

// envGroupCAS maps a compare-and-set's error: one that matched no row is the
// group gone (ErrNotFound) or moved by another writer (ErrEnvGroupChanged).
func (s *PGStore) envGroupCAS(ctx context.Context, id string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, pgx.ErrNoRows), errors.Is(err, ErrNotFound):
		return s.envGroupLostRace(ctx, id)
	}
	return classifyEnvGroup(err)
}

// envGroupLostRace tells a compare-and-set that matched no row apart: the
// group is gone (ErrNotFound) or another writer moved it (ErrEnvGroupChanged).
func (s *PGStore) envGroupLostRace(ctx context.Context, id string) error {
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM env_groups WHERE id = $1)`, id).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return ErrEnvGroupChanged
	}
	return fmt.Errorf("environment group: %w", ErrNotFound)
}

// EnvGroupCutoverComplete reports whether the one-shot OpenBao-to-Postgres
// cutover has run (w8/m114; transitional, removed with the migrator).
func (s *PGStore) EnvGroupCutoverComplete(ctx context.Context) (bool, error) {
	var done bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM env_group_cutover)`).Scan(&done)
	return done, err
}

// CompleteEnvGroupCutover records that the cutover has run; idempotent.
func (s *PGStore) CompleteEnvGroupCutover(ctx context.Context, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO env_group_cutover (completed_at) VALUES ($1) ON CONFLICT (id) DO NOTHING`, at)
	return err
}
