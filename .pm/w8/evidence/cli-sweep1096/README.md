# CLI sweep 1096 — GraphQL dryRun create also persists nothing (live, 2026-10-20)

No fixture persisted (name list empty). No new bug filed.

- GraphQL `createService(..., dryRun:true)` → a preview object with the would-be name as id, and the REST name list confirms nothing was created. The dry-run contract is consistent across REST (sweep 1094) and GraphQL.
