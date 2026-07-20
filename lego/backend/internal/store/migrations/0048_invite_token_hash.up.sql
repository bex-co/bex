-- w1/041 (deferred from w1/m53 L6): invite tokens hashed at rest. The invite
-- link's token was stored plaintext in tenant_invites.token, so a DB read
-- compromise yielded live, directly-redeemable invite links. Store only
-- sha256(token) (hex): creation writes the hash and carries the plaintext just
-- long enough to email it; acceptance hashes the presented token and looks up
-- by hash. Existing outstanding rows are hashed in place (sha256() is a
-- Postgres 11+ built-in), so links already emailed keep redeeming.
--
-- Companion behavior change (docs/ADR024-members.md): resend now MINTS A NEW
-- token — a hash-at-rest store cannot reproduce the old plaintext for the
-- resent mail, so the freshly emailed link supersedes the original, which
-- stops redeeming.
ALTER TABLE tenant_invites ADD COLUMN token_hash text;
UPDATE tenant_invites SET token_hash = encode(sha256(convert_to(token, 'UTF8')), 'hex');
ALTER TABLE tenant_invites ALTER COLUMN token_hash SET NOT NULL;
ALTER TABLE tenant_invites DROP COLUMN token;

-- Acceptance looks up by the hashed capability; unique doubles as a guard
-- against the (cryptographically negligible) token collision.
CREATE UNIQUE INDEX tenant_invites_token_hash_idx ON tenant_invites (token_hash);
