# CLI sweep 849 — env-group link/clone cycle (live, 2026-10-20)

Disposable env group, web service, and group clone created, linked, unlinked, all deleted (204s; all GETs 404, including the clone in its home workspace). No new bug filed.

- Link is `POST /env-groups/{id}/services/{serviceId}` with **no body**: sending `{}` → named 400 `request body is not allowed for this operation`. With an empty body → 200 and the group reads back `serviceLinks:[{id, name, type:"web"}]` (the w8/done-063 object-form contract); DELETE link → 204.
- Group clone (`POST /env-groups/{id}/clone {"name":…}`) → 201 with env-var keys carried over and empty `serviceLinks`. It landed in the caller's default workspace (documented owner resolution, sweep 743) — cleanup verified there explicitly, not assumed from the canary list.
- While linked, the service's own `GET …/env-vars` stayed `[]`: group values materialize to the workload, they do not merge into the service's direct env-var list (Render's same direct-vs-linked distinction, cf. the env-var endpoint's own doc note).
