# w22 · m445 — Render SDK v1.10.0's sandbox contract holds on bex: its typed errors fire and its new calls are answered

**Worker:** worker22 **Goal:** code written against Render's Python and TypeScript SDK v1.10.0 behaves the same pointed at bex: every sandbox error code the SDK turns into a named error class is the code bex answers in that situation, an operation on an ended sandbox carries what `SandboxEndedError` reports, and the calls v1.9.0 and v1.10.0 added are served, a group's project environment included. The SDK's request and error shapes are frozen as contract fixtures. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                              | est | depends_on       |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Extract from the SDK v1.10.0 source the error-code to error-class table and the new calls' request and response shapes, and freeze them as fixtures | 60m | —                |
| t002 | Compare bex's answers code by code and list each code that is missing or spelled differently                                                       | 40m | t001             |
| t003 | Supply the missing codes, with one set of constants for REST, GraphQL and MCP                                                                      | 60m | t002             |
| t004 | What `SandboxEndedError` reports (that the sandbox ended, and when) and a failed image import's `failureCode`                                      | 45m | t002             |
| t005 | Extend `sandbox/sdk_compat_test.go`: one case per mapped code, and the replayed `images.list` and `images.delete` requests                         | 45m | t003, t004       |
| t011 | SDK v1.10.0's group environment: `createGroup({environmentId})` and `setGroupEnvironment()` replayed against bex | 45m | t001, w22/m603 |
| t012 | Python `sandboxes.create(wait=True)` canceled while waiting terminates the new sandbox on bex; bex's SDK scenario imports `render.sandbox` | 40m | t001 |
| t006 | Update the checklist's SDK section and move w32/m284's live-walk target to v1.10.0                                                                 | 20m | t005, t011, t012             |
| t007 | Render parity                                                                                                                                      | 20m | t003, t004, t011, t012       |
| t008 | Simplify                                                                                                                                           | 20m | t007             |
| t009 | Test coverage                                                                                                                                      | 30m | t007             |
| t010 | Closeout                                                                                                                                           | 10m | t006, t008, t009 |

## Definition of done

- A fixture file under the backend's test data lists every sandbox API error code SDK v1.10.0 maps to a named error class, with the SDK version and commit it was read from.
- For each code in it, a test drives bex into the situation and asserts the status and the `code` string. Changing the spelling of any one code in bex fails a test. A code whose situation cannot occur on bex (snapshots before w32/m285, brokered credentials) is listed as such with the reason, not silently skipped.
- An exec or file operation on a terminated or errored sandbox answers what the SDK needs to raise `SandboxEndedError` with the time the sandbox ended.
- A failed image import reports a `failureCode` in the field the SDK's `SandboxImage` reads.
- The requests `images.list()` (filters, paging, newest first) and `images.delete()` send are replayed against the router and answered as the SDK expects.
- `docs/cli-compatibility-checklist.md`'s SDK section has no row that contradicts what is served, and ADR104 D9 names v1.10.0.
- SDK v1.10.0's `createGroup({environmentId})` and `setGroupEnvironment(id, environmentId | null)` requests (TypeScript, and Python's `create_group(environment_id=…)` and `set_group_environment()`) are replayed against the router and the group reads the environment each call set (the server side is w22/m603).
- A Python `sandboxes.create(wait=True)` canceled while the sandbox is still `creating` leaves no running sandbox on bex: the SDK's terminate on a `creating` sandbox is accepted and the sandbox ends.
- The backend suite is green against the real database.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w22` (2026-11-16), item 4, filed by `/pmx 1,3,4,5 to w22`.
  - **Render** (render.com/changelog: "Python SDK v1.9.0" and "TypeScript SDK v1.9.0", 2026-11-16; [github.com/render-oss/sdk/releases](https://github.com/render-oss/sdk/releases), read 2026-11-16). Added in both: `images.list()` ("newest first, with filtering and pagination"), `images.delete()` ("fails while sandboxes or snapshots still use it, and the error names them"), `SandboxEndedError` ("raised by exec and file operations on a terminated or errored sandbox. It reports when the sandbox ended"), a TTL in seconds on `snapshots.create()` (`0` keeps the snapshot), alpha brokered credentials. TypeScript only: `copyTo()` and `copyFrom()`, `SandboxImage.failureCode`. Breaking in both: `snapshots.create()` no longer takes the expiry time; "every sandbox call now raises a specific error for known API codes, such as `SandboxGroupSuspendedError`, rather than a generic `ClientError`".
  - **Not read:** the SDK's full code-to-class table. The release detail page answered 404 during the brainstorm, so the table comes from the source in t001. Until then the size of t003 is an estimate.
  - **bex today:** `grep -rn -i 'sandbox_group_suspended' lego/backend/internal` finds nothing, so at least that code is not answered. `lego/backend/internal/sandbox/sdk_compat_test.go` covers v1.8.0's names and create shapes. `docs/cli-compatibility-checklist.md` (~line 362) still grades the SDK's `*_group`, `images.*` and `builds.*` as refused or a non-goal, although groups (w22/m311) and images (w2/m358) are served. Ended-sandbox reads were shipped by w3/m318.
  - `.pm/w32/blocked/m284` (SDK v1.8.0 compatibility) is parked on QA credentials for its live walk.
- **Retargeted to SDK v1.10.0 (2026-11-23)** by `/pmx them all to w22`, item 3 of `/pm-brainstorm one aspect of render parity for w22`, before any task started. Render published Python and TypeScript v1.10.0 on 2026-11-20, four days after v1.9.0 (release notes read 2026-11-23): `createGroup()`/`create_group()` accept an environment id ("Creating a group in a protected environment requires the Admin role in the workspace"); `setGroupEnvironment()`/`set_group_environment()` put a group in an environment, move it, or take it out with `null`/`None`; `render.experimental.sandbox` is deprecated in favor of `render.sandbox` (the old path still works); and Python's `sandboxes.create(wait=True)` "terminates the new sandbox if the call is canceled while it waits for the sandbox to become ready". The release notes add no error class, so the v1.9.0 table t001 extracts is expected to carry over; t001 reads it from v1.10.0 and records any difference. The TypeScript source (`typescript/src/sandboxes/index.ts` at `typescript/v1.10.0`) sends `environmentId` in the `POST /sandbox-groups` body, and the move through `POST`/`DELETE /environments/{id}/resources`, which bex answers 400 for an `sbg-` id until w22/m603. The only `render.experimental.sandbox` import in the repository is `scripts/render-sdk-compat/scenario.py`. Assumption: Python's `set_group_environment` uses the same routes (release notes; t011 reads its source).
- **Goal linkage:** ADR008 § Render parity first; ADR104 D9 ("the SDK is part of the contract"), gap 5 (SDK walk) and gap 15 (freeze the SDK shapes as contract fixtures).
- **Expected outcome:** a user's `except SandboxGroupSuspendedError` or `catch (e instanceof SandboxEndedError)` branch runs on bex as it does on Render.
- **Why now:** v1.10.0 is current (v1.9.0, which it supersedes, is the breaking release that makes error codes part of what user code branches on), and m284 is parked a version behind. This milestone needs no credentials, so it removes the two-version backlog m284 would otherwise meet when it unparks.
- **Boundaries:** the live walk stays with w32/m284 (t006 only retargets it). Snapshot TTL behavior stays with w32/m285; this milestone freezes its request shape. Brokered credentials are alpha on Render and stay a named refusal (ADR104 § Out of scope).
- **Render parity included:** error codes are customer-facing on REST, GraphQL and MCP, so t007 compares each surface.
