---
name: pm-brainstorm
description: >-
  Analyze and decompose a product topic into a text-only proposal for the repository's .pm board. Use when the user wants to pressure-test scope, size milestones, identify dependencies, or prepare exact PM board commands without writing board files.


allowed-tools: Read, Bash(ls:*), Bash(find:*), Bash(cat:*), Bash(rg:*), Bash(git fetch:*), Bash(git pull:*), Bash(git status:*), Bash(git show:*), Bash(git diff:*), Bash(git log:*), Bash(git ls-tree:*)
---

# Task: Propose milestones and tasks

`/pm-brainstorm` is the **divergent** half of the pair: it thinks a topic through, **proposes** work, and hands orchestration and materialization to `/pm`. It writes **nothing** — the output is a text proposal. `$ARGUMENTS` is the topic/goal.

The board conventions — hierarchy, sizing rule, milestone quality gate, standing closing tasks, templates — live **canonically** in [`.claude/skills/pm/SKILL.md`](../pm/SKILL.md). Read that file and apply its rules; do not restate or diverge from them here.

## Proposal count

Propose **five distinct, meaningful items** per run unless the user explicitly requests a different count. An item is a milestone or an inbox note, not an implementation task or a standing closing task. Each item must have concrete evidence, project-goal linkage, an observable outcome, and a why-now rationale.

Do not stop after restating the target workstream's existing inbox. Research enough candidates to select five that survive the anti-goal, deduplication, and quality gates. For a workstream-only request such as `w4`, look across the project for work that can be scheduled there; for a specific topic, stay within that topic. Existing board work may be reused or reshaped with explicit references, but must not be presented as a new discovery or filed twice.

Do not pad the count by splitting one outcome into artificial items, inflating sub-hour fixes into milestones, inventing gaps, or reopening deferred decisions. If substantive research still yields fewer than five eligible items, explain the specific evidence or scope constraints and the candidates rejected; never count rejected or speculative filler as meaningful proposals.

## Steps

1. **Load the canon and the anti-goals.** Read `.claude/skills/pm/SKILL.md` (conventions) and `.pm/DO_NOT_DO.md` (hard constraint). If a proposed item conflicts with an anti-goal, reject it explicitly and explain why.
2. **Sync autonomously and keep going.** Run `git fetch origin main` and `git status`. On local `main`, run `git pull --ff-only origin main` when behind; a dirty tree or untracked files alone are not a reason to stop — Git will refuse a fast-forward that would overwrite them. Preserve local work: do not reset, clean, stash, commit, or rebase to make brainstorming possible. If the pull is refused, the branch has diverged, or another branch is checked out, read the latest fetched `origin/main` board and source with `git show origin/main:<path>` and `git ls-tree`, comparing relevant local changes with `git diff` as needed. If fetching fails, use the available checkout and state that freshness could not be verified. Resolve routine issues yourself, briefly report the source revision and any fallback, and always continue to the proposal without asking the user how to proceed.
3. **Load context.** Read the relevant `.pm` board state (workstream `README.md`s, open milestones, inbox notes — `find .pm -name README.md`, plus loose notes) so proposals fit the existing roadmap and reuse its numbering/naming. Choose any existing `wN` using available capacity, dependency locality, and collision avoidance; every worker is general-purpose, and prior milestone topics do not create a specialty or ownership claim. Propose a new `wN` only when more independent queue capacity is useful. Also check `wN/done/` and any nested `done/` folders (`find .pm -type d -name done`) for milestones that already shipped the same capability — read their titles/READMEs before proposing, and drop or reshape any candidate that duplicates completed work instead of proposing it fresh. For any **Render-parity** topic, also read `docs/ADR018-render-parity.md` — the parity ledger already maps each REST/GraphQL/MCP/UI gap to `✅/◐/✖/—` and to an owning milestone/inbox note; reuse its assessment and cross-references (don't re-derive them), cite the matrix row a proposal closes, and don't propose work for a cell it marks `—` (deliberate non-goal) without re-opening that decision.
4. **Analyze & decompose without pausing.** Infer scope from the request and current board, pressure-test it, surface dependencies and risks, and break it into candidate tasks, each with a rough estimate and `depends_on` links. State reasonable assumptions in the proposal instead of asking clarifying questions. Where a product decision is unresolved, recommend an option or give conditional alternatives; do not treat brainstorming as authorization to implement them. Always deliver the text proposal in the same run.
5. **Size and gate** each cluster of work using `/pm`'s sizing rule and milestone quality gate. Undersized work → propose an inbox note instead of a milestone, and say so. Work that fails the quality gate → mark it **not meaningful**, do not propose it as a milestone, and suggest a better-scoped alternative.
6. **Emit the proposal as text only.** Give the full detail first: the target workstream, each proposed milestone (task table + definition of done + source + goal linkage + expected outcome + why-now rationale) and/or inbox note, numbered in proposed priority order. Propose **implementation tasks only**: `/pm` appends the standing closing tasks (Render parity when the milestone is feature dev/a fix touching REST/GraphQL/MCP/UI, then Simplify, then Test coverage, then Closeout) itself when it materializes, so do not include them — but do flag in the proposal whether you expect Render parity to apply, so `/pm` and the user aren't guessing. Close with a **"Summary (priority order)"** section: a numbered list of every candidate (milestones and inbox notes together) using the same numbers as above — one line each: `N. <title> (wN, ~size) — one-line outcome` — so the user can scan and pick by number without rereading the detail. Do **not** write files.
7. **Hand off to `/pm`.** End by giving the exact `/pm` command(s) to materialize the proposal, e.g.:
   - `/pm new milestone w1 <title>` (then the tasks), or
   - `/pm add w1 <idea>` for sub-hour work, or
   - `/pm promote w1/NNN` to promote an existing inbox note.

## Topic

$ARGUMENTS
