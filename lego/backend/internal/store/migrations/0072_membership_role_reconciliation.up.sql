-- Durable outbox for the Postgres -> OpenFGA half of membership role changes.
-- tenant_members is the source of truth; one row per member carries only the
-- latest desired role, so an older failed downgrade can never overwrite a newer
-- administrator change when the worker retries.
CREATE TABLE membership_role_reconciliations (
    tenant_id text NOT NULL,
    subject text NOT NULL,
    role text NOT NULL CHECK (role IN ('admin', 'developer', 'contributor', 'viewer', 'billing')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, subject),
    FOREIGN KEY (tenant_id, subject) REFERENCES tenant_members(tenant_id, subject) ON DELETE CASCADE
);

CREATE INDEX membership_role_reconciliations_due_idx
    ON membership_role_reconciliations (next_attempt_at, updated_at);
