CREATE TABLE env_group_cutover (
    id           boolean     PRIMARY KEY DEFAULT true CHECK (id),
    completed_at timestamptz NOT NULL
);

-- Metadata was already migrated before this marker was retired. Keep the
-- preceding binary's serving gate open if this migration is rolled back.
INSERT INTO env_group_cutover (completed_at) VALUES (CURRENT_TIMESTAMP);
