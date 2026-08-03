# ADR049 — `render.yaml` is bex's Blueprint contract

**Status:** Accepted 2026-08-02; implementation handed to `w1/m63`.

---

## Context

bex calls its application manifest `bex.yml` while describing it as “render.yaml-shaped.” That wording overstates compatibility. The current implementation is a useful Blueprint subset, but it is not a contractually defined Render-compatible dialect:

- new Git-connected Blueprints, the dashboard, the store, examples, MCP descriptions, and helper scripts default to `bex.yml`, whereas Render defaults to `render.yaml` and permits an explicit custom path;
- `lego/backend/internal/apps/deploy.go` decodes into handwritten Go structs with permissive `yaml.Unmarshal`. `validateBlueprintDialect` rejects a short list of retired bex spellings, but an unknown or unsupported Render field can still be accepted and discarded;
- the decoder loses field presence for many scalar and collection fields. The update path then copies zero/default values into existing App, Database, and Key Value specs, even where Render distinguishes “omitted,” “explicitly empty,” and “set”;
- `autoDeployTrigger: checksPass` currently becomes the same boolean as `commit`, even though bex does not implement a branch-check gate;
- adapter gaps remain for capabilities that bex already has elsewhere, including registry credentials, platform-subdomain policy, static routes and headers, Postgres disk autoscaling, and PgBouncer;
- `scripts/app-apply.sh` is a second manifest compiler that bypasses the Blueprint store, authorization, validation plan, sync history, and several canonical resource shapes;
- validation reports the first semantic error, its plan counts declarations instead of current-state actions, and its multipart limit is 2 MiB rather than Render's documented 10 MB;
- `docs/ADR006-bex-api.md` and the Blueprint row in `docs/ADR018-render-parity.md` contain claims that no longer match either the implementation or Render's current behavior.

This creates a dangerous false-success mode: a user can submit a valid Render Blueprint, receive a successful bex validation or sync, and end up with materially different infrastructure. An explicit “unsupported” error is incomplete parity; a successful no-op is incorrect behavior.

### Upstream contract researched

Research was refreshed on 2026-08-02 against Render's primary sources:

- the [Blueprint YAML reference](https://render.com/docs/blueprint-spec) says the default root filename is `render.yaml`, defines resource-specific required fields and create-versus-existing omission behavior, and distinguishes `autoDeployTrigger: commit`, `checksPass`, and `off`;
- Render's [official JSON Schema](https://render.com/schema/render.yaml.json) is JSON Schema 2020-12, rejects unevaluated root fields and additional resource fields, and requires fields such as service `type`, `name`, and (except Key Value) `runtime`. The snapshot fetched for this decision had SHA-256 `665539cb0c191856ba38d292b985a963880bb69b030d666e5fe7788e78e7e696`;
- the [Validate Blueprint endpoint](https://api-docs.render.com/reference/validate-blueprint) accepts `multipart/form-data`, requires `ownerId` plus a file, permits files up to 10 MB, and validates without mutation;
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
- an explicit empty list is distinct from an omitted list;
- `pro plus`, `pro max`, and `pro ultra` use Render's Blueprint spellings at the boundary and translate to internal tier IDs only after validation.

Resource deletion remains manual. Render now explicitly documents that Blueprint sync never deletes a resource removed from the file, so bex's no-sync-delete behavior is parity, not a divergence.

### D6 — Validation and planning match the Render-compatible wire contract

The REST validation endpoint keeps Render's multipart shape and 200-with-validation-result behavior, raises its Blueprint file limit to 10 MB, and reports every independently actionable error it can safely discover. Each error carries a stable code, JSON-style path, message, and source line/column when known. Errors and plans never contain secret values.

The plan is a current-state diff, not a declaration count. It identifies create, update, no-op, unsupported, and conflict actions and is the exact immutable input to execution. Validation without store access may return a clearly labeled structural plan; it must not present declaration counts as the number of changes.

Dashboard, GraphQL, and MCP may wrap the result in their native envelope, but field paths, codes, semantics, and plan actions come from the same core result.

### D7 — Close adapter gaps; reject unavailable product features

The first capability-registry pass must distinguish an adapter omission from a missing product capability.

Fields backed by existing bex mechanisms are implementation work for `w1/m63`, including:

- `dockerCommand`, platform-subdomain policy, static `headers`/`routes`, and image/build registry credentials;
- Postgres `storageAutoscalingEnabled`, `connectionPool`, and `fromDatabase.property: connectionPoolString`;
- environment-scoped env groups, nested/ungrouped resource inventory, `sync: false` value collection, and documented references to existing workspace resources;
- exact plan spellings, defaults, omission behavior, and deprecated-field precedence.

Fields whose semantics bex cannot truthfully provide are rejected. In particular:

- `autoDeployTrigger: checksPass` is unsupported until bex can observe and gate on branch checks; it is never collapsed to `commit`;
- service/database `region` is unsupported while bex exposes one configured placement rather than per-resource Render regions;
- service `disk` is unsupported under the stateless-first persistent-disk anti-goal;
- root and per-resource preview configuration is unsupported under the explicit PR-preview-environment anti-goal;
- a newly discovered upstream field is unsupported-by-default until classified.

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

Rejected. Parity is honest behavior, not checkbox maximization. Both features conflict with explicit roadmap anti-goals; they remain visible, field-specific unsupported cases.

## Verification

`w1/m63` owns implementation. Its conformance corpus must cover official Render examples, every capability-registry state, duplicate and unknown fields, all supported resource locations, create-versus-existing omission semantics, references, explicit empties, secret redaction, multi-error validation, current-state plans, filename discovery, and all public entrypoints. The unmodified official Render CLI must validate representative accepted and rejected files against bex-api.

At closeout, `docs/ADR006-bex-api.md`, `docs/ADR018-render-parity.md`, examples, dashboard copy, MCP descriptions, and the CLI compatibility checklist must be generated from or reconciled with the capability registry. No Blueprint row may use a blanket ✅ without evidence from the corpus.
