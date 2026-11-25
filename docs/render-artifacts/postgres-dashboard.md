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
- **Recovery** (`Vve`): the title "Recovery", **Point-in-Time Recovery**, **Export**. A replica reads "Backups are only taken for primary databases."
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
- Render's Logs and Metrics pages hide the resource header ([w3/117](../../.pm/w3/117.md); [w3/106](../../.pm/w3/106.md) covers a service's).
- A Key Value instance keeps its single page ([w3/115](../../.pm/w3/115.md)).
- Render's document title has no page name; bex's names the page first, as its service tabs do.
- Render's Sharing page (legacy, flagged) and its log stream override (a [`DO_NOT_DO.md`](../../.pm/DO_NOT_DO.md) non-goal) have no bex counterpart.
