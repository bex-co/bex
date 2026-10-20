-- A checked dispatch must clear only the rollback image it accepted. Source
-- revisions distinguish a newer identical-image rollback without letting a
-- replicas, tier, or display-name write supersede unrelated source work.
ALTER TABLE apps
    ADD COLUMN source_revision bigint NOT NULL DEFAULT 0;

CREATE FUNCTION bump_app_source_revision()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.source_revision := OLD.source_revision + 1;
    RETURN NEW;
END;
$$;

CREATE TRIGGER apps_source_revision
    BEFORE UPDATE OF repo, image, branch ON apps
    FOR EACH ROW
    EXECUTE FUNCTION bump_app_source_revision();
