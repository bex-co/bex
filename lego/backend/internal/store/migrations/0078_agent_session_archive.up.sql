-- ADR065 D1: the Devin-style archive flag. Archive is LIST-state, not
-- lifecycle-state: archived_at is orthogonal to phase (any phase can be
-- archived), NULL means the session is in the working set, and the phase CHECK
-- is untouched. Retention expiry additionally stamps it (D5) so a session whose
-- snapshot aged out leaves the working set by itself. The default list read
-- excludes archived rows; the existing agent_sessions_workspace_created_idx
-- already serves the ordered workspace scan.
ALTER TABLE agent_sessions ADD COLUMN archived_at timestamptz;

-- ADR065 D2 made transcript retention cascade-only (the age-based prune this
-- index was created for never shipped and is now removed), so the created_at
-- index is pure write amplification on every teed part. Drop it; ordered
-- replay + the seq-keyset page ride the (session_id, seq) primary key.
DROP INDEX agent_session_transcripts_created_idx;
