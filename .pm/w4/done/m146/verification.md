# Empty Blueprint environments — verification

## Implementation and regression

The shared `mergeBlueprintEnv` returns nil when both inputs are empty, matching
the stored JSON representation. Nonempty merges keep their existing ordering,
ownership and allocation behavior. This fixes both planning and apply without
adapter or dashboard special cases.

The new serialized regression failed on the original code: unchanged static
empty environments reported update/envVars, and both repeated applies opened
deploy records and changed restartedAt/resourceVersion. With normalization, the
five App kinds (web, static, worker, private and cron) pass empty, omitted,
group-only and seed-only cases. Two applies after JSON reload preserve identity,
specification, generation, serving revision and resource version, with no new
deploy records. Separate controls preserve undeclared literals/SecretRefs and
detect genuine environment updates. Group links and seed callbacks retain their
existing ownership paths.

The 12 adapter cases cover REST multipart, GraphQL and MCP: explicit-empty and
omitted environments report noop, a publish-path change reports exactly that
changed field, and a new name reports create. GraphQL's empty changed-fields
list and REST/MCP's optional omission retain their existing shapes.

## Shared scope

- One merge caller: the service envVars field applier. Three exported-applier
  call sites serve the overall plan, per-field probe and apply wrapper.
- Five action-plan callers cover direct stack preflight, validation, connected
  create, sync preparation and sync fallback. Preview and Git auto-sync use
  these same paths.
- `classifyServiceEnv` separates literals, seeds, group links and references;
  references resolve before the two compiled service apply passes. The legacy
  fields-nil path and separate Database/KeyValue appliers are unchanged.
- Existing tests retain database-first/SecretRef handling, forward references,
  mixed-stack validation atomicity, domain/list round trips, authorization and
  nonowned fields. No new authorization, value disclosure or error mapping is
  introduced.
- Both dashboard routes use `BlueprintPlanSummary`, which excludes noop from
  change totals and handles absent changed fields. No frontend change is needed.

## Parity and review

The official [Blueprint specification](https://render.com/docs/blueprint-spec)
and [lifecycle documentation](https://render.com/docs/infrastructure-as-code),
checked 2026-10-02, describe updating affected resources while preserving omitted
environment values; sync:false is initial-create-only and generated values are
preserved. These support the preservation contract. Exact wire shapes and
no-deploy assertions here are Bex checks, not claims of a live Render mutation.
Existing ADR049/ADR018 contracts need no wording change.

Three parallel simplify reviews covered reuse, quality and efficiency. No
further changes were justified: the empty branch avoids allocations and an
unnecessary nonempty clone; tests reuse existing service/store/transport helpers.

The cron fixture with an explicit command exposed independent create/reapply
command drift. The image-default cron fixture isolates this environment fix;
the separate finding is tracked in inbox 174 rather than silently ignored.

## Remaining hosted acceptance

The release pipeline must deploy the backend fix. QA must then replay the
README's free static HTTP and three-adapter checks, both repeated applies with
a fresh page, omitted-env/path/new-name controls, and group-only web control,
then delete owned fixtures and revoke its session. No production fixture was
created by this implementation run. Live sibling/reference and connected Git
cases are not inferred from local checks. t003 hosted parity and t006 closeout
remain open.

## Final local checks

GOWORK=off go test ./... passed for the complete backend (apps 7.135s). Backend lint passed with zero issues, and diff whitespace checks passed. External Postgres/OpenFGA integration variables were unset; those opt-in suites are not claimed as exercised.
