# w6 · m457 — External Postgres and Key Value connections are admitted up to the datastore's own limit, not 64

**Worker:** worker6 **Goal:** an allowlisted client outside the cluster can open as many connections to a database or Key Value as its plan allows, as on Render, and the one past the limit gets the server's own refusal instead of a dropped connection. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Decide the external admission rule and record it](t001.md) | 45m | — |
| t002 | [Each external route carries its datastore's connection limit](t002.md) | 1h | t001, w6/m456/t001, w6/134 |
| t003 | [pg-sni-proxy admits up to the route's limit](t003.md) | 1h | t002 |
| t004 | [kv-sni-proxy admits up to the store's limit](t004.md) | 45m | t002 |
| t005 | [Production check from an allowlisted client](t005.md) | 45m | t003, t004, w6/m456/t006 |
| t006 | [ADR018 row and the proxies' env inventory](t006.md) | 20m | t005 |
| t007 | [Render parity](t007.md) | 30m | t006 |
| t008 | [Simplify](t008.md) | 15m | t007 |
| t009 | [Test coverage](t009.md) | 30m | t007 |
| t010 | [Closeout](t010.md) | 10m | t008, t009 |

## Definition of done

- On production, from an allowlisted client outside the cluster: 97 concurrent connections to a `basic-256mb` probe database succeed (Render's 100 less PostgreSQL's 3 `superuser_reserved_connections`) and the next receives `FATAL: sorry, too many clients already`, not an EOF or SSL error; a `starter` probe Key Value accepts 250 external TLS connections and the 251st receives `ERR max number of clients reached`.
- No proxy refuses a route below its datastore's limit, and the per-workspace cap is no lower than the sum of its public datastores' limits, or off.
- ADR009, ADR021 and ADR018 record the rule, and `lego/operator/AGENTS.md` documents the proxies' env vars as they now behave.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-17, item 3, filed by the user's `/pm them to w6`. Render: external Key Value connections "always require authentication" and are reachable only from the instance's IP allowlist ([Key Value](https://render.com/docs/key-value)); Render documents each plan's connection limit and no separate cap for external connections ([compute plans](https://render.com/docs/compute-plans)).
- **bex today:** `pg-sni-proxy` and `kv-sni-proxy` default to 64 connections per resource and 256 per workspace (`lego/operator/cmd/pg-sni-proxy/main.go:80`, `lego/operator/cmd/kv-sni-proxy/main.go:66`; `deploy/gitops` overrides neither). So every external Postgres client stops at 64 where Render's stops at 100, and every paid Key Value at 64 where Render's stops at 250 or more. The shed closes the connection (`sniproxy.Limiter.AcquireRoute`), which a client reports as a network error. The caps came with `91a24194f` (2026-08-16, "remediate repository security review") and are recorded in no ADR.
- **Goal linkage:** ADR008 § Render parity first, which amends an earlier hardening choice that conflicts with parity instead of keeping it.
- **Expected outcome:** serverless and edge apps, BI tools and migrations that reach a datastore from outside get Render's connection count, and a client at the limit reads why.
- **Why now:** it affects every external Postgres client on every plan, and the failure looks like a network fault. w6/m456 and w6/134 supply the per-plan figures it needs.
- **Render parity task:** included. External connection behavior is tenant-facing; connection info on REST, GraphQL, MCP and the dashboard does not change.
