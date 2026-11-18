# w9 · m474 — Render's Terraform provider compatibility: its own acceptance suite re-recorded against bex, each resource graded

**Worker:** worker9 **Goal:** bex knows, resource by resource, whether Render's official Terraform provider works against it, from Render's own acceptance tests, and every bex-side gap is on the board **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | A harness runs Render's Terraform provider acceptance suite against a bex API and keeps the recordings out of git | 1h | m472/t004 |
| [t002](t002.md) | `docs/terraform-compatibility-checklist.md`: one row per provider resource and data source, graded | 45m | t001 |
| [t003](t003.md) | Run the suite against bex and triage every failure: bex gap or the test's own Render-account assumption | 1h 30m | t001, t002 |
| [t004](t004.md) | File every bex-side gap on the board and add a Terraform provider row to ADR018 | 30m | t003 |
| [t005](t005.md) | Simplify | 20m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004 |
| [t007](t007.md) | Closeout | 15m | t005, t006 |

## Definition of done

- `scripts/terraform-provider-verify.sh` runs any of the provider's 16 resource packages against a given bex host and prints per-test results; no recording or log is committed.
- `docs/terraform-compatibility-checklist.md` grades every package (passes, fails with an owning board item, or recorded omission) for a pinned provider and Terraform version.
- ADR018 has a "Terraform provider" row pointing at the checklist, and every bex-side failure is a filed board item.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w9, preferrably one aspect of render parity` (2026-11-17), item 3, filed by `/pmx 1,2,3 to w9`. `render-oss/terraform-provider-render` (tip `06af456`, 2026-11-16) records its acceptance tests with go-vcr and re-records against any Render-compatible host with `UPDATE_RECORDINGS=true` (`internal/provider/testhelpers/provider.go`). The same pattern graded Render's SDKs in w32/m284 and the CLI in `docs/cli-compatibility-checklist.md`.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). Render's official clients are surfaces bex must serve; the provider is Render's official IaC client beside Blueprints ([render.com/docs/terraform-provider](https://render.com/docs/terraform-provider)).
- **Expected outcome:** a graded, re-runnable answer to "does Render's Terraform provider work on bex", and the remaining gaps on the board.
- **Why now:** route coverage is complete (every operation the provider calls is mounted or a recorded omission), so what is left is behavior only a run shows: read-back diffs and asynchronous create semantics. w9/m472 fixes the request refusals static analysis found; this run should follow it so it does not re-find them. The provider changed two days before filing, so pinning now is cheap.
- **Render parity task:** omitted. This milestone ships a harness and documentation, no REST/GraphQL/MCP/UI change; the fixes it finds are filed as their own items, which carry their own parity task.
- **Optional, not scheduled:** a weekly `/pm routine` to re-run the harness, only if the user asks for one.
