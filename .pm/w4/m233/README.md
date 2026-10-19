# w4 · m233 — Stop Key Value allowlist saves from silently restarting (and wiping) the store

**Worker:** worker4 **Goal:** changing who can reach a Key Value from outside never restarts it unannounced. A save that must restart says so before it happens and, for a Persistence Off store, asks for the data-loss confirmation. **Status:** in progress (t001–t006 done 2026-10-19; t007 live closeout waits on the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Trace every `spec.public` writer and the Postgres sibling; pick the no-restart design — **DONE** | 30m | — |
| [t002](done/t002.md) | Operator: keep the TLS listener once issued so toggling external access does not roll the pod — **DONE** | 60m | w4/m233/t001 |
| [t003](done/t003.md) | Announce and confirm the saves that still restart; correct the Networking copy — **DONE** | 40m | w4/m233/t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 15m | w4/m233/t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 10m | w4/m233/t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 45m | w4/m233/t004 |
| [t007](t007.md) | Closeout | 20m | w4/m233/t005, w4/m233/t006 |

Seven tasks, 220m (3h40m). One **major** finding. This milestone schedules the work only.

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
