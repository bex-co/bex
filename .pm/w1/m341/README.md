# w1 · m341 — Handoff, pickup and ssh: move work between a laptop and a Cloud Agent

**Worker:** worker1 **Goal:** a developer hands uncommitted local work to a new Cloud Agent session and picks the agent's branch back up locally, and opens a shell in the session's sandbox from `bex` (Devin `/handoff`, `/pickup`, `devin ssh` parity). **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Git snapshot without touching the worktree | 75m | — |
| t002 | `bex agent handoff` | 90m | t001 |
| t003 | `bex agent pickup` | 60m | — |
| t004 | `bex agent ssh` and chat `/ssh`, `/pickup` | 45m | t003 |
| t005 | Document handoff, pickup and ssh | 30m | t002, t004 |
| t006 | Live end-to-end on production | 60m | t005 |
| t007 | Render parity (Devin reference) | 15m | t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 30m | t007 |
| t010 | Closeout | 10m | t009 |

## Definition of done

1. On production, `bex agent handoff` in a QA repository checkout with uncommitted changes pushes `bex-agent/handoff-<8hex>`, creates a session on it, and that session's sandbox starts from the handoff commit (shown by the session's transcript or PR diff).
2. The worktree, real index, stash and HEAD are byte-identical before and after `handoff` (automated test against a bare remote), and a create failure deletes the pushed branch.
3. `bex agent pickup <session>` fetches the session branch and switches to it fast-forward-only; it refuses a dirty tree without `--stash`, a diverged local branch, and a repo mismatch, each with the documented exit code.
4. `bex agent ssh <session>` opens a shell on the session's sandbox through the system `ssh`; chat's `/ssh` and `/pickup` work.
5. `docs/bex-cli.md` documents handoff, pickup and ssh.

## Source + Goal linkage

- **Source:** ADR106 D5 (`ssh`) and D6; driver evidence `lego/agent-image/driver/src/git-setup.ts` `ensureRepo` (checks out `remoteBranchOid || baseOid`). Approved `/pm-brainstorm ADR106 for w1` item 4 via `/pmx`, 2026-11-11. Design: [ADR106](../../../docs/ADR106-cli-cloud-agents-automations.md).
- **Goal linkage:** ADR008 AI-native surfaces; Devin CLI parity (`/handoff`, `/pickup`, `devin ssh`). Recorded difference: no conversation transfer (bex ships no local agent).
- **Expected outcome:** local and cloud work flow both ways without copy-pasting diffs; the last client-only phase of ADR106 lands.
- **Why now:** it needs no server or driver change and completes ADR106's client-only phases; depends on m340 for `handoff --chat` and the chat slash commands. Render parity is included with Devin as the reference product. `bex agent forward` is out of scope until OD1 is decided (w1/145).
