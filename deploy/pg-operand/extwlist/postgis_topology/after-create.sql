-- CreateTopology and AddTopoGeometryColumn run as the caller and record each
-- topology and layer in these metadata tables (ADR092 D3).
GRANT USAGE ON SCHEMA topology TO "@database_owner@";
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA topology TO "@database_owner@";
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA topology TO "@database_owner@";
