# CLI sweep 1145 — one-off jobs refuse with a coded 410 (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `POST /services/{id}/jobs` → 410 with code `ONE_OFF_JOBS_UNSUPPORTED` and a message naming the supported alternatives (ssh into the service, pre-deploy command, cron job). This is a documented bex divergence from Render, expressed as a first-class coded refusal rather than a 404/501.
- `GET /services/{id}/jobs` → 200 `[]` (existing jobs remain listable, per the refusal message), and 404 after the service is deleted.
