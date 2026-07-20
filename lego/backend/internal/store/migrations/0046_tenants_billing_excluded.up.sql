-- Billing exclusion (docs/ADR040-billing-metronome.md §7, Mode A). A genuinely
-- internal / superadmin / test workspace must never enter Metronome — no
-- customer, no events, no contract, no invoice — so collection is structurally
-- impossible, not merely configured off. It still sees estimatedCost (a
-- metering-layer read, independent of billing state). Default false ⇒ every
-- existing workspace is billable, byte-identical to before. This flag decides
-- whether money is owed, so it is admin-only, set through the control-plane
-- internal API, and written to audit_events (never tenant-editable).
ALTER TABLE tenants ADD COLUMN billing_excluded boolean NOT NULL DEFAULT false;
