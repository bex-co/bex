# CLI sweep 858 — periodic status: pin, filings shipped by the parallel worker (live, 2026-10-20)

Status sweep; nothing created.

- Production pin unchanged: `9d7ac8afa` → `ca30db3a3646`. w8/135 retest still → 400 `invalid request body at /registry`; w8/137's five operations and w8/138's `serviceId` are likewise not live yet. All three findings stay open until a pin passes their fixes.
- The parallel worker has since pushed `0392fe911` (the w8/137 + w8/138 filings and loop evidence through sweep 850, "committed on its behalf") and `e8d28e88e` (the w8/137 fix on `origin/main`). Only evidence 851–857 remains local/uncommitted now.
