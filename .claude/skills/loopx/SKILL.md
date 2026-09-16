---
name: loopx
description: Autonomously drain a `.pm` workstream — triage every open item (milestone or inbox-note task), then work it to done, park it in `blocked/`, or delete it as invalid, until the workstream has no open items left. Use when the user asks to "loop", drain, or clear a whole workstream's backlog (e.g. `/loopx w1`). Sequential, not interval-based; for a timed poll use /loop.
---

# Task: Drain a `.pm` workstream item by item

`/loopx <wN>` — repeatedly pick the next **open item** in workstream `<wN>` — a pending milestone `wN/mN/` **or** an open inbox note `wN/NNN.md` (a sub-hour task) — **triage** it, then give it one of four outcomes: **work it and move it to `done/`**, **close it as already met**, **move it to `blocked/`**, or **delete it as invalid**. Continue until the workstream has no open items left. This is a long-running autonomous loop over the `.pm` board; it composes `/pm` (board reads/writes), your own implementation work, and `/ship`.

Parse the target workstream from `$ARGUMENTS` (e.g. `w1`). If `$ARGUMENTS` is empty, **STOP** and ask which workstream to drain — never guess.

## Preconditions (verify once, up front)

1. `git branch --show-current` is `main`. If not, STOP and ask (same rule as `/ship`).
2. `git status` — note pre-existing uncommitted changes. Do not sweep unrelated changes into an item's ship; if the tree is dirty with work you didn't do, surface it and ask before starting.
3. The workstream `.pm/<wN>/README.md` exists. If not, STOP and report.
4. Read `.pm/DO_NOT_DO.md` once — it governs every triage verdict below.

## Build the queue (once, then refresh each iteration)

Open items in `<wN>` are everything **not** already under `done/` or `blocked/`:

- **Milestones** — `- [ ] **mN**` in `.pm/<wN>/README.md` with a live directory `.pm/<wN>/mN/`.
- **Inbox notes** — `.pm/<wN>/NNN.md` (plain markdown, no frontmatter). These are the sub-hour tasks; they are part of the backlog, not background noise.

Cross-check checkboxes against on-disk state (`ls .pm/<wN>/m*/ .pm/<wN>/*.md .pm/<wN>/done/ .pm/<wN>/blocked/`). If they disagree, trust the files and flag the drift.

**Order:** pending milestones by ascending number first, then open inbox notes by ascending number — unless a stated dependency forces otherwise, in which case say so.

If the queue is empty, go to **Exit**.

## The loop

For each item, announce which one you picked and a one-line plan before doing work.

### 1. Triage

Read the item in full — for a milestone, `.pm/<wN>/mN/README.md` plus every task file `tNNN.md` not already in `done/`; for an inbox note, the note itself. Then check its premise against the current codebase (the note may be weeks old and the bug already fixed). Pick exactly one verdict:

| Verdict | When | Action |
| --- | --- | --- |
| **Work** | The premise holds and nothing external gates it. | step 2 → 3 → **done** |
| **Already met** | The described end state is **already true in the code** — verify it, don't assume. | **done** + evidence |
| **Blocked** | A gate you cannot clear: physical hardware, a credential/console only the user has, an upstream item elsewhere, or a decision only the user can make. | **blocked** |
| **Invalid** | The premise is false, the work is a duplicate of a shipped/live item, or it conflicts with `.pm/DO_NOT_DO.md`. | **delete** |

Rules for triage:

- **Evidence, not vibes.** "Already met" and "invalid" both require a concrete pointer — `file.go:123`, a passing test, the surviving duplicate's id, the `DO_NOT_DO` line.
- **When torn between blocked and invalid, choose blocked.** Deleting is the only outcome that destroys information.
- **Size check on inbox notes.** If a note turns out to be > ~1h across more than one task, run `/pm promote <wN/NNN>` to materialize it as a milestone, then work that milestone in this same iteration.
- **Never split a milestone's verdict.** If some tasks are implementable and one is gated, the milestone is **blocked** — finish every implementable task first, then park it (this is the `w11` pattern: implemented tasks done, the gated task named).

### 2. Implement it (Work verdict only)

Do the actual engineering, task by task, in the order the item implies:

- Follow all `CLAUDE.md` rules (id minting, boilerplate headers, `.env.example` sync, prettier on markdown, skill layout, etc.).
- Milestones ship features **end to end** — include the frontend tasks alongside the backend ones; do not stop at the API.
- Run the relevant test suites and make them pass before considering a task done — `make test` (from `lego/operator/`), `cd lego/backend && go test ./...`, dashboard `yarn test`, whichever the change touches. Never mark a task complete on unverified code.
- You may delegate independent sub-tasks to subagents (Agent tool) to parallelize, but you own correctness.
- Keep `.pm` status in sync as you finish tasks (task frontmatter, milestone README `**Status:**` + the `— DONE` row, workstream checkbox). These are `/pm`-governed writes — follow `.claude/skills/pm/SKILL.md` exactly.

### 3. Land the outcome

Every item ends in exactly one of these on-disk states. Leave **no tombstone, stub, or redirect** at an item's old path.

**done** — the work is real and verified.

- Task: `status: done`, row `— **DONE**`, file → `wN/mN/done/tNNN.md`.
- Milestone with no open tasks: `mv` the whole directory → `wN/done/mN/`, `rmdir` the original, flip the workstream checkbox to `- [x]`.
- Inbox note: `mv` → `wN/done/NNN.md`.
- For an **already met** item, append one line to the moved item recording the evidence and the date, so the record says why it closed without a code change.

**blocked** — implementable work finished, a named gate remains.

- `mv` the milestone directory → `wN/blocked/mN/` (or the note → `wN/blocked/NNN.md`).
- Leave the workstream README line unchecked (`- [ ]`) and rewrite it as `**BLOCKED (<exactly what you need from whom>)**` with the path updated to `blocked/…`, plus which tasks are already done. See `.pm/w11/README.md` for the shape.
- Surface every blocker again in the final report — a gate the user never reads is a gate that never clears.

**delete** — the item is invalid.

- `git rm` the file or directory and remove its line from the workstream `README.md`.
- Record in the run report: the item, the verdict, and the evidence. Never delete an item that contains completed task records — move it to `done/` instead so the history survives.

Then run `npx prettier@3.4.2 --write "**/*.md"` (repo rule).

### 4. Ship it

Invoke **`/ship`**. Because you made the changes this session, `/ship` runs session-aware: it stages exactly what you touched and writes the commit message from your knowledge.

- **Code-bearing items ship alone** — one milestone (or one worked note) per commit, so history and rollback stay clean.
- **Board-only outcomes** (blocked, deleted, already-met) need no commit of their own; batch them into a single `docs(pm):` ship. They must be shipped before the run ends — don't leave board mutations uncommitted.

`/ship` ends at a successful push — it does not watch CI or the deploy. Do not start the next item until `/ship` reports the shipped HEAD. If `/ship` surfaces a failure it cannot fix (a rebase conflict it can't resolve, a rejected push), treat it as a **run-level block** and stop.

### 5. Continue

Refresh the queue (the board changed) and loop back to triage the next item.

## Exit

Stop and give a final summary when any of these holds:

- **Drained:** no open items remain in `<wN>`. Report every item and its outcome, grouped: shipped, closed as already met, parked in `blocked/` (with each gate), deleted (with each reason).
- **Run-level block:** a ship failure you can't resolve, or the tree is in a state you shouldn't push. Per-item blocks do **not** stop the run — they get parked and the loop continues.
- **Budget/interrupt:** the user interrupts, or you've run long enough that a checkpoint is warranted — report progress (done, in-flight, remaining) so the run resumes cleanly.

## Guardrails

- **Never ship red.** A failing test suite is a per-item block, not a footnote. `/ship`'s own gate backs this up — don't route around it.
- **Never ship half-work.** If an item can't be finished, it becomes **blocked** with the implementable part complete — not a partial "done".
- **Stay in `<wN>`.** Only touch items from the requested workstream. Workers are general-purpose, but this run is scoped to the queue the user named.
- **Deletion is a claim, not a shortcut.** Every deleted item costs you a sentence of evidence in the report. If you can't write that sentence, it isn't invalid.
- **Report honestly.** If you skipped a task, mocked something, or a suite was flaky, say so in that item's summary — don't present partial work as complete.
