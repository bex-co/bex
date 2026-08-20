-- codex round-16 #12: index created_at so the daily retention sweep can
-- delete claims outside the replay horizon without a full table scan.
CREATE INDEX IF NOT EXISTS git_webhook_replays_created_at_idx
    ON git_webhook_replays (created_at);
