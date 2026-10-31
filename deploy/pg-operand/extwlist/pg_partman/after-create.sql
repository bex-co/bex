-- pg_partman is configured by editing its part_config tables (retention,
-- premake, ...), and run_maintenance() runs as the caller. Without its
-- background worker (ADR092 D3) only the owner ever runs it.
GRANT SELECT, INSERT, UPDATE, DELETE ON @extschema@.part_config, @extschema@.part_config_sub TO "@database_owner@";
