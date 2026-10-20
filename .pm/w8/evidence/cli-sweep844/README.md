# CLI sweep 844 — service clone still blocked on the `fsn1` region (live, 2026-10-20)

Disposable source service (with a health-check path and one env var) created, clone attempted, source deleted (204, then GET 404). No clone was created; nothing else to clean. No new bug filed.

- `bex services create --name qa844-clone --from <src-id> --confirm` → `Error: region must be one of: frankfurt, ohio, oregon, singapore, virginia`. The clone reads the source's `fsn1` region and the pinned CLI's region enum refuses it client-side.
- This is the already-documented clone failure tracked inside blocked **w8/075** (re-confirmed here at pin `ca30db3a3646`, as in sweep 653-era probes) — recorded as a still-live confirmation, not refiled.
