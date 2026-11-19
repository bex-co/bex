# w5 · m502 — A parked item wakes when production's pin carries the commit its gate names

**Worker:** worker5 **Goal:** work whose only remaining gate is a production deploy (or a date) resumes as soon as the gate clears, in every workstream, without a per-workstream routine or a person re-running `/loopx`. **Status:** todo (t001–t007 done; t008 closeout checks DoD 5 on the live board)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | One checkable gate form on the workstream line, enforced by `pm-board-validate.py`; migrate the release gates that name no commit — **DONE** | 1h30m | — |
| [t002](done/t002.md) | `loopx-guard.py board` reports `blocked.ready`: parked items whose gate commits the latest production pin carries — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Date gates join `blocked.ready` once their date has passed — **DONE** | 45m | t002 |
| [t004](done/t004.md) | `/loopx` takes `blocked.ready` before `next`, and a listening loop wakes when a pin makes an item ready — **DONE** | 45m | t002, t003 |
| [t005](done/t005.md) | Narrow w3's `pin-gated-recheck` routine to gates the guard cannot read — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t006 |
| [t008](t008.md) | Closeout | 15m | t007 |

Total: **8 tasks, about 5h45m**.

## Record (2026-11-19, t001–t007)

- **The forms (t001).** ``pin must carry `<sha>`[, `<sha>`…]`` and `date gate: YYYY-MM-DD[THH:MMZ]` inside the `**BLOCKED (…)**` summary, read by `scripts/pm_gates.py` and documented in the `/pm` skill (**A gate that clears by itself has one checkable form**), `worker.md`'s blocked outcome and `.pm/AGENTS.md`. `scripts/pm-board-validate.py` fails an unchecked BLOCKED line that waits for a production pin, deploy or roll without the pin form, or names a date gate without the date form, or names a commit the repository does not have.
- **The migration (t001).** 35 edits on 34 lines in 11 workstreams (w2, w3, w5, w6, w7, w8, w9, w10, w21, w22, w32), each commit taken from the item's own commits or README. Pure release gates got the pin form; date gates the date form (w21/091's date moved to 2026-11-23T14:30Z by its own rule: the first pin with `8227312bc` was written at 13:59Z, after T0). Gates that also need a person and whose pin part already held (w2/m477–m479, w7/m465, w7/m468, w10/m373, w10/m375, w10/m491, w10/m497) now record "production already carries `<sha>`", so they do not wake a worker for a gate only the user can clear; w10/m377's past "release gate cleared" became a record. w9/m495 already had the form.
- **The guard (t002–t004).** `board` lists `blocked.ready` (with `blocked.pin` and `blocked.problems`), keeps those items out of `blocked.review`, and makes the first one `next` after an own CI failure and before ready items. The pin is the newest commit whose subject is `chore(deploy): pin platform images to <sha>` on `origin/main`; a docs commit quoting that line in its body is not one. `dispatch` remembers the gate line of a parked item it dispatches, so a worker that parks it again unchanged is not offered again. `wait` exits 0 when a parked item's gate clears and never sleeps past the next date gate.
- **w3's routine (t005).** Removed: every release gate on the board is now in the pin form, which the guard reads, and the row left date and outside-event gates alone. `.pm/w3/routine/README.md` says so.
- **Simplify (t006).** The parked-item list reuses `classify` for its names and pick order; one repeated scan and an unused constant removed.
- **Tests (t007).** DoD 1: `GateTest.test_a_release_gate_is_ready_exactly_when_the_pin_carries_its_commit`, `test_every_named_commit_must_be_carried`, `test_an_unknown_commit_never_clears_and_is_reported`, `test_a_docs_commit_quoting_a_pin_subject_in_its_body_is_not_a_pin`. DoD 2: `test_a_date_gate_is_ready_only_once_its_date_has_passed`, `test_a_gate_with_a_pin_and_a_date_needs_both`. DoD 3: `next` is `w2/m3` before the ready `m4` in the first test, `test_wait_wakes_when_a_pin_clears_a_gate`, `test_wait_does_not_wake_for_a_gate_still_closed`, `test_a_dispatched_item_is_not_offered_again_until_its_gate_line_changes` (`scripts/test_loopx_guard.py`). DoD 4: `test_a_release_gate_that_names_no_commit_fails`, `test_a_release_gate_in_the_checkable_form_passes`, `test_a_pin_gate_naming_no_commit_here_fails`, `test_a_date_gate_needs_its_form_and_a_real_date`, `test_gates_that_need_a_person_and_closed_lines_are_free_text`, `test_head_reads_gates_from_the_committed_tree`, `test_the_repositorys_own_board_passes` (`scripts/test_pm_board_validate.py`). Every new test but the negative wait case fails against the pre-milestone scripts; removing `hand_over` fails the dispatch test, and counting a non-ancestor as carried fails both pin tests.
- **On the working tree before the ship (pin `dff52c596d13`):** 14 parked items in 10 workstreams are `blocked.ready`: w2/m480, w3/m489, w3/075, w5/m370, w6/m470, w6/m482, w6/m483, w6/m484, w9/m494, w10/m490, w21/083, w22/m314, w22/020, w32/044. Still gated: w3/063, w6/m512, w7/095, w7/m427, w7/m453 (pin holds, date 2026-11-21T19:00Z not), w8/339, w8/m504, w9/m448, w9/m495, w21/091, w21/107.

## Definition of done

1. With production's pin at a commit that contains every commit an item's gate names, `python3 scripts/loopx-guard.py board <wN>` lists that item under `blocked.ready`; with the pin behind any of them, it does not. Tests cover both.
2. An item with a date gate is listed under `blocked.ready` once the date has passed, and not before (test).
3. `/loopx` takes `blocked.ready` items before `next`, and a listening loop's `wait` returns when a new pin commit makes an item ready (test).
4. `python3 scripts/pm-board-validate.py` refuses a release or pin gate without the checkable form, and passes the board after the migration.
5. At closeout, on the live board, `board` lists every parked item whose gate commits the current pin carries (spot-checked for w5 and two other workstreams).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5`, round 10 item 2 (2026-11-18), selected with `/pmx them all to w5`.
- **Evidence (2026-11-19, production pin `7af50a28168d`):** of 180 parked items, 12 in 8 workstreams (w6 ×3, w8 ×2, w21 ×2, and one each in w2, w5, w9, w10 and w32) have release gates whose named commits the pin already carries, and nothing has resumed them. 20 more release gates name no commit on their workstream line; w5's m369, m371 and m372 are among them, and the pin carries the commits their READMEs name. Only w3 has a `pin-gated-recheck` routine (`.pm/w3/routine/README.md`). `blocked_review` (`scripts/loopx-guard.py`) wakes a parked item only when its files change, and `.claude/skills/loopx/SKILL.md` says a gate that clears without a board change "wakes nothing".
- **Goal linkage:** the autonomous delivery loop (`/loopx`, `/pm`) that ships all board work.
- **Expected outcome:** finished work reaches its definition of done within one listen interval of the deploy that carries it.
- **Why now:** parked items grow every day, and each idle one is shipped code whose live acceptance nobody has checked.
- **Render parity:** omitted. Internal tooling.
