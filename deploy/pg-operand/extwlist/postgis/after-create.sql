-- pgextwlist runs this as the bootstrap superuser right after CREATE EXTENSION
-- postgis (ADR092 D3). The extension's objects belong to that superuser, so
-- give the database owner what an ordinary PostGIS app needs: registering its
-- own spatial reference systems.
GRANT SELECT, INSERT, UPDATE, DELETE ON @extschema@.spatial_ref_sys TO "@database_owner@";
