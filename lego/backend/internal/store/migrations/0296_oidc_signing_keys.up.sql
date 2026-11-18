-- w2/m476 (docs/ADR116-managed-oidc.md D5, D6): the key set every workspace's
-- Managed OIDC issuer publishes and signs with. One ES256 P-256 key per row,
-- the private key as PKCS#8 DER; kid is the public key's RFC 7638 thumbprint.
-- A key is published from created_at and signs from activates_at; the latest
-- key whose activates_at has passed is the one that signs. bex-api inserts the
-- first key itself (under an advisory lock, so replicas starting together
-- insert one) and `/api oidc-rotate-key` inserts the rest.
CREATE TABLE IF NOT EXISTS oidc_signing_keys (
    kid          text        PRIMARY KEY,
    private_key  bytea       NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    activates_at timestamptz NOT NULL,
    CHECK (activates_at >= created_at)
);
