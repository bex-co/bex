-- Checked dispatch recovery looks up an already-recorded release by App and
-- generation, including terminal history, before deciding whether to insert.
CREATE INDEX IF NOT EXISTS deploys_app_generation_created_idx
    ON deploys (app_id, generation, created_at, id);
