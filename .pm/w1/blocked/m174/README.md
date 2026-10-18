# w1 · m174 — Get main's red checks passing again: x/net CVEs, lint OOM, flaky tests, Dependabot, schema re-pin

**Worker:** worker1 **Goal:** every check on `main` that is red today passes reliably, so a red check means something again and no known reachable Go vulnerability ships. **Status:** blocked — shipped `e8beef717` (t001–t008 done; t005 finished by `w1/m176` and `w1/m177`). Follow-up t010 is done 2026-10-18: it repairs the new GitOps validation regressions and passes local validation. Closeout still waits on CI history: the latest go lint and govulncheck streaks are 18 and 20 green runs respectively, but gitops (render) has three new failures (details below). Render schema drift's next two scheduled runs are Mondays 2026-10-19 and 10-26 at 06:17Z. The dispatched drift run already passes (`38805794187`, 2026-10-15).

## Tasks (in order)

| id   | title                                                                                         | est | depends_on                    |
| ---- | --------------------------------------------------------------------------------------------- | --- | ----------------------------- |
| t001 | Bump `golang.org/x/net` to ≥ v0.60.0 in all four Go modules; govulncheck green — **DONE**                | 30m | —                             |
| t002 | Add `gomod` ecosystems to `.github/dependabot.yml` — **DONE**                                            | 15m | t001                          |
| t003 | Stop the whole-program dead-code pass being killed for memory (`make lint` Error 137) — **DONE**         | 45m | —                             |
| t004 | Fix three flaky gitops tests (Zot GC replay, `closed_labels`, LogDelivery off by one second) — **DONE** | 45m | —                             |
| t005 | Unstick Dependabot PRs #83 (removed `Github` icon) and #78 (SHA-pinned action inventory) — **DONE**      | 40m | —                             |
| t006 | Review and re-pin Render's Blueprint schema (`buildSourceGit.baseDir` → `dockerContext`) — **DONE**      | 30m | —                             |
| t010 | Repair GitOps validation on the CI runner — **DONE** | 30m | t004 |
| t007 | Simplify — **DONE**                                                                                      | 15m | t002, t003, t004, t005, t006, t010  |
| t008 | Test coverage — **DONE**                                                                                 | 30m | t002, t003, t004, t005, t006, t010  |
| t009 | Closeout | 10m | t008, t010 |

## Definition of done

- `govulncheck (Go workspace)`, `go lint`, `gitops (render)` and `Render schema drift` each pass on 10 consecutive runs on `main` (scheduled checks: their next 2 scheduled runs), with run ids recorded.
- `go lint`'s dead-code step actually runs to completion on every one of those runs (no `Error 137`).
- Dependabot PR #78 is landed and closed. #83's split is recorded in `w1/done/125`: the approved portions shipped through m176/m177; the remaining majors are deferred.
- Stale GitHub issue #61 (backend gate red since 2026-08-28; the backend suite is green) is closed.

## Evidence

- **govulncheck** (run `38690089246`, and every recent main run since 2026-10-09): GO-2026-6617, GO-2026-6612 and related advisories in `golang.org/x/net` v0.56.0 (types), v0.57.0 (operator, backend), v0.58.0 (cli), fixed in v0.60.0; reachable, e.g. `v1alpha1/image_network.go:96` → `fmt.Sprintf` → `http2.*`. The check does not block deploys, and `.github/dependabot.yml:3` leaves Go to that check.
- **go lint**: 6 of the last 8 runs (`38690089254`, `38639567839`, `38490116982`, `38488129704`, `38484221853`, `38483585183`) print `0 issues.` four times, then `make: *** [Makefile:106: lint] Error 137`: the single `deadcode` invocation over `./backend/... ./operator/... ./cli/...` (`lego/operator/Makefile:105-114`) is OOM-killed on the self-hosted ARM64 runner.
- **gitops (render)** flakes: `ZotGCMetricsTest.test_fresh_failure_replay_reload_and_idle_expiry` (`scripts/test_log_shipper.py:402`, the `docker exec … cat >> /tmp/zot.log` append at `:474` exits 2; runs `38331653642`, `38462537656`, `38639567794`), `PlatformLogsTest.test_core_services_collected_with_closed_labels` (`:354`; runs `37754585720`, `36975770320`), `LogDeliveryTest` (`scripts/test_log_delivery.py:33`; one second off, run `37699407947`).
- **Dependabot**: #83 (66 dashboard updates, open since 2026-10-06) fails typecheck because `lucide-react` removed `Github`, used in `dashboard-header.tsx`, `new-session-composer.tsx`, `connect-github-card.tsx`, `git-credentials-menu.tsx`, `self-host-exit.tsx`, `service-detail-header.tsx`, `service-source-picker.tsx`. #78 (2 action bumps, open since 2026-10-01) fails `scripts (test)`: "GitHub Action refs differ from the reviewed SHA-pinned inventory".
- **Render schema drift** red since run `38271824184` (2026-10-12): upstream renamed `buildSourceGit.baseDir` to `dockerContext` against the `a0d4e8a3…` pin set by w1/m170.

## Resume check (2026-10-18)

- **A real regression, not only elapsed time.** GitOps runs `39014340784`, `39059845725`, and `39104356083` fail because the fallback-TLS guard still expects the direct verifier call replaced by w1/131's retry wrapper. All three also report `rg: command not found`: the two new TLS predicates introduced a tool that the runner does not install. Run `39059845725` additionally reports a broken pipe while checking the existing database-attribution rule.
- **t010 repairs these causes.** The overlapping TLS-wrapper fix landed independently as `8333a44c5` during verification and is preserved. The remaining repair keeps its workflow-to-wrapper-to-verifier contract. Its predicates use the already-required grep. Captured text reaches early-exiting grep/awk checks through here-strings, so large valid log-shipper values cannot fail with SIGPIPE. GitOps now triggers when the wrapper changes.
- **CI history, newest first.** The latest ten go lint successes are `39104355989`, `39096869408`, `39067825731`, `39067635375`, `39061111330`, `39059955389`, `39059845727`, `39058723844`, `39055424327`, `39014884921`. The corresponding govulncheck successes are `39104356069`, `39096869392`, `39067825740`, `39067635420`, `39061111281`, `39059955134`, `39059845711`, `39058723801`, `39055424267`, `39014884899`. GitOps needs a new ten-run streak after the repair; no additional schema-drift schedule has fired since the earlier check. Do not close t009 yet.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-10-15, item 2 (approved by the user the same day); CI/incident sweep, re-verified from the run logs above.
- **Goal linkage:** V0 roadmap 7 (security review): no known reachable vulnerability in shipped Go; and a CI signal the release pipeline can trust.
- **Expected outcome:** reachable x/net CVEs gone, the dead-code check on every run, no flake-driven reds, Dependabot flowing again, the Blueprint schema pin current.
- **Why now:** the vulnerabilities are reachable in production code today, and checks that are always red train everyone to ignore them (the dead-code check has been silently off about half the time).
- **Render parity omitted:** no REST/GraphQL/MCP/UI change. Build Sources stay refused wholesale (`blueprint_compiler.go:471-477`, ADR049), so the schema re-pin changes nothing a user sees.
- **Constraints:** stay on the self-hosted runners (`DO_NOT_DO.md` `#CI-RUNNERS`, `#RUNNER-HOSTS`); fix memory use, never move a job to GitHub-hosted runners.
