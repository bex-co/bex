# Where Render shows a Postgres plan's connection limit (w6/m459 t001, w6/138)

Read on 2026-11-17 from Render's docs and their screenshots; no live Render session was available. The Metrics row was read on 2026-11-18 from Render's production dashboard code (w6/138). The Key Value counterpart is in [key-value.md](key-value.md#where-render-shows-the-connection-limit-w6m459-t001-2026-11-17).

Render does have a connection limit per Postgres plan: render.com/docs/compute-plans § Render Postgres plans groups the plans under 100, 200, 400 and 500 connections ("The compute plan only determines your database instance's RAM, CPU, and connection limit"), and render.com/docs/postgresql-creating-connecting § Connection limits gives the same table by RAM. bex carries the figure as `tiers.PostgresTier.MaxConnections` (w6/134).

| Render surface | Shows the limit? | Source |
| --- | --- | --- |
| `/new/database` plan cards | No. Each card shows the plan name, RAM, price and CPU only ("Basic-256mb 256 MB (RAM) $6 / month 0.1 CPU") | render.com/docs/postgresql-creating-connecting step 6, "select a compute plan for your database. This determines its available CPU and RAM", screenshot `pg-instance-types.png` ("Selecting a Postgres compute plan") |
| Info page | Not shown in any of Render's screenshots or text. The docs point to the Info page for connection details only (`postgresql-connection-details.png`) | render.com/docs/postgresql-creating-connecting |
| Metrics page, Active Connections chart | Yes: "Limit 100" beside the title on every plan under 8 GB, as the Key Value chart reads "Limit 250" | Render's dashboard code, read 2026-11-18 (below); render.com/docs/service-metrics § Database activity names the chart; `postgresql-metrics.png` shows only the Memory chart ("Plan limit 4 GB") |
| REST, MCP, Blueprint | No plan catalog | render.com/docs/compute-plans |

So bex's Postgres create and plan pickers show no connection limit (w6/m459 t002, t004).

## The Active Connections chart (w6/138, 2026-11-18)

No screenshot of Render's Postgres chart shows its header, and the repository holds no Render dashboard login. Render's dashboard code is public, though: `https://dashboard.render.com/login` loads `assets/index-KWK7ZvXU.js` without a session, and its route table loads the database Metrics page from `assets/DatabaseMetrics-BbWQy-0B.js` (through `AsyncComponents-DBIK2D2p.js`). That chunk, read on 2026-11-18, renders the Postgres chart as the Key Value one in `index-KWK7ZvXU.js`, which drew "Limit 250" in `kv-active-connections.png`:

- **Title:** "Active Connections", with the tooltip "Shows the number of active client connections. Useful for tracking connection spikes or avoiding connection limits."
- **Subtitle:** a "Limit" link to the database's Plan page, then the limit as a whole number in the locale's grouping (`toLocaleString()`).
- **The figure:** the latest point of the database's `CONNECTIONS_LIMIT` metric, or the plan's `maxConnections` when that series is empty. Render's OpenTelemetry stream carries the same figure as `render.postgres.connection.limit`, "the maximum number of concurrent connections supported by a particular Render Postgres instance (as determined by its compute plan)" (render.com/docs/metrics-streams-reference, added 2025-04-17 beside `render.keyvalue.connection.limit`).
- **The plot:** the limit series is passed to the chart as its limit, as the Key Value chart's is. Render's Key Value screenshot draws its 0 to 25 axis under "Limit 250", so the limit does not stretch the axis.

So a `basic-1gb` database reads "Limit 100". bex's chart reads the same since w6/138, from GraphQL `databaseInstanceTypes.maxConnections` (`tiers.PostgresTier.MaxConnections`). Like bex's Key Value chart, it reads the current plan's figure and draws no limit line. Its "Limit" is plain text, as on bex's CPU, Memory and Key Value charts, where Render's links to the Plan page.
