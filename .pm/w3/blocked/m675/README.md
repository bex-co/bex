# w3 · m675 — A Postgres's and a Key Value's header is Render's: the plan badge, View docs, and Connect with the internal URL, the external URL and the CLI command

**Worker:** worker3 **Goal:** on every page of a Postgres database or a Key Value instance, the header carries what Render's does: the name, status and plan badge (which opens the plan), a View docs link and a Connect menu. Connect holds the internal URL, the external URL or Render's note that external traffic is not allowed, and the PSQL or Redis CLI command. The header's kebab menu goes, as Render has none; its actions stay on Info. **Status:** blocked (t001–t004 and t006–t008 done; t005's local walk holds on dev-3; its production walk waits for a pin carrying the milestone, then t009)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's datastore header and its Connect menu — **DONE** | 30m | — |
| [t002](done/t002.md) | One Connect menu for both datastores — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | The header: plan badge, View docs and Connect; the kebab goes — **DONE** | 1h | t001 |
| [t004](done/t004.md) | The header's pending frame equals its ready state at 1440 and 390 — **DONE** | 30m | t002, t003 |
| [t005](t005.md) | Verify in a real browser, locally and on production | 45m | t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 40m | t006 |
| [t009](t009.md) | Closeout | 10m | t007, t008 |

Total: **9 tasks, about 5h40m**. **Starts after w3/m649 closes:** the Key Value header lives in the layout m649 is building (`keyvalue.$keyValueId.tsx`), and the Key Value plan badge opens m649's Info `#compute` card.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-25 without sign-in (`index-D5V4ZgbE.js`, sha256 `8d15889ace2d…`); find the header by its strings, since chunk names change with every deploy. Not walked signed in.
- **Actions:** a datastore's header has two actions, a View docs link and a Connect dropdown, and no overflow menu. Suspend, resume, restart and delete live on the Info page.
- **Connect:**
  - the internal URL;
  - the external URL, or, when no IP may connect, "External traffic not allowed. Add IP addresses in the Networking section." with a link to Networking;
  - "PSQL Command" for Postgres, "Redis CLI Command" for Key Value.
  - The values load when the menu opens. A member without permission to see connection info gets a disabled button with the reason.
- **Plan badge:** a Postgres's opens `<path>/plan`; a Key Value's opens `<path>/info#compute`.

## bex today

Verified 2026-11-25 on main `c196617a8b`, with w3/m649's work in progress.

- `dashboard/src/routes/databases.$databaseId.tsx` draws the name, `DatabaseStatusBadge` and a kebab (`DatabaseRowActions`: Suspend or Resume, Restart, Delete). Info's danger zone (`database-danger-actions.tsx`, `#danger-zone`) offers the same actions. The list rows keep their own menu.
- The Key Value layout (m649) draws the name and `KeyValueStatusBadge`, and no actions.
- The connection strings sit in Info's Connection card only:
  - Postgres: `connection-info-panel.tsx` with Password, Internal, External, PSQL and CA;
  - Key Value: `keyvalue.connInternal`, `connExternal` with its pending, failed and unavailable states, and `connCli`.
  - Revealing them needs `can_view_sensitive` (`useCapabilities().canViewSensitive`, `reasonKey`, `PermissionTooltip`).
- A service's header already has a Connect popover to copy from: `ServiceConnectButton` in `features/services/components/service-detail-header.tsx`, with `ConnectCodeRow`.
- bex's docs have https://bex.co/docs/platform/postgres and https://bex.co/docs/platform/key-value (both 200 on 2026-11-25).

## Definition of done

1. At 1440 and 390, on every Postgres page (Info, Logs, Metrics, Plan, Apps, Recovery) and every Key Value page (Info, Logs, Metrics), the header shows the name, the status, the plan badge, View docs and Connect.
2. The plan badge opens Plan for Postgres and Info's `#compute` card for Key Value. View docs opens bex's page for that datastore.
3. Connect copies each value. With external access closed, it shows Render's note, linking to the Networking/access card on Info. A Viewer sees the button disabled, with the reason.
4. The header has no kebab. Suspend, resume, restart and delete still work from Info's danger zone.
5. The header's pending frame matches its ready state at 1440 and 390.
6. Walked locally and, once a production pin carries the milestone, on production as the QA user in `bex-canary` (`tea-daif693dqjvc73e7as3g`), with a Postgres and a Key Value instance created for the walk (disposable).

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w3` (2026-11-25), item 1. Before filing, it was checked against Render's bundle and bex's code (the headers above, the capability gate, the docs pages). The conflict check found none with w1/m645–m648. It found two with w3/m649 (the Key Value layout and the `#compute` card), so this milestone starts after m649.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first). A datastore's header is the first thing on each of its pages.
- **Expected outcome:** someone who knows Render copies a connection string from whichever page of the datastore they are on, with one click, and finds the plan from the header, as on Render.
- **Why now:** w3/m641 and w3/m649 split both datastores into several pages. The connection strings now sit on one card of one page, and the header that opens every page has a menu Render does not have.
- **Render parity task:** included (t006). Dashboard only. The connection-info reads and their permission gate do not change, and t006 confirms that REST, GraphQL and MCP are untouched.

## Local walk (2026-11-26, t005 steps 1 and 3)

On dev-3 (main `6ba91ca5b7` plus the milestone), as an owner, with a Free Postgres and a Free Key Value created for the walk (disposable, deleted after it), at 1440 and 390:

- Every page (Postgres Info, Logs, Metrics, Plan, Apps, Recovery; Key Value Info, Logs, Metrics) shows the name, the status, the plan badge ("Free", opening `/databases/<id>/plan` and `/keyvalue/<id>/info#compute`), View docs (bex's docs, new tab) and Connect. No kebab.
- Connect read the internal URL on open and copied it. A public database's External tab showed the external URL and `bex psql <id>`, which copied. With the database's IP rules emptied, and on the private Key Value, External showed "External traffic not allowed. Add IP addresses in the Access control section." (Networking for Key Value). Its link landed on `info#access-control` (`info#networking`) and closed the menu.
- Info's danger zone still offers Delete, Restart and Suspend. Suspend and Resume both ran from it, and the header read "Suspended" then "Available", its plan badge, View docs and Connect unchanged, as Render's header is on a suspended datastore.
- With the database's and the instance's reads held in flight, the pending header measured the same as the ready one: 65 px tall at 1440 and 105 px at 390, View docs and Connect at the same boxes (90×32 and 98×32), the actions wrapping under the name at 390.
- The Viewer step was not walked locally: dev-3's bex-api runs with fail-open authorization (`BEX_ALLOW_INSECURE_AUTHZ`), so every member there is admin-equivalent. The disabled Connect, with the reason, is covered by `datastore-connect-menu.test.tsx` and both datastores' pages tests. Walk it on production with a member who cannot reveal secrets, if `bex-canary` has one.

## Decisions and deviations

- Render's Connect has two tabs, Internal and External, as the 2026-07-09 capture saw. The CLI row is on External and shows Render's CLI command (`render psql <id>`), not the psql connection string. bex shows `bex psql <id>` and `bex kv-cli <id>`, the CLI's Render-compatible commands, so t002's "the PSQL or Redis CLI command" is the CLI command.
- The "external traffic not allowed" note names the bex card that holds the IP rules: Postgres's is "Access control" (`#access-control`), Key Value's "Networking". Render's says "Networking section" for both.
- The values are read each time Connect opens, with the Info cards' own fetch, and dropped when it closes, as the Info card's Reveal does. Render reads once per page.
- The status badge stays beside the name, as the definition of done asks, though Render's header has none. That, Render's eyebrow, its Service ID row and its Free-plan "Upgrade your instance" are filed as [w3/133](../../133.md).
