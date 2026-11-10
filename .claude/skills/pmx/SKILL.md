---
name: pmx
description: >-
  Materialize a /pm-brainstorm proposal onto a .pm workstream, ship the filing, then drain that workstream with /loopx. Use when the user invokes /pmx with a target workstream (e.g. `/pmx them all to w3`, `/pmx 1,3 to w5`) or says "/pm them all to wN and then /loopx wN".
---

# Task: File a proposal, ship it, drain the workstream

`/pmx [<selection>] to <wN>` runs three steps back to back, without pausing between them: `/pm <selection> to <wN>` → `/ship` the filing → `/loopx <wN>`.

## Arguments

Parse from `$ARGUMENTS`:

- **`<wN>`** (required) — the last `wN` token. If there is none, **STOP** and ask which workstream; never guess.
- **`<selection>`** — `them all`, `all` or empty selects every numbered item in the proposal's **Summary (priority order)** plus the existing-item actions in its `/pm` hand-off (e.g. `/pm unblock …` of items whose gate cleared). Item numbers (`1,3`, `2-4`) select only those items; a hand-off action is included only when named.

## 1. Find the proposal

- Use the most recent `/pm-brainstorm` proposal in this conversation: its detail sections (task tables, definition of done, source, goal linkage, expected outcome, why now, Render-parity flag), its Summary and its `/pm` hand-off commands.
- If the conversation has none, run `/pm-brainstorm for <wN>` first and use what it proposes.
- `<wN>` from the arguments wins over the workstream the proposal named.

## 2. Materialize with `/pm`

[`.claude/skills/pm/SKILL.md`](../pm/SKILL.md) is canonical — read it and `.pm/DO_NOT_DO.md`, and apply their rules exactly; do not restate or diverge from them here.

1. **Sync first.** On `main`, `git fetch origin main` and `git pull --ff-only origin main`. If the fast-forward is refused, keep the local tree and let step 3's `/ship` rebase. Then rescan numbering (next free `mN` across `.pm`, next free `NNN` in `<wN>`): the proposal's numbers are provisional.
2. **Re-check each selected item at write time.** Another session may have filed or shipped it since the proposal, or it may now conflict with `DO_NOT_DO.md`. Skip such an item and report why; never file it twice.
3. **Run the matching `/pm` subcommand per item, in priority order:** `new milestone` (the proposal's implementation tasks with their titles, estimates and `depends_on`; task files filled from the proposal's evidence — concrete paths, steps, acceptance criteria; then the standing closing tasks, with Render parity included or omitted as the proposal flagged), `add` for inbox notes, `promote` for an existing note, `unblock` for a recorded gate-cleared finding.
4. **Record decision state honestly.** Selecting an item is approval to schedule it, not sign-off on a divergence. Write the proposal's recommendation for an open decision as the proposal's recommendation and the rule it rests on (e.g. ADR008 Render parity first), never as a user decision. Where the repository rules make that recommendation the default (parity work is never parked on an optional decision), the tasks implement it and name the still-open sign-off in the milestone's `## Source + Goal linkage`.
5. Run `npx prettier@3.4.2 --write "**/*.md"`, then verify task ids match paths, README lines and `(N tasks)` counts, links and `depends_on`.

## 3. Ship the filing

Invoke `/ship` for this board-only change, staging only the files step 2 wrote: one `docs(pm): file <ids> (from /pm-brainstorm for <wN>)` commit. Invoking `/pmx` is the user's explicit request to ship this filing and the items `/loopx` lands. Shipping before the drain puts the items on `origin/main` where other sessions see them, and keeps `/loopx`'s per-item commits free of filing noise.

If `/ship` stops on something it cannot resolve, stop here and report: the filing is a run-level block, as in `/loopx`.

## 4. Drain

Post a short checkpoint (what was filed, what was skipped and why), then invoke `/loopx <wN>` in the same turn and follow it to its own **Exit**. Its queue is every open item in `<wN>`, not only the ones just filed — that is intended.

## Report

The final report is `/loopx`'s report, headed by the filing: the new ids with their titles, the skipped items with their reasons, and every recommendation recorded in place of an open user decision.
