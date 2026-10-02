# w1 · m170 — Re-pin Render's Blueprint schema: Build Sources, workflow services, and the per-kind compute-plan split

**Worker:** worker1 **Goal:** `scripts/render-schema-drift.sh` is green against Render's live `render.yaml.json`, and every construct the new schema admits is either honored the way Render honors it or refused by name. Nothing is accepted and then ignored. **Status:** done 2026-10-02 (post-ship: one `workflow_dispatch` run of `Render schema drift`, see t007)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Triage the upstream diff and re-pin the schema + registry — **DONE** | 45m | — |
| t002 | Refuse Build Sources, workflow services and environment-less projects by name — **DONE** | 50m | t001 |
| t003 | Per-kind plan enums and exact-size compute/Key Value plan IDs — **DONE** | 40m | t001 |
| t004 | Render parity — **DONE** | 25m | t002, t003 |
| t005 | Simplify — **DONE** | 10m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004, t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- `lego/backend/internal/apps/schema/render.yaml.json` is Render's live bytes (`a0d4e8a3…`, fetched 2026-10-02). The digest is the same in `scripts/render-schema-drift.sh`, `RenderBlueprintSchemaSHA256` and `capabilities.json`. `bash scripts/render-schema-drift.sh` exits 0, and its self-test passes.
- Every new schema field and enum value is classified in `capabilities.json`. The exhaustiveness and fixture tests pass, and no entry uses an `ignored` state.
- Compute plan IDs that equal a bex rung exactly resolve to that rung in Blueprints and on direct plan writes. Other sizes fail by name at the plan field.
- `buildSources`/`buildSource`, `type: workflow`, `fromService` type `workflow`/property `slug`, and a project with no environments each produce exactly one named diagnostic at the field's path.
- The new tests fail on the pre-change code. `go test ./internal/apps/... ./internal/api/...` (with Postgres) and `make lint` pass.
- ADR049 records the disposition table. ADR018 carries rows for compute plan IDs, Build Sources and workflow services.

## Source + Goal linkage

- **Source:** inbox note `w1/117` (split out of `w1/m165` by user decision 2026-09-30; evidence in `.pm/w1/done/m165/README.md` § Scope discovery), promoted 2026-10-02 during `/loopx w1`. The note's diff (`57aa0a1f…`) had moved again by 2026-10-02 (`a0d4e8a3…`). The surface was the same, but the new snapshot also carries the per-kind compute-plan IDs, `fromService.property: slug`, `serviceType: workflow`, the optional `projects[].environments`, and the stricter `buildFilter`.
- **Goal linkage:** ADR008 Render-compatible hosting; ADR049's fail-closed Blueprint contract (D3 pin + registry, D7 no lossy approximation); ADR018 parity ledger.
- **Expected outcome:** the `render-schema` drift job goes green on a real comparison. Blueprints written to today's `blueprint-spec` (e.g. `plan: 0.5c-512mb`, `plan: 256mb`) deploy on bex. Render-only constructs fail with a message that names the construct, and never deploy something different.
- **Why now:** the drift job had been red on genuine drift since m165 repaired it. Until the re-pin, every Blueprint using Render's now-documented compute plan IDs was refused as schema-invalid.
- **Render parity included:** this milestone is a Render-contract change. Where Render's behavior was uncertain, it was checked against `blueprint-spec` (`render.com/docs/blueprint-spec.md`, 2026-10-02) and the Build Sources announcement.

## Triage (2026-10-02) — verdict: Work

The premise holds. `scripts/render-schema-drift.sh` failed with `pinned=665539cb… upstream=a0d4e8a3…`, and the surface diff (43 fields, 5 enum pointers, `plan` removed) matched the note. Per construct:

- **Build Sources: refuse (unsupported).** The feature is a Render **private beta** ([render.com/blog/build-reuse-private-beta](https://render.com/blog/build-reuse-private-beta), 2026-09-21, opt-in form). It has zero mentions in `blueprint-spec`, and `render.com/docs/build-sources` returns 404 (rechecked 2026-10-02). Its observable contract is one immutable build deployed to N services, with build-time env vars scoped apart from runtime env, plus first-class REST resources (`buildSourceId` in the OpenAPI). bex builds per service and has no build-only env scope. Compile-time inlining would therefore be the lossy approximation ADR049 D7 forbids, and ADR049 has no `ignored` state (the note's option 1 does not exist in the registry validator). A Blueprint valid on Render that bex refuses is a parity gap, but here closing it means building a new product surface (build-artifact reuse). That is a future milestone of its own, not a re-pin side effect.
- **Workflow services: refuse.** `DO_NOT_DO.md` "Workflows & tasks", and the ADR018 `—` row.
- **Plan split: implement.** Render now names plans by compute plan ID, and a new service defaults to `0.5c-512mb`. bex already had `w8/011` aliases for Postgres/KV, but not for compute. The KV aliases were also never applied on the Blueprint path.
- **Cron `plan: free`: keep accepting (documented divergence).** Render's new `cronPlan` has no free rung. bex keeps a free cron tier, and existing tenant Blueprints declare it, so the compiled schema admits `free` (coordinator decision 2026-10-02). The pinned bytes are unchanged.
- **Optional `projects[].environments`: keep requiring.** The docs still say "one or more environments", and bex would otherwise silently apply nothing.

Number note: `w1/m169` is taken by a parallel session (`w1/blocked/m169` in a sibling worktree), so this is m170.
