# w9 · m343 — Automation run notifications: email per run outcome, and the auto-pause alert ADR101 promised

**Worker:** worker9 **Goal:** a failing or paused agent automation reaches its creator. Each automation has an email setting (Always / On failure / On success / Never, default On failure) as in Devin, repeated failures back off on the 1st, 3rd and 9th, and a platform auto-pause always sends a push, an inbox entry and an email. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | An automation stores its email notification setting: `always`, `on_failure`, `on_success` or `never` | 60m | — |
| [t002](t002.md) | A run's outcome emails the creator per the setting, with failure backoff on the 1st, 3rd and 9th consecutive failure | 90m | t001 |
| [t003](t003.md) | Auto-pause tells the creator: push, inbox entry and email, as ADR101 D7 promises | 45m | t002 |
| [t004](t004.md) | Dashboard: the automation form and page show the email notification setting, en and zh | 60m | t001 |
| [t005](t005.md) | `bex agent automations` carries the notification setting, on flags and in files | 30m | t001 |
| [t006](t006.md) | Live check on production: a failing automation emails on the 1st and 3rd failure, and its auto-pause notifies | 45m | t003, t004, t005 |
| [t007](t007.md) | Render parity (Devin): automation notifications match Devin across REST, GraphQL, MCP, CLI and dashboard | 20m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

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
