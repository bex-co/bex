-- The geocoder's functions read and its loader writes the tiger schema's
-- lookup tables as the caller (ADR092 D3).
GRANT USAGE ON SCHEMA tiger TO "@database_owner@";
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA tiger TO "@database_owner@";
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA tiger TO "@database_owner@";
