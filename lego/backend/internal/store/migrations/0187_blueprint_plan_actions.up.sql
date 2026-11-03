-- w2/m256 t012: a planned sync keeps the plan it was approved against, so the
-- dashboard can show it (and its Approve control) without planning again.
ALTER TABLE blueprint_syncs ADD COLUMN plan_actions jsonb;
