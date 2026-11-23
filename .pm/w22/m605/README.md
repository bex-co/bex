# w22 · m605 — Render's Terraform provider suite, rerun whole at one production commit after its twelve fixes

**Worker:** worker22 **Goal:** `docs/terraform-compatibility-checklist.md` grades Render's official Terraform provider from one run of its whole acceptance suite against bex at production's pin, not from ten partial runs at ten commits; every test that still fails is either a recorded omission or non-goal, or a new bex-side failure class with an owning board item; and a broader configuration is applied on production. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                      | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Run all 16 packages once on a `dev-N` stack built from production's pin source, production-shaped                                         | 75m | —          |
| t002 | Classify every failing test by the first request it stops at: omission, non-goal, test assumption, or a new bex-side class (13 onward)   | 60m | t001       |
| t003 | File each new class with an owner, sized by the board's rule                                                                               | 30m | t002       |
| t004 | Production: apply, plan and destroy a broader configuration in `bex-canary` with the registry's v1.9.1 provider                           | 60m | t001       |
| t005 | The checklist's Run K column and totals; ADR018's Terraform row marked ✅ only if every non-omission test passes                            | 30m | t002, t004 |
| t006 | Simplify                                                                                                                                   | 20m | t005       |
| t007 | Test coverage                                                                                                                              | 20m | t005       |
| t008 | Closeout                                                                                                                                   | 10m | t003, t006, t007 |

## Definition of done

- One run, recorded as Run K in the checklist, covers all 16 packages at a single bex commit that production's pin carries, with the stack's commit, region mapping, authorization mode, plan and key recorded, and the recordings' sha256.
- Every failing test in Run K names the first request it stops at and is classed as a recorded omission (ADR018 § Render operation omissions), a non-goal (`.pm/DO_NOT_DO.md`), a test assumption, or a new bex-side class with a board item that owns it.
- The checklist's per-package grades and totals come from Run K; the older runs stay as history.
- On production, as the QA user in `bex-canary`, a configuration wider than w9/m472 t005's (at least a private service, a background worker, Postgres, Key Value, an environment group and a project with an environment, none using an omitted field) applies, plans clean, and destroys, and every resource answers 404 afterwards.
- ADR018's Terraform row says what Run K and the production apply show.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w22` (2026-11-23), item 5, filed by `/pmx them all to w22`; verified at `origin/main` `cb469e7cfd`.
  - `docs/terraform-compatibility-checklist.md` pins provider `06af456d9bdb…` (still the upstream tip on 2026-11-23; no commit since 2026-11-16; latest release v1.9.1) and Terraform 1.17.2. Its grades are a patchwork of Runs A–J on dev-9 at different commits: postgres was last graded in Run A, before w9/m481 lifted the protected-environment 403 for an Admin's key; keyvalue and redis in Run I; the four service packages pass 19 of 27 only by combining Run H with Run J's three scaling tests. The checklist itself says "Each failing test stops at its first refused request, so a fixed row can still uncover a later failure. Re-run the harness after each fix."
  - All twelve failure classes are fixed (w9/105–113, w9/122/m521, w9/123, w9/129). Production's pin is `9fdf7e0a1171` (`cded300699`, 2026-11-23), which carries w9/129 (`23d4c7636e`) and w9/123; class 12 was found on production by w9/m472 t005 and was never rerun in the suite.
  - Harness: `scripts/terraform-provider-verify.sh <package>... | all` (offline tests in `scripts/terraform-provider-verify.test.sh`).
- **Goal linkage:** ADR008 § Render parity first; ADR018's Terraform provider row (`docs/ADR018-render-parity.md`, the create-service row cites w9/m472).
- **Expected outcome:** one honest, current score for "Render's Terraform provider works on bex", and an owner for every remaining failure, instead of a score assembled from runs that predate the fixes.
- **Why now:** the last fix (w9/129) reached production on 2026-11-23, and the provider has not moved since 2026-11-16, so a full run now measures a stable target with every known class fixed.
- **Placement:** w22 by the user's choice of workstream; the earlier runs are w9's history (w9/m472–m481), which this milestone only reads.
- **Render parity omitted:** this milestone grades and files; it changes no REST, GraphQL, MCP or UI surface. Each new class it finds is fixed in its own item, which carries its own Render parity task.
