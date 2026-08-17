-- ADR047/ADR051 persistence closure (w5/m71): prompts are first-class durable
-- turn intents, separate from the assistant's byte-transparent UI-message parts.
CREATE TABLE agent_session_turns (
    session_id            text        NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    turn                  int         NOT NULL CHECK (turn > 0),
    prompt                text        NOT NULL CHECK (length(prompt) <= 100000),
    delivery_mode         text        NOT NULL DEFAULT '',
    transcript_complete   boolean     NOT NULL DEFAULT false,
    transcript_truncated  boolean     NOT NULL DEFAULT false,
    truncation_reason     text        NOT NULL DEFAULT '',
    created_at            timestamptz NOT NULL DEFAULT now(),
    completed_at          timestamptz,
    PRIMARY KEY (session_id, turn)
);

-- The initial task remains recoverable for legacy rows. Historical follow-up
-- prompts never landed in durable storage and therefore cannot be invented.
INSERT INTO agent_session_turns (session_id, turn, prompt, delivery_mode)
SELECT id, 1, agent_config->>'task', ''
FROM agent_sessions
WHERE jsonb_typeof(agent_config->'task') = 'string'
  AND length(btrim(agent_config->>'task')) > 0
ON CONFLICT DO NOTHING;

-- Driver ordinals restart at zero in every fresh sandbox. Keep seq as the
-- monotonic per-session replay cursor, but make the idempotency key turn-local.
ALTER TABLE agent_session_transcripts ADD COLUMN part_index bigint;

WITH numbered AS (
    SELECT session_id, seq, row_number() OVER (
        PARTITION BY session_id, turn ORDER BY seq
    ) - 1 AS part_index
    FROM agent_session_transcripts
)
UPDATE agent_session_transcripts AS transcript
SET part_index = numbered.part_index
FROM numbered
WHERE transcript.session_id = numbered.session_id
  AND transcript.seq = numbered.seq;

ALTER TABLE agent_session_transcripts ALTER COLUMN part_index SET NOT NULL;
ALTER TABLE agent_session_transcripts
    ADD CONSTRAINT agent_session_transcripts_turn_part_key
    UNIQUE (session_id, turn, part_index);
