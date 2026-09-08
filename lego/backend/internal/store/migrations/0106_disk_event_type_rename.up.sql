-- w8/m34: rename legacy bex disk event spellings to Render's wire vocabulary.
-- Live webhook subscription filters may still carry disk_attached/disk_detached
-- from pre-rename clients or manual inserts; rewrite them so endpoints keep
-- matching newly emitted disk_created/disk_deleted events.
-- Immutable webhook attempt/delivery payloads are historical evidence and are
-- deliberately left byte-unchanged (w8/m25).
-- Service-feed history needs no row rewrite: types are projected from audit
-- verbs at read time (apps.AddDisk / apps.DeleteDisk), so renaming the
-- constants is enough for historical audit rows.
UPDATE webhook_endpoints
SET event_types = (
    SELECT COALESCE(array_agg(canonical ORDER BY ord), ARRAY[]::text[])
    FROM (
        SELECT DISTINCT ON (canonical) canonical, ord
        FROM (
            SELECT
                CASE
                    WHEN t = 'disk_attached' THEN 'disk_created'
                    WHEN t = 'disk_detached' THEN 'disk_deleted'
                    ELSE t
                END AS canonical,
                ord
            FROM unnest(event_types) WITH ORDINALITY AS u(t, ord)
        ) rewritten
        ORDER BY canonical, ord
    ) deduped
)
WHERE event_types && ARRAY['disk_attached', 'disk_detached']::text[];
