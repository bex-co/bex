-- w21/m258 (ADR094 D3): a display title and up to 20 filter/cost-attribution
-- tags per agent session. Both are set at create and read-only afterwards.
ALTER TABLE agent_sessions
    ADD COLUMN title text NOT NULL DEFAULT '',
    ADD COLUMN tags text[] NOT NULL DEFAULT '{}',
    ADD CONSTRAINT agent_sessions_title_check CHECK (char_length(title) <= 200),
    ADD CONSTRAINT agent_sessions_tags_check CHECK (cardinality(tags) <= 20);
