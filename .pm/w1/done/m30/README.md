# w1 · m30 — SIGTERM shutdown fix + Dependabot residual watch

**Worker:** worker1 **Goal:** bex-api exits promptly on SIGTERM instead of serving through the full grace period with a cancelled context, and every remaining Dependabot residual is re-checked for a now-safe fix. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                                                                                     | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Fix bex-api's SIGTERM handling: serve the HTTP server in a goroutine instead of blocking on `ListenAndServe`, select on `<-ctx.Done()`, then `httpSrv.Shutdown(timeoutCtx)` — same fix for the `BEX_CP_ADDR` internal-API server — **DONE** | 30m | —          |
| t002 | Add a regression test proving a SIGTERM'd bex-api binary actually exits within the grace period instead of continuing to serve — **DONE**                                              | 20m | t001       |
| t003 | Dependabot residual triage: re-check the 6 residuals recorded in `w1/018.md` (lodash, minimatch/picomatch, srvx, `@tanstack/start-server-core`, vite 8.x) and apply any now-safely-fixable — **DONE** | 30m | —          |
| t004 | Simplify: run `/simplify` over the code this milestone changed — **DONE**                                                                                                                 | 20m | t002, t003 |
| t005 | Test coverage: confirm t002's regression test is meaningful (asserts real exit behavior, not just "no error"); no additional tests needed for the dependency-version bumps — **DONE**    | 15m | t004       |
| t006 | Closeout: verify DoD, mark done, move to `w1/done/m30/` — **DONE**                                                                                                                        | 15m | t005       |

## Definition of done

bex-api exits promptly on SIGTERM (verified by the new regression test) instead of serving through the full grace period with a cancelled context; every Dependabot residual is re-checked and any now-safe fix is applied, with the rest re-recorded with current status.

## Closeout (2026-07-13)

**DoD met.**

- **SIGTERM fix (t001/t002):** `serveUntilShutdown(ctx, *http.Server)` in `lego/backend/cmd/api/main.go` now backs both the public API server and the `BEX_CP_ADDR` internal server — serve in a goroutine, `<-ctx.Done()`, then `Shutdown` with a 10s bound. The public server no longer blocks past SIGTERM; background loops still stop on the same context. Guarded by `main_test.go` (verified to fail against a blocking implementation, pass against the fix).
- **Dependabot residuals (t003):** re-checked all 6 against the live registry + `yarn npm audit`. **5 of 6 closed** — `lodash`→4.18.1 (resolution floor `^4.18.1`), `minimatch`→3.1.5 + 9.0.9 and `picomatch`→2.3.2 (both via `yarn up -R`; each consumer's own caret range absorbed the patch, so the multi-major trap dissolved), `srvx`→0.11.22 (added resolution `^0.11.13`); `vite` 8.x is no longer a security residual (audit flags no vite advisory — 7.3.6 direct + 8.1.4 transitive both clean). **1 genuinely deferred:** `@tanstack/start-server-core` (<1.167.30) needs a coordinated `@tanstack/react-start` suite bump — the framework-level upgrade this milestone scoped out; promote when the dashboard is between feature landings. Full table in `t003.md`.
- **Incidental fix:** t003's `yarn test` surfaced a **pre-existing** red suite — `dashboard/src/routes/__tests__/index.test.tsx` broke when w1/m31 (`41e9b61`) added `useProjects` (an Apollo `useQuery`) to `HomePage` without a test mock. Added the missing `vi.mock`, restoring the CI-gating suite to green (129 files / 803 tests). Verified the failures pre-date this milestone (identical on pristine `main`); the fix is orthogonal to the dependency bumps but was required by t003's own "yarn test green" criterion.

**Follow-up filed:** the hardcoded 10s shutdown timeout could later be made configurable (`BEX_SHUTDOWN_TIMEOUT`) / derived from `terminationGracePeriodSeconds` — flagged by `/simplify`'s altitude pass, scoped out here (new env var + docs). Not milestone-blocking.

## Source + Goal linkage

- **Source:** `.pm/w1/018.md` (Dependabot residual triage, 2026-07-12) and `.pm/w1/019.md` (SIGTERM bug, found via `w3/m7`'s E2E 2026-07-12) — both sub-hour on their own, grouped per the sizing rule, same pattern as `w1/m23`/`w6/m15`/`w7/m10`.
- **Goal linkage:** platform reliability (w1's founding charter) — the SIGTERM bug directly affects every rollout's zero-downtime-deploy guarantee that `w1/m3`'s elastic substrate and `w1/m20`'s autoscaling depend on.
- **Expected outcome:** no in-flight request is ever served by a bex-api pod whose control-plane context has already been cancelled; Dependabot's residual count only includes genuinely-unfixable items.
- **Why now:** the SIGTERM bug has been flagged across two prior `/pm-brainstorm` rounds without being picked up; grouping it with the adjacent Dependabot re-check clears both remaining w1 loose notes in one pass.
- **Render parity omitted:** internal process-lifecycle correctness + dependency hygiene, no REST/GraphQL/MCP/UI surface — same omission rationale as `w1/m26`/`m27`/`m28`.
