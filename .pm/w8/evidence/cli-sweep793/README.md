# CLI sweep 793 — project/environment service membership (live, 2026-10-20)

Disposable project, environment, and free web service created, linked, unlinked, deleted (204s; GETs 404 after). No new bug filed.

- Two distinct link surfaces, both verified: `PUT /projects/{id}/service-links {"serviceIds":[…]}` sets project membership (service GET gains `projectId`); `PUT /environments/{id}/service-links` sets environment membership (env GET lists the service; service GET gains both `environmentId` and `projectId`).
- An extra `environmentId` field on the project-level PUT was tolerated and did not create environment membership — env membership only appeared after the environment-level PUT, so the two layers are independent, not conflated.
- Unlink with an empty list → 200, env `serviceIds` empty again; full cleanup verified by 404s.
