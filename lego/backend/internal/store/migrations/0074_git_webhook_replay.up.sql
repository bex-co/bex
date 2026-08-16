-- codex round-8 #9: durable replay claims for the signed git webhook. The
-- HMAC signature authenticates the body's bytes, not its freshness: a captured
-- (body, signature) pair — resent with any X-GitHub-Delivery header, which is
-- unsigned and only keys event facts — would otherwise re-enter redeploy/sync
-- and stamp a fresh deploy generation per replay. One row per processed
-- mutating delivery (push/delete); ON CONFLICT DO NOTHING makes the claim
-- atomic, so concurrent duplicates collapse and sequential replays are
-- skipped. Rows accrue at genuine-delivery volume (the same order as deploys
-- themselves), so no retention sweep is needed.
CREATE TABLE IF NOT EXISTS git_webhook_replays (
    digest     text PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now()
);
