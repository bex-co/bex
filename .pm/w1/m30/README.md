# w1 · m30 — SIGTERM shutdown fix + Dependabot residual watch

**Worker:** worker1 **Goal:** bex-api exits promptly on SIGTERM instead of serving through the full grace period with a cancelled context, and every remaining Dependabot residual is re-checked for a now-safe fix. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                                                     | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Fix bex-api's SIGTERM handling: serve the HTTP server in a goroutine instead of blocking on `ListenAndServe`, select on `<-ctx.Done()`, then `httpSrv.Shutdown(timeoutCtx)` — same fix for the `BEX_CP_ADDR` internal-API server | 30m | —          |
| t002 | Add a regression test proving a SIGTERM'd bex-api binary actually exits within the grace period instead of continuing to serve                                                          | 20m | t001       |
| t003 | Dependabot residual triage: re-check the 6 residuals recorded in `w1/018.md` (lodash, minimatch/picomatch, srvx, `@tanstack/start-server-core`, vite 8.x) and apply any now-safely-fixable | 30m | —          |
| t004 | Simplify: run `/simplify` over the code this milestone changed                                                                                                                            | 20m | t002, t003 |
| t005 | Test coverage: confirm t002's regression test is meaningful (asserts real exit behavior, not just "no error"); no additional tests needed for the dependency-version bumps               | 15m | t004       |
| t006 | Closeout: verify DoD, mark done, move to `w1/done/m30/`                                                                                                                                   | 15m | t005       |

## Definition of done

bex-api exits promptly on SIGTERM (verified by the new regression test) instead of serving through the full grace period with a cancelled context; every Dependabot residual is re-checked and any now-safe fix is applied, with the rest re-recorded with current status.

## Source + Goal linkage

- **Source:** `.pm/w1/018.md` (Dependabot residual triage, 2026-07-12) and `.pm/w1/019.md` (SIGTERM bug, found via `w3/m7`'s E2E 2026-07-12) — both sub-hour on their own, grouped per the sizing rule, same pattern as `w1/m23`/`w6/m15`/`w7/m10`.
- **Goal linkage:** platform reliability (w1's founding charter) — the SIGTERM bug directly affects every rollout's zero-downtime-deploy guarantee that `w1/m3`'s elastic substrate and `w1/m20`'s autoscaling depend on.
- **Expected outcome:** no in-flight request is ever served by a bex-api pod whose control-plane context has already been cancelled; Dependabot's residual count only includes genuinely-unfixable items.
- **Why now:** the SIGTERM bug has been flagged across two prior `/pm-brainstorm` rounds without being picked up; grouping it with the adjacent Dependabot re-check clears both remaining w1 loose notes in one pass.
- **Render parity omitted:** internal process-lifecycle correctness + dependency hygiene, no REST/GraphQL/MCP/UI surface — same omission rationale as `w1/m26`/`m27`/`m28`.
