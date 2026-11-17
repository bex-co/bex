# Where Render shows a Postgres plan's connection limit (w6/m459 t001)

Read on 2026-11-17 from Render's docs and their screenshots; no live Render session was available. The Key Value counterpart is in [key-value.md](key-value.md#where-render-shows-the-connection-limit-w6m459-t001-2026-11-17).

Render does have a connection limit per Postgres plan: render.com/docs/compute-plans § Render Postgres plans groups the plans under 100, 200, 400 and 500 connections ("The compute plan only determines your database instance's RAM, CPU, and connection limit"), and render.com/docs/postgresql-creating-connecting § Connection limits gives the same table by RAM. bex carries the figure as `tiers.PostgresTier.MaxConnections` (w6/134).

| Render surface | Shows the limit? | Source |
| --- | --- | --- |
| `/new/database` plan cards | No. Each card shows the plan name, RAM, price and CPU only ("Basic-256mb 256 MB (RAM) $6 / month 0.1 CPU") | render.com/docs/postgresql-creating-connecting step 6, "select a compute plan for your database. This determines its available CPU and RAM", screenshot `pg-instance-types.png` ("Selecting a Postgres compute plan") |
| Info page | Not shown in any of Render's screenshots or text. The docs point to the Info page for connection details only (`postgresql-connection-details.png`) | render.com/docs/postgresql-creating-connecting |
| Metrics page, Active Connections chart | Not captured. The chart exists ("The number of open connections to your database from all connecting clients. This graph is also available for Render Key Value instances", render.com/docs/service-metrics § Database activity), and the Key Value one reads "Limit 250", but no screenshot of the Postgres one shows its header | render.com/docs/service-metrics; `postgresql-metrics.png` shows only the Memory chart ("Plan limit 4 GB") |
| REST, MCP, Blueprint | No plan catalog | render.com/docs/compute-plans |

So bex's Postgres create and plan pickers show no connection limit, and `DatabaseInstanceType` carries none (w6/m459 t002, t004). Whether Render's Postgres Active Connections chart reads a limit needs a live capture.
