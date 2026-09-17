-- A services inventory batch has two dimensions with two different sources:
-- existence comes from the apps table (authoritative even when Kubernetes is
-- unreachable) and state comes from the live App CR list. One `complete` flag
-- described both, so a failed Kubernetes list still produced a batch stamped
-- authoritative with every service's state collapsed to 'unknown' — and nothing
-- downstream could tell that apart from a genuine platform-wide outage.
--
-- states_observed records whether the state dimension was actually sampled. The
-- count series stays unbroken either way; only its state column becomes suspect.
ALTER TABLE product_inventory_batches
    ADD COLUMN IF NOT EXISTS states_observed boolean NOT NULL DEFAULT true;

COMMENT ON COLUMN product_inventory_batches.states_observed IS
    'False when the batch''s resource states could not be sampled (the Kubernetes list failed) and every state defaulted to unknown. Resource counts remain trustworthy; per-state breakdowns for this bucket do not.';
