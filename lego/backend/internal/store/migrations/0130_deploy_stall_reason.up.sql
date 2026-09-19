-- w4/m112: why an OPEN deploy is not progressing.
--
-- failure_reason and cancel_reason are both terminal by contract — they are
-- stamped as the row closes — so nothing named the cause of a rollout that is
-- still in flight. Live on 2026-09-17 a service whose Health Check Path 404ed
-- sat update_in_progress for the full 900s budget while the deploy page, the
-- events feed and the log stream were all bare: a user could not tell a
-- failing probe from a slow image pull until the budget expired.
--
-- The control plane now projects the operator's own Ready-condition diagnosis
-- onto the open row each pass, and clears it when the row goes terminal (at
-- which point failure_reason owns the story). Empty string, not NULL: it is a
-- transient observation with a natural "nothing to report" value, and every
-- read path treats "" as absent — the same convention failure_reason uses.
ALTER TABLE deploys
  ADD COLUMN IF NOT EXISTS stall_reason text NOT NULL DEFAULT '';
