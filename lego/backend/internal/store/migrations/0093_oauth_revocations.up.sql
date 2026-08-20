CREATE TABLE IF NOT EXISTS oauth_revocations (
    subject text NOT NULL,
    client_id text NOT NULL,
    revoked_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (subject, client_id)
);
