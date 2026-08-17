-- w2/m70: Render requires a unique webhook name. Normalize legacy blanks and
-- disambiguate legacy duplicates deterministically before enforcing the
-- workspace-scoped, case-insensitive invariant for every future write.
UPDATE webhook_endpoints SET name = url WHERE btrim(name) = '';

WITH ranked AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY tenant_id, lower(btrim(name))
               ORDER BY created_at, id
           ) AS ordinal
    FROM webhook_endpoints
)
UPDATE webhook_endpoints e
SET name = e.name || ' [' || e.id || ']'
FROM ranked r
WHERE e.id = r.id AND r.ordinal > 1;

CREATE UNIQUE INDEX webhook_endpoints_tenant_name_idx
    ON webhook_endpoints (tenant_id, lower(btrim(name)));
