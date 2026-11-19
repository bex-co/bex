# w5 · m502 — A parked item wakes when production's pin carries the commit its gate names

**Worker:** worker5 **Goal:** work whose only remaining gate is a production deploy (or a date) resumes as soon as the gate clears, in every workstream, without a per-workstream routine or a person re-running `/loopx`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | One checkable gate form on the workstream line, enforced by `pm-board-validate.py`; migrate the release gates that name no commit | 1h30m | — |
| [t002](t002.md) | `loopx-guard.py board` reports `blocked.ready`: parked items whose gate commits the latest production pin carries | 1h30m | t001 |
| [t003](t003.md) | Date gates join `blocked.ready` once their date has passed | 45m | t002 |
| [t004](t004.md) | `/loopx` takes `blocked.ready` before `next`, and a listening loop wakes when a pin makes an item ready | 45m | t002, t003 |
| [t005](t005.md) | Narrow w3's `pin-gated-recheck` routine to gates the guard cannot read | 15m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t006 |
| [t008](t008.md) | Closeout | 15m | t007 |

Total: **8 tasks, about 5h45m**.

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
