DROP INDEX IF EXISTS agent_session_transcripts_legacy_idx;
ALTER TABLE agent_session_transcripts DROP COLUMN IF EXISTS item_state;
