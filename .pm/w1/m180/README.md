# w1 · m180 — Serve datastore CPU and memory metrics on Render's paths

**Worker:** worker1 **Goal:** Postgres and Key Value CPU, memory and their limits are served on Render's metric paths, bex GraphQL, MCP and the dashboard. **Status:** todo

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | --------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design the datastore CPU and memory queries and confirm the series exist in production | 45m | —          |
| t002 | Serve datastore cpu, memory and their limits on REST                              | 1h  | t001       |
| t003 | GraphQL and MCP serve the same datastore metrics                                  | 40m | t002       |
| t004 | Dashboard datastore Metrics panel shows CPU and Memory with limit lines           | 1h  | t003       |
| t005 | Update ADR018 and verify live                                                     | 40m | t004       |
| t006 | Render parity                                                                     | 20m | t005       |
| t007 | Simplify                                                                          | 15m | t006       |
| t008 | Test coverage                                                                     | 40m | t006       |
| t009 | Closeout                                                                          | 10m | t008       |

## Definition of done

- `GET /v1/metrics/{cpu,memory,cpu-limit,memory-limit}?resource=<dpg-…|red-…>` returns series for live datastores (Key Value `memory` per t001's decision); GraphQL and MCP return the same; other datastore pairs keep the coded 400; service metrics are unchanged.
- The dashboard Postgres and Key Value Metrics pages chart CPU and memory with limit lines in en/zh, with matching loading skeletons.
- `docs/ADR018-render-parity.md:224` no longer marks datastore CPU/memory ◐, and t005's production check shows non-empty series on REST, MCP and the dashboard.

## Evidence (verified 2026-10-17 at `2c95fd35b`)

- `lego/backend/internal/metrics/datastore.go:381-382` refuses every datastore pair but Key Value memory; `render_contract_test.go:212-219` pins that 400; `mcp.go:264` lists no cpu/memory; `datastore-metrics-panel.tsx:80-96` has no CPU/memory chart.
- `docs/ADR018-render-parity.md:224`: "Datastore CPU and Postgres memory are not served (◐)" — no owner. w8/061 (done) only replaced the bare 404 with the coded 400 and said to "serve the series if bex has it" (`.pm/w8/done/061.md:38`).
- Render's pinned OpenAPI `resourceIdParam`: "IDs of resources (server, cron job, postgres, or redis)".

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-10-17 item 3 (ADR018 unowned-gap sweep), verified the same day and approved by the user for filing.
- **Goal linkage:** Render parity (ADR018) for the managed data products (ADR009 Postgres, ADR021 Key Value).
- **Expected outcome:** users can see whether a slow database is CPU- or memory-bound and needs a bigger plan; Render CLI/MCP metric reads on `dpg-`/`red-` ids stop failing.
- **Why now:** Prometheus has recovered (w4/m144) and the Key Value exporter starvation fix is awaiting verification (w4/181), so the series are trustworthy now; plan choices just changed (w1/122 made `starter` the default).
- **Render parity included:** REST, GraphQL, MCP and dashboard surfaces all change.
- **Related, not taken over:** `w4/blocked/181` (exporter CPU budget) — independent; its verification stays there.
