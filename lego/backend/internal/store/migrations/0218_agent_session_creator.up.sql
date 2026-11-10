-- w7/m296: who started a session becomes part of its read model and a list
-- filter (Devin's "sessions you started" default, v3 user_ids).
--
-- Backfill only what is exact. A session an automation started ran as the
-- automation's creator (ADR101 D4), and an automation that was never edited
-- (version 1) has had the same creator since it was created. Every other row
-- from before 0212 has no recorded author anywhere, so it stays empty rather
-- than guessed.
UPDATE agent_sessions s
SET created_by = a.created_by
FROM agent_automations a
WHERE s.created_by = ''
  AND s.automation_id = a.id
  AND a.workspace_id = s.workspace_id
  AND a.version = 1;

-- The dashboard's default list: one creator's sessions in a workspace, newest
-- first, in the keyset order the cursor pages.
CREATE INDEX agent_sessions_creator_idx
    ON agent_sessions (workspace_id, created_by, created_at DESC, id DESC);
