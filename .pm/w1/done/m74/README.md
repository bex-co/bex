# w1 · m74 — The second MCP fold: the per-field `update_*` / `rename_*` tools

**Worker:** worker1 **Goal:** finish what `w1/m71` started — the per-field grammar that merely spells itself `update_*` instead of `set_*` folds into the same patch tools, on a rule anyone can check. **Status:** done (2026-08-18) — measured **187 → 175 tools** (−12, no new tools), the full backend suite, `make lint` across all four modules, and the deprecation guard green.

## Tasks (in order)

| id   | title                                                    | est | depends_on |
| ---- | -------------------------------------------------------- | --- | ---------- |
| t001 | Answer the three questions `051.md` raised, from REST      | 45m | — — **DONE** |
| t002 | Fold the service-scoped tools                              | 1h  | t001 — **DONE** |
| t003 | Fold the datastore + grouping tools                        | 1h  | t001 — **DONE** |
| t004 | Ledger, ADR018 + ADR006, and the repo-wide name sweep      | 45m | t002, t003 — **DONE** |
| t005 | Test coverage                                              | 1h  | t002, t003 — **DONE** |
| t006 | Closeout                                                   | 15m | t004, t005 — **DONE** |

## The rule this milestone applies

`w1/m71` folded 30 `set_*` setters into five patch tools by mirroring REST `PATCH`. The same rule decides this milestone's scope, and it decides `051.md`'s three open questions without needing new judgement:

> **A tool folds if REST carries its field in the resource's `PATCH` body. A tool keeps its own name if REST puts it behind its own route.**

That is not a stylistic preference — it is the property that keeps the three adapters from drifting, and it is already what `update_service` was built to satisfy.

| tool | REST | disposition |
| --- | --- | --- |
| `update_service_plan` | `PATCH` `serviceDetails.plan` | **fold** (`plan` + `dryRun`) |
| `update_idle_timeout` | `PATCH` `serviceDetails.idleTTLSeconds` | **fold** |
| `update_publish_path` | `PATCH` `serviceDetails.publishPath` | **fold** |
| `update_cron_job` | `PATCH` `schedule` / `command` | **fold** |
| `scale_service` | `POST .../scale` — its own route | keep |
| `update_static_routes` / `update_static_headers` | `PUT .../routes` / `PUT .../headers` | keep |
| `get_autoscaling` / `disable_autoscaling` | `GET`/`DELETE .../autoscaling` | keep (m71 already decided this) |
| `update_postgres_plan` / `_version` / `_disk_autoscaling`, `rename_postgres` | all `PATCH` fields on `PostgresPatch` | **fold** |
| `update_key_value_plan`, `rename_key_value` | `PATCH` fields on `KeyValuePatch` | **fold** |
| `rename_project`, `rename_environment` | the patch tools already take `name` | **fold** (they are duplicates) |

**12 tools retired, none created: 187 → 175.**

## `051.md`'s three questions, answered

1. **`dryRun` on a multi-field patch.** `update_postgres`/`update_key_value` already have it, and their `Preview*` twins already validate the whole patch. `update_service` gains REST's exact rule: `dryRun` with `plan` previews the plan change and writes nothing; `dryRun` with no plan is a read-only reflect. **Deliberate divergence:** where REST silently drops the other fields of a dry-run body, MCP refuses the call — an agent that asked to preview a command change should be told the tool cannot, not handed an unchanged object that implies it did.
2. **Plan changes are billing events.** They stay billing events: the Service layer's payment and billing gates are unchanged, and the folded argument is documented as billable. What the fold removes is a tool name, not a guard.
3. **`update_cron_job` was once an upstream name.** Upstream **removed** it (#89) because theirs was a placeholder returning a dashboard link; the pin classifies bex's as `Extension`, so no agent written against current Render calls it. It folds, and the ledger records the loss of the "bex makes Render's stub real" positioning — ADR018's row keeps its ✅ because that column scores reachability.

## Definition of done

- Every `Parity1to1` tool untouched; the surface count measured, not estimated.
- Each folded field reaches the same Service verb it did before, with `dryRun` semantics preserved where they existed.
- Retired names recorded in the same ledger `w1/m71` used, with the exact replacement call, and no hard-coded old name left anywhere in the repo.
- Zero ADR018 ✅ lost.

## Source + Goal linkage

- **Source:** `.pm/w1/051.md`, filed 2026-08-18 at the end of `w1/m71` and promoted the same day by user direction.
- **Goal linkage:** ADR008's AI-native pillar — most MCP clients degrade between 40 and 60 tools, and this is the last large block of one-tool-per-field on the agent surface.

## What shipped

**Twelve tools retired, none created — 187 → 175**, measured off the live registry, not estimated. The four `Parity1to1`-adjacent classes are untouched (10 / 1 / 8 unchanged; `Extension` 168 → 156).

**The rule did the deciding, which is the point.** `scale_service`, `update_static_routes`, `update_static_headers` and the autoscaling verbs survive not because they felt different but because REST gives each its own route; everything folded is a field REST already carries in the resource's `PATCH` body. That is checkable by anyone reading `rest.go`, which is what keeps the next person from re-litigating the boundary.

**`051.md`'s three questions, as resolved in practice.**

1. *`dryRun` on a multi-field patch.* `update_postgres`/`update_key_value` already had it, so the plan/version/name folds inherited it. `update_service` gained it on REST's terms — previews `plan`, writes nothing, bare dry run reflects — with one deliberate divergence: a dry run carrying any other settable field is **refused and the fields named**, where REST silently drops them. An agent that asked to preview a command change should be told the tool cannot.
2. *Plan changes are billing events.* Still are — the payment and plan-billing gates live in the Service layer and the fold did not touch them. What changed is that the billing consequence is now stated in the argument description itself, where an agent reads it, instead of implied by a tool's name.
3. *`update_cron_job` was an upstream name.* Upstream removed it (#89) because theirs was a placeholder; bex's was real. It folded, and the ledger records the lost positioning while ADR018 keeps its ✅ — that column scores reachability.

**One invariant nearly lost, and kept.** `TestDiskAutoscalingCapDescriptionMatchesCatalog` asserts the disk-autoscaling cap figure in the tool description is interpolated from the shared plan catalog, so an agent's number cannot drift from the operator's. Folding the tool would have dropped the `fmt.Sprintf`; the description on `update_postgres` now carries it instead, and the test was repointed rather than deleted.

**Tests follow the capability, not the name.** Every pre-existing test for a retired tool was repointed at the patch tool (plan changes, renames, version upgrades, dry runs, the GraphQL↔MCP parity pair), and new coverage asserts what a fold can break: each folded field applied alone, schedule-without-command leaving the command intact, a rename leaving allowlist and parameters intact, and the dry-run refusal not writing the field it refused.
