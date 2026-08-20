# w2 · m79 — Upstream drift re-baseline: Render API + official CLI

**Worker:** worker2 **Goal:** re-baseline bex's two compatibility ledgers against current upstream — diff the live Render OpenAPI/docs into ADR018 with a dated re-baseline, verify-or-bump the pinned `render-oss/cli` and re-run the compatibility checklist — filing every discovered gap as a board item instead of fixing inline. **Status:** done

## Tasks (in order)

| id                           | title                                                                                                                                     | est | depends_on   |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- | --- | ------------ |
| t001                         | Diff current Render OpenAPI + public docs against ADR018 rows; list drifted/new capabilities (never reopening DO_NOT_DO `—` rows) — **DONE** | 60m | —            |
| t002                         | Update ADR018: adjust/add rows, stamp a dated re-baseline note — **DONE**                                                                   | 45m | w2/m79/t001  |
| t003                         | Verify-or-bump the pinned upstream render-oss/cli in lego/cli; build + test; note breaking flag/behavior changes — **DONE**                 | 45m | —            |
| t004                         | Re-run the CLI compatibility checklist against a live dev stack with the current CLI; update rows + evidence — **DONE**                     | 60m | w2/m79/t002, w2/m79/t003 |
| t005                         | File each newly found gap as an inbox note or milestone candidate; summarize the re-baseline in this README — **DONE**                      | 30m | w2/m79/t004  |
| t006 (standing closing task) | Simplify (standing): run /simplify over whatever code changed (CLI bump fallout) — **DONE**                                                 | 20m | w2/m79/t005  |
| t007 (standing closing task) | Test coverage (standing): CLI launcher suite green on the bump; regression tests only for trivially small wire-shape fixes — **DONE**       | 30m | w2/m79/t005  |
| t008 (standing closing task) | Closeout (standing): verify DoD, mark done, move milestone to done/ — **DONE**                                                              | 15m | w2/m79/t007  |

## Definition of done

`docs/ADR018-render-parity.md` carries a dated 2026-08 re-baseline note with every row re-verified or updated against current Render; `lego/cli` pins the current upstream release and `cd lego/cli && go test ./...` is green; `docs/cli-compatibility-checklist.md` is re-run against a live dev stack with the current CLI and evidence updated; every newly found gap exists as a filed inbox note or milestone candidate (none silently fixed or silently dropped); DO_NOT_DO `—` rows remain untouched.

## Re-baseline summary (2026-08-19)

- **ADR018:** Method note + rows for service outbound IPs (✖), Managed OIDC (✖), Postgres `connectionPool` OpenAPI enum divergence on the existing pooler row; dedicated-outbound evidence refresh only (`—` kept).
- **CLI pin:** `render-oss/cli` **v2.22.0 → v2.24.0** (`fe8a6188119ee1a53dcf3e5c19f6a5302e840c3f`, module `v1.1.3-0.20260819172634-fe8a6188119e`). Validate + `go test ./...` green. New surface: root `completion`.
- **Checklist:** header/evidence to v2.24.0; live `cli-compat.sh verify` on CAPD **dev-2** — auth/list/env PASS; creates FAIL on tenant NS provisioning (`.pm/w2/026.md`), not wire shape.
- **Filed gaps:** `.pm/w2/023.md` (outbound IPs), `024.md` (`connectionPool` REST), `025.md` (Managed OIDC), `026.md` (dev-2 EnsureTenant).
- **Simplify:** pin/docs only — no launcher code churn beyond the module bump; no further cleanup.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-08-18 (round 2, item 4). ADR018 and the CLI checklist are point-in-time audits (ADR018's passes date 2026-07-08 → 2026-07-27 incl. the w1/m55 canonical-surface re-verification; the checklist's dev-9 baseline and production supplements date 2026-07-18–21); Render ships continuously and bex's compatibility promise decays silently.
- **Goal linkage:** Render parity is the core product bet (ADR006/ADR018); the CLI is the fifth surface (docs/cli-compatibility-checklist.md).
- **Expected outcome:** the ledgers are trustworthy again as of 2026-08; drift becomes filed work items instead of future user bug reports.
- **Why now:** ~5–6 weeks of upstream drift is about the cadence at which prior re-audits found real gaps; cheaper to re-baseline than rediscover via bug reports.
- **Render parity closing task omitted as redundant:** this milestone IS the parity work — a separate parity closing task would audit the audit.
- **Repo facts (checked 2026-08-18):** the checklist's server oracle is the unmodified **v2.21.0** CLI (w2/m57 et al.), while `lego/cli` already pins **v2.22.0** (commit `d8fd7c2bb09d56beaca5df15ac2aefcb5ae5f427`, module pseudo-version `v1.1.3-0.20260721145337-d8fd7c2bb09d`, pinned 2026-07-21 — `lego/cli/UPSTREAM_RENDER_CLI.md`). t003's "bump" means re-pinning to whatever upstream release is current; if v2.22.0 is still latest, t003 verifies + records that instead of moving the pin. Pin bumps follow `UPSTREAM_RENDER_CLI.md`'s procedure (three recorded values diffed by `scripts/bex-cli-validate.sh` in CI; `RootCmd` export check; Bex-native command-name collision check; `isRootVersionRequest` re-diff).
- **Out of scope (whole milestone):** fixing discovered gaps (file them); forking/patching the CLI (DO_NOT_DO); reopening `—` non-goal rows (disks, one-off jobs/ephemeral shells beyond the recorded carve-outs, CDN cache purge, log/metric drains, enterprise SSO/SCIM/maintenance runs/dedicated IPs, workflows & tasks); cutting a bex-cli release (that's /release, rides the `bex-cli/v*` tag train).
