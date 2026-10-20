# CLI sweep 1193 — project environments are create-inline and cascade-delete (live, 2026-10-20)

Two disposable projects (one with an inline environment) and one service created and deleted (204, verified 404). No new bug filed.

- Environments are declared inline at project create (`environments: [{name}]` → 201 with a minted `evm-` id in `environmentIds`) — the counterpart of the nested-CRUD 404 from sweep 1156: creation is declarative, not a subresource POST.
- Deleting the project cascades: the environment id afterwards reads 404 (`environment not found`) at its own route. No orphaned environment survives its project.
