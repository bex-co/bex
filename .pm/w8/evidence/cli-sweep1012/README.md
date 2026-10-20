# CLI sweep 1012 — periodic status: three open findings unchanged (live, 2026-10-20)

One disposable service created for the 138 probe and deleted (204). QA session re-verified (`whoami` → the QA user in `bex-canary`). No new bug filed.

- Pin unchanged: `414a5d4b9` → `3ede47d3c5a9`. Spot-checks: [w8/137](../../137.md) → 405, [w8/138](../../138.md) → override keys still lack `serviceId`, [w8/139](../../139.md) → 400. All three stay open pending a pin containing their fixes.
