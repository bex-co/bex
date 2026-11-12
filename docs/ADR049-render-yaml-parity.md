# ADR049 — `render.yaml` is bex's Blueprint contract

**Status:** Accepted 2026-08-02; implemented and production-verified by `w1/m63` on 2026-08-03. Schema pin re-verified byte-identical to upstream 2026-08-16 (`w8/m19` t001; digest `665539cb…`), then **re-pinned 2026-10-02 to `a0d4e8a3…`** (`w1/m170`; Build Sources, workflow services, and the per-kind compute-plan split; see [Schema re-pin 2026-10-02](#schema-re-pin-2026-10-02-w1m170)). The weekly drift check is `.github/workflows/render-schema-drift.yml` (Mondays 06:17 UTC).

---

## Context

bex calls its application manifest `bex.yml` while describing it as “render.yaml-shaped.” That wording overstates compatibility. The current implementation is a useful Blueprint subset, but it is not a contractually defined Render-compatible dialect:

- new Git-connected Blueprints, the dashboard, the store, examples, MCP descriptions, and helper scripts default to `bex.yml`, whereas Render defaults to `render.yaml` and permits an explicit custom path;
- `lego/backend/internal/apps/deploy.go` decodes into handwritten Go structs with permissive `yaml.Unmarshal`. `validateBlueprintDialect` rejects a short list of retired bex spellings, but an unknown or unsupported Render field can still be accepted and discarded;
- the decoder loses field presence for many scalar and collection fields. The update path then copies zero/default values into existing App, Database, and Key Value specs, even where Render distinguishes “omitted,” “explicitly empty,” and “set”;
- `autoDeployTrigger: checksPass` currently becomes the same boolean as `commit`, even though bex does not implement a branch-check gate;
- adapter gaps remain for capabilities that bex already has elsewhere, including registry credentials, platform-subdomain policy, static routes and headers, Postgres disk autoscaling, and PgBouncer;
- `scripts/app-apply.sh` is a second manifest compiler that bypasses the Blueprint store, authorization, validation plan, sync history, and several canonical resource shapes;
- validation reports the first semantic error, its plan counts declarations instead of current-state actions, and its multipart request limit is 2 MiB rather than Render's documented 10 MB;
- `docs/ADR006-bex-api.md` and the Blueprint row in `docs/ADR018-render-parity.md` contain claims that no longer match either the implementation or Render's current behavior.

This creates a dangerous false-success mode: a user can submit a valid Render Blueprint, receive a successful bex validation or sync, and end up with materially different infrastructure. An explicit “unsupported” error is incomplete parity; a successful no-op is incorrect behavior.

### Upstream contract researched

Research was refreshed on 2026-08-02 against Render's primary sources:

- the [Blueprint YAML reference](https://render.com/docs/blueprint-spec) says the default root filename is `render.yaml`, defines resource-specific required fields and create-versus-existing omission behavior, and distinguishes `autoDeployTrigger: commit`, `checksPass`, and `off`;
- Render's [official JSON Schema](https://render.com/schema/render.yaml.json) is JSON Schema 2020-12, rejects unevaluated root fields and additional resource fields, and requires fields such as service `type`, `name`, and (except Key Value) `runtime`. The snapshot fetched for this decision had SHA-256 `665539cb0c191856ba38d292b985a963880bb69b030d666e5fe7788e78e7e696`;
- the [Validate Blueprint endpoint](https://api-docs.render.com/reference/validate-blueprint) accepts `multipart/form-data`, requires `ownerId` plus a file, limits the entire request body (including the file) to 10 MB, and validates without mutation;
- the [Blueprint lifecycle documentation](https://render.com/docs/infrastructure-as-code) confirms that sync does not delete resources removed from the file and that existing environment values not overwritten by the Blueprint are retained.

The upstream schema is unversioned at its URL and can change independently of bex. Runtime correctness therefore cannot depend on fetching it live.

## Decision

### D1 — The public filename is `render.yaml`; `bex.yml` is a deprecated filename alias

For application Blueprints, bex adopts Render's name and default:

1. An explicitly configured Blueprint path always wins.
2. Without an explicit path, discovery checks `render.yaml`.
3. `bex.yml` remains a deprecated fallback with the **same grammar and semantics** as `render.yaml`.
4. If both default candidates exist and no path was selected, discovery fails with an ambiguity error instead of guessing.

The alias is filename compatibility only. It is not a separate bex dialect and must not enable otherwise-invalid fields. Existing stored Blueprint records keep their explicit paths. New dashboard and API flows default to `render.yaml`; warnings and docs point legacy repositories to a plain rename.

**YAML anchors and aliases (w8/156, 2026-10-21).** A Blueprint's `*alias` resolves to its `&anchor` node as in any YAML parser, because real Render Blueprints share one `envVars` block that way. The walk counts every materialized node against the same budgets, so an alias bomb fails on `BLUEPRINT_YAML_NODES`; locations point at the anchor. Merge keys (`<<:`) stay refused.

**Deploy to bex button (w8/163, 2026-10-21).** Template authors can add the bex counterpart of Render's button to a README:

```md
[![Deploy to bex](https://dashboard.bex.co/images/deploy-to-bex-button.svg)](https://dashboard.bex.co/deploy?repo=https://github.com/OWNER/REPO)
```

`repo` must be an `https://` clone URL; a trailing `/tree/<branch>` selects the branch, otherwise the repository's default branch is used. An optional `path=` names a Blueprint file other than `render.yaml`. The link signs the user in if needed, then opens the Blueprint review with everything prefilled; approval still happens on that page.

**Create refusals leave no row (w8/166, 2026-10-21).** `POST /v1/blueprints` (and GraphQL/MCP create) runs the per-service create checks that `blueprints validate` reports before admitting the Blueprint, so a manifest those checks refuse answers 400 with nothing created. A failure that only the apply itself can find (a platform fault or a workspace race) still leaves the admitted Blueprint in `error`; clients should list Blueprints after a non-validation error.

There is no application-level `bex.yaml`. `deploy/gitops/base/bex.yaml` is an internal platform deployment manifest with a different purpose; reusing its name for tenant applications would make repository searches and operator instructions ambiguous.

Existing wire fields such as MCP/GraphQL `bexYaml` remain decode aliases for client compatibility, but new descriptions call the value `manifest`. Renaming a transport field is not required to fix the manifest contract.

### D2 — Parity means equivalent behavior or an explicit refusal

Every field in the pinned Render schema must have exactly one machine-readable capability state:

| State | Meaning |
| --- | --- |
| `equivalent` | bex accepts the Render field and implements the same observable create and sync behavior. |
| `translated` | bex maps the field onto a different internal mechanism with equivalent user-visible behavior; the mapping is documented and tested. |
| `unsupported` | validation fails at the exact field path before any write, with the reason and no fallback behavior. |
| `deprecated` | Render itself documents the field as an alias; bex normalizes it to the canonical field with the same precedence rules. |
| `extension` | a bex-only capability under the namespaced `x-bex` object, outside the Render compatibility claim. |

There is no `ignored` state. Unknown fields, unclassified upstream additions, invalid aliases, and misplaced create-API fields fail closed. A schema-valid Blueprint can still be unsupported by bex, but it cannot appear to have applied successfully.

The parity claim is therefore precise: **bex is a fail-closed, behavior-compatible subset of the pinned Render Blueprint contract**, plus explicitly namespaced extensions. Full acceptance of every Render product feature is not required.

### D3 — Pin the upstream schema and overlay a capability registry

The repository vendors a reviewed Render schema snapshot with retrieval metadata and digest. A bex-owned overlay records the capability state, semantic handler, and test fixture for every schema field and allowed enum value.

CI performs two checks:

1. the vendored schema and capability registry are internally exhaustive—an unclassified field or enum is a failure;
2. a scheduled drift check fetches the upstream URL and reports a digest/structural diff for review. It never silently updates the runtime contract.

Production validation uses only the reviewed repository snapshot. Upstream changes enter bex through an explicit review that either implements them or marks them unsupported.

The local schema permits a structured `x-bex` extension object at the root and resource-entry levels. Existing unnamespaced `builder` becomes `x-bex.builder`. Before strict enforcement, stored manifests are audited; any legacy extension is either mechanically migrated when unambiguous or reported for user remediation. New manifests never get a hidden legacy mode based on filename.

### D4 — One compiler owns validate, preview, plan, and apply

All entrypoints use one Blueprint compiler:

```text
YAML source
  → source-preserving AST (duplicate-key and YAML-type checks)
  → pinned schema + bex capability validation
  → cross-resource semantic validation
  → presence-aware normalized IR
  → authorized current-state resolution
  → deterministic action plan
  → apply executor
```

The AST preserves source paths, line/column positions, explicit null/empty values, and field presence. The normalized IR is internal and contains no API-adapter assumptions. Current-state resolution is workspace-scoped and authorization-aware.

The same compiler and plan feed:

- REST validation, preview, create, manual sync, and Git auto-sync;
- GraphQL and MCP adapters;
- the dashboard review/deploy flow;
- deploy-from-chat;
- the local helper workflow.

`scripts/app-apply.sh` becomes a thin caller of the authenticated bex-api Blueprint path (or is retired if that environment cannot provide the API/auth prerequisites). It must not parse YAML or write App/Database CRs itself. This is not a new first-party CLI product.

No mutation occurs when parsing, schema validation, semantic validation, authorization, current-state resolution, or planning fails. Once execution begins, failures are recorded against the planned action and are retryable; this ADR does not pretend Kubernetes and Postgres writes form one distributed transaction.

**Validate and preview run each create's plan (w5/m126).** For every resource the plan would create, validate and preview run the plan its apply runs before that create's first write, so a refusal shows up before any partial apply:

- a new service runs `planNewApp`;
- a new Postgres or Key Value is dry-run through admission exactly as the apply creates it;
- the workspace count cap is checked across every resource the stack creates, which no single create's dry-run can see.

Each refusal carries the apply's message and code. A coded refusal whose `field` param names a declared field is located there rather than by its message, so a Key Value named `Plan_Cache` is refused at its `name`, not its `plan` (w5/093). The protected-environment confirmation, the payment gate and `fromGroup` stay apply-only (ADR018's Blueprint row).

**The plan and the apply resolve a declared service the same way (w5/m133).** A service resolves to the deploy's own workspace's App by the name the manifest declares, which is the name the service was created under, never the name it is displayed as. That is true whatever other workspaces the caller can reach, and the lookup reads the request's one listing of the workspace. Of twins in two namespaces, the workspace's own copy answers. A service still only in the shared namespace answers too, and two copies of a name in one namespace are refused. The apply gets the matched App fresh before patching it. Its later steps address the App it wrote by its public id: env-group links, seeded values and maintenance mode. `fromService` host references, the host-collision preview and reference validation resolve a service the same way the apply does. With no workspace resolved, only Apps that no workspace owns count. Before this, the apply looked each service up through the general service lookup. That lookup matches displayed names and, when a deploy names no workspace (MCP `deploy` never does), reaches every workspace the caller belongs to. An ownerless deploy by a member of two workspaces could patch the other workspace's same-named service.

### D5 — Sync is presence-aware and field-specific

The compiler carries a presence bit for every field through planning. A zero value is never used as a proxy for omission.

Each capability entry defines three behaviors:

- the default for a newly created resource;
- what omission means when adopting or syncing an existing resource;
- what an explicit empty/null value means, if legal.

Render's rules are the default. Examples that must be locked by conformance fixtures include:

- omitted service `plan` defaults to Render's new-service default but preserves an existing service plan;
- omitted `numInstances`, IP allow lists, auto-deploy setting, subdomain policy, database disk size/autoscaling, and connection-pool setting preserve existing values where the Render reference says so;
- omitted `buildFilter` clears existing build filters, because Render documents this field as replacement semantics;
- environment variables not declared by the Blueprint are retained, while explicitly Blueprint-owned values continue to reconcile;
- explicit empty lists follow each field’s semantics: replacement fields may differ from omission, while `envVars: []` preserves undeclared existing values and produces no change when the service is otherwise unchanged;
- `pro plus`, `pro max`, and `pro ultra` use Render's Blueprint spellings at the boundary and translate to internal tier IDs only after validation.

For service environments, an explicit `envVars: []` declares no replacements and retains undeclared values. After merging, an empty literal environment uses the same representation as a serialized App read. An unchanged empty or group-only declaration therefore produces a service no-op; existing environment groups keep their separate conservative update plan. Nonempty declared values still reconcile in stable order. This preserves field presence while avoiding a deployment caused only by empty-slice representation.

Resource deletion remains manual. Render now explicitly documents that Blueprint sync never deletes a resource removed from the file, so bex's no-sync-delete behavior is parity, not a divergence.

### D6 — Validation and planning match the Render-compatible wire contract

The REST validation endpoint keeps Render's multipart shape and 200-with-validation-result behavior, and reports every independently actionable error it can safely discover. Each error carries a stable code, JSON-style path, message, and source line/column when known. Errors and plans never contain secret values.

**Manifest size divergence (w8/m48, 2026-10-02):** bex limits the decoded manifest to **512 KiB (524,288 bytes)**, including comments and whitespace. This pre-decode guard bounds YAML amplification before the structural node, nesting, scalar, collection, and resource budgets run. Those budgets remain unchanged. Render's documented 10 MB limit covers its entire multipart request body; bex deliberately enforces the smaller manifest limit.

REST accepts both Render's multipart file upload and bex's existing JSON `{bexYaml, ownerId?, blueprintId?}` form. A manifest over 512 KiB receives HTTP 413 with `Blueprint manifests are limited to 512 KiB`, including 600 KiB and 3 MiB uploads. The independently bounded request envelope allows **4 MiB**: six bytes per manifest byte for worst-case JSON escapes plus 1 MiB for metadata and multipart headers. Exactly-at-limit decoded manifests remain admissible; crossing either byte bound produces the same named refusal. The route's envelope bound applies before OpenAPI decoding even when the global API body limit is disabled or configured differently. Other routes retain their configured limits.

GraphQL, MCP, and dashboard validation retain the shared compiler result (`valid: false`, `BLUEPRINT_YAML_TOO_LARGE`) with that same 512 KiB message; their ordinary transport body limits still apply before the compiler. The dashboard uses GraphQL and displays the returned diagnostic, so it needs no independent size rule.

The plan is a current-state diff, not a declaration count. It identifies create, update, no-op, unsupported, and conflict actions and is the exact immutable input to execution. Validation without store access may return a clearly labeled structural plan; it must not present declaration counts as the number of changes.

**A service update's `changedFields` name what moved, once each (w4/m229).** Each path is probed alone against the live service, as w4/m124 introduced. The probe is still judged beside every other field the file declares. Narrowing the file to one field made a declared `repo` look omitted. Every probe of an image-backed service then re-ran the inherited-repo switch (w8/m70), so the image-to-Git preview listed all twelve declared paths, including the unchanged name and Free plan. An omission that changes a service gets one path too: the path it omits. An image service taking the Blueprint's repo is listed as `repo`, and a cleared `buildFilter` as `buildFilter`. Neither is attributed to every declared field. The full apply is unchanged. Postgres and Key Value keep their per-field probes, and env groups keep their conservative declared-set update.

**A move is a changed field (w9/094, 2026-11-12).** On Render a Blueprint applies every change its file declares, and its plan lists each one. An existing service, Postgres or Key Value that the file moves plans as `update`:

- into another `projects[].environments[]` entry, the action lists `environment`, plus `project` when the project changes too;
- out of every environment through `ungrouped:`, it lists both paths;
- into an environment or project the sync will create, it lists the paths that change.

A service that stays in an environment whose `networking.isolation` the file turns on or off lists `networkIsolation`, because the apply relabels it. The paths come from one comparison (`diffPlacement`), used by both the plan and the apply: the project, environment and network-isolation labels on the resource against the ones its declaration resolves to. So every move an approval applies is listed, and a resource declared at the top level, which a sync leaves where it is, lists none. Until w9/094 the planner compared settings only. A change that only moved a resource planned `no_changes`, so a Blueprint whose syncs wait for approval never offered it, while a sync without approval applied it. Approving the plan now applies the move, which records `service_moved` for a service (w9/093). The paths need no new field in REST's `rawPlan`, GraphQL's `changedFields` or MCP's plan, and the dashboard lists them like any other changed field.

**An environment is a planned resource (w9/095, 2026-11-12).** A sync writes each `projects[].environments[]` entry before any resource in it. It creates the environment when its project has none of that name. It sets the environment's own settings, `permissions.protection` and `networking.isolation`, when the file changes them; an omitted block keeps the stored value. Projects have no settings of their own in Render's schema (`name`, `environments`, `buildSources`). A sync creates a project only with its first environment or for its build sources, and the plan already lists both. Until w9/095 the plan listed no environment. A file whose only change was an environment's protection or isolation, or a new environment with nothing in it, planned `no_changes`. A Blueprint whose syncs wait for approval therefore never applied it. The plan now lists each environment the sync writes, before the resources and in the order the apply writes them:

- `create`, for an environment the sync will create;
- `update`, for an environment whose own settings the file changes, with `changedFields` naming `permissions.protection` and/or `networking.isolation`.

An unchanged environment is not listed, so an unchanged file's validate and plan output, `totalActions` included, is what it was before w9/095.

The action's `kind` is `environment`. Its `name` is the environment's name, `sourcePath` is its entry (`#/projects/<i>/environments/<j>`), and `resourceId` is its public `evm-` id once it exists. The plan and the sync share one comparison (`parsedGrouping.controls` → `environmentControls.changed`), so an approval applies exactly the listed settings. An unchanged file still plans `no_changes`. Changing an existing environment's settings still needs the workspace Admin's `can_manage`, at approval as at any sync. Render's validate plan (`services`, `databases`, `keyValue`, `envGroups`, `workflows`, `totalActions`) has no environment entry, and its `rawPlan` is "not a stable contract". bex therefore reuses the existing action shape with a new `kind`, the name its capability registry already gives the entry. `totalActions` counts a listed environment, as Render's counts "modifications to individual configuration fields". The pinned Render CLI drops `actions` from validate and only prints `rawPlan`, so it is unaffected. The dashboard labels the row **Environment** and counts environments apart from resources in its headline ("2 resources and 1 environment will change").

Dashboard, GraphQL, and MCP may wrap the result in their native envelope, but field paths, codes, semantics, and plan actions come from the same core result.

### D7 — Close adapter gaps; reject unavailable product features

The first capability-registry pass must distinguish an adapter omission from a missing product capability.

Fields backed by existing bex mechanisms are implementation work for `w1/m63`, including:

- `dockerCommand`, platform-subdomain policy, static `headers`/`routes`, and image/build registry credentials;
- Postgres `storageAutoscalingEnabled`, `connectionPool`, and `fromDatabase.property: connectionPoolString`;
- environment-scoped env groups, nested/ungrouped resource inventory, `sync: false` value collection, and documented references to existing workspace resources;
- exact plan spellings, defaults, omission behavior, and deprecated-field precedence.

A container's command is `dockerCommand` for `runtime: docker` and `runtime: image` alike (w5/080). Render's schema allows it on any server or cron service, and REST, GraphQL and MCP already set an image service's command this way. It becomes the spec's start command, and Generate Blueprint exports an image service's command as `dockerCommand`. A native runtime uses `startCommand`, and declaring both is refused.

Prebuilt-image services accept explicit `autoDeploy: false` and `autoDeployTrigger: off`, matching direct-create disabled values (`autoDeploy: "no"` in the Render CLI/REST contract). Omission and explicit disablement both leave image automation off. Enabled automation (`true` / `commit`), mixed Git/image sources, and Git-only build settings remain incompatible. This is neutral-value compatibility, not image auto-deployment support (w4/m99).

Fields whose semantics bex cannot truthfully provide are rejected. In particular:

- `autoDeployTrigger: checksPass` is translated (w1/m181): a push records the commit and a completed GitHub check suite deploys it once all of its check runs pass (`docs/render-artifacts/auto-deploy-checks-pass.md`). It is never collapsed to `commit`, and a prebuilt image service refuses it like any enabled trigger;
- service/database `region` was unsupported while bex exposed one configured placement rather than per-resource Render regions. Amended 2026-10-29 by user decision D1 ([w8/152](../.pm/w8/done/152.md)): **one region policy on every surface.** Blueprint validate, plan, sync and apply now treat `region` exactly as REST, GraphQL and MCP create do. Any Render region (`oregon`, `ohio`, `virginia`, `frankfurt`, `singapore`) is accepted, and the resource is placed in `BEX_REGION`. Readback amended by user decision D7 (2026-10-29, [w8/218](../.pm/w8/218.md)): services, Postgres and Key Value report `frankfurt` for internal `fsn1` on REST, GraphQL, MCP, Blueprint export and dashboard metadata. This is the stable public alias accepted by the Render CLI, so a cloned service can submit its source region without an explicit override. Public filters compare that alias; the internal placement and create-side normalization remain unchanged. This covers services, cron jobs, Postgres and Key Value, at the top level and under `projects[].environments[]`. There is no error and no per-resource refusal. Render's export writes `region` on every resource, and 75 of 100 sampled exports failed on `region` alone. A value outside Render's region enum is still refused by the pinned schema at its exact path. The five `region` registry entries and the `#/definitions/region/enum` values are `translated` (`resourcemeta.CheckRequestedRegion` holds the create side of the same rule).;
- service `disk` is unsupported under the stateless-first persistent-disk anti-goal _(anti-goal reversed 2026-08-22 by [ADR082](ADR082-persistent-disks.md); the four `disk` registry entries flip to translated handlers as that work lands)_;
- root and per-resource preview configuration is unsupported under the explicit PR-preview-environment anti-goal;
- a newly discovered upstream field is unsupported-by-default until classified.

The registry itself now states these dispositions verbatim (`w8/m19` t002, 2026-08-16): every `unsupported` entry's reason is either an anti-goal citation (previews → the w2/m11 rejection; disks → ADR018's stateless-first row; one-off `job` references), a concrete semantic mismatch (`dpg` fromService references, Postgres majors below 13), or a promoted handler — the original m63 "audit baseline" placeholder text no longer appears.

If a supposedly adapter-backed field cannot meet the observable Render semantics during implementation, its registry entry changes to `unsupported`; the milestone must not ship a lossy approximation.

### D8 — Roll out strictness without preserving two dialects

Strict enforcement is staged:

1. Land the pinned schema, registry, and compiler in audit mode; compare its result with the current path on repository fixtures and stored Blueprint manifests.
2. Inventory and remediate existing stored incompatibilities, especially unnamespaced `builder`, omitted-field assumptions, and files containing silently ignored fields.
3. Make the new compiler authoritative for validation and new Blueprint creation.
4. Make it authoritative for manual/Git sync and deploy-from-chat; remove the handwritten and shell compilers.
5. Change default discovery and UI/examples to `render.yaml`, retaining only the documented `bex.yml` fallback.

Any temporary audit switch is removed at closeout. The end state has one grammar and one compiler, not permanent strict/legacy modes.

## Invariants

- A successful validation has no unknown, unclassified, or silently ignored field.
- Every mutation executes the plan produced by the same compiler version; apply does not reinterpret YAML.
- Invalid or unsupported input causes zero resource writes.
- Filename choice never changes grammar or semantics.
- Omitted and explicitly empty fields remain distinguishable through apply.
- All resource lookup is workspace-scoped and authorized.
- Validation errors, plans, logs, and persisted Blueprint metadata never expose secret values.
- REST, GraphQL, MCP, dashboard, deploy-from-chat, and helper workflows cannot disagree about manifest validity.

### Diagnostics (w4/m102, 2026-09-14)

`anyOf` failures (service kinds and env-var forms) pick the branch the instance actually declared — `type`/`runtime` consts, or which env-var key is present — instead of the fewest leaf errors. A static service with a disallowed `plan` is told `plan` is extra, not that legal `staticPublishPath` is illegal. Create-validator messages that reach a Blueprint author use manifest keys (`staticPublishPath`); REST/MCP `POST` create still say `publishPath`. A `type` that matches no service kind lists the union `web`, `worker`, `pserv`, `cron`, `keyvalue`, `redis`. The schema pin and SHA are unchanged: static sites still have no per-instance `plan`.

### Schema re-pin 2026-10-02 (w1/m170)

Render's published schema moved twice after the 2026-08-16 pin (`57aa0a1f…` seen 2026-09-30 by `w1/m165`, then `a0d4e8a3…` fetched 2026-10-02). The pin, `RenderBlueprintSchemaSHA256`, `capabilities.json` and `scripts/render-schema-drift.sh` now carry `a0d4e8a3…`. Each upstream addition is classified below. The deciding question was whether bex would refuse a Blueprint that Render accepts. Where bex has the same mechanism, the Blueprint is now accepted. Where it does not, the Blueprint is refused by name at the field's path. Nothing is accepted and then ignored.

**Re-pin 2026-10-15 (`w1/m174`):** upstream renamed `buildSourceGit.baseDir` to `dockerContext` (property and the `required` branch of its runtime condition); nothing else changed. The pin is now `8a3fb8da…` in all four places, and the capability entry moved to `#/definitions/buildSourceGit/properties/dockerContext` with the same state: Build Sources remain refused by name, so no accepted Blueprint changes.

**`dockerContext` resolves inside `rootDir` (w8/m128, 2026-10-20):** a Blueprint's `dockerContext` is relative to the service's `rootDir`, as the pinned `buildSourceGit` description and Render's monorepo guide say, so Render's own export (`rootDir: backend`, `dockerContext: .`, `dockerfilePath: ./Dockerfile`) compiles to the build Render runs. The export writes the same rootDir-relative value and omits the default `.`. A file written to bex's earlier repo-root reading (`rootDir: x`, `dockerContext: x`) now means `x/x`, as on Render.

**`dockerContext` spellings (w8/292, 2026-11-12):** `./`, `backend/` and `a//b` name the same directories as `.`, `backend` and `a/b`, as a `rootDir` does (w8/165). The path rule now judges the normalized value on every surface (Blueprint, `POST /v1/services`, `PATCH /v1/services/{id}`, the dashboard's Docker Build Context row), so Render's one-click templates that write `dockerContext: ./` (`RAG-chatbot-template`, `web-research-agent-template`) validate. `..`, an absolute path, a backslash and a control character are still refused, and a refusal is located at the `dockerContext` key. `dockerfilePath` names a file and is still judged as typed.

| Upstream change | bex disposition | Why |
| --- | --- | --- |
| Per-kind plan enums (`serverPlan`, `cronPlan`, `keyValuePlan`, `postgresPlan` replace `plan`). `blueprint-spec` now names plans by compute plan ID, and a new service defaults to `0.5c-512mb`. Since w1/122 (2026-10-15) a plan-less service lands on starter, matching that default. | **Translated** where the ID's CPU/RAM equals a bex rung: compute `0.5c-512mb`→starter, `1c-2g`→standard, `2c-4g`→pro, `4c-8g`→pro-plus, `4c-16g`→pro-max, `8c-32g`→pro-ultra; Key Value `256mb`→starter, `1g`→standard, and since w5/m266 every other `keyValuePlan` value: `5g`→pro, `10g`→pro_plus, `20g`→pro_max, `40g`→pro_ultra, `3g` and `50g` as their own rungs, and the spaced legacy names (`pro plus`→pro_plus; `40g`/`pro ultra`/`50g` refused with `INSTANCE_TYPE_UNAVAILABLE` while paused); Postgres `0.1c-256mb`→basic-256mb, `0.5c-1g`→basic-1gb, `1c-4g`→pro-4gb, `2c-4g`→basic-4gb, and `1c-2g` is itself a bex Postgres plan (w2/m171). Every other size is **unsupported** and fails with `plan names a Render compute plan size that bex does not offer`. | This extends `w8/011`'s exact-size alias rule (no nearest-rung guesses) to compute. The compute aliases live in `tiers.Compute.ByRenderPlan`, so REST/GraphQL/MCP plan writes accept them too. Before this change, the Blueprint parser did not canonicalize Key Value plans. Legacy names keep their prior classification. |
| `cronPlan` has no `free` | **Follows upstream (w8/048, 2026-10-15):** a cron job's `plan: free` is refused by the pinned `cronPlan` enum at `#/services/n/plan`, as Render refuses it. | The 2026-10-02 bex overlay that added `free` to the compiled `cronPlan` (`admitBexFreeCronPlan`) is removed with the user's decision to match Render; cron jobs are paid-only ([ADR030 §7](ADR030-pricing.md)). A tenant Blueprint still declaring `plan: free` on a cron fails validation at that entry until it names a paid plan. `TestBlueprintCronPlanFreeIsRefused` covers it. |
| Top-level, `ungrouped` and `projects[].buildSources`; `services[].buildSource` | **Translated since w3/m270 (2026-11-05)**; w1/m170 refused them by name. Each entry is a build source carrier and a linked service builds from it ([ADR097 § Blueprints](ADR097-build-sources.md#blueprints-w3m270)). | Build Sources are a Render **private beta** ([announcement](https://render.com/blog/build-reuse-private-beta), 2026-09-21), absent from `blueprint-spec`. The refusal's premise (bex builds per service, with no build-only env scope) ended when w2/m254 shipped carriers: one build feeds every linked service, with its own build-time env. |
| `workflowService` (`type: workflow`), `serviceType` `workflow`, `fromService.property: slug` | **Unsupported**. A workflow service gets a single `Render Workflows (type: workflow) are not available on bex` at `…/type`, not one error per field. | This is a recorded non-goal: `DO_NOT_DO.md` "Workflows & tasks", and the ADR018 `—` row. `slug` is defined for workflow services only. |
| `projects[].environments` no longer required | Followed since w3/m275: a project may hold only `buildSources`, and the apply creates it for them; a project with neither still fails `BLUEPRINT_PROJECT_ENVIRONMENTS_REQUIRED`. | `blueprint-spec` still says a project "defines one or more environments", and the relaxation exists only to let a project hold only `buildSources`. bex materializes projects from their environments, so an environment-less project would apply as a silent no-op. |
| `buildFilter` must carry `paths` or `ignoredPaths`; `registryCredential` gains `type: object` | Follows upstream. | A stricter schema with no new vocabulary. |

The compiler needed two schema-preparation fixes for the new shape. Both are guarded by `blueprint_repin_test.go`, and reverting either fails it. `normalizeRenderResourceContainers` now keeps `ungrouped`'s own properties when it merges the resource collections; before, `ungrouped.buildSources` would have been an unknown field instead of a named refusal. `closeBlueprintSchemaObjects` no longer closes `if`/`then`/`else` subschemas. Closing `serverService`'s "buildSource is linked" branch would reject every field it does not list.

### Copying a sibling's generated value (w1/m175, 2026-10-15)

Render's own examples copy a private service's generated password into the service that uses it: `render-examples/ghost` (`MYSQL_PASSWORD`), `wordpress` (`MYSQL_PASSWORD`) and `mattermost` (`MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`), each a `fromService.envVarKey` naming a `generateValue: true` variable. bex refused the whole Blueprint, because a copy was resolved at parse time and a generated value has none.

Render's documented semantics, which bex now follows:

- `fromService` values "update to match the current value of the referenced property on each Blueprint sync. They do not update immediately whenever that property changes." A copy is therefore taken at apply and taken again on every sync.
- `generateValue` sets a value only "if the environment variable doesn't already exist", so the source is minted once; an unchanged source makes the re-copy a no-op.
- Declaration order is not documented as significant; bex copies after every service in the file is applied and seeded, so a consumer may precede its source.

How bex carries it: the copy never becomes a `spec.Env` literal (the value is a secret), and an earlier `spec.Env` entry for the key is dropped, since container env would shadow it. After every service is applied, each consumer's copies are compared with their sources: keys it lacks are seeded in one write (one roll), a key whose source changed is re-set, and an unchanged sync writes nothing and restarts nothing. Reading a source is a sensitive reveal, checked before any service is written: a caller who cannot reveal the source's values cannot copy them. The plan names each copy in the service action's message without its value and reports an existing consumer as a possible `envVars` update, as env-group values are. Still refused, each located at the consumer's entry before any write: a copy of an unanswered `sync: false` prompt, of a variable generated by a linked env group (its value lives in the group's store), of another copy, and a consumer that also declares the copied key.

### Names are unique per kind (w1/126, 2026-10-15)

Render's `mattermost` example names its database and its web service `mattermost`, and bex refused the file with `duplicate name "mattermost"`. Render's Blueprint spec asks for "a unique name for each resource", but Render's own validator (`render blueprints validate`, Render CLI v2.21.0 against `api.render.com`, 2026-10-15) accepts the unchanged file. Probes against the same validator accepted a database and a service sharing a name, a Key Value and a service, and a database and a Key Value. It refused two services sharing a name even with different types ("service name dup of type worker is used more than once") and two databases.

bex now matches: names are unique within services, within databases, within Key Value instances and within env groups. The compiler's per-kind check (`BLUEPRINT_DUPLICATE_RESOURCE`) is the one place that enforces this. The parser's old cross-kind set is gone. Each reference resolves within its own kind: `fromDatabase` against databases, a `keyvalue`-typed `fromService` against Key Value, and any other `fromService` against services alone. Validation locates a service-reference problem at a `fromService` entry, never at a same-named `fromDatabase`. Nothing beneath the Blueprint needed to change. The interactive create paths already checked names per kind, the plan and claims are keyed by `(kind, name)`, and databases and Key Value instances are named by opaque `dpg-`/`red-` ids, so their Kubernetes objects never collide with a service's.

### `fromService` port references follow the real port (w6/m148, 2026-10-20)

Render's [Blueprint spec](https://render.com/docs/blueprint-spec) defines `fromService` `port` as "The port of the service's HTTP server" and gives `my-service:10000` as a `hostport` example. It does not document whether the value comes from `PORT` or from the port Render detects. Before w6/m148, bex resolved both properties at parse time to the platform default 3000. A target whose port had been changed in Settings (w4/m121), or an image-port private service listening on 8123, was addressed at `:3000` on every sync.

bex now resolves `port` and `hostport` at apply, from the applied target App, the same way it resolves `host` ([ADR089](ADR089-render-container-compatibility.md) § Port slice decisions, item 4; [ADR041](ADR041-service-addresses.md) D3 amended):

- The value is the target's current serving port, `App.ServicePort()`. That is its configured port, or the primary of the serving release for an image-port private service. `host` is still the slug alone.
- If the target's port is not known yet, the variable is **omitted**. That happens for an image-port private service before its first image metadata read. The consumer's first release then runs without `port`/`hostport`, never with a guessed 3000. A `host` reference to the same target is written at once.
- The consumer records each port-carrying reference in its `app.bex.co/service-refs` annotation, with the label `app.bex.co/has-service-refs`. bex-api's `ServiceRefPropagator` compares each record with its target every 20 seconds. When a literal is stale, it rewrites it through the tracked, optimistically locked patch, which opens one `config_change` deploy. This covers the first discovery, a later `setPort` on any surface, and a promotion that moves the primary. A suspended consumer gets the saved value but no deploy; its next deploy after resume carries it. A Blueprint sync re-resolves the references too, and removing a reference from the file clears its record.
- The value stays a plain literal, so the Environment view shows it and it ships through ordinary deploy history. An operator-resolved `valueFrom` was rejected (ADR089).

Divergences from Render:

- Render documents that `fromService` values "update to match the current value of the referenced property on each Blueprint sync. They do not update immediately whenever that property changes" (quoted under w1/m175 above). bex updates port references **without** a sync, within one propagator tick, as a deploy of the consumer. This is deliberate: an image-port target's port is first known after its build, not at any sync, so waiting for the next sync would leave the consumer without the variable. `host` and `envVarKey` copies keep Render's sync-time rule.
- For a private service with several ports, the value is the primary bex selected (an explicit port, else 3000 if the image declares it, else the lowest declared TCP port). Render does not document which port it would use.
- A web target with no port set resolves to `10000`, as Render (w8/168; it was 3000, [ADR018](ADR018-render-parity.md) `PORT` row).
- Before discovery the variable is absent, not set. An app that needs it at boot can fail its first release; the `config_change` deploy that follows carries the value.
- Port discovery itself reads only the target image's declared `EXPOSE` ports and covers only Docker private services, Git-built or prebuilt (w6/095) ([ADR018](ADR018-render-parity.md) Open ports row).

Verified live on production on 2026-10-20 ([evidence](../.pm/w6/done/m148/evidence/README.md), DoD 3). An image-port target's consumer had no `API_HOSTPORT`/`API_PORT` on first apply. After the propagator's `config_change` deploy it had `multi-port:8123` and `8123`. A configured web target changed from 3000 to 8080 with `setPort` was resolved as `:8080` by its consumer within 11 seconds. Guards: `TestBlueprintPortReferencesFollowAnEditedTargetPort` (the capability-registry fixture for `port`/`hostport`), `TestBlueprintPortReferencesToAnImagePortTargetNeverGuess`, and the `TestServiceRefPropagator*` tests.

### A Blueprint is its id: several may track one file (w8/m153, 2026-11-11)

**Amended for Render parity** ([ADR008 § Render parity first](ADR008-vision.md#render-parity-first)). This section replaces "One blueprint per repo+branch, and the product saying so" (w4/m125, 2026-09-21). That decision treated migration 0016's `UNIQUE (tenant_id, repo, branch)` as settled design and made a second create answer `409 BLUEPRINT_CONNECTION_CONFLICT`. It did not weigh Render's documentation, and ADR018 recorded no signed-off difference:

> You can create multiple Blueprints from a single YAML file. Each Blueprint creates and manages a completely independent set of resources. […] Render appends a suffix to the name of each new resource to prevent collisions with your existing resources. — render.com/docs/infrastructure-as-code § Replicating a Blueprint, read 2026-11-11

The pinned CLI's `ea blueprints create` defaults to `--existing-resources create_new` for exactly this, so the second create it sends was refused, and the refusal's advice (a `confirm` phrase) cannot be sent from the CLI.

**Decisions.**

- **Identity is the id.** A workspace may hold any number of Blueprints for one repository, branch and file. Migration 0229 drops the unique key and keeps a plain index on the same columns. Every update, sync, plan, approval and delete addresses one row by id, as before.
- **A create is always a new Blueprint.** Render's create (`POST /v1/blueprints` with `source`, GraphQL `createPlannedBlueprint`, MCP `create_blueprint`) and bex's one-step create (the earlier REST body, GraphQL `createBlueprint`, the dashboard) both insert a row with a new id. No create answers `BLUEPRINT_CONNECTION_CONFLICT` and none acts on a connection takeover phrase; the code and the phrase's row-level meaning are retired.
- **A deleted Blueprint is not brought back.** A create after a delete used to revive the deleted row under its old id (w8/m37, w4/120), so an id minted weeks earlier reappeared with another connection's history under it. The deleted row now stays disconnected, readable by nobody, and the new Blueprint starts with its own id, generation and history. Old queued work can never regain authority, because its row can never be live again. The reconnection note (w4/120, w2/074) has nothing left to mark and is no longer written.
- **One manager per resource is unchanged.** Render: "Do not manage a particular resource with more than one Blueprint." Resource claims and `BLUEPRINT_RESOURCE_CONFLICT`, with its `takeover blueprint <id>` phrase, guard that as before. A `create_new` Blueprint cannot collide: its resources carry its own suffix. An `adopt` create of a file whose resources another live Blueprint manages is still a new Blueprint, but it never moves them silently: Render's create plans a failed sync whose notice names the resource and its manager, and a failed plan cannot be approved; the one-step create answers `BLUEPRINT_RESOURCE_CONFLICT` and proceeds only with the phrase.
- **The one-step create retries a failed create instead of duplicating it.** Its admission precedes the apply, so a create whose apply fails leaves its row in `error`. Repeating that create (same file, same mode) continues on the row until a sync has succeeded. It is told apart from a deliberate second Blueprint by the Blueprint's state, not by anything the caller sends: the newest live row for the same repository, branch, file and `existingResources` mode that is applying or failed, has no successful run and was never planned is the retry target; anything else is a new Blueprint. A retry writes what a sync writes, the manifest under a new claim, and keeps the row's id, suffix, name and auto-sync setting, which the user may have changed since. The check and the write hold the row, so a delete or a sync that settles in between is never overwritten. While a create of the same file is still applying, a second one answers `BLUEPRINT_SYNC_BUSY`, which also covers a double submit. A Blueprint that was ever planned came from Render's create, which undoes its own row when the plan cannot be made and retries a failed sync by planning again; a one-step create never takes such a row over.
- **A refused takeover leaves nothing behind.** The one-step create asks the one-manager rule before admission. A create that names another Blueprint's resource without the phrase answers `BLUEPRINT_RESOURCE_CONFLICT` and records no Blueprint; it used to be admitted first, which would now leave a second Blueprint in `error` for every push to re-run into the same refusal.
- **The one-step create takes `existingResources`.** `adopt` stays its default (it manages the names the file gives, as it always did); `create_new` makes a suffixed copy, with the suffix a preview reviewed (`resourceSuffix`) or a fresh one.
- **The review step reports matches, not a conflict.** `blueprintPreview` (REST `POST /v1/blueprints/preview`, MCP `preview_blueprint`) no longer reports `BLUEPRINT_CONNECTION_CONFLICT`. For a Blueprint not yet created it returns `matchedResources`, the declared resources whose names already exist in the workspace, and reviews in the `existingResources` mode it is given (default `adopt`), returning the suffix a `create_new` review used. A review of a create that would retry a failed one is made as that Blueprint, with its suffix, and a one-step create carrying a different suffix is refused rather than silently given the Blueprint's own. MCP's `create_blueprint` is Render's create and assigns its own suffix; the one in a review is an example there. The dashboard follows Render's creation flow, which "displays a notice if your new Blueprint matches existing resources": when there are matches it shows the notice, reviews and creates as `create_new`, and so never asks for a takeover to make a copy.
- **A direct deploy registers the unsuffixed Blueprint.** `deploy` with a repository still records a Blueprint for it. With several on one source it refreshes the oldest live one that manages unsuffixed names, which is the stack the deploy applied, under a per-source lock that replaces the unique key's arbitration. A source whose unsuffixed Blueprint was deleted is not registered again.
- **A push reaches every Blueprint on the branch.** The webhook already queued one intent per `(delivery, blueprint)` and the worker syncs each under its own claim (w8/m38), so two Blueprints on one branch each sync against their own resources and one failing does not stop the other. The single-row lookup by repository and branch is removed; nothing reads "the Blueprint for this repo+branch" any more.
- **Still fail closed.** An unreadable blueprint table refuses the one-step create rather than reading as "a new Blueprint", which would leave a duplicate beside a failed one.

**Names are unique on every create and on a rename (w8/250, 2026-11-11).** Render keeps Blueprint names unique among approved Blueprints. Render's create and the approval enforced that. The one-step create (GraphQL `createBlueprint`, the earlier REST body, MCP, the dashboard) and a rename did not, so two copies of one file both took the repository's name. Both now answer `409 BLUEPRINT_NAME_TAKEN` for a name another approved Blueprint has, compared without case or surrounding spaces. A create that sends no name is not refused for its default: it gets the repository's name, or, when that is taken, the name with the Blueprint's resource suffix (`<repo>-ab12`), or the first free of `<repo>-2`, `<repo>-3`, … for a Blueprint without a suffix. The review returns that free name as `suggestedName` when the name the create would use is taken, worked out against the `name` a form proposes. The dashboard fills it in until the user types their own, and shows a refusal at the name field. A retried create keeps the name its Blueprint has. Blueprints that already share a name stay as they are.

**A planned sync's ownership conflict offers no `confirm` phrase (w8/251, 2026-11-11).** When a Blueprint names a resource another Blueprint manages, the refusal used to end "retry with confirm=…" on every surface. Render's create, plan and approve have no `confirm` field and the pinned CLI has no flag for one, so the advice could not be followed from the surface that printed it. Render also documents no transfer between Blueprints ("Do not manage a particular resource with more than one Blueprint"). A failed plan's notice, and the refusal an approval meets while it applies, now read: `service "api" is managed by blueprint exs-… ("name"), and a resource has one managing Blueprint. For a separate copy, create a new Blueprint with existingResources "create_new". To manage this service here, remove it from that Blueprint's file and sync that Blueprint first, which releases it.` REST, GraphQL and MCP print the same text. The one-step create, `syncBlueprint` and the review made for them accept `confirm`, so they keep the phrase in the message and in `params.confirm`.

### Claims may not outlive the management they record (w4/m125)

`w8/m23` promised that a managed resource "records A as its managing blueprint, visible on the blueprint's `resources[]`". Only half held. `resources[]` is derived from the current manifest, while the durable claim was released in exactly one place — disconnect. So any resource a sync stopped declaring — a takeover, an ordinary IaC edit, a rename — became an orphan that was _also locked_: managed by nobody, adoptable by no other blueprint without a phrase naming a blueprint that disclaimed it. Live, one blueprint id answered both "I do not manage static-site" and "static-site is managed by me" at the same moment.

A sync now diffs its claims against what the manifest still declares and releases the difference, inside the same fenced `(generation, runID)` window as the ownership stamps, so a retired worker cannot release claims a successor just took. The CR marker is cleared with the claim. **Only the claim is released** — the resource keeps running, which stays deliberate (`w4/119`: not deleting it is right).

The claim is also now readable: `blueprintId` appears on every service, database and Key Value read, projected from the CR label — the claim's mirror, written and cleared in the same window — which is why it is affordable on list reads as well as single ones rather than becoming a per-resource query. A dashboard "Managed by Blueprint" cell was considered and deliberately deferred: the blueprint detail page's `resources[]` is the established direction of that link and is, as of this milestone, finally accurate.

**A resource conflict names the manifest kind (w5/098).** `BLUEPRINT_RESOURCE_CONFLICT` carries params `resource`, `kind`, `owningBlueprintId` and `confirm`. `kind` is the kind plan actions use (`service`, `postgres`, `key_value`), so a client labels a conflict exactly as it labels a plan action. The message keeps its display wording, as in `key value "cache" is managed by blueprint …`. `kind` used to carry that display text, which the dashboard had to accept as a lookup key.

## Consequences

### Positive

- A Render user can bring a `render.yaml` to bex and get either equivalent behavior or an actionable refusal.
- Schema drift becomes reviewable work instead of a production surprise.
- Presence-aware planning fixes destructive sync resets and makes previews honest.
- Existing product capabilities stop being hidden behind an incomplete YAML adapter.
- One compiler removes the direct-CR helper's behavioral fork.

### Costs and trade-offs

- Strict validation will reject some files that bex currently “accepts.” That is an intentional correction of false success and requires an audit/migration window.
- The pinned contract can lag Render briefly; the scheduled drift report makes the lag explicit.
- The compiler and current-state planner are a larger abstraction than the present structs, but they replace multiple parsers and scattered semantics.
- bex remains a documented subset where product anti-goals differ from Render.

## Rejected alternatives

### Keep `bex.yml` as a similar native dialect

Rejected. It preserves the ambiguity that caused this problem and makes every Render compatibility claim conditional on undocumented differences.

### Accept the full Render schema and ignore fields bex cannot implement

Rejected. Successful validation followed by a materially different deployment is worse than an explicit unsupported error.

### Fetch Render's schema at runtime

Rejected. The upstream URL is unversioned; an upstream edit could change production acceptance without a bex release or review.

### Maintain a strict API compiler and a simpler shell compiler

Rejected. Two compilers inevitably diverge on validation, authorization, grouping, references, and write ordering.

### Implement persistent disks and previews to claim 100% acceptance

Rejected. Parity is honest behavior, not checkbox maximization. Both features conflict with explicit roadmap anti-goals; they remain visible, field-specific unsupported cases. _(Update 2026-08-22: the disk anti-goal is reversed by [ADR082](ADR082-persistent-disks.md) on its own merits — not to claim acceptance percentage; previews remain rejected.)_

## Verification

`w1/m63` owns implementation. Its conformance corpus must cover official Render examples, every capability-registry state, duplicate and unknown fields, all supported resource locations, create-versus-existing omission semantics, references, explicit empties, secret redaction, multi-error validation, current-state plans, filename discovery, and all public entrypoints. The unmodified official Render CLI must validate representative accepted and rejected files against bex-api.

At closeout, `docs/ADR006-bex-api.md`, `docs/ADR018-render-parity.md`, examples, dashboard copy, MCP descriptions, and the CLI compatibility checklist must be generated from or reconciled with the capability registry. No Blueprint row may use a blanket ✅ without evidence from the corpus.

### Initial environment-group composition (w4/m148)

For a new Blueprint App, group references and authorized membership are prepared before its first CR is published. The first deploy keeps its original identity and generation; it represents the complete initial group configuration. Public create inputs cannot supply arbitrary Kubernetes Secret names. Existing-service link, unlink and group-content updates retain their auto-deploy gate and service-over-group precedence.

A persisted `apps.creation_pending` flag prevents the control-plane projector from reconstructing an incomplete App between identity allocation and initial composition. Completion clears the flag after membership revalidation. Error compensation removes the durable intent before deleting its owned App, including an ambiguous completion response. A process crash can leave an incomplete creation requiring operator recovery; its deploy times out with an initialization failure, and timeout never releases an uncommitted configuration automatically.

Real-Postgres tests cover the initial deploy record, projector interleavings, group Secrets and controlled Kubernetes generation/status transitions. They do not claim real build-job execution or external HTTP acceptance; those remain the milestone's hosted gate. Render documents [group linking and precedence](https://render.com/docs/configure-environment-variables), but authenticated first-deploy timing was not measured. Bex's deterministic group ordering and existing auto-deploy-off update policy remain deliberate choices.

### Detach notices before and after sync (w4/m133)

A blueprint-scoped preview now adds `detach` plan actions for services, Postgres databases and Key Value stores the blueprint currently claims but the proposed manifest no longer declares. Each action carries the existing `kind`, `name`, `resourceId` and `message` fields. A rename therefore shows both the new resource's action and the old resource's detach action. The comparison is restricted to the authorized blueprint's claims; unrelated workspace resources are never removals.

`blueprintPreview` already accepts `blueprintId`. Manifest validation now accepts that optional identifier too, across REST, GraphQL and MCP. Omitting it retains ordinary manifest validation: YAML alone cannot identify which blueprint is being replaced, so that mode cannot promise a complete detach preview.

After a successful sync, `detachedResources` identifies surviving resources that left management. Sync history records the detachment notice in its existing `note` field; the dashboard renders that note after reload and warns before sync in the plan summary. The current declared set may still be `in_sync`: that status is not a claim that every resource ever managed was deleted or stopped. Removed resources keep their independent running and billing lifecycle. Failed syncs must not report the planned detachments as completed ones.

This preserves Render's documented [non-destructive Blueprint removal](https://render.com/docs/infrastructure-as-code#deleting-resources) (rechecked 2026-09-22). Named plan actions, the result list, and history notices are additive bex diagnostics; no matching live Render response shape is claimed. The actual claim release remains the fenced reconciliation from w4/m125.

### Service display labels and Blueprint names (w4/121)

For an existing service, `services[].name` matches its `immutableName`, not its mutable display label (`name`). The dashboard's service details show **Blueprint name** when that value differs from the display name. Use the displayed Blueprint name when authoring a manifest to update that service; changing the manifest name means creating or selecting a different service, not renaming its label. API calls to service-by-ID endpoints should continue to use `id`.

The public-host `slug` is a separate field. It can acquire a suffix to resolve a cross-workspace hostname collision, so do not infer a Blueprint name from the URL or slug. For example, a service displayed as `Customer API` could have `immutableName: api` and `slug: api-abcd`; its manifest must use `name: api`. This exposes the existing identity contract without changing matching behavior.

## Automatic source ordering and recovery (w1/m182, 2026-10-20)

Render documents that pushes modifying a linked Blueprint trigger automatic configuration updates and that Auto Sync can be disabled ([Blueprints](https://render.com/docs/infrastructure-as-code)). It does not document delivery ordering, lease fencing, crash recovery, coalescing or force-push guarantees. The rules below are **bex correctness choices, not measured Render guarantees**.

An accepted automatic intent retains the signed delivery's commit and path. At execution, automatic sync resolves the current linked branch, fetches the file at that immutable commit, and resolves the branch again after admission before staging or workload changes. It uses provider branch identity rather than SHA sorting, ancestry, author dates or arrival order. A genuine force push therefore works in either direction. Pending work can consume a later head that did not modify the Blueprint file and created no further intent. Explicit manual reviewed-commit sync continues to fetch the reviewed commit exactly, even when the branch has advanced.

One automatic intent per Blueprint is leased at a time across API replicas; other Blueprints remain independently eligible. Every claim/reclaim increments its authority token. Automatic admission binds the current token, Auto Sync policy and repository/branch/path to the existing Blueprint execution generation. Stage, mutation-family assertions and successful completion check that authority. Reclaim fences an old active automatic run and lets its successor proceed; a resumed old worker cannot report success, settle the replacement intent, or start another mutation family. As with the existing execution fence, an individual Kubernetes request already past its assertion can finish; the apply is not a transaction across infrastructure providers.

A retryable Git read, a source that advances during fetch/admission, a busy apply or a process restart retains accepted work for lease recovery. Auto Sync off, disconnect or a changed path prevents queued work from applying. Permanent apply refusal settles the intent with a bounded explanation. Existing sync history records the actual immutable consumed commit and an existing `note` names both accepted and consumed commits; intent evidence retains the original accepted commit. REST, GraphQL, MCP and the dashboard continue to read the same history fields.

Verification includes a failing-before delayed-A/current-B regression, force-push and unchanged-file successor cases, pre/post-fetch provider failures, policy races, concurrent real-Postgres claims and reclaimed-worker fencing. A signed-webhook/worker walkthrough uses real Postgres plus a controlled Kubernetes client to exercise rapid/reversed delivery, restart, stale-token replay, force push, Auto Sync off and disconnect. It is not a live GitHub delivery or running workload acceptance test.

## Render template spellings (w5/m143, 2026-10-20)

Render's own Deploy-to-Render templates use four inputs its published `render.yaml` schema forbids. Because the templates ship deploy buttons, Render evidently accepts them. bex now accepts them the same way:

- **Unquoted booleans as env values** (`value: true`): the service receives the text `true`/`false`. 13 of the 170 `github.com/render-examples` files did this. A boolean anywhere else keeps its type and its schema checks.
- **Capitalized plan names** (`plan: Starter`) on services and databases: plans are case-insensitive.
- **A bare `envVars:`** (null): an empty list.
- **Env-var prompt metadata** (`displayName`, `description`, `sensitive`, `required` on an `envVars` entry): accepted and ignored, since they configure nothing. The same keys elsewhere are still schema refusals.

The w8/m53 fence that kept booleans as "located schema refusals" was a reading of the schema, not of Render's behavior; it is withdrawn for env values.

**Corpus guard.** `TestRenderExamplesCorpus` (opt-in: `BEX_RENDER_EXAMPLES_DIR`, files fetched with `scripts/render-examples-corpus.sh` because they are third-party) runs every render-examples `render.yaml` through `ValidateBlueprint`. A file may fail only for:

- a deliberate divergence: previews and `previewValue`, workflows, the 10 GB disk minimum (the reserved `PORT` and `RENDER_*` references were removed by w5/m149 and w5/m148; `region` was removed by w8/152, which normalizes it to `BEX_REGION`);
- a refusal Render makes too: the misspelled `dockerFilePath`, a free `private_service`, retired Postgres plans such as `pro-8gb` (bex sells `pro-4gb` since w2/m171);
- a gap already filed: free-form resource names and dotted env keys, both w5/169.

**`sync: false` in an env group is ignored, not refused (w8/293, 2026-11-12).** Render's spec says "You can't apply `sync: false` to environment variables defined in an environment group. If you do this, Render ignores the environment variable." bex used to refuse the whole file, which blocked three Render templates (`ravendr`, `forem`, `git-scm.com`). The entry is now skipped: validate answers `valid: true`, a plan (create, plan, sync, preview) carries one `warning` notice located at the entry, the key is not prompted for, and a sync leaves whatever the group holds for that key. The corpus guard no longer excuses it.

Any other refusal fails the test as a new class to triage. On 2026-10-20 all 170 files passed this rule.

Through dev-5's REST `POST /v1/blueprints/validate`, the unmodified `retool` and `redash` templates validate. `langfuse4-template`, `hasura-graphql` and `zulip` now fail only on the reserved `PORT`, and `saleor` only on a `RENDER_*` `fromService` reference.

### Preview opt-outs (w8/203, 2026-10-28)

The capability registry permits explicit disabled preview values with no stored effect: root/service `previews.generation: off`, root `previewsEnabled: false`, and service `pullRequestPreviewsEnabled: false`. Root `expireAfterDays` inside an off object is inert. This value-aware compiler rule is shared by validate, plan, apply and sync on every surface; requests to enable previews and preview-only plan/value fields remain refused. REST create/PATCH likewise accept off and the deprecated string `"no"`. Building previews remains a non-goal.

## Blueprint onboarding: plan, then approve (w2/m256, 2026-11-03)

Render CLI v2.35.0 made `render ea blueprints create | plan | approve | update` public alpha. Their contract (the CLI's regenerated client, not Render's public OpenAPI) is what bex serves:

- **Create** — `POST /v1/blueprints` with Render's `createBlueprintRequest` (`name`, `ownerId`, `source {repo, branch?, path?}`, `autoSync?`, `existingResources?`) saves the Blueprint in the `created` status and **plans** its first sync without applying it. The answer is `201 {blueprint, planningOutcome: planned | no_changes | failed, notices, sync}`; `blueprint.status` is `created`. A body without `source` is bex's earlier create, which still creates and applies at once.
- **Plan** — `POST /v1/blueprints/{id}/syncs` re-reads the file at the branch head and plans: `201` with a `created` sync when something would change, `200` with `sync: null` for `no_changes`, and a failed plan persists an `error` sync with the notices (validation errors carry `path`, `line`, `column`).
- **Approve** — `POST /v1/blueprints/{id}/approve {manualEnvVars}` applies the **newest** sync, which must be `created`: a newest sync that failed planning answers `409 BLUEPRINT_SYNC_PLANNING_FAILED`; an applied, superseded or failed-at-apply one answers `409 BLUEPRINT_NOTHING_TO_APPROVE`. Approval applies exactly the planned commit and path (the reviewed-source pin, w8/m41), and admission turns the planned row into the run, so the sync reported as approved keeps its id. The Blueprint leaves `created` (in_sync, paused, or error) with the apply's result.
- **Manual env vars** — a plan's `requiredManualEnvVars` are the `sync: false` keys without a value on services the plan **creates** (an existing service keeps the value it has), as `{serviceType: web|pserv|worker|cron, serviceName, key}`. Approval must supply each (`BLUEPRINT_MANUAL_ENV_VARS_MISSING`) and nothing else (`BLUEPRINT_MANUAL_ENV_VAR_UNKNOWN`); values are keyed per service, so two services may take different values for one key.
- **Auto-sync** does not run while a Blueprint is `created`, as Render documents ("autoSync will not apply when the blueprint has the Created status"). A `no_changes` plan takes a `created` Blueprint to `in_sync` (below).
- **Names** — unique among approved Blueprints in the workspace, ignoring case (`409 BLUEPRINT_NAME_TAKEN`).
- **`update --branch`** — PATCH `branch` (w2/065); the CLI then plans with `POST …/syncs`.

**Plans read the workspace as it is now (w2/m256 t014, 2026-11-08).** Every plan (create, `POST …/syncs`, the sync preview) and every sync compares the file with the resources that exist at that moment, never with the last applied sync. A defined resource deleted since is planned as `create` and the next approved or ordinary sync re-creates it, as Render documents ("the next time you sync your Blueprint, Render detects that a defined resource is missing and recreates it", render.com/docs/infrastructure-as-code). A resource whose deletion was accepted counts as missing even while its finalizer is still tearing it down; every by-id read already answers 404 for it. Planning plans the re-create even while the old App still holds the service name (the plan's admission dry-run meets that name last and does not count it); applying it then answers `409 BLUEPRINT_SERVICE_STILL_DELETING` before anything is written, and planning and approving again after the teardown re-creates it. A create whose plan cannot be made at all (not one whose file has problems, which is recorded as a failed plan) leaves nothing behind: its row is removed, so retrying the create starts clean. (Until w8/m153 a create after a delete revived the deleted row, and this undo put it back; a deleted row is no longer touched.) Two more rules keep a `created` Blueprint from getting stuck:

- A plan that finds no changes needs no approval, as on Render ("No changes to approve"). It supersedes any plan still waiting, and a `created` Blueprint becomes `in_sync`: it stores the planned file and stamps `lastSync`, so manual sync and auto-sync run it from then on. Its resources come under its ownership on its next sync, because no apply ran.
- Approval reads only the current connection's plans. A Blueprint revived by a re-create before w8/m153 keeps its history, but the earlier connection's applied sync never stands in for the new connection's plan. Since w8/m153 a create after a delete is a new Blueprint, so it has no earlier connection.

**Existing resources (w2/m261, 2026-11-03).** `adopt` (Render's default) brings same-named resources under the Blueprint. `create_new` mints a 4-character base36 suffix once at create (Render's shape, as on its `onrender.com` slugs) and appends `-<suffix>` to every service, Key Value, database and env group the Blueprint declares, at the root and inside project environments, and to every `fromService`/`fromDatabase`/`fromGroup` reference. The rename happens on the decoded file before normalization, so the plan, adoption by name, `requiredManualEnvVars` and every later plan, approval, sync and auto-sync see the suffixed names, and validation errors still point at the customer's YAML. Projects and environments keep their names: they group resources rather than being managed copies of them. References to a resource the file does not declare (an existing env group or a uniquely named workspace database the Blueprint links to) keep their names. A Blueprint created after another on the same source was deleted is a new Blueprint and mints its own suffix: the deleted one's resources stay running, as Render leaves a disconnected Blueprint's resources. The one-step create takes the same two modes since w8/m153 (`adopt` by default) and is always its own Blueprint: it no longer replaces a connection or clears another Blueprint's suffix (see [A Blueprint is its id](#a-blueprint-is-its-id-several-may-track-one-file-w8m153-2026-11-11)).

**Approving from the dashboard (w2/m256 t012, 2026-11-03).** A created Blueprint applies only by approving its plan. The manual sync (REST `POST /v1/blueprints/{id}/sync`, GraphQL `syncBlueprint`, MCP `sync_blueprint`) refuses it with 409 `BLUEPRINT_AWAITING_APPROVAL` and creates nothing, so no client can skip the plan or its `requiredManualEnvVars`. A planned sync stores its actions (`blueprint_syncs.plan_actions`, migration 0187), and GraphQL `blueprintPlannedSync` reads the waiting plan without planning again. The Blueprint page the API's `dashboardUrl` links to shows that plan, a required field per manual env var and **Approve** (or **Plan again**) in place of **Sync**. This matches the shape of Render's Blueprint review page, which lists the resources a sync would create before **Apply** (render.com/docs/infrastructure-as-code); a CLI-created Blueprint on render.com's dashboard was not walked live.

**Divergences.** A create without `source.branch` reads the repository's default branch, as on Render (w5/181), and falls back to `main` only when GitHub cannot be asked. `rawPlan` (Render: "not a stable contract") carries bex's plan actions. GraphQL (`createPlannedBlueprint`, `planBlueprintSync`, `approveBlueprint`) exposes the same verbs as bex extensions; MCP serves them under Render's own tool names and arguments (`create_blueprint`, `plan_blueprint`, `approve_blueprint`, `get_blueprint`, `update_blueprint`; w5/m283), with `create_blueprint` defaulting `existingResources` to `create_new` as Render's MCP server does.
