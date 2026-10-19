# CLI sweep 600 — Key Value data plane: mid-life policy/persistence changes with live data

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `1a49cb806`, 2026-10-19. Owned fixture `qa600-kv` (`red-dbb9hbsgnrlc73bhss9g`, free), deleted at close (get → not-found, list clean). Completes sweep 599's Unrun item. Credentials were only ever handled inside private helper processes; nothing sensitive is printed here.

- **Getting a data plane:** `bex kv-cli` is TTY-only (passthrough args included — upstream guard). With external access disabled (default, empty IP allowlist), the server-provided `cliCommand` targets the internal `<id>.<workspace>.svc:6379` host, which a laptop cannot resolve (`nodename nor servname provided`). This is deliberate and documented in [service.go](/lego/backend/internal/keyvalue/service.go:1114): the cliCommand follows the reachable endpoint, internal unless public — mirroring Render's model. Not filed.
- **Enabling external access:** setting an IP allowlist moved the instance through `config_restart` back to `available`; the first connection-info read during the roll answered `409 key value credentials are still converging`, then converged. The cliCommand switched to `redis-cli --sni <id>.kv.bex.co -u rediss://…` and a session connected (prompt at `<id>.kv.bex.co:6379`).
- **Official invocation matters:** plain `redis-cli -h <host>` failed (`Server closed the connection`; with `--tls`, `SSL_connect failed`) — only the server-issued `--sni … -u rediss://…` form connects, because the SNI router needs the explicit server name. Via that form: `PING → PONG`, `SET qa600key valore600 → OK`, `GET → valore600`.
- **Mid-life changes with data present:** `--persistence-mode snapshot` and `--memory-policy volatile_ttl` each rolled the instance (`config_restart` → `available` with the new values). After settling, `GET qa600key` still returned `valore600`, and live `CONFIG GET` agreed with the API: `maxmemory-policy volatile-ttl`, `appendonly no`, and snapshot `save` points (`3600 1 300 100 60 10000`).

## Unrun

- `persistence-mode off` with keys present (documented data-loss semantics; refused-or-warned behavior not probed).
- Eviction behavior at the free plan's memory ceiling under `allkeys-lru` vs `noeviction`.
