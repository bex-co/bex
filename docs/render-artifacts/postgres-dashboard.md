# Render Postgres dashboard: sidebar, routes and pages

Read 2026-11-25 for w3/m641 from Render's public dashboard bundle, without sign-in. Not walked signed in: the bundle is the evidence, as it was for w3/m555 and w3/m556.

**Evidence.** `https://dashboard.render.com/` loads `assets/index-BPMVVQol.js` (sha256 `4eb3557f6970…`), which holds the Postgres layout, its route table and most of its pages. Its imports `AsyncComponents-iAj2RoG_.js` (`40cc59e5dee5…`), `DatabaseMetrics-BTF8-Jmy.js` (`7b9afc7cb5a3…`) and `VersionUpgrade-DGn9cPAO.js` (`e108aafc2e6d…`) hold the Metrics and version-upgrade pages. The hashes change with every Render deploy, so find the layout by its title map, the frozen object holding `"/recovery":"Recovery"`, as [metrics-page.md § Status-code label](metrics-page.md#status-code-label-w3080-2026-11-20) describes the method.

## Title map

`"" Info, /info Info, /metrics Metrics, /logs Logs, /plan Plan, /recovery Recovery, /sharing Sharing, /apps Apps`; `/upgrade`, `/high-availability-upgrade` and `/version-upgrade` map to `null`, so they are sub-pages with no sidebar title.

## Sidebar

The layout hands the shared resource shell `project: "datastores"` and this sidebar:

| Group | Entries, in order | Gate |
| --- | --- | --- |
| top level | **Info** (`/d/<id>/info`, also active on the bare `/d/<id>`) | — |
| Monitor | **Logs**, **Metrics** | Logs with `viewDatabaseLogs`, Metrics with `viewDatabaseMetrics`; the group is empty without both |
| Manage | **Plan** (also active on `/high-availability-upgrade`), **Apps**, Sharing (badged Legacy), **Recovery** | Plan, Apps and Recovery on a primary only; Sharing behind the `share-postgres` and `grandfather-owners-into-sharing` flags |

- Before the database loads, the sidebar already draws Info, Logs and Metrics (`initialSideNav`).
- A read replica gets Info and Monitor only, and its Info is also active on `/plan`.
- The back link reads **Dashboard** (`/`), or **Environment** when the database sits in a project environment.

## Routes

`/d/:id` renders the layout, with these children:

| Path | Page |
| --- | --- |
| `*`, `info` | Info (the bare URL shows Info with the URL unchanged, and so does any unknown sub-path) |
| `apps` | Apps |
| `recovery` | Recovery |
| `metrics`, `metrics/:period` | Metrics |
| `sharing` | Sharing (legacy) |
| `logs` | Logs |
| `plan` | Plan (a replica gets its own plan page) |
| `upgrade` | replaces itself with `/plan`, search and hash kept |
| `high-availability-upgrade` | a primary replaces it with `/plan`, search kept; a replica gets the HA upgrade page |
| `version-upgrade` | the "Postgres Version Upgrades" page |

The layout's document title is `<name> ・ Database` on every page. Its header carries View docs and Connect, and its plan badge links `/plan` (a replica without update permission: `info#postgres-instance`). Its banners are the suspension notice, scheduled maintenance and a Stripe connection.

## Pages

Each page's cards, in Render's order:

- **Info** (`eI`): the clone, expiry and storage-almost-full banners, the title "Info", then **General** (Name, Created, Status, PostgreSQL Version, Region, **Read Replicas** with Add Read Replica, or a replica's Primary link, Datadog API key), a replica's compute plan (replicas only), **Connections** (Hostname, Port, Database, Username, Password, Internal and External Database URL), **Credentials** (primary), **Networking** (the IP allow list, primary), a log stream override, **PostgreSQL Configuration Overrides**, and the delete card, which also holds **Restart Database** and **Suspend Database**.
- **Plan** (`vve`, primary): the storage-almost-full banner, **Compute** (`#compute`: the plan cards, Edit, then "Review your compute and storage selections" and Update compute), **Storage** (`#storage`), **Storage Autoscaling**, **High Availability** (`#high-availability`: the switch and "Manually trigger a failover"). No title heading.
- **Recovery** (`Vve`): the title "Recovery", **Point-in-Time Recovery**, the storage-almost-full banner, **Export**. A replica reads "Backups are only taken for primary databases."
- **Apps**: the title "Apps", pgAdmin and PgHero ([postgres-admin-apps.md](postgres-admin-apps.md)).
- **Logs** (`wve`): the full-screen log viewer.
- **Metrics** (`DatabaseMetricsWithPermission`): the time range and event filters, the Instance, Network and Database metric groups, then **table and index sizes**, **processes** and **top queries** (those three need `viewDatabaseConnectionInfo`).

**The two open points, settled.** Read Replicas render on Info, as a field of the General card. High Availability's controls render on Plan, as its last card.

## Page map (bex, w3/m641)

Each card of bex's former single Postgres page goes to the page whose Render counterpart holds it:

| bex card (old anchor) | Page | Render counterpart |
| --- | --- | --- |
| Details (`#metadata`) | Info | General |
| Connections (`#connection`) | Info | Connections |
| Access control (`#access-control`): credentials, IP allow list, pooler | Info | Credentials, Networking |
| SQL console (`#sql-console`) | Info | none; it sits before the danger zone |
| Danger zone (`#danger-zone`): delete, restart, suspend | Info | the delete card |
| Metrics (`#metrics`) | Metrics | the metric groups |
| Insights (`#insights`): sizes, processes, top queries, table scans, parameters | Metrics | sizes, processes, top queries |
| Plan (`#plan`) with its Storage row | Plan | Compute, Storage |
| Disk autoscaling (was in the Metrics card's disk header) | Plan, a row under Storage | Storage Autoscaling |
| High availability (`#high-availability`) with its read replicas | Plan | High Availability (Render's Read Replicas are on Info) |
| Apps (`#apps`) | Apps | Apps |
| Recovery (`#recovery`) | Recovery | Point-in-Time Recovery, Export |

## What bex does (w3/m641)

- A Postgres database's pages swap the workspace nav for this sidebar: the back link (Dashboard), the name, Info / Monitor: Logs, Metrics / Manage: Plan, Apps, Recovery. A Viewer or a Billing member (denied `can_view_logs`) gets no Logs, a Billing member (denied `can_view_activity`) no Metrics, and an emptied group goes.
- `/databases/<id>` and `/databases/<id>/info` show Info; `/logs`, `/metrics`, `/plan`, `/apps` and `/recovery` show their pages, each with its sidebar label as its heading and a document title led by it ("Recovery ・ orders ・ Database ・ bex Dashboard"). Render's `/d/<id>/<page>` reaches them through the `/d/` alias, query and hash kept.
- `/upgrade` lands on `/plan`, `/high-availability-upgrade` on `/plan#high-availability`, `/metrics/<period>` on `/metrics`, and `/version-upgrade`, `/sharing` and any other sub-path on Info.
- The old single page's links land on their page: `?tab=logs` on `/logs` with its range and filter keys, and each anchor of a card that left Info on that card's page, the hash kept.

## Still apart

- Render shows Read Replicas on Info; bex keeps them in the High Availability card on Plan ([w3/116](../../.pm/w3/116.md)).
- Render's Logs and Metrics pages hide the resource header ([w3/117](../../.pm/w3/117.md)). A signed-in, read-only walk on 2026-11-25 confirmed it: a Postgres database's Info page draws the header, and its Logs page (at 1440 and 390) and Metrics page (at 1440) do not. [w3/106](../../.pm/w3/done/106.md) did the same for a service's.
- A Key Value instance keeps its single page ([w3/115](../../.pm/w3/done/115.md), promoted to w3/m649).
- Render's document title has no page name; bex's names the page first, as its service tabs do.
- Render's Sharing page (legacy, flagged) and its log stream override (a [`DO_NOT_DO.md`](../../.pm/DO_NOT_DO.md) non-goal) have no bex counterpart.

## Almost-full warning (w3/m656)

Read 2026-11-25 from Render's public dashboard bundle, without sign-in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`) holds the rule and the banner, `database-BZ7Tvd7O.js` (`a0a69cc0775a…`) the query, and three of its imports the helpers: `urls-ubVUii_6.js` (`830179d15f15…`) the plan URL, `IntercomHandler-CwplQFZP.js` (`3ad80ad016f2…`) the support link and `plans-wgP2-MCL.js` (`852fd70ad525…`) the custom-plan test. Find the rule by `storageUsedPercent` beside `1073741824`.

- **Data.** The `database(id)` query asks for `storageUsedPercent`, `storageUsed` and `storageTotal` beside `...databaseFields`, whose fragment carries `diskSizeGiB` and `isMaxPlan`. The bundle reads all three through `Number()`, so their wire types are not visible; the byte counts do not fit GraphQL's 32-bit `Int` past 2 GiB.
- **Rule.** `total = Number(storageTotal)` and `percent = Number(storageUsedPercent)`. When both are non-zero, it warns if `percent > 80` and `(1 − percent / 100) × total / 1073741824 < 5`, that is more than 80% used and less than 5 GiB left. A missing or zero number never warns.
- **Copy.** "Your database is almost full. This may lead to errors reading from or writing to your database and can prevent automated backups. Consider clearing some space in your database or ‹ending›." The ending is one of three:

  | When | Ending | Link |
  | --- | --- | --- |
  | `isMaxPlan` | "contact support to update your custom compute plan" on a custom plan (its name matches `/(custom\|dedicated\|w\.)/i`), else "contact support to create a custom compute plan" | "contact support" opens Render's chat (`mailto:support@render.com`) |
  | `diskSizeGiB` is set (flexible storage) | "increase its storage" | `/d/<id>/plan#storage` |
  | otherwise | "upgrade to a compute plan with more storage" | `/d/<id>/plan` |

- **Look and placement.** An inline warning. Info draws it after the clone and expiry banners, above the title "Info"; a primary's Plan draws it first, above Compute; Recovery draws it under Point-in-Time Recovery, above Export.
- **Beside it.** Recovery's Create export is disabled when `Number(storageUsed) > 1099511627776`: "Exports are disabled for databases exceeding 1TB disk used."

### What bex does (w3/m656)

- GraphQL's `Database` carries the three fields: `storageTotal` and `storageUsed` as decimal strings, `storageUsedPercent` as a Float. REST and MCP do not carry them, since Render's public `postgres` objects (`diskSizeGB`, `diskAutoscalingEnabled`) and its MCP have no usage field.
- `storageTotal` is the database's logical size, `diskSizeGB` × 2^30 bytes, which is the capacity its Metrics page draws (w4/m91). `storageUsed` is the fullest instance volume's newest `kubelet_volume_stats_used_bytes` sample in the last ten minutes, from the series the Metrics page's disk graph reads: the primary's, the HA standby's and each read replica's volume, since each holds a full copy. `storageUsedPercent` is used over total.
- Writes fail only once the physical volume fills. Below 10 GiB the physical volume (Hetzner's floor) is larger than the logical size, so a small database can keep writing past its line and its percent can pass 100. bex warns at Render's line anyway, since Render's `storageTotal` is the provisioned size too.
- `storageUsed` and `storageUsedPercent` are null without a recent sample, after a failed read or one slower than 3 seconds, and for a member without `can_view_activity` (a Billing member, who cannot open Metrics either). `storageTotal` is always there. A query that selects none of the three makes no usage read.
- The dashboard reads the three in a query of their own (`DatabaseStorage`), so a database page never waits on the usage read. The warning appears once they answer and Render's rule holds.
- The endings: a plan whose storage is fixed (Free) gets "upgrade to a compute plan with more storage", linking `/databases/<id>/plan`. Storage already at its plan's maximum, which no plan grows further, gets "contact support to create a custom compute plan" as plain text. Any other database gets "increase its storage", linking `/databases/<id>/plan#storage`, the Plan page's Storage row. Render's `/d/<id>/plan#storage` lands there through the `/d/` alias, and so does `/databases/<id>#storage`.
- Placement: above Info's and Plan's headings, and on Recovery under its heading, above its one card. bex's Recovery card holds both point-in-time recovery and exports, so the warning cannot sit between them.

### Still apart

- Render's "update your custom compute plan": bex has no custom plans.
- Render's "contact support" opens its chat; bex's is text, as bex has no support channel to link.
- Render disables Create export above 1 TB used; bex does not ([w3/130](../../.pm/w3/130.md)).

## Plan review (w3/m674)

Read 2026-11-26 from Render's public dashboard bundle, without sign-in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`) holds the review component and the three pages that open it, `PlanPickerAddOns__Repricing-qgrymlMa.js` (`c8f0ec934566…`) the add-on price ratios, and `PlanSpec-DBr3UJW1.js` (`c0767418c043…`) the price format. Find the component by "Review your compute and storage selections." and its arithmetic by `billedInstances`.

- **One component** (`BN` in that build), a large modal: the title "Apply changes?", the lead, the line items, a rule, "Total" and its price, then an inline warning when the page passes one and an inline error when the change failed, and the footer's Cancel and the change's button. Each line has a small caps label, its figures (a plan's CPU and RAM, plus its connection limit where it has one), an Edit icon beside the last figure when the line has one, and its monthly price. Cancel and the close button return to editing with the choice kept (`resumeEditing`). Services' and cron jobs' compute changes open it too.
- **Compute change** (the Plan page's Compute section, Save): lead "Review your compute and storage selections."; lines **Compute plan** (the new plan, Edit "Edit compute plan" back to the picker), **High availability** when it is on ("Enabled", Edit "Edit high availability", which resets the change and scrolls to `#high-availability`), **Read replicas** when there are any ("1 replica", "2 replicas") and **Storage** unless the new plan is Free ("25 GB", or "25 GB (x3 instances)" when more than one instance bills); button **Update compute**; warning "Your database[ and its replicas] will be unavailable for a few seconds|minutes while updating.", seconds when high availability is on.
- **Storage change** (the Storage section, Save): lead "Review your storage selection."; the same lines priced at the new size, Compute plan without Edit, Storage with Edit "Edit storage" back to the field; button **Update storage**; no warning.
- **Arithmetic** (`WP`): billed instances = 1 + (HA ? 1 : 0) + replicas. The HA line is ⌊plan price × the HA ratio⌋, 1 in that chunk; replicas are count × plan price; storage is GB × the storage ratio ($0.30 per GB) × billed instances; the total is their sum. Prices print as `$<amount> / month`, with two decimals only when the amount has cents.

### What bex does (w3/m674)

- `ReviewChangesDialog` (`dashboard/src/common/components/review-changes-dialog.tsx`) draws Render's component on the dashboard's `ConfirmDialog`. A Postgres plan change, a Postgres storage change and a Key Value plan change open it in place of the old confirm and the old direct Save. A failed change shows its error in the dialog and keeps it open.
- Every price comes from bex-api's price sheet through the plan catalogue: `monthlyUsd` for Compute plan, and `storageMonthlyUsdPerGB`, added to GraphQL's `DatabaseInstanceType` for this, for Storage ($0.21, ADR030; "0.00" for Free's included storage). An HA standby is priced like the primary because it meters like one (ADR030), and each read replica's instance and volume meter like the primary's (ADR009), which is Render's arithmetic at bex's prices. `dashboard/src/features/databases/lib/plan-review.ts` computes the lines in whole cents, so they add up to the total they show.
- A compute change's Storage line is the size the change leaves: the larger of the database's storage and the new plan's floor (`EffectiveStorageGB`), so moving from a 5 GB plan to a 10 GB one prices 10 GB. Like every other bex storage estimate, it prices the provisioned size while bex meters used bytes (ADR030).
- Render's copy, buttons, warnings and Edit targets, in all eight languages. Cancel on the storage review returns to the size field with the size kept, as Render's does.

### Still apart

- Render's prices print as `$55 / month`; bex's as `$55.00/month`, as its plan cards do.
- Render shows the plan picker read-only until Edit; bex's picker is always open, with its own Cancel and Save.
- Services' and cron jobs' compute changes still confirm without the review ([w3/132](../../.pm/w3/132.md)).
