DROP TABLE IF EXISTS sandbox_group_credential_versions;
DROP TABLE IF EXISTS sandbox_group_credentials;
ALTER TABLE sandbox_groups DROP COLUMN IF EXISTS credential_fingerprint_salt;
