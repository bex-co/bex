-- w4/652: under Render's Wait overlap policy a Rollback, a deploy with an image
-- override, and any deploy behind one of those wait for the deploy in flight
-- like a Restart does (0211). restart_deferred now marks every such waiting
-- row (its trigger says which verb), and deferred_request holds the
-- parameters the verb runs with once nothing is in flight (empty for a
-- Restart, which takes none; NULL on every row that never waited).
ALTER TABLE deploys ADD COLUMN deferred_request jsonb;
