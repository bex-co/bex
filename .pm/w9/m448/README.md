# w9 · m448 — Every cron job is charged at least $1 a month, as on Render

**Worker:** worker9 **Goal:** a cron job whose runs cost less than $1 in a billing month is charged up to $1 on that month's invoice, and one whose runs cost $1 or more is charged nothing extra, as Render's "minimum monthly charge of $1 per cron job service" **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Capture Render's $1 cron minimum: proration, suspended and deleted cron jobs, invoice line | 45m | — |
| [t002](t002.md) | Amend ADR030 and the ADR018 cron row: bex charges the $1 minimum | 20m | t001 |
| [t003](t003.md) | Top up each cron job's billing-period run charge to $1 with a deterministic micro-USD meter event | 1h 30m | t001 |
| [t004](t004.md) | Provision the cron-minimum Stripe meter and price and sync them onto every contract | 45m | t003 |
| [t005](t005.md) | Show the minimum: a "Cron job minimum" charges line and "$1/month minimum" on the cron create form and Plan page | 1h | t003 |
| [t006](t006.md) | Real-Postgres tests of the top-up: no runs, runs over $1, deleted mid-period, re-run idempotent | 45m | t003 |
| [t007](t007.md) | Render parity | 30m | t004, t005, t006 |
| [t008](t008.md) | Simplify | 20m | t007 |
| [t009](t009.md) | Test coverage | 30m | t007 |
| [t010](t010.md) | Closeout | 15m | t008, t009 |

## Definition of done

- On production, a cron job in a paid workspace that ran nothing (or ran for cents) during a billing period shows a "Cron job minimum" line on that period's Stripe invoice that brings its charge to exactly $1.00, and a cron job whose runs cost more than $1 in the same period has no such line.
- Settling the same period twice (a retried seal, a manual `SettleWorkspace`) does not charge the minimum twice.
- The New Cron Job form and a cron job's Plan page say a cron job is billed per second of run time with a $1/month minimum, in every dashboard language; the workspace charges card lists the minimum as its own row.
- `docs/ADR030-pricing.md` no longer says bex "adds no $1 monthly floor", and `docs/ADR018-render-parity.md`'s Cron job row records the minimum with evidence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for one aspect of render-parity` (2026-11-16), filed by `/pmx … for w9`. Evidence:
  - [render.com/docs/cronjobs](https://render.com/docs/cronjobs) (read 2026-11-16): "Billing is prorated by the second, based on active running time during a given month." and "There is a minimum monthly charge of $1 per cron job service."
  - `docs/ADR030-pricing.md` (w8/048 paragraph, "Extended to Cron Jobs 2026-10-15") cites that sentence and then says "bex's per-second compute billing applies as for any paid service; it adds no $1 monthly floor." `docs/ADR018-render-parity.md` records no such divergence and no user sign-off for it.
  - [w4/546](../../w4/done/546.md) shipped per-minute cron pricing and left the floor as "ADR030's separate decision".
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first): every customer-facing behavior matches render.com, and a divergence stands only when ADR018 records it with the user's sign-off. Closes the last cron-billing difference on ADR018's Cron job row.
- **Expected outcome:** cron jobs bill as on Render: per second of run time, never less than $1 per cron job per month. Price surfaces state the minimum before someone creates one.
- **Why now:** cron jobs are paid-only (w8/048) and priced per minute of run time (w4/546); the floor is the one remaining cron-billing difference, and ADR030 currently carries an unsigned divergence that the AGENTS.md parity rule requires amending. Each month it stays open is another invoice period billed differently from Render.
- **Decision state:** this milestone implements the `/pm-brainstorm` **recommendation**, matching Render, because the repository's parity rule makes that the default (ADR008; parity work is never parked on an optional decision). It is not a user decision. The alternative, keeping no floor, is a divergence that would need the user's sign-off recorded in ADR018; until then the tasks implement the floor.
- **Render parity task:** included. The milestone changes GraphQL (charges breakdown) and the dashboard (charges card, cron create form, Plan page). REST and MCP have no billing surface on Render, so they stay `—`.
