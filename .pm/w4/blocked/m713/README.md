# w4 · m713 — A Postgres database bills its provisioned storage, and a Key Value instance carries no storage charge, as on Render

**Worker:** worker4 **Goal:** a Postgres invoice charges the storage capacity the database has (its plan or chosen size, grown by a manual increase or disk autoscaling), on every instance Render bills, and a Key Value invoice has no storage line, as on Render. The used-bytes reading stays on the Billing page and the usage API, shown at $0. **Status:** blocked (t001, t002, t003, t004, t006, t007, t008 done; t005's dev-4 half done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record a Postgres's storage capacity as its own hourly meter — **DONE** | 1h30m | — |
| [t002](done/t002.md) | Bill capacity instead of used bytes from the cutover; a Key Value's storage is never charged — **DONE** | 1h15m | t001 |
| [t003](done/t003.md) | The Billing page names both lines — **DONE** | 45m | t002 |
| [t004](done/t004.md) | ADR023, ADR030 and the ADR018 row state Render's basis — **DONE** | 25m | t002 |
| [t005](t005.md) | Live: on dev-4 and then production, a Postgres bills its size and a Key Value bills no storage | 40m | t003, t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 5h50m**.

## Decision (2026-11-27, `/loopx w4` worker, promoting w4/745)

The basis is picked by Render's behavior, as the worker rules require ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first). It is not a user decision:

- **Render (read 2026-11-27):** its [faq](https://render.com/docs/faq) ("What does Render bill for?") bills "Storage capacity for Render Postgres databases and persistent disks", "Prorated by the second". [pricing](https://render.com/pricing) lists Postgres "Expandable storage ($0.30/GB)" and gives Key Value plans an included storage amount with no storage rate. [postgresql-high-availability](https://render.com/docs/postgresql-high-availability): "Your standby instance always has the same compute plan and storage as your primary instance and is billed accordingly." [postgresql-read-replicas](https://render.com/docs/postgresql-read-replicas): "Read replicas always have the same compute plan and storage as their primary database and are billed accordingly."
- **bex adopts:** a new meter records each Postgres's capacity, and it is billed at the existing rate ($0.21/GB-month, Render's $0.30 × 0.70) on the existing Stripe meter `storage_gb_hours`. `storage_gb_seconds` keeps measuring used bytes for Postgres and Key Value. From the cutover it is a reading shown at $0 and is never exported. Free Postgres capacity stays at $0 (w4/427).
- **The variant the note left open:** keep billing used bytes and record the divergence in ADR018 with the user's sign-off. Render bills capacity, so that variant does not gate this milestone. If the user wants it, it is a separate item.

## Definition of done

- **A full paid hour:** a paid 10 GB Postgres (no HA, no replicas) with about 2 GB used, present the whole hour, writes a capacity row of 36,000 GB-seconds. That is 10 GB, not the 10.74 decimal GB of its `10Gi` PVC. It bills $2.10 over a 730-hour month. Its used-bytes row of about 7,200 GB-seconds is shown at $0 and sends Stripe nothing.
- **HA and replicas:** an HA standby and each read replica add their own capacity, as Render bills them.
- **Resizes and partial hours:** a manual increase or an autoscale bills from the moment it takes effect. A database created or deleted mid-hour is prorated, by the second as Render bills, or to within the meter's sampling step.
- **Free Postgres:** its capacity costs $0.
- **Key Value:** from the cutover no Key Value hour sends Stripe a storage event, and the estimate has no storage charge for one. The Blueprint and plan-review estimates price no Key Value storage (`pricing/monthly.go:221`, `StorageMonthlyUSDPerGB`).
- **Before the cutover:** hours keep the basis they were invoiced on. Nothing already sent to Stripe is re-priced.
- **Usage reads:** `GET /v1/usage`, GraphQL `usage` and MCP `get_usage` return both kinds with the same totals on all three.
- **Billing page:** each Postgres shows a billed "Storage" line (capacity) and a "Storage used" line at $0. A Key Value shows only the "Storage used" line.
- **Compaction:** a compacted month keeps both readings and its billed total.
- **Docs:** [ADR023](../../../../docs/ADR023-usage-metering.md, [ADR030](../../../../docs/ADR030-pricing.md and the Usage metering row in [docs/ADR018/bex-ahead-of-render.md](../../../../docs/ADR018/bex-ahead-of-render.md state the capacity basis. The row no longer calls it "Documented drift".
- **Live:** on dev-4 and then production (`bex-canary`, the QA user's workspace, with a disposable Postgres and Key Value), the usage read shows the capacity row at plan GB × seconds and no billed Key Value storage.

## Source + Goal linkage

- **Source:** promoted from [w4/745](../../done/745.md) on 2026-11-27 by `/loopx w4`. That note came from w4/m688 t002, a full read of Render's faq on 2026-11-27.
- **Evidence (`origin/main` `3b55d72024`):**
  - `queryStorageGBSeconds` (`lego/backend/internal/usage/service.go:1420`) averages `kubelet_volume_stats_used_bytes` for both datastore kinds.
  - `Sheet.rateFor` (`lego/backend/internal/pricing/pricing.go:829`) prices Postgres and Key Value alike at `storage`.
  - `stripeMeterEvent` (`lego/backend/internal/billing/stripe.go:574`) exports both as `storage_gb_hours`.
  - `MonthlyEstimate` (`pricing/monthly.go:221`) adds a storage line to a Key Value.
  - The operator provisions `<storageGB>Gi` (`lego/operator/internal/controller/database_controller.go:528`), and on Hetzner the physical volume has a 10 GiB floor (`lego/backend/internal/metrics/datastore.go:301`).
  - ADR018 lists the used-bytes basis as unsigned "Documented drift".
- **Goal linkage:** Render parity (billing and invoicing), [ADR008](../../../../docs/ADR008-vision.md#render-parity-first.
- **Expected outcome:** a Postgres invoice matches what Render would charge for the same database, a Key Value invoice has no storage line, and the ledgers say so.
- **Why now:** every paid Postgres is billed for less than Render charges, and every paid Key Value for more, in every workspace today. The longer the used-bytes basis runs, the more invoices differ from Render's.
- **Render parity:** included. The change alters what REST, GraphQL, MCP and the Billing page report and bill.

## Parked (2026-11-28)

The code shipped in `9535759cbe`: the capacity meter, the cutover at 2026-12-01T00:00:00Z, the Stripe export, the Billing page, the estimates and the ADRs. dev-4 measured a full hour of a 10 GB Postgres at 36,000 GB-seconds (t005). What is left clears by itself:

- production's pin must carry `9535759cbe`;
- the first billed hour, 2026-12-01T00:00Z, must close.

Then a worker runs t005's production check in `bex-canary` and closes out with t009. No user action is needed.
