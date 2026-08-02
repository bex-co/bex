# ADR048 — Mobile: the dashboard on the phone

**Status:** Accepted · market research + dashboard-surface inventory 2026-08-01; native delivery amendment accepted 2026-08-02 and scheduled as w11. Depends on ADR047 (agent sessions) for its differentiating tier; the supervision tier stands alone.

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

**Native-push implementation boundary (w11/m5, 2026-08-02).** bex-api now projects the existing composed deploy/service/cron event feed into one durable, caller-scoped logical inbox item and at most one leased delivery per active installation. The optional Expo transport is constructed only from validated server configuration, persists acceptance tickets, checks receipts, bounds retry and ambiguity, prunes only the exact invalid token generation, and sends a closed four-field data envelope containing an opaque notification id plus an allowlisted relative route. The existing email toggles remain independent; the shared push policy adds typed event, urgency, timezone, working/quiet-hours, deferral, and exact per-service override semantics through REST and GraphQL, with matching dashboard controls. The native client asks for permission only after an explicit gesture and server-availability check, keeps the installation id in SecureStore, validates every received or persisted route again, and clears protected local state before best-effort logout cleanup. MCP deliberately gains no device-token verb: device registration is an installation-bound client operation, not an agent/tool surface. Production release remains blocked until an EAS project, APNs/FCM credentials, and redacted physical iOS and Android evidence satisfy the m5 qualification matrix; simulator/export evidence is not promoted to that gate.

### D3 — Tier 2: fast follows, not launch-blocking

Single env-var quick view/edit (bulk import stays desktop); cron run-now/cancel/history; Postgres/key-value overview + connection status + read-only insights; usage/billing glance + month-to-date bandwidth (widget material); invite accept (invites are opened on phones); ADR047 phase-2 live chat attach (`useChat` session UI) once it ships. The web shell (WebSocket + xterm.js) technically works on mobile but is break-glass, not primary — keyboard ergonomics.

**One-variable environment boundary (w11/m8, 2026-08-02).** This is a deliberately narrow incident-follow-up, not a mobile configuration screen. The service card initially fetches only keys plus one opaque whole-environment revision; values remain masked and are fetched one key at a time only after an explicit reveal gesture. The reveal query is `no-cache`, and the native client holds at most one variable's value in process memory. Hiding it, changing service, backgrounding the app, crossing an identity/workspace boundary, or unmounting the screen clears that state; it is never written to Apollo cache, AsyncStorage, SecureStore, diagnostics, confirmation copy, or a persistence adapter.

Saving freezes the exact service, existing key, edited value, and observed revision behind the shared safe-action confirmation, but dismisses the keyboard, removes the plaintext editor, and displays only service + key in the dialog. The typed mutation sends one literal key/value patch, `saveMode: deploy`, and `expectedEnvRevision`; the backend still authorizes the reveal with `can_view_sensitive` and the write with `can_create`, and refuses to use this narrow path to create a missing key. OpenBao KV-v2 compare-and-set makes two edits from one observation resolve to one success and one honest conflict. Reveal, mutation, and masked-refresh requests are time-bounded and invalidated across service, identity, background, and unmount boundaries so a late response cannot repopulate secret state. On conflict or any ambiguous write outcome the client clears the value, blocks further reveal/edit, refreshes the values-free masked list, and requires a fresh reveal/confirmation; it never guesses that a failed request applied. A committed write whose follow-up refresh fails is reported separately from a failed write, and a no-op CAS is reported without claiming a rollout. Server-side compensation restores source and derived state only when doing so cannot overwrite a newer writer, with stable restored, conflict, and restoration-failed outcomes across GraphQL, REST, and MCP.

Mobile does **not** add, delete, rename, or generate variables; it has no replace-all/bulk import-export, env-group, secret-file, save-only draft, clipboard, or environment-workbench path. Those existing API/dashboard capabilities remain desktop/tool surfaces. The opaque `evr1_…` revision contains no tenant, service, key, or value and is an echo-only concurrency token, not a credential or authorization mechanism.

**Cron companion boundary (w11/m8, 2026-08-02).** A `cron_job` service gains a compact, cursor-paged run-history card containing only the opaque `crr-…` id, normalized status, and reported start/finish timing, plus a link to the existing general service logs. Mobile never requests the Kubernetes Job name, schedule, command, shell, or any cron configuration mutation. Confirmed run-now is still offered while a run is active because one backend call atomically cancels and replaces it; suspension removes run-now without removing cancellation of an already-active run. Terminal and unknown rows are never cancel targets.

**Datastore companion boundary (w11/m8, 2026-08-02).** Postgres and Key Value detail screens add compact, read-only incident evidence using opaque resource ids. Postgres shows disk use/capacity, active connections, logical size, process state/waits/duration, and table size/scan counters; Key Value shows lifecycle-derived availability (explicitly not a protocol ping), disk use/capacity, memory, and connected-client observations. Metric freshness comes from the newest real scrape timestamp, while live SQL insight freshness means the last successful network response. Empty, partial, stale, source-unavailable, and transport-error states remain distinct; the client never invents zeroes. Mobile does not request connection strings, passwords, hosts, users, raw SQL, parameters, PITR/recovery, failover/HA/replication lag, allowlists, persistence/eviction settings, or plan controls. Top-query rows are deliberately omitted: their raw SQL may contain literals, and the current backend contract cannot distinguish an unavailable `pg_stat_statements` source from a genuinely empty result.

Cron mutations reuse the safe-action confirmation inventory, freeze the exact service/run target, single-flight double taps, and carry a 15-second abort signal. The client polls authoritative history for the exact accepted run id or terminal cancellation instead of treating a mutation response as convergence. A timeout, missing id, failed refresh, or convergence timeout locks both cron actions until a successful values-free history refresh; a newly appearing scheduled run cannot be misattributed to an ambiguous manual trigger. Service changes, identity/workspace resets, and unmounts abort or invalidate in-flight work so a late response cannot publish into a new boundary. Backend run-now rejects a suspended cron before billing or any run/cancel intent write, while billing enforcement intentionally does not prevent canceling an existing run.

### D4 — Non-goals: what stays on desktop

Service creation wizard, the settings surface (build config, health checks, domains + DNS verification, IP allow-lists, deploy hooks), blueprint authoring/sync, environment/project topology, bulk environment import/export, env-group and secret-file management, webhook endpoint configuration, registry credentials, autoscaling configuration, database parameter overrides, workspace/billing administration, API/SSH key management. Low-frequency, high-parameter, consequence-heavy — the phone adds risk, not ease; every exemplar draws the same line. **Destructive verbs stay off mobile entirely** (delete service/database/workspace, PITR, failover) — the Claude Code no-bypass-permissions-on-mobile posture. The urgent slice of the settings long tail is eventually covered by the agent (Railway pattern), not by porting screens.

### D5 — Delivery vehicle: first-party Expo native client; responsive web remains complementary

The 2026-08-02 implementation directive deliberately fires the native trigger early: bex ships a first-party Expo Router client under `mobile/`, seeded from a mature native codebase but stripped to bex-owned configuration and supervision primitives. Native is the primary mobile product vehicle for w11 because push reliability, OS-protected credential storage, background agent/deploy notifications, and the eventual Live Activities/widgets surface are central rather than speculative. The market evidence above still constrains the _product shape_: native does not turn the phone into a configuration dashboard.

This is a real native API client, not a WebView wrapper. It uses the system browser for authentication and typed bex-api calls for product data. The existing dashboard keeps its Kratos HttpOnly-cookie architecture and remains the desktop/configuration surface. A responsive/installable PWA and web push remain valid complementary work, especially for self-hosters who do not distribute a custom binary, but they no longer gate the native supervision milestones. App-store release automation, Live Activities, and widgets stay trigger-gated; building the client does not assert that those distribution investments are already justified.

### D6 — Phasing

1. **Foundation (w11/m1–m2):** sanitized Expo shell, system-browser OAuth Authorization Code + PKCE, OS-secure session storage, typed bex-api access, workspace isolation, and accessible navigation.
2. **Supervise (w11/m3–m5):** status/deploys/events/logs/metrics, safe one-tap operations, and urgency/working-hours-aware native push.
3. **Delegate (w11/m6):** ADR047 phase-1 Sessions tab (create task → push → PR link + evidence); it remains gated on the backend phase.
4. **Steer + Tier 2 (w11/m7–m8):** ADR047 phase-2 live attach and needs-decision steering, then the explicitly bounded env-var/cron/datastore/usage/invite fast follows.
5. **Native-only distribution triggers:** Live Activities, widgets, and store/release automation are held until engagement and paid-plan volume justify them.

---

## Consequences and gaps to close

1. **Native push is implemented but not release-qualified** — w11/m5 adds the durable device-subscription, logical inbox, delivery/receipt worker, policy, dashboard, and Expo client path. The remaining launch gate is operational rather than architectural: bind the first-party EAS project, provision APNs/FCM and enhanced-push credentials, enable the optional production transport, and capture redacted physical iOS and Android evidence. PWA/web-push remains a complementary self-hoster path, not the native launch dependency.
2. **Responsive debt** — the dashboard is desktop-first today; the Tier-1 routes need a deliberate mobile pass (navigation shell, touch targets, log viewer virtualization on small viewports).
3. **Notification hygiene is scope, not polish** — urgency tiers, per-service granularity (exists), working-hours scheduling; GitHub's lesson is that skipping this burns users immediately.
4. **The differentiator is hostage to ADR047's schedule** — M2/M3 track it; M1 stands alone and is independently valuable (RenDeploy proves people pay for just that).
5. **Native auth is a public-client boundary** — w11/m2 registers one secretless Hydra client and follows [RFC 8252](https://www.rfc-editor.org/rfc/rfc8252.html): system browser, exact reverse-domain redirect, Authorization Code + PKCE S256, short-lived audience-bound access tokens, rotating refresh tokens, and OS-protected storage. WebView login, embedded client secrets, AsyncStorage tokens, and dashboard-auth changes are forbidden. PWA cookie/ITP verification remains separate work if web push is later pursued.
6. **Destructive-verb policy** (D4) should be enforced server-side eventually (e.g. an OpenFGA condition or client-attestation), not just by UI omission — carried as an open question, acceptable as UI-only in v0.
7. **One-key env editing is optimistic, not casual reveal** — OpenBao's version is exposed only as an opaque whole-map revision, successful writes compare-and-set it, and every conflict forces a masked refresh. This closes lost updates for the mobile path without enabling mount-wide mandatory CAS or changing legacy REST/MCP/dashboard write semantics.

## Alternatives considered

- **PWA-only v0** — was the original decision and remains architecturally credible, but was superseded by the explicit 2026-08-02 Expo implementation directive. The useful constraint survives: app-store release ceremony and native-only embellishments do not gate the API/client milestones.
- **Port the full dashboard responsively** — rejected: the settings surface is where mobile adds risk over ease; every exemplar (GitHub, Railway, Cursor, Codex, Claude Code) explicitly scopes mobile to supervision + approval + delegation.
- **Replit-style creation-first mobile** (build new services from a phone) — deferred, not rejected: the service-creation wizard stays desktop for now; the ADR047 agent path is bex's creation-shaped mobile story and subsumes it (prompt → running code → PR) without porting the wizard.
- **Third-party-client strategy** (publish API, let a RenDeploy-alike emerge) — rejected as a strategy (fine as a side effect): the agent-session differentiator and the push channel are first-party assets, and the paid third-party clients existing at all is evidence the first-party gap is a churn risk.
