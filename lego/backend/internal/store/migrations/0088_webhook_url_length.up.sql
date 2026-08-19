-- codex round-15 #2: bound webhook destination length. The shared worker
-- reloads every enabled endpoint every two seconds; an HTTPS URL was previously
-- unbounded (only the 2 MiB request body cap). Rewrite any legacy over-limit
-- row to a non-routable sentinel so the CHECK can land, and disable it.
UPDATE webhook_endpoints
SET enabled = false,
    disabled_reason = 'url exceeds maximum length',
    url = 'https://invalid.invalid/oversized-legacy-url'
WHERE octet_length(url) > 2048;

ALTER TABLE webhook_endpoints
    ADD CONSTRAINT webhook_endpoints_url_len_chk
    CHECK (octet_length(url) <= 2048);
