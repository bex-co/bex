# w5 · m112 — Test gates: no silent skips in CI, the real-DB backend suite runs locally, and /ship won't push onto a red main

**Worker:** worker5 **Goal:** A test that can't run fails loudly in CI; the Postgres + OpenFGA + OpenBao suite is one command away locally and runs before backend pushes; nothing is pushed onto a failing deploy. **Status:** done (2026-10-05)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add a dependency guard that skips locally and fails in CI — **DONE** | 45m | — |
| t002 | Adopt the guard in every dependency-gated test — **DONE** | 1h | t001 |
| t003 | One command for local Postgres, OpenFGA and OpenBao test dependencies — **DONE** | 45m | t001 |
| t004 | Make /ship run the real-DB suite for backend changes — **DONE** | 30m | t003 |
| t005 | Stop /ship before pushing onto a red main — **DONE** | 30m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- Under CI, a missing promtool, Postgres/OpenFGA or Valkey dependency makes its tests fail instead of skip; locally they skip with the command that enables them.
- `scripts/backend-test-deps.sh` starts the CI-pinned Postgres 17, OpenFGA and OpenBao and prints the env; with it, `cd lego/backend && go test ./...` runs the 59 gated files.
- `/ship` runs that suite when a push touches `lego/backend/**` or `lego/types/**`, and stops before pushing onto a main whose latest deploy failed, unless the push is the fix.
- `bash scripts/skill-layout-validate.sh` passes.

## Source + Goal linkage

- **Source:** 2026-10-05: w4/m172 (`d1e4c9315`) broke `TestEnvGroupReadSideOwnerIDTargetingE2E` (404 vs 403). The test is gated on `BEX_TEST_DB_URI`/`BEX_TEST_OPENFGA_URL`, so it skipped locally and failed only in CI; deploys stayed red for about an hour while three more commits landed. The same review found two more silent skips: the promtool test (backend CI lacks promtool) and the Valkey engine test (non-production args).
- **Goal linkage:** ADR008 reliable delivery: `deploy.yml` builds only when every suite passes (`lego/AGENTS.md:17`).
- **Expected outcome:** Regressions like w4/m172's are caught before push, no test silently stops running in CI, and main isn't pushed onto while red.
- **Why now:** It happened on 2026-10-05 and blocked deploys for an hour, and this queue adds many backend changes.
- **Render parity omitted:** CI and developer tooling only; no REST/GraphQL/MCP/UI change.

## Evidence — 2026-10-05

- `testenv.Skip` replaces 98 dependency skips. Backend CI declares `BEX_TEST_REQUIRE_DEPS=1`, so a missing test DB, OpenFGA, OpenBao or promtool fails there. Jobs that run backend packages without dependencies (cli-test, cli-release, mobile) still skip, because strictness is opt-in per job.
- Correction to this milestone's premise: the Valkey engine suite was never skipped in CI (`operator-test.yml` sets `BEX_TEST_VALKEY=1`). It passed w4/191 because it booted Valkey with `--requirepass` instead of the production args; w5/m110 fixed that.
- `scripts/backend-test-deps.sh` gives the same pinned dependencies locally. The full backend suite passed strict against it: 68 packages, including the w4/m172-era `TestEnvGroupReadSideOwnerIDTargetingE2E` that only CI used to run.
- `/ship` now gates pushes with `scripts/ship-gates.sh`: no stacking on a red main, and the real-DB backend suite for backend/types/authz changes. This milestone's own ship is the gate's first live use.
- Follow-up: w5/081 (a JWT-only OpenBao block in workspaces_e2e never runs in CI).
