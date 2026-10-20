# CLI sweep 1206 — GraphQL schema inventory (live, 2026-10-20)

Read-only introspection. No new bug filed.

- The Mutation type exposes 173 fields and Query 114. Service lifecycle is complete over GraphQL (`createService`, `deleteService`, `suspendService`, `resumeService`, `scaleService`, `rollbackService`, `updateServicePlan`, `patchServiceEnvironment`, link setters), with no `updateService` — updates flow through the create-or-update `createService` and the targeted mutations, consistent with earlier GraphQL sweeps.
- Full introspection (`__type`) is available to the authenticated owner.
