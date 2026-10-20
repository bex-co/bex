# CLI sweep 1000 — periodic status at sweep 1000: three open findings unchanged (live, 2026-10-20)

One disposable service created for the 138 probe and deleted (204; name list empty). No new bug filed.

- Pin unchanged: `414a5d4b9` → `3ede47d3c5a9` (the last hundred sweeps ran against this one pin). Spot-checks: [w8/137](../../137.md) → 405, [w8/138](../../138.md) → override keys still lack `serviceId`, [w8/139](../../139.md) → 400. All three fixes (`e8d28e88e`, `d98da2870`, `9e9c28e82`) are on `origin/main` only; all three findings stay open until a pin contains them and the live acceptance probes pass.
- Sweeps 899–1000 evidence (102 local paths with the filings) remains uncommitted pending a ship instruction or the parallel worker's next recording commit.
