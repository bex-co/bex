-- w4/m38: replica-shared idempotency for the official Render CLI's rotating
-- refresh token. token_hash is SHA-256 of the INBOUND refresh token; the raw
-- inbound credential is never persisted.
--
-- HISTORY (codex-security 2026-08 F2): response_body originally held Hydra's
-- verbatim token response byte-for-byte — which for this grant INCLUDES the
-- rotated refresh token and the new access token, i.e. a live credential pair,
-- in plaintext. Since 0099 the row is a mint MARKER only (response_body is
-- always empty; the response bytes live solely in the minting process). This
-- file records the original shape for archaeological continuity; do not
-- "restore" the caching behavior it appears to describe.
CREATE TABLE cli_refresh_idempotency (
    token_hash      bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    response_body   bytea NOT NULL CHECK (octet_length(response_body) <= 1048576),
    response_status integer NOT NULL CHECK (response_status BETWEEN 200 AND 299),
    expires_at      timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- Supports the bounded opportunistic expiry sweep on each fresh refresh.
CREATE INDEX cli_refresh_idempotency_expires_idx
    ON cli_refresh_idempotency (expires_at);
