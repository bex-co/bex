ALTER TABLE deploys
    DROP COLUMN resolved_image,
    DROP COLUMN rollback_of,
    DROP COLUMN generation;
