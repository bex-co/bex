# CLI sweep 576 — service events across the lifecycle (owned fixture, REST)

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T20:14–20:16Z. Owned fixture `qa576-events` (`srv-dbb7k47o17is73furtc0`, free hello-app) read via same-credential REST (`/tmp/qa501/events576.py`; no CLI events verb) while driven through create → restart (CLI) → suspend/resume (REST) → rename (PATCH) → delete. Fixture deleted; service, events, and URL all 404 after.

- **The timeline is complete, typed, and strictly descending** (wrapper `{event, cursor}`): create yields `deploy_started`/`deploy_ended`; restart adds the same pair with `deployStatus: succeeded` and the acting user id in details; suspend yields `suspender_added` then `service_suspended`; resume yields `suspender_removed` then `service_resumed` (the latter landed ~10 s after the resume call — visible eventual consistency, present on the next read); a rename PATCH records `display_name_changed`.
- **Paging is exact:** a `limit=3` cursor walk over the 8-event history (3 pages) reproduces the `limit=100` order byte-identically. Dialects: limit range/type named 400s; an unknown query param → named `unsupported query parameter "surprise" for list-events`; `type` is a real closed-set filter (`type=service_suspended` → exactly 1; `type=banana` → 400 enumerating the closed set).
- **One surface difference, recorded not filed:** a garbage events cursor → `400 malformed cursor`, where deploys/logs cursors answer an empty page (sweeps 562/568). The events cursor is an opaque encoded tuple, so refusing garbage is defensible; the two conventions now coexist — worth knowing, not a defect on this evidence.
- **Isolation:** synthetic and Postgres ids → uniform 404; the baseline cron serves its own 5 events; hello-go returns `[]` (its deploys predate the retained window — retention depth itself not measured); no fixture event leaks into another service's read. Events of the deleted fixture 404 with the service (consistent with sweep 544).

Method note: `services update --env-var` does not exist in the pinned CLI (create-only flag) — the env leg was replaced by the rename PATCH above; env-change events remain unrun.

## Unrun

- Env-var change and deploy-failure event payloads on an owned service (failure leg was covered incidentally by sweep 562's stall diagnosis, not as an events read).
- Event retention depth (hello-go's empty read bounds it only loosely).
