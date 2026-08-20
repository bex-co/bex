# w10 · m10 — Board & docs truth sweep round 3: w5 m71 collision, FUTURE-MAYBE & ADR018 sync

**Worker:** worker10 **Goal:** the verified board/docs drifts from the 2026-08-18 audit read true again — above all the w5 m71 number collision that hides a genuinely-open milestone from every board scan **Status:** done

**Resolution (2026-08-19):** between this milestone's scoping (2026-08-18) and implementation start, unrelated concurrent sessions ("2026-08-18 triage" + the w5/m74 closeout) had already fixed three of the four findings — verified against current `main` before touching anything, per the same duplication check m9 needed: the w5 m71 collision was already repaired (renumbered to `w5/m73`, shipped, done), `w5/m66`'s t003 row already carries its `— **DONE**` marker, and `w1/049`/`w1/050` are both already closed (not merely relisted — genuinely resolved, so the original "list them in the inbox" ask no longer applies). t002 (FUTURE-MAYBE) and half of t003 (the two ADR018 pointers) were still genuinely open and are fixed here. t004's original targets (w1 049/050, w5 036–040) turned out already accurate or moot, but the same re-verification pass found a fresh, real gap in the same class — `w5/046.md` (filed by the just-landed `w5/m74` closeout) was missing from `w5/README.md`'s inbox section — added instead. `npx prettier@3.4.2 --check` clean on every touched file.

## Tasks (in order)

| id   | title                                                                       | est | depends_on       |     |
| ---- | ---------------------------------------------------------------------------- | --- | ---------------- | --- |
| t001 | Repair the w5 m71 number collision (renumber the open milestone, add its row) | 45m | —                | — **DONE** (found already fixed by another session; verified, no change needed) |
| t002 | FUTURE-MAYBE: move the replica-local-buckets entry to Done (shipped as w1/m58) | 10m | —                | — **DONE** |
| t003 | Pointer fixes: ADR018 :286 done-path + :306 tense, w5/m66 t003 DONE marker    | 20m | t001             | — **DONE** (w5/m66 marker found already fixed; the two ADR018 pointers were real and fixed here) |
| t004 | README inbox-section sync: w1 notes 049/050, w5 notes 036–040                 | 20m | t001             | — **DONE** (original targets found already closed/accurate; added the newly-discovered `w5/046` gap instead) |
| t005 | Simplify — /simplify over the changed docs                                    | 15m | t002, t003, t004 | — **DONE** (pure prose/pointer edits, nothing to simplify) |
| t006 | Test coverage — verification re-run of the drift audit checks                 | 20m | t002, t003, t004 | — **DONE** |
| t007 | Closeout                                                                       | 15m | t006             | — **DONE** |

## Definition of done

Every open w5 milestone has exactly one unchecked `- [ ]` row in `w5/README.md` and a collision-free number whose task-file IDs match their path (the open "Key Value metrics: typed id + resolved CR name" milestone no longer shares `m71` with the done agent-session-persistence milestone in `w5/done/m71/`, and its promotion from note `w5/044` is recorded); `FUTURE-MAYBE.md`'s "durable/shared state for bex-api's replica-local buckets" entry sits under Done citing `w1/done/m58`; `docs/ADR018-render-parity.md:286` cites `.pm/w8/done/005.md` and `:306` describes `w3/m41` as code-complete with live-verify pending (not completed); `w5/m66`'s README marks t003 `— **DONE**`; `w1/README.md` lists open notes 049/050 and `w5/README.md` lists open notes 036–040 in their Inbox sections; `npx prettier@3.4.2 --write "**/*.md"` leaves no diff.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-08-18 drift audit — the m2/m3 truth-sweep lineage. Verified findings: the w5 m71 collision (open Key-Value-metrics milestone at `.pm/w5/m71/` vs done milestone at `w5/done/m71/`, no checkbox row anywhere, source note `w5/044` never recorded as promoted); `FUTURE-MAYBE.md:17` still Deferred though its trigger fired (w1/m52 two replicas) and the work shipped 2026-07-30 as `w1/done/m58` (which cites the entry as its source); ADR018 stale path (:286) and completed-tense on open `w3/m41` (:306); `w5/m66` t003 row/frontmatter contradiction; missing inbox listings in the w1 and w5 READMEs. Everything else in the audit checked clean.
- **Goal linkage:** board truth is the scheduling substrate for every worker — the m71 collision literally hides open work from `/pm status` and `/loop-worker` scans, and the `/pm` canon's IDs-must-match-path rule names exactly this class of drift as the anti-example to repair on contact.
- **Expected outcome:** a board where checkbox state, numbering, and cross-references can be trusted again; the hidden Key-Value-metrics milestone becomes schedulable.
- **Why now:** a hidden open milestone is actively costing scheduling correctness today; the other drifts compound the longer they sit.
- **Render parity note:** omitted — pure board/docs work with no REST/GraphQL/MCP/UI surface change.
