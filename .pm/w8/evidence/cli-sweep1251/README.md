# CLI sweep 1251 — free Key Value at its memory ceiling: eviction and noeviction (live, 2026-10-20)

One free Key Value (`red-dbbjga6cbnls73997g7g`, `bex-canary`) created with `bex kv create --plan free --ip-allow-list cidr=<own IPv4>/32,…`. Data-plane access went through private `redis-cli --tls --sni` with the server-issued credentials (never printed), over IPv4 to avoid the w9/m164 / w8/140 IPv6 path. Deleted with `bex kv delete --confirm` (GET 404). Closes sweep 601's Unrun item. No new bug filed.

- Defaults: `maxmemory 107374182` (102.4 MB, 80% of the free plan's 128 Mi) with `allkeys-lru`, matching the API's `maxmemoryPolicy: allkeys_lru`.
- `allkeys-lru` under pressure: 140 × 1 MB `SET`s in one pipeline → 0 errors, `evicted_keys 60`, `DBSIZE 80`, `used_memory 101.15M`. The oldest key is evicted and the newest kept. The server did not restart (uptime kept growing), so there was no container OOM.
- `bex kv update <id> --memory-policy noeviction --confirm` → `config_restart`, then `available` in about 20 s. Live `CONFIG GET maxmemory-policy` reads `noeviction`, and the 80 keys survived the restart (journal_snapshot). Forty more 1 MB `SET`s → 40 × `OOM command not allowed when used memory > 'maxmemory'.` Render's documented noeviction behavior.
- One `api.bex.co` TLS handshake timed out from this client mid-sweep; the next requests answered in under 0.3 s (local network blip, also seen in sweep 1238). Not filed.
