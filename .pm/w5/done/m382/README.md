# w5 · m382 — Agent delivery hardening: one live session per branch, a verified head SHA, and a protection re-check before every takeover fast-forward

**Worker:** worker5 **Goal:** one session's sandbox can affect only its own delivery: no session is created or spawned onto a branch another live session holds, the head SHA bex records and steers on is GitHub's, and a taken-over pull request's branch is never fast-forwarded once GitHub protects it. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A session cannot be created or spawned onto a branch another live session holds on the same repository — **DONE** | 40m | — |
| [t002](done/t002.md) | The completer records GitHub's head for the session branch, not the sandbox's self-reported one — **DONE** | 30m | — |
| [t003](done/t003.md) | A takeover fast-forward re-checks branch protection before it writes — **DONE** | 20m | — |
| [t004](done/t004.md) | Devin parity and surface consistency — **DONE** | 15m | t001, t002, t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](done/t007.md) | Closeout — **DONE** | 10m | t006 |

Total: **7 tasks, about 2h40m**.

## Definition of done

1. Creating or spawning a session on a `bex-agent/*` branch that another session in the workspace is working on (archived or not) on the same repository is refused with 409 `AGENT_SESSION_BRANCH_IN_USE` on REST, GraphQL and MCP; a spawned child is refused too for a branch a finished session could still push to; a person's or automation's create may reuse a finished session's branch, and an omitted branch (`bex-agent/<id>`) is always accepted. An idempotent retry gets its session back, not a 409.
2. A completed turn whose sandbox reports a `RemoteHead` that differs from the session branch's head on GitHub records GitHub's head (or none when GitHub cannot be read), so CI-failure steering keys on a commit that is really on the branch.
3. A takeover fast-forward onto a pull request branch GitHub protects is refused with `pr_branch_protected` before `FastForwardBranch` runs, even when the branch was unprotected when the takeover began.
4. Each is covered by a regression test that fails on the pre-milestone code.

## Source + Goal linkage

- **Source:** inbox note [w5/188](../../done/188.md), from security review round 24 ([ADR107](../../../../docs/ADR107-security-review-round24.md), agent-plane hardening notes H-2, H-4, H-5). Promoted by `/loopx w5` on 2026-11-13: about 1h30m across three tasks.
- **Evidence:** `agentsessions/service.go` accepts any `bex-agent/*` branch and `agentsessions/spawn.go` passes the requested branch through; `agentsessions/completion.go` records `report.Git.RemoteHead` as `HeadSHA`, which `pr_feedback.go` compares with check-suite heads; `pr_takeover.go` checks `GitHub.BranchProtected` when a takeover starts, but `agentsession/pullrequest.go` `fastForwardTakenOver` relies only on GitHub refusing the write.
- **Goal linkage:** cloud coding agents (ADR047, ADR090, ADR102); the security-review lineage (ADR107).
- **Expected outcome:** a sandbox can push only to, and steer only on, its own session's delivery.
- **Why now:** round 24 filed these as the remaining agent-plane hardening; none is exploitable across a tenant or role today, and each closes before more delivery features land on these paths.
- **Devin parity:** included (t004). Devin works each session on its own branch; the refusal is a new error on the create and spawn surfaces, so it must read alike on REST, GraphQL, MCP and the dashboard.
