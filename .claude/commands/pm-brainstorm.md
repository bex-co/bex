---
description: Propose .pm milestones and tasks for a topic — decompose, size, and hand orchestration to /pm
argument-hint: "<topic or goal to break down>"
allowed-tools: Read, Bash(ls:*), Bash(find:*), Bash(cat:*)
---

# Task: Propose milestones and tasks

`/pm-brainstorm` is the **divergent** half of the pair: it thinks a topic through, **proposes** work, and hands orchestration and materialization to `/pm`. It writes **nothing** — the output is a text proposal. `$ARGUMENTS` is the topic/goal.

The board conventions — hierarchy, sizing rule, milestone quality gate, standing closing tasks, templates — live **canonically** in [`.claude/commands/pm.md`](pm.md). Read that file and apply its rules; do not restate or diverge from them here.

## Steps

1. **Load the canon and the anti-goals.** Read `.claude/commands/pm.md` (conventions) and `.pm/DO_NOT_DO.md` (hard constraint). If a proposed item conflicts with an anti-goal, reject it explicitly and explain why.
2. **Load context.** Read the relevant `.pm` board state (workstream `README.md`s, open milestones, inbox notes — `find .pm -name README.md`, plus loose notes) so proposals fit the existing roadmap and reuse its numbering/naming. Pick the workstream the topic belongs to (or propose a new `wN`).
3. **Discuss & decompose.** Talk the topic through with the user: pressure-test scope, surface dependencies and risks, and break it into candidate tasks, each with a rough estimate and `depends_on` links.
4. **Size and gate** each cluster of work using `/pm`'s sizing rule and milestone quality gate. Undersized work → propose an inbox note instead of a milestone, and say so. Work that fails the quality gate → mark it **not meaningful**, do not propose it as a milestone, and suggest a better-scoped alternative.
5. **Emit the proposal as text only** — the target workstream, each proposed milestone (task table + definition of done + source + goal linkage + expected outcome + why-now rationale) and/or inbox note. Propose **implementation tasks only**: `/pm` appends the standing closing tasks (Render parity when the milestone is feature dev/a fix touching REST/GraphQL/MCP/UI, then Simplify, then Test coverage, then Closeout) itself when it materializes, so do not include them — but do flag in the proposal whether you expect Render parity to apply, so `/pm` and the user aren't guessing. Do **not** write files.
6. **Hand off to `/pm`.** End by giving the exact `/pm` command(s) to materialize the proposal, e.g.:
   - `/pm new milestone w1 <title>` (then the tasks), or
   - `/pm add w1 <idea>` for sub-hour work, or
   - `/pm promote w1/NNN` to promote an existing inbox note.

## Topic

$ARGUMENTS
