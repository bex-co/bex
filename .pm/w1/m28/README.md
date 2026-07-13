# w1 · m28 — Gate deploys on real CI test runs

**Worker:** worker1 **Goal:** A commit that breaks a backend, operator, or dashboard test fails CI and the `deploy.yml` job that would have shipped it to prod does not run; `BEX_TEST_DB_URI`/`BEX_TEST_OPENFGA_URL`-gated integration tests execute in CI against real ephemeral services instead of silently skipping. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                                              | est | depends_on           |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | --------------------- |
| t001 | New CI workflow: `cd lego/backend && go test ./...` on PR + push touching `lego/backend/**`                                                                                       | 30m | —                     |
| t002 | New CI workflow: `cd lego/operator && make test` (envtest + unit, auto-runs manifests/generate/fmt/vet first) on PR + push touching `lego/operator/**` or `lego/types/**`         | 40m | —                     |
| t003 | New CI workflow: `yarn typecheck && yarn lint && yarn test` on PR + push touching `dashboard/**`                                                                                   | 30m | —                     |
| t004 | Wire ephemeral Postgres + OpenFGA service containers into the backend test workflow (GitHub Actions `services:`) so `BEX_TEST_DB_URI`/`BEX_TEST_OPENFGA_URL`-gated tests actually run instead of permanently skipping | 45m | t001                  |
| t005 | Restructure `deploy.yml` so build+push+deploy `needs:` the new test jobs — the enforcement point has to be inside the same triggered workflow run, since this repo pushes directly to `main` (no PR gate) | 30m | t001, t002, t003      |
| t006 | Live verification: deliberately break a test on a scratch branch/commit, confirm the new CI fails and blocks the deploy job, then confirm a passing commit deploys normally        | 30m | t005                  |
| t007 | Update `CLAUDE.md`/`lego/operator/CLAUDE.md`/`dashboard/CLAUDE.md`'s command sections to note these are now CI-enforced, not just local dev commands                               | 20m | t006                  |
| t008 | Simplify: run `/simplify` over the CI workflow changes                                                                                                                             | 20m | t007                  |
| t009 | Test coverage: N/A in the usual sense (this milestone's "tests" are the CI workflows themselves) — instead, confirm t006's break/fix drill is documented as the acceptance evidence | 15m | t007                  |
| t010 | Closeout: verify DoD, mark done, move to `w1/done/m28/`                                                                                                                            | 15m | t008, t009            |

## Definition of done

A commit that breaks a backend, operator, or dashboard test fails CI and the `deploy.yml` job that would have shipped it to prod does not run; `BEX_TEST_DB_URI`/`BEX_TEST_OPENFGA_URL`-gated integration tests execute in CI against real ephemeral services instead of silently skipping.

## Source + Goal linkage

- **Source:** CI-workflow audit during `/pm-brainstorm more milestones to work on`, 2026-07-13 (`.github/workflows/*.yml` — no `go test`/`make test`/`yarn test` anywhere; `deploy.yml`'s single ungated `build-and-deploy` job).
- **Goal linkage:** platform reliability — the same w1 charter as `m26`/`m27`, but foundational to both: those milestones' own "Test coverage" closing tasks are only meaningful if CI actually runs them.
- **Expected outcome:** every push to `main` that touches `lego/`/`dashboard/` runs its test suite before anything reaches prod; the DR/pull-path drills from `m26`/`m27` and every future milestone's tests become real safety nets instead of aspirational documentation.
- **Why now:** this is the highest-leverage single gap found this session — it doesn't just risk one incident, it means no regression anywhere in the stack is caught before shipping, silently, on every single deploy to date. Not Render parity (internal engineering infrastructure has no Render-side analog) — pursued on its own reliability merits, per user decision 2026-07-13.
- **Render parity omitted:** pure CI/infra, no REST/GraphQL/MCP/UI surface.
