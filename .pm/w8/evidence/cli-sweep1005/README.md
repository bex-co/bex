# CLI sweep 1005 — buildFilter and start-command placement on image services (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `buildFilter` on a prebuilt-image service → 400 (a repo-only concept, refused rather than stored inertly).
- `serviceDetails.startCommand` → 400 unknown field; the `envSpecificDetails` placement → 200, and on an image-runtime service it reads back as `dockerCommand` (the image's command is the start command — Render's mapping, consistent with sweep 985).
