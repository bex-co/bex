-- pgextwlist runs this as the bootstrap superuser right after CREATE EXTENSION
-- pg_repack (ADR092 D3). A table's owner runs the pg_repack client with
-- --no-superuser-check (pg_repack 1.5.2+): it reads the repack schema's views
-- and creates its log table, key type and copy of the table in that schema.
GRANT USAGE, CREATE ON SCHEMA repack TO "@database_owner@";
GRANT SELECT ON ALL TABLES IN SCHEMA repack TO "@database_owner@";
-- repack_trigger is SECURITY DEFINER and inserts into the log table the owner
-- created. Owned by the bootstrap superuser it would run the owner's own
-- triggers and defaults on that table as superuser; owned by the database
-- owner it runs with exactly the owner's rights (property 2).
ALTER FUNCTION repack.repack_trigger() OWNER TO "@database_owner@";
