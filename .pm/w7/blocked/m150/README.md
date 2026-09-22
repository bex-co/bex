# w7 · m150 — Sandbox file copy through the pinned CLI

**Worker:** worker7 **Goal:** The distributed CLI can exchange files with an authorized sandbox using bounded, single-use transfers. **Status:** blocked (t001, t002, t004, t006–t008 done; t003 live upload and t005 live roundtrip remain gated; t009 closeout pending)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin the file-transfer contract and limits — **DONE** | 40m | — |
| t002 | Mint and redeem authorized file-connect tokens — **DONE** | 50m | w7/m150/t001 |
| t003 | Stream bounded uploads through the sandbox gateway | 55m | w7/m150/t002 |
| t004 | Stream bounded downloads and propagate failures — **DONE** | 55m | w7/m150/t002 |
| t005 | Verify the CLI round trip and update compatibility evidence | 40m | w7/m150/t003, w7/m150/t004 |
| t006 | Render parity — **DONE** | 30m | w7/m150/t001, w7/m150/t002, w7/m150/t003, w7/m150/t004, w7/m150/t005 |
| t007 | Simplify — **DONE** | 25m | w7/m150/t006 |
| t008 | Test coverage — **DONE** | 45m | w7/m150/t006 |
| t009 | Closeout | 15m | w7/m150/t007, w7/m150/t008 |

## Definition of done

- [x] Contract fixtures match the actual pinned client rather than help output alone.
- [x] Supported paths, archive/link behavior, limits and partial-transfer outcome are explicit.
- [x] Sandbox groups are recorded as a deliberate non-goal; App SSH restrictions stay unchanged.
- [x] Tokens bind caller, workspace, sandbox, operation and destination/source path.
- [x] Expired, replayed, foreign and wrong-operation tokens are refused across replicas.
- [x] OAuth and connect-token routes preserve their intended authentication boundaries.
- [ ] The real pinned CLI uploads supported fixtures successfully.
- [x] Path traversal, unsafe archive entries/links and configured size limits are enforced.
- [x] Cancellation stops transfer work and leaves only the documented partial-file outcome; bex-api gains no direct pods/exec privilege.
- [x] Downloaded supported fixtures are byte-identical to sandbox contents.
- [x] Missing paths, authorization loss and interrupted streams report truthful errors.
- [x] No whole-file buffering or unbounded background transfer survives disconnect.
- [ ] Upload then download returns byte-identical data through the distributed CLI.
- [x] Foreign owner, token replay, wrong path, oversize and interrupted transfer cases are evidenced.
- [ ] Compatibility grading cites the implementation and dated live result; disposable fixtures are removed.

## Source + Goal linkage

- **Source:** Approved w7 brainstorm item 3 via `$pm all for w7`; promoted `w7/046`, archived at `.pm/w7/done/046.md`; w7/m147 file-copy exclusion and `docs/cli-compatibility-checklist.md` copy gap.
- **Goal linkage:** ADR008 pillar 5 and ADR018 sandbox coverage: agents can provide inputs and retrieve outputs through the shipped CLI.
- **Expected outcome:** The pinned upstream CLI uploads and downloads supported files byte-for-byte; foreign access, replay, path escape and oversized transfer are refused.
- **Why now:** The distributed client exposes copy but its server routes return 404. Reuse the shipped connect-token and gateway seams from w7/m147 rather than writing or forking a CLI.
- **Render parity:** Included: tenant-facing changes; compare the applicable ADR018 coverage and all exposed surfaces.
- **Sizing:** 240m implementation; 355m including standing closing tasks, 9 tasks.

- **Approved scope decision:** sandbox file copy is in scope; sandbox groups are a deliberate non-goal. This narrowly extends sandbox transfer support and does not reopen App-instance SSH restrictions.

## Triage 2026-09-17 (`/loopx w7`) — Work, not started

Reached at the end of a drain that shipped w7/048, 052, 053, 054, 055, 056, 057 and parked m148/m149. Triaged rather than begun, because starting a 9-task feature with ~355m of work would have left half-work, which the drain forbids. **Not blocked** — nothing external gates the implementation half, and mislabelling it would hide available work.

Two things the next run should know before picking it up:

- **t001-t004 need no cluster.** t001 is reading the pinned upstream CLI's upload/download handshake and recording the contract; t002-t004 are token minting and bounded streaming in `lego/backend`. All of that is workable offline against `lego/cli/UPSTREAM_RENDER_CLI.md` and the pinned dependency.
- **t005 and several DoD lines need a live sandbox**, and therefore a working local cluster — "The real pinned CLI uploads supported fixtures successfully", "Upload then download returns byte-identical data through the distributed CLI", and the dated compatibility-grading result. The local CAPD cluster is currently rotted (see `w7/blocked/m148`: machines 20d old, backing containers gone), so that half will need the documented reprovision first:

  ```sh
  kubectl --context kind-bex-mgmt delete cluster bex --wait=false
  bash scripts/mock-cluster.sh
  ```

Sequencing suggestion: do t001 first regardless — the contract is the input to everything else, and pinning it against the real client rather than help output is explicitly what the first DoD line asks for.

## Drain verdict — 2026-09-21: blocked after implementation

Implemented the bounded REST + CLI file-copy path and completed every task that can be verified without a live sandbox. t002, t004 and t006–t008 are done; t003 retains only its live upload criterion, t005 retains the live roundtrip/dated evidence, and t009 cannot close before those hold. This is a partial compatibility grade, not completed live acceptance.

**Correction to t001's earlier evidence:** the original contract did not actually name numeric limits and overstated escaping-symlink rejection. The shared fixture now records the server policy explicitly: 256 MiB wire and decoded streams, 10,000 entries, 4096-byte paths, 64 levels, five minutes, absolute paths with no symlink ancestors, regular files/directories only, and no directory merge. The pinned client can create an outward symlink but refuses subsequent traversal through it. The server refuses links entirely. Empty raw CLI uploads may arrive chunked and are supported; nonempty raw uploads require Content-Length.

**Verification:** local backend and CLI suites; focused race checks on file tokens/tickets/gateway; actual `/tmp/bex-m150` launcher through the real mint/redeem and gateway handlers with a disposable local-process Executor; binary, zero-byte and nested-directory roundtrips; missing/existing destinations; late raw/directory failures; and token/path/archive/refusal matrices. The launcher preserves an existing local file after a failed download. A deliberate Go overlay mutation that finalized gzip after execution failure was detected by three regression cases. Postgres/OpenFGA integration endpoints were not configured; no live database/authorization or gVisor acceptance is claimed. Workspace lint includes a pre-existing CLI error-string punctuation correction needed for staticcheck.

**External gate and next action:** the shared OrbStack VM remains capped at 16 GiB and kernel `global_oom` kills dev-7 database/auth/control-plane processes; the failed recovery and isolation evidence are in [m149](../m149/README.md). The user/operator must restore sufficient shared VM memory capacity, scheduling the shared restart required for a higher limit or freeing enough capacity without disrupting unrelated stacks. Then run `bash scripts/dev-env.sh 7 up`, deploy the changed API/gateway, create a disposable sandbox, run the pinned launcher roundtrip and negative cases, remove fixtures, and record the dated live result before completing t003/t005/t009. No additional product decision or transfer implementation is outstanding.
