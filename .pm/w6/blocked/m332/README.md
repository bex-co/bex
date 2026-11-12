# w6 · m332 — Render's mid-size plans have a node: cx53 capacity, and the four plan IDs that fit it

**Worker:** worker6 **Goal:** Pro Plus and Pro Max services run, and Render's `2c-8g`, `2c-16g`, `8c-16g` and `12c-24g` plans are accepted and run on a cx53 node, as on Render. **Status:** blocked — t001 done: on 2026-11-12, cx53 is `resource_unavailable` in fsn1, nbg1 and hel1, so t002 has no location to move to. t003 step 1 (Render's prices) is done; its rung steps wait with t004. The gate is cx53 stock in eu-central, or the user revisiting the 2026-11-02 keep-cx53 decision (cpx62 is in stock in fsn1 at €152.99/month).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Probe cx53 stock in fsn1, nbg1 and hel1](done/t001.md) — **DONE** | 30m | — |
| t002 | [Place `bex-tenant-xl` where cx53 is in stock](t002.md) | 1.5h | t001 |
| t003 | [Render's prices for `2c-8g`, `2c-16g`, `8c-16g`, `12c-24g`, and their rungs](t003.md) | 1h | t001 |
| t004 | [Accept the four IDs on every surface and run one on production](t004.md) | 1.5h | t002, t003 |
| t005 | [Render parity](t005.md) | 30m | t004 |
| t006 | [Simplify](t006.md) | 15m | t005 |
| t007 | [Test coverage](t007.md) | 30m | t006 |
| t008 | [Closeout](t008.md) | 10m | t007 |

## Definition of done

- On production a `pro_plus` and a `pro_max` service reach live on a `bex-tenant-xl` node (the gate of `w2/m250` t003), and the pool returns to 0.
- `2c-8g`, `2c-16g`, `8c-16g` and `12c-24g` are accepted on REST, GraphQL, MCP, Blueprints and the dashboard picker at Render's price less 30%, and an `8c-16g` service runs live on production.
- `4c-32g`, `8c-64g`, `12c-48g` and `12c-96g` stay in `w6/blocked/114` behind the dedicated-vCPU limit.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-11-11 (w4/575, the long-standing blockers and the two limits, for Render parity), item 1; `w2/blocked/m250` (cx53 `resource_unavailable` in fsn1 on 2026-11-02 and 2026-11-03, so Pro Plus and Pro Max stay Pending); `w6/blocked/114`; ADR018 "Change instance plan / type".
- **Goal linkage:** ADR008 Render parity first; Render sells these plans, and a plan bex accepts must run.
- **Expected outcome:** the paid mid-size plans run again, and four more of Render's plan IDs work without the dedicated-vCPU limit.
- **Why now:** an accepted paid plan that never gets a node is worse than a missing one, and a stock probe costs almost nothing. The user's 2026-11-02 decision (keep cx53, no cpx62) stands: the move stays on cx53, in another location of the same network zone.
- **Render parity task:** included (plans, create and the picker are tenant-facing).
