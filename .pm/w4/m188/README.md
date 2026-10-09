# w4 · m188 — Translate hosting times and cron previews

**Worker:** worker4 **Goal:** relative hosting ages/countdowns and cron schedule explanations follow the selected dashboard language while preserving the exact API instants and scheduling semantics. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Translate relative ages and countdowns with the active translator | 45m | — |
| t002 | Translate supported cron preview phrases and weekdays in both forms | 35m | — |
| t003 | Audit and verify the shared callers, direct session label and aliases | 25m | w4/m188/t001, w4/m188/t002 |
| t004 | Render parity: retain UTC schedule and API timestamp contracts | 15m | w4/m188/t003 |
| t005 | Simplify the changed localization paths | 15m | w4/m188/t004 |
| t006 | Test locale changes, current English controls and existing guards | 40m | w4/m188/t004, w4/m188/t005 |
| t007 | Closeout after live acceptance and cleanup | 10m | w4/m188/t006 |

Total: **7 tasks, about 3h 5m**. Two minor defects with distinct helpers and shared-consumer verification exceed the inbox sizing rule. This commit schedules the work; fixes are not implemented.

## Definition of done

Repeat the recorded owned Free cron journey and form probes in [finding.md](finding.md):

- Fresh Chinese Events at desktop and 390×844 renders localized relative-time values: a just-completed success reads 刚刚, a two-minute age reads 2分钟, and a next occurrence 83 full days away reads 83天后. Exact `datetime` values still match the API's created/last-success/next instants. English keeps now / 2m / in 83d for the same buckets. Existing last-success label wording is separately owned by w4/227.
- Fresh Chinese Create and Settings show 每 5 分钟 · 使用 UTC for `*/5 * * * *` and 每周一 00:00 · 使用 UTC for `0 0 * * MON`. The English control retains the correct English phrase. Typing previews and Cancel never changes the persisted annual schedule.
- Repeat the exercised adjacent cases: `*/40 * * * *` has no false uniform-interval preview (Create's translated generic hint; Settings no preview), while `99 99 * * *` shows a localized error and Settings Save remains disabled. The other required Create fields do not bypass validation.
- Trigger one owned run and wait for the same run to finish. UI and queried history say successful, both configured output lines arrive, and REST/GraphQL/MCP raw timestamps and schedules agree as in the recorded safe projections. Translation does not mutate the service, create an extra job or alter UTC scheduling.
- Delete the sole test cron; service and run-history GETs return 404, Overview removes its exact name, baseline resources stay present and the isolated session is revoked.

The remaining 27 shared JSX sites, direct session accessible label, all unexercised phrase/bucket branches, in-place language switching, SSR and clock-boundary behavior are explicitly assigned verification/test work in t003/t006; they were not all live-probed during discovery. Do not mark this milestone done until the recorded live outcomes and assigned checks pass.

## Source + Goal linkage

- **Source:** user-requested continuous `$qa-find-bugs`, 2026-10-09 cycle28, muse.env, user-directed w4. [Finding](finding.md) separates both causes, controls, current pinned libraries, caller inventory and exact durable requests/responses.
- **Goal linkage:** ADR008 usable hosting; [dashboard i18n rules](../../../dashboard/AGENTS.md#internationalization-i18n); [ADR038](../../../docs/ADR038-cron-jobs.md) truthful schedules and run information; [ADR006](../../../docs/ADR006-bex-api.md) / [ADR018](../../../docs/ADR018-render-parity.md) common hosting contracts.
- **Expected outcome:** Chinese users can read when a resource/run was created and when a cron will run, plus its schedule explanation, without English fragments or any change to execution.
- **Why now:** fresh Chinese screens and correct raw API times isolate two existing hardcoded-English helper gaps. Translating only the outer labels leaves both defects; the relative helper reaches 27 shared sites and a direct session name.
- **Render parity included:** tenant-facing presentation changes. Current Render docs establish UTC and last-success semantics; authenticated Chinese UI styling was not observed. Preserve the REST/GraphQL/MCP contract and document any discovered divergence.
- **Dedupe:** uncovered localization gaps beside w6/done/m102, w4/done/m152, w6/done/m107, w9/done/m91 and w9/done/061, not evidence those milestones regressed. No open owner, matching anti-goal or landed undeployed localization fix. The separate w4/227 label remains separate.
- **Cleanup:** the owned cron was deleted and its old-session whoami returned 401; all six resource-family ID sets match baseline. No raw Kubernetes garbage-collection proof or wider old-DoD closeout is claimed.
