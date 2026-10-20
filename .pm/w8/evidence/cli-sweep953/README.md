# CLI sweep 953 — GraphQL createService lifecycle (live, 2026-10-20)

One disposable free web service created, read, and deleted. No new bug filed.

- `createService(name:, type:, plan:, image:)` → 200 with the new service id (the arg is `image`, per the server's own suggestion after a wrong guess). REST read → 200, so GraphQL and REST share one store.
- Bogus plan `qa_bogus_plan` → `data.createService: null` with a coded `bad request: plan must be one of free|starter|standard|pro|pro_plus|pro_max|pro_ultra` error; the follow-up list by name is empty, proving no partial create.
- Cleanup: DELETE → 204, GET → 404, name-filtered list empty.
