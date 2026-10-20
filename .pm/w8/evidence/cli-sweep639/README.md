# CLI sweep 639 — `idleTTLSeconds` PATCH dialects: nested-only field with a named range

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixture `qa639-web` (`srv-dbbb5civfpec73dm0ikg`, free hello-app), deleted at close: REST 404, zero `qa639` names. REST boundary.

- **Accepted values:** 300, 0, 59, and 86401 each stored and read back (`idleTTLSeconds: 86401` final) — the full documented span is writable on the free plan, including values above a day.
- **Range named:** `-1` → `400 idleTTLSeconds must be 0-604800`. (86401 ≤ 604800, so the probes stayed in range; the ceiling itself was not crossed by a valid-shaped value in this sweep — see Unrun.)
- **Type and placement:** `"300"` as a string → `400 … invalid value for field "serviceDetails.idleTTLSeconds"`; the top-level spelling `{idleTTLSeconds: …}` → `400 request body contains unknown field "idleTTLSeconds"` — the field exists only nested, like `port` (sweep 611) but without the top-level alias.

## Unrun

- 604800 vs 604801 (the named ceiling's exact edge).
- Actual spin-down/wake timing at a small TTL (needs an idle watch).
