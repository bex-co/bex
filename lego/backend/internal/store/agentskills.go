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
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// AgentSkill is one workspace skill for cloud agents (ADR099 D2): a SKILL.md
// document's frontmatter fields, its body, and any other frontmatter keys
// kept verbatim in Extra.
type AgentSkill struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	Triggers    []string
	Enabled     bool
	Body        string
	Extra       string
	Version     int
	UpdatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TurnSkill is one skill a turn invoked explicitly (ADR099 D4).
type TurnSkill struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

var (
	// ErrAgentSkillLimit is a create refused at the workspace's skill cap.
	ErrAgentSkillLimit = errors.New("agent skill limit reached")
	// ErrAgentSkillBytes is a write that would take the workspace's enabled
	// skills over AgentSkillLimits.EnabledBytes.
	ErrAgentSkillBytes = errors.New("agent skill enabled bytes exceeded")
	// ErrAgentSkillVersion is an update whose expected version is stale.
	ErrAgentSkillVersion = errors.New("agent skill version changed")
)

// AgentSkillLimits are the workspace caps a write is checked against inside
// its transaction (ADR099 D2).
type AgentSkillLimits struct {
	Count        int
	EnabledBytes int
}

// agentSkillDocOverhead stands for a delivered SKILL.md's fences, keys and
// quoting on top of its field bytes, so the SQL sum tracks the rendered size.
const agentSkillDocOverhead = 64

// AgentSkillBytes is what one enabled skill counts toward
// AgentSkillLimits.EnabledBytes; agentSkillBytesSQL is the same measure in SQL.
func AgentSkillBytes(s AgentSkill) int {
	return len(s.Name) + len(s.Description) + len(s.Body) + len(s.Extra) + agentSkillDocOverhead
}

var agentSkillBytesSQL = `octet_length(name) + octet_length(description) + octet_length(body) + octet_length(extra) + ` +
	strconv.Itoa(agentSkillDocOverhead)

const agentSkillColumns = `id, workspace_id, name, description, triggers, enabled, body, extra,
	version, updated_by, created_at, updated_at`

// agentSkillMetaColumns reads a list row without its body and extra
// frontmatter, which only a single read and a dispatch need.
const agentSkillMetaColumns = `id, workspace_id, name, description, triggers, enabled, '', '',
	version, updated_by, created_at, updated_at`

func scanAgentSkill(row pgx.Row) (AgentSkill, error) {
	var out AgentSkill
	err := row.Scan(&out.ID, &out.WorkspaceID, &out.Name, &out.Description, &out.Triggers, &out.Enabled,
		&out.Body, &out.Extra, &out.Version, &out.UpdatedBy, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func collectAgentSkills(rows pgx.Rows, err error) ([]AgentSkill, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (AgentSkill, error) {
		return scanAgentSkill(row)
	})
}

// ListAgentSkills returns the workspace's skills by name, without Body and
// Extra.
func (s *PGStore) ListAgentSkills(ctx context.Context, workspaceID string) ([]AgentSkill, error) {
	return collectAgentSkills(s.Pool.Query(ctx,
		`SELECT `+agentSkillMetaColumns+` FROM agent_skills WHERE workspace_id=$1 ORDER BY name`, workspaceID))
}

// AgentSkillsForDispatch returns, whole, what a dispatch delivers: the
// workspace's enabled skills plus the named ones even if disabled since.
func (s *PGStore) AgentSkillsForDispatch(ctx context.Context, workspaceID string, invoked []string) ([]AgentSkill, error) {
	if invoked == nil {
		invoked = []string{}
	}
	return collectAgentSkills(s.Pool.Query(ctx,
		`SELECT `+agentSkillColumns+` FROM agent_skills
		 WHERE workspace_id=$1 AND (enabled OR name = ANY($2)) ORDER BY name`, workspaceID, invoked))
}

// GetAgentSkill reads one skill by id; ErrNotFound when there is none.
func (s *PGStore) GetAgentSkill(ctx context.Context, id string) (AgentSkill, error) {
	if !textBindable(id) {
		return AgentSkill{}, ErrNotFound
	}
	out, err := scanAgentSkill(s.Pool.QueryRow(ctx, `SELECT `+agentSkillColumns+` FROM agent_skills WHERE id=$1`, id))
	if err != nil {
		return AgentSkill{}, classify("agent skill", err)
	}
	return out, nil
}

// GetAgentSkillByName reads one of the workspace's skills by its name.
func (s *PGStore) GetAgentSkillByName(ctx context.Context, workspaceID, name string) (AgentSkill, error) {
	if !textBindable(name) {
		return AgentSkill{}, ErrNotFound
	}
	out, err := scanAgentSkill(s.Pool.QueryRow(ctx,
		`SELECT `+agentSkillColumns+` FROM agent_skills WHERE workspace_id=$1 AND name=$2`, workspaceID, name))
	if err != nil {
		return AgentSkill{}, classify("agent skill", err)
	}
	return out, nil
}

// checkEnabledSkillBytes refuses a write that would take the workspace's
// enabled skills, with in counted in place of its stored row, over the cap.
// It runs under lockAgentSkills.
func checkEnabledSkillBytes(ctx context.Context, tx pgx.Tx, in AgentSkill, limit int) error {
	if !in.Enabled || limit <= 0 {
		return nil
	}
	var others int
	if err := tx.QueryRow(ctx, `SELECT coalesce(sum(`+agentSkillBytesSQL+`), 0) FROM agent_skills
		WHERE workspace_id=$1 AND enabled AND id <> $2`, in.WorkspaceID, in.ID).Scan(&others); err != nil {
		return err
	}
	if others+AgentSkillBytes(in) > limit {
		return ErrAgentSkillBytes
	}
	return nil
}

// CreateAgentSkill inserts a skill unless the workspace already holds
// limits.Count of them (ErrAgentSkillLimit) or its enabled skills would pass
// limits.EnabledBytes (ErrAgentSkillBytes). The checks and the insert share
// a transaction under lockAgentSkills, so concurrent creates cannot
// overshoot. A taken name is ErrConflict.
func (s *PGStore) CreateAgentSkill(ctx context.Context, in AgentSkill, limits AgentSkillLimits) (AgentSkill, error) {
	var out AgentSkill
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockAgentSkills(ctx, tx, in.WorkspaceID); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM agent_skills WHERE workspace_id=$1`, in.WorkspaceID).Scan(&n); err != nil {
			return err
		}
		if n >= limits.Count {
			return ErrAgentSkillLimit
		}
		if err := checkEnabledSkillBytes(ctx, tx, in, limits.EnabledBytes); err != nil {
			return err
		}
		var err error
		out, err = scanAgentSkill(tx.QueryRow(ctx, `
			INSERT INTO agent_skills (id, workspace_id, name, description, triggers, enabled, body, extra, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING `+agentSkillColumns,
			in.ID, in.WorkspaceID, in.Name, in.Description, in.Triggers, in.Enabled, in.Body, in.Extra, in.UpdatedBy))
		return err
	})
	if errors.Is(err, ErrAgentSkillLimit) || errors.Is(err, ErrAgentSkillBytes) {
		return AgentSkill{}, err
	}
	if err != nil {
		return AgentSkill{}, classify("agent skill", err)
	}
	return out, nil
}

// UpdateAgentSkill replaces a skill's editable fields when its version is
// still ifVersion, and bumps the version, under the same lock and
// enabled-bytes check as a create. A stale version is ErrAgentSkillVersion,
// a missing row ErrNotFound and a taken name ErrConflict.
func (s *PGStore) UpdateAgentSkill(ctx context.Context, in AgentSkill, ifVersion int, limits AgentSkillLimits) (AgentSkill, error) {
	var out AgentSkill
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockAgentSkills(ctx, tx, in.WorkspaceID); err != nil {
			return err
		}
		if err := checkEnabledSkillBytes(ctx, tx, in, limits.EnabledBytes); err != nil {
			return err
		}
		var err error
		out, err = scanAgentSkill(tx.QueryRow(ctx, `
			UPDATE agent_skills
			SET name=$3, description=$4, triggers=$5, enabled=$6, body=$7, extra=$8, updated_by=$9,
			    version=version+1, updated_at=now()
			WHERE id=$1 AND version=$2 AND workspace_id=$10
			RETURNING `+agentSkillColumns,
			in.ID, ifVersion, in.Name, in.Description, in.Triggers, in.Enabled, in.Body, in.Extra, in.UpdatedBy, in.WorkspaceID))
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_skills WHERE id=$1 AND workspace_id=$2)`,
				in.ID, in.WorkspaceID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrNotFound
			}
			return ErrAgentSkillVersion
		}
		return err
	})
	switch {
	case errors.Is(err, ErrAgentSkillBytes), errors.Is(err, ErrAgentSkillVersion), errors.Is(err, ErrNotFound):
		return AgentSkill{}, err
	case err != nil:
		return AgentSkill{}, classify("agent skill", err)
	}
	return out, nil
}

// DeleteAgentSkill removes one skill; ErrNotFound when there is none.
func (s *PGStore) DeleteAgentSkill(ctx context.Context, id string) error {
	if !textBindable(id) {
		return ErrNotFound
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM agent_skills WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
