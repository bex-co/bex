-- w4/m709 (docs/ADR023-usage-metering.md § Bandwidth served to a DDoS attack
-- is not billed): "As part of Render's DDoS protection, Render does not bill for
-- bandwidth usage incurred from a DDoS attack" (render.com/docs/faq, read
-- 2026-11-27).
--
-- egress_attack_windows is a span a bex operator declared a workspace under
-- attack, from the control-plane API only. Every egress_bytes hour a live (not
-- withdrawn) window overlaps is left off the bill: the outbox sends it as 0,
-- it uses none of the plan's included bandwidth, the free-plan spin-down does
-- not count it, the usage reads leave it out and compaction does not fold it
-- into usage_monthly. usage_hourly keeps the bytes it measured, so withdrawing
-- a window puts its unsent hours back on the bill. service_ids narrows a window
-- to some of the workspace's services ('{}' covers all of them).
CREATE TABLE IF NOT EXISTS egress_attack_windows (
    id              text PRIMARY KEY,
    workspace_id    text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    service_ids     text[] NOT NULL DEFAULT '{}',
    starts_at       timestamptz NOT NULL,
    ends_at         timestamptz NOT NULL,
    actor           text NOT NULL,
    reason          text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    withdrawn_at    timestamptz,
    withdrawn_by    text NOT NULL DEFAULT '',
    withdraw_reason text NOT NULL DEFAULT '',
    CONSTRAINT egress_attack_windows_span CHECK (starts_at < ends_at)
);

-- "The live windows of workspace W overlapping hours [a, b)": every billing
-- read asks it per usage row, so it is served from the live rows only.
CREATE INDEX IF NOT EXISTS egress_attack_windows_live_idx
    ON egress_attack_windows (workspace_id, starts_at, ends_at)
    WHERE withdrawn_at IS NULL;

-- One row per declaration, extension and withdrawal, with its actor and
-- reason: the operator audit trail. It is not an audit_events row, which every
-- member of the workspace reads in its audit log, and Render's has no such
-- event.
CREATE TABLE IF NOT EXISTS egress_attack_window_changes (
    window_id text NOT NULL REFERENCES egress_attack_windows (id) ON DELETE CASCADE,
    action    text NOT NULL CHECK (action IN ('declare', 'extend', 'withdraw')),
    starts_at timestamptz NOT NULL,
    ends_at   timestamptz NOT NULL,
    actor     text NOT NULL,
    reason    text NOT NULL,
    at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS egress_attack_window_changes_window_idx
    ON egress_attack_window_changes (window_id, at);

-- Hours Stripe already had when a window covered them are credited back on the
-- workspace's next invoice (t004). Each row is one negative invoice item: the
-- bytes and cents it credits beyond the earlier rows of its workspace-month,
-- recorded before Stripe is called so seq keys the call's idempotency and a
-- second replica's insert of the same seq does nothing. invoice_item_id and
-- sent_at are set once Stripe accepted it.
CREATE TABLE IF NOT EXISTS egress_attack_credits (
    workspace_id    text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    month           date NOT NULL,
    seq             integer NOT NULL CHECK (seq > 0),
    bytes           bigint NOT NULL CHECK (bytes > 0),
    amount_cents    bigint NOT NULL CHECK (amount_cents > 0),
    created_at      timestamptz NOT NULL DEFAULT now(),
    invoice_item_id text NOT NULL DEFAULT '',
    sent_at         timestamptz,
    PRIMARY KEY (workspace_id, month, seq)
);

CREATE INDEX IF NOT EXISTS egress_attack_credits_unsent_idx
    ON egress_attack_credits (created_at)
    WHERE sent_at IS NULL;
