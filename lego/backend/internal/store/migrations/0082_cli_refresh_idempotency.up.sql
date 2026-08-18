-- w4/m38: replica-shared idempotency for the official Render CLI's rotating
-- refresh token. token_hash is SHA-256 of the INBOUND refresh token; the raw
-- credential is never persisted. response_body is kept byte-for-byte so a
-- duplicate request receives the exact OAuth response the first caller saw.
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
