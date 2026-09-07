# w2 · m88 — Split self-hosted CI runner pools by trust level

**Worker:** worker2 **Goal:** PR-facing CI jobs and production-credentialed deploy jobs never share a runner host — a poisoned CI runner can no longer be the machine that later holds deploy credentials _(goal narrowed 2026-09-07 by user decision `.pm/DO_NOT_DO.md` `#RUNNER-HOSTS`: single-Mac fleet is a hard constraint, so the split is label-level pool routing, and the shared-host residual is accepted)_ **Status:** done 2026-09-07 — label-level pool split live and fail-closed-validated; fleet recovered from full outage (Docker Desktop down + OrbStack zombie fleet crash-looping; 13 stale registrations purged); green-run evidence on both pools (`scripts (test)` 34160723241 on `bex-ci` runner `c57df3d66c97`; deploy 33994179845 `build-and-deploy` on `bex-production` runner `57789c8a59a8`); ADR083 + ADR019 + runbook + validator record the disposition final; host separation + shared-runner credential rotation rejected/waived per `#RUNNER-HOSTS`.

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Define the trust-pool label scheme + fleet re-label runbook — **DONE**                     | 30m | —          |
| t002 | Update every workflow's `runs-on` to its trust pool — **DONE**                             | 45m | t001       |
| t003 | Enforce pool-per-trust in `scripts/github-actions-validate.sh` — **DONE**                  | 30m | t002       |
| t004 | Verify green runs on both pools; record the split in ADR083 + ADR019 — **DONE** | 30m | t003       |
| t005 | Simplify — **DONE**                                                             | 20m | t004       |
| t006 | Test coverage — **DONE**                                                        | 30m | t004       |
| t007 | Closeout — **DONE**                                                             | 15m | t005, t006 |

## Definition of done

- Credentialed workflows (`deploy.yml`, `app-cluster.yml`, `infra.yml`, restore drills, and any other job holding production/infra secrets) target only `production`-pool runners; test/lint/build workflows target only `ci`-pool runners.
- `scripts/github-actions-validate.sh` fails closed on a credentialed workflow targeting the `ci` pool (and still rejects any GitHub-hosted `runs-on`, per `DO_NOT_DO.md #CI-RUNNERS`).
- At least one real green run per pool after the re-label (a CI workflow on `ci`, a deploy on `production`).
- The operator-side re-label steps are a written runbook; the repo work fails safe if the fleet is not yet re-labeled (jobs queue rather than run on the wrong pool).
- ADR083's follow-up 1 and ADR019 §runner custody record the split as done.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-09-01 #3; `docs/ADR083-security-review-round20.md` §Follow-ups item 1 (unowned) — finding 3 (high, accepted): one shared pool serves untrusted PR jobs and production deploys. `.pm/DO_NOT_DO.md #CI-RUNNERS` names the split a sanctioned hardening path (never a revert to GitHub-hosted).
- **Goal linkage:** V0 roadmap item 7 (security review); bounds a round-20 accepted residual without touching the accepted self-hosted custody decision.
- **Expected outcome:** compromise of a PR-facing runner no longer reaches hosts that hold production credentials.
- **Why now:** the custody shift is fresh (2026-08-23) and the fleet already showed operational fragility (`w6/040` multi-day pipeline outage) — hardening while the topology is actively operated is cheaper than retrofitting; m89 (secretless build) composes with this split.
- **Render parity:** **omitted** — pure CI/platform infrastructure; no REST/GraphQL/MCP/dashboard surface.
