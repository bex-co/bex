-- ADR097 §5, w9/m180 t002: a build source build's fan-out to its linked
-- services is durable. A carrier deploy that went live is claimed with a
-- lease (fanout_claimed_at) and marked done (fanned_out_at) once every linked
-- service has been considered; the reconciler re-runs a live build whose
-- fan-out never completed. Rows of other apps leave both NULL and are never
-- read: the pending query joins build_source carriers only.
ALTER TABLE deploys
    ADD COLUMN fanout_claimed_at timestamptz,
    ADD COLUMN fanned_out_at timestamptz;

-- Builds that went live before this migration were fanned out by the
-- best-effort path; mark them done so the first resync redeploys nothing.
UPDATE deploys d
SET fanned_out_at = COALESCE(d.finished_at, d.updated_at)
FROM apps a
WHERE a.id = d.app_id AND a.type = 'build_source' AND d.status = 'live';
