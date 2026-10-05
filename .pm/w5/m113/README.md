# w5 · m113 — Availability failure edges carry the Ready transition time (re-scopes w4/200)

**Worker:** worker5 **Goal:** `server_failed` and its datastore twins are stamped at the Ready transition even when the debounced first unhealthy pass moved the checkpoint, so outages aren't understated. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Replay the reconciler's real observation sequence in the PG test | 30m | — |
| t002 | Bound service failure edges by the last availability change | 45m | t001 |
| t003 | Apply the same bound to datastores | 30m | t002 |
| t004 | Replay w4/200's probe live and settle w4/200 | 30m | t003 |
| t005 | Render parity | 15m | t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- A store test replaying the reconciler's sequence (healthy → debounced pass with phase Deploying → unhealthy) stamps `server_failed` at the Ready transition; the datastore twin does the same.
- The recovery edge and the anti-flap debounce are unchanged.
- A live replay of w4/200's probe shows `server_failed` within ~10 s of the first 503, and w4/200 is closed or re-scoped with that evidence.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `44a612e8d` (w4/196), plus w4/200 filed the same day, which attributes the remaining ~40 s to operator Ready lag (cause marked unverified). Code-verified mechanism: the debounced first unhealthy pass still records the phase change, which moves the checkpoint's `updated_at` past the Ready transition. `availabilityEdgeAt` then falls back to the next pass's observation time, one 30 s resync late, matching the ~38–40 s measured before and after w4/196. The recovery edge has no debounce and is exact, as observed.
- **Goal linkage:** ADR010 observability and ADR052 notifications: truthful incident timing.
- **Expected outcome:** The Events feed, webhooks and push report when an outage started to within one probe period.
- **Why now:** Keeps w4/200's operator diagnosis from chasing the wrong component, and finishes a fix shipped on 2026-10-04.
- **Render parity included:** event timestamps change on every surface.
- **Re-scopes w4/200:** a pointer was added to `w4/200.md`; t004 settles it.
