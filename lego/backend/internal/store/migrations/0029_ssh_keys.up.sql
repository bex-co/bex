-- w2/m39: identity-scoped SSH public keys. Only public material is stored;
-- fingerprint is OpenSSH's SHA256 form and is globally unique so one presented
-- key can resolve to exactly one authenticated subject at the SSH gateway.

CREATE TABLE ssh_keys (
    id          text PRIMARY KEY,
    subject     text NOT NULL,
    name        text NOT NULL,
    public_key  text NOT NULL,
    fingerprint text NOT NULL UNIQUE,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ssh_keys_subject_idx ON ssh_keys (subject, created_at, id);
