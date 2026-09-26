---
name: loopx
description: Autonomously drain a `.pm` workstream — triage every open item (milestone or inbox-note task), then work it to done, park it in `blocked/`, or delete it as invalid, until the workstream has no open items left. Use when the user asks to "loop", drain, or clear a whole workstream's backlog (e.g. `/loopx w1`). Sequential, not interval-based; for a timed poll use /loop.
---

# Task: Drain a `.pm` workstream item by item

`/loopx <wN>` — repeatedly pick the next **open item** in workstream `<wN>` — a pending milestone `wN/mN/` **or** an open inbox note `wN/NNN.md` (a sub-hour task) — **triage** it, then give it one of four outcomes: **work it and move it to `done/`**, **close it as already met**, **move it to `blocked/`**, or **delete it as invalid**. Continue until the workstream has no open items left. This is a long-running autonomous loop over the `.pm` board; it composes `/pm` (board reads/writes), your own implementation work, and `/ship`.

Parse the target workstream from `$ARGUMENTS` (e.g. `w1`). If `$ARGUMENTS` is empty, **STOP** and ask which workstream to drain — never guess.

## Preconditions (verify once, up front)

1. `git branch --show-current` is `main`. If not, STOP and ask (same rule as `/ship`).
2. `git status` — note pre-existing uncommitted changes, then keep going. Do not sweep unrelated changes into an item's ship: stage files by name, never `git add -A`.
   - **Uncommitted `.pm` board work is never a reason to stop.** Teammate sessions file milestones and notes into the same workstream continuously, often mid-run; that is a queue refresh, not a dirty tree. Pick the new items up in the next queue refresh and drain them like any other. If a teammate's filing is still uncommitted when you land your own outcome, commit it as its own `docs(pm): file <items>` commit first — separate from yours, so authorship stays legible — then ship your work. Never edit the content of an item you did not file except to record your own verdict on it.
   - **You share one working tree with those sessions.** Before each board write, re-read the file you are about to edit (it may have changed since you last read it) and apply the smallest edit that lands your verdict. That is the whole mitigation — it does not require coordination, a pause, or a question.
   - Only genuinely foreign **source** changes — uncommitted edits under `lego/`, `dashboard/`, `infra/`, `scripts/` that no one in this session made and no `.pm` item explains — are worth surfacing. Say what they are, leave them unstaged, and drain around them; stop only if they make the tree unshippable (see **Exit**).
3. The workstream `.pm/<wN>/README.md` exists. If not, STOP and report.
4. Read `.pm/DO_NOT_DO.md` once — it governs every triage verdict below.

## Build the queue (once, then refresh each iteration)

Open items in `<wN>` are everything **not** already under `done/` or `blocked/`:

- **Milestones** — `- [ ] **mN**` in `.pm/<wN>/README.md` with a live directory `.pm/<wN>/mN/`.
- **Inbox notes** — `.pm/<wN>/NNN.md` (plain markdown, no frontmatter). These are the sub-hour tasks; they are part of the backlog, not background noise.

Cross-check checkboxes against on-disk state (`ls .pm/<wN>/m*/ .pm/<wN>/*.md .pm/<wN>/done/ .pm/<wN>/blocked/`). If they disagree, trust the files and flag the drift.

**Order:** pending milestones by ascending number first, then open inbox notes by ascending number — unless a stated dependency forces otherwise, in which case say so.

**Urgency may reorder, and never stops the loop.** You may pull a security or data-loss item ahead of the prescribed order — name the reason in one line and take it. What you must not do is stop to ask which to work first: an item further down the queue looking more important than the one in front of you is a reordering decision you are equipped to make, not a question for the user. They can always reorder by interrupting.

If the queue is empty, go to **Exit**.

## The loop

For each item, announce which one you picked and a one-line plan before doing work.

**Every item gets its verdict without a user round-trip.** An unusual shape — a record-only note, one you withdrew earlier in this session, a duplicate, an empty stub, a note whose premise a teammate already fixed — maps to a row in the tables below: land that verdict on disk and move to the next item in the same turn. The run pauses only for the **Exit** conditions, which are about the repository being unshippable, not about an item being unusual.

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
- **Non-implementation notes have preset verdicts — apply them without asking.** Many inbox notes are not "build this"; they are records, withdrawals, or debris. Route them by shape, in this order (first match wins):

  | Shape | Verdict | Landing |
  | --- | --- | --- |
  | **Blank or stub** — no `Why:` line and no substantive body, or a placeholder never filled in | **invalid** | `git rm`, drop its README line |
  | **Duplicate with no unique content** — the surviving item covers it entirely and this note adds no findings of its own | **invalid** | `git rm`, cite the survivor's id |
  | **Duplicate or withdrawal that carries its own findings/history** — including a note you withdrew earlier in this run | **done** | `done/`, append the survivor's id + date; never `git rm` history |
  | **Record-only** — a coverage record, sweep log, or audit trail documenting work already performed, with nothing left to implement | **done** | `done/`, append one line dating the closure |
  | **Already fixed by someone else** — the premise held when filed but a commit since resolved it | **done** | `done/`, append the fixing commit SHA |

  These are the common cases a drain hits; none of them is a reason to pause. If a note matches none of the shapes and none of the four verdicts, prefer **blocked** with the ambiguity named — parking is always available and never destroys information.

- **Never split a milestone's verdict.** If some tasks are implementable and one is gated, the milestone is **blocked** — finish every implementable task first, then park it (this is the `w11` pattern: implemented tasks done, the gated task named).

### 2. Implement it (Work verdict only)

Do the actual engineering, task by task, in the order the item implies:

- Follow all `AGENTS.md` rules (id minting, boilerplate headers, `.env.example` sync, prettier on markdown, skill layout, etc.).
- Milestones ship features **end to end** — include the frontend tasks alongside the backend ones; do not stop at the API.
- Run the relevant test suites and make them pass before considering a task done — `make test` (from `lego/operator/`), `cd lego/backend && go test ./...`, dashboard `yarn test`, whichever the change touches. Never mark a task complete on unverified code.
- You may delegate independent sub-tasks to subagents (Agent tool) to parallelize, but you own correctness.
- Keep `.pm` status in sync as you finish tasks (task frontmatter, milestone README `**Status:**` + the `— DONE` row, workstream checkbox). These are `/pm`-governed writes — follow `.claude/skills/pm/SKILL.md` exactly.

### 3. Land the outcome

Every item ends in exactly one of these on-disk states, applied **automatically as soon as the verdict is reached** — no confirmation, no "should I move this?". Leave **no tombstone, stub, or redirect** at an item's old path.

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

Refresh the queue (the board changed) and loop back to triage the next item. The refresh reads the board as it is **now**, so items a teammate filed since you started are simply part of the queue — triage them in the normal order alongside the ones you began with.

## Exit

**Before you write a final summary, refresh the queue.** If it returns open items, the user has not interrupted, and you can still do work — discard the summary and take the next item. Writing a summary is not a decision to stop; finishing the queue is.

Stop and give a final summary when any of these holds:

- **Drained:** a queue refresh returns **zero** open items in `<wN>`. Report every item and its outcome, grouped: shipped, closed as already met, parked in `blocked/` (with each gate), deleted (with each reason). "Drained" is measured against the queue as it stands at that refresh, never against the queue as it stood at invocation — items others filed while you worked are part of it, and a workstream under active filing may legitimately keep you working for a long time. That is the requested behavior.
- **Run-level block:** a ship failure you can't resolve, or the tree is in a state you shouldn't push. Per-item blocks do **not** stop the run — they get parked and the loop continues.
- **Interrupt:** the user sends a new message, or the context budget is genuinely exhausted — you cannot fit another item's work. **Running for a long time is not an exit**, and neither is a queue that keeps growing; both are checkpoints. See below.

<<<<<<< Updated upstream
**Not exit conditions**, and never a reason to pause mid-drain: an item that is a record rather than a task; an item you yourself withdrew or filed earlier; a duplicate; an empty note; an item already fixed upstream; a verdict that feels unusual. Each of those has a row in the triage tables — apply it and keep going.

Also **never** a reason to pause: **new work arriving while you drain.** A teammate session filing fresh milestones or notes into `<wN>` — uncommitted, half-written, or landing seconds ago — is the queue doing its job, not a hazard. A peer session showing `busy` against the same workstream changes nothing either. Refresh the queue, take the new items in order, and drain them. A backlog that grows during the run means the run continues; it does not mean the run stops to ask about it.

## Checkpoints — these do NOT end the run

There is a real need to tell the user things mid-drain: a blocker parked, a premise disproved, a milestone that turned out three times its estimate, a queue that doubled. Serve it with a **checkpoint**: a few lines of progress, then **immediately take the next item in the same turn**.

A checkpoint is a report, not a stop. The distinction matters because the pull to stop arrives disguised as a duty to report — you finish an item, notice something the user should know, write it up, and the write-up reads like an ending. It is not. If workable items remain, the summary you just wrote is a checkpoint; post it and keep going.

Two specific traps, both of which have actually happened:

- **Re-surfacing parked blockers is a reminder, never a justification.** The final report is required to list every gate again, because a gate the user never reads is a gate that never clears. Listing them next to a decision to stop makes already-parked items look like live obstacles holding up the loop. They are not — they are parked precisely so the loop can continue past them.
- **"This is unbounded" is not a finding.** A queue refilled by a concurrent worker, a backlog that outpaces you, a workstream that will not reach zero today — none of these is an exit condition. Note it in a checkpoint and take the next item.

=======
>>>>>>> Stashed changes
## Guardrails

- **Never ship red.** A failing test suite is a per-item block, not a footnote. `/ship`'s own gate backs this up — don't route around it.
- **Never ship half-work.** If an item can't be finished, it becomes **blocked** with the implementable part complete — not a partial "done".
- **Stay in `<wN>`.** Only touch items from the requested workstream. Workers are general-purpose, but this run is scoped to the queue the user named.
- **Deletion is a claim, not a shortcut.** Every deleted item costs you a sentence of evidence in the report. If you can't write that sentence, it isn't invalid.
- **Report honestly.** If you skipped a task, mocked something, or a suite was flaky, say so in that item's summary — don't present partial work as complete.
- **Never stop with workable items in the queue.** If a refresh returns open items, the user has not interrupted, and you can still do work, the run continues — whatever else is true about how long it has taken, how the backlog is trending, or how much you have to report. Ending a drain early with a full queue is the one failure this skill cannot recover from on its own, because the user has to notice and restart it.
