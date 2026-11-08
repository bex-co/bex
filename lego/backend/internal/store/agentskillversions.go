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
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// AgentSkillVersionRetention is how many versions of one skill are kept; the
// save that writes version N drops versions up to N-AgentSkillVersionRetention
// (ADR099 D2).
const AgentSkillVersionRetention = 200

// AgentSkillVersion is one saved version of a workspace skill: the whole
// SKILL.md as it was after that save. A list leaves Content empty.
type AgentSkillVersion struct {
	SkillID   string
	Version   int
	Content   string
	Message   string
	CreatedBy string
	CreatedAt time.Time
	ShortID   string
}

// AgentSkillVersionShortID is a version's display handle: the first 7 hex
// characters of sha256(skillID|version|content). agent_skill_versions'
// backfill computes the same in SQL.
func AgentSkillVersionShortID(skillID string, version int, content string) string {
	sum := sha256.Sum256([]byte(skillID + "|" + strconv.Itoa(version) + "|" + content))
	return hex.EncodeToString(sum[:])[:7]
}

// saveAgentSkillVersion writes the version a create or update just produced
// (saved is the row RETURNING gave; in carries VersionContent and
// VersionMessage) and drops versions past the retention. It runs in the
// save's transaction, under lockAgentSkills, so a rolled-back save keeps no
// version and a kept one is never missing.
func saveAgentSkillVersion(ctx context.Context, tx pgx.Tx, saved, in AgentSkill) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO agent_skill_versions (skill_id, version, content, message, created_by, created_at, short_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		saved.ID, saved.Version, in.VersionContent, in.VersionMessage, saved.UpdatedBy, saved.UpdatedAt,
		AgentSkillVersionShortID(saved.ID, saved.Version, in.VersionContent)); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `DELETE FROM agent_skill_versions WHERE skill_id=$1 AND version <= $2`,
		saved.ID, saved.Version-AgentSkillVersionRetention)
	return err
}

const agentSkillVersionColumns = `skill_id, version, content, message, created_by, created_at, short_id`

func scanAgentSkillVersion(row pgx.Row) (AgentSkillVersion, error) {
	var out AgentSkillVersion
	err := row.Scan(&out.SkillID, &out.Version, &out.Content, &out.Message, &out.CreatedBy, &out.CreatedAt, &out.ShortID)
	return out, err
}

// ListAgentSkillVersions returns up to limit of the skill's versions,
// newest first, without Content. before is a keyset cursor: only versions
// below it are returned, and 0 starts from the newest.
func (s *PGStore) ListAgentSkillVersions(ctx context.Context, skillID string, before, limit int) ([]AgentSkillVersion, error) {
	if !textBindable(skillID) {
		return []AgentSkillVersion{}, nil
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT skill_id, version, '', message, created_by, created_at, short_id
		FROM agent_skill_versions
		WHERE skill_id=$1 AND ($2 <= 0 OR version < $2)
		ORDER BY version DESC LIMIT $3`, skillID, before, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (AgentSkillVersion, error) {
		return scanAgentSkillVersion(row)
	})
}

// GetAgentSkillVersion reads one version with its Content; ErrNotFound when
// the skill has no such version (never saved, or dropped by the retention).
func (s *PGStore) GetAgentSkillVersion(ctx context.Context, skillID string, version int) (AgentSkillVersion, error) {
	if !textBindable(skillID) {
		return AgentSkillVersion{}, ErrNotFound
	}
	out, err := scanAgentSkillVersion(s.Pool.QueryRow(ctx,
		`SELECT `+agentSkillVersionColumns+` FROM agent_skill_versions WHERE skill_id=$1 AND version=$2`, skillID, version))
	if err != nil {
		return AgentSkillVersion{}, classify("agent skill version", err)
	}
	return out, nil
}

// RewriteAgentSkillVersions walks every stored version and replaces the
// content rewrite returns ok for, recomputing the short id from the new
// content; version number, message, author and time stay. The update only
// applies to the content that was read, so a concurrent run or a trim loses
// harmlessly. It returns how many rows it changed.
func (s *PGStore) RewriteAgentSkillVersions(ctx context.Context, rewrite func(content string) (string, bool)) (int, error) {
	const batch = 200
	changed, lastSkill, lastVersion := 0, "", 0
	for {
		rows, err := s.Pool.Query(ctx, `
			SELECT skill_id, version, content FROM agent_skill_versions
			WHERE (skill_id, version) > ($1, $2) ORDER BY skill_id, version LIMIT $3`, lastSkill, lastVersion, batch)
		if err != nil {
			return changed, err
		}
		page, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AgentSkillVersion, error) {
			var v AgentSkillVersion
			return v, row.Scan(&v.SkillID, &v.Version, &v.Content)
		})
		if err != nil {
			return changed, err
		}
		for _, v := range page {
			next, ok := rewrite(v.Content)
			if !ok || next == v.Content {
				continue
			}
			tag, err := s.Pool.Exec(ctx, `
				UPDATE agent_skill_versions SET content=$4, short_id=$5
				WHERE skill_id=$1 AND version=$2 AND content=$3`,
				v.SkillID, v.Version, v.Content, next, AgentSkillVersionShortID(v.SkillID, v.Version, next))
			if err != nil {
				return changed, err
			}
			changed += int(tag.RowsAffected())
		}
		if len(page) < batch {
			return changed, nil
		}
		last := page[len(page)-1]
		lastSkill, lastVersion = last.SkillID, last.Version
	}
}
