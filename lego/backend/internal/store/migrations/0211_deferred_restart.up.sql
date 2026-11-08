-- w5/m284: a Restart requested while a deploy is in flight waits for it, as
-- Render's Wait overlap policy does, instead of ending it.
--
-- A Restart re-releases whatever is serving, so it cannot pick its release
-- until the in-flight deploy settles. restart_deferred marks such a row: while
-- it is open it holds the one latest-pending slot (status queued,
-- overlap_pending) with no release yet (generation 0) and no App CR change.
-- When the app has no other open deploy, the deferred-restart worker selects
-- the serving release and promotes the row in place, clearing the flag. A
-- newer trigger or a cancel closes it like any pending row; the flag then
-- means only "this restart never ran".
ALTER TABLE deploys ADD COLUMN restart_deferred boolean NOT NULL DEFAULT false;

CREATE INDEX deploys_restart_deferred_idx ON deploys (created_at)
    WHERE restart_deferred AND status = 'queued';
