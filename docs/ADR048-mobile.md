# ADR048 — Mobile: the dashboard on the phone

**Status:** Proposed · market research + dashboard-surface inventory 2026-08-01. Not yet scheduled. Depends on ADR047 (agent sessions) for its differentiating tier; the supervision tier stands alone.

---

## Context

### The question

Which of the dashboard's features — and which parts of ADR047's cloud coding-agent sessions — belong on a phone, for ease of use for bex customers? "Mobile" here means the experience a customer reaches from their phone, whether as a responsive PWA or a native app; the delivery vehicle is itself a decision below (D5).

### What the market verifiably does (research pass 2026-08-01)

The developer-cloud market has converged on one answer: **the phone is a supervision and approval surface, not a configuration surface.** The load-bearing findings:

- **GitHub Mobile** — the archetype. Core loop: notifications inbox → triage → PR review/merge; explicitly _not_ an editor ("a way to do high-impact work on GitHub quickly and from anywhere", [docs](https://docs.github.com/en/get-started/using-github/github-mobile)). Push notifications are "one of the core value adds of a mobile app" (launch [blog](https://github.blog/news-insights/product-news/github-for-mobile-is-now-available/)); notification hygiene (granular types, "Working Hours" scheduling) shipped as a first-class feature because users were "blasted" ([2021 update](https://github.blog/news-insights/product-news/new-push-notifications-scheduling-releases-github-mobile/)). In 2025 the app became an **agent mission control**: assign Copilot coding-agent tasks from the phone ([May 2025](https://github.blog/changelog/2025-05-19-github-copilot-coding-agent-in-public-preview/)), start tasks from Home with push on draft-PR-ready ([Sept 2025](https://github.blog/changelog/2025-09-24-start-and-track-copilot-coding-agent-tasks-in-github-mobile/)), steer/track from an agents page ([Oct 2025](https://github.blog/changelog/2025-10-28-a-mission-control-to-assign-steer-and-track-copilot-coding-agent-tasks/)).
- **Railway** — the only direct PaaS competitor with an official app ([iOS, June 2026](https://railway.com/changelog/2026-06-19-railway-mobile-app-for-ios)). Shape: push for deploy events/crashes → service state, metrics, logs → and the strategic twist: the **Railway Agent covers every workflow "that doesn't have a dedicated mobile screen yet"** — it stages changes; the user reviews the patch and approves from the phone. The agent is the escape hatch for the mobile long tail.
- **Cursor / OpenAI Codex / Claude Code** — all converged 2025–26 on phone-as-remote-control for background agents. Cursor: agents on web+mobile as a **PWA first** ([June 2025](https://cursor.com/blog/agent-web)), native iOS later as a "remote control surface". Codex (in the ChatGPT app): the phone handles "the short decision points that usually block agentic coding work: clarifying instructions, choosing between implementation paths, approving commands, or reviewing a diff" ([OpenAI](https://openai.com/index/work-with-codex-from-anywhere/)). Claude Code (Claude app Code tab): "a client for Claude Code sessions rather than a place where code runs"; push "when a long-running task finishes or when it needs a decision"; **Bypass-permissions mode is disallowed from mobile** ([docs](https://code.claude.com/docs/en/mobile)).
- **Vercel** — no native app at all; mobile strategy is the responsive dashboard + **web push** for all notification types ([Dec 2025 changelog](https://vercel.com/changelog/push-notifications-support-on-desktop-and-mobile)). Validates PWA-first as a credible v0. The paid third-party client [Vercelios](https://www.vercelios.com/) (Live Activities build progress, widgets, one-tap redeploy/rollback, live build logs) is a proxy for native-only demand.
- **Render has no mobile app** — and a _paid_ third-party client exists ([RenDeploy](https://apps.apple.com/us/app/rendeploy-deployment-center/id6747311096): status, one-tap restart, live logs, manual deploys, env viewing, deploy history). Same pattern at Fly.io ([FlyScoop](https://community.fly.io/t/flyscoop-mobile-app-for-monitoring-managing-fly-io-resources/4071) — "a companion to the on-call engineer's desktop environment, not a replacement") and historically Heroku (Nezumi). Proof of unmet demand among exactly bex's target customers.
- **Ops archetypes** (PagerDuty, Datadog): the most-used loop is alert → evidence (graphs/logs on the phone) → ack/act; urgency tiers with DND override for critical pages are alert-fatigue design, not polish ([Datadog](https://www.datadoghq.com/blog/mobile-incident-management-datadog/)).
- **The one outlier thesis:** Replit treats the phone as a full **creation** device — chat with the Agent, watch it build and deploy, "no laptop required" ([blog](https://replit.com/blog/try-agent)) — because the AI agent removed the need for an IDE. Everyone else runs the supervision thesis.

Synthesis: universally-on-mobile = push notifications (the anchor), status at a glance, read-only live logs + basic metrics, one-tap safe verbs (restart/redeploy/rollback/cancel), review-and-approve, and delegate-to-agent. Universally-on-desktop = code authoring, complex initial setup (repo wiring, IaC/blueprints, billing config), deep debugging, bulk/admin operations, dangerous permission modes.

### What the dashboard has today (surface inventory)

The dashboard (`dashboard/src/routes/`, feature modules under `dashboard/src/features/`) already exposes, with mobile-relevant transport in place:

- **Real-time primitives that survive mobile networks:** SSE log tails (`GET /v1/logs/subscribe`; `features/logs/hooks/use-live-logs.ts`, deploy logs at 5s-poll fallback), Apollo polling (resource lists 30s, deploy timeline 3s, metrics 30s), and the web shell's WebSocket + ticket pattern.
- **Supervision surface:** home resource list grouped by project with lifecycle actions; deploy history + trigger/cancel/rollback + live deploy logs; service events; logs with filters; Render-style metrics; datastore insights (top queries/table scans/processes).
- **Ops verbs:** restart/suspend/resume across services, Postgres, and key-value; cron run-now/cancel-run/history; env-var and secret-file CRUD; scaling + autoscaling; plan changes.
- **Heavy configuration surface** (the majority of screens): service settings (build/deploy config, health checks, custom domains + DNS verification, IP allow-lists, deploy hooks, notifications), blueprints, env groups, environments/projects topology, webhook endpoints, registry credentials, workspace/team/billing administration, API/SSH keys, Postgres parameter overrides / PITR / failover.
- **Notification plumbing:** per-service notification settings and per-caller email toggles exist; outbound event webhooks exist; there is **no push-notification channel** today.

### Why ADR047 changes the answer

ADR047's agent sessions are accidentally a mobile-first product: phase 1 is fire-and-forget (prompt in → draft PR + evidence out — GitHub Mobile's exact agent flow); phase 2 streams the AI SDK UI-message protocol consumed by `useChat` — a chat UI, the best-understood mobile interaction pattern; steering is new prompt turns (texting-shaped); delivery is a draft PR — the canonical 2025–26 mobile verb (review/approve). And per the Railway lesson, once sessions exist the agent becomes the escape hatch for every dashboard feature mobile doesn't port.

---

## Decision

### D1 — Thesis: supervision-first, agent-differentiated

bex mobile runs the market-validated supervision loop (push → status → logs/evidence → one-tap safe verb) as its foundation, and differentiates with the delegation loop (assign an agent task → get pinged on needs-decision/PR-ready → review and approve) the moment ADR047 phase 1 exists. Mobile is explicitly **not** a port of the dashboard: the configuration surface stays on desktop (D4).

### D2 — Tier 1: the mobile MVP

| Feature | Existing primitive | Mobile shape |
| --- | --- | --- |
| Push notifications | per-service notification settings, outbound event webhooks | the anchor: deploy failed/succeeded, service crashed, cron run failed, usage threshold, agent needs-decision / PR-ready; urgency tiers + working hours from day 1 |
| Service status list | home page resource queries (30s poll) | glanceable cards: status, latest deploy, tap-through |
| Deploys: view / trigger / cancel / rollback | `TriggerDeploy`/`CancelDeploy`/`RollbackService`, SSE deploy-log tail | the "fix it from the taxi" verbs — short, reversible, pre-parameterized |
| Live logs (read-only tail) | SSE `/v1/logs/subscribe` + filters | every third-party PaaS client ships this; ours already rides SSE |
| Lifecycle verbs | restart/suspend/resume (service, Postgres, key-value) | one-tap with confirmation |
| Events timeline | `ServiceEvents` | the "what just happened" context behind any alert |
| Metrics snapshot | CPU/mem/network queries (30s poll) | sparklines + current values, not the full filterable charts |
| **Agent sessions (the differentiator, with ADR047 ph.1)** | `POST /v1/agent-sessions`, transcript store, draft-PR delivery | Sessions tab: task composer (repo+branch+prompt), session list, transcript + evidence view, PR link; push on needs-decision / PR-ready |

PR _review_ itself is delivered to GitHub and deep review stays in GitHub Mobile — bex links out rather than rebuilding it.

### D3 — Tier 2: fast follows, not launch-blocking

Single env-var quick view/edit (bulk import stays desktop); cron run-now/cancel/history; Postgres/key-value overview + connection status + read-only insights; usage/billing glance + month-to-date bandwidth (widget material); invite accept (invites are opened on phones); ADR047 phase-2 live chat attach (`useChat` session UI) once it ships. The web shell (WebSocket + xterm.js) technically works on mobile but is break-glass, not primary — keyboard ergonomics.

### D4 — Non-goals: what stays on desktop

Service creation wizard, the settings surface (build config, health checks, domains + DNS verification, IP allow-lists, deploy hooks), blueprint authoring/sync, environment/project topology, env-group management, webhook endpoint configuration, registry credentials, autoscaling configuration, database parameter overrides, workspace/billing administration, API/SSH key management. Low-frequency, high-parameter, consequence-heavy — the phone adds risk, not ease; every exemplar draws the same line. **Destructive verbs stay off mobile entirely** (delete service/database/workspace, PITR, failover) — the Claude Code no-bypass-permissions-on-mobile posture. The urgent slice of the settings long tail is eventually covered by the agent (Railway pattern), not by porting screens.

### D5 — Delivery vehicle: PWA first, native on triggers

**v0 is the existing dashboard, responsive + installable + web push.** Vercel validates the strategy outright; Cursor launched mobile agents as a PWA. The work is a responsive pass over the Tier-1 routes (TanStack Start/shadcn are amenable), a mobile navigation shell, a web-push service worker, and a bex-api push channel driven by the same event plumbing behind outbound webhooks. No app-store cycle gates ADR047 phase 1.

**Native (wrapper) when three triggers fire:** (a) agent sessions are live and notification engagement proves out; (b) iOS Live Activities for build/deploy/agent progress + home-screen widgets are wanted — the genuinely native-only Vercelios feature set; (c) paid-plan volume justifies app-store presence and review overhead.

### D6 — Phasing

1. **M1 — Supervise (PWA):** responsive Tier-1 routes, web push for deploy + crash events, one-tap restart/rollback/cancel.
2. **M2 — Delegate:** ADR047 phase-1 Sessions tab (create task → push → PR link + evidence). Thin surface; can land with ADR047 phase 1 itself.
3. **M3 — Steer:** ADR047 phase-2 live chat attach, needs-decision push, env-var quick edit, cron/datastore Tier-2 verbs.
4. **M4 — Go native:** wrapper app, Live Activities, widgets, urgency-tiered push with DND override for critical.

---

## Consequences and gaps to close

1. **No push channel exists** — new bex-api web-push service (VAPID keys, subscription store in the control-plane DB, emitter off the existing event pipeline) + a service worker in the dashboard. The single biggest net-new backend piece; everything else in M1 is frontend work.
2. **Responsive debt** — the dashboard is desktop-first today; the Tier-1 routes need a deliberate mobile pass (navigation shell, touch targets, log viewer virtualization on small viewports).
3. **Notification hygiene is scope, not polish** — urgency tiers, per-service granularity (exists), working-hours scheduling; GitHub's lesson is that skipping this burns users immediately.
4. **The differentiator is hostage to ADR047's schedule** — M2/M3 track it; M1 stands alone and is independently valuable (RenDeploy proves people pay for just that).
5. **Auth on mobile browsers** — Kratos session cookies + the PWA install flow need verification on iOS Safari (ITP) before committing to web push there; iOS web push requires the installed-PWA context.
6. **Destructive-verb policy** (D4) should be enforced server-side eventually (e.g. an OpenFGA condition or client-attestation), not just by UI omission — carried as an open question, acceptable as UI-only in v0.

## Alternatives considered

- **Native app first** — rejected for v0: app-store cycles gate iteration, Vercel/Cursor prove PWA credibility, and the native-only wins (Live Activities, widgets) are M4 triggers, not launch requirements.
- **Port the full dashboard responsively** — rejected: the settings surface is where mobile adds risk over ease; every exemplar (GitHub, Railway, Cursor, Codex, Claude Code) explicitly scopes mobile to supervision + approval + delegation.
- **Replit-style creation-first mobile** (build new services from a phone) — deferred, not rejected: the service-creation wizard stays desktop for now; the ADR047 agent path is bex's creation-shaped mobile story and subsumes it (prompt → running code → PR) without porting the wizard.
- **Third-party-client strategy** (publish API, let a RenDeploy-alike emerge) — rejected as a strategy (fine as a side effect): the agent-session differentiator and the push channel are first-party assets, and the paid third-party clients existing at all is evidence the first-party gap is a churn risk.
