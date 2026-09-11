-- w8/m38: durable Blueprint auto-sync intents accepted at the signed git
-- webhook before acknowledgment. One logical intent per (delivery_digest,
-- blueprint_id); the bounded worker claims pending rows and pins commit_sha
-- into ReviewedBlueprintSource on execute.
CREATE TABLE blueprint_auto_sync_intents (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    blueprint_id TEXT NOT NULL REFERENCES blueprints (id) ON DELETE CASCADE,
    delivery_digest TEXT NOT NULL,
    commit_sha TEXT NOT NULL,
    path TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    claimed_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    CONSTRAINT blueprint_auto_sync_intents_state_chk
    CHECK (state IN ('pending', 'claimed', 'completed', 'failed')),
    CONSTRAINT blueprint_auto_sync_intents_delivery_blueprint_uq
    UNIQUE (delivery_digest, blueprint_id)
);

-- Pending claim: oldest-first admission for the worker (FOR UPDATE SKIP LOCKED).
CREATE INDEX blueprint_auto_sync_intents_pending_claim_idx
ON blueprint_auto_sync_intents (state, created_at)
WHERE state IN ('pending', 'claimed');
