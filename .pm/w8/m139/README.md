# w8 · m139 — Free web services share 750 instance hours per workspace per month, as on Render

**Worker:** worker8 **Goal:** Each workspace gets Render's 750 Free instance hours per calendar month, counted only while a Free web service is running (spun-down time is free). When they run out, the workspace's Free web services are suspended until the start of the next month, with warning and exhaustion emails and a "Monthly included usage" readout, as on Render. **Status:** todo

## Tasks (in order)

| id   | title                                                                         | est | depends_on                 |
| ---- | ----------------------------------------------------------------------------- | --- | -------------------------- |
| t001 | Month-to-date Free instance hours per workspace from the existing usage meter  | 40m | —                          |
| t002 | Exhaustion: suspend Free web services until the month starts; resume on reset  | 60m | w8/m139/t001               |
| t003 | Notifications: approaching (e.g. 80%) and exhausted emails                     | 30m | w8/m139/t001               |
| t004 | Surfaces: monthly included usage on REST/GraphQL/MCP and the Usage page        | 40m | w8/m139/t001               |
| t005 | Render parity: ADR030/ADR018, suspension reason and resume rules               | 20m | w8/m139/t002, w8/m139/t003, w8/m139/t004 |
| t006 | Simplify the changed code                                                     | 15m | w8/m139/t005               |
| t007 | Test coverage: accrual, sleep exclusion, exhaustion, month rollover, upgrade   | 40m | w8/m139/t005               |
| t008 | Close out the milestone                                                       | 30m | w8/m139/t006, w8/m139/t007 |

## Definition of done

On a dev environment with a shortened budget (configurable, e.g. 0.05 h), and live on production for the read side:

- Two Free web services running in one workspace accrue Free hours at 2× wall time; a slept service accrues none (the activator's hibernation is excluded).
- Crossing the budget suspends every Free web service in the workspace with a suspension reason naming the monthly limit (`suspended` + suspender `billing`-like, not `user`); paid services keep running. The owner gets the exhaustion email; at 80% the warning email.
- At the month boundary the services resume (or become resumable, matching Render: t005 decides from Render's docs/behavior) and the counter resets.
- Upgrading a suspended Free service to a paid plan lets it run immediately.
- Production: `GET` usage (REST/GraphQL `usage`/MCP `get_usage`) shows `freeInstanceHours {used, included: 750}` for `bex-canary`, matching the instance-seconds meter.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep p8q1, 2026-10-24 UTC, `bex-canary` (Hobby), platform pin `29d63e1f7bea`. Source-level finding (a 750-hour exhaustion cannot be produced live in a QA sweep).
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); pricing ([ADR030](../../../docs/ADR030-pricing.md)). [render.com/docs/free](https://render.com/docs/free): "Render grants 750 Free instance hours to each workspace per calendar month", consumed only while running; when exhausted, "Render suspends all of your Free web services until the start of the next month"; usage appears in the Billing page's "Monthly Included Usage", with emails when approaching and exceeding. ADR030 §6 caps a Free service at one instance but sets no monthly budget, so N Free services run without bound at $0.
- **Expected outcome:** the Free tier's cost ceiling matches Render's, closing an unbounded free-compute path, and tenants see the same monthly allowance.
- **Why now:** major (pricing/abuse parity). The meter already exists (below), so this is accounting plus enforcement. Render parity task included: usage surfaces, dashboard and emails.

## Evidence

- `rg -i "free.?instance.?hours|instance.hours|750"` over `lego/`, `dashboard/src`, `docs/`, `.pm/`: no implementation, ADR or board item. The only nearby rule is ADR030 §6 (one running instance per Free service).
- The meter exists: `lego/backend/internal/store/usage.go:33` (`UsageKindInstanceSeconds`, broken out by tier, `usage.go:102,153`), surfaced as month-to-date usage by `internal/usage/service.go:168,660` and MCP `get_usage` ("instance-seconds by tier", `usage/mcp.go:41`). GraphQL `usage(period, ownerId)` returns `UsageSummary {billing, coverage, estimatedCost, period, services, workspaceId}`, with no free-hours or included-usage field.

## Fix (target)

- t001: Free hours = month-to-date `instance_seconds` for tier `free` / 3600, per workspace (verify the meter excludes hibernated time; if it counts pods at zero replicas, fix that first).
- t002: a replica-safe sweeper (precedent: w8/m126's `FreeExpirySweeper`) suspends Free web services through the normal suspend path with a distinct suspender, and lifts that suspender at the month boundary. A user-suspended service stays user-suspended.
- t004: add `freeInstanceHours {used, included}` (or Render's "Monthly included usage" shape if the pinned OpenAPI has one) to usage on every surface, and a Usage-page meter.

## Blast radius

Every workspace's Free web services. Paid services, static sites (Render's Free static sites do not consume instance hours) and datastores are unaffected. Existing heavy Free users will be suspended in the first month after release, so the release note and the 80% email ship first.

## Unverified

- Whether Render resumes automatically at the month start or requires a manual resume (docs: "suspends … until the start of the next month").
- Whether the bex meter excludes hibernated (scaled-to-zero) time.

## Dedupe

Searched `.pm` open and done, `docs/` (ADR030, ADR040, ADR018) and DO_NOT_DO for free instance hours / 750 / included usage: none. w8/m126 (free Postgres 30-day expiry) is the closest precedent for a free-tier sweeper.
