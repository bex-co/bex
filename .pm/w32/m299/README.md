# w32 · m299 — Remaining Render SDK gaps: relative remote paths and mint during resume

**Worker:** worker32 **Goal:** the two residuals the m284 static review found are fixed, so the SDK walk can grade every method green **Status:** in progress (t001–t007 done; t008 closes after the production isolation-matrix run)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Verify Render's behavior for both residuals — **DONE** | 30m | — |
| t002 | Accept relative remote paths and resolve them under the exec user's home — **DONE** | 60m | t001 |
| t003 | Mint answers at once against a suspended sandbox — **DONE** | 60m | t001 |
| t004 | Docs: remove the residuals — **DONE** | 15m | t002, t003 |
| t005 | Render parity — **DONE** | 20m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- On a dev or production stack, the unmodified Python SDK's `copy_to("f.txt", "rel/f.txt")` lands in the sandbox user's `$HOME/rel/f.txt`, and `copy_from` reads it back.
- An SDK `exec` against a suspended sandbox succeeds with default timeouts.
- Both residual lines are gone from the checklist.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w32`, 2026-11-09, item 3. The residuals recorded by w32/m284 t001's static review (`docs/cli-compatibility-checklist.md` § Render SDK, "Known residuals").
- **Goal linkage:** Pillar 5 sandboxes and Render parity (ADR008): moving SDK code to bex should need only a base-URL change.
- **Expected outcome:** SDK scripts using relative paths, or touching a suspended sandbox, work unchanged.
- **Why now:** These are the only known failures the m284 live walk would hit. Fixing them first lets that walk grade every method green once the QA sign-in is back; this milestone doesn't need that sign-in.
- **Render parity:** Included: sandbox file and exec behavior on REST changes, and GraphQL/MCP must stay aligned. Dashboard N/A.

## Render's behavior (t001, checked 2026-11-09)

- **Relative paths:** the Render SDK v1.8.0 `copy_to` docstring says "A relative remote_path resolves under the sandbox's home directory and an absolute one addresses the filesystem, as scp does". The SDK sends a relative path as is, after `posixpath.normpath`.
- **Mint during suspension:** the SDK mints run and file tokens through its main API client, which has the httpx default 5 s timeout. The exec proxy reads with `read=45.0`, the download with `read=45.0`, and the upload with `read=None`. So Render answers a mint at once while the sandbox resumes, and the redeem is what waits.

## Progress — 2026-11-09

- **t002:** `sandboxfiles.ValidatePath` accepts clean relative paths: no `..` escape, the same byte limit, and levels counted below home. The gateway's scripts share a `cdParent` prelude that resolves a relative parent under the physically resolved `$HOME`, so the symlink-ancestor refusal still holds for the part the caller named. An unset `HOME` refuses rather than falling back to `/`. The CLI contract fixture's path policy is updated.
- **t003:** the run and file mint routes use `withoutResumeWait`: they start the resume and answer at once. `StreamExec` opens the SSE stream if it has to wait for a resume, sends `: keep-alive` every 15 s (`sandboxexec.KeepAliveInterval`, now shared with the gateway), and reports a later failure as the gateway's `error` event (`sandboxexec.ErrorEvent` via `core.ErrorReply`). A running sandbox's stream is unchanged: a pre-stream refusal is still an HTTP status.
- **t004:** the checklist's residuals are replaced by what changed. One residual stays recorded: a download from a sandbox that is still resuming waits before its first byte, so a resume over the SDK's 45 s read timeout fails that one download, since a binary body cannot carry a keep-alive.
- **t005 Render parity:** both behaviors are REST-only (the connect-token routes); GraphQL and MCP have no file transfer or token mint, and MCP `sandbox_exec` still waits within its bound. Dashboard N/A.
- **t006 Simplify:** applied the shared keep-alive and error event, `core.ErrorReply` (which keeps the timeout reply), relative depth counting, the unset-`HOME` refusal, context keys read once per wait, and trimmed comments.
- **t007:** `resume_connect_test.go` covers the immediate mint on both routes, the early-open stream that fails as an event, the unchanged running-sandbox refusal, and keep-alive pacing. `shell_test.go` runs the real scripts with a symlinked `$HOME`, `.` and nested parents, a symlink under home and a missing parent. `ticket_test.go` covers the relative accept and refuse table.
- **t008 next:** the production isolation matrix now checks a relative upload read back through `$HOME`, and a mint against an idle-suspended sandbox E that must answer in under 5 s and whose exec must then complete.
