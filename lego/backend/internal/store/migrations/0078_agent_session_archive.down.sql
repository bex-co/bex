CREATE INDEX agent_session_transcripts_created_idx
    ON agent_session_transcripts (created_at);

ALTER TABLE agent_sessions DROP COLUMN archived_at;
