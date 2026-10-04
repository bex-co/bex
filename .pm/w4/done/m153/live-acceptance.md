# m153 live acceptance — 2026-10-02 (UTC 2026-10-03T07:06–07:16Z)

**Deployed:** `bex-controller-manager` / `bex-api` `ghcr.io/bex-co/bex-operator@sha256:e396cf3585ac…` and the dashboard, from GitOps pin `20fb64867` → `1263d12ae`. That commit contains both `c92ebb526` and `125f4ddfc`, as well as the requested `18958459c`. **Fixture:** Free, public Key Value `qa-20261003-kv-durable-m153` / `red-db0aim2v5tvc73ejdm8g` on Valkey 8. Created 07:06:00Z with `journal-snapshot` and `allkeys-lru`; Available at 07:07:02Z.

**Client:** the finding's external TLS client: `redis-cli --tls --sni <host> --cacert /etc/ssl/cert.pem -h <host> -p 6379 --user default --raw`. Each command was a separate process. The password came from GraphQL `keyValueConnectionInfo` into `REDISCLI_AUTH` only; it was never printed or stored. Runtime mode was read from both `INFO persistence` `aof_enabled` and `CONFIG GET appendonly`. Expiry was read with `PEXPIRETIME`.

## Sequence 1 — Journal → Snapshot → new writes → Journal

| UTC | Step | Result |
| --- | --- | --- |
| 07:07:17 | Baseline `qa:baseline`, `INCR qa:counter`, `qa:ttl EX 3600` | baseline present, counter 1, `PEXPIRETIME 1791014836983`, aof 1 / appendonly yes |
| 07:07:41 | `setKeyValuePersistenceMode snapshot` | GraphQL/REST `config_restart` until 07:08:39, then `available` |
| 07:08:40 | Read | baseline, counter 1, same PEXPIRETIME; aof 0 / appendonly **no**; uptime 30 s (restarted) |
| 07:09:06 | `SET qa:snapshot-new` (no TTL), `INCR` → 2 | dbsize 4 |
| 07:09:24 | Switch to `journal_snapshot` | `config_restart` until 07:09:49, then `available` |
| 07:09:51 | Read after restart (uptime 32 s) | **`qa:snapshot-new` present, counter 2**, baseline intact, same PEXPIRETIME, aof 1 / appendonly **yes** |

(Before the fix, this step lost `snapshot-new` and rolled the counter back to 1.)

## Sequence 2 — SAVE control, ordinary restart, then switch via the dashboard UI

| UTC | Step | Result |
| --- | --- | --- |
| 07:10:17 | Switch to `snapshot` | `config_restart` → `available` 07:10:42 |
| 07:10:47 | `SET qa:marker2`, `SET qa:counter 23`, `SAVE` | OK; aof 0 |
| 07:11:04 | `setKeyValueMaxmemoryPolicy noeviction` | `config_restart` → `available` 07:11:34 |
| 07:11:35 | Read after the ordinary restart (uptime 34 s) | **marker2 present, counter 23**, all earlier keys, same PEXPIRETIME, `maxmemory-policy noeviction` |
| 07:12:33 | Fresh detail page in an independent headless Chromium. The shared MCP browser was busy. | Status Available (API mode `snapshot`) |
| 07:12:34 | UI: Edit persistence mode → Journal + Snapshot → Save changes. No typed confirm for a durable→durable change. | `setKeyValuePersistenceMode` returned `journal_snapshot`. The Status row showed **Restarting** from +4 s to +16 s, then Available at +20 s. 0 console errors. |
| 07:13:05 | Read after the restart (uptime 41 s) | **marker2 and counter 23 survive**; baseline, snapshot-new and ttl present; same PEXPIRETIME; aof 1 / appendonly yes; dbsize 5 |

## Cross-surface coherence

- After the UI switch, GraphQL `{status: available, persistenceMode: journal_snapshot, maxmemoryPolicy: noeviction}` matches REST `options {persistenceMode: journal_snapshot, maxmemoryPolicy: noeviction}`, MCP `get_key_value` and the reloaded dashboard card ("Journal + Snapshot"). During each handoff, GraphQL and REST both reported `config_restart` and the UI showed "Restarting", never Available, until conversion had finished.
- Card copy: "Switching between Journal + Snapshot and Snapshot only preserves data. Switching to or from Off discards all data."
- Normalization: `dryRun` with `journal-snapshot` and with `journal_snapshot` both return `journal_snapshot`; `snapshot` returns `snapshot`. No restart was triggered and the status stayed `available`.

## Rename and cleanup

- `renameKeyValue` → `qa-20261003-kv-m153-rn`. A 35-character name was first rejected by the existing 30-character validation, as designed. ID, `externalHost` and the SHA-256 fingerprint of the external/internal connection strings and CLI command (`bea4dfc12ba390ec`) were identical before and after. REST, MCP and the reloaded UI showed the new name with the same ID, and data still read back over TLS.
- `deleteKeyValue` 07:14:33Z → `true`. Before the delete, the KeyValue carried `app.bex.co/kv-tls-cleanup` and a Ready Certificate, with TLS Secret UID `b0d03965-…`. At 07:15:19Z the KeyValue, Certificate, CertificateRequest, StatefulSet, pod, Service, PVC and all three Secrets were gone. REST returned 404 and the GraphQL list omitted the fixture. The session was revoked at the end of the run.

**Not live-exercised** (covered by the real-Valkey 7.2/8.1 matrix and controller tests in verification.md): Valkey 7, paid plans, Off transitions, interrupted/failed handoffs, Blueprint and direct CR writers.
