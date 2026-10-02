# Persistence handoff verification — 2026-10-02

## Transition contract

Legacy empty mode means Journal + Snapshot. The PVC marker records the safely
applied file format; the owned serving pod supplies the initial legacy
format. Unchanged legacy workloads are not rolled merely to install the handoff. A completed live handoff is recorded before a newer requested generation
can supersede it. An initializer consumes that handoff once.

| Applied source | Requested target | Authority and rollout prerequisite |
| --- | --- | --- |
| Journal + Snapshot | Journal + Snapshot | Current journal; ordinary restart, no conversion. |
| Journal + Snapshot | Snapshot only | Load current journal, save the complete RDB, stop the conversion process, then commit the snapshot marker. |
| Snapshot only | Journal + Snapshot | For a running store, enable AOF on the exact serving process and verify rewrite completion before replacing its workload. A suspended store converts its saved RDB on resume. |
| Snapshot only | Snapshot only | Current RDB; ordinary snapshot-mode shutdown/restart semantics. |
| Either durable mode | Off | Commit discard intent, remove managed persistence files, then start empty. |
| Off | Either durable mode | Clear historical managed files before initializing the requested format; never resurrect a prior durable dataset. |
| Off | Off | Pod initialization clears managed persistence files; no automatic persistence is enabled. |

The last three rows cover all five pairs involving Off. Unchanged API saves do
not themselves require a new pod. The initializer runs on pod initialization,
not every same-pod container restart. Ordinary snapshot crash-loss semantics
remain distinct from a successful managed durable-mode transition.

## Shared paths and boundaries

- Existing-resource writes converge on `KeyValuePatch.apply` in
  `lego/backend/internal/keyvalue/service.go` or `ApplyBlueprintKeyValueSpec` in
  `lego/backend/internal/apps/blueprint_plan.go`. Blueprint and direct-CR changes
  reach the same operator projection as REST, GraphQL and MCP.
- REST underscore/hyphen normalization, omitted fields, invalid-mode refusal,
  protected-environment checks and the Free durable default remain in their
  existing shared backend paths.
- The existing tenant datastore-control policy in
  `lego/backend/internal/store/namespaces.go` admits the operator's namespace;
  this change needs no new network-policy or pod-exec permission.
- Postgres, web, static, cron, worker and private services do not use this
  KeyValue conversion path.
- Desired mode is not an assertion of completed application. The new
  `TestPersistenceTransitionStatusAcrossAdapters` verifies REST, GraphQL and MCP
  expose `config_restart` for accepted/pending changes, `unavailable` for a
  failed transition, and `available` only for current Ready state.

## Verification

- Full backend `go test ./...` passed. The external integration environment
  variables were not set for this run; this is not a claim of real Postgres or
  OpenFGA integration coverage.
- Full dashboard suite: 458 files, 3,933 tests passed. Dashboard lint, typecheck
  and knip passed. Confirmation tests cover both Off directions, cancellation,
  incorrect phrases and retry after a failed save; durable-to-durable changes
  remain direct.
- The local dev-4 stack was brought up using the healthy shared CAPD cluster;
  its old kubeconfig pointed to a retired endpoint. Other workstreams' stacks
  were not rebuilt.

### Test isolation incident

At 2026-10-02 21:46:43 UTC, an initial controller test accidentally called the
real Redis client against the pre-existing Homebrew Redis on `127.0.0.1:6379`.
Its commands enabled AOF; no keyspace writes, deletion or restart occurred.
The service config, startup log, first-rewrite log and AOF file creation time
established the prior `appendonly no` setting. It was restored with
`CONFIG SET appendonly no`, and `CONFIG GET` plus `aof_enabled:0` verified the
correction. Newly generated AOF files were retained, not deleted. The controller
tests now inject their engine callback and fail if an unsafe source invokes it;
the isolated Docker engine tests use their own allocated ports and volumes.

## Final local acceptance

- `GOWORK=off BEX_TEST_VALKEY=1 make test` passed: operator unit tests, envtest,
  code generation, and actual persisted-volume Valkey 7.2.14/8.1.9 tests. The
  initializer runs as UID 999 / GID 1000 in the engine tests. The operator CI
  workflow now requires this engine suite instead of silently skipping it.
- The original startup-flags projection reproduced the saved counter rollback
  from 23 to 1 and loss of the new key. The fixed path preserved both, original
  expiration deadlines, writes acknowledged after handoff, and a forced stop
  without a final snapshot. All nine mode pairs, repeated tokens, reversal,
  suspended conversion, interrupted conversion and failed-rewrite retry passed.
- Additional real-engine tests verified missing manifests and invalid markers
  fail closed on both pinned majors. Unit tests cover actual-pod mode/revision,
  process restarts, stale informer reads, desired-generation supersession,
  pending/failure status and withholding an unsafe workload update.
- `make lint` passed across all four Go modules and whole-program dead-code
  analysis. The GitHub Actions validator and diff whitespace check passed.
- Three-agent `/simplify` review completed. It reused `podReady`, separated raw
  template mode parsing from the legacy fallback, applied pod defaults once,
  skipped unchanged marker writes and bounded failed-engine reads/cleanup.
- Follow-up legacy tests prove pending-template flags cannot replace the actual
  serving format, genuine old Off flags remain unchanged on upgrade, and
  zero-replica revision convergence cannot authorize an unknown source.
- A final route-navigation regression proves a pending destructive confirmation
  cannot transfer to another Key Value. Its focused suite passed 21 tests; the
  regression failed when only the production resource key was removed.
- Fourteen focused unknown-mode tests passed after disabling edits and saves on
  absent, failed or unrecognized reads, including already-open drafts. Final
  dashboard lint/typecheck/knip passed.

Known-new storage gets an explicit pending bootstrap marker and initializes the
requested durable format before committing its ordinary marker. Before-first-start
mode changes are supported. Existing durable sources are validated on same-mode
starts too; a missing initialized journal, snapshot, or marker on a populated
volume cannot silently become an empty database.

An unmarked legacy suspended store with no serving pod has an explicit safety
limit: zero-replica revision convergence does not prove the last persisted mode.
A changed durable rollout/resume fails closed with `PersistenceSourceUnknown` until a
platform operator establishes that mode. Explicit Off remains a destructive reset. New/already-initialized stores convert
on resume automatically; unchanged legacy suspended workloads remain untouched.

Production acceptance has not been run against this change. The live
Free/public TLS, fresh-page, rename, three-surface and exact-cleanup sequences in
the milestone README remain required after the operator and dashboard deploy.
This gates the hosted controls in t003/t004 and acceptance in t006/t007; implementation and local verification are complete.

## Concurrent implementation history

The separate unshipped attempt from commit `c898468ec` is retained unchanged in
`implementation.patch`, with its original checks and Docker limitations in
[prior-implementation-verification.md](prior-implementation-verification.md).
Its history exposed additional legacy/bootstrap cases covered by this run.
The current implementation supersedes that patch; do not apply it over the
shipped code. This run's healthy Docker results clear the prior local-engine
gate, while the production acceptance gate remains.

Concurrent commit c92ebb526 also shipped a replacement offline initializer. Its [original evidence](current-implementation.md) is retained. The rebase reconciles that code with the live-process handoff and reruns affected checks.

## Rebase verification

The combined backend suite passed. The combined dashboard suite passed 459 files
and 3,955 tests, followed by lint, typecheck and knip. Independent controller
review retained both implementations' safeguards, including active AOF manifest
entries, completed rewrite counts and the uncached desired-generation readiness
fence.

The added empty-manifest test initially failed in its recovery boot: both pinned
Valkey versions parse a manifest even with journaling disabled. The initializer
had correctly refused it. The corrected fixture checks SHA256 equality of the
snapshot, marker and journal files after refusal, explicitly restores its
original manifest, and then verifies journal recovery. All corrupt-manifest
cases passed on both majors; the production guard was not weakened.

Final combined verification passed: engine-enabled operator make test
(controller package 324.821s), all-module make lint/deadcode, and the final
focused corrupt-manifest matrix (40.077s). The workflow validator also passed.
