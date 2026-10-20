# CLI sweep 1118 — query array forms are per-operation (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `ownerId[]=` on the services list → 400: the bracket-array spelling is declared only on operations that opt into it (the webhooks list reads `ownerId[]` explicitly in its handler). The services list takes the plain repeated-parameter form instead.
- Repeated `type` parameters (`type=web_service&type=cron_job`) → 200: multi-value filters work by repetition.
