# w7 · m150 — Sandbox file copy through the pinned CLI

**Worker:** worker7 **Goal:** The distributed CLI can exchange files with an authorized sandbox using bounded, single-use transfers. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin the file-transfer contract and limits | 40m | — |
| t002 | Mint and redeem authorized file-connect tokens | 50m | w7/m150/t001 |
| t003 | Stream bounded uploads through the sandbox gateway | 55m | w7/m150/t002 |
| t004 | Stream bounded downloads and propagate failures | 55m | w7/m150/t002 |
| t005 | Verify the CLI round trip and update compatibility evidence | 40m | w7/m150/t003, w7/m150/t004 |
| t006 | Render parity | 30m | w7/m150/t001, w7/m150/t002, w7/m150/t003, w7/m150/t004, w7/m150/t005 |
| t007 | Simplify | 25m | w7/m150/t006 |
| t008 | Test coverage | 45m | w7/m150/t006 |
| t009 | Closeout | 15m | w7/m150/t007, w7/m150/t008 |

## Definition of done

- [ ] Contract fixtures match the actual pinned client rather than help output alone.
- [ ] Supported paths, archive/link behavior, limits and partial-transfer outcome are explicit.
- [ ] Sandbox groups are recorded as a deliberate non-goal; App SSH restrictions stay unchanged.
- [ ] Tokens bind caller, workspace, sandbox, operation and destination/source path.
- [ ] Expired, replayed, foreign and wrong-operation tokens are refused across replicas.
- [ ] OAuth and connect-token routes preserve their intended authentication boundaries.
- [ ] The real pinned CLI uploads supported fixtures successfully.
- [ ] Path traversal, unsafe archive entries/links and configured size limits are enforced.
- [ ] Cancellation stops transfer work and leaves only the documented partial-file outcome; bex-api gains no direct pods/exec privilege.
- [ ] Downloaded supported fixtures are byte-identical to sandbox contents.
- [ ] Missing paths, authorization loss and interrupted streams report truthful errors.
- [ ] No whole-file buffering or unbounded background transfer survives disconnect.
- [ ] Upload then download returns byte-identical data through the distributed CLI.
- [ ] Foreign owner, token replay, wrong path, oversize and interrupted transfer cases are evidenced.
- [ ] Compatibility grading cites the implementation and dated live result; disposable fixtures are removed.

## Source + Goal linkage

- **Source:** Approved w7 brainstorm item 3 via `$pm all for w7`; promoted `w7/046`, archived at `.pm/w7/done/046.md`; w7/m147 file-copy exclusion and `docs/cli-compatibility-checklist.md` copy gap.
- **Goal linkage:** ADR008 pillar 5 and ADR018 sandbox coverage: agents can provide inputs and retrieve outputs through the shipped CLI.
- **Expected outcome:** The pinned upstream CLI uploads and downloads supported files byte-for-byte; foreign access, replay, path escape and oversized transfer are refused.
- **Why now:** The distributed client exposes copy but its server routes return 404. Reuse the shipped connect-token and gateway seams from w7/m147 rather than writing or forking a CLI.
- **Render parity:** Included: tenant-facing changes; compare the applicable ADR018 coverage and all exposed surfaces.
- **Sizing:** 240m implementation; 355m including standing closing tasks, 9 tasks.

- **Approved scope decision:** sandbox file copy is in scope; sandbox groups are a deliberate non-goal. This narrowly extends sandbox transfer support and does not reopen App-instance SSH restrictions.
