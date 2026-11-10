# w21 — project workstream (worker21)

**Worker:** worker21. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

## Milestones

- [x] **m257** — Cloud Agents MCP phase 1: contract hygiene — honest descriptions, `state`, `outcome` (8 tasks) → [`done/m257/`](done/m257/README.md) ← from [ADR094](../../docs/ADR094-coding-agent-mcp.md) D10 phase 1
- [x] **m258** — Cloud Agents MCP phase 2: the `/mcp/cloud-agents` toolset, `wait_agent_sessions`, idempotent create, `bex.cloud_agents` scope (11 tasks) → [`done/m258/`](done/m258/README.md) ← from ADR094 D10 phase 2; after m257
- [x] **m259** — Cloud Agents MCP phase 3: schema-validated structured output (`submit_result`) (9 tasks) → [`done/m259/`](done/m259/README.md) ← from ADR094 D10 phase 3; after m258 — DONE 2026-11-05
- [ ] **m260** — moved to **w9/m172** (2026-11-05, user decision: w9 follows up all w21 work) → [`../w9/m172/`](../w9/m172/README.md)
- [x] **m278** — [Agent Automations: scheduled cloud-agent sessions (Devin parity)](done/m278/README.md) (12 tasks) ← from [ADR101](../../docs/ADR101-agent-automations.md)
- [x] **m279** — [Agent child sessions: an agent can spawn and track child sessions (Devin parity)](done/m279/README.md) (11 tasks) ← from [ADR094](../../docs/ADR094-coding-agent-mcp.md) deferral; t002–t003 after w9/m179
- [x] **m280** — [Skill version history: every save keeps a version you can read, diff and restore (Devin parity)](done/m280/README.md) (8 tasks) ← from [ADR099](../../docs/ADR099-agent-skills.md) follow-up

## Inbox

- [x] **001** — Dashboard `/agents` and mobile read `state`/`outcome` instead of decoding `phase` → [`001.md`](done/001.md)
- [x] **002** — Dashboard `/agents` create sends an `idempotencyKey` and shows `title`/`tags` → [`002.md`](done/002.md)
- [x] **003** — Dashboard `/agents` renders `outcome.result` and the in-flight partial result → [`003.md`](done/003.md)
- [ ] **004** — moved to **w9/076** (2026-11-05) → [`../w9/076.md`](../w9/076.md)
- [x] **005** — Dashboard `/agents` creates with a `maxSpendUSD` ceiling and shows `outcome.cost` → [`005.md`](done/005.md)
- [ ] **006** — moved to **w9/078** (2026-11-05, user decision pending) → [`../w9/blocked/078.md`](../w9/blocked/078.md)
- [ ] **007** — moved to **w9/077** (2026-11-05) → [`../w9/077.md`](../w9/077.md)
- [ ] **008** — **BLOCKED (needs the repository that builds `bex.co/docs` and permission to push there, from the user)** — Document the "Deploy to bex" button in the public docs → [`blocked/008.md`](blocked/008.md)
- [ ] **009** — **BLOCKED (needs a disposable mailbox, a GitHub account and a test card from the user; the logged-out half is verified)** — Check the Deploy-to-bex link survives sign-up and the payment gate for a new account → [`blocked/009.md`](blocked/009.md)
- [x] **010** — Automations: count a workspace's automations in the database, and read the previous run's phase with one query → [`done/010.md`](done/010.md)
- [x] **011** — Skill versions saved before migration 0210 read with SQL-style YAML quoting, so their next diff shows one spurious description line → [`done/011.md`](done/011.md)
- [ ] **012** — **BLOCKED (needs read access to the production cluster: `~/.ssh/id_bex` for `scripts/fetch-app-kubeconfig.sh`; code-side findings recorded in the note)** — Blueprint deploys sit `queued` for 20-35 minutes, sync/create answer 500/502, and a missed rollout deadline is never retried → [`blocked/012.md`](blocked/012.md)
- [x] **013** — Automation create form logs React hydration error #418 (time zone default differs between server and browser) → [`013.md`](done/013.md)
- [x] **014** — Skill version history shows the author as a raw user id → [`014.md`](done/014.md)
- [x] **015** — The create forms (New Environment Group, New Service) accept duplicate environment keys and secret-file names and silently drop the earlier value, while the Edit editors refuse them → [`015.md`](done/015.md)
- [x] **016** — A custom Logs range that ended in the past still shows today's application lines when Live is on; the live tail has a lower bound but no upper bound (regression of w6/m111 target A) → [`016.md`](done/016.md)
- [x] **017** — The Chinese UI keeps two English accessible names: the toast landmark `Notifications alt+T` on every page, and the Metrics chart markers that show the raw event type (`commit ignored`) → [`017.md`](done/017.md)
- [ ] **018** — In the ⌘K search, a renamed service's accessible name glues its current name to its original name (`eden-dash-v3block-eden-mono`) → [`018.md`](018.md)
- [ ] **019** — Several UI texts fall below WCAG AA contrast (4.5:1): log instance ids in both themes, and the 404 chip, project health line, Partial data label and success badge in the light theme → [`019.md`](019.md)
- [ ] **020** — Postgres Recovery: the restore dialog's "Point in time" is sent as UTC (`${targetTime}:00Z`) although the recovery window beside it is shown in the viewer's local time, and nothing bounds it to the window → [`020.md`](020.md)
- [ ] **021** — The webhook event picker shows the raw `disk_snapshot_restored` under "Other" (en and zh): w2/064 added the served type without a picker label, the same leak w4/039 fixed for `service_moved`, and no guard test exists → [`021.md`](021.md)
- [ ] **022** — Webhook enabled/disabled state: the detail header's status link is named "Status" whatever it shows, and the list row, settings card and header tooltip print the backend's raw English reason ("disabled manually") in both languages → [`022.md`](022.md)
- [ ] **023** — API key names take no label rule: a 5000-character name and a name with a newline and BEL are accepted, and the revoke confirmation for a long name is a 3230px dialog whose Cancel and Revoke sit off-screen for a pointer → [`023.md`](023.md)
- [ ] **024** — Billing shows the credit balance as `$1000551.64` (no thousands separators, in en and zh) because four billing-summary amounts bypass the locale-aware `money()` helper the rest of the page uses → [`024.md`](024.md)
- [ ] **025** — A Logs (and Deploys) load failure is not announced to assistive technology: the first-load error state is a plain `EmptyState` with no role, while the same viewer's later-error banner has `role="alert"` → [`025.md`](025.md)
- [ ] **026** — Postgres Insights shows the platform's own activity as the customer's: Active processes lists 9 blank background workers above the 3 real app sessions, and Top queries is led by monitoring queries (rank 1 averages 153 s), a side effect of w4/m115's `pg_monitor` grant → [`026.md`](026.md)
- [ ] **027** — The Postgres and Key Value detail pages' Overview / Metrics / Logs tabs are plain buttons that show the selected tab only by a CSS border class, so assistive technology cannot tell which view is active → [`027.md`](027.md)
- [ ] **028** — The global + New menu words its webhook entry "Create a new webhook" / "创建新 Webhook" while every other entry reads "New …" / "新建…", because it reuses the create page's heading key → [`028.md`](028.md)
