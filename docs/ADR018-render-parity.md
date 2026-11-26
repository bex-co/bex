# Render parity matrix — REST · GraphQL · MCP · UI

A single, evidence-based map of how far bex actually matches [render.com](https://render.com), one row per Render capability across the four surfaces bex mirrors. It replaces scattered "Render-compatible" assertions with a checkable ledger: what has parity, what diverges, what is missing, and what is a deliberate non-goal — each cell backed by a pointer to code (bex side) or Render's own spec/docs (Render side).

**Ledger layout (2026-11-23, w10/m607).** This file is the index of the parity ledger. Each section's rows live in their own file under [ADR018/](ADR018/), which the section's heading below points at: amend a row there, not here. The headings stay here so every `ADR018-render-parity.md#…` link still resolves; a new section gets a file of its own and a pointer heading here. The MCP pins, the upstream drift re-baseline rounds, the mobile scope notes and the dated cross-cutting corrections that used to open this file are in [ADR018/surface-pins-and-corrections.md](ADR018/surface-pins-and-corrections.md). The split keeps every file far below the 1 MiB blob limit `scripts/repo-artifact-guard.sh` enforces.

**Method.** Four API-focused passes from 2026-07-08, re-verified against Render's live API/docs through 2026-07-27, plus an authenticated page-by-page dashboard walk on 2026-07-15: bex's public REST vs Render's OpenAPI spec; bex's GraphQL vs Render's captured dashboard operations; bex's MCP tools vs the current `render-oss/render-mcp-server`; and bex's dashboard vs Render's live dashboard. The design record for bex's side is [ADR006-bex-api.md](ADR006-bex-api.md) ("one Core, three adapters"); this doc is the parity ledger over it. [Dashboard-walk evidence](render-artifacts/dashboard-walk/README.md). Render's changelog covers what no API pin sees, such as a feature Render ships only in its dashboard (w4/m553, 2026-11-22). Every entry since its first (2023-03-31) has one row in [render-changelog.md](render-artifacts/render-changelog.md): matched, partial, a gap with the board item that owns it, a non-goal, or n/a with the reason. The weekly `render-changelog` job in `.github/workflows/render-schema-drift.yml` runs `scripts/render-changelog-drift.sh`, which reads Render's Atom feed and changelog page and fails on an entry with no row or a row whose entry is gone. What Render ships in its dashboard without announcing it, and what it keeps behind a per-workspace experiment flag, is in [dashboard-operations.md](render-artifacts/dashboard-operations.md) (w1/m645, 2026-11-26): one row for every GraphQL operation and every feature-flag gate in Render's public dashboard bundle (703 operations and 8 flags on 2026-11-26), each matched, partial, a gap with its board item, a non-goal, n/a or flagged. The weekly `render-dashboard-ops` job in the same workflow runs `scripts/render-dashboard-ops.sh`, which crawls the bundle from `dashboard.render.com` and fails on an operation or flag with no row, or a row whose operation or flag is gone. A flag gate that leaves the bundle means Render made the feature GA or dropped it, and its row names the item to reopen, so a flagged experiment is never taken for a GA feature. Render also moves a runtime's default version, and back-dates rows, on its per-language version pages without a changelog entry (2026-09-09, 09-17, 10-12, 11-03, 11-19 and 11-20). The daily [`render-version-drift.yml`](../.github/workflows/render-version-drift.yml) (w3/m637, 2026-11-26) runs `scripts/render-version-drift.sh`, which reads the seven version pages Render's `llms.txt` lists and fails when a default bex gives a service created since 2026-06-05 differs from the one Render gives a service created the same day, naming the page, the row and the bex table, and when Render adds a version page or changes one's markup. Its list of reviewed exceptions is empty: a kept difference needs the user's sign-off in the Runtime row of [services-lifecycle.md](ADR018/services-lifecycle.md).

**What the MCP column scores.** A `✅` in the MCP column means the **capability is reachable over MCP** — it has never meant bex ships a tool with Render's name for it, and the Legend's "Render capability = a noun/verb Render exposes on _any_ of its surfaces" already implies this. Stating it explicitly because it is load-bearing for `w1/m71`: consolidating bex-invented tools (folding per-field `set_*` setters into a resource's `update_*`) preserves every `✅` in this column, because the capability stays reachable. Renaming or removing a `Parity1to1` tool does **not** preserve parity, because an agent written against Render's official server calls it by name. The 15 `Parity1to1` tools, and the upstream arguments of the 9 `Superset` ones, are the contract; the 223 `Extension` tools are bex's to reshape.

**Legend.** ✅ parity (evidence pointer) · ◐ partial (divergence documented) · ✖ missing (gap — see backlog) · — deliberate non-goal (rationale inline). "Render capability" = a noun/verb Render exposes on _any_ of its surfaces; a `—` on the REST column can still be a `✅` elsewhere (Render itself splits work across surfaces — e.g. read-only SQL is MCP-only on both sides).

**A fifth surface.** REST/GraphQL/MCP/UI above are bex's own adapters; Render's _official CLI_ (`render-oss/cli`) is a fifth, external surface bex never builds its own competitor to (`.pm/DO_NOT_DO.md`) — [docs/cli-compatibility-checklist.md](cli-compatibility-checklist.md) runs it unmodified against bex-api and tracks the result the same way this ledger does. It surfaced several REST wire-format bugs this matrix's capability-level rows don't catch. Its fixed-client browser login is implemented through Hydra device authorization and locally verified with two real users, refresh rotation, and scoped logout (`w4/m27`); production rollout evidence is still pending. **Agent skills (w8/m143):** Render's `render skills install` gives an agent skills for the platform the user is on. The `bex` launcher therefore replaces upstream's `skills` tree, which installed Render's skills, with an embedded bex pack under the same subcommands and flags. That is parity of behavior, not a divergence ([bex-cli.md](bex-cli.md), checklist `skills` row).

> **Headline.** bex has real parity on the **service lifecycle, env vars, environment groups, secret files, custom domains, managed Postgres lifecycle/data protection/logs, managed Key Value, service logs, metrics, and autoscaling config** core across all four surfaces, and is _ahead_ of Render on several AI-native verbs (API-key management over the API, deploy-from-chat, inbound push webhooks, and GraphQL/MCP webhook management — [§ bex ahead of Render](ADR018/bex-ahead-of-render.md#bex-ahead-of-render)). The remaining wire-format and UI-depth gaps are mapped to owning milestones/notes in the [§ Gap backlog](ADR018/gap-backlog.md#gap-backlog). (Background-worker and cron-job service types shipped in w1/m15; environment groups + secret files in w1/m16; managed Key Value landed on REST/GraphQL/MCP in w2/m7 and the dashboard in w5/m12; autoscaling config shipped in w1/m20; static sites — build → object-store origin + redirects/rewrites/headers — shipped in w1/m21; deploys as first-class objects — list/get/trigger in w2/m5, cancel/rollback in w2/m10, deploy-body params (commitId/clearCache/deployMode) + restart consolidation in w2/m30 — shipped across REST/GraphQL/MCP.)

---

## Services & lifecycle

This section lives in [ADR018/services-lifecycle.md](ADR018/services-lifecycle.md) since 2026-11-23 (w10/054), to keep this file under the 1 MiB blob limit. Amend its rows there.

## Deploys

This section lives in [ADR018/deploys.md](ADR018/deploys.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

## Environment & config

This section lives in [ADR018/environment-config.md](ADR018/environment-config.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

## Custom domains

This section lives in [ADR018/custom-domains.md](ADR018/custom-domains.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

## Managed Postgres

This section lives in [ADR018/managed-postgres.md](ADR018/managed-postgres.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

## Other datastores & storage

This section lives in [ADR018/other-datastores-storage.md](ADR018/other-datastores-storage.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

## Deployment sources & IaC

This section lives in [ADR018/deployment-sources-iac.md](ADR018/deployment-sources-iac.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

### Dashboard-walk interaction follow-through

Moved with its section to [ADR018/deployment-sources-iac.md](ADR018/deployment-sources-iac.md#dashboard-walk-interaction-follow-through).

## Logs

This section lives in [ADR018/logs.md](ADR018/logs.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

## Metrics

This section lives in [ADR018/metrics.md](ADR018/metrics.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

## Identity, workspaces & account

This section lives in [ADR018/identity-workspaces-account.md](ADR018/identity-workspaces-account.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

## Billing & invoicing

This section lives in [ADR018/billing-invoicing.md](ADR018/billing-invoicing.md), added on 2026-11-25 (w6/177) as a file of its own. It holds each billing row's evidence and owner, and the billing choices still waiting for the user's sign-off. Amend its rows there.

## API contract

This section lives in [ADR018/api-contract.md](ADR018/api-contract.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

### Allowlisted response-schema divergences (w3/m272)

Moved with its section to [ADR018/api-contract.md](ADR018/api-contract.md#allowlisted-response-schema-divergences-w3m272).

## Platform events & integrations

This section lives in [ADR018/platform-events-integrations.md](ADR018/platform-events-integrations.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

## Resource-object metadata parity (w6/m28)

This section lives in [ADR018/resource-object-metadata.md](ADR018/resource-object-metadata.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

## bex ahead of Render

This section lives in [ADR018/bex-ahead-of-render.md](ADR018/bex-ahead-of-render.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

## Gap backlog

This section lives in [ADR018/gap-backlog.md](ADR018/gap-backlog.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

## Evidence sources

- **bex side** — [ADR006-bex-api.md](ADR006-bex-api.md) (design + verified-parity notes); `lego/backend/internal/{apps,logs,metrics,apikeys,postgres,secrets}/{rest,graphql,mcp}.go`; `dashboard/src/routes/` + `features/`.
- **Render REST** — OpenAPI spec `https://api-docs.render.com/openapi/render-public-api-1.json`; reference `https://api-docs.render.com/reference/introduction`; endpoint index `https://api-docs.render.com/llms.txt`.
- **Render MCP** — `render-oss/render-mcp-server` v0.3.0 (24 tools): `cmd/server.go` + `pkg/*/tools.go`; docs `https://render.com/docs/mcp-server`.
- **Render dashboard IA** — `render.com/docs` (service-metrics, logging, deploys, scaling, disks, jobs, ssh, custom-domains, postgresql\*, configure-environment-variables, team-members, notifications, audit-logs).
- **Render GraphQL** — captured live in [render-artifacts/](render-artifacts/) (`team-members.graphql`); the dashboard operation names bex mirrors are noted in [ADR006-bex-api.md](ADR006-bex-api.md).

## Account deletion (w2/m84, ADR086)

This section lives in [ADR018/account-deletion.md](ADR018/account-deletion.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

### List-query census (w4/164, 2026-09-27)

Moved with its section to [ADR018/account-deletion.md](ADR018/account-deletion.md#list-query-census-w4164-2026-09-27).

## Render operation omissions

This section lives in [ADR018/render-operation-omissions.md](ADR018/render-operation-omissions.md) since 2026-11-23 (w10/m607), to keep this file under the 1 MiB blob limit. Amend its rows there.

### Sandbox create flags (w2/m263)

Moved with its section to [ADR018/render-operation-omissions.md](ADR018/render-operation-omissions.md#sandbox-create-flags-w2m263).

### CLI runtime copy: `ea blueprints` next steps (w2/m263)

Moved with its section to [ADR018/render-operation-omissions.md](ADR018/render-operation-omissions.md#cli-runtime-copy-ea-blueprints-next-steps-w2m263).
