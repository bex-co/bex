# CLI sweep 543 — reads against a deleted service

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD-built CLI, 2026-10-19T10:55–10:58Z.

One free web fixture created → `live` → one seeded request (3 log rows confirmed readable pre-delete: request + app lines) → deleted by exact ID → read probes against the dead ID:

| probe | observed result |
| --- | --- |
| `logs -r <deleted-id> -o json` and `-o text` | Exit `1`, server `404 (NOT_FOUND): not found` — no zombie access to the deleted service's log history through either output mode. |
| `services instances <deleted-id>` | Exit `1`, `404 Not Found` (deploy reads after delete were the same named 404 in sweeps 501/515). |

Deleted means gone on every read surface probed, with the same not-found shape a synthetic ID gets (sweep 504) — no existence-after-death oracle distinguishing "deleted" from "never existed" beyond that shared 404.

- A mixed multi-resource query (`-r <live baseline>,<deleted id>`, read-only) fails closed with the same `404` rather than returning the live half — one unresolvable resource poisons the combined read. Consistent with the single-resource behavior; recorded because sweep 460's multi-resource success case had both resources live.
- Unrun: events after delete.
- Deploy lag means deployed revisions can precede local `main` (production pin `8e57d4eaa121` at sweep time; w8/129, w8/130 fixes are on main ahead of it, w8/131's fix in progress in-tree).
