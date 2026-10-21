# CLI sweep 1296 — `services create --from` without `--region` (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `f64d4c85f2cd`, bex-canary, HEAD-built pinned CLI.

- Source: a free echo image web service, `srv-dbcfeqciimkc73f6l1eg`, `region: fsn1`.
- `bex services create --from <src> --name <n>-c` → exit 1, `Error: region must be one of: frankfurt, ohio, oregon, singapore, virginia` (local CLI validation of the source's `fsn1`). Nothing was created.
- `… --region frankfurt` → exit 0. The clone `srv-dbcfer4iimkc73f6l1gg` was placed in `fsn1`.
- Recorded as an addendum to w8/152 (region policy). `docs/cli-compatibility-checklist.md:102/143/187/209` still names the value `local-capd`, which is stale; production reports `fsn1`.
- Both services were deleted (204 → 404).
