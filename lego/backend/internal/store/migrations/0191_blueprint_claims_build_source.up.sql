-- w3/m270: a Blueprint manages build sources (ADR097 § Blueprints), so a
-- build source name is claimable like a service, database or Key Value name.
ALTER TABLE blueprint_resource_claims
DROP CONSTRAINT blueprint_resource_claims_kind_chk;

ALTER TABLE blueprint_resource_claims
ADD CONSTRAINT blueprint_resource_claims_kind_chk
CHECK (kind IN ('service', 'database', 'key_value', 'build_source'));
