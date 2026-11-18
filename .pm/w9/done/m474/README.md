# w9 · m474 — Render's Terraform provider compatibility: its own acceptance suite re-recorded against bex, each resource graded

**Worker:** worker9 **Goal:** bex knows, resource by resource, whether Render's official Terraform provider works against it, from Render's own acceptance tests, and every bex-side gap is on the board **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A harness runs Render's Terraform provider acceptance suite against a bex API and keeps the recordings out of git — **DONE** | 1h | m472/t004 |
| [t002](done/t002.md) | `docs/terraform-compatibility-checklist.md`: one row per provider resource and data source, graded — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Run the suite against bex and triage every failure: bex gap or the test's own Render-account assumption — **DONE** | 1h 30m | t001, t002 |
| [t004](done/t004.md) | File every bex-side gap on the board and add a Terraform provider row to ADR018 — **DONE** | 30m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004 |
| [t007](done/t007.md) | Closeout — **DONE** | 15m | t005, t006 |

## Definition of done

- `scripts/terraform-provider-verify.sh` runs any of the provider's 16 resource packages against a given bex host and prints per-test results; no recording or log is committed.
- `docs/terraform-compatibility-checklist.md` grades every package (passes, fails with an owning board item, or recorded omission) for a pinned provider and Terraform version.
- ADR018 has a "Terraform provider" row pointing at the checklist, and every bex-side failure is a filed board item.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w9, preferrably one aspect of render parity` (2026-11-17), item 3, filed by `/pmx 1,2,3 to w9`. `render-oss/terraform-provider-render` (tip `06af456`, 2026-11-16) records its acceptance tests with go-vcr and re-records against any Render-compatible host with `UPDATE_RECORDINGS=true` (`internal/provider/testhelpers/provider.go`). The same pattern graded Render's SDKs in w32/m284 and the CLI in `docs/cli-compatibility-checklist.md`.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first). Render's official clients are surfaces bex must serve; the provider is Render's official IaC client beside Blueprints ([render.com/docs/terraform-provider](https://render.com/docs/terraform-provider)).
- **Expected outcome:** a graded, re-runnable answer to "does Render's Terraform provider work on bex", and the remaining gaps on the board.
- **Why now:** route coverage is complete (every operation the provider calls is mounted or a recorded omission), so what is left is behavior only a run shows: read-back diffs and asynchronous create semantics. w9/m472 fixes the request refusals static analysis found; this run should follow it so it does not re-find them. The provider changed two days before filing, so pinning now is cheap.
- **Render parity task:** omitted. This milestone ships a harness and documentation, no REST/GraphQL/MCP/UI change; the fixes it finds are filed as their own items, which carry their own parity task.
- **Optional, not scheduled:** a weekly `/pm routine` to re-run the harness, only if the user asks for one.

## Closeout (2026-11-18)

- **Target and pins.** Production's pin (`054f29128f64`) did not carry w9/m472 (`5a387b8a7`), so every grade comes from dev-9, built from main at `35d7bbdf2`, plus `agent-up` for OpenBao. Provider `06af456`, Terraform 1.17.2.
- **t001.** `scripts/terraform-provider-verify.sh` runs any of the 16 packages (or `all`) and prints a per-test table. It refuses a work directory inside the repository and sweeps what each test leaves. `git status` after every run showed only this milestone's files; the cassettes and logs stayed under `/tmp`.
- **t002.** [docs/terraform-compatibility-checklist.md](../../../../docs/terraform-compatibility-checklist.md) has one row per package (16), naming every resource and data source. It is linked from `docs/AGENTS.md` and ADR018.
- **t003.** Run A (production-shaped: `BEX_REGION=fsn1`, the API key as minted) passed 3 of the 47 recorded tests. Run B (diagnostic: region `oregon` and the key granted Admin) passed 5. Every failure is classified in the checklist's nine failure classes, with the request, Render's recorded answer and bex's answer. The rest are recorded omissions or non-goals. A supplementary `apply`/`plan`/`destroy` graded registry credentials, which have no upstream test.
- **t004.** The bex-side gaps are filed as w9/105 (region readback, needs a user decision against ADR049 D1/D7) and w9/106–w9/113. ADR018 has a "Render's Terraform provider" row in Deployment sources & IaC.
- **t005.** `/simplify`: folded `snapshot()` into its one caller, simplified the package-path and build-failure parsing, dropped an unused awk variable and renamed a variable that shadowed `head`. The tests stayed green.
- **t006.** `scripts/terraform-provider-verify.test.sh` (13 checks, in `scripts.yml`) covers:
  - argument, credential and work-directory refusals, including a symlink that resolves into the repository;
  - the result table (bex against offline tests, the first error, a non-empty plan, a build failure, an empty package);
  - the sweep against a fake API (cursor paging, only new resources deleted with projects last, a survivor failing the sweep).

  Three mutations were each caught: dropping the repository guard, a sweep that ignores the snapshot, and losing the offline classification.
- **Cleanup.** Terraform or the sweep destroyed every resource, and the workspace read empty at the end. The API keys were revoked. `dev-env.sh 9 agent-down` and `down` ran, and the throwaway tenant namespaces were deleted.

