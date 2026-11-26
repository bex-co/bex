-- w1/m659: two facts Render's deploy page Notices read off a deploy that the
-- row did not keep. A Blueprint sync's deploy ("Synced from <Blueprint>")
-- records the Blueprint and the sync run that opened it, durable blp- and
-- bsr- keys, NULL for every other deploy and for rows written before. A
-- settings deploy that moved the service from a paid plan to the free one
-- ("Updated to a free compute plan") records that move.
ALTER TABLE deploys
    ADD COLUMN blueprint_id text,
    ADD COLUMN blueprint_sync_id text,
    ADD COLUMN free_plan_transition boolean NOT NULL DEFAULT false;
