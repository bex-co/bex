# w21 · m337 — Dashboard routes hydrate cleanly across server and browser environments

**Worker:** worker21 **Goal:** every dashboard route fixture server-renders in one environment (UTC, English) and hydrates in another (a non-UTC time zone, Chinese, a later clock, a different cookie and storage state) with no recoverable hydration error, so the #418 class is caught by `yarn test` instead of by a production QA pass. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Harness: server-render in one environment and hydrate in another | 60m | — |
| t002 | Pin the test time zone and run the hydration project outside UTC | 30m | t001 |
| t003 | Run every route fixture through the harness | 90m | t001, t002 |
| t004 | Fix what the sweep finds (time-boxed) and replay the historical fixes | 60m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 20m | t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- `src/test/hydration.ts` exposes a probe that renders to a string under one environment (time zone, language, clock, cookies, storage) and hydrates under another, and reports React's recoverable errors; its self-tests reproduce the w6/049 (projected month) and w21/013 (time zone default) mismatches.
- Every route fixture in `src/routes/__tests__` passes the probe for (UTC, English) to (Asia/Shanghai, Chinese, a clock a minute later), and for the w21/054 case (the language cookie missing while localStorage says `zh`).
- Reverting the w6/049 and w21/013 fixes locally makes the sweep fail; the check is recorded in this README.
- `yarn lint` and `yarn test` pass and `yarn test` takes under 60 seconds longer than before.
- w21/054 is either reproduced and fixed here, or this README records that the harness cannot reproduce it either, and 054 stays blocked with that evidence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w21`, 2026-11-11. At least seven server/client mismatches were found only on production by live QA: w6/030, w6/049, w9/080, w21/013, w6/m107, w4/m152, and w21/054 (cause unknown, blocked). Each guard so far was written per component after the finding (`src/test/hydration.ts` is used by 10 test files). `vitest.config.ts` pins no time zone, and Node applies a change of `process.env.TZ` at runtime (checked on Node 24), so one test can render in UTC and hydrate in another zone.
- **Goal linkage:** a clean first paint on every page, the dashboard's skeleton and hydration rules (`dashboard/AGENTS.md`), and the Render-style dashboard UX bex ships against ([ADR008](../../../docs/ADR008-vision.md) Render parity first).
- **Expected outcome:** a component that formats a time, a month, a language or any other environment-dependent value without a hydration-safe rule fails `yarn test`.
- **Why now:** w21/054 cannot move without a reproduction (a local stub run on 2026-11-11 did not reproduce it), and each new mismatch costs a QA round plus a fix.
- **Render parity task:** omitted. This is test infrastructure; a fix t004 makes changes a value's formatting hydration-safely, not what the page offers, and Render has no hydration contract to compare.
