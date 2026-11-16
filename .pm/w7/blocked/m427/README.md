# w7 · m427 — Service metrics reach back as far as the workspace plan keeps them: 7, 14 or 30 days, as on Render

**Worker:** worker7 **Goal:** every metrics read (services and datastores, on REST, GraphQL, MCP and the dashboard) reaches back exactly as far as the workspace's plan keeps metrics on Render: Hobby 7 days, Pro 14, Scale and Enterprise 30, with the store really holding 30 days. **Status:** blocked — date gate, no user action (t001–t004, t007 and t008 done and shipped: `e63162236` the volume, `d1df2ee3a` the 30-day retention with its alert and guard, `ee88c6a4a` the read floor and `metricsRetention`, `781c66f31` the dashboard picker; t005, t006 and t009 wait for 2026-11-24T15:13Z, eight days after the retention rolled out)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Measure Prometheus's tenant series and choose how 30 days are kept; record it in ADR010; mark the ADR018 row ◐ — **DONE** | 60m | — |
| t002 | GitOps: metrics are kept 30 days on a volume sized from the measurement, with a fill alert — **DONE** | 90m | t001 |
| t003 | Per-plan read floor on every metrics read path — **DONE** | 90m | t001 |
| t004 | `metricsRetention` and dashboard ranges bounded by the plan — **DONE** | 60m | t003 |
| t005 | Live check on production, date-gated: eight days after t002 rolls out | 45m | t002, t004 |
| t006 | Render parity | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

On production, as the QA user:

1. The metrics store keeps tenant-facing series for 30 days, on a volume sized from t001's measurement, and its fill alert is healthy.
2. Eight days after that rollout, for a service that has run the whole time: a Hobby workspace's metrics read with a start 14 days back (REST `/v1/metrics/cpu`, GraphQL `metrics`, MCP `get_metrics` and the dashboard) returns nothing older than 7 days, and a Scale workspace's returns all eight days. Pro's 14-day window is covered by tests.
3. The dashboard offers no preset and no custom range beyond the plan's window, and names the plan that extends it.
4. ADR010 and ADR018 state the per-plan windows.
5. `cd lego/backend && go test ./...`, `make lint`, `bash scripts/gitops-validate.sh`, and dashboard `yarn lint` + `yarn test` pass.

The full 30-day horizon can only be observed 30 days after the rollout. The closeout files a dated inbox note for that check instead of holding the milestone open.

## Record

**2026-11-16, implementation (t001–t004, with t007 and t008 run ahead of the production check).** What shipped is in each task file under `done/`. In short:

- **Reads** (`ee88c6a4a`). One rule under the two core verbs floors every chart read at the workspace plan's window: Hobby 7 days, Pro 14, Scale and Enterprise 30. A start before the floor moves forward along the request's own grid, a range wholly before it answers empty, and status-code discovery takes the same floor. Month-to-date bandwidth and `/v1/usage` are not floored.
- **Dashboard** (`781c66f31`). The Metrics page's range picker disables the presets wider than the plan's window, refuses a custom start before it, and says what each plan keeps. It reads the window from the new GraphQL `metricsRetention`.
- **Store** (`e63162236`, then `d1df2ee3a`). Production Prometheus keeps 30 days, capped at 30GB of blocks, on a 40Gi volume that was grown first and confirmed before the retention was raised. `MetricsStoreVolumeFillingUp` and `scripts/metrics-retention-guard.sh` watch it.
- **Checked on production that day: the store only.** The retention flags and limits, the grown claim and filesystem, one clean restart (replay 15.8 s, peak 966 MiB of the 2 GiB limit, no container restart, every target up), and the new alert rule healthy and inactive. The figures are in t002's record.
- **Not yet checked: anything a workspace sees.** The read floor and the picker have run only in tests. Definition-of-done items 2 and 3 are t005, and the picker was not walked in a browser here (the local mock cluster was down).

**2026-11-16, parked on the date gate (t005 step 1).** The retention rolled out at **2026-11-16T15:13:04Z**, when the server started with `--storage.tsdb.retention.time=30d`. t005 needs the store to hold more than a Hobby window, so it runs eight days later.

- **The gate clears by itself at 2026-11-24T15:13Z.** Nothing is needed from anyone.
- **The checks that prove it cleared**, the date and that production serves the read floor (the image pin carries the dashboard commit, which contains the backend one):

  ```bash
  python3 -c 'import datetime as d; print(d.datetime.now(d.timezone.utc) >= d.datetime(2026, 11, 24, 15, 13, 4, tzinfo=d.timezone.utc))'
  git fetch origin main
  pin=$(git log origin/main --grep='pin platform images to' -1 --format=%s | sed -E 's/.*pin platform images to ([0-9a-f]+).*/\1/')
  git merge-base --is-ancestor 781c66f31 "$pin" && echo "the pin carries 781c66f31"
  ```

- **Then:** t005 (the production walk on a Hobby and a Scale workspace as the QA user), t006 (Render parity), t009 (closeout).

**For t005.**

- **What the store holds.** It had 3.38 days of history at the rollout (the oldest sample is from about 2026-11-13T06:00Z), and none was dropped. On 2026-11-24 it holds about 11.4 days. So a Scale read that starts 14 days back begins near 2026-11-13, earlier than "about the rollout time" in step 3, and a Hobby read's first point must be no older than 7 days.
- **The first point sits on the request's grid.** It is the first `start + k × step` at or after now minus the window, so it can be up to one step younger than the floor. It is never older.
- **Check the store again there:** `prometheus_tsdb_retention_limit_seconds` is 2,592,000, `time() - prometheus_tsdb_lowest_timestamp_seconds` is past 8 days, and `MetricsStoreVolumeFillingUp` is inactive.
- **Production access.** The local SSH host-key pin was stale on 2026-11-16. t001's record says how the kubeconfig was fetched through ADR019's recovery flow without changing `.env` or the pin files.
- **The 30-day note.** Date it 2026-12-16, rollout plus 30 days. The Scale window is fully stored from 2026-12-13, because of the 3.38 days the store already had. Ask it for a restart of the server with 30 days of blocks on disk, watching the replay time and the memory peak against the 2 GiB limit, and for the volume's fill against the 0.47 GB a day it was sized on. `w7/m430` adds datastore series, and its t002 records them.

**Order.** t007 (Simplify) and t008 (Test coverage) list t006 as a dependency and were done first, because the review and the tests had to precede the ship. If t005 or t006 changes code, both get another pass.

**Found on the way, filed:** `w7/066` (the local gofmt gates use whichever `gofmt` is first on PATH, which is not the version CI formats with) and `w7/067` (Loki's production volume is 20Gi while the manifest and its guard say 60Gi).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for render parity`, 2026-11-16, item 2; filed by `/pmx them all to w7` the same day.
- **Render reference:** render.com/docs/service-metrics § Metrics retention period, read 2026-11-16: Hobby 7 days, Pro 14 days, Scale / Enterprise 30 days. render.com/pricing lists "Extended metrics retention" under Scale.
- **bex today:** `deploy/gitops/base/prometheus.yaml:289` sets `retention: 3d` on an 8Gi volume, and the production overlay patches only memory (`deploy/gitops/overlays/prod/kustomization.yaml`, w4/m144). The dashboard offers 7d, 14d and 30d presets on every plan (`dashboard/src/features/metrics/lib/range.ts`: "Ranges beyond Prometheus's retention … simply show the retained window"). `w6/done/065` measured a 720h read returning about three days and left the retention as "a capacity question … out of scope". `lego/backend/internal/core/plan_gate.go` has `LogRetention` and `LogFloor` (w9/m389) and no metrics counterpart. ADR018 § Metrics says nothing about retention.
- **Goal linkage:** ADR008 Render parity first; ADR010 (observability); ADR030 §9 (workspace plans gate Render's plan features).
- **Expected outcome:** a 7-day chart shows seven days of data, and metrics history is a real difference between plans, as on Render.
- **Why now:** every bex workspace keeps less metrics history than Render's lowest plan. Logs got the same rule two days ago in w9/m389, so the mechanism (a per-plan floor in the core verb and a bounded range picker) is fresh and can be copied.
- **Decision state:** the windows are Render's documented ones. One point is this proposal's recommendation, not a user decision: Render says an upgrade does not reveal older lines for logs, and says nothing for metrics, so this milestone applies the current plan's window with no memory of earlier plans. That is unverified against Render.
- **Sequencing:** `w7/m430` adds datastore series. Its t002 records the added series so this milestone's sizing can be re-checked.
- **Render parity:** included: metrics reads change on REST, GraphQL, MCP and the dashboard.
