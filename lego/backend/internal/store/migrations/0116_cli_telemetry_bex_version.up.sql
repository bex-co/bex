-- w5/m94: the bex launcher's own release identity, alongside the upstream pin.
--
-- cli_version (0113) holds the pinned render-oss/cli release, which also names
-- the User-Agent and is what the compatibility ledger tracks -- so it cannot
-- also carry bex's release. The launcher stamps X-Bex-CLI-Version on requests
-- to bex-api and the ingest handler reads it here. Empty means the reporter
-- did not send one: an unmodified upstream `render` binary pointed at bex, or
-- any pre-m94 bex build. Never rejected, never inferred.
ALTER TABLE cli_telemetry_events
    ADD COLUMN bex_version text NOT NULL DEFAULT '';
