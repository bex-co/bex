# 048 hosted acceptance, free side — 2026-10-18

[Results](results.json) · [the free cron Blueprint used](free-cron-render.yaml). Production `api.bex.co` (deploy `429d49ccf`), QA workspace bex-canary, both native clients (bex CLI v0.3.3 and the pinned Render CLI), authenticated with a temporary QA API key that was revoked after the run.

- **Free cron create:** `services create --type cron_job --plan free` is refused by both clients with `400 bad request: a cron_job requires a paid plan; plan must be one of starter|standard|pro|pro_plus|pro_max|pro_ultra`. Nothing was created.
- **Blueprint free cron:** `blueprints validate` on a `type: cron` service with `plan: free` returns `valid: false` from both clients. The error names `services[0].plan` at line 9, column 11, with the same message.
- **Not run:**
  - The plan-less create resolving to Starter, and the `--plan free` update refusal. Both need a paid cron fixture.
  - The production free-cron inventory. Deciding what to do with existing free crons is the user's call.
