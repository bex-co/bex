-- ADR102 D9: the subject that created the session. A running session that
-- spawns a child acts with this person's authority (the same rule an
-- automation fire follows, ADR101 D4): the spawn re-checks can_create for
-- this subject, so a person who lost access cannot keep spawning through a
-- session they started. Empty for rows that predate the column; those
-- sessions cannot spawn.
ALTER TABLE agent_sessions
    ADD COLUMN created_by text NOT NULL DEFAULT '';
