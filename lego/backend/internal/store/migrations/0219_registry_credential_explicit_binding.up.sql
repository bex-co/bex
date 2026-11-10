-- w4/m301 (w4/474, w4/476): a registry credential applies only to the
-- services that select it, as on Render. Until now an image service with no
-- registry_credential_id pulled with the newest workspace credential for its
-- image's host. That auto-match is removed, so this one-time backfill writes
-- the credential each such service resolved to as an explicit binding: a
-- private image that pulls today keeps pulling. (Production held no registry
-- credentials when this shipped, so it changed no row there; the backfill is
-- for self-hosted installations.)
--
-- The host rules mirror registrycreds: an image's registry is its first path
-- segment when that holds a dot or colon or is localhost, else Docker Hub;
-- a stored host drops a scheme and trailing slashes, and Docker Hub's aliases
-- fold to docker.io.
WITH image_hosts AS (
    SELECT a.id, a.tenant_id,
           CASE WHEN position('/' IN a.image) > 0
                     AND (split_part(a.image, '/', 1) ~ '[.:]' OR lower(split_part(a.image, '/', 1)) = 'localhost')
                THEN lower(split_part(a.image, '/', 1))
                ELSE 'docker.io' END AS host
    FROM apps a
    WHERE a.registry_credential_id IS NULL
      AND coalesce(a.image, '') <> ''
      AND coalesce(a.repo, '') = ''
), folded_images AS (
    SELECT id, tenant_id,
           CASE WHEN host IN ('index.docker.io', 'registry-1.docker.io', 'registry.hub.docker.com') THEN 'docker.io' ELSE host END AS host
    FROM image_hosts
), credential_hosts AS (
    SELECT id, workspace_id, created_at,
           rtrim(regexp_replace(lower(trim(host)), '^https?://', ''), '/') AS host
    FROM registry_credentials
), folded_credentials AS (
    SELECT id, workspace_id, created_at,
           CASE WHEN host IN ('index.docker.io', 'registry-1.docker.io', 'registry.hub.docker.com') THEN 'docker.io' ELSE host END AS host
    FROM credential_hosts
), resolved AS (
    SELECT DISTINCT ON (i.id) i.id AS app_id, c.id AS credential_id
    FROM folded_images i
    JOIN folded_credentials c ON c.workspace_id = i.tenant_id AND c.host = i.host
    ORDER BY i.id, c.created_at DESC
)
UPDATE apps SET registry_credential_id = resolved.credential_id
FROM resolved
WHERE apps.id = resolved.app_id;
