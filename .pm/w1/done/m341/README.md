# w1 · m341 — Handoff, pickup and ssh: move work between a laptop and a Cloud Agent

**Worker:** worker1 **Goal:** a developer hands uncommitted local work to a new Cloud Agent session and picks the agent's branch back up locally, and opens a shell in the session's sandbox from `bex` (Devin `/handoff`, `/pickup`, `devin ssh` parity). **Status:** done (2026-11-12; verified on production, see Live check)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Git snapshot without touching the worktree — **DONE** | 75m | — |
| t002 | `bex agent handoff` — **DONE** | 90m | t001 |
| t003 | `bex agent pickup` — **DONE** | 60m | — |
| t004 | `bex agent ssh` and chat `/ssh`, `/pickup` — **DONE** | 45m | t003 |
| t005 | Document handoff, pickup and ssh — **DONE** | 30m | t002, t004 |
| t006 | Live end-to-end on production — **DONE** | 60m | t005 |
| t007 | Render parity (Devin reference) — **DONE** | 15m | t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 30m | t007 |
| t010 | Closeout — **DONE** | 10m | t009 |

## Definition of done

1. On production, `bex agent handoff` in a QA repository checkout with uncommitted changes pushes `bex-agent/handoff-<8hex>`, creates a session on it, and that session's sandbox starts from the handoff commit (shown by the session's transcript or PR diff).
2. The worktree, real index, stash and HEAD are byte-identical before and after `handoff` (automated test against a bare remote), and a create failure deletes the pushed branch.
3. `bex agent pickup <session>` fetches the session branch and switches to it fast-forward-only; it refuses a dirty tree without `--stash`, a diverged local branch, and a repo mismatch, each with the documented exit code.
4. `bex agent ssh <session>` opens a shell on the session's sandbox through the system `ssh`; chat's `/ssh` and `/pickup` work.
5. `docs/bex-cli.md` documents handoff, pickup and ssh.

## Source + Goal linkage

- **Source:** ADR106 D5 (`ssh`) and D6; driver evidence `lego/agent-image/driver/src/git-setup.ts` `ensureRepo` (checks out `remoteBranchOid || baseOid`). Approved `/pm-brainstorm ADR106 for w1` item 4 via `/pmx`, 2026-11-11. Design: [ADR106](../../../../docs/ADR106-cli-cloud-agents-automations.md).
- **Goal linkage:** ADR008 AI-native surfaces; Devin CLI parity (`/handoff`, `/pickup`, `devin ssh`). Recorded difference: no conversation transfer (bex ships no local agent).
- **Expected outcome:** local and cloud work flow both ways without copy-pasting diffs; the last client-only phase of ADR106 lands.
- **Why now:** it needs no server or driver change and completes ADR106's client-only phases; depends on m340 for `handoff --chat` and the chat slash commands. Render parity is included with Devin as the reference product. `bex agent forward` is out of scope until OD1 is decided (w1/145).

## Live check (t006, 2026-11-12)

The run used a locally built `bex` with the QA device login, in the `bex` workspace. Its target was a scratch clone of the fixture repository `bex-co/bex-hello-go-live` (auto-deploy off). The clone's only change was an untracked `QA-HANDOFF.md` (`started on a laptop`).

| Check | Result |
| --- | --- |
| `handoff --confirm --max-spend 1 --wait -o json "Append the line 'finished in the cloud' to QA-HANDOFF.md …"` | It showed the plan: repository, branch `bex-agent/handoff-9793d609`, from `7ad20c989b80 + uncommitted changes`, the diffstat, and `Untracked files sent (1): QA-HANDOFF.md`. It pushed, and the session succeeded in 168 s. The agent appended to `QA-HANDOFF.md`, a file that existed only in the uncommitted local change, so the sandbox started from the handoff commit. The agent committed `dbc0ea0`, pushed, and opened draft PR #11. The checkout's HEAD and its untracked file were unchanged |
| `pickup` with the untracked file still there | Exit 2, `the checkout has uncommitted changes: commit them, or pass --stash` |
| `pickup <dashboard URL> --stash` | Printed `Stashed your changes …`, then `On bex-agent/handoff-9793d609 at dbc0ea068c75` and the PR link. The file read `started on a laptop` / `finished in the cloud`. History: the agent's commit, then the handoff commit, then the base. The stash held `bex pickup ags-…` |
| `ssh` with no running sandbox | Exit 1, `… has no running sandbox; send it a message to start one` |
| `ssh` during a turn (`send … 'sleep 90'`) | `bex agent ssh <id> -- -o BatchMode=yes … 'echo connected; pwd'` printed `connected` and `/workspace` |

**Found and fixed during the check.** A remote command after `--` was put before the destination, and ssh read it as the hostname (`hostname contains invalid characters`). `sshArgv` now places the destination after ssh's leading options and their values, and before a command. `TestSSHArgv` covers this.

**Cleanup.** PR #11 was closed and its branch deleted. The session was deleted, and so was the scratch clone with its stash. The 12 `bex-agent/*` branches still in the fixture repository predate this run.

**Re-check after `/simplify` (same day).** The review refactor changed four things:
- `create` became `request` plus `createSession`, so every flag is checked before the push;
- git errors carry exit codes, and git runs in the C locale;
- the index is copied into a temporary directory, and any read error other than a missing index is refused;
- untracked files are listed with `-z` and counted, and pickup switches through `for-each-ref`.

A second live round on a fresh clone then ran: `handoff` (branch `bex-agent/handoff-10aad91b`, `+ uncommitted changes`, `Untracked files sent (1): QA-HANDOFF-2.md`), then the agent's commit and PR #12. `pickup --stash -o json` answered `{"branch":…, "commit": "01a7bb3…", "pullRequest": ".../pull/12"}`, and the file read `second round` / `r2 cloud`. PR #12 was closed with its branch deleted, and the session and clone were deleted.
