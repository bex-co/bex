-- Restoring the pending-challenge constraint is only possible while no
-- pending row lacks a challenge: a traffic-era pending row has no TXT value
-- to give back. Refuse loudly instead of inventing challenges tenants never
-- published; operators verify or delete those rows before downgrading.
DROP INDEX IF EXISTS domains_pending_verification_idx;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM domains WHERE claim_state = 'pending' AND (challenge IS NULL OR challenge = '')) THEN
        RAISE EXCEPTION 'cannot restore domain TXT challenges while challengeless pending claims exist';
    END IF;
END
$$;

ALTER TABLE domains
    ADD CONSTRAINT domains_pending_challenge_check
        CHECK (claim_state = 'verified' OR (challenge IS NOT NULL AND challenge <> ''));
