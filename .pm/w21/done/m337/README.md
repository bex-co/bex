# w21 · m337 — Dashboard routes hydrate cleanly across server and browser environments

**Worker:** worker21 **Goal:** every dashboard route fixture server-renders in one environment (UTC, English) and hydrates in another (a non-UTC time zone, Chinese, a later clock, a different cookie and storage state) with no recoverable hydration error, so the #418 class is caught by `yarn test` instead of by a production QA pass. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Harness: server-render in one environment and hydrate in another — **DONE** | 60m | — |
| t002 | Pin the test time zone and run the hydration project outside UTC — **DONE** | 30m | t001 |
| t003 | Run every route fixture through the harness — **DONE** | 90m | t001, t002 |
| t004 | Fix what the sweep finds (time-boxed) and replay the historical fixes — **DONE** | 60m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 20m | t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- `src/test/hydration.ts` exposes a probe that renders to a string under one environment (time zone, language, clock, cookies, storage) and hydrates under another, and reports React's recoverable errors; its self-tests reproduce the w6/049 (clock read across a month boundary), w6/m107 (zone-dependent formatting) and w21/013 (default time zone) mismatches and stay quiet for their fixes. **Met.**
- Every route test, every `*page*.test.tsx` and the automation form test is probed automatically after its assertions pass (`src/test/route-hydration-sweep.ts`): server in UTC, browser in another zone a minute later, in English or Chinese. **Revised 2026-11-11 from "every route fixture passes both pairs":** one pair per test (chosen by name), because a probe mounts the tree again and a stateful fixture answers its second mount differently; a page whose server pass errors is inconclusive (14 of 585 probes, all mock limits), and the single exemption is a fixture that holds a loader pending. **Met**, and it found and fixed one real mismatch (the Router page).
- Reverting a historical fix fails a test: w21/013 fails the sweep; w6/049 fails `charges-card.test.tsx`'s boundary tests (the usage page fixture does not reach that row). **Revised from "fails the sweep"**, with the reason recorded in t004. **Met.**
- `yarn lint` and `yarn test` pass (also with the shell in `TZ=Asia/Shanghai`), and `yarn test` takes 79–81 s against about 76 s before. **Met.**
- w21/054 is not reproduced: the harness expresses its cookie and storage environment, the swept pages do not reach the lazy zh catalog behind a dehydrated boundary, and 054 stays blocked with that recorded. **Met** as written (the either-way clause).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w21`, 2026-11-11. At least seven server/client mismatches were found only on production by live QA: w6/030, w6/049, w9/080, w21/013, w6/m107, w4/m152, and w21/054 (cause unknown, blocked). Each guard so far was written per component after the finding (`src/test/hydration.ts` is used by 10 test files). `vitest.config.ts` pins no time zone, and Node applies a change of `process.env.TZ` at runtime (checked on Node 24), so one test can render in UTC and hydrate in another zone.
- **Goal linkage:** a clean first paint on every page, the dashboard's skeleton and hydration rules (`dashboard/AGENTS.md`), and the Render-style dashboard UX bex ships against ([ADR008](../../../../docs/ADR008-vision.md) Render parity first).
- **Expected outcome:** a component that formats a time, a month, a language or any other environment-dependent value without a hydration-safe rule fails `yarn test`.
- **Why now:** w21/054 cannot move without a reproduction (a local stub run on 2026-11-11 did not reproduce it), and each new mismatch costs a QA round plus a fix.
- **Render parity task:** omitted. This is test infrastructure; a fix t004 makes changes a value's formatting hydration-safely, not what the page offers, and Render has no hydration contract to compare.
