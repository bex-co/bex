-- w6/m50 t001: a failed sync's real error was computed and then discarded
-- before the UPDATE that stamps the run's terminal state, leaving every
-- error-state blueprint_syncs row with no forensic trail at all.
ALTER TABLE blueprint_syncs
    ADD COLUMN error_message TEXT;
