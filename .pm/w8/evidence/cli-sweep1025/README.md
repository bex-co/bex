# CLI sweep 1025 — workspace-scoped lists vs explicit owner create (live, 2026-10-20)

One disposable free web service created with an explicit `ownerId` of the user's other workspace, deleted in the same sweep (204; GET 404). No new bug filed. (This loop's fixtures otherwise stay in `bex-canary`; this single cross-workspace create was to prove list scoping and was fully cleaned up — not a pattern to repeat.)

- An explicit-owner create lands in that workspace: it appears in that workspace's service list and **not** in `bex-canary`'s list. List scoping follows the owner, direct access follows membership (sweep 1024).
