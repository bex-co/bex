# w4 · m26 — Audit-log Render-shape verification (evidence-first)

**Worker:** worker4 **Goal:** The last unowned ◐ on the parity ledger gets resolved with evidence: Render's actual audit-log response schema — unresolvable from public docs when bex's audit surface was built "best-effort" — is hunted down with the tools that now exist (pinned OpenAPI, official CLI source, live capture from the real Render account), and bex's shape is aligned or the divergence is recorded as verified. **Status:** todo

## Tasks (in order)

| id   | title                                                                       | est | depends_on |
| ---- | ----------------------------------------------------------------------------- | --- | ---------- |
| t001 | Hunt Render's actual audit-log response schema                              | 40m | —          |
| t002 | Diff vs bex's fields + envelope; align, or record parity-by-evidence        | 60m | t001       |
| t003 | Update ADR018:134 with the evidence and resolution                          | 15m | t002       |
| t004 | Render parity                                                                 | 20m | t003       |
| t005 | Simplify                                                                      | 15m | t004       |
| t006 | Test coverage                                                                 | 25m | t004       |
| t007 | Closeout                                                                      | 15m | t006       |

## Definition of done

The audit-logs ◐ cells (REST/GraphQL/UI) in docs/ADR018-render-parity.md carry dated evidence: either bex's response is aligned to a captured Render shape (fields + list envelope), or the divergence is documented as verified — with the capture (or the documented absence of any capturable surface) attached. "Confirmed parity-by-evidence, ledger annotated" is a success outcome; "still couldn't find it" must record where was looked so the next round doesn't repeat the hunt.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 13, 2026-07-15 — parity-ledger mining: ADR018:134 is the only substantive ◐ with no owning milestone or note anywhere on the board ("Render's exact JSON response schema wasn't resolvable from public docs at authoring time… best-effort rendering… bex's own established `{object, cursor}` per-item shape rather than an unverified Render envelope").
- **Goal linkage:** Render parity; w4 owns the audit log (w4/m10 built it, w4/m23 chores it).
- **Expected outcome:** the last evidence-blocked ledger cell stops being permanently unverifiable; clients get either a Render-true shape or a documented, deliberate one.
- **Why now:** "blocked on evidence" was assessed before the pinned OpenAPI, the CLI-source workflow (w9/m2), and the real-Render-account capture workflow (planned for w5/m32) existed — the blockage may simply have expired.
- **Render parity:** included — this milestone *is* the parity check for the audit-log surface; t004 stays narrow (cross-surface consistency of whatever t002 ships).
