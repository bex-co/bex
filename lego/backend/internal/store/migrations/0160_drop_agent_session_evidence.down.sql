-- Re-adds the column for a rollback to a binary that still reads it. Rows keep
-- no digest (the up migration discarded it); the empty-object default is what
-- such a binary treats as "no evidence".
ALTER TABLE agent_sessions
    ADD COLUMN evidence jsonb DEFAULT '{}'::jsonb;
