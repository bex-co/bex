# w32 · m285 — Sandbox filesystem snapshots, as on Render (the runtime kind follows w2/m322)

**Worker:** worker32 **Goal:** users capture a filesystem snapshot of a running sandbox and start new sandboxes from it by id or name, with Render's lifecycle (status, 30-day expiry, delete), while `--kind runtime` answers a named 400 until w2/m322's memory checkpoint exists, recorded as a temporary difference (user decision D1=A, 2026-11-11) **Status:** blocked — t001–t007 done; t008's production walk passed every check but one, which needs production cluster read; it also found t009 (§ Blocked — 2026-11-17, after the walk)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: filesystem snapshots on the rootfs commit; the runtime kind is refused by name until w2/m322 — **DONE** | 1h | w2/m278 |
| [t002](done/t002.md) | Capture a filesystem snapshot from a running sandbox — **DONE** | 2.5h | t001 |
| [t003](done/t003.md) | Create a sandbox from a snapshot by id or name — **DONE** | 2h | t002 |
| [t004](done/t004.md) | List, get and delete snapshots; expire and purge them — **DONE** | 1.5h | t002 |
| [t005](done/t005.md) | Render parity — **DONE** | 15m | t003, t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005 |
| [t009](t009.md) | Captures failed in the minutes after a platform deploy: find why and make them survive it — **BLOCKED** (production logs) | 1h30m | t007 |
| [t008](t008.md) | Closeout (live walk on production) — **BLOCKED** (registry check, t009) | 40m | t006, t007, t009 |

## Definition of done

- On production, `bex sandboxes snapshots create <id> --kind filesystem --name base --wait` reaches `available` without terminating the sandbox, and `bex sandboxes create --from base` starts a sandbox containing the captured files.
- `snapshots list|get|delete` work on REST, GraphQL and MCP; delete while creating and expired/foreign snapshots answer named errors; expired snapshots are purged with their images.
- `--kind runtime` (Render's default, so an omitted kind too) answers a named 400, and ADR018 records it as a temporary difference that w2/m322 closes (user decision D1=A, 2026-11-11), not as a signed-off divergence.
- The checklist snapshot rows are regraded from `[-]`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w32, 2026-11-07, item 5. Render docs render.com/docs/sandboxes-snapshots; bex refuses snapshots by name (`SANDBOX_SNAPSHOTS_UNSUPPORTED`), and the checklist `[-]` rests only on 'non-goal at this pin', not a user decision.
- **Goal linkage:** Pillar 5 and [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first): snapshot restore is how Render users fork a prepared sandbox.
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

## Blocked (2026-11-17)

**Release gate (clears by itself; no decision needed).** t001–t007 shipped in `8968f97e3` (bex-api: REST, GraphQL and MCP snapshot verbs, migration 0282; ADR104 D4, ADR018, the checklist). Production runs the bex-api image of the newest `chore(deploy): pin platform images to <sha>` commit on `main`. That pin was `58ef086ab21d` when this milestone parked, and it does not contain `8968f97e3`; deploy.yml batches, so the next pin lands at an hourly run once the current one is six hours old (from about 10:40 UTC on 2026-11-17). Production still answered `404 SANDBOX_SNAPSHOTS_UNSUPPORTED` to `bex sandboxes snapshots list` at 05:20 UTC. Check:

```bash
git fetch origin main
pin=$(git log origin/main --grep='^chore(deploy): pin platform images to' -1 --format=%s | sed -E 's/.*images to ([0-9a-f]+).*/\1/')
git merge-base --is-ancestor 8968f97e3 "$pin" && echo "pinned: walk the definition of done"
```

Then t008 walks the definition of done in `bex-canary` as the QA user with the released `bex` (v0.7.0, Render CLI v2.36), deleting every fixture afterwards:

- `sandboxes create --wait`, write `~/marker.txt` with `exec`; `snapshots create <id> --kind runtime` and with no `--kind` answer `400 SANDBOX_SNAPSHOT_RUNTIME_UNSUPPORTED`;
- `snapshots create <id> --kind filesystem --name base-<nonce> --wait` reaches `available` and `sandboxes get <id>` still reads `running`;
- `sandboxes create --from base-<nonce> --wait` and `--snapshot-id <snp>` both start sandboxes whose `cat ~/marker.txt` prints the marker, and the snapshot remains;
- `snapshots list|get|delete` on REST (the CLI), GraphQL (`sandboxSnapshots`, `sandboxSnapshot`, `deleteSandboxSnapshot`) and MCP (`list_`, `get_`, `delete_sandbox_snapshot`); a delete during a capture is `409 snapshot_creating`, a deleted snapshot reads 404 and deletes again with 204, and a restore from it is `404 snapshot_not_found`;
- a capture with `ttlSeconds: 60` (REST) reads 404 after it expires and its `snapshots/<ws>-sandbox/snapshots/<snp-id>` repository loses its tag at the next sweep.

Then regrade the checklist rows (`sandboxes snapshots …`, `snapshots.*`) and ADR018's two 🚧 rows with the walk's evidence, tick t002's and t003's live criteria, and close.

## Unblock — 2026-11-17 (`/loopx w32`, verified evidence)

- **Affected:** w32/m285 t008. No user decision was needed or taken: the § Blocked gate was a release gate that clears by itself.
- **Gate cleared, the pin contains the work.** The newest `chore(deploy): pin platform images to` commit on `main` is `ff18968a8` (`b767149494fa`, written 2026-11-17 06:16 UTC), and `git merge-base --is-ancestor 8968f97e3 b767149494fa` succeeds.
- **Production runs that pin.** deploy.yml run 44027592896 (head `b767149494fa`, `workflow_dispatch`) concluded `success`. Its `pin image digests in GitOps (write-back)` step wrote operator/bex-api digest `sha256:e3d3a360…`, and its `roll bex onto the freshly-pushed image` step reported `deployment "bex-api" successfully rolled out` at 06:30:21 UTC (then `bex-controller-manager`, `bex-ssh-gateway` and `bex-drain-guard`). The live walk below is the observable check: production stops answering `404 SANDBOX_SNAPSHOTS_UNSUPPORTED`.
  - Direct cluster reads were not available on this host: `scripts/fetch-app-kubeconfig.sh` found the two control-plane nodes, but there is no `~/.ssh/bex` here and `~/.ssh/id_ed25519` is refused (`Permission denied (publickey)`). The walk needs only the public API, so this is not a gate.
- **Remaining gates:** none. **Next (worker):** t008 walks the definition of done in `bex-canary` as § Blocked lists, then closes the milestone.

## Production walk — 2026-11-17 (t008, `/loopx w32`)

The walk ran in `bex-canary` (`tea-daif693dqjvc73e7as3g`, group `sbg-dbpugr7bq3ic73c0acmg`) as the QA user, from 06:50 to 07:17 UTC. The CLI was the released `bex` v0.7.0 (Render CLI v2.36.0; `checksums.txt` matched and `cosign verify-blob` said `Verified OK`). GraphQL and MCP calls used the same login's token. Production ran pin `b767149494fa` until 06:53 and `21762a1df1fb` after it (run 44031784852 rolled bex-api 06:53:01–06:53:44). Both pins contain `8968f97e3`. Before the walk, `snapshots list` already answered `[]`, not `404 SANDBOX_SNAPSHOTS_UNSUPPORTED`.

**Definition of done, item by item:**

- **Capture and restore: holds.**
  - `snapshots create sbx-dbtvi9teg69s73cfbvfg --kind filesystem --name base-e848 --wait` returned `available` in 16 s (`snp-dbtvie5eg69s73cfbvi0`, 6,809,004 B). Afterwards the sandbox still read `running`, and `exec` still answered.
  - `sandboxes create --from base-e848 --wait` and `--snapshot-id snp-dbtvie5eg69s73cfbvi0 --wait` each started a running sandbox in under 5 s. In both, `cat ~/marker.txt` printed the marker written before the capture.
  - The snapshot remained after both restores, and a restore moved its `expiresAt` to 30 days after that use. One restored sandbox also resumed and still had the marker after the snapshot was deleted.
- **The surfaces and their errors: hold.**
  - **List and get on all three surfaces:** CLI/REST `snapshots list|get`, GraphQL `sandboxSnapshots`/`sandboxSnapshot`, and MCP `list_`/`get_sandbox_snapshot` each returned the snapshot. The REST fields equal the pinned spec's 13.
  - **Delete during a capture:** the CLI, REST, GraphQL and MCP each answered `409 snapshot_creating`.
  - **A deleted snapshot:** deleting it through GraphQL left it `404 snapshot_not_found` on REST, CLI, GraphQL and MCP. A second REST delete answered 204, and so did a second MCP delete. Restoring it by id or by name answered `404 snapshot_not_found`.
    - A second delete through the pinned CLI answers 404, because that CLI reads the snapshot before deleting it (`cmd/sandboxsnapshotsdelete.go:67`). That is the client's own behavior, not a bex difference.
  - **An expired snapshot:** a REST capture with `ttlSeconds: 60` (`snp-dbtvjoomlqgs73cq4d1g`) read `available` with `expiresAt` 60 s after the request. After that it read `404 snapshot_not_found`, left the list, refused restores by id and name with 404, and a delete answered 204. Two more captures with 30 s lifetimes read 404 within a second of expiring.
  - **Another group or workspace:** a second group in the same workspace could not read, restore (by id or name) or delete the snapshot: each answered `404 snapshot_not_found`. Listing the group with another workspace's `ownerId` answered `404 SANDBOX_GROUP_NOT_FOUND`.
  - **A failed snapshot:** restoring it answered `409 snapshot_not_available`.
  - **A suspended sandbox:** capturing it answered `409 sandbox_not_running`.
  - **A second capture of a sandbox** while one is in flight answered `429 SANDBOX_SNAPSHOT_LIMIT`.
- **The runtime kind: holds.** `--kind runtime` and no `--kind` both answered `400 SANDBOX_SNAPSHOT_RUNTIME_UNSUPPORTED` with the `--kind filesystem` hint. ADR018 records it as a temporary difference that w2/m322 closes.
- **The SDKs:** `scripts/render-sdk-compat.sh` now walks the snapshot methods instead of expecting refusals. The unmodified Render SDK v1.8.0 passed every step on production: Python 41 of 41, TypeScript 35 of 35. Each covered both runtime refusals, a `filesystem` create with wait (the sandbox kept running), get and list, restores by id and by name (the marker came back), delete during a capture (the SDKs' NotReady error), and a deleted snapshot reading, restoring and listing as gone.
  - The first Python run lost one unrelated step: an upload to `/tmp` answered 403 `forbidden` once. It passed on the rerun and in two separate probes.
- **The checklist rows: regraded** (`docs/cli-compatibility-checklist.md`), with this walk as the evidence. They are `[~]`, not `[x]`, because of t009.
- **Expired snapshots purged with their images: not verified.** Each expired or deleted snapshot left every read, every list and every restore. The registry side cannot be seen without production cluster read: is the `snapshot` tag gone from `snapshots/tea-daif693dqjvc73e7as3g-sandbox/snapshots/<snp-id>`?
- **New finding, now t009:** three captures ended `failed` ("the sandbox's filesystem could not be committed"). All three came between 06:55 and 07:00, after the 06:53 deploy: 3 of the 9 captures in that window failed. None of the 21 captures before or after it failed (15 in the walk, 6 in the SDK runs).

Every fixture is gone: the snapshots, the three sandboxes, the second group, the CLI login (`bex logout`) and both Kratos sessions (`--logout`).

## Blocked — 2026-11-17, after the walk (`/loopx w32`)

- **Gate: production cluster read access from the worker's host.** Two checks need it, and nothing else in the milestone does.
  - **What the user does:** put the production control-plane SSH key at `~/.ssh/bex` on this host (`BEX_SSH_PRIVATE_KEY_FILE` in `.env.example`). Then `scripts/fetch-app-kubeconfig.sh` works here.
  - **What was tried:** on 2026-11-17 it found both control-plane nodes, but this host has no `~/.ssh/bex`, and `~/.ssh/id_ed25519` is refused with `Permission denied (publickey)`. This checkout's `.env` has no `HCLOUD_TOKEN`; the walk took it from another checkout's `.env`.
  - **Alternative:** the user authorizes a read-only probe on the `production-observe` environment, as w22/m321 had.
  - Then `/unblock w32`.
- **t008, the registry purge:** list the tags of `snapshots/tea-daif693dqjvc73e7as3g-sandbox/snapshots/<snp-id>` in the in-cluster Zot. Use the operator-minted read user (`BEX_SANDBOX_SNAPSHOT_REGISTRY_SECRET`) or the `_catalog` route of `scripts/measure-sandbox-storage.sh`.
  - Snapshots to check: the expired ones `snp-dbtvjoomlqgs73cq4d1g`, `snp-dbtvmn3vsrns73ebrku0` and `snp-dbtvnkbvsrns73ebrli0`, and the deleted ones `snp-dbtvie5eg69s73cfbvi0` and `snp-dbtvjdivk30s73cqtadg`.
  - Expected: no `snapshot` tag remains. Also check that bex-api logged no `reap … (the sweep retries)` errors for them.
  - Then change ADR018's capture row from 🚧 to ✅.
- **t009, the failed captures:** their reasons are only in bex-api's logs, so the same access is needed. A capture run across the next production deploy can reproduce the failures without that access, but it cannot explain them.
- **Not a gate:** everything else in the definition of done holds on production (§ Production walk — 2026-11-17).
