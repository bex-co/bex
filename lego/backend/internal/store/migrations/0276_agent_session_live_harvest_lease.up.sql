-- w4/677: every bex-api replica's Completer harvests a running turn's driver
-- log about once a minute, and each harvest of a long session's 16 MiB log is
-- expensive, so two replicas doubled the work for nothing. A replica now takes
-- this lease before a running turn's harvest (ClaimAgentSessionLiveHarvest)
-- and skips it while another replica's lease is still current. NULL until the
-- first harvest.
ALTER TABLE agent_sessions ADD COLUMN live_harvest_until timestamptz;
