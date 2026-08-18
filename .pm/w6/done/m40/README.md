# w6 · m40 — ADR registry integrity: one number, one document, enforced

**Worker:** worker6 **Goal:** every `ADRnnn` citation in the repo resolves to exactly one document, every ADR is discoverable from `CLAUDE.md`'s index, and CI refuses to let a fourth collision appear **Status:** done

## Tasks (in order)

| id   | title                                                                       | est | depends_on               |
| ---- | --------------------------------------------------------------------------- | --- | ------------------------ |
| t001 | Settle the renumbering for the three collided ADRs — **DONE**                            | 25m | —                        |
| t002 | Rewrite every cross-reference to the renumbered ADRs — **DONE**                          | 45m | w6/m40/t001              |
| t003 | Index the three unindexed ADRs in `CLAUDE.md` — **DONE**                                 | 25m | w6/m40/t001              |
| t004 | Fail-closed CI guard: duplicate number, missing index entry, dangling link — **DONE**    | 45m | w6/m40/t002, w6/m40/t003 |
| t005 | Simplify the code this milestone changed — **DONE**                                      | 15m | w6/m40/t004              |
| t006 | Test coverage: the guard's red/green self-test — **DONE**                                | 30m | w6/m40/t004              |
| t007 | Closeout — **DONE**                                                                      | 10m | w6/m40/t006              |

## Definition of done

`ls docs/ADR*.md | sed 's/.*ADR\([0-9]*\).*/\1/' | sort | uniq -d` returns **nothing**. Every `docs/ADR*.md` appears in `CLAUDE.md`'s docs index. No `ADRnnn-…` reference anywhere in the repo (`docs/`, `.pm/`, `CLAUDE.md`, `scripts/`, code comments) resolves to a missing or ambiguous file. The CI guard turns red on a reintroduced duplicate number, on an ADR absent from the index, and on a dangling ADR link — each proved by its own red/green self-test — and each renamed file records its old→new number in its header so existing external citations stay traceable.

## The defect (measured at HEAD, 2026-08-17) — RESOLVED

72 ADRs in `docs/`. **Three numbers were claimed by two files each**, and in every case the number stayed with the more-cited document (counts are filename references across `docs/`, `.pm/`, `CLAUDE.md`, `scripts/`, and Go source):

| number | kept it                                                  | refs | renumbered to                             | refs |
| ------ | -------------------------------------------------------- | ---- | ----------------------------------------- | ---- |
| ADR040 | `ADR040-billing-metronome.md`                            | 47   | **ADR070**`-openchoreo-evaluation.md`     | 10   |
| ADR049 | `ADR049-render-yaml-parity.md`                           | 14   | **ADR071**`-tenant-billing-credits.md`    | 4    |
| ADR060 | `ADR060-build-worker-reliability-and-performance.md`     | 24   | **ADR072**`-security-review-round7.md`    | 9    |

**Three ADRs were absent from `CLAUDE.md`'s docs index** — `ADR030-pricing.md`, `ADR033-workflows.md`, and the OpenChoreo evaluation — all three now indexed.

**Security-lineage note.** Moving round 7 to ADR072 places it numerically after round 13, which looks wrong until you notice the chain was never numerically ordered: it already skips 062 and 065 (taken by ADR062-sandbox-credential-vault and ADR065-agent-session-archive). Each round names its predecessors explicitly and each filename carries its `roundN`, so the ordering signal is intact. Keeping the number on round 7 instead would have cost ~3× the rewrites and rotted every `ADR060 D1`–`D8` citation the build-worker rollout depends on.

## Source + Goal linkage

- **Source:** measured against HEAD by `/pm-brainstorm` 2026-08-17 (`for w6 to work on`, second round). Not a report inherited from elsewhere — the collision and index counts above were enumerated directly.
- **Goal linkage:** [`GOAL.md`](../../GOAL.md) #7 (security review). The ADR028 → ADR045 → ADR055 → ADR056 → ADR057 → ADR072 → ADR061 → ADR063 → ADR064 → ADR066 → ADR067 → ADR068 security-review lineage is a **thirteen-document chain navigated entirely by number**, and it is the chain that generated two of the three collisions. ADR numbers are also the citation system `CLAUDE.md`, `DO_NOT_DO.md`, and every milestone README use to point at architectural decisions.
- **Expected outcome:** an `ADRnnn` citation becomes unambiguous again and stays that way. Today `docs/ADR068-security-review-round13.md` cites "ADR060 D5" meaning the build-worker ADR while `ADR072-security-review-round7.md` also exists, and the parity ledger's "ADR049 fail-closed" sits alongside an unrelated ADR049 on tenant billing credits.
- **Why now:** the collision count is growing, and each one was **created by a well-intentioned rename meant to fix an earlier collision** — `ADR072-security-review-round7.md` records "Renamed from ADR058 to resolve the number collision with ADR058-release-engineering", and ADR063/ADR065 carry the same rename history. The process reliably reintroduces the bug, which is exactly why `t004`'s guard, not the renumbering, is the durable half. The work is purely mechanical with zero runtime risk, so it is cheapest to do before the next security round adds a fourteenth link to the chain.
- **Render parity task omitted:** documentation and CI tooling only. No REST, GraphQL, MCP, or dashboard surface is touched.
