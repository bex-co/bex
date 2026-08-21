---
name: loop-worker
description: Autonomously work a `.pm` workstream to completion — pick the next pending milestone, implement every task end to end, /ship it, then move to the next until none remain. Use when the user asks to "loop", drain, or work through a whole workstream's backlog (e.g. `/loop-worker w1`). Sequential, not interval-based; for a timed poll use /loop.
---

# Task: Drain a `.pm` workstream milestone by milestone

`/loop-worker <wN>` — repeatedly pick the next **pending** milestone in workstream `<wN>`, implement it fully, `/ship` it, and continue to the next, until the workstream has no pending milestones left (or a milestone genuinely blocks). This is a long-running autonomous loop over the `.pm` board; it composes `/pm` (board reads/writes), your own implementation work, and `/ship`.

Parse the target workstream from `$ARGUMENTS` (e.g. `w1`). If `$ARGUMENTS` is empty, **STOP** and ask which workstream to drain — never guess.

## Preconditions (verify once, up front)

1. `git branch --show-current` is `main`. If not, STOP and ask (same rule as `/ship`).
2. `git status` — note pre-existing uncommitted changes. Do not sweep unrelated changes into a milestone's ship; if the tree is dirty with work you didn't do, surface it and ask before starting.
3. The workstream `.pm/<wN>/README.md` exists. If not, STOP and report.

## The loop

Repeat until the exit condition below:

### 1. Pick the next pending milestone

Read `.pm/<wN>/README.md`. In the `## Milestones` list, pending milestones are the unchecked ones (`- [ ] **mN**`). Pick the **lowest-numbered** pending milestone that still has a live directory (`.pm/<wN>/mN/`, not under `done/`). Cross-check by listing `.pm/<wN>/m*/` and its `done/` folder — the checkbox and the on-disk state must agree; if they disagree, trust the task files and flag the drift.

If there are **no** pending milestones, go to **Exit**.

Announce which milestone you picked and give a one-line plan before doing work.

### 2. Understand the milestone

Read `.pm/<wN>/mN/README.md` and every task file `.pm/<wN>/mN/tNNN.md` (skip any already in `done/`). These define the scope, order, and acceptance. Milestones ship features **end to end** — include the frontend tasks alongside the backend ones; do not stop at the API. Respect `.pm/DO_NOT_DO.md` and the milestone's own acceptance criteria.

### 3. Implement it

Do the actual engineering, task by task, in the order the milestone implies:

- Follow all `CLAUDE.md` rules (id minting, boilerplate headers, `.env.example` sync, prettier on markdown, skill layout, etc.).
- Run the relevant test suites and make them pass before considering a task done — `make test` (from `lego/operator/`), `cd lego/backend && go test ./...`, dashboard `yarn test`, whichever the change touches. Never mark a task complete on unverified code.
- You may delegate independent sub-tasks to subagents (Agent tool) to parallelize, but you own correctness.
- Keep the milestone's `.pm` status in sync as you finish tasks (task frontmatter, milestone README `**Status:**` + the `— DONE` row, workstream checkbox) — these are `/pm`-governed writes, so follow the conventions in `.claude/skills/pm/SKILL.md` exactly. When the milestone is fully done, move it per the `.pm` rule: a done milestone with no open tasks moves **whole** to `.pm/<wN>/done/mN/` (`mv` the directory, then `rmdir` the empty original — leave no tombstone), and its workstream checkbox flips to `[x]`.

### 4. Ship it

Invoke **`/ship`** (the `ship` skill) for this milestone's changes. Because you made the changes this session, `/ship` runs session-aware: it stages exactly what you touched and writes the commit message from your knowledge. `/ship` ends at a successful push — it does not watch CI or the deploy. **Do not proceed to the next milestone until `/ship` reports the shipped HEAD.**

If `/ship` surfaces a failure it cannot fix (rebase conflict it can't resolve, rejected push), treat it as a **block** (see below).

### 5. Continue

Loop back to step 1 to pick the next pending milestone.

## Exit

Stop the loop and give a final summary when any of these holds:

- **Done:** no pending milestones remain in `<wN>`. Report which milestones you shipped this run.
- **Blocked:** a milestone needs a decision only the user can make (ambiguous scope, a `DO_NOT_DO` conflict, an external credential/access you lack, or a ship failure you can't resolve). Stop **before** shipping half-work — report the exact blocker and what you'd need to proceed. Do not skip a blocked milestone to grab a later one unless the user says so.
- **Budget/interrupt:** the user interrupts, or you've been running long enough that a checkpoint is warranted — report progress (shipped, in-flight, remaining) so the run can be resumed cleanly.

## Guardrails

- **One milestone per ship.** Never batch two milestones into one commit; each milestone lands as its own shipped unit so history and rollback stay clean.
- **Never ship red.** A failing test suite is a block, not a footnote. `/ship`'s own gate (all three suites must pass before deploy) backs this up — don't try to route around it.
- **Stay in `<wN>`.** Only pick milestones from the requested workstream. Workers are general-purpose, but this run is scoped to the queue the user named.
- **Report honestly.** If you skipped a task, mocked something, or a suite was flaky, say so in the per-milestone summary — don't present partial work as complete.
