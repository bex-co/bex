# CLI sweep 993 — GraphQL suspend/resume share the REST store (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- GraphQL `suspendService(id:)` → `suspended: suspended`, and the REST GET immediately agrees; `resumeService` flips both back. One store across transports.
- Suspending an unknown id via GraphQL → `data: null` with a `NOT_FOUND`-coded error, matching the GraphQL query behavior in sweep 962.
