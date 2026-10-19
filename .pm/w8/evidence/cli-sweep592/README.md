# CLI sweep 592 — plan gate by service type: only web and static are free-creatable

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Refusal-only sweep: a final list scan proves zero `qa592` fixtures exist. (The intended cron flight was abandoned at the gate — cron cannot be created on the free plan, and this loop never submits paid plans.)

- `cron_job`, `background_worker`, and `private_service` with `--plan free` each fail with the same named server 400: `a <type> requires a paid plan; plan must be one of starter|standard|pro|pro_plus|pro_max|pro_ultra`, exit 1, no row written.
- `web_service` and `static_site` remain free-creatable (proven across sweeps 579–591).
- A harness slip worth recording: names containing underscores (from the type name itself) hit `RESOURCE_NAME_INVALID` before the plan gate — validation order is name grammar first, then the plan gate.

## Unrun

- The cron run-trigger/history/cancel flight (`POST/GET/DELETE /cron-jobs/{id}/runs`) — requires a paid cron fixture this loop will not create; history dialects remain covered by sweeps 254/304.
- Plan-gate behavior on update (free web service PATCHed with a cron-only field, etc.).
