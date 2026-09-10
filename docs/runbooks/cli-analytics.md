# CLI analytics at obs.bex.co

The [CLI usage dashboard](https://obs.bex.co/d/bex-cli-usage/cli-usage) is the internal ops view of observed CLI usage. Its JSON and datasource live in `deploy/gitops/base/values/grafana.values.yaml`; Argo provisions both. The existing ADR088 ops-workspace login gate controls access. Tenant dashboards and APIs do not expose this dataset.

## Data and access

`POST /v1/cli-telemetry-events` stores authenticated deliveries in `cli_telemetry_events`. The dedicated `bex_cli_analytics` login can select only the projection `cli_analytics.events`, created by `scripts/cli-analytics.sql`; it cannot read the base table or unrelated product tables, create objects in the analytics schema, or change events. The view exposes only analytical fields, omitting event IDs and the client's untrusted workspace claim. Subject and workspace are server-attributed; workspace may be the caller's default, not the client's reported active workspace. Analysts can query the view through Grafana, including its retained identifiers; access is therefore restricted to the ops audience.

The role has no superuser, role/database creation, replication or RLS-bypass privilege. Its connection limit is six, default transactions are read-only, and statement/idle-transaction timeouts are ten seconds. Grafana pools at most four connections, retains at most two idle connections, and verifies the CNPG server hostname and CA. Product counts come from SQL; Prometheus stores no user, workspace or installation labels.

SQL uses the existing receipt-time index and the retained event window. Distinct first-seen/cohort queries may scan all retained rows; the timeouts bound their database occupancy. At materially larger event volume, measure query plans before adding indexes or daily aggregates. Do not silently replace distinct counts with summed daily counts.

## Bootstrap and recovery

1. Select the intended cluster with an explicit `KUBECONFIG`. Migration `0113_cli_telemetry_events` must already be deployed, and `bex-system/bex-db` must have a ready primary.
2. Run `DRY_RUN=1 bash scripts/cli-analytics-bootstrap.sh` to inspect the targets, then `bash scripts/cli-analytics-bootstrap.sh`. It creates/updates the view and restricted role, reuses the existing password (or mints one), and installs `monitoring/grafana-cli-analytics` with `password` and `ca.crt`. No `.env` addition or manually copied password is needed. Namespace overrides use the existing `BEX_SYSTEM_NAMESPACE` / `BEX_MONITORING_NAMESPACE` conventions.
3. Preserve the Secret using the existing SealedSecret custody workflow (ADR016). The production encrypted copy is `deploy/gitops/overlays/prod/grafana-cli-analytics.sealedsecret.yaml`. When resealing, project only `apiVersion`, `kind`, `metadata.name`, `metadata.namespace`, `type` and `data` from the Secret JSON before piping to `kubeseal --controller-namespace kube-system --controller-name sealed-secrets --format yaml`; never carry a last-applied annotation containing plaintext data into the manifest. Annotate the existing Secret `sealedsecrets.bitnami.com/managed=true` to let the controller adopt it, and update the encrypted copy when the password or CA changes. Never print or commit the plaintext Secret.
4. Ship the Grafana values. Its next GitOps rollout reads the password and mounts the CA. For later secret/CA changes, rerun bootstrap and roll only `monitoring/deployment/grafana` after the Secret is installed. Missing bootstrap credentials keep the new Grafana pod pending, rather than substituting the product database owner's credentials.
5. Open the board and verify all three filter queries and the SQL panels. A reachable empty table yields zero count tiles, unavailable rate tiles, and no cohort rows. A broken datasource produces a query error, never a fabricated healthy value.

After database recovery, reapply `scripts/cli-analytics.sql` (bootstrap does this) against the restored schema. Reuse the retained Secret password. Rerun bootstrap after CNPG CA rotation so Grafana receives the new trust root; keep the encrypted copy current.

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
| Collection health | Global server-observed ingest rate, rejection share and p95 ingest latency from Prometheus, plus age of the newest retained SQL event. No requests is not proof of collector health, and quiet CLI usage is not proof of an outage. |

## Collection boundaries

The telemetry-enabled source postdates the `bex-cli/v0.2.1` release. A source build or later telemetry-enabled release is required. The current sender reports upstream version rather than Bex release version, so the board labels it accordingly.

Opt-outs, missing authentication, early setup failures, sender backoff and delivery loss are unobserved. On a fresh headless installation, delivery also waits until the upstream one-time notice has been shown. Root version requests and login/logout are not normal telemetry events. Successful `bex glm`, `muse`, `kimi` and `deepseek` launches replace the process before the completion hook; their launch adoption and provider-session lifetime require separate instrumentation. Arguments, prompts and secret values are never part of the intended event contract.

Rows follow audit retention (90 days by default) and the account-deletion cleanup. Counts and cohorts describe remaining retained observations, not permanent install history or billing-grade usage.

## Verification

Run `python3 -m unittest scripts/test_cli_analytics.py -v` against a disposable local Postgres server selected by `PGHOST`, `PGPORT` and `PGUSER`. The suite creates a uniquely named database and reader role and removes only those afterward. In CI, `BEX_CLI_ANALYTICS_TEST_CONTAINER` selects the pinned ephemeral PostgreSQL service; the GitOps workflow runs the same tests. Never point the test harness at production.

The tests execute every committed SQL panel/filter, verify empty/filtered windows, distinct counts, failure denominators, duration exclusions, marker deduplication, mature cohorts, historical rolling activity, and denied reader mutations/product-table access. `bash scripts/gitops-validate.sh` covers the GitOps render and monitoring contracts.

For live verification, run a real authenticated, telemetry-enabled CLI command and confirm its event appears on the board. Do not insert fixture rows into production to populate charts. Capture desktop and narrow-width views and inspect datasource responses for query errors. With a new dataset, an empty mature-cohort table is expected.
