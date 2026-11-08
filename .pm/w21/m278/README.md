# w21 · m278 — Agent Automations: scheduled cloud-agent sessions (Devin parity)

**Worker:** worker21 **Goal:** a customer defines an automation (a schedule trigger plus a session template) and bex starts an agent session at each tick, safely across replicas, as the creator, with spend and failure controls. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Record the ADR101 decisions | 30m | — |
| t002 | Schema, store and id kind | 90m | t001 |
| t003 | Schedule model: cron, time zone, once, ends-at | 75m | t001 |
| t004 | Service: CRUD, template validation, authorization | 90m | t002, t003 |
| t005 | Scheduler loop: claim, record, fire, recover | 120m | t004 |
| t006 | REST, GraphQL and MCP adapters | 90m | t004 |
| t007 | Dashboard: Automations list, form and run history | 120m | t006 |
| t008 | Live verification on production as the QA user | 45m | t005, t007 |
| t009 | Render parity (Devin reference for agent surfaces) | 25m | t008 |
| t010 | Simplify | 15m | t009 |
| t011 | Test coverage | 45m | t009, t010 |
| t012 | Closeout | 10m | t011 |

## Definition of done

- On production, an every-5-minutes automation starts exactly one session per tick for three ticks; pausing stops it; an overlapping tick is recorded `skipped_overlap`; a creator who lost access causes `denied` and auto-pause.
- N concurrent scheduler loops against real Postgres never start a second session for one fire, including after a crash between record and create.
- The same operations exist with the same fields and error codes on REST, GraphQL, MCP and the dashboard `/automations`.
- ADR101 is accepted; the ADR018 row and `docs/AGENTS.md` entry exist.

## Source + Goal linkage

- **Source:** User request 2026-11-07 (`/pm-brainstorm w21`, then `/pm them all to w21`; this explicitly places the items in w21 despite the 2026-11-05 default of following w21 work in w9). Design: [ADR101](../../docs/ADR101-agent-automations.md). Reference: Devin, verified live in the Devin dashboard on 2026-11-06 (Automations, Schedule trigger, Mine/All tabs).
- **Goal linkage:** ADR008 pillar 5 (cloud coding agents); Devin parity. Customers can run recurring agent work (nightly checks, weekly dependency updates) without building their own scheduler and long-lived API key.
- **Expected outcome:** Recurring agent work runs from a first-class, auditable, spend-bounded definition instead of a customer-side cron calling `create_agent_session`.
- **Why now:** Secrets (w9/m174), skills (w9/m175) and approval mode (w9/m176) are making sessions safe to run unattended; scheduling is the missing trigger that turns them into a recurring workflow. The loop reuses `Service.Create` unchanged, so shipping it before those milestones land costs nothing: their fields join the template as they arrive.
- **Render parity:** included. This milestone changes REST, GraphQL, MCP and dashboard surfaces for cloud coding agents; the reference product is Devin.
- **Dependencies:** none block the first fire. `secretNames` (w9/m174), `skills` (w9/m175) and `approvalMode` (w9/m176) are added to the template as each lands.
- **Out of this milestone** (ADR101 follow-ups): Webhook/GitHub triggers, preflight checks, persistent memory, `queue`/`allow` overlap, "message an existing session", service-account runs, templates, run webhook events.
