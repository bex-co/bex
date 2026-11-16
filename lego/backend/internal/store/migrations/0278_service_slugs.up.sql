-- Every slug (public subdomain) a service has had, never released: Render never
-- reuses a deleted service's subdomain (docs/render-artifacts/duplicate-service-names.md,
-- Rule 4), and a slug minted again would let a CNAME left at the old host verify
-- for the new holder (ADR005 § Accepted residuals). CreateApp claims a slug here
-- and takes a suffix when it is held. No foreign keys, since rows outlive their
-- service and workspace; only RollbackAppCreation removes one, for a create that
-- never completed.
CREATE TABLE service_slugs (
    slug       text PRIMARY KEY,
    service_id text NOT NULL
);

-- Records an insert by a bex-api replica still on the previous release while
-- this one rolls out, which claims nothing (it may reuse a retired slug in that
-- window, but the slug is recorded from then on).
CREATE FUNCTION bex_record_service_slug()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO service_slugs (slug, service_id) VALUES (NEW.slug, NEW.id)
    ON CONFLICT (slug) DO NOTHING;
    RETURN NULL;
END;
$$;

CREATE TRIGGER apps_record_service_slug
AFTER INSERT ON apps
FOR EACH ROW EXECUTE FUNCTION bex_record_service_slug();

-- Every live service keeps the slug it has: no host changes.
INSERT INTO service_slugs (slug, service_id)
SELECT slug, id FROM apps
ON CONFLICT (slug) DO NOTHING;

-- Services deleted before this migration took their slugs with their rows. The
-- names bex still records for them are what is left: the creation name kept for
-- usage catch-up (usage_retired_resources) and the last shown name kept for
-- billing lines (resource_display_names). A slug was its service's creation name
-- unless that was taken, so this retires every bare slug those services held. A
-- suffixed one is lost, and a renamed service retires its shown name, so that
-- name's next service takes a suffix it would not have needed. Build source
-- carriers (bds-) are skipped: their slug is their id, never a display name.
INSERT INTO service_slugs (slug, service_id)
SELECT r.name, r.resource_id
FROM usage_retired_resources r
WHERE r.resource_kind = 'service' AND r.resource_id LIKE 'srv-%'
  AND r.name ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'
UNION ALL
SELECT d.display_name, d.resource_id
FROM resource_display_names d
WHERE d.resource_kind = 'service' AND d.resource_id LIKE 'srv-%'
  AND d.display_name ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'
  AND NOT EXISTS (SELECT 1 FROM apps a WHERE a.id = d.resource_id)
ON CONFLICT (slug) DO NOTHING;
