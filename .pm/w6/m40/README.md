# w6 · m40 — ADR registry integrity: one number, one document, enforced

**Worker:** worker6 **Goal:** every `ADRnnn` citation in the repo resolves to exactly one document, every ADR is discoverable from `CLAUDE.md`'s index, and CI refuses to let a fourth collision appear **Status:** todo

## Tasks (in order)

| id   | title                                                                       | est | depends_on               |
| ---- | --------------------------------------------------------------------------- | --- | ------------------------ |
| t001 | Settle the renumbering for the three collided ADRs                            | 25m | —                        |
| t002 | Rewrite every cross-reference to the renumbered ADRs                          | 45m | w6/m40/t001              |
| t003 | Index the three unindexed ADRs in `CLAUDE.md`                                 | 25m | w6/m40/t001              |
| t004 | Fail-closed CI guard: duplicate number, missing index entry, dangling link    | 45m | w6/m40/t002, w6/m40/t003 |
| t005 | Simplify the code this milestone changed                                      | 15m | w6/m40/t004              |
| t006 | Test coverage: the guard's red/green self-test                                | 30m | w6/m40/t004              |
| t007 | Closeout                                                                      | 10m | w6/m40/t006              |

## Definition of done

`ls docs/ADR*.md | sed 's/.*ADR\([0-9]*\).*/\1/' | sort | uniq -d` returns **nothing**. Every `docs/ADR*.md` appears in `CLAUDE.md`'s docs index. No `ADRnnn-…` reference anywhere in the repo (`docs/`, `.pm/`, `CLAUDE.md`, `scripts/`, code comments) resolves to a missing or ambiguous file. The CI guard turns red on a reintroduced duplicate number, on an ADR absent from the index, and on a dangling ADR link — each proved by its own red/green self-test — and each renamed file records its old→new number in its header so existing external citations stay traceable.

## The defect (measured at HEAD, 2026-08-17)

71 ADRs in `docs/`. **Three numbers are claimed by two files each:**

| number | file A                                               | file B                             |
| ------ | ---------------------------------------------------- | ---------------------------------- |
| ADR040 | `ADR040-billing-metronome.md`                        | `ADR040-openchoreo-evaluation.md`  |
| ADR049 | `ADR049-render-yaml-parity.md`                       | `ADR049-tenant-billing-credits.md` |
| ADR060 | `ADR060-build-worker-reliability-and-performance.md` | `ADR060-security-review-round7.md` |

**Three ADRs are absent from `CLAUDE.md`'s docs index:** `ADR030-pricing.md`, `ADR033-workflows.md`, `ADR040-openchoreo-evaluation.md`.

## Source + Goal linkage

- **Source:** measured against HEAD by `/pm-brainstorm` 2026-08-17 (`for w6 to work on`, second round). Not a report inherited from elsewhere — the collision and index counts above were enumerated directly.
- **Goal linkage:** [`GOAL.md`](../../GOAL.md) #7 (security review). The ADR028 → ADR045 → ADR055 → ADR056 → ADR057 → ADR060 → ADR061 → ADR063 → ADR064 → ADR066 → ADR067 → ADR068 security-review lineage is a **thirteen-document chain navigated entirely by number**, and it is the chain that generated two of the three collisions. ADR numbers are also the citation system `CLAUDE.md`, `DO_NOT_DO.md`, and every milestone README use to point at architectural decisions.
- **Expected outcome:** an `ADRnnn` citation becomes unambiguous again and stays that way. Today `docs/ADR068-security-review-round13.md` cites "ADR060 D5" meaning the build-worker ADR while `ADR060-security-review-round7.md` also exists, and the parity ledger's "ADR049 fail-closed" sits alongside an unrelated ADR049 on tenant billing credits.
- **Why now:** the collision count is growing, and each one was **created by a well-intentioned rename meant to fix an earlier collision** — `ADR060-security-review-round7.md` records "Renamed from ADR058 to resolve the number collision with ADR058-release-engineering", and ADR063/ADR065 carry the same rename history. The process reliably reintroduces the bug, which is exactly why `t004`'s guard, not the renumbering, is the durable half. The work is purely mechanical with zero runtime risk, so it is cheapest to do before the next security round adds a fourteenth link to the chain.
- **Render parity task omitted:** documentation and CI tooling only. No REST, GraphQL, MCP, or dashboard surface is touched.
