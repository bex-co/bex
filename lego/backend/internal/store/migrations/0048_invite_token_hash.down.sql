-- Hashing is one-way: the plaintext cannot be recovered, so rolling back
-- restores the column shape with empty tokens — outstanding invite links stop
-- redeeming by token (login-time email match still works) until re-invited.
DROP INDEX tenant_invites_token_hash_idx;
ALTER TABLE tenant_invites ADD COLUMN token text NOT NULL DEFAULT '';
ALTER TABLE tenant_invites DROP COLUMN token_hash;
