# Render docs pages

Every page in [Render's documentation index](https://render.com/docs/llms.txt), with what bex does about it and the hash of the text its row was written against (w4/m688). [render-changelog.md](render-changelog.md) sees what Render announces, and [dashboard-operations.md](dashboard-operations.md) what its dashboard bundle can do. Neither sees Render rewriting a docs page, which is where it states its limits, defaults and behaviours. Render moved runtime defaults on its version pages six times between 2026-09-09 and 11-20 without a changelog entry; the daily `render-version-drift` job now compares those seven pages with bex's tables (w3/m637). A reading of three pages no bex document cited found two gaps no ledger had: the Cloudflare headers (w4/m687) and the plan-fee waiver (w4/m689). This file gives every page an owner, and its hash records which version of the page the row describes.

`scripts/render-docs-drift.sh` reads `llms.txt` and fetches each page's markdown copy (`https://render.com/docs/<page>.md`). It hashes the page's text after dropping what a re-render changes on its own: the "For the complete documentation index" banner under the title, image placeholders, trailing whitespace and runs of blank lines. The hash is the first 12 hex digits of that text's SHA-256. The `render-docs` job in [`render-schema-drift.yml`](../../.github/workflows/render-schema-drift.yml) runs the script every Monday. It fails, naming the page and its row's pointer, when a page has no row, when a row's page is no longer in `llms.txt`, when a page's hash moved, when a listed page cannot be fetched, or when a row breaks the rules below. The job's cache keeps each page's text under its hash, so a rewritten page's failure quotes the paragraphs that changed since its row was read. The job never writes this file. A person re-reads the page, re-checks the row's disposition and pointer (and the ADR018 row it names), and copies the new hash from the failure or from `scripts/render-docs-drift.sh hashes <page>`. `scripts/render-docs-drift.test.sh` checks the script against synthetic pages, and this file's rows on every push. No page text is committed: the pages are Render's.

`llms.txt` also lists two topic bundles, `sandboxes/all` and `workflows/all`. Each concatenates pages the index already lists, so they have no rows.

Read 2026-11-27: 225 pages. 124 are product pages, 74 deploy guides, 12 Workflows pages, 11 Sandboxes pages and 4 security advisories. 132 rows are `matched`, 57 `partial`, 9 `gap`, 22 `non-goal` and 5 `n/a`. That day, 32 product pages had at most one `render.com/docs/<page>` reference anywhere in `docs/`, `.pm/`, `lego/`, `dashboard/src` or `scripts/`. Each of them, and each advisory, was read in full for this file, and every claim bex does not match and no item owned got an inbox note: `w4/740`, `w4/741`, `w4/742`, `w4/743`, `w4/744`, `w4/745`, `w4/746`, `w4/747`, `w4/748`, `w4/749`, `w4/750`.

## Dispositions

Each row has one disposition, and its pointer says why:

- `matched`: bex ships what the page describes. The pointer is the ADR018 row, ADR, artifact, shipped board item or code that shows it.
- `partial`: bex ships part of it. The pointer says what bex has and names the board item that owns the rest, or the ADR018 row that records the divergence.
- `gap`: bex lacks the page's subject. The pointer names the open or parked board item that owns it (`wN/mN` or `wN/NNN`); a gap with no owner is filed with `/pm add` before its row lands.
- `non-goal`: bex declines it. The pointer cites the [`DO_NOT_DO.md`](../../.pm/DO_NOT_DO.md) line or the [ADR018](../ADR018-render-parity.md) `—` row.
- `n/a`: there is nothing for bex to match. The pointer gives the reason: a third-party guide whose Render behaviors are matched elsewhere, or Render company news.

A page makes many claims. For a page read in full, the disposition is the weakest that applies: one claim bex lacks makes it `partial`, and the pointer names the item that owns that claim (a new inbox note where none did). Every other page takes its disposition from the ADR018 rows, ledgers and board items that already cite it; when the job first reports such a page changed, read it in full. A deploy guide is `matched` when [render-examples-live.md](render-examples-live.md) deployed its template unchanged and it went live, or failed on Render too for the template's own reason. It is `partial` when the template failed on bex, naming the item. A guide with no template run is `matched` when every capability it uses has an ADR018 row, and its pointer says "not run on bex".

## Pages, in `llms.txt` order

### Start

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`your-first-deploy`](https://render.com/docs/your-first-deploy) | matched | ADR018 § Services & lifecycle: Create service (web / private); the first deploy's page and log ([deploy-detail-page.md](deploy-detail-page.md), [live-deploy-following.md](live-deploy-following.md)) | `2a0b8dca45dd` |
| [`free`](https://render.com/docs/free) | matched | free instances spin down on idle and wake on any request but `/robots.txt` (ADR018 § Deploys, health paths note), free Postgres expires after 30 days and free Key Value is in-memory ([ADR030](../ADR030-pricing.md)), free services cannot send on SMTP ports (ADR018 § Services & lifecycle: Outbound SMTP ports (25 / 465 / 587)), workspace caps (ADR018 § Identity, workspaces & account: Workspace resource-cap visibility) | `3c0ffaa80f69` |
| [`platform-features-by-plan`](https://render.com/docs/platform-features-by-plan) | partial | the plan gates follow Render's table ([pricing.md](pricing.md), [ADR030](../ADR030-pricing.md), `lego/backend/internal/core/plan_gate.go`); SSO, dedicated IPs and HIPAA are declined (ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); ADR018 § Platform events & integrations: Dedicated outbound IPs (—)) | `6dfe0a110bba` |
| [`llm-support`](https://render.com/docs/llm-support) | partial | bex's MCP server and skills pack ship ([ADR025](../ADR025-connect-an-agent.md), `w4/m630`); flagless MCP connect is `w4/m629`, `agents.md` setup is `w4/m632`, the Claude Code plugin is `w4/m631` | `7ea74ee25fcc` |
| [`faq`](https://render.com/docs/faq) | partial | free hours, pipeline minutes, spend limits and included domains ([ADR030](../ADR030-pricing.md) §10, §13, §14, §16), the free spin-down (ADR018 § Services & lifecycle: Suspend / Resume); the Performance pipeline is `w6/m310`, DDoS protection `w21/126`; not billing a DDoS attack's bandwidth is `w4/744`, billing provisioned Postgres storage and no Key Value storage is `w4/745` | `8e3993c8d9bd` |
| [`migrate-from-heroku`](https://render.com/docs/migrate-from-heroku) | partial | the concept map matches ADR018 rows (web, worker, cron, pre-deploy, health checks, Postgres, Key Value, environment groups); log drains are declined (DO_NOT_DO: External log/metric drains); Cursor Origin is `w1/149`, DDoS protection `w21/126` | `d9521d0d321f` |
| [`migrate-from-railway`](https://render.com/docs/migrate-from-railway) | partial | the concept map matches ADR018 rows (private network, disks, `PORT`, health checks, cron, notifications); Slack notifications, PR environments and log streams are declined (DO_NOT_DO); Cursor Origin is `w1/149`, DDoS protection `w21/126` | `7f107f17a771` |

### Compute

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`compute-plans`](https://render.com/docs/compute-plans) | partial | Render's spec-based plan IDs and the Compute page ship ([compute-page.md](compute-page.md), ADR018 § Deployment sources & IaC: Blueprint compute plan IDs); the larger plans are `w6/m332` and `w6/114` | `bd1481e16a1e` |
| [`service-types`](https://render.com/docs/service-types) | matched | ADR018 § Services & lifecycle: Create service (web / private), Background worker (no HTTP port), Cron job (schedule + command + run history), Static site; Postgres and Key Value (ADR018 § Managed Postgres, § Other datastores & storage) | `af91b52f2abd` |
| [`static-sites`](https://render.com/docs/static-sites) | partial | ADR018 § Services & lifecycle: Static site; [ADR029](../ADR029-static-sites.md); serving every static site from a global CDN is `w4/742` | `9f05e01c5829` |
| [`web-services`](https://render.com/docs/web-services) | partial | ADR018 § Services & lifecycle: Create service (web / private) and WebSockets (no maximum duration); port detection and extra ports diverge as ADR018 § Services & lifecycle: Open ports (`serviceDetails.openPorts`) records | `a3f33b818b5e` |
| [`private-services`](https://render.com/docs/private-services) | partial | ADR018 § Services & lifecycle: Create service (web / private) and Private network: who sends and who receives, by service type; UDP and extra ports diverge as ADR018 § Services & lifecycle: Open ports (`serviceDetails.openPorts`) records (`w3/054`) | `f2997fa71324` |
| [`background-workers`](https://render.com/docs/background-workers) | matched | ADR018 § Services & lifecycle: Background worker (no HTTP port) | `85f259002a5b` |
| [`cronjobs`](https://render.com/docs/cronjobs) | matched | ADR018 § Services & lifecycle: Cron job (schedule + command + run history); [cron-runs.md](cron-runs.md) | `fe8c9f24a28d` |
| [`multi-service-architecture`](https://render.com/docs/multi-service-architecture) | partial | service types, internal addresses, `PORT` and Blueprint wiring match (ADR018 § Services & lifecycle: Internal address (private network, Connect → Internal)); Cursor Origin is `w1/149`; a static site served from a global CDN is `w4/742` | `b051a54a2107` |
| [`key-value`](https://render.com/docs/key-value) | matched | ADR018 § Other datastores & storage: Key Value (Valkey / Redis); [ADR021](../ADR021-keyvalue-management.md) | `0a566f6490ec` |
| [`disks`](https://render.com/docs/disks) | partial | ADR018 § Other datastores & storage: Persistent disks ([ADR082](../ADR082-persistent-disks.md)); an image that runs as a non-root user cannot write its disk, `w6/186` | `c5f596c23dce` |

### Deploys

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`deploys`](https://render.com/docs/deploys) | matched | ADR018 § Deploys: Trigger a deploy, Cancel deploy, Zero-downtime rollout (multiple instances); the step timeline Render shows behind `instance_overview` is `w1/155` | `6fd81ba66407` |
| [`language-support`](https://render.com/docs/language-support) | matched | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory; [runtime-inference.md](runtime-inference.md) | `b351a70d811d` |
| [`git-provider`](https://render.com/docs/git-provider) | partial | GitHub matches (ADR018 § Deployment sources & IaC: Git connections (GitHub / GitLab app)); GitLab and Bitbucket are declined (DO_NOT_DO: GitLab, Bitbucket git-provider integration); Cursor Origin is `w1/149` | `d588b68811f5` |
| [`build-pipeline`](https://render.com/docs/build-pipeline) | partial | pipeline minutes are metered and included per plan ([ADR023](../ADR023-usage-metering.md), [ADR030](../ADR030-pricing.md)); the Performance pipeline tier is `w6/m310` | `82c16f3cab64` |
| [`monorepo-support`](https://render.com/docs/monorepo-support) | matched | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory; ADR018 § Services & lifecycle: Build Filters (path-scoped auto-deploy) | `5ea21a4cbf10` |
| [`troubleshooting-deploys`](https://render.com/docs/troubleshooting-deploys) | partial | ADR018 § Logs: Build logs; ADR018 § Services & lifecycle: Health checks (path → readiness gating), Service events / activity feed; a service with no healthy instance answers 503, not Render's 502, `w8/282` | `2e25e04cc7cf` |
| [`service-previews`](https://render.com/docs/service-previews) | non-goal | DO_NOT_DO: PR preview environments; ADR018 § Services & lifecycle: PR preview environments (rejected by the user 2026-07-12); `IS_PULL_REQUEST` and `[skip render]` match | `3d198334ca11` |
| [`preview-environments`](https://render.com/docs/preview-environments) | non-goal | DO_NOT_DO: PR preview environments; ADR018 § Services & lifecycle: PR preview environments; turning previews on is refused by name and the opt-outs are accepted ([ADR049](../ADR049-render-yaml-parity.md)) | `f01b50675415` |
| [`docker`](https://render.com/docs/docker) | matched | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory; ADR018 § Services & lifecycle: Create service (web / private); [docker-build-env.md](docker-build-env.md) | `6f1075312181` |
| [`deploying-an-image`](https://render.com/docs/deploying-an-image) | partial | ADR018 § Deployment sources & IaC: Registry credentials (private images); the pull-failure causes ship (`w4/m669`); the periodic image-availability check is `w4/m671` | `9a55d719124f` |
| [`docker-secrets`](https://render.com/docs/docker-secrets) | matched | ADR018 § Environment & config: Secret files (mounted at `/etc/secrets`); [docker-build-env.md](docker-build-env.md) | `6bc3fea6b55b` |
| [`native-runtimes`](https://render.com/docs/native-runtimes) | partial | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory; the native tools table's `prince` is missing, `w8/336` | `028c544bee0a` |
| [`configure-environment-variables`](https://render.com/docs/configure-environment-variables) | matched | ADR018 § Environment & config: Env vars (list · get · replace · set · delete), Environment groups (+ link / unlink), Secret files (mounted at `/etc/secrets`) | `3342da17673f` |
| [`environment-variables`](https://render.com/docs/environment-variables) | matched | ADR018 § Environment & config: Default environment variables (`RENDER`, `RENDER_*`, `IS_PULL_REQUEST`); [default-env-live.md](default-env-live.md) | `582258ae2585` |

### Sandboxes

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`sandboxes`](https://render.com/docs/sandboxes) | partial | Render's `/v1/sandboxes` API, CLI and SDKs ([ADR104](../ADR104-sandbox-render-parity.md); `w32/m283`–`m285`); memory and processes surviving idle suspension is `w2/m322` | `48f65e79a3fa` |
| [`sandboxes-limits`](https://render.com/docs/sandboxes-limits) | partial | Render's `/v1/sandboxes` API, CLI and SDKs ([ADR104](../ADR104-sandbox-render-parity.md); `w32/m283`–`m285`); the limits are ADR018 § Render operation omissions rows; runtime across suspension is `w2/m322` | `495ad1458b1c` |
| [`sandboxes-creating`](https://render.com/docs/sandboxes-creating) | matched | Render's `/v1/sandboxes` API, CLI and SDKs ([ADR104](../ADR104-sandbox-render-parity.md); `w32/m283`–`m285`); network policy (`w32/m283`); sandbox groups and snapshots (ADR018 § Render operation omissions) | `b27720325038` |
| [`sandboxes-executing`](https://render.com/docs/sandboxes-executing) | matched | Render's `/v1/sandboxes` API, CLI and SDKs ([ADR104](../ADR104-sandbox-render-parity.md); `w32/m283`–`m285`); ADR018 § Render operation omissions: How `exec` runs a command | `b4a4b22d19d2` |
| [`sandboxes-files`](https://render.com/docs/sandboxes-files) | matched | Render's `/v1/sandboxes` API, CLI and SDKs ([ADR104](../ADR104-sandbox-render-parity.md); `w32/m283`–`m285`); ADR018 § Render operation omissions: Entries in a copied directory, Links in a copied directory | `26a4a8409ae2` |
| [`sandboxes-snapshots`](https://render.com/docs/sandboxes-snapshots) | partial | filesystem snapshots ship (`w32/m285`, ADR104); the runtime kind, with memory, follows `w2/m322` | `685804b24fa6` |
| [`sandboxes-claude-managed-agents`](https://render.com/docs/sandboxes-claude-managed-agents) | gap | the agent template's executor install script and network allow-list on the default image are `w22/m447` | `cae108d42c71` |
| [`sandboxes-cursor-cloud-agents`](https://render.com/docs/sandboxes-cursor-cloud-agents) | gap | the agent template's executor install script and network allow-list on the default image are `w22/m447` | `34a78c615c2b` |
| [`sandboxes-openai-agents-api`](https://render.com/docs/sandboxes-openai-agents-api) | gap | the agent template's executor install script and network allow-list on the default image are `w22/m447` | `be8121e3beb7` |
| [`sandboxes-sdk-typescript`](https://render.com/docs/sandboxes-sdk-typescript) | matched | the unmodified Render SDK drives bex sandboxes, each method graded (`w32/m284`, [ADR104](../ADR104-sandbox-render-parity.md)) | `fa3735c6fa77` |
| [`sandboxes-sdk-python`](https://render.com/docs/sandboxes-sdk-python) | matched | the unmodified Render SDK drives bex sandboxes, each method graded (`w32/m284`, [ADR104](../ADR104-sandbox-render-parity.md)) | `b3ba6cc11dec` |

### Workflows

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`workflows`](https://render.com/docs/workflows) | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) | `d8c80ac86427` |
| [`workflows-tutorial`](https://render.com/docs/workflows-tutorial) | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) | `57b38e2825db` |
| [`workflows-limits`](https://render.com/docs/workflows-limits) | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) | `f1df17a34b08` |
| [`workflows-defining-tasks`](https://render.com/docs/workflows-defining-tasks) | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) | `030693789eca` |
| [`workflows-defining-tasks/v0`](https://render.com/docs/workflows-defining-tasks/v0) | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) | `f8d5acfe5685` |
| [`workflows-defining-flows`](https://render.com/docs/workflows-defining-flows) | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) | `e7b9cd96a5b9` |
| [`workflows-running`](https://render.com/docs/workflows-running) | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) | `fec6b6e856b6` |
| [`workflows-local-development`](https://render.com/docs/workflows-local-development) | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) | `09468b697018` |
| [`workflows-sdk-typescript`](https://render.com/docs/workflows-sdk-typescript) | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) | `cd4bf5846d8e` |
| [`workflows-sdk-typescript/v0`](https://render.com/docs/workflows-sdk-typescript/v0) | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) | `2a5ac3b4dbce` |
| [`workflows-sdk-python`](https://render.com/docs/workflows-sdk-python) | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) | `8db0a9d9a8d5` |
| [`workflows-sdk-python/v0`](https://render.com/docs/workflows-sdk-python/v0) | non-goal | DO_NOT_DO: Workflows & tasks; ADR018 § Platform events & integrations: Workflows & tasks (Beta) (—) | `7c5d8ca68daf` |

### Render Postgres

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`postgresql`](https://render.com/docs/postgresql) | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info; [ADR009](../ADR009-postgresql-management.md) | `8edfb17e061a` |
| [`postgresql-creating-connecting`](https://render.com/docs/postgresql-creating-connecting) | matched | ADR018 § Managed Postgres: Postgres CRUD + connection-info; [postgres-connection-limit.md](postgres-connection-limit.md) | `da5995c6dde7` |
| [`postgresql-credentials`](https://render.com/docs/postgresql-credentials) | matched | ADR018 § Managed Postgres: Access control (IP allowlist) · users · pooler | `5d85d51bca1d` |
| [`postgresql-extensions`](https://render.com/docs/postgresql-extensions) | partial | ADR018 § Managed Postgres: Postgres extensions (`CREATE EXTENSION`) ([ADR092](../ADR092-postgres-extension-supply.md)); PostGIS, pg_partman, DuckDB, incremental views, text similarity and time-series are `w8/m92`, `w8/m104`, `w8/m109`–`m112` | `7ec9c23173c0` |
| [`postgresql-upgrading`](https://render.com/docs/postgresql-upgrading) | partial | ADR018 § Managed Postgres: Postgres major-version upgrade ([postgres-version-upgrade.md](postgres-version-upgrade.md)); Render's guided upgrade page is `w3/m676` | `86ac79dcbda7` |
| [`postgresql-logical-replication`](https://render.com/docs/postgresql-logical-replication) | partial | ADR018 § Managed Postgres: Logical replication (publisher), (subscriber), (lifecycle); Render's answers where its page is silent are `w34/001` | `eebba3bc88fb` |
| [`postgresql-legacy-instance-types`](https://render.com/docs/postgresql-legacy-instance-types) | matched | bex has no database from before October 2024 ([compute-page.md](compute-page.md)); a create or plan change naming a legacy plan is refused (`lego/backend/internal/postgres/admission.go`), as Render's are not available for new databases, and storage is billed per GB ([ADR030](../ADR030-pricing.md)) | `4507127b7d93` |
| [`postgresql-backups`](https://render.com/docs/postgresql-backups) | partial | ADR018 § Managed Postgres: Backups · PITR / recovery ([postgres-recovery-window.md](postgres-recovery-window.md)); the window following the workspace plan is `w7/m453` | `6ce9b452b29c` |
| [`postgresql-high-availability`](https://render.com/docs/postgresql-high-availability) | matched | ADR018 § Managed Postgres: HA · failover · read replicas ([postgres-ha.md](postgres-ha.md)) | `62700182ad72` |
| [`postgresql-read-replicas`](https://render.com/docs/postgresql-read-replicas) | matched | ADR018 § Managed Postgres: HA · failover · read replicas | `9d31b1a1bce7` |
| [`postgresql-connection-pooling`](https://render.com/docs/postgresql-connection-pooling) | matched | ADR018 § Managed Postgres: Access control (IP allowlist) · users · pooler | `24d812ef9dd1` |
| [`postgresql-apps`](https://render.com/docs/postgresql-apps) | matched | ADR018 § Managed Postgres: Postgres admin apps (pgAdmin, PgHero) ([postgres-admin-apps.md](postgres-admin-apps.md), `w4/m551`) | `6abc276157b6` |
| [`postgresql-pg-repack`](https://render.com/docs/postgresql-pg-repack) | matched | ADR018 § Managed Postgres: Postgres extensions (`CREATE EXTENSION`) ([ADR092](../ADR092-postgres-extension-supply.md)) | `dd577ec9adb3` |
| [`postgresql-performance-troubleshooting`](https://render.com/docs/postgresql-performance-troubleshooting) | matched | the page's SQL (`pg_stat_activity`, `pg_blocking_pids`, `pg_terminate_backend`, `EXPLAIN`) works because the owner role holds `pg_monitor` ([ADR009](../ADR009-postgresql-management.md) § Insights visibility); ADR018 § Managed Postgres: Postgres observability | `ed43fdc857e3` |

### Networking

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`regions`](https://render.com/docs/regions) | partial | the five region names are accepted and read back, and every resource runs in one placement ([ADR049](../ADR049-render-yaml-parity.md) D7, signed off); bex's own placement `fsn1` is still accepted, `w8/415` | `5f7fa439346c` |
| [`inbound-ip-rules`](https://render.com/docs/inbound-ip-rules) | matched | ADR018 § Custom domains: Inbound IP allowlist (`ipAllowList`) for web services + static sites; ADR018 § Identity, workspaces & account: Workspace inbound IP rules | `aeefb09ac98c` |
| [`web-service-caching`](https://render.com/docs/web-service-caching) | partial | ADR018 § Services & lifecycle: Edge caching (`serviceDetails.cache.profile`), Cache purge, whose row records the cache as one origin tier, not a global CDN: `w4/742` | `e03cd98d3687` |
| [`websocket`](https://render.com/docs/websocket) | matched | ADR018 § Services & lifecycle: WebSockets (no maximum duration) | `a0c5216e4239` |
| [`tls`](https://render.com/docs/tls) | partial | Let's Encrypt certificates for every host (ADR018 § Custom domains: Certificates from Let's Encrypt and Google Trust Services); Google Trust Services is `w21/m618` | `db122cd9c536` |
| [`custom-domains`](https://render.com/docs/custom-domains) | partial | ADR018 § Custom domains: Custom domains (list · add · get · delete), Verify / DNS instructions; wildcard domains are `w4/m326`, Google Trust Services certificates `w21/m618` | `32d177743c82` |
| [`configure-cloudflare-dns`](https://render.com/docs/configure-cloudflare-dns) | matched | ADR018 § Custom domains: Verify / DNS instructions: the apex takes a flattened CNAME, a proxied record verifies once DNS-only and stays verified ([ADR005](../ADR005-custom-domain.md)); the wildcard Worker section works around Render's own Cloudflare edge, which bex does not run ([wildcard-custom-domains.md](wildcard-custom-domains.md)) | `8a0f0061b051` |
| [`configure-namecheap-dns`](https://render.com/docs/configure-namecheap-dns) | partial | ADR018 § Custom domains: Verify / DNS instructions (the `www` CNAME, the apex record, AAAA removal); one stable apex `A` address, as Render's `216.24.57.1`, is `w4/740` | `e2b752f7c586` |
| [`configure-other-dns`](https://render.com/docs/configure-other-dns) | partial | ADR018 § Custom domains: Verify / DNS instructions (CNAME, ALIAS/ANAME or flattening, `dig` checks); one stable apex `A` address, as Render's `216.24.57.1`, is `w4/740` | `9513a145203b` |
| [`private-network`](https://render.com/docs/private-network) | partial | ADR018 § Services & lifecycle: Internal address (private network, Connect → Internal), Private network: who sends and who receives, by service type; any protocol (UDP) is `w3/054` | `d691bf679678` |
| [`private-link`](https://render.com/docs/private-link) | gap | AWS PrivateLink connections, their plan gate and price are `w7/071`, parked on the user's call | `b63cba6aec40` |
| [`outbound-ip-addresses`](https://render.com/docs/outbound-ip-addresses) | partial | the read ships on every surface (ADR018 § Services & lifecycle: Service outbound IPs); a fixed published set is `w21/m616` ([ADR125](../ADR125-stable-tenant-egress.md)) | `47b7a0409ac7` |
| [`dedicated-ips`](https://render.com/docs/dedicated-ips) | non-goal | DO_NOT_DO: Enterprise/managed-infra surfaces (dedicated outbound IPs); ADR018 § Platform events & integrations: Dedicated outbound IPs (—); the shared published set is `w21/m616`, and build traffic through it is `w4/741` | `4ea2a4faf63d` |
| [`outbound-bandwidth`](https://render.com/docs/outbound-bandwidth) | matched | metered and included per plan as on Render ([ADR023](../ADR023-usage-metering.md), [ADR030](../ADR030-pricing.md), [pricing.md](pricing.md)) | `4fd10e830938` |
| [`outbound-connection-resets`](https://render.com/docs/outbound-connection-resets) | partial | ADR018 § Services & lifecycle: Service outbound IPs; stable egress through Floating IPs is `w21/m616` ([ADR125](../ADR125-stable-tenant-egress.md)) | `1d5dc707fc57` |
| [`static-site-headers`](https://render.com/docs/static-site-headers) | matched | ADR018 § Deployment sources & IaC: Header rules · redirects / rewrites; [ADR029](../ADR029-static-sites.md) | `3fa59edf7a6a` |
| [`redirects-rewrites`](https://render.com/docs/redirects-rewrites) | matched | ADR018 § Deployment sources & IaC: Header rules · redirects / rewrites; [ADR029](../ADR029-static-sites.md) | `a34fb72b5272` |

### Operational Controls

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`render-dashboard`](https://render.com/docs/render-dashboard) | matched | ADR018 § Identity, workspaces & account: Workspace lifecycle (create · rename · delete · change plan), Account Appearance, Avatars; ADR018 § Billing & invoicing: Billing page | `d87e2869d809` |
| [`ssh`](https://render.com/docs/ssh) | matched | ADR018 § Services & lifecycle: SSH into a running instance; ADR018 § Identity, workspaces & account: SSH public keys ([ADR035](../ADR035-ssh.md)) | `05c758f5c707` |
| [`projects`](https://render.com/docs/projects) | partial | ADR018 § Deployment sources & IaC: Projects & environments (grouping) ([project-environments.md](project-environments.md), [protected-environments.md](protected-environments.md)); the project page, environment settings, New menu and Move Services are `w1/m682`–`m685` | `6989a5daa50f` |
| [`scaling`](https://render.com/docs/scaling) | matched | ADR018 § Services & lifecycle: Manual scale (instance count), Autoscaling config (min/max + CPU/mem target) | `774829f53d34` |
| [`rollbacks`](https://render.com/docs/rollbacks) | matched | ADR018 § Deploys: Rollback; [rollback-comparison.md](rollback-comparison.md) | `179cc7c649dc` |
| [`maintenance-mode`](https://render.com/docs/maintenance-mode) | matched | ADR018 § Services & lifecycle: Tenant-facing Maintenance Mode; [maintenance-mode.md](maintenance-mode.md) | `8e56bb12dae7` |
| [`one-off-jobs`](https://render.com/docs/one-off-jobs) | non-goal | ADR018 § Services & lifecycle: One-off jobs (run a command) (—); DO_NOT_DO: one-off jobs and ephemeral shell instances | `d266b5ad9d1a` |
| [`infrastructure-as-code`](https://render.com/docs/infrastructure-as-code) | partial | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC, whose open rows are the gap backlog's ([ADR049](../ADR049-render-yaml-parity.md)) | `6dd373eed38a` |
| [`blueprint-spec`](https://render.com/docs/blueprint-spec) | partial | ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC and Blueprint compute plan IDs ([ADR049](../ADR049-render-yaml-parity.md)); a template that reads another service's address starts too early, `w6/189` | `221a58042527` |
| [`terraform-provider`](https://render.com/docs/terraform-provider) | partial | ADR018 § Deployment sources & IaC: Render's Terraform provider (`render-oss/render`) ([terraform-compatibility-checklist.md](../terraform-compatibility-checklist.md)); the open failures are `w22/037` and `w22/038` | `7919f228d300` |

### Observability

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`service-metrics`](https://render.com/docs/service-metrics) | matched | ADR018 § Metrics: Core metrics, Extended metrics ([metrics-page.md](metrics-page.md)) | `92e8d1775a7b` |
| [`health-checks`](https://render.com/docs/health-checks) | matched | ADR018 § Services & lifecycle: Health checks (path → readiness gating); ADR018 § Deploys: Zero-downtime rollout (multiple instances) | `aa8d06d3721b` |
| [`uptime-best-practices`](https://render.com/docs/uptime-best-practices) | partial | ADR018 § Services & lifecycle: Multiple instances run on different nodes, Graceful shutdown; the Cloudflare request headers are `w4/m687` | `e026726a4cf2` |
| [`logging`](https://render.com/docs/logging) | matched | ADR018 § Logs: Application logs (query + live tail), Request / HTTP logs + structured filters, Build logs, Log explorer display and shortcuts | `2e977c0196f3` |
| [`log-streams`](https://render.com/docs/log-streams) | non-goal | ADR018 § Logs: Log streams (external drains) (—); DO_NOT_DO: External log/metric drains; the page's other claims (no static-site logs, a parsed `level`) match ADR018 § Logs: Application logs (query + live tail) | `28413b01076b` |
| [`metrics-streams`](https://render.com/docs/metrics-streams) | non-goal | ADR018 § Metrics: Metric streams (external) (—); DO_NOT_DO: External log/metric drains | `f5aca80a07db` |
| [`metrics-streams-reference`](https://render.com/docs/metrics-streams-reference) | non-goal | ADR018 § Metrics: Metric streams (external) (—); DO_NOT_DO: External log/metric drains | `d9c46d3b23d1` |
| [`webhooks`](https://render.com/docs/webhooks) | partial | ADR018 § Platform events & integrations: Outbound event webhooks ([webhooks-api.md](webhooks-api.md), [webhooks-ui.md](webhooks-ui.md), [datastore-webhook-events.md](datastore-webhook-events.md)), whose ◐ cells record the divergence | `4588c401a493` |
| [`notifications`](https://render.com/docs/notifications) | matched | ADR018 § Platform events & integrations: Notifications (deploy/failure alerts); [ADR052](../ADR052-notifications.md) | `dc5251493cdd` |

### Integrations

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`api-keys`](https://render.com/docs/api-keys) | matched | ADR018 § Identity, workspaces & account: API-key management (create · list · revoke · metadata) | `3011b8fcd1eb` |
| [`mcp-server`](https://render.com/docs/mcp-server) | partial | bex serves Render's MCP tools ([ADR094](../ADR094-coding-agent-mcp.md); ADR018 MCP column); a DCR client connecting with no flags is `w4/m629`, the MCP docs page `w4/730` | `0da4f24a4f32` |
| [`cli`](https://render.com/docs/cli) | matched | the unmodified Render CLI runs against bex, command by command ([cli-compatibility-checklist.md](../cli-compatibility-checklist.md)); `bex` launches it ([ADR113](../ADR113-cli-usage-telemetry.md)) | `43be7f94c885` |
| [`cli-reference`](https://render.com/docs/cli-reference) | matched | every command and flag graded against a live bex ([cli-compatibility-checklist.md](../cli-compatibility-checklist.md)); `workflows` commands are declined there | `04f1207447b4` |
| [`api`](https://render.com/docs/api) | matched | Render's public REST API ([ADR006](../ADR006-bex-api.md); ADR018 § API contract: Rate limits (429 + Retry-After), Error body `code`) | `2f9a123aabc0` |
| [`oidc`](https://render.com/docs/oidc) | partial | each workspace has its own OIDC issuer and native builds get the token (`w2/m476`, `w2/m480`, [ADR116](../ADR116-managed-oidc.md)); AWS, Anthropic and OpenAI, and GCP are `w2/m477`, `w2/m478`, `w2/m479` | `8dee9738f3ce` |
| [`datadog`](https://render.com/docs/datadog) | non-goal | DO_NOT_DO: External log/metric drains; Postgres refuses `datadogAPIKey` by name ([ADR009](../ADR009-postgresql-management.md)); ADR018 § Metrics: Metric streams (external) (—) | `99db081733ba` |

### User Management

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`team-members`](https://render.com/docs/team-members) | matched | ADR018 § Identity, workspaces & account: Workspace members & roles (list · invite · change role · remove); [team-members-roles-2026-11-23.md](team-members-roles-2026-11-23.md) | `816a26129299` |
| [`login-settings`](https://render.com/docs/login-settings) | matched | ADR018 § Identity, workspaces & account: Workspace-enforced 2FA, Workspace-enforced Google login; [login-settings.md](login-settings.md) | `8ff4ca37096c` |
| [`audit-logs`](https://render.com/docs/audit-logs) | matched | ADR018 § Identity, workspaces & account: Audit logs; [audit-logs-api.md](audit-logs-api.md) | `458c63be5b58` |
| [`organizations`](https://render.com/docs/organizations) | partial | ADR018 § Identity, workspaces & account: Organization-enforced 2FA and Google login ([organizations.md](organizations.md)); per-seat billing for Enterprise organizations is `w7/082` | `b5a288b34884` |
| [`saml-sso`](https://render.com/docs/saml-sso) | non-goal | ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—); DO_NOT_DO: Enterprise/managed-infra surfaces | `f2f2182dade9` |

### Platform Protections

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`ddos-protection`](https://render.com/docs/ddos-protection) | partial | ADR018 § Custom domains: Automatic DDoS protection for every tenant host (◐); the user's sign-off or the Cloudflare decision is `w21/126` | `6cfff412e588` |
| [`platform-maintenance`](https://render.com/docs/platform-maintenance) | non-goal | ADR018 § Platform events & integrations: Maintenance runs (—); DO_NOT_DO: Enterprise/managed-infra surfaces (maintenance runs) | `3a5d52225035` |
| [`certifications-compliance`](https://render.com/docs/certifications-compliance) | gap | a Documents page with the GDPR DPA and audit reports for every workspace is `w4/746`; HIPAA-enabled workspaces are `w7/071` | `62f8b9a6ae9f` |
| [`hipaa-compliance`](https://render.com/docs/hipaa-compliance) | gap | HIPAA-enabled workspaces, the BAA flow and the 20% fee are `w7/071`, parked on the user's call | `8a0396f32a1d` |
| [`hipaa-best-practices`](https://render.com/docs/hipaa-best-practices) | gap | HIPAA workspaces are `w7/071`; roles, audit logs, the private network, PITR and encrypted disks and backups match (ADR018 § Identity, workspaces & account: Audit logs; [ADR082](../ADR082-persistent-disks.md) D3); new Postgres and Key Value volumes are encrypted at rest (ADR018 § Managed Postgres and § Other datastores & storage: Encryption at rest); moving existing ones and the registry's images is `w4/m714` | `8a2504d7bbc0` |
| [`shared-responsibility-model`](https://render.com/docs/shared-responsibility-model) | gap | the page itself is `w4/748`, security advisories `w4/749`; the platform duties it lists are done (runtime patching, the private network, status.bex.co), DDoS protection is `w21/126` | `41819dbff1bc` |
| [`penetration-testing`](https://render.com/docs/penetration-testing) | gap | the published testing policy is `w4/748`, parked on the user's approval and a push to the docs site; reports go to GitHub private vulnerability reporting or the `bex.co/.well-known/security.txt` contact, as [`SECURITY.md`](../../SECURITY.md) says; DDoS protection is `w21/126` | `943b19cbecf8` |

### Compare

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`render-vs-heroku-comparison`](https://render.com/docs/render-vs-heroku-comparison) | partial | 100-minute requests, disks, cron, the private network, health checks, zero-downtime deploys and free TLS match (ADR018 rows); Brotli is a signed-off divergence (ADR018 § Deploys); DDoS protection is `w21/126`; a static site served from a global CDN is `w4/742` | `67464834c404` |
| [`render-vs-vercel-comparison`](https://render.com/docs/render-vs-vercel-comparison) | partial | long requests up to 100 minutes and WebSockets match (ADR018 § Services & lifecycle: Request duration / upload timeout, WebSockets (no maximum duration)), with the ◐ divergence that row records | `1fa02558689e` |

### Additional docs

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`backup-postgresql-to-s3`](https://render.com/docs/backup-postgresql-to-s3) | matched | a Blueprint with a Docker cron job, `fromDatabase` and `sync: false` values ([ADR049](../ADR049-render-yaml-parity.md)); ADR018 § Services & lifecycle: Cron job (schedule + command + run history); ADR018 § Managed Postgres: Backups · PITR / recovery | `63012b4a0a62` |
| [`bun-version`](https://render.com/docs/bun-version) | matched | the daily `render-version-drift` job compares the page with bex's dated defaults (`w3/m637`, `lego/operator/internal/build/nativeversion.go`) | `f848a026e8be` |
| [`community`](https://render.com/docs/community) | n/a | Render's community forum moving, company news with no product behavior | `6f3c8d286b1b` |
| [`compute-plans-update`](https://render.com/docs/compute-plans-update) | partial | the 2026-08-26 spec-based plan IDs ship ([compute-page.md](compute-page.md)); the larger plans are `w6/m332` and `w6/114` | `88b678112a84` |
| [`connect-to-mongodb-atlas`](https://render.com/docs/connect-to-mongodb-atlas) | partial | a third-party guide: env vars and Render's region names match (ADR018 § Environment & config: Env vars); a fixed outbound IP set to allowlist in Atlas is `w21/m616` | `31700f99575f` |
| [`connecting-to-redis-with-ioredis`](https://render.com/docs/connecting-to-redis-with-ioredis) | matched | [render-examples-live.md](render-examples-live.md) row 8 (`ioredis`) went live | `c0df09b3c496` |
| [`elixir-erlang-versions`](https://render.com/docs/elixir-erlang-versions) | matched | the daily `render-version-drift` job compares the page with bex's dated defaults (`w3/m637`, `lego/operator/internal/build/nativeversion.go`) | `77e940532da9` |
| [`formspree`](https://render.com/docs/formspree) | n/a | a third-party forms guide; the Render behaviors it uses, a static site's build-time env vars and build command, match (ADR018 § Services & lifecycle: Static site) | `c61f71e19fa5` |
| [`from-github-pages`](https://render.com/docs/from-github-pages) | matched | ADR018 § Services & lifecycle: Static site; Render's Jekyll quickstart builds unchanged, with Ruby taken from `.ruby-version` or the `Gemfile` (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory) | `85ad921a5a60` |
| [`migrate-mongodb-graphql-to-render`](https://render.com/docs/migrate-mongodb-graphql-to-render) | partial | native runtimes, Docker, private services, custom domains and autoscaling match (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory); GitLab and Bitbucket are declined (DO_NOT_DO: GitLab, Bitbucket git-provider integration); Cursor Origin is `w1/149` | `b7d5e5252d65` |
| [`migrate-mongodb-static-hosting-to-render`](https://render.com/docs/migrate-mongodb-static-hosting-to-render) | partial | static sites, TLS, auto-deploy and rollback match (ADR018 § Services & lifecycle: Static site); PR previews are declined (DO_NOT_DO: PR preview environments); Cursor Origin is `w1/149`; serving every static site from a global CDN is `w4/742` | `b35d7aa8c52a` |
| [`node-version`](https://render.com/docs/node-version) | matched | the daily `render-version-drift` job compares the page with bex's dated defaults (`w3/m637`, `lego/operator/internal/build/nativeversion.go`) | `aedb058f19c3` |
| [`poetry-version`](https://render.com/docs/poetry-version) | matched | the daily `render-version-drift` job compares the page with bex's dated defaults (`w3/m637`, `lego/operator/internal/build/nativeversion.go`) | `7c50a1ca8ba9` |
| [`python-version`](https://render.com/docs/python-version) | matched | the daily `render-version-drift` job compares the page with bex's dated defaults (`w3/m637`, `lego/operator/internal/build/nativeversion.go`) | `4f270c73089a` |
| [`quotaguard`](https://render.com/docs/quotaguard) | n/a | a third-party proxy guide; the Render behaviors it uses, env vars and the private network, match (ADR018 § Environment & config: Env vars; ADR018 § Services & lifecycle: Private network: who sends and who receives, by service type) | `00363ce28cb0` |
| [`rails-caching-redis`](https://render.com/docs/rails-caching-redis) | matched | the page's Blueprint runs as written: `type: redis`, `maxmemoryPolicy`, `ipAllowList: []` and `fromService` `connectionString` (ADR018 § Other datastores & storage: Key Value (Valkey / Redis); [key-value.md](key-value.md)) | `f05b7886d9f4` |
| [`ruby-version`](https://render.com/docs/ruby-version) | matched | the daily `render-version-drift` job compares the page with bex's dated defaults (`w3/m637`, `lego/operator/internal/build/nativeversion.go`) | `517991dcb3c1` |
| [`rust-toolchain`](https://render.com/docs/rust-toolchain) | matched | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory; `rust-toolchain` files are read (`lego/operator/internal/build/nativeversion.go`) | `a76ef97c6f70` |
| [`troubleshooting-python-deploys`](https://render.com/docs/troubleshooting-python-deploys) | matched | ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory; `PYTHON_VERSION` and `.python-version` as the version page says (`w3/m637`) | `6c54fbd0b291` |
| [`tutorial-rag-chatbot`](https://render.com/docs/tutorial-rag-chatbot) | n/a | a MongoDB partner tutorial; the Render behaviors it uses, a web service and an env var, match (ADR018 § Services & lifecycle: Create service (web / private)) | `24b469759301` |
| [`uv-version`](https://render.com/docs/uv-version) | matched | the daily `render-version-drift` job compares the page with bex's dated defaults (`w3/m637`, `lego/operator/internal/build/nativeversion.go`) | `3c649a1e1d1a` |
| [`valkey-faq`](https://render.com/docs/valkey-faq) | matched | every Key Value runs Valkey, and every new one Valkey 8 with no version choice on any create surface (`w4/743`); `keyvalue` and `redis` are the same Blueprint type (ADR018 § Other datastores & storage: Key Value (Valkey / Redis); [ADR021](../ADR021-keyvalue-management.md)) | `d4abcaa6957a` |

### Quickstarts

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`deploy-ackee`](https://render.com/docs/deploy-ackee) | matched | [render-examples-live.md](render-examples-live.md) row 21 (`ackee`) fails on Render too, for the template's own reason | `78be177217f6` |
| [`deploy-actix-todo`](https://render.com/docs/deploy-actix-todo) | matched | a Rust web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory) with Postgres (ADR018 § Managed Postgres: Postgres CRUD + connection-info); not run on bex | `e85ed0aa53e1` |
| [`deploy-adminer`](https://render.com/docs/deploy-adminer) | matched | [render-examples-live.md](render-examples-live.md) row 22 (`adminer`) went live | `1011fbece0fe` |
| [`deploy-astro`](https://render.com/docs/deploy-astro) | matched | [render-examples-live.md](render-examples-live.md) row 23 (`astro-ssr`) went live | `1376030aaf5f` |
| [`deploy-beego`](https://render.com/docs/deploy-beego) | matched | a Go web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory); not run on bex | `20763b5ac870` |
| [`deploy-blitz`](https://render.com/docs/deploy-blitz) | matched | [render-examples-live.md](render-examples-live.md) row 24 (`blitzjs`) fails on Render too, for the template's own reason | `ad8c53750de2` |
| [`deploy-bun-docker`](https://render.com/docs/deploy-bun-docker) | matched | [render-examples-live.md](render-examples-live.md) row 14 (`bun-docker`) went live | `be86cde42595` |
| [`deploy-celery`](https://render.com/docs/deploy-celery) | matched | [render-examples-live.md](render-examples-live.md) row 7 (`celery`) fails on Render too, for the template's own reason | `0aaec29119ac` |
| [`deploy-clickhouse`](https://render.com/docs/deploy-clickhouse) | matched | [render-examples-live.md](render-examples-live.md) row 10 (`clickhouse`) went live | `ccb9e296f5bc` |
| [`deploy-create-react-app`](https://render.com/docs/deploy-create-react-app) | matched | a static site (ADR018 § Services & lifecycle: Static site); not run on bex | `1262b9e0b382` |
| [`deploy-django`](https://render.com/docs/deploy-django) | matched | [render-examples-live.md](render-examples-live.md) row 4 (`django`) fails on Render too, for the template's own reason | `02eded663b6e` |
| [`deploy-docusaurus`](https://render.com/docs/deploy-docusaurus) | matched | a static site (ADR018 § Services & lifecycle: Static site); not run on bex | `37feddb839dd` |
| [`deploy-elasticsearch`](https://render.com/docs/deploy-elasticsearch) | partial | [render-examples-live.md](render-examples-live.md) row 26 (`elasticsearch`): the image runs as a non-root user and cannot write its disk, `w6/186` | `b2aa691d65ca` |
| [`deploy-elixir-cluster`](https://render.com/docs/deploy-elixir-cluster) | matched | a Elixir web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory), whose nodes find each other through `RENDER_DISCOVERY_SERVICE` (ADR018 § Environment & config: Default environment variables); not run on bex | `a5d2f3a2f84b` |
| [`deploy-elysiajs`](https://render.com/docs/deploy-elysiajs) | matched | [render-examples-live.md](render-examples-live.md) row 27 (`elysiajs-hello-world`) went live | `0c173e1315a1` |
| [`deploy-fastapi`](https://render.com/docs/deploy-fastapi) | matched | [render-examples-live.md](render-examples-live.md) row 2 (`fastapi`) went live | `fbd9838523f5` |
| [`deploy-fathom-analytics`](https://render.com/docs/deploy-fathom-analytics) | matched | a Docker web service (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory) with Postgres (ADR018 § Managed Postgres: Postgres CRUD + connection-info); not run on bex | `666c86b573b9` |
| [`deploy-flask`](https://render.com/docs/deploy-flask) | matched | a Python web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory); not run on bex | `d4d782fca02c` |
| [`deploy-forem`](https://render.com/docs/deploy-forem) | matched | web, worker, Postgres and Key Value services (ADR018 § Services & lifecycle, § Managed Postgres, § Other datastores & storage); not run on bex | `af7e0d06a4c8` |
| [`deploy-gatsby`](https://render.com/docs/deploy-gatsby) | matched | a static site (ADR018 § Services & lifecycle: Static site); not run on bex | `2d6e0f7e5242` |
| [`deploy-ghost`](https://render.com/docs/deploy-ghost) | matched | [render-examples-live.md](render-examples-live.md) row 30 (`ghost`) went live | `d8beea61119c` |
| [`deploy-go-gin`](https://render.com/docs/deploy-go-gin) | matched | a Go web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory); not run on bex | `db12826514ad` |
| [`deploy-go-nethttp`](https://render.com/docs/deploy-go-nethttp) | matched | [render-examples-live.md](render-examples-live.md) row 3 (`go-web-server`) went live | `7511b597d662` |
| [`deploy-goatcounter`](https://render.com/docs/deploy-goatcounter) | matched | a Docker web service (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory) with Postgres (ADR018 § Managed Postgres: Postgres CRUD + connection-info); not run on bex | `e11e04f707e9` |
| [`deploy-gotify`](https://render.com/docs/deploy-gotify) | matched | a Go web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory) with Postgres (ADR018 § Managed Postgres: Postgres CRUD + connection-info); not run on bex | `ae7b5a7e68e9` |
| [`deploy-hasura-graphql`](https://render.com/docs/deploy-hasura-graphql) | matched | [render-examples-live.md](render-examples-live.md) rows 32 and 33 (`hasura-graphql-metadata-db-only`, `hasura-graphql`) went live | `fbf570e085b2` |
| [`deploy-hooks`](https://render.com/docs/deploy-hooks) | matched | ADR018 § Deploys: Secret deploy hook | `7b9ca984357e` |
| [`deploy-hugo`](https://render.com/docs/deploy-hugo) | matched | a static site (ADR018 § Services & lifecycle: Static site); not run on bex | `a4ffdbf825d5` |
| [`deploy-jekyll`](https://render.com/docs/deploy-jekyll) | matched | a static site (ADR018 § Services & lifecycle: Static site); not run on bex | `b6723796d6c1` |
| [`deploy-matomo`](https://render.com/docs/deploy-matomo) | matched | [render-examples-live.md](render-examples-live.md) row 35 (`matomo`) fails on Render too, for the template's own reason | `ad365fc2f5ae` |
| [`deploy-mattermost`](https://render.com/docs/deploy-mattermost) | matched | [render-examples-live.md](render-examples-live.md) row 36 (`mattermost`) fails on Render too, for the template's own reason | `fc3c0ab9b583` |
| [`deploy-metabase`](https://render.com/docs/deploy-metabase) | matched | [render-examples-live.md](render-examples-live.md) row 39 (`metabase`) went live | `8b641c13f6ac` |
| [`deploy-minio`](https://render.com/docs/deploy-minio) | partial | [render-examples-live.md](render-examples-live.md) row 19 (`minio`): the image runs as a non-root user and cannot write its disk, `w6/186` | `9273d617f4f4` |
| [`deploy-mongodb`](https://render.com/docs/deploy-mongodb) | matched | [render-examples-live.md](render-examples-live.md) row 40 (`mongodb`) went live | `948b6dd269d2` |
| [`deploy-mysql`](https://render.com/docs/deploy-mysql) | matched | [render-examples-live.md](render-examples-live.md) row 42 (`mysql`) went live | `57a97119cbf9` |
| [`deploy-n8n`](https://render.com/docs/deploy-n8n) | matched | [render-examples-live.md](render-examples-live.md) row 12 (`n8n`) went live | `f20c288fd060` |
| [`deploy-nextjs-app`](https://render.com/docs/deploy-nextjs-app) | matched | [render-examples-live.md](render-examples-live.md) row 43 (`nextjs-hello-world`) went live | `2830349a9b03` |
| [`deploy-node-express-app`](https://render.com/docs/deploy-node-express-app) | matched | [render-examples-live.md](render-examples-live.md) row 1 (`express-hello-world`) went live | `53ef83a6f740` |
| [`deploy-node-fastify-app`](https://render.com/docs/deploy-node-fastify-app) | matched | [render-examples-live.md](render-examples-live.md) row 29 (`fastify-hello-world`) went live | `f463153d4ef9` |
| [`deploy-node-hapi-app`](https://render.com/docs/deploy-node-hapi-app) | matched | a Node web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory); not run on bex | `d466c54c4e06` |
| [`deploy-nuxtjs`](https://render.com/docs/deploy-nuxtjs) | matched | [render-examples-live.md](render-examples-live.md) row 44 (`nuxtjs-hello-world`) fails on Render too, for the template's own reason | `71ba0efd9c63` |
| [`deploy-open-web-analytics`](https://render.com/docs/deploy-open-web-analytics) | matched | a Docker web service (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory); not run on bex | `0c8398bf5ea5` |
| [`deploy-openclaw`](https://render.com/docs/deploy-openclaw) | matched | a Blueprint with a Docker service and a persistent disk (ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC; § Other datastores & storage: Persistent disks); not run on bex | `7513681f921f` |
| [`deploy-paradedb`](https://render.com/docs/deploy-paradedb) | matched | [render-examples-live.md](render-examples-live.md) row 45 (`paradedb`) went live | `ff74487f04b0` |
| [`deploy-pgweb`](https://render.com/docs/deploy-pgweb) | matched | a Go web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory) with Postgres (ADR018 § Managed Postgres: Postgres CRUD + connection-info); not run on bex | `9ba72e6a4172` |
| [`deploy-phoenix-distillery`](https://render.com/docs/deploy-phoenix-distillery) | matched | a Elixir web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory) with Postgres (ADR018 § Managed Postgres: Postgres CRUD + connection-info); not run on bex | `33fd5c8af70d` |
| [`deploy-phoenix`](https://render.com/docs/deploy-phoenix) | matched | a Elixir web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory); not run on bex | `db4b95b11ee5` |
| [`deploy-php-laravel-docker`](https://render.com/docs/deploy-php-laravel-docker) | matched | a Docker web service (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory) with Postgres (ADR018 § Managed Postgres: Postgres CRUD + connection-info); not run on bex | `90c9f393b2c7` |
| [`deploy-prisma-orm`](https://render.com/docs/deploy-prisma-orm) | matched | a Node web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory) with Postgres (ADR018 § Managed Postgres: Postgres CRUD + connection-info); not run on bex | `69dc1abd8710` |
| [`deploy-prometheus`](https://render.com/docs/deploy-prometheus) | partial | [render-examples-live.md](render-examples-live.md) row 49 (`prometheus`): the image runs as a non-root user and cannot write its disk, `w6/186` | `5e534d5e1e67` |
| [`deploy-puppeteer-node`](https://render.com/docs/deploy-puppeteer-node) | matched | a Node web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory); not run on bex | `bfabd3dabd50` |
| [`deploy-rabbitmq`](https://render.com/docs/deploy-rabbitmq) | partial | [render-examples-live.md](render-examples-live.md) row 53 (`rabbitmq`): the image runs as a non-root user and cannot write its disk, `w6/186` | `790f78b429fe` |
| [`deploy-rails-6-7`](https://render.com/docs/deploy-rails-6-7) | matched | [render-examples-live.md](render-examples-live.md) rows 5 and 54 (`rails-7`, `rails-6`) went live | `bf74fd6c9777` |
| [`deploy-rails-8`](https://render.com/docs/deploy-rails-8) | matched | a Ruby web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory) with Postgres (ADR018 § Managed Postgres: Postgres CRUD + connection-info), a pre-deploy command and a worker; not run on bex | `2820c5576158` |
| [`deploy-rails-sidekiq`](https://render.com/docs/deploy-rails-sidekiq) | matched | a Blueprint with a Ruby web service, a background worker and Key Value (ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC); not run on bex | `94dc1d437da5` |
| [`deploy-redash`](https://render.com/docs/deploy-redash) | matched | [render-examples-live.md](render-examples-live.md) row 55 (`redash`) went live | `54ee4c09812a` |
| [`deploy-redwood`](https://render.com/docs/deploy-redwood) | matched | [render-examples-live.md](render-examples-live.md) row 16 (`redwoodjs`) fails on Render too, for the template's own reason | `fd4ad744ada0` |
| [`deploy-remix`](https://render.com/docs/deploy-remix) | matched | [render-examples-live.md](render-examples-live.md) row 57 (`remix-postgres`) fails on Render too, for the template's own reason | `6d232803e187` |
| [`deploy-retool`](https://render.com/docs/deploy-retool) | matched | [render-examples-live.md](render-examples-live.md) row 59 (`retool`) fails on Render too, for the template's own reason | `5912239a30d2` |
| [`deploy-rocket-rust`](https://render.com/docs/deploy-rocket-rust) | matched | a Rust web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory); not run on bex | `05a2187517e7` |
| [`deploy-rust-graphql`](https://render.com/docs/deploy-rust-graphql) | matched | a Rust web service on the native runtime (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory); not run on bex | `242ce94fa655` |
| [`deploy-shopify-app`](https://render.com/docs/deploy-shopify-app) | matched | a Docker web service (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory) with Postgres (ADR018 § Managed Postgres: Postgres CRUD + connection-info); not run on bex | `771d5cf7bfc8` |
| [`deploy-shynet`](https://render.com/docs/deploy-shynet) | matched | [render-examples-live.md](render-examples-live.md) row 60 (`shynet`) went live | `19df18381f35` |
| [`deploy-sidekiq-worker`](https://render.com/docs/deploy-sidekiq-worker) | matched | [render-examples-live.md](render-examples-live.md) row 6 (`sidekiq`) fails on Render too, for the template's own reason | `2a2c36ded4e5` |
| [`deploy-strapi`](https://render.com/docs/deploy-strapi) | matched | [render-examples-live.md](render-examples-live.md) rows 15 and 61 (`strapi-sqlite`, `strapi-postgres`) went live | `eec7b28c0034` |
| [`deploy-svelte`](https://render.com/docs/deploy-svelte) | matched | [render-examples-live.md](render-examples-live.md) row 62 (`svelte`) went live | `771f003d20d8` |
| [`deploy-sveltekit`](https://render.com/docs/deploy-sveltekit) | matched | [render-examples-live.md](render-examples-live.md) rows 63 and 64 (`sveltekit-static`, `sveltekit`) went live | `c21091d20b90` |
| [`deploy-temporal`](https://render.com/docs/deploy-temporal) | partial | [render-examples-live.md](render-examples-live.md) row 66 (`temporal`): its worker starts before the schema exists, `w6/189`, and its Elasticsearch cannot write its disk, `w6/186` | `86b4cd69bc63` |
| [`deploy-to-render`](https://render.com/docs/deploy-to-render) | matched | bex's `/deploy?repo=` link creates the Blueprint from the repository (`dashboard/src/routes/deploy.tsx`; ADR018 § Deployment sources & IaC: Blueprint / `render.yaml` IaC) | `ddc3b05ab805` |
| [`deploy-vue-js`](https://render.com/docs/deploy-vue-js) | matched | a static site (ADR018 § Services & lifecycle: Static site); not run on bex | `ecdcbf487457` |
| [`deploy-webdis-docker`](https://render.com/docs/deploy-webdis-docker) | matched | a Docker web service (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory); not run on bex | `ce27dbdec6be` |
| [`deploy-wordpress`](https://render.com/docs/deploy-wordpress) | partial | [render-examples-live.md](render-examples-live.md) row 20 (`wordpress`): the image runs as a non-root user and cannot write its disk, `w6/186` | `fd241841abf5` |
| [`deploy-yc-qm`](https://render.com/docs/deploy-yc-qm) | partial | a project with Postgres, a disk-backed MinIO and a Docker service ship; its `qm check --live` runs a one-off job, which bex declines (ADR018 § Services & lifecycle: One-off jobs (run a command) (—)) | `4ce0c7e1ca7f` |
| [`deploy-zulip`](https://render.com/docs/deploy-zulip) | matched | a Docker web service (ADR018 § Services & lifecycle: Runtime, build strategy + Root Directory) with Postgres (ADR018 § Managed Postgres: Postgres CRUD + connection-info); not run on bex | `b4b303aeea80` |

### Advisories

| Page | Disposition | Pointer | Hash |
| --- | --- | --- | --- |
| [`advisory-emudhra`](https://render.com/docs/advisory-emudhra) | matched | the native Node, Go and Bun lines are patched and the common image carries bookworm-security's CA store (`lego/operator/internal/build/native.go`, `nativebases.go`); telling customers is `w4/749` | `70f1e19e08b9` |
| [`advisory-php2shell`](https://render.com/docs/advisory-php2shell) | n/a | bex has no native PHP runtime, as Render runs PHP only on Docker, so the fix is the customer's image | `5c5659b367c8` |
| [`advisory-ruby2shell`](https://render.com/docs/advisory-ruby2shell) | matched | the patched Ruby 3.3.14, 3.4.13 and 4.0.10 lines ship (`w3/m635`, `lego/operator/internal/build/native.go`); telling customers is `w4/749` | `0cf28ca5390d` |
| [`advisory-upload2shell`](https://render.com/docs/advisory-upload2shell) | partial | new native builds carry the fixed Ghostscript (`0f4ab445b0`, `nativebases.go`), and the one running service built before it was rebuilt on 2026-11-27 (`w4/750`); the advisory to owners is `w4/749` | `a307c053d33a` |
