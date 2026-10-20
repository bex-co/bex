# CLI sweep 701 — Key Value suspend/resume lifecycle (live, 2026-10-19)

Disposable free Key Value `qa701-kv` (`red-dbbbra2vfpec73dm0ktg`), deleted and verified absent (zero `qa701` in list).

- Create → `creating` → `available` in ~40s (as sweep 658).
- `POST /key-value/{id}/suspend` (no body) → 202, status `suspended`.
- `POST /key-value/{id}/resume` → 202, transitions through `config_restart` back to `available` in ~35s.

Full datastore lifecycle (create/suspend/resume/delete) behaves end to end. No bug filed.
