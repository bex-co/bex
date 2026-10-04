# m145 implementation and verification — 2026-10-02

The original existing-reference failure is covered without changing the serving release. Effective service-local Save-only writes now attach an opaque random notification to the App; no-op writes return before notification. A separate App-only status controller compares the saved source names and bytes against the successful release record. It patches only `undeployedChanges` with a resource-version precondition. The runtime controller retains its generation filter and cannot dispatch a deploy from that notification.

The status controller also rechecks configured/pending Apps after 60 seconds. This repairs a comparison racing provisional Secret projection followed by failed App-patch compensation, including cases with no committed App event. First-source pending membership stays outside the current runtime template during unrelated reconciliation. Successful candidate dispatch alone does not clear the flag: the comparison follows the active serving revision. Runtime status retries preserve a newer independently written flag.

Missing or unreadable historical evidence is conservative. A legacy release may recover source names from its owned live template but cannot reconstruct old values from mutable Secrets; no migration rollout is introduced. Optional deleted sources get empty release snapshots on the next deploy, distinguishing an applied deletion from lost evidence. Existing broader cancellation status for plan/pre-deploy fields is preserved. Cron release membership is recorded before the template is applied; long snapshot label values use the existing Kubernetes name helper.

## Caller and scope audit

| Surface | Evidence and scope |
| --- | --- |
| Service writers | Three adapters (`secrets/rest.go`, `graphql.go`, `mcp.go`) call `PatchEnvironment`; sparse and CAS branches are the two `finalizeEnvironmentPatch` callers. One metadata notification site covers both. Auth, existence, validation and revision checks still precede the mutation. |
| Service reads | `AppView` reads the status field; REST and MCP share the Render service serializer (false remains omitted), and GraphQL service/server aliases carry the exact bool. Adapter tests exercise real handlers/resolvers. |
| Runtime families | Deployment-backed web/private/worker and CronJob configuration comparison are covered. Static-site write safety is tested, but static publication has no equivalent retained build-input record and is excluded from the new status controller. OpenSandbox, Postgres and Key Value are outside this comparison. |
| Linked groups | `envgroups/patch.go` has a separate versioned transaction. Referenced group values are compared on the periodic status check, but this change adds no group notification or new transaction/rollout guarantee. |
| UI | Two notice predicates (service header and Environment route) consume the same Server bool. The shared editor has service and static-site callers. No optimistic true flag or extra secret reveal/deploy is introduced. Save-only feedback states that values were saved without deploying; a committed mutation followed by either a rejected refetch or an Apollo result error ends the draft with reload feedback. |
| Store replay | `store/reconciler.go` updates the current App's spec/labels and preserves metadata, including the notification. Recreating a deleted CR does not preserve its old metadata; that existing recovery boundary is not certified here. |

## Behavioral evidence

- Backend tests cover repeated existing-file v1 → v2 → v3 → v1 saves, no-op/fixed-clock behavior, first sources, deletion/mixed edits, sparse/CAS conflicts and failed notification compensation. Compensation retains a previously pending projection as well as sources already in spec. No Save-only write changes release identity or opens a deploy row.
- Operator unit tests cover saved versus serving values, source membership, second saves, reverts, applied deletion, candidate-versus-Ready clearing, missing/legacy/error cases, competing resource versions and runtime status retries. Existing cancellation/rollback/Restart coverage remains; cancellation envtest harnesses now drive both independently registered controllers.
- The new real-manager envtest observes metadata-only existing-file notifications through the API/cache for v2 → v1 → v3. App generation/spec/release identity, Deployment resource version/template and the v1 snapshot remain unchanged while status becomes true → false → true. Fixtures and the manager are cleaned up.
- Before-fix evidence: the original operator runtime code fails the new assertion with `ordinary runtime reconciliation erased the saved/runtime difference`; old backend code misses effective-save notifications; removing pending-reference compensation loses a previously staged projection. The old dashboard hook misses refresh-failed feedback after an accepted mutation. These intentional failing runs were isolated with Go overlays or temporary files, which were removed before final checks.
- Local evidence logs: `/tmp/bex-m145-operator-before.log`, `/tmp/bex-m145-backend-before.log`, `/tmp/bex-m145-compensation-before.log`, `/tmp/bex-m145-dashboard-regression-before.log`.

## Simplify dispositions

Three concurrent reuse, quality and efficiency reviews completed. Applied: share source-membership projection through `applySources`; validate selected runtime snapshots without rereading mutable sources; avoid a second unchanged cron record write; correct the editor's awaited-refetch comment. Kept fail-closed snapshot validation and the bounded compensation-race recheck. A tiny cross-test fixture-helper merge was not worth expanding unrelated test coupling.

## Parity and acceptance boundary

[ADR004](../../../../docs/ADR004-app-deployment.md#ordinary-save-only-differences-w4m145) and [ADR018](../../../../docs/ADR018-render-parity.md) record the verified scope. Render's primary environment, Restart and rollback documentation was rechecked on 2026-10-02: deferred environment application, running-config Restart and preserved saved settings remain intact. The bool/notices and shared three-choice secret-file editor are Bex extensions.

No production fixture or QA session was created for this implementation. Local tests do not satisfy t006's external HTTP/API/UI replay. Release owner must deploy backend/operator/dashboard, then QA must repeat both ordinary saves and cancel/deploy/rollback/Restart/hook controls, check list/detail/Events and notices, and delete the owned fixture, verify by-id 404/no runtime residue, and revoke its session. The milestone remains blocked until that evidence exists.

## Required checks

- Full backend `go test ./...`: PASS (`/tmp/bex-m145-backend-test.log`). Optional real Postgres/OpenFGA integrations were not configured and are not claimed.
- Dashboard `yarn lint && yarn test`: PASS, 459 files / 3,936 tests (`/tmp/bex-m145-dashboard-{lint,test}.log`).
- Types `go test ./...`: PASS (`/tmp/bex-m145-types-test.log`).
- Operator `make test`, including generated manifests, vet and all 186 envtest specs: PASS (`/tmp/bex-m145-operator-test-verified.log`, controller package 219.693s). The earlier final run exposed a shared-process controller-name collision; the new test now uses the existing independent-manager name-validation setting. Production name validation remains enabled.
- Workspace `make lint`: PASS, zero issues in all four modules plus deadcode (`/tmp/bex-m145-go-lint-final.log`).
- Focused final controller regressions: PASS (`/tmp/bex-m145-controller-regressions-final.log`).
- Skill layout validation and repository Markdown formatter: PASS.

After a clean autostash/rebase to `c92ebb526`, backend apps/secrets suites and the real-manager notification/startup envtests passed again (`/tmp/bex-m145-{backend,operator}-merged.log`). Upstream changes were datastore-specific; the only overlapping edited file was ADR018, merged without conflict.
