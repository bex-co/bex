-- Refuse to erase the only distinction that prevents an unverified claim from
-- becoming live after a downgrade. Operators must verify or delete pending
-- rows before rolling back this migration.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM domains WHERE claim_state = 'pending') THEN
        RAISE EXCEPTION 'cannot downgrade domain claim state while pending claims exist';
    END IF;
END
$$;

ALTER TABLE domains
    DROP CONSTRAINT domains_verification_attempts_check,
    DROP CONSTRAINT domains_challenge_version_check,
    DROP CONSTRAINT domains_pending_challenge_check,
    DROP CONSTRAINT domains_claim_state_check,
    DROP COLUMN verified_at,
    DROP COLUMN last_verification_at,
    DROP COLUMN verification_attempts,
    DROP COLUMN challenge_version,
    DROP COLUMN challenge,
    DROP COLUMN claim_state;
