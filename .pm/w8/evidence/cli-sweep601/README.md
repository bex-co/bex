# CLI sweep 601 — `persistence-mode off` with live keys: accepted, keys gone after the roll

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `1a49cb806`, 2026-10-19. Owned fixture `qa601-kv` (`red-dbb9m7qbpdss73ci0mvg`, free, external access temporarily enabled), deleted at close (get → not-found, list clean). Completes sweep 600's Unrun item.

- Setup: `SET qa601key valore601 → OK`, read back present under `journal_snapshot`.
- `keyvalues update --persistence-mode off` succeeded with no extra confirmation or phrase (beyond the standard `--confirm`), rolled the instance (`config_restart` → `available`, mode `off`), and after settling `GET qa601key` returned empty: the roll restarts the pod, and with persistence off the data does not survive it. Live `CONFIG GET` agrees (`appendonly no`, `save` empty).
- Not filed: this is the flag's own documented contract — the pinned help text for `--persistence-mode` states `off (off loses data on restart)`. The destructive consequence is disclosed at the exact surface that triggers it; there is no silent-loss discrepancy to report. (The MCP-side confirm phrase guards a different transition — the first external enable on an already-off instance.)

## Unrun

- Eviction behavior at the free memory ceiling (`allkeys-lru` vs `noeviction` under pressure).
- Persistence `off` → `journal_snapshot` re-enable: whether new writes after re-enable are durable across a later roll.
