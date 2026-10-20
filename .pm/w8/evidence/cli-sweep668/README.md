# CLI sweep 668 — protected-environment member guard (live, 2026-10-19)

Fixtures: project `qa668-proj`, protected environment `qa668-env`, member service `qa668-web` created with `--environment-id` (auto-joined the project; `GET /environments/{id}` listed the service in `serviceIds`). All deleted and verified absent (REST 404 / project list `[]` / zero `qa668` services).

Guard behavior:

- `bex services delete` on the member → 400 `PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED: "qa668-web" is a member of a protected environment; retry with confirm="sudo delete service qa668-web" to delete it`.
- REST `POST /services/{id}/suspend` on the member → same coded refusal with the `sudo suspend service …` phrase. Matches ADR032: the guard arms Delete/Suspend (not Resume) on protected members.
- The exact-phrase retry was not completed over REST (a JSON body on DELETE is refused with `request body is not allowed for this operation`; the phrase rides request context per ADR032, and the CLI/dashboard path is the documented carrier).
- Deleting the protected environment and project with the member still inside succeeded and left the service running ungrouped; the then-unguarded service deleted normally. This preserve-and-detach teardown is intentional and documented (ADR032 lines 27/35: bex deliberately differs from Render's empty-environment 409).

No bug filed.
