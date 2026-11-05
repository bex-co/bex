DELETE FROM blueprint_resource_claims WHERE kind = 'build_source';

ALTER TABLE blueprint_resource_claims
DROP CONSTRAINT blueprint_resource_claims_kind_chk;

ALTER TABLE blueprint_resource_claims
ADD CONSTRAINT blueprint_resource_claims_kind_chk
CHECK (kind IN ('service', 'database', 'key_value'));
