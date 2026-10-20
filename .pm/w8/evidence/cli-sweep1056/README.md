# CLI sweep 1056 — GraphQL delete lands in the same store (live, 2026-10-20)

One disposable free web service created via REST and deleted via GraphQL. No new bug filed.

- `deleteService(id:)` → `true`; the REST GET immediately answers 404. Deletion is transport-agnostic — the service is gone for every surface, not just the one that deleted it (the write-side complement of sweeps 952/993/1015).
