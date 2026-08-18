# w9 · m85 — Dashboard typecheck covers test files: stop silent fixture rot

**Worker:** worker9 **Goal:** a dashboard test fixture that drifts out of sync with the type it claims to build fails `yarn typecheck` (and CI) instead of rotting silently. **Status:** todo

## Tasks (in order)

| id   | title                                                                           | est | depends_on |
| ---- | ------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Add a typecheck pass that includes `**/*.test.{ts,tsx}` + wire into CI          | 40m | —          |
| t002 | Fix the type drift the new pass surfaces across existing fixtures               | 60m | t001       |
| t003 | Simplify (closing)                                                              | 20m | t002       |
| t004 | Test coverage: guard tripwire — a deliberately-wrong fixture fails typecheck    | 20m | t002       |
| t005 | Closeout (closing)                                                              | 10m | t004       |

## Definition of done

`yarn typecheck` type-checks `**/*.test.{ts,tsx}`; a test fixture that drifts from the type it claims to build **fails the command**; CI runs the pass on dashboard changes. All existing fixture drift surfaced by enabling it is fixed and the suite stays green.

## Source + Goal linkage

- **Source:** `.pm/w5/041.md` (promoted 2026-08-17 via `/pm-brainstorm` "what to do for w9"); surfaced independently by two reviewers during the `w5/m65` `/simplify`: `dashboard/tsconfig.app.json` excludes `**/*.test.tsx` from `tsc -b` and vitest does not typecheck, so a fixture can drift from its type with nothing failing.
- **Goal linkage:** first-party dashboard correctness/DX (`dashboard/CLAUDE.md`); a build guardrail that removes an entire class of silent rot.
- **Expected outcome:** fixture/type drift becomes a hard `yarn typecheck` failure; every future dashboard change (including w9/m84) gets its test fixtures typechecked for free.
- **Why now:** with 2,100+ dashboard tests the hole is live and growing; capping it now is cheapest, and the dashboard build is warm from the m60–m83 sweep. **Render parity OMITTED** — pure build-config + type-drift repair, no REST/GraphQL/MCP/UI surface change (noted per the standing-task rule).
- **Sizing note:** t002's true size depends on how much drift exists; if enabling the pass surfaces near-nothing, this clears the ~1h bar only marginally — acceptable given the guardrail value, but if it turns out trivial, fold the remainder into an inbox note at closeout rather than padding.
