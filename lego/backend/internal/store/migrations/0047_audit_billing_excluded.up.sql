-- Billing-exclusion changes are privileged (they decide whether a workspace is
-- ever charged), so each toggle writes an audit_events row. This typed nullable
-- boolean records the value it was set TO — the same one-column-per-value shape
-- as maintenance_mode_to (0031), keeping a free-form details object (and any
-- secret) structurally out of audit_events. Nil for every other verb.
ALTER TABLE audit_events ADD COLUMN billing_excluded_to boolean;
