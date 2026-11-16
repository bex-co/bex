# w32 · m285 — Sandbox filesystem snapshots, as on Render (the runtime kind follows w2/m322)

**Worker:** worker32 **Goal:** users capture a filesystem snapshot of a running sandbox and start new sandboxes from it by id or name, with Render's lifecycle (status, 30-day expiry, delete), while `--kind runtime` answers a named 400 until w2/m322's memory checkpoint exists, recorded as a temporary difference (user decision D1=A, 2026-11-11) **Status:** todo — unblocked 2026-11-16 (w2/m278 closed, D1=A); t001 next

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: filesystem snapshots on the rootfs commit; the runtime kind is refused by name until w2/m322 | 1h | w2/m278 |
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
- `--kind runtime` (Render's default, so an omitted kind too) answers a named 400, and ADR018 records it as a temporary difference that w2/m322 closes (user decision D1=A, 2026-11-11), not as a signed-off divergence.
- The checklist snapshot rows are regraded from `[-]`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w32, 2026-11-07, item 5. Render docs render.com/docs/sandboxes-snapshots; bex refuses snapshots by name (`SANDBOX_SNAPSHOTS_UNSUPPORTED`), and the checklist `[-]` rests only on 'non-goal at this pin', not a user decision.
- **Goal linkage:** Pillar 5 and [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first): snapshot restore is how Render users fork a prepared sandbox.
- **Expected outcome:** Render users' snapshot-and-restore workflows run on bex for the filesystem kind, and the runtime-kind gap is explicit and tied to w2/m322 instead of a blanket refusal.
- **Why now:** w2/m278 builds the same rootfs-commit path for suspend, so snapshots reuse fresh, verified mechanism. **Depends on w2/m278 closing** (start t001 only after it is in `w2/done/`).
- **Render parity:** Included: new REST/GraphQL/MCP operations on Render's paths. Dashboard N/A.
- **Decision (settled 2026-11-11, w2/m278 D1=A):** Runtime snapshots (memory and processes) are Render's default; bex's substrate captures the filesystem only. The user funded memory checkpointing as w2/m322, so the runtime kind is planned, not declined. Until that mechanism exists the milestone refuses `runtime` by name and records a temporary difference.

## Blocked — 2026-11-08 (`/loopx w32`)

- **Gate:** w2/m278 is itself parked in `w2/blocked/` on (1) a user sign-off on memory/process loss at suspension (or funding checkpointing) and (2) its production walk once `9cb1c257a` deploys. This milestone reuses that exact rootfs-commit/restore path, which is not yet proven on production, and decision (1) decides whether Render's default `--kind runtime` can be served at all.
- **Release condition:** w2/m278 is in `w2/done/` (or its decision record says the rootfs commit is verified on production and records the memory/process answer). Then restore this milestone with `/unblock w32` and start t001.
- **Why not proceed on the parity-sufficient option now:** filesystem snapshots alone would not depend on the decision, but they would be built on an unverified production mechanism; the upstream walk is the cheaper, sanctioned order.

## Unblock — 2026-11-16 (`/loopx w32`, verified evidence)

- **Affected:** w32/m285, t001–t008.
- **Source:** verified evidence, plus the user decision already recorded on w2/m278 (D1=A, 2026-11-11). No new user decision was taken here.
- **Gate 1, the rootfs commit proven on production: cleared.** w2/m278 is in `w2/done/m278/` (closed 2026-11-11, `63d40d997`). Its t011 walk on production: `sbx-dbq1ddip92ls73cjjjg0` went `suspending` → `suspended`, and the exec that resumed it read a file written before the suspension.
- **Gate 2, the memory/process answer: cleared.** D1=A funds memory and process checkpointing as w2/m322, and w2/m278's record says of this milestone: "With D1=A, runtime snapshots are planned (m322), not refused." ADR018's `idleTimeoutSeconds` row already carries the suspend side as a temporary difference.
- **Checked, and not a gate here:** w2/m322 is itself parked (its t001 found no upstream route to checkpoint a gVisor pod; the user chooses between a patched runsc shim and waiting). That parks the runtime kind only. Serving it was never in this milestone (t001 "Out of scope"), and ADR104 D5 already rules that "filesystem snapshots and the rest of m285 ship now".
- **Chosen behavior and scope:** the filesystem kind ships now. `kind: runtime`, and an omitted `kind` (Render's default is `runtime`), answer a named 400 that points at `--kind filesystem`. bex never downgrades a runtime request to a filesystem capture, because the caller would get a snapshot without the memory and processes Render's default promises. ADR018 records a temporary difference that w2/m322 closes; the wording "divergence awaiting the user's sign-off" is retired, since the user answered by funding the checkpoint.
  - **Render basis** (pinned OpenAPI `create-sandbox-snapshot`, and render.com/docs/sandboxes-snapshots read 2026-11-16): `kind` "Defaults to `runtime`"; a runtime snapshot "Captures the writable filesystem, along with memory and CPU state", a filesystem snapshot "the writable filesystem only"; "Creating a snapshot does not terminate the sandbox".
- **Details that moved since the filing (2026-11-07); the tasks are followed by intent:**
  - Sandbox groups shipped (w22/m311), and Render scopes snapshots to the source sandbox's group. The routes are `POST /v1/sandboxes/{id}/snapshots` and `/v1/sandbox-groups/{sandboxGroupId}/snapshots[/{snapshotId}]`; "scope to the workspace" in t001 is superseded.
  - The pin is CLI v2.36 with `ttlSeconds` beside the deprecated `expiresAt`.
  - ADR104 D4 adds requirements t001 must settle before t002 ships: nothing a caller chooses reaches the privileged commit Job, a capture rate limit, a time and size bound, no privileged Pod left behind, and the w22/m313 storage meter (`StorageKindSnapshot`) instead of a second meter. ADR104 D11 adds the per-group and per-workspace storage bounds.
- **Mechanism found (t001 records it):** the pinned OpenSandbox server 0.2.2 serves `POST /sandboxes/{id}/snapshots`. It creates a standalone `SandboxSnapshot`; the same commit Job as a pause freezes the container, commits and pushes its root filesystem, then resumes it, and the pod is kept. The server's tenant role already holds `sandboxsnapshots` in `<ws>-sandbox` namespaces. This path has not run on production yet; the commit Job itself has (w3/m42, w2/m278).
- **Remaining gates:** none for t001–t007. t008's live check needs a production deploy that carries the work; that gate clears by itself.
- **Next (worker):** t001, then t002–t004 on one service path for REST, GraphQL and MCP, the closing tasks, and the production walk in the definition of done. The runtime kind is filed separately, behind w2/m322.

