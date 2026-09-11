# w4 · m99 — Restore image-create compatibility and long-name alias admission

**Worker:** worker4 **Goal:** Valid CLI image creates accept an explicitly disabled setting, and long service names reconcile their required platform aliases so healthy first deploys reach live. **Status:** in progress — implementation and local regressions complete; live acceptance pending

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Accept explicitly disabled auto-deploy on prebuilt image creates | 35m | — |
| t002 | Admit bounded platform alias names without weakening target ownership | 45m | — |
| t003 | Verify both shared validation and alias families | 45m | t001, t002 |
| t004 | Render parity across create and deployment surfaces | 20m | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test the CLI failures and admission boundary | 35m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- Repeat t001's exact CLI create with a fresh owned free fixture and `--auto-deploy=false`: exit 0, disabled no/off state, and no unsupported Git automation. Omitted flag still succeeds.
- Repeat t002's long- and short-name image-service creates in a production-equivalent tenant namespace: both initial deploys reach `live`, both HTTPS endpoints serve 200, and legitimate bounded aliases pass admission with fixed targets and exact App ownership.
- Meaningful enabled/invalid-value and foreign-owner/actor/target admission controls preserve rejection. Shared callers have behavioral regression coverage.
- Delete each recorded fixture ID and verify detail/list/instances absence; no pre-existing resources are changed.

## Source + Goal linkage

- **Source:** continuous `$qa-find-bugs-cli` requested for w4, 2026-09-09 PDT / 2026-09-10 UTC. Durable sanitized commands, wire bodies, output and attribution are in t001/t002.
- **Goal linkage:** Render-compatible hosting (ADR006/ADR018), tenant namespace isolation (ADR043), truthful image/Blueprint configuration (ADR049), and working free-tier wake routing (w6/m47).
- **Expected outcome:** no false rejection of explicit disabled image auto-deploy; no failed initial deployment caused by the operator's own long-name alias format.
- **Why now:** the shipped v2.27.0 CLI makes both journeys reproducible against production; current main still contains both causes. Implementation is now present; the live runtime acceptance below still gates closeout.
- **Render parity included:** both successful create semantics and deployment state are tenant-visible. Shared-code blast-radius work is separate t003.

## Hunt cleanup and limits

The long runtime fixture and short live control were deleted with the installed CLI; detail and instance lookups returned 404. The initial port-10000 fixture was also deleted; it is not positive runtime evidence. Final CLI list reconciliation returned exactly the original 10 IDs: zero baseline resources missing and zero owned fixtures remaining. No paid resources, workspace settings or pre-existing resources were changed. Platform revision was not observable; platform fixes and post-fix acceptance remain unverified.

## Implementation checkpoint — 2026-09-11

- The shared image-source registry accepts explicit false, and Blueprint validation accepts false/off. Enabled automation and Git-only source/build settings remain rejected. REST and all MCP service-create tools reject unknown legacy auto-deploy strings; GraphQL retains its Boolean input type. The valid trigger's existing REST precedence is unchanged.
- Admission uses a purpose-specific prefix plus the exact full name through 63 characters, otherwise the constructor's first 54 characters, a separator, and eight lowercase hex digits. The constructor itself is unchanged. CEL does not recompute SHA-256; trusted operator identity, controller owner name/kind/API version, labels, fixed targets/ports, and both namespace bindings remain enforced.
- Actual Kubernetes API-server admission tests exercise the operator's static/maintenance/activator constructors at full-name lengths 62, 63, 64 and the maximum App-label length, in both default and hosting namespaces. CREATE/UPDATE controls cover wrong actors, targets, ports, owners, labels, selectors, type transitions, and malformed names.
- Backend regressions cover direct image-capable types, the exact CLI wire body, REST/GraphQL/MCP creation, Blueprint validation/apply/reapply/current-state planning, and invalid/enabled neighbors. The installed Bex v0.2.1 CLI successfully creates with both omitted and explicit false flags against the composed changed backend with a synthetic human OAuth grant and in-memory Kubernetes storage. This is binary/adapter evidence, **not live runtime evidence**.
- All four `specFromCreate` production callers were inspected: direct create, Blueprint current-state planning, pre-apply validation, and stack create/update. Both registry loops share the neutral-value decision. Existing PATCH/source-switch behavior remains covered by the backend suite. Dashboard image-create input deliberately omits autoDeploy (`dashboard/src/features/services/lib/create-service-input.ts`), while returned state uses the tested GraphQL projection; no UI change is needed.
- Validation: full backend `GOWORK=off go test ./...`, operator `GOWORK=off make test`, focused real-admission tests, installed-binary integration test, and `bash scripts/gitops-validate.sh` passed. Full `make lint` passed using a private Go 1.27-built pinned deadcode analyzer; the existing cached analyzer had been built with Go 1.26 and could not load the CLI module. The database/OpenFGA env-gated integration suites were not supplied external services by this local run. Simplify reuse/quality/efficiency reviews completed; stronger policy-specific negative assertions and early fixture-cleanup registration were applied.
- Earlier QA fixture `srv-dah41v9c7cos73dm39ug` was recovered from this run's private creation ledger and deleted. Final detail/instances returned 404; resource-list reconciliation found no run-owned IDs and no missing baseline IDs. No pre-existing tenant resources were mutated.

### Remaining acceptance (do not close early)

Implementation shipped as `818de80b8`. The long-name first deploy, HTTPS, GraphQL agreement, free sleep/wake and fixture teardown now pass ([live evidence](live-acceptance.md)). The explicit disabled create and short-name control still need the updated backend, followed by final baseline reconciliation and isolated-session cleanup. The configured local kind and OrbStack API servers are unavailable; the production cluster is reachable. No production policy/workload was changed directly during implementation. Keep the milestone open until the remaining observations exist; do not substitute local regression tests for deployed behavior.

## Live acceptance progress

[2026-09-11 live evidence](live-acceptance.md) proves the long-name first deployment, HTTPS, GraphQL agreement, actual alias ownership, sleep/wake, and fixture teardown. The short-name and explicit disabled-create acceptance still await the backend update; the milestone remains open.
