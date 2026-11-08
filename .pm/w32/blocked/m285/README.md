# w32 · m285 — Sandbox filesystem snapshots, as on Render (runtime kind refused pending sign-off)

**Worker:** worker32 **Goal:** users capture a filesystem snapshot of a running sandbox and start new sandboxes from it by id or name, with Render's lifecycle (status, 30-day expiry, delete), while `--kind runtime` answers a named refusal recorded as a divergence awaiting the user's sign-off **Status:** blocked — waits on w2/m278 (production walk + user decision); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: filesystem snapshots on the rootfs commit; runtime kind refused pending sign-off | 1h | w2/m278 |
| t002 | Capture a filesystem snapshot from a running sandbox | 2.5h | t001 |
| t003 | Create a sandbox from a snapshot by id or name | 2h | t002 |
| t004 | List, get and delete snapshots; expire and purge them | 1.5h | t002 |
| t005 | Render parity | 15m | t003, t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- On production, `bex sandboxes snapshots create <id> --kind filesystem --name base --wait` reaches `available` without terminating the sandbox, and `bex sandboxes create --from base` starts a sandbox containing the captured files.
- `snapshots list|get|delete` work on REST, GraphQL and MCP; delete while creating and expired/foreign snapshots answer named errors; expired snapshots are purged with their images.
- `--kind runtime` (Render's default) answers a named 400, and ADR018 records it as a divergence awaiting the user's sign-off.
- The checklist snapshot rows are regraded from `[-]`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w32, 2026-11-07, item 5. Render docs render.com/docs/sandboxes-snapshots; bex refuses snapshots by name (`SANDBOX_SNAPSHOTS_UNSUPPORTED`), and the checklist `[-]` rests only on 'non-goal at this pin', not a user decision.
- **Goal linkage:** Pillar 5 and [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first): snapshot restore is how Render users fork a prepared sandbox.
- **Expected outcome:** Render users' snapshot-and-restore workflows run on bex for the filesystem kind, and the runtime-kind gap is explicit and awaiting a decision instead of a blanket refusal.
- **Why now:** w2/m278 builds the same rootfs-commit path for suspend, so snapshots reuse fresh, verified mechanism. **Depends on w2/m278 closing** (start t001 only after it is in `w2/done/`).
- **Render parity:** Included: new REST/GraphQL/MCP operations on Render's paths. Dashboard N/A.
- **Open decision:** Runtime snapshots (memory and processes) are Render's default; bex's substrate captures the filesystem only. The milestone refuses `runtime` by name and records the divergence; settle it with w2/m278's pending 'what a suspended sandbox keeps' sign-off.

## Blocked — 2026-11-08 (`/loopx w32`)

- **Gate:** w2/m278 is itself parked in `w2/blocked/` on (1) a user sign-off on memory/process loss at suspension (or funding checkpointing) and (2) its production walk once `9cb1c257a` deploys. This milestone reuses that exact rootfs-commit/restore path, which is not yet proven on production, and decision (1) decides whether Render's default `--kind runtime` can be served at all.
- **Release condition:** w2/m278 is in `w2/done/` (or its decision record says the rootfs commit is verified on production and records the memory/process answer). Then restore this milestone with `/unblock w32` and start t001.
- **Why not proceed on the parity-sufficient option now:** filesystem snapshots alone would not depend on the decision, but they would be built on an unverified production mechanism; the upstream walk is the cheaper, sanctioned order.
