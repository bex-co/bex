---
description: Propose .pm milestones and tasks for a topic — decompose, size, and hand a text proposal to /pm
argument-hint: "<topic or goal to break down>"
allowed-tools: Read, Bash(ls:*), Bash(find:*), Bash(cat:*)
---

# Task: Propose milestones and tasks

`/pm-brainstorm` is the **divergent** half of the pair: it thinks a topic through and **proposes** work. It writes **nothing** — the output is a text proposal the user materializes with `/pm`. `$ARGUMENTS` is the topic/goal.

## The `.pm` hierarchy (what you are proposing into)

| Level | Path | Meaning | Effort |
| --- | --- | --- | --- |
| Workstream | `wN/` | a themed track / roadmap | — |
| Inbox note | `wN/NNN.md` | one idea or a **sub-hour** unit of work | ≤ ~1h |
| Milestone | `wN/mN/` | a shippable chunk (README + task files) | **> ~1h**, multiple tasks |
| Task | `wN/mN/tNNN.md` | a single unit | tens of minutes |

## Steps

1. **Load anti-goals first.** Read `.pm/DO_NOT_DO.md` and treat it as a hard constraint. If a proposed item conflicts, reject it explicitly and explain why.
2. **Load context.** Read the relevant `.pm` (workstream `README.md`s, open milestones, inbox notes — `find .pm -name README.md`, plus loose notes) so proposals fit the existing roadmap and reuse its numbering/naming. Pick the workstream the topic belongs to (or propose a new `wN`).
3. **Discuss & decompose.** Talk the topic through with the user: pressure-test scope, surface dependencies and risks, and break it into candidate tasks, each with a rough estimate (tens of minutes) and `depends_on` links.
4. **Apply the sizing rule** to each cluster of work — sum the task estimates:
   - **> ~1h across more than one task** → propose a **milestone**: a title, an ordered task table (`id | title | est | depends_on`), and a definition of done.
   - **≤ ~1h** → propose a **loose inbox note** instead, not a milestone. Say so explicitly (too small for a milestone).
5. **Quality gate for meaningful work.** For each milestone candidate, require all of:
   - direct goal linkage (which project goal/pillar it advances),
   - expected outcome (observable impact),
   - why it matters now (dependency/risk/sequence reason). If any are missing or weak, mark it **not meaningful**, do not propose it as a milestone, and suggest a better-scoped alternative.
6. **Emit the proposal as text only** — the target workstream, each proposed milestone (with task table + definition of done + source + goal linkage + expected outcome + why-now rationale) and/or inbox note. Do **not** write files.
7. **Hand off.** End by giving the exact `/pm` command(s) to materialize it, e.g.:
   - `/pm new milestone w1 <title>` (then the tasks), or
   - `/pm add w1 <idea>` for sub-hour work, or
   - `/pm promote w1/NNN` to promote an existing inbox note.

## Topic

$ARGUMENTS
