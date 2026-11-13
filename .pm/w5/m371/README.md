# w5 · m371 — Security review round 24: the agent, sandbox and hosting surfaces shipped since round 23

**Worker:** worker5 **Goal:** every tenant-facing surface shipped since the last broad review (round 23, ADR085, 2026-08-29) has a recorded security verdict; confirmed findings are fixed with regression tests or filed with an owner; ADR107 records the round in the ADR028 lineage. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Scope and threat-model the surfaces shipped since round 23 | 45m | — |
| [t002](t002.md) | Audit the cloud-agent plane | 2h | t001 |
| [t003](t003.md) | Audit the sandbox plane | 1h30m | t001 |
| [t004](t004.md) | Audit the hosting surfaces | 1h30m | t001 |
| [t005](t005.md) | Fix the confirmed findings with regression tests; file the overflow | 2h | t002, t003, t004 |
| [t006](t006.md) | Record the round in ADR107 and the security lineage | 40m | t005 |
| [t007](t007.md) | Render parity | 20m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 30m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 9h40m**.

## Definition of done

1. Every surface in t001's scope table has a verdict (no finding, finding fixed, finding filed, or accepted residual with its DO_NOT_DO anchor) in ADR107, the round-24 record in `docs/`.
2. Every confirmed Critical or High finding is fixed, covered by a regression test that fails without the fix, and deployed; each Medium or Low is fixed or filed as a board item with severity and owner.
3. ADR107 is linked from the security lineage in `docs/AGENTS.md` and the root `AGENTS.md` key entry points.
4. No accepted residual in `.pm/DO_NOT_DO.md` (#PSL, #CI-RUNNERS, #RUNNER-HOSTS, #ENV-REVIEWERS, #APPROVAL-MODE) is re-filed as a finding.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 9, item 3 (2026-11-12), filed by `/pmx 1,2,3,4` with w5 chosen. Re-checked at filing against `origin/main` `98b3587a8`: no open security-review item exists on the board; w3/m365 (push protection and branch rules, done 2026-11-13) is repository settings, not these surfaces.
- **Last broad review:** round 23 ([ADR085](../../../docs/ADR085-security-review-round23.md), codex-security `5PePz5` at `3cbb7b8e`, accepted 2026-08-29). Since then only the static-site rewrite proxy has had a focused review ([ADR103](../../../docs/ADR103-security-review-static-rewrite-proxy.md), 2026-11-10).
- **Shipped since** (ADR086–ADR106 and recent milestones): account deletion (ADR086); the obs.bex.co ops gate (ADR088); agent-owned delivery, where the agent commits, pushes and opens PRs (ADR090); Postgres extensions (ADR092); the Cloud Agents MCP (ADR094); agent attachments (ADR095); build sources and cross-service image copies (ADR097); agent session secrets (ADR098); workspace skills (ADR099); scheduled automations (ADR101); child sessions (ADR102); PR feedback and `/bex` takeover (`agentsessions/pr_feedback.go`, `pr_takeover.go`); sandbox Render parity, groups and OCI image import (ADR104, w2/m358, w2/m359); the bare-metal sandbox node (ADR105); the CLI's agent ssh, handoff and pickup (ADR106); Postgres credential rotation (w5/m265); Key Value internal auth (w5/m268); Blueprint plan and approve over MCP (w5/m283); readable webhook secrets (w5/m285).
- **Goal linkage:** GOAL item 7 (security review) and the ADR028 review lineage.
- **Expected outcome:** the 75 days of new attack surface, much of it running untrusted code or holding GitHub tokens, get a verdict before open signup.
- **Why now:** cards are on file for every workspace and charged since 2026-11-12 (ADR075 D7, w6/m362), Render's sandboxes went public beta on 2026-11-11 and bex's match them, and the agent plane now acts on GitHub with installation tokens and on PR comments.
- **Render parity: included, conditionally.** Fixes are likely to change refusals on REST, GraphQL or MCP; t007 checks them against render.com for hosting surfaces and Devin for agent surfaces, or records that no surface changed.
