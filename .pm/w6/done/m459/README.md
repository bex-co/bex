# w6 · m459 — Plan pickers and the Key Value instance card show each plan's connection limit

**Worker:** worker6 **Goal:** as on Render, a user choosing or reviewing a Key Value plan sees its connection limit beside its RAM, read from bex-api's catalog, and the Postgres pickers show `max_connections` if Render's do. **Status:** done (2026-11-17: on production the Key Value plan cards, Details card, plan picker and Metrics chart read each plan's connection limit; see Closeout)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Which Render surfaces show the connection limit](done/t001.md) — **DONE** | 30m | — |
| t002 | [GraphQL instance types carry the limit](done/t002.md) — **DONE** | 30m | t001, w6/m456/t001, w6/134 |
| t003 | [The Key Value create cards, instance card and plan picker show the limit](done/t003.md) — **DONE** | 1h | t002 |
| t004 | [The Postgres plan pickers show `max_connections`, if Render's do](done/t004.md) — **DONE** | 30m | t002 |
| t005 | [Every locale, both widths, skeletons unchanged](done/t005.md) — **DONE** | 30m | t003, t004 |
| t006 | [Render parity](done/t006.md) — **DONE** | 30m | t005 |
| t007 | [Simplify](done/t007.md) — **DONE** | 15m | t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 30m | t006 |
| t009 | [Closeout](done/t009.md) — **DONE** | 10m | t007, t008 |

## Definition of done

- The Key Value create page's plan cards, the Key Value detail page's instance card and its plan picker show each plan's connection limit ("50 connections" on free), read from `keyValueInstanceTypes`, matching w6/m456's table.
- The Postgres create and plan pickers show `max_connections` if t001 found Render's do, and t001's finding is recorded either way.
- Every locale carries the copy, and pending and ready states match at desktop and narrow-mobile widths.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-17, item 5, filed by the user's `/pm them to w6`. Render's Key Value create page shows "Free — $0/mo, 25 MB RAM, 50 connections", and its detail page's Key Value Instance card shows the plan's name, RAM and connection limit ([key-value.md](../../../../docs/render-artifacts/key-value.md), captured 2026-07-09).
- **bex today:** `KeyValueInstanceType` (`lego/backend/internal/keyvalue/graphql.go:74`) carries no limit, and the dashboard shows none.
- **Goal linkage:** ADR008 § Render parity first, with Render's dashboard as the UI reference.
- **Expected outcome:** users see the limit where they pick a plan, so a connection refused after w6/m456 is explainable.
- **Why now:** w6/m456 starts enforcing 50 and 250 connections; a limit the user cannot see before choosing reads as an outage.
- **Render parity task:** included. The plan catalog is a GraphQL-only bex extension because Render has no public catalog API; t006 confirms that stays true.

## Findings (t001, t004, t006; 2026-11-17)

- **Where Render shows the limit** ([key-value.md](../../../../docs/render-artifacts/key-value.md#where-render-shows-the-connection-limit-w6m459-t001-2026-11-17), [postgres-connection-limit.md](../../../../docs/render-artifacts/postgres-connection-limit.md), Render's docs and screenshots read 2026-11-17): every Key Value plan card ("50 Connections"), the Key Value Instance card, and "Limit 250" beside the Key Value Active Connections chart. Render's Postgres plan cards show name, RAM, price and CPU only, so the Postgres pickers stay as they are (t004) and `DatabaseInstanceType` gets no `maxConnections`.
- **Shipped:** GraphQL `KeyValueInstanceType.connectionLimit` (`Int!`, from `tiers.Valkey`); "N connections" on its own line on each Key Value plan card (create page, plan picker), a Connection limit row in the Details card, and on the Metrics tab Render's title "Active Connections" with "Limit N" beside it (a drift t006 fixed in place). The loading slots carry the extra line; the create page's in-page loading grid has one slot per plan instead of three. REST and MCP still carry no plan catalog, as Render has none. ADR021 § 9 records it.
- **Filed:** [w6/138](../../138.md), whether Render's Postgres Active Connections chart reads a limit (needs a live Render capture).

## BLOCKED — 2026-11-17: the production deploy carrying this milestone's commit

- **Done.** t001–t008 (above).
- **Gate (time, clears by itself).** A push or scheduled `deploy.yml` run deploys `main` once the production pin is 6 h old, and a dispatched run deploys at once.
- **How to check it cleared.** `git log origin/main --grep "pin platform images"` shows a pin whose producer contains this milestone's commit, and `api.bex.co`'s GraphQL answers `{ keyValueInstanceTypes { id connectionLimit } }` with `free` 50.
- **Then.** t009: sign in as the QA user (`bash scripts/qa-login.sh`) and check on dashboard.bex.co that `/keyvalue/new` reads "50 connections" on Free and "1,000 connections" on Standard, and that a Key Value's Details card, plan picker and Metrics tab read its plan's limit; then close.

## UNBLOCKED — 2026-11-17: the production deploy carrying `64200b09c` landed

- **Affected:** w6/m459 t009. **Source:** verified evidence, read by the `/loopx w6` worker at 22:23Z.
- **The pin.** `e36296ae8` ("pin platform images to e0a5ba20aac8", 18:33Z) is on `origin/main`, and `git merge-base --is-ancestor 64200b09c e0a5ba20aac8` holds.
- **The API.** Signed in as the QA user, `api.bex.co`'s GraphQL answers `{ keyValueInstanceTypes { id connectionLimit } }` with free 50, starter 250, standard 1000, 3g 3000, pro 5000, pro_plus 10000, pro_max 20000, pro_ultra 40000 and 50g 50000, w6/m456's table.
- **Remaining gates:** none. **Next:** t009's signed-in check on dashboard.bex.co, then close.

## Closeout — 2026-11-17 (t009)

- **Production check** (pin `e36296ae8`, signed in as the QA user; no probe created):
  - `/keyvalue/new`: every plan card reads its limit on its own line under the RAM, Free "50 connections", Starter "250 connections", Standard "1,000 connections" through 50 GB "50,000 connections", at 1280 px and at 390 px with no horizontal overflow.
  - `bex-canary`'s `qa-20261110-81b5-kv` (`red-dbpf5q5me2kc73f2l0og`, Starter): the Details card reads Instance type "Starter" and Connection limit "250"; the Instance type picker's cards each read their limit (Free "50 connections", disabled as a downgrade, Starter checked with "250 connections"); the Metrics tab's chart is titled "Active Connections" with "Limit 250" beside it. The plan was not changed.
  - The QA session was logged out (`qa-login.sh --logout`) and the dashboard then redirected to `/auth/login`.
- **Definition of done.** The create cards, the Details card and the plan picker show each plan's limit read from `keyValueInstanceTypes`, matching w6/m456's table; t001's Postgres finding is recorded (Render's Postgres plan cards show none, so bex's do not); t005 checked every locale and both widths, and production renders both widths.
