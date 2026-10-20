# CLI sweep 1146 — webhook secret is show-once; eventFilter is a string list (live, 2026-10-20)

One disposable webhook created and deleted (204, verified 404). No new bug filed. The signing secret value is not recorded here.

- An object-shaped `eventFilter` → 400 `invalid request body at /eventFilter`; the declared shape is a flat list of event-type strings.
- Create → 201 including the signing `secret` exactly once. The subsequent GET returns only `enabled`, `eventFilter`, `id`, `name`, `url` — no secret key on any later read.
- PATCH `enabled: false` → 200 applied; PATCH with a bogus event type → 400.
- DELETE → 204; GET after → 404; second DELETE → 404.
