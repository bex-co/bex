-- w4/m379: Render's postgres_read_replicas_changed requires fromReplicas and
-- toReplicas. The replica row recorded no value until now (w5/m138). One typed
-- column per side, like every other per-verb audit detail; NULL for every
-- other verb and for rows recorded before these columns existed, which
-- Retrieve event then renders without the pair rather than inventing one.
ALTER TABLE audit_events
    ADD COLUMN read_replicas_from integer,
    ADD COLUMN read_replicas_to integer;
