# CLI sweep 1084 — GraphQL has no updateService verb (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `updateService` is not a GraphQL mutation (the error suggests the real neighbors: `scaleService`, `deleteService`, `createService`, …). GraphQL service updates ride the create-or-update `createService` half, per the schema's own design; REST PATCH remains the update surface. The service name was unchanged after the rejected guess.
