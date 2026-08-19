# Dashboard parity walk

**Captured:** 2026-07-15

**Scope:** authenticated, page-by-page comparison of Render's live dashboard and the bex dashboard running against the local mock cluster

**Verdict:** the shipped page families are substantially aligned, with seven actionable gaps: six bounded dashboard-polish notes and one cross-layer Postgres-logs milestone.

This is a UI-depth audit, not a new product surface. It complements the capability-level matrix in [ADR018](../../ADR018-render-parity.md) by checking the controls, page structure, and empty/degraded states that a checked UI cell can otherwise hide.

## Evidence set

The walk used authenticated Playwright sessions against both products. Semantic captures recorded headings, tabs, links, buttons, labels, alerts, table headers, and the visible main-region text before each screenshot. Screenshots live in the gitignored `.playwright-mcp/` directory and use these bare-name prefixes:

- `render-walk-*.png` — 30 live Render routes.
- `bex-walk-*.png` — 23 equivalent bex routes.
- `bex-walk-git-cron-*.png` — the Git-backed cron comparator.

The reports are also retained locally as `.playwright-mcp/render-walk-report.json`, `.playwright-mcp/bex-walk-report.json`, and `.playwright-mcp/bex-cron-git-report.json`. Resource names, identities, connection strings, and command contents are intentionally omitted from this tracked artifact. One live Render command contained a credential-like value; it was treated as sensitive and is not reproduced here.

## Seeded bex fixtures

The isolated `w5` development stack contained:

- a Git-backed web service with a failed deploy and deploy-detail record;
- a Git-backed static site;
- both image-backed and Git-backed cron services (the Git-backed one is the like-for-like settings comparator);
- a managed Postgres and a managed Key Value;
- a project with a staging environment and assigned resources;
- a Pro workspace with two accepted members.

The local stack intentionally lacked OpenBao, Loki, and Prometheus. Pages that depend on those optional stores were still walked and their honest degraded states recorded. That configuration limitation is not classified as a product gap.

## Route inventory

| Family | Render routes walked | bex routes walked |
| --- | --- | --- |
| Service | root, Settings, Environment, Logs, Metrics, Events, Deploys, deploy detail, Scaling | root, Settings, Environment, Logs, Metrics, Events, Deploys, deploy detail, Scaling, Plan |
| Static site | root, Settings, Redirects/Rewrites, Headers | root, Settings (including route/header editors) |
| Cron | root, Settings | root, Settings; repeated with a Git-backed cron |
| Postgres | Info, Logs, Metrics, Recovery | consolidated detail page |
| Key Value | create page; detail unavailable in the live account | create/detail implementation represented by the seeded bex detail page |
| Projects and environments | Projects, environment detail, environment settings | project detail/settings plus environment cards and dialogs |
| Environment groups | list, create | list/degraded state |
| Workspace | overview, Team, Usage/Billing, Audit, Settings | overview, Team and Audit, Usage, Settings |

The family verdicts and dispositions are in:

- [Services](services.md)
- [Datastores](datastores.md)
- [Workspace, projects, and environments](workspace.md)

## Reproduce the walk

1. Start the isolated mock-cluster stack with `bash scripts/dev-env.sh 5 up`, then start the dashboard with the command printed by that script. Do not use another worker's namespace or ports.
2. Register a test identity and seed the fixture types listed above. Accept a second identity's workspace invitation so Team has a real second member.
3. In an authenticated live Render account, choose representative Git web, static-site, cron, and Postgres resources. It is acceptable to record a page as unreachable when the account has no safe fixture; do not create billable resources merely for the audit.
4. Walk the route inventory above in order. At each route, record semantic page structure and take a desktop screenshot using a **bare filename** so it lands in `.playwright-mcp/`.
5. Walk the equivalent bex route in the same order. For each divergence, inspect the screenshot pair and classify it as a real gap, an already-owned gap, a deliberate superset, an information-architecture-only difference, a configured degraded state, or an explicit non-goal.
6. File only unowned real gaps. Update ADR018 when the evidence changes a UI claim, then run `npx prettier@3.4.2 --write "**/*.md"`.

No product test coverage applies to this docs-only audit. Reproducibility is the test: the fixture list, route inventory, capture method, evidence names, and classification rules above make the comparison repeatable.
