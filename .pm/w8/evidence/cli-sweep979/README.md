# CLI sweep 979 — autoscaling fail-closed on free services (live, 2026-10-20)

Two disposable free web services created and deleted (204s). No new bug filed.

- `GET …/autoscaling` on a free service → 200 with the disabled zero config (`enabled:false, min:0, max:0`).
- A PUT missing `criteria` → schema 400 naming `/criteria`; a complete PUT (min 1, max 2, CPU criterion) on the free plan → coded 400 `autoscaling requires a paid instance type`, and the follow-up GET shows the config unchanged. The paid gate refuses on the free side and writes nothing.
