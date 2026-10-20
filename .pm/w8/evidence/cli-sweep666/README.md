# CLI sweep 666 — projects/environments CRUD dialects (live, 2026-10-19)

All fixtures deleted and verified absent (`GET /projects?ownerId=…` → `[]`).

- `POST /projects {"name","ownerId"}` → 201 with owner echo and empty `environmentIds`. Missing name → 400 `invalid request body at /name`. Synthetic owner → 403 `FORBIDDEN` (no cross-workspace creation).
- Nested `POST /projects/{id}/environments` → 404 (not a route). Correct dialect is `POST /environments {"name","projectId"}` → 201 with id `evm-…`, empty member lists, `protectedStatus: unprotected`; `GET /environments?projectId=…` lists it.
- `bex environments <project>` on an empty project printed `null` (same CLI empty-list wart as sweep 656).
- `DELETE /environments/{id}` → 204; `DELETE /projects/{id}` → 204.

Dialects match Render's flat `/environments` shape. No bug filed.
