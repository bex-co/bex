-- Custom-domain verification is the traffic DNS record pointing at the
-- service (docs/ADR005-custom-domain.md, amended 2026-10-19: Render's
-- model). The random _bex-challenge TXT is retired: pending rows no longer
-- carry a challenge, so the 0086 constraint requiring one must go. The
-- challenge columns themselves stay for one rolling window — a pre-amendment
-- replica still writes and matches them; this migration only clears values
-- and drops the constraint. Verified rows are untouched; pending rows stay
-- pending and verify from their traffic record instead.
ALTER TABLE domains
    DROP CONSTRAINT domains_pending_challenge_check;

UPDATE domains
SET challenge = NULL
WHERE claim_state = 'pending';

-- The verification sweep's due scan (ListPendingDomainClaims) filters and
-- orders pending rows; keep it off a seq-scan over all verified history.
CREATE INDEX domains_pending_verification_idx
    ON domains (last_verification_at, created_at, id)
    WHERE claim_state = 'pending';
