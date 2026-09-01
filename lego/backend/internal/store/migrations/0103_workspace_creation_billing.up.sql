-- m90: workspace billing belongs to the workspace being created. Provisional
-- attempts deliberately live outside tenants so no existing tenant/resource
-- read can expose an unpaid workspace.
ALTER TABLE tenants
    ADD COLUMN billing_email text NOT NULL DEFAULT '';

CREATE TABLE workspace_creation_attempts (
    id text PRIMARY KEY,
    workspace_id text NOT NULL UNIQUE,
    owner_subject text NOT NULL,
    name text NOT NULL,
    plan text NOT NULL CHECK (plan IN ('hobby', 'pro', 'scale', 'enterprise')),
    billing_email text NOT NULL,
    payment_required boolean NOT NULL,
    state text NOT NULL DEFAULT 'prepared'
        CHECK (state IN ('prepared', 'setup_pending', 'setup_succeeded', 'finalized', 'cleanup_pending', 'expired')),
    provider_customer_id text NOT NULL DEFAULT '',
    provider_setup_intent_id text NOT NULL DEFAULT '',
    provider_payment_method_id text NOT NULL DEFAULT '',
    provider_subscription_id text NOT NULL DEFAULT '',
    provider_livemode boolean,
    expires_at timestamptz NOT NULL,
    cleanup_claimed_until timestamptz,
    finalized_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (billing_email = lower(btrim(billing_email)) AND billing_email <> ''),
    CHECK ((state = 'finalized' AND finalized_at IS NOT NULL) OR state <> 'finalized')
);

CREATE INDEX workspace_creation_attempts_owner_idx
    ON workspace_creation_attempts (owner_subject, updated_at DESC);
CREATE INDEX workspace_creation_attempts_cleanup_idx
    ON workspace_creation_attempts (expires_at, id)
    WHERE state NOT IN ('finalized', 'expired');
CREATE INDEX workspace_creation_attempts_retention_idx
    ON workspace_creation_attempts (updated_at, id)
    WHERE state IN ('finalized', 'expired');
