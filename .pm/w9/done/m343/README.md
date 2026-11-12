# w9 · m343 — Automation run notifications: email per run outcome, and the auto-pause alert ADR101 promised

**Worker:** worker9 **Goal:** a failing or paused agent automation reaches its creator. Each automation has an email setting (Always / On failure / On success / Never, default On failure) as in Devin, repeated failures back off on the 1st, 3rd and 9th, and a platform auto-pause always sends a push, an inbox entry and an email. **Status:** done (2026-11-12)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | An automation stores its email notification setting: `always`, `on_failure`, `on_success` or `never` — **DONE** | 60m | — |
| [t002](done/t002.md) | A run's outcome emails the creator per the setting, with failure backoff on the 1st, 3rd and 9th consecutive failure — **DONE** | 90m | t001 |
| [t003](done/t003.md) | Auto-pause tells the creator: push, inbox entry and email, as ADR101 D7 promises — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Dashboard: the automation form and page show the email notification setting, en and zh — **DONE** | 60m | t001 |
| [t005](done/t005.md) | `bex agent automations` carries the notification setting, on flags and in files — **DONE** | 30m | t001 |
| [t006](done/t006.md) | Live check on production: a failing automation emails on the 1st and 3rd failure, and its auto-pause notifies — **DONE** | 45m | t003, t004, t005 |
| [t007](done/t007.md) | Render parity (Devin): automation notifications match Devin across REST, GraphQL, MCP, CLI and dashboard — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007, t008 |
| [t010](done/t010.md) | Closeout — **DONE** | 10m | t009 |

## Definition of done

On production:

- A new automation reads back `notifications.email: on_failure` on REST, GraphQL, MCP, the CLI and the dashboard, and each surface can change it.
- With `on_failure`, an automation whose runs keep failing emails its creator after the 1st and 3rd failures and not the 2nd; a success resets the count.
- With `on_success`, only succeeded runs email.
- A platform auto-pause (`repeated_failures`, `creator_lost_access`, `template_invalid`) sends exactly one push, one inbox entry and one email, whatever the email setting.
- No run or pause notifies twice across retries or replicas.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w9` (2026-11-11), item 1, filed by `/pmx` 2026-11-12.
  - ADR101 D7 (`docs/ADR101-agent-automations.md:135`) promises the creator a push on auto-pause; both pause sites (`lego/backend/internal/agentsessions/automation_scheduler.go:142`, `:276-283`) only `log.Printf`, and `lego/backend/internal/notifications` has no automation code (checked at `3dd698375`).
  - Devin: an Email notification action with Always / On failure / On success, failure backoff on the 1st, 3rd and 9th consecutive failure ([Automations](https://docs.devin.ai/product-guides/automations)); scheduled sessions default to On failure only ([Scheduled Sessions](https://docs.devin.ai/product-guides/scheduled-sessions)).
- **Goal linkage:** ADR008 AI-native pillar (pillar 5, cloud coding agents) and Devin parity for agent automations (ADR101).
- **Expected outcome:** a paused or repeatedly failing automation no longer goes unnoticed; today the only trace is a server log line.
- **Why now:** the user turned on eight daily `rt-*` automations on 2026-11-11; any of them can auto-pause tonight without anyone knowing.
- **Recommendations recorded (not user decisions):** the default `on_failure` follows Devin's scheduled-session default, and `never` follows Devin's scheduled-session option; existing automations take the default on migration. Under ADR008 parity-first these are implemented as the default; the user can change them.
- **Overlap:** ADR106 D7 `bex agent automations` (w1/m338, done) is why t005 exists; outbound webhook events for runs stay out (ADR101:234 needs a naming decision).
- **Parity:** included (t007), against Devin, because the change touches REST, GraphQL, MCP, the CLI and the dashboard.

## Live check (2026-11-12, production a5be11e4e)

- **Setup:** QA user (cookie session, workspace `tea-d98210cbbpdc73dcrkvg`), deploy run 43195174637 (dispatched). Fixture `qa-notify-check` (`aut-dbqp1dq8vjhs73do983g`): chat-only template, `maxSpendUSD` 0.1, a secret name no workspace secret has, so each fire refuses at session create without starting a session or spending anything. Deleted afterwards (reads `AGENT_AUTOMATION_NOT_FOUND`).
- **Backfill:** the eight existing `rt-*` automations read `notifications.email: on_failure` on GraphQL right after the rollout.
- **Create / read (PASS):** GraphQL create with `notifications: {email: on_failure}` read back `on_failure`; the dashboard detail row reads "Email notifications: On failure" and the edit select "On failure" at 1280 px and 390 px; the pending skeleton carries the same row.
- **Backoff and pause (PASS):** three Run-now fires at 10:11:10, :11 and :13 UTC each settled `failed_create` ("no workspace agent secret has this name"), and the third auto-paused the automation `repeated_failures`. bex-api logged exactly `notifications: automation aut-dbqp1dq8vjhs73do983g run failed notice emailed to its creator` twice (the 1st and 3rd failure; the 2nd backed off) and `… pause repeated_failures notice emailed to its creator` once, at 10:11:14–15.
- **Edit (PASS):** switching the dashboard select to Never and saving read back `never` on the detail page and on GraphQL.
- **Not observable live:** the pause push and its inbox row. The QA account has no push device and its saved push policy lists only deploy, server and cron failures, so no push is due; `TestAutomationPausePushReachesOnlyTheCreator` covers it. REST, MCP and the CLI were not exercised live: the same core is covered by `TestAutomationNotificationsAcrossSurfaces` and the CLI contract fixture.
