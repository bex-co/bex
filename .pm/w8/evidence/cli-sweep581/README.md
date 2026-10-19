# CLI sweep 581 — `services create` refusal dialects (client enums vs named server 400s)

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Probe `/tmp/qa501/sweep581.sh`; every name was `qa581-*` and a final list scan proved zero leaked fixtures (the single created service was deleted, REST 404 verified).

- **Client-side enum refusals (exit 1, cobra grammar, no request sent):** `--type banana`, `--runtime banana`, `--region moon`, `--previews banana` each fail with `must be one of …` listing the pinned values. `--region fsn1` also fails client-side — the platform's own region stamp is outside the closed Render-region enum (recorded contract; creates omit `--region` and let the server stamp it).
- **Client-side structural refusals:** no name → `name is required`; cron without schedule/command → `cron-command and cron-schedule are required for cron jobs`; web service with neither repo nor image → `either repo or image is required`; both → `cannot specify both --repo and --image`; put without `--file` class errors behave the same way (sweep 580).
- **Named server 400s (exit 1):** bad plan → `plan must be one of free|starter|standard|pro|pro_plus|pro_max|pro_ultra`; name dialects (uppercase, leading hyphen, underscore, 64 chars) → uniform `RESOURCE_NAME_INVALID` naming the lowercase/digit/hyphen, ≤30-char, no-edge-hyphen rule; negative instances → `replicas must be 0-100`; bad and 6-field cron schedules → `SCHEDULE_INVALID … valid 5-field cron expression`; static site with an image → `a static_site builds from a Git repo; a prebuilt image is not supported`; health path without slash → `health check path must start with /`; shutdown delay 301 → `maxShutdownDelaySeconds must be 1-300`.
- **The one creation:** `--num-instances 0` is the client's unset sentinel; the create succeeded and the server normalized to `numInstances: 1`. Fixture `qa581-inst0` (`srv-dbb8dqno17is73furue0`) was inspected, deleted within the minute, and verified 404 with zero `qa581` names remaining.

## Caution recorded

The `--num-instances 0` create omitted `--plan`, and the server defaulted the row to `starter`, not `free`. It was deleted immediately, but future sweeps must pass `--plan free` explicitly on every create rather than rely on the server default.

## Unrun

- `--build-filter-*`, `--env-var`, and `--secret-file` create-time dialects (covered in part by sweeps 555/578 at other boundaries).
- Name length exactly 30 vs 31 (the 30-char cap is named in the error; boundary pair not exercised).
- Duplicate-name create against an owned fixture's live name.
