-- 0020: apps.slug — the globally-unique public subdomain (w4/m19 t002),
-- distinct from apps.name (workspace-unique only, UNIQUE (tenant_id, name)).
-- Render calls this the service's "slug": defaults to the name, gets a random
-- short suffix ("-xxxx") the moment a bare-name collision would occur across
-- the whole platform. The operator derives the public host from this column
-- (spec.subdomain), never from apps.name, so two same-named apps in different
-- tenants never claim the same Ingress host.
ALTER TABLE apps ADD COLUMN slug text;

-- Backfill: bare name for the first app to ever use it (oldest wins, matching
-- CreateApp's own tie-break elsewhere in this schema), a random 4-char suffix
-- for any pre-existing cross-tenant duplicate so the new UNIQUE index below
-- can be added without a manual data fixup.
WITH ranked AS (
    SELECT id, name,
           row_number() OVER (PARTITION BY name ORDER BY created_at, id) AS rn
    FROM apps
)
UPDATE apps a
SET slug = CASE
    WHEN r.rn = 1 THEN r.name
    ELSE r.name || '-' || substr(md5(random()::text || a.id), 1, 4)
END
FROM ranked r
WHERE a.id = r.id;

ALTER TABLE apps ALTER COLUMN slug SET NOT NULL;
CREATE UNIQUE INDEX apps_slug_idx ON apps (slug);
