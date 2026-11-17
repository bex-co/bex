# CLI analytics at obs.bex.co

The [CLI usage dashboard](https://obs.bex.co/d/bex-cli-usage/cli-usage) is the internal ops view of observed CLI usage. Its JSON and datasource live in `deploy/gitops/base/values/grafana.values.yaml`; Argo provisions both. The existing ADR088 ops-workspace login gate controls access. Tenant dashboards and APIs do not expose this dataset.

## Data and access

`POST /v1/cli-telemetry-events` stores authenticated deliveries in `cli_telemetry_events`. The dedicated `bex_cli_analytics` login can select only the projection `cli_analytics.events`, created by `scripts/cli-analytics.sql`; it cannot read the base table or unrelated product tables, create objects in the analytics schema, or change events. The view exposes only analytical fields, omitting event IDs and the client's untrusted workspace claim. Subject and workspace are server-attributed; workspace may be the caller's default, not the client's reported active workspace. Analysts can query the view through Grafana, including its retained identifiers; access is therefore restricted to the ops audience.

The role has no superuser, role/database creation, replication or RLS-bypass privilege. Its connection limit is six, default transactions are read-only, and statement/idle-transaction timeouts are ten seconds. Grafana pools at most four connections, retains at most two idle connections, and verifies the CNPG server hostname and CA. Product counts come from SQL; Prometheus stores no user, workspace or installation labels.

Grafana uses the Postgres plugin bundled in its pinned image. `plugins.preinstall_disabled=true` disables the automatic installer: otherwise Grafana 13 stops the bundled plugin before trying to replace its files, then fails on the deliberately read-only root filesystem, leaving `plugin.notRegistered`. See [the pinned Grafana configuration implementation](https://github.com/grafana/grafana/blob/v13.2.1/pkg/setting/setting_plugins.go).

SQL uses the existing receipt-time index and the retained event window. Distinct first-seen/cohort queries may scan all retained rows; the timeouts bound their database occupancy. At materially larger event volume, measure query plans before adding indexes or daily aggregates. Do not silently replace distinct counts with summed daily counts.

## Bootstrap and recovery

1. Select the intended cluster with an explicit `KUBECONFIG`. Migrations `0113_cli_telemetry_events` and `0116_cli_telemetry_bex_version` must already be deployed, and `bex-system/bex-db` must have a ready primary. The view selects `bex_version`, so applying it before 0116 fails closed rather than serving a board without the release axis.
2. Run `DRY_RUN=1 bash scripts/cli-analytics-bootstrap.sh` to inspect the targets, then `bash scripts/cli-analytics-bootstrap.sh`. It creates/updates the view and restricted role, reuses the existing password (or mints one), and installs `monitoring/grafana-cli-analytics` with `password` and `ca.crt`. No `.env` addition or manually copied password is needed. Namespace overrides use the existing `BEX_SYSTEM_NAMESPACE` / `BEX_MONITORING_NAMESPACE` conventions.
3. Preserve the Secret under SealedSecret custody (ADR016): rerun step 2 with `SEAL_TO=deploy/gitops/overlays/prod/grafana-cli-analytics.sealedsecret.yaml` (requires `kubeseal`) and ship that manifest. The shared bootstrap follows the same custody contract as the product reader (see [product-analytics.md](product-analytics.md) bootstrap step 3): a SealedSecret-owned Secret is never rewritten, and CA drift fails closed until resealed. Never print or commit the plaintext Secret.
4. Ship the Grafana values. Its next GitOps rollout reads the password and mounts the CA. For later secret/CA changes, rerun bootstrap and roll only `monitoring/deployment/grafana` after the Secret is installed. Missing bootstrap credentials keep the new Grafana pod pending, rather than substituting the product database owner's credentials.
5. Open the board and verify all three filter queries and the SQL panels. A reachable empty table yields zero count tiles, unavailable rate tiles, and no cohort rows. A broken datasource produces a query error, never a fabricated healthy value.

After database recovery, reapply `scripts/cli-analytics.sql` (bootstrap does this) against the restored schema. Reuse the retained Secret password. After CNPG CA rotation, rerun bootstrap with `SEAL_TO` and ship the reseal so Grafana receives the new trust root, then roll Grafana.

## Metric definitions

The default range is seven days, refresh is five minutes, and daily/cohort boundaries are UTC. The selected command, upstream version and output filters apply to all product panels. Last-event freshness and the Prometheus collection-health panels remain global. SQL filters quote values; an empty dataset still has an explicit All selection.

| Metric | Definition and interpretation |
| --- | --- |
| Observed commands | Stored event deliveries in the selected receipt-time window, including help. Delivery is best-effort; this is not every CLI invocation. |
| Active installations / identities / workspaces | Distinct nonblank identifiers in the window. Installations are not people; one identity can have several installations and automation can report an identity. Daily and rolling 1/7/30-day counts are distinct within each window, never sums of daily values. |
| First seen / returning | Earliest observation under the selected filters in retained history. Returning means activity on a later UTC day. Multiple invocations on the first day still count as one first-seen installation. Purges and account deletion can move that boundary. |
| Next-week return | First-seen UTC calendar-week cohort, followed by any observed activity under the same filters during the next calendar week. Only cohorts whose entire next week has elapsed at the selected end are shown. The first cohort can be truncated by collection/retention history. Extend the range to 30/90 days as history accumulates; recent cohorts are not zero-retention cohorts. |
| Failure share | Unsuccessful classified attempts divided by classified attempts. Discovery, validation, setup and execution errors count as failures; success/explicit-exit use the exit code. Help, version and unknown outcome kinds are excluded. An empty denominator is unavailable. A successful CLI deploy command does not establish the deployment's eventual Ready outcome. |
| Command duration | Nonnegative duration for success, execution-error or explicit-exit events. The main command table uses non-interactive, non-streaming samples only. The duration table separates interactive, non-interactive and session/stream lifetimes (logs, ssh, shell, psql); it shows sample counts and p50/p95. Flags are not collected, so wait modes cannot be split further. |
| Agents / CI | Presence of reported environment-marker names; neither absence nor terminal attachment proves human use. Agent and CI shares overlap. Multiple markers for the same family/provider count once per invocation. Generic CI appears only without a more specific marker; multiple different families/providers can count the same invocation. |
| Environment / versions | Observed OS/architecture, output format, TUI-launched state and upstream Render CLI version. An installation can appear in multiple categories over time; category counts are not additive. |
| Collection health | Global, from Prometheus: ingest rate, rejection share, p95 ingest latency, telemetry coverage, stored events by completion kind and backoff-causing answers, plus the age of the newest retained SQL event. Quiet CLI usage is not an outage; telemetry coverage tells the two apart (see [Collection health](#collection-health)). |

## Collection boundaries

The telemetry-enabled source postdates the `bex-cli/v0.2.1` release. A source build or later telemetry-enabled release is required. Two version axes are now reported and filterable independently: the pinned upstream Render CLI version, and the bex launcher's own release (`X-Bex-CLI-Version`, w5/m94). A client that sends no release header -- an unmodified upstream `render` binary, or any build older than the header -- is recorded as `unknown`, which is a real population rather than a gap.

Opt-outs, missing authentication, early setup failures, sender backoff and delivery loss are unobserved; an unauthenticated run has no identity to attribute an event to, so it is silent by construction. Outside CI, nothing is sent until the Bex telemetry notice has been shown on that machine (ADR113 D5, the same gate as Render's CLI): the run that shows it sends nothing, and a machine that never runs `bex` in a terminal never sends unless `scripts/install-bex.sh` (or another installer running `bex analytics notice`) showed it. The launcher-native paths below wait for the same notice. Login and logout remain non-events.

The release that ships this gate makes every machine start one run later, including machines that had already seen Render's notice and so sent before, and `--version` and provider launches stop reporting on machines that never saw it. Counts dip at that release. The board's "Bex releases" annotation marks where each launcher release first reported, so the dip lines up with a marker rather than looking like an outage. Root version requests are now reported with completion kind `version`, and a successful `bex code` / `bex glm` / `muse` / `kimi` / `deepseek` launch reports itself immediately before it replaces the process (w5/m94); both carry only the command path, never the arguments handed to the launched agent. What the launched agent then does is the provider's session, not a bex command, and is never collected.

Rows follow audit retention (90 days by default) and the account-deletion cleanup. Counts and cohorts describe remaining retained observations, not permanent install history or billing-grade usage.

## Collection health

The bottom row of the board answers "is the collector quiet or broken?" (ADR113 D6, D8). Two Prometheus counters feed it, and neither carries a value a client sent:

- `bex_api_cli_requests_total{client}` counts the REST requests whose `User-Agent` is the CLI's (`render-cli/…`), excluding the telemetry deliveries themselves. `client` is `bex` when the launcher's `X-Bex-CLI-Version` header is present and `render-cli` for an unmodified Render CLI pointed at bex. It is counted on the server, so it includes opted-out clients and machines that never passed the disclosure gate.
- `bex_api_cli_telemetry_events_total{completion_kind, output_format, client}` counts stored events. Each label is clamped to a closed set (the eight upstream kinds plus `other`, the four formats plus `unknown`, the two clients). Commands and versions are not labels: ask the retained rows.

Every replica exports all of their series, and the ingest route's `202`, `429`, `500` and `503` answers, at 0 when it starts. A series that first appears at 1 is invisible to `increase()`, and each rollout starts both replicas empty, so without this the first events and the first failure on every new pod went uncounted.

| Panel | What it shows | Alert |
| --- | --- | --- |
| Telemetry coverage | Stored events per CLI request over the last hour, by client, against the requests themselves. The level depends on how many API calls a command makes; read the trend. A drop toward zero while requests continue means collection stopped. | `CLITelemetryIngestSilent` |
| Stored events by completion kind | Events per hour by kind, the execution_error share of the last 30 minutes (`bex:cli_telemetry_execution_error_share:ratio30m`), and the alert line at three times the 7-day share (`bex:cli_telemetry_execution_error_share:avg7d`), the rules the alert itself reads. | `CLIExecutionErrorShareHigh` |
| Backoff-causing answers | Deliveries answered 429 (the CLI stops sending for 5 minutes) or 503 (30 minutes). A 404 (an hour) is counted under `route="unmatched"` and cannot appear here. | `CLITelemetryIngestFailing` |

All three alerts are warning tier: they reach the daily digest, never a page.

- **`CLITelemetryIngestFailing`**: more than 5% of deliveries answered 5xx for 15 minutes, with at least two failed deliveries in each 10-minute window. `503` means the control-plane store is off (`BEX_CP_DB_URI` unset); `500` means the insert failed. Read `kubectl -n bex-system logs deploy/bex-api`, then the `bex-db` primary. Every minute of it costs each affected CLI half an hour of events.
- **`CLITelemetryIngestSilent`**: at least 30 CLI requests in two hours, some in the last 30 minutes, and no event stored in two hours. In order: the route is still mounted (`curl -s -o /dev/null -w '%{http_code}' -X POST https://api.bex.co/v1/cli-telemetry-events` answers `401`, not `404`); a fleet-wide backoff (the failing alert and the backoff panel); a launcher release in the last day that changed what is sent. A population of clients that never sends (opted out, or headless and never shown the notice) can trip it on its own: the coverage panel splits by client.
- **`CLIExecutionErrorShareHigh`**: the execution_error share of the last 30 minutes is over three times its 7-day share, with at least 10 events and 5 execution errors. Find the command in SQL (`SELECT command, workspace_id, count(*) FROM cli_analytics.events WHERE received_at > now() - interval '1 hour' AND completion_kind = 'execution_error' GROUP BY 1, 2 ORDER BY 3 DESC`), then its REST route on the bex-api gateways board and the last bex-api deploy. One workspace running a failing command in a loop (a QA sweep, an agent) also trips it; check the workspace before rolling anything back.

## Verification

Run `python3 -m unittest scripts/test_cli_analytics.py -v` against a disposable local Postgres server selected by `PGHOST`, `PGPORT` and `PGUSER`. The suite creates a uniquely named database and reader role and removes only those afterward. In CI, `BEX_CLI_ANALYTICS_TEST_CONTAINER` selects the pinned ephemeral PostgreSQL service; the GitOps workflow runs the same tests. Never point the test harness at production.

The tests execute every committed SQL panel/filter, verify empty/filtered windows, distinct counts, failure denominators, duration exclusions, marker deduplication, mature cohorts, historical rolling activity, and denied reader mutations/product-table access. `bash scripts/gitops-validate.sh` covers the GitOps render and monitoring contracts.

For live verification, run a real authenticated, telemetry-enabled CLI command and confirm its event appears on the board. Do not insert fixture rows into production to populate charts. Capture desktop and narrow-width views and inspect datasource responses for query errors. With a new dataset, an empty mature-cohort table is expected.
