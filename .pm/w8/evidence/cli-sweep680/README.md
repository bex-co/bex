# CLI sweep 680 — owners and members reads (live, 2026-10-19)

Read-only.

- `GET /owners/{canary}/members` → 200, one member (the QA account). Synthetic owner → 403 `FORBIDDEN`.
- `GET /owners/{canary}` → 200 `{name:"bex-canary", type:"team"}`.
- `GET /owners/own-0000000000000000000000` (synthetic user-style id) → 200 with the **caller's** default workspace (`bex`), matching the pinned schema's documented contract: an `own-` id returns "the user's default workspace" — resolved from the authenticated caller, not from the supplied suffix. Recorded as spec-shaped behavior.

No bug filed.
