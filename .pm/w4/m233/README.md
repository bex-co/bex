# w4 · m233 — Stop Key Value allowlist saves from silently restarting (and wiping) the store

**Worker:** worker4 **Goal:** changing who can reach a Key Value from outside never restarts it unannounced. A save that must restart says so before it happens and, for a Persistence Off store, asks for the data-loss confirmation. **Status:** in progress (t001–t006 and t008 done 2026-10-19; t007 live closeout waits on the t008 deploy and on w4/362)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Trace every `spec.public` writer and the Postgres sibling; pick the no-restart design — **DONE** | 30m | — |
| [t002](done/t002.md) | Operator: keep the TLS listener once issued so toggling external access does not roll the pod — **DONE** | 60m | w4/m233/t001 |
| [t003](done/t003.md) | Announce and confirm the saves that still restart; correct the Networking copy — **DONE** | 40m | w4/m233/t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 15m | w4/m233/t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 10m | w4/m233/t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 45m | w4/m233/t004 |
| [t008](done/t008.md) | Report `available` through a save that does not restart the store — **DONE** | 40m | w4/m233/t006 |
| [t007](t007.md) | Closeout | 20m | w4/m233/t005, w4/m233/t006, w4/m233/t008 |

Eight tasks, 260m (4h20m). One **major** finding. This milestone schedules the work only.

## Definition of done

Replay [finding.md](finding.md) with the [recorded probes](evidence.json) on a fresh owned Free Key Value with Public access on and Persistence **Off**. Keep the connection URL and runner IP out of every transcript.

- **Toggle without restart.** With external access already enabled once, save an empty list, then save your IP again.
  - Status stays `available` throughout. It never reads `config_restart`, so this is the opposite of today's two restarts.
  - A key written before the first save still reads back after the second (`GET` → the value, `DBSIZE` ≥ 1).
  - External access is refused while the list is empty and allowed again within about 10 s of the second save.
- **First enable is announced.** On a store created without Public access, the first save of a rule shows, before saving, that it restarts the store.
  - With Persistence Off, Save requires the typed confirmation used by the persistence change.
  - Over REST, `PATCH /v1/key-value/{id}` with that change and no `confirm` is refused with a coded error, and succeeds with `confirm`. GraphQL and MCP behave the same.
- **Control.** Adding or removing a rule while external access stays on: no restart and no prompt (as today).
- **Copy.** The Networking card never claims "Internal access is unchanged" for a save that restarts the store. The same check is done for the Postgres card after t001's verdict.
- **Checks pass.** Operator `make test`, backend `go test ./...` (real-DB gate), dashboard `yarn lint` and `yarn test`.
- **Cleanup.** No fixture remains, and only that run's session is revoked.

## Source + Goal linkage

- **Source:** live functional `/qa-find-bugs w4`, pass 8be45a, 2026-10-19 UTC, `muse.env`, workspace `bex-canary`. See [finding.md](finding.md) and [evidence.json](evidence.json). Production was pinned to `94a76a923136`, and research is on main `fe734f586`.
- **Goal linkage:** [ADR021](../../../docs/ADR021-keyvalue-management.md) (Key Value: external SNI endpoint, IP allowlist, persistence modes) and [ADR008](../../../docs/ADR008-vision.md) truthful product state. Every other restarting setting on the same page already warns, and Persistence Off already requires a typed confirmation.
- **Expected outcome:**
  - Owners can open or close external access to a cache without dropping connections or losing data.
  - Where a restart cannot be avoided, they are told first.
- **Why now:**
  - The data-loss path takes two routine saves and no warning.
  - Persistence Off is the documented choice for caches, which are exactly the stores people expose and lock down while debugging.
- **Render parity included:** REST, GraphQL, MCP and dashboard behavior change. t004 compares Render's documented Key Value access-control behavior.
- **Unverified:**
  - The internal client interruption (inferred from the pod roll).
  - The Postgres card.
  - Whether environment allow-lists flip member stores (t001).

## Hosted probe — qa pass 0a57a8 (2026-10-19, pin `db7e50ced338`)

Live functional `/qa-find-bugs w4`, workspace `bex-canary` (Hobby: one Key Value at a time, so two fixtures in turn):

- `qa-20261019-0a57a8-kv1` (`red-dbasmd5212nc73a1i6v0`): created public with the runner's /32 and `persistenceMode:"off"`.
- `qa-20261019-0a57a8-kv2` (`red-dbasoll212nc73a1i730`): created private with `persistenceMode:"off"`.

The connection URL went through a loopback 0600 sink and the runner CIDR through a loopback endpoint. Neither appears below. External commands were `redis-cli --sni <host> -u <url>`; without `--sni` the proxy closes the TLS handshake. Both fixtures are deleted (`DELETE` 204, then `GET` 404), and only this run's session is revoked.

| DoD bullet | Result | Probe |
| --- | --- | --- |
| Toggle: status stays `available` | **FAIL (cosmetic)** → t008 | kv1 `PATCH {"ipAllowList":[]}` 07:49:20 answered `status=config_restart`; reads `config_restart` 07:49:21, then `available` 07:49:23. Re-add 07:50:06 answered `config_restart`, then `available` 07:50:09. kv2 (after its first enable): the same 1–2 s blips at 07:59:17 and 07:59:37. Nothing restarted, as the next two rows show. |
| Toggle: the key survives | PASS | kv1 `SET qa:0a57a8 survived` before → `GET` `survived`, `DBSIZE` 1 after both saves. `uptime_in_seconds` 143 at 07:50:40 (started before the SET). kv2: `uptime` 18 → 72 s across its off/on, and the key survived. |
| Toggle: refused while empty, back within ~10 s | PASS | PING every 2 s: PONG to 07:49:16, refused from 07:49:22 (2 s after the empty save), PONG again from 07:50:08 (2 s after the re-add). |
| First enable announced before saving | PASS | kv2 Networking card, before Save: "This store has never had external access, so the first save restarts it and connections drop for up to a minute." |
| Persistence Off: Save needs the typed confirmation | PASS | Dialog "Delete all data and enable external access?": "…Its persistence mode is Off, so the restart deletes all data. This cannot be undone. Type sudo clear key value qa-20261019-0a57a8-kv2…". The confirm button is disabled until typed. After confirming: `config_restart` 07:58:18 → `available` public 07:58:59, toast "Allowlist updated.", external PONG. |
| REST: refused without `confirm`, succeeds with it | **FAIL** → w4/362 | Without: `400 {"code":"KEY_VALUE_RESTART_CONFIRMATION_REQUIRED",…"retry with confirm=\"sudo clear key value qa-20261019-0a57a8-kv2\""}` (correct). With `?confirm=<phrase>`: `400 unsupported query parameter "confirm" for update-key-value`, because the OpenAPI query validator never allowlisted it. |
| GraphQL / MCP the same | GraphQL PASS; MCP not run | `setKeyValueIpAllowList(id, entries)` without `confirm` → `extensions.code KEY_VALUE_RESTART_CONFIRMATION_REQUIRED`, `extensions.confirm` = the phrase. The success path ran through the dashboard. |
| Control: add/remove a rule while public, no restart or prompt | PASS | kv1 dashboard: added `203.0.113.7/32` → no dialog, toast "Allowlist updated.", and the status read "Available" throughout at 250 ms sampling of the page. |
| Copy | PASS (Key Value) | kv1 (has had access): "…Internal access is unchanged." is now true. kv2 (never had access): the restart sentence replaces it. The Postgres card was not checked. |

Remaining before t007: t008 (status) and w4/362 (REST `confirm`), each deployed and replayed. MCP and the Postgres card are still unverified.
