# CLI sweep 774 — concurrent duplicate writes (live, 2026-10-19)

Disposable free web service `qa774-web` (`srv-dbbcpva720pc73c0kn2g`). Deleted and verified absent (REST 404).

- Two simultaneous `POST suspend` → both 202 (idempotent, sweep 648 semantics under race).
- Two simultaneous whole-set `PUT /env-vars` with disjoint keys → the final set was exactly one writer's (`B` only, revision `evr1_…M`): replace-all writes serialize last-writer-wins with no torn merge and no 500.

No bug filed.
