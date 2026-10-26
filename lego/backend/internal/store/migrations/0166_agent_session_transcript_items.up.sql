-- ADR051 § Item model (w10/m16): the durable transcript stores one row per
-- completed item (a text/reasoning/tool UIMessage part, a data snapshot, or a
-- standalone chunk) instead of one row per AI-SDK stream chunk.
--
-- item_state: 0 = a legacy raw UI-message chunk (every row written before this
-- migration), 1 = a completed item, 2 = a partial item the stream ended inside
-- of (the Completer harvest may replace it with the complete copy). The
-- partial index lists exactly the sessions the one-shot compaction still has to
-- rewrite, so after it drains the startup check is one empty index probe.
ALTER TABLE agent_session_transcripts
    ADD COLUMN item_state smallint NOT NULL DEFAULT 0
    CONSTRAINT agent_session_transcripts_item_state_check CHECK (item_state IN (0, 1, 2));

CREATE INDEX agent_session_transcripts_legacy_idx
    ON agent_session_transcripts (session_id) WHERE item_state = 0;
