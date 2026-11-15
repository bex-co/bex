# `/loopx` worker: take one item to an outcome

You were dispatched by a `/loopx <wN>` orchestrator (see [`SKILL.md`](SKILL.md)) to handle **exactly one** item: an open milestone `wN/mN/` (with all of its tasks, closing tasks included) or one open inbox note `wN/NNN.md`. If the dispatch says you are a _routine worker_ instead, skip to [Routine runs](#routine-runs): you run one scheduled command, not an item. If it says you are a _CI-fix worker_, skip to [CI fix runs](#ci-fix-runs). Your context is fresh on purpose. Everything you need is on disk: the item's files under `.pm/<wN>/`, `.pm/DO_NOT_DO.md`, the root and scoped `AGENTS.md` files, the board conventions in [`.claude/skills/pm/SKILL.md`](../pm/SKILL.md), the `## Local dev environment` section at the top of `.pm/<wN>/README.md` (this workstream's own `dev-N` stack), and the code. Do not rely on anything else.

Your job ends when the item has reached exactly one outcome and that outcome is on `origin/main`: worked and moved to `done/`, closed as already met, parked in `blocked/` with its gate written down, deleted as invalid, or, for a note too large for one sitting, promoted to a milestone. Then return the [result block](#result-block).

**Scope**

- Do not pick another item. Do not touch other items' board entries except to record your own verdict on this one.
- Do not run any `scripts/loopx-guard.py` mode other than `outcome` (`start`, `end`, `board`, `wait`, `dispatch`, `settle`, `routine-due`, `routine-done` belong to the orchestrator).
- Run your checks in the foreground and wait for them, with a long timeout where they need one (`/ship`'s gates can take 30 minutes). Never end your turn while a command of yours is still running in the background: the orchestrator would be woken by a result that is not one.
- Load `/ship` and read the `/pm` skill once each. After that, follow the instructions already in your context instead of loading them again.
- Never ask the user, and never wait for a confirmation. A question you cannot settle from evidence is a **block**, with exactly what is needed from whom as its gate.
- Work in this checkout. Do not create a git worktree for the item or for a subagent: `/ship` pushes `main` from here.
- **You share this working tree with teammate sessions.**
  - Start from `main`. Uncommitted `.pm` board work you did not write is a teammate's filing, not a problem. If your item is itself such a filing, or one is still uncommitted when you land your outcome, commit it first as its own `docs(pm): file <items>` commit, separate from yours, so authorship stays legible. Never edit the content of an item you did not file except to record your verdict on it.
  - Before each board write, re-read the file you are about to edit (it may have changed since you last read it) and apply the smallest edit that lands your verdict.
  - Foreign **source** changes (uncommitted edits under `lego/`, `dashboard/`, `infra/`, `scripts/` that you did not make and no `.pm` item explains) stay where they are: never stage, stash, or discard them, and stage your own files by name, never `git add -A`. Only when they make your change unshippable is the item a block; name them in `notes`.

An open item is a claim that the work is still wanted and still undone. Boards go stale: work lands through another item or a direct commit, a decision or a `DO_NOT_DO.md` rule retires the idea, the surface it targets gets removed or renamed. Never start implementing on the strength of a checkbox alone. Every item earns its implementation by passing triage first.

## 1. Triage the item

Read it in full: a milestone's `README.md` plus every task file `tNNN.md` not already in its `done/`, or the note itself. Then check its premise against the current codebase (the note may be weeks old and the bug already fixed). If the item's checkbox and its on-disk state disagree, trust the files and repair the drift as you land the outcome.

Items restored by [`/unblock`](../unblock/SKILL.md) carry dated decisions and next actions on the board. Read and apply those decisions; verify their factual prerequisites without asking the user to decide the same thing again. A genuinely new blocker still gets its own precise gate.

Pick exactly one verdict, and write it down with its evidence in a few lines before acting on it:

| Verdict | When | Action | Result `outcome` |
| --- | --- | --- | --- |
| **Work** | The premise holds and nothing external gates it. Stale details (moved paths, renamed commands) do not change this: follow the intent and note the deviations. | step 2 → 3 (**done**) → 4 | `shipped` |
| **Already met** | The described end state is **already true in the code**. Verify it, don't assume: run the definition of done's checks, reproduce a bug note's repro. Missing tests or an unchecked parity surface make this _work_, not a close. | step 3 (**done** + evidence) → 4 | `closed` |
| **Blocked** | A gate you cannot clear: physical hardware, a credential or console only the user has, an upstream item elsewhere, or a decision only the user can make. | finish what is implementable, then step 3 (**blocked**) → 4 | `blocked` |
| **Invalid** | The premise is false, the work duplicates a shipped or live item, or it conflicts with `.pm/DO_NOT_DO.md`. | step 3 (**delete**) → 4 | `dropped` |
| **Promote** | An inbox note that turns out to be more than ~1h across more than one task. | `/pm promote <wN/NNN>`, ship that board change alone (step 4), and stop. The orchestrator picks the new milestone with a fresh worker. | `promoted` |

Rules for triage:

- **Evidence, not vibes.** "Already met" and "invalid" both require a concrete pointer: `file.go:123`, a passing test, the surviving duplicate's id, the `DO_NOT_DO` line.
- **Render parity work is never blocked on an optional decision** ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first)).
  - If a pending user decision is not required to reach Render's behavior in every workspace, it is not a gate. Ship the parity-sufficient option, and file the open variant as its own item.
  - Example: m149's seccomp choice did not block `render-v1` under RuntimeDefault.
  - An older ADR that conflicts with parity is likewise not a gate. Amend it as part of the work.
- **Cloud coding-agent work matches Devin the same way** (user preference 2026-10-30; [ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)). For agent sessions, `/agents`, sandboxes and PR delivery, Devin's observable behavior is the reference, and the same rules apply: ship the Devin-consistent option instead of waiting on an optional decision, and file variants separately. Whenever an item leaves a design choice open, pick the option that matches the reference product (render.com for hosting, Devin for coding agents). Verify how that product actually behaves before picking, and cite it in the item.
- **When torn between blocked and invalid, choose blocked.** Deleting is the only outcome that destroys information. An item the user explicitly routed or approved that you merely _suspect_ is unwanted is parked with the question as its gate, never deleted.
- **Non-implementation notes have preset verdicts. Apply them without asking.** Many inbox notes are not "build this"; they are records, withdrawals, or debris. Route them by shape, in this order (first match wins):

  | Shape | Verdict | Landing |
  | --- | --- | --- |
  | **Blank or stub**: no `Why:` line and no substantive body, or a placeholder never filled in | **invalid** | `git rm`, drop its README line |
  | **Duplicate with no unique content**: the surviving item covers it entirely and this note adds no findings of its own | **invalid** | `git rm`, cite the survivor's id |
  | **Duplicate or withdrawal that carries its own findings or history** | **done** | `done/`, append the survivor's id + date; never `git rm` history |
  | **Record-only**: a coverage record, sweep log, or audit trail documenting work already performed, with nothing left to implement | **done** | `done/`, append one line dating the closure |
  | **Already fixed by someone else**: the premise held when filed but a commit since resolved it | **done** | `done/`, append the fixing commit SHA |

  If a note matches none of the shapes and none of the verdicts, prefer **blocked** with the ambiguity named. Parking is always available and never destroys information.

- **Never split a milestone's verdict.** If some tasks are implementable and one is gated, the milestone is **blocked**: finish every implementable task first, then park it (the `w11` pattern: implemented tasks done, the gated task named).

The last tasks of every milestone are the standing closing tasks defined in the `/pm` skill: Render parity when present, Simplify, Test coverage, Closeout. They are real work, not bookkeeping. Run `/simplify` for Simplify, write meaningful tests for Test coverage, and walk every surface the Render parity task names.

## 2. Implement it (Work verdict only)

Do the actual engineering, task by task, in `depends_on` order:

- Follow all `AGENTS.md` rules, root and the scoped guide of every module you touch (id minting, boilerplate headers, `.env.example` sync, prettier on markdown, skill layout, etc.).
- Milestones ship features **end to end**: include the frontend tasks alongside the backend ones; do not stop at the API.
- Run the suites the change touches and make them pass before considering a task done. Never mark a task complete on unverified code.
  - `lego/operator/`, `lego/types/` → `make test` (from `lego/operator/`)
  - `lego/backend/` → `cd lego/backend && go test ./...`
  - `lego/cli/` → `cd lego/cli && go test ./...`
  - any Go module → `make lint` (from `lego/operator/`)
  - `dashboard/` → `yarn lint` and `yarn test`
  - `scripts/` → the script's own `*.test.sh` or `test_*.py`
  - skills → `bash scripts/skill-layout-validate.sh`
  - markdown → `npx prettier@3.4.2 --write` on the files you touched
- A bug note is not fixed until its own repro fails to reproduce and a regression test covers it. Prove the test is real: revert the fix and watch it fail.
- When a definition of done needs a signed-in check on production (`dashboard.bex.co` / `api.bex.co`), sign in as the QA user with `bash scripts/qa-login.sh`. It reads `QA_EMAIL`/`QA_PASSWORD` from `.env` or `muse.env` itself, so never read those files or type the password; follow the login and logout steps in `.claude/skills/qa-find-bugs/SKILL.md`. Only exit 2 (credentials missing in both files) is a blocked gate.
- You may delegate independent sub-tasks to subagents to parallelize, but you own correctness, and they work in this checkout too.
- Keep `.pm` status in sync as you finish tasks (task frontmatter, milestone README `**Status:**` + the `— DONE` row, workstream checkbox). These are `/pm`-governed writes: follow `.claude/skills/pm/SKILL.md` exactly, `/pm done <wN/mN/tNNN>` for each finished task.

## 3. Land the outcome

Every item ends in exactly one of these on-disk states, applied as soon as the verdict is reached, with no confirmation. Leave **no tombstone, stub, or redirect** at an item's old path, and rewrite the links that pointed at it (the `/pm` skill's **Moves rewrite inbound links**; `bash scripts/docs-links-validate.sh` must pass).

**done**: the work is real and verified.

- Task: `status: done`, row `— **DONE**`, file → `wN/mN/done/tNNN.md`.
- Milestone with no open tasks: `mv` the whole directory → `wN/done/mN/`, `rmdir` the original, flip the workstream checkbox to `- [x]`. Do not close out until the milestone's definition of done actually holds. If it cannot hold here, the milestone is **blocked**, not done.
- Inbox note: `mv` → `wN/done/NNN.md`.
- For an **already met** item, append one line to the moved item recording the evidence and the date, so the record says why it closed without a code change.

**blocked**: implementable work finished, a named gate remains.

- `mv` the milestone directory → `wN/blocked/mN/` (or the note → `wN/blocked/NNN.md`).
- Leave the workstream README line unchecked (`- [ ]`) and rewrite it as `**BLOCKED (<exactly what you need from whom>)**` with the path updated to `blocked/…`, plus which tasks are already done. See `.pm/w11/README.md` for the shape. A gate that clears by itself (a production deploy carrying your commit) says so, with the check that proves it.
- Preserve partial code that cannot ship: push nothing half-done, and keep it in a named stash (`git stash push -u -m "loopx/<wN>/<id>" -- <your paths>`). Name the stash in `notes`.
- Point to `/unblock <wN>` when the gate needs the user.

**delete**: the item is invalid.

- `git rm` the file or directory and remove its line from the workstream `README.md`.
- Never delete an item that contains completed task records: move it to `done/` instead so the history survives. If the reason generalizes into a rule, propose a `DO_NOT_DO.md` entry in `notes`; do not write one yourself.

Then run `npx prettier@3.4.2 --write` on the markdown you touched outside `.pm/`.

## 4. Ship it

Invoke **`/ship`** ([`.claude/skills/ship/SKILL.md`](../ship/SKILL.md)). Because you made the changes this session, `/ship` runs session-aware: it stages exactly what you touched and writes the commit message from your knowledge.

- **One outcome per ship.** Code and the item's own board move ship together; an outcome with no code (already met, blocked, deleted, promoted) ships alone as `docs(pm): <close|park|drop|promote> <target> — <one line>`. Never fold two items into one commit. A milestone large enough to need it may ship at task boundaries, provided every ship passes its gates and is self-contained.
- `/ship` ends at a successful push. It does not watch CI or the deploy, and neither do you: the orchestrator reads CI for your commits afterwards, and a workflow your push turns red comes back to the loop as a fix.
- If `/ship` surfaces a failure it cannot fix (a rebase conflict that needs the user, a rejected push), that is a block: report `outcome: blocked` with the failure in `notes`, and leave the board change unshipped in a named stash.

What stops a ship is the push's **own** defect: `/ship`'s real-DB backend suite or dashboard checks failing on your change. Fix that properly before pushing; it is never waived, and never worked around with `--no-verify` or by deleting, skipping, or weakening a check.

### Main is red: fix CI first, then resume

A red main is never a reason to stop or to hold commits locally (user decision 2026-10-21; `AGENTS.md` Rules). `/ship`'s gate only warns about it. Unpushed commits in a checkout other sessions share look like unexplained divergence, and a teammate's `git reset` can discard them, so the paused item ships in the same run either way:

1. **Pause the item and diagnose the red run.** Commit the item locally. Read the failing job's log (`gh run view <id> --log-failed`) and compare it with the last green run (`gh run list --workflow deploy.yml`), including what changed between the two SHAs. Also check whether main already contains a fix: a red verdict can be stale when every later run was cancelled.
2. **Fix it if you can, whatever the cause.** Fixing main is in scope for every workstream.
   - **Code or test failure** (yours or anyone's): make the smallest correct fix, with a regression test where one fits. Ship it alone as `fix(ci): …`, saying in the commit that it fixes red main, with `--fixes-red-main`.
   - **Flake or infra fault** (runner crash, emulation SIGSEGV, network, setup failure): rerun only the failed jobs (`gh run rerun <id> --failed`) in the background, and **do not wait for it**. If the same failure has repeated, land a repo-side root-cause fix: a retry, a pinned toolchain or runner setting, or splitting or caching the failing step. Ship that as `fix(ci): …` with `--fixes-red-main`. Never fix it by weakening a gate, deleting a test, skipping a job or reverting runners to GitHub-hosted (`DO_NOT_DO.md` #CI-RUNNERS).
3. **Resume immediately.** Ship the paused item's commits through `/ship` right after the fix is pushed. Do not wait for the fix's own CI run.
4. **If it cannot be fixed now, ship anyway.** That covers a fix that needs credentials, runner or host access, or a decision only the user has; a stale red that has nothing left to fix; or a flake still in flight. Ship the paused item's commits without waiting, and record the red run, its cause and what you tried in `notes`.

List a `fix(ci)` commit in `commits` like any other, so the orchestrator watches its CI too.

## Guardrails

- **Triage before code.** Write down an evidence-backed verdict before changing anything.
- **Never ship your own red.** Run the suites as their own command and read the result before you commit. Chaining `&& git commit` onto the end of a test command is exactly how a red push happens. `/ship`'s gate backs this up; don't route around it.
- **Never ship half-work.** If an item can't be finished, it becomes **blocked** with the implementable part complete, not a partial "done".
- **Deletion is a claim, not a shortcut.** Every deleted item costs you a sentence of evidence. If you can't write that sentence, it isn't invalid. Never delete an item to avoid the work in it.
- **The tree is shared.** Stage by name; a teammate's files are never yours to stage, stash, or discard.
- **No evidence dumps in git.** Raw run output stays out of `.pm/` (`.pm/AGENTS.md`); write the conclusion and how to reproduce it into the item.
- **Report honestly.** If you skipped a task, mocked something, or a suite was flaky, say so in `notes`. Do not present partial work as complete, or a close-by-triage as work you did. The orchestrator checks your claim against `origin/main`.

## Routine runs

When the dispatch says you are a **routine worker**, there is no board item. You run the one command the dispatch quotes, as written, and nothing else. The command comes from a row in `.pm/<wN>/routine/README.md` that the user scheduled through `/pm routine`; running it under `/loopx` is the explicit request to run it and to ship what it produces. Limits written into the command itself (`DRY_RUN=1`, audit-only, no-ship) still hold. It can be anything:

- **A slash command** (`/rt-remove-dead-code go`, `/qa-find-bugs-cli w4`, `/simplify`): invoke that skill with exactly those arguments and follow its `SKILL.md`, including any contract it tells you to read (proof standards, verify gates, budgets, STOPs, where its findings go). Where the skill is stricter than the rules below, the skill wins.
- **A shell command**: run it from the repository root, read its output, and act on what it reports within the command's evident purpose (fix what it flags, or surface it as follow-ups). Do not stretch it into unrelated work.
- **A task in plain words**: do that task, scoped as narrowly as it reads.

The triage and landing sections above do not apply, except where the task itself is about board items. Whatever the command is:

- Run the checks for every module you change (step 2's list) and ship each change through `/ship` ([Main is red](#main-is-red-fix-ci-first-then-resume) applies). Never ship your own red.
- Write the board only when the command itself does so (a QA skill filing its findings through `/pm`, a sweep of parked items). List the item ids it filed, extended, or moved in `notes`.
- Anything else that needs board work goes into `followups`: a STOP that needs a person, a finding outside the command's scope, a candidate too large for one run. The orchestrator files each one as an inbox note. Write each one so it stands alone: the problem, the evidence, the paths, and a testable end state.
- `result: found` means the run shipped at least one commit (code, or board items filed or extended). `empty` means it ran and had nothing to ship. `failed` means the command could not run at all: missing credentials, a tool or service that is down, a skill that refuses to start. Say why in `notes`, and do not retry in a loop.
- Leave nothing of yours uncommitted and every shipped commit on `origin/main`. A half-done change goes into a named stash (`git stash push -u -m "loopx/routine/<name>" -- <your paths>`), never into a ship.

Your final message is this block instead of LOOPX-RESULT, and nothing else:

```
LOOPX-ROUTINE
routine: <name>
result: found|empty|failed
commits: <short SHA, comma-separated; - if none>
followups:
- <one line per follow-up: problem, evidence, paths, testable end state; or - if none>
notes: <at most two lines: skipped candidates and why, flakes, stash name>
```

## CI fix runs

When the dispatch says you are a **CI-fix worker**, there is no board item either. A workflow on `main` went red on a commit this loop pushed, and the dispatch gives you the workflow, that commit, and the run URL. Your one job is to turn that workflow green again:

- Read the failure first: `gh run view <run id> --log-failed`, where the run id is the last part of the URL. Check whether main already contains a fix. Then reproduce it locally when the environment allows. Some jobs cannot be reproduced on this machine (another architecture, the real cluster, a built image): work from the log.
- Fix the cause in the code or the test that is wrong, with a regression test where one fits. Never delete, skip, or weaken the failing check, never revert runners to GitHub-hosted, and never revert someone else's commit to get to green.
- Run the checks for every module you change (step 2's list), then ship the fix alone through `/ship` as `fix(ci): …` with `--fixes-red-main`. Do not wait for CI: the orchestrator reads the result of your push and comes back if it is still red.
- A flake or infra fault has no code to fix on the first occurrence: rerun the failed jobs in the background (`gh run rerun <id> --failed`), do not wait, and report `outcome: shipped` with no commits and the rerun in `notes`. If it has repeated, land the repo-side root-cause fix instead.
- If the cause is outside this repository (an expired secret, runner or host access, an upstream release, a decision only the user has), there is nothing to fix here. File one inbox note with `/pm add` in the dispatching workstream, with the run URL and what a person has to do, ship it, and report `outcome: blocked`.

End with the LOOPX-RESULT block, with `item: <the label from the dispatch, e.g. ci:test-backend>` and `outcome: shipped` or `blocked`.

## Result block

Your final message is exactly this block and nothing else. Everything worth keeping is already in the commits and on the board:

```
LOOPX-RESULT
item: <wN>/<id>
outcome: shipped|closed|blocked|dropped|promoted
commits: <short SHA, comma-separated; - if none>
gate: <exactly what is needed from whom; - unless blocked>
promoted_to: <the new milestone, e.g. w4/m415; - unless promoted>
notes: <at most two lines: deviations, flakes, skipped steps, stash name, a red main you met, proposed DO_NOT_DO rule>
```
