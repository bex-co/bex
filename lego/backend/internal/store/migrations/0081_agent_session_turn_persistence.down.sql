ALTER TABLE agent_session_transcripts
    DROP CONSTRAINT agent_session_transcripts_turn_part_key;
ALTER TABLE agent_session_transcripts DROP COLUMN part_index;
DROP TABLE agent_session_turns;
