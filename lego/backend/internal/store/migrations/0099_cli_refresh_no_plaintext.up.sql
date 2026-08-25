-- codex-security 2026-08 F2: cli_refresh_idempotency.response_body held
-- Hydra's VERBATIM token response for the CLI's offline_access grant — the
-- newly issued access token AND the rotated refresh token, live credentials
-- carrying bex.read/write/sensitive — as plaintext bytea, readable by anyone
-- with database or backup read access. The 0082 comment claimed "the raw
-- credential is never persisted"; that was true of the INBOUND token only.
--
-- The row is now a MARKER (mint happened for this digest, still inside the
-- rotation-grace window), never a response cache: response bytes live only in
-- the minting process. Scrub every existing row's material and empty the
-- column going forward.
UPDATE cli_refresh_idempotency
   SET response_body = ''::bytea
 WHERE octet_length(response_body) > 0;
