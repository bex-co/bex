# w6 · m456 — Key Value connections stop at the plan's limit, as on Render (free 50 … 50g 50,000)

**Worker:** worker6 **Goal:** every Key Value instance accepts exactly the connections its plan allows on Render, no fewer (a paid store is capped at 1,024 by its internal proxy today) and no more (a free store takes 1,024 where Render's takes 50), and the client past the limit receives Valkey's own `ERR max number of clients reached`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Render's connection limit on every Valkey plan, pinned in tiers](t001.md) | 45m | — |
| t002 | [Production read: which stores already exceed their new limit](t002.md) | 30m | t001 |
| t003 | [`valkey-server` starts with `--maxclients` from its plan](t003.md) | 1h | t001 |
| t004 | [The limit reaches running servers and plan upgrades with `CONFIG SET`](t004.md) | 45m | t003 |
| t005 | [kv-internal-proxy admits the plan's limit and lets Valkey answer the refusal](t005.md) | 1h | t003 |
| t006 | [Ship, verify on production, and record the limits in ADR021 and ADR018](t006.md) | 45m | t002, t004, t005 |
| t007 | [Render parity](t007.md) | 30m | t006 |
| t008 | [Simplify](t008.md) | 15m | t007 |
| t009 | [Test coverage](t009.md) | 30m | t007 |
| t010 | [Closeout](t010.md) | 10m | t008, t009 |

## Definition of done

- On production, a free Key Value accepts 50 concurrent client connections and the 51st receives `ERR max number of clients reached`; a `standard` store accepts 1,000; upgrading the free store to `starter` raises its limit to 250 without restarting `valkey-server`.
- A `pro` store's internal proxy admits its 5,000 (a test proves it no longer refuses at 1,024), and at every store's limit the refusal is Valkey's error on both the plain and the TLS listener, never a closed connection.
- Every existing store adopted its limit without a Valkey pod restart.
- A guard test pins every Valkey plan id and alias bex accepts to Render's figure; ADR021 and ADR018 record the limits; t002's stores over their new limit are reported.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-17, item 1, filed by the user's `/pm them to w6`. Render: [compute plans § Render Key Value plans](https://render.com/docs/compute-plans#render-key-value-plans), read 2026-11-17 (free 50, `256mb` 250, `1g` 1,000, `3g` 3,000, `5g` 5,000, `10g` 10,000, `20g` 20,000, `40g` 40,000, `50g` 50,000), and [Key Value](https://render.com/docs/key-value): "Specify a compute plan to determine the instance's available RAM and connection limit." Render's dashboard shows the limit on each plan card ([key-value.md](../../../docs/render-artifacts/key-value.md)).
- **bex today:** `valkeyArgs` (`lego/operator/internal/controller/keyvalue_controller.go`) sets no `--maxclients`, so Valkey's default 10,000 applies, and every tenant connection passes `kv-internal-proxy`, whose `MaxConnections` defaults to 1,024 per listener (`lego/operator/internal/kvinternalauth/proxy.go:50`) and sheds the excess by closing the connection. ADR018 has no row for connection limits.
- **Goal linkage:** ADR008 § Render parity first: every customer-facing behavior matches render.com in every workspace, and this divergence has no sign-off in ADR018.
- **Expected outcome:** a paid store serves the connections its plan sells (5,000 on `pro`, where it stops at 1,024 today), a free store stops at Render's 50, and a client at the limit reads why.
- **Why now:** w5/m266 (2026-11-05) put `3g` through `pro_max` on sale, and each is held to 1,024 connections, a fifth of what Render sells as `pro`. Free stores take 20 times Render's 50, a cost exposure that grows with every free signup. w6/m457 and w6/m459 read the table t001 adds.
- **Render parity task:** included. The limit is tenant-visible on every client connection, and w6/m459 shows it in the dashboard; REST, GraphQL and MCP fields do not change, and t007 confirms they say nothing that contradicts it.
