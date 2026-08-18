-- A custom domain is a durable ownership claim before it is serving intent.
-- Existing rows were already routed by the projector, so they are backfilled
-- verified. New claim-aware writers explicitly create pending rows with an
-- application-generated, cryptographically-random TXT challenge; the verified
-- schema default keeps synchronous-proof writers safe during a rolling rollout.
ALTER TABLE domains
    -- verified keeps pre-m84 writers safe during a rolling rollout: those
    -- writers still run the synchronous ownership gate. New claim writers
    -- explicitly insert pending together with a random challenge.
    ADD COLUMN claim_state text NOT NULL DEFAULT 'verified',
    ADD COLUMN challenge text,
    ADD COLUMN challenge_version bigint NOT NULL DEFAULT 1,
    ADD COLUMN verification_attempts bigint NOT NULL DEFAULT 0,
    ADD COLUMN last_verification_at timestamptz,
    ADD COLUMN verified_at timestamptz;

UPDATE domains
SET claim_state = 'verified',
    verified_at = created_at;

ALTER TABLE domains
    ADD CONSTRAINT domains_claim_state_check
        CHECK (claim_state IN ('pending', 'verified')),
    ADD CONSTRAINT domains_pending_challenge_check
        CHECK (claim_state = 'verified' OR (challenge IS NOT NULL AND challenge <> '')),
    ADD CONSTRAINT domains_challenge_version_check
        CHECK (challenge_version > 0),
    ADD CONSTRAINT domains_verification_attempts_check
        CHECK (verification_attempts >= 0);
