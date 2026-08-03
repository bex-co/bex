# w1 · m63 — `render.yaml` parity: one strict, presence-aware Blueprint compiler

**Worker:** worker1 **Goal:** make a Render Blueprint produce equivalent bex behavior or a precise pre-write refusal—never a successful silent no-op—through one pinned-schema compiler shared by every surface. **Status:** done (2026-08-03; deployed production verification complete)

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | Pin Render's schema and create the exhaustive capability registry + drift check | 50m | — | — **DONE** |
| t002 | Strict YAML AST/schema front end with duplicate-key, unknown-field, and source-location errors | 60m | t001 | — **DONE** |
| t003 | Presence-aware normalized IR + authorized current-state action planner | 60m | t002 | — **DONE** |
| t004 | Service create/sync semantics: defaults, omission, explicit empty, scaling, and env preservation | 60m | t003 | — **DONE** |
| t005 | Postgres + Key Value create/sync semantics: defaults, omission, explicit empty, and plan spellings | 50m | t003 | — **DONE** |
| t006 | Close App adapter gaps: Docker command/context, subdomain policy, static rules, registry credentials | 60m | t003 | — **DONE** |
| t007 | Close datastore adapter gaps: storage autoscaling, PgBouncer, and pooled references | 45m | t003 | — **DONE** |
| t008 | Complete grouping, reference, and `sync: false` semantics across every resource location | 60m | t003 | — **DONE** |
| t009 | Fail-closed unsupported/deprecated fields and move bex-only `builder` under `x-bex` | 45m | t002 | — **DONE** |
| t010 | Render-compatible validation wire contract: 10 MB, multi-error results, and honest plans | 50m | t003, t008, t009 | — **DONE** |
| t011 | Make `render.yaml` canonical; migrate stored paths/copy/examples with an unambiguous `bex.yml` fallback | 45m | t009, t010 | — **DONE** |
| t012 | Route every Blueprint entrypoint through the compiler; retire the shell compiler and repair resource inventory/docs | 60m | t004, t005, t006, t007, t008, t010, t011 | — **DONE** |
| t013 | Render parity check across REST/GraphQL/MCP/dashboard/official CLI | 40m | t012 | — **DONE** |
| t014 | Simplify pass over the Blueprint compiler and adapters | 30m | t013 | — **DONE** |
| t015 | Test coverage: schema exhaustiveness, conformance corpus, sync semantics, and cross-entrypoint invariance | 60m | t013, t014 | — **DONE** |
| t016 | Closeout | 15m | t015 | — **DONE** |

## Definition of done

- New Blueprint discovery defaults to `render.yaml`; an explicit path wins; `bex.yml` remains a warning-emitting filename-only alias with identical grammar; implicit discovery refuses when both files exist.
- The repository pins a reviewed Render JSON Schema snapshot and an exhaustive capability registry. CI fails for an unclassified field/enum and reports upstream schema drift without changing production behavior.
- Every public Blueprint operation—validate, preview, create, manual/Git sync, deploy-from-chat, REST, GraphQL, MCP, dashboard, and the helper workflow—uses the same compiler, normalized IR, and action plan. `scripts/app-apply.sh` contains no YAML-to-CR compiler.
- A successful validation contains no unknown, unsupported, or silently ignored field. Persistent disks, previews, per-resource regions, and `checksPass` return field-specific pre-write errors; no task implements those anti-goal/missing capabilities.
- Omitted, explicitly empty, and set values remain distinct through create and sync. Render's documented field-specific defaults/preservation/reset rules are proven against existing services, Postgres, Key Value, env vars, and build filters.
- Adapter-backed Render fields named in ADR049 are either observably equivalent or explicitly reclassified unsupported—never approximated. Nested/ungrouped resources and env groups, existing-resource references, `sync: false` prompts, validation plans, and Blueprint resource inventory are complete.
- The multipart validation endpoint accepts up to 10 MB, returns all independently actionable errors with stable path/line/column metadata and no secret values, and distinguishes structural declaration summaries from authorized current-state action plans.
- The official Render CLI validates representative accepted and rejected fixtures against bex-api; REST, GraphQL, MCP, dashboard, and CLI show the same codes/paths/semantics. Backend/operator/dashboard suites and lint are green.
- `docs/ADR006-bex-api.md`, `docs/ADR018-render-parity.md`, the CLI checklist, examples, UI copy, and MCP descriptions no longer claim blanket Blueprint parity or describe stale behavior.

## Source + Goal linkage

- **Source:** [docs/ADR049-render-yaml-parity.md](../../../docs/ADR049-render-yaml-parity.md), accepted 2026-08-02 after the user's `bex.yaml` versus `render.yaml` parity investigation; explicit `$pm` handoff to w1 in the same session.
- **Goal linkage:** ADR008 pillar 4 (“API and agent control surface”) and bex's primary open-source Render-alternative promise. A trustworthy `render.yaml` is the portable IaC boundary across humans, agents, the official Render CLI, and Git auto-sync.
- **Expected outcome:** a Render user can submit one Blueprint unchanged and receive either equivalent infrastructure or a precise unsupported-field result before mutation; no accepted manifest silently loses intent, and every bex entrypoint agrees on the plan.
- **Why now:** the newly shipped Git-connected Blueprint create/review flow defaults new users into the permissive parser, while ADR018 currently marks the capability fully green. Each additional field or UI flow compounds the false-success risk and the cost of migrating stored manifests. The schema, current parser gaps, and official behavior were freshly researched for ADR049, making this the cheapest point to replace the contract rather than patch another field. Render parity is included because the entire milestone is a user-facing compatibility fix; persistent disks and preview environments stay explicit unsupported cases per `.pm/DO_NOT_DO.md`, and the existing helper remains a thin API client rather than becoming a first-party CLI.
