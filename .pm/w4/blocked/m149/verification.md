# Key Value TLS cleanup — implementation verification

## Protocol

The TLS finalizer is independent of the existing backup finalizer. Every live
canonical KeyValue is enrolled before its Certificate can be created, including
Free/private instances and historical public instances. Private/no-issuance
cleanup succeeds when the exact Certificate and Secret are absent. No new backup
job or purge obligation follows from TLS enrollment. A legacy paid resource
already held by the backup finalizer also performs TLS cleanup before releasing
that existing obligation.

The Certificate producer stamps KeyValue UID provenance on its Secret template.
Deletion verifies the canonical namespace and lifetime, records the owned
Certificate issuance identity durably, deletes with object preconditions, and
observes the producer absent before touching its issued Secret. Secret reads use
the uncached tenant client. Legacy ownerless Secrets require exact reserved
names, matching cert-manager metadata and creation within the current lifetime;
foreign or ambiguous artifacts fail closed. Pending deletion and access errors
retain cleanup state; namespace termination can own local garbage collection
without releasing a backup obligation.

This is not a bulk cleanup of already orphaned Secrets. A legacy Free resource
already deleted before enrollment has no parent lifetime left to finalize.
Unproven legacy material requires operator investigation rather than automatic
adoption. No live cluster or cert-manager flags were changed.

## Verification

Restoring the original deletion handler through a Go overlay makes
`TestKeyValueTLSDeletionLifetime/issued_ownerless` fail because the issued TLS
Secret survives. The current focused TLS/public-front-door/backup suite passes.
The producer test also proves that a failed finalizer write prevents issuance.
Three simplify reviews completed: existing provenance, namespace, defaulted-spec
and status helpers are reused; local preconditioned deletion preserves ownership
where the shared delete-and-wait helper cannot. No shared lifecycle helper or
Secret cache/RBAC scope changed.

The audit harness is wired into the existing scripts CI workflow, and the
workflow policy validator and ShellCheck pass. `GOWORK=off GOFLAGS=-p=2 make
test` passed, including code generation and envtest (controller package 92.835s).
The final focused TLS/public-front-door/backup follow-up passed (0.756s after the final test-fixture refactor),
including eight additional cases: never-issued private, namespace termination
while retaining the paid backup duty, noncanonical namespace, foreign
Certificate, Secret read refusal, replacement UID, updated resourceVersion,
and delayed Secret deletion requiring observed absence. The final all-module `make lint` passed: four modules report zero issues and
whole-program dead-code analysis completed successfully.
The local controller tests do not run cert-manager or Kubernetes garbage
collection. An isolated shell harness tests exact TLS inventory and API-read
failures without sourcing credentials or contacting a cluster.

## Remaining acceptance

The release pipeline must deploy the operator. QA must then create fresh Free
public and private fixtures, capture the public TLS Secret UID, delete through
fresh detail pages, and verify that complete inventories reach zero without
manual deletion. REST/GraphQL/MCP reads and fresh Overview must retain their
existing not-found/absence behavior. The corrected audit must pass, with any
optional S3 checks explicitly skipped. Delete owned fixtures and revoke the QA
session. No hosted fixtures were created by this implementation run.

## Shared consumer and parity evidence

Source census (local, no hosted requests):

| Surface | Exact source/test evidence | Preserved contract |
| --- | --- | --- |
| REST delete | `lego/backend/internal/keyvalue/rest.go`, `RegisterREST` DELETE handler → `Service.DeleteKeyValue` in `service.go` | `DELETE /v1/key-value/{id}` returns 204 after accepted CR deletion; cleanup converges asynchronously. |
| GraphQL delete | `graphql.go`, `GraphQLMutation` `deleteKeyValue` → the same `Service.DeleteKeyValue` | Boolean success/error; optional confirmation passed through `core.WithConfirm`. These are the two transport callers. |
| Authorization/protection | `fresh_gate_test.go`, `TestDeleteKeyValueFailsClosedOnFreshRevocation`; `protection_test.go`, `TestProtectedKeyValueDeleteAndSuspend`; `ownerid_test.go` foreign-workspace delete assertion | Fresh authorization, tenant isolation and protected-resource confirmation remain at the shared service boundary. |
| Accepted-deletion reads | `deletion_read_test.go`, `TestDeletingKeyValueIsAbsentFromEveryByIDRead` and `TestDeletingKeyValueNotFoundAcrossSurfaces` | List omits terminating stores; detail/connection/capability reads return not found. REST, GraphQL and MCP adapter tests preserve their error shapes. |
| MCP | `mcp.go`, `RegisterMCP` | Read tools call shared read services; there is no Key Value delete tool. No new tool is introduced. |
| Dashboard row/detail | `dashboard/src/features/keyvalue/hooks/use-delete-key-value.ts`, `useDeleteKeyValue`; consumers `key-value-row-actions.tsx` and `key-value-danger-actions.tsx` | Both retain typed confirmation, permission gating, protected-confirmation handling and success callback behavior. Existing component tests mock the hook; they verify UI behavior, not operator completion. No dedicated delete-hook test exists. |
| Workspace/direct CR deletion | `purger.go`, `WorkspacePurger.PurgeWorkspace`; `purger_test.go`, `TestWorkspacePurger_DeletesOnlyTheGivenTenantsKeyValues` | Workspace purge issues CR deletion; it and direct Kubernetes deletion converge through operator finalizers. API acceptance does not bypass cleanup. |
| Other resource families | TLS cleanup is scoped to the `KeyValue` reconciler and exact `<CR-name>-kv-tls` artifacts | Web, static, cron, worker and private services remain on App lifecycle/TLS paths; PostgreSQL remains on its database/CNPG path. Their runtime deletion was not exercised by this focused audit. |

The official [Render Delete Key Value reference](https://api-docs.render.com/reference/delete-key-value)
was checked during this milestone: it documents `DELETE /v1/key-value/{redisId}`
and a 204 success response, plus 400/401/404/429/500/503 errors. Bex's existing
REST acceptance shape is retained. That public reference does not document
internal TLS Secret retention or Kubernetes cleanup sequencing; the operator
ownership/finalizer guarantees are Bex implementation evidence, not a Render
claim.

## Audit regression evidence

`scripts/delete-audit.test.sh` runs the real audit from a temporary repository
with no `.env`, a fresh credential-free subprocess environment and a mock
`kubectl`. Recorded requests verify namespace scoping, GET-only access and
name-only Secret/Certificate reads. It covers Secret-only residue,
Certificate-only residue, clean inventory, each TLS read failing, and unrelated
namespace/name decoys. All six cases pass. Optional S3 inspection is explicitly
reported as skipped in every fixture, not as verified cleanup.

Running that harness against the saved pre-change audit reproduced exit 0 with
`11 passed, 0 failed` despite Secret-only and Certificate-only residue. The new
audit returns failure for each residue and each access error, and succeeds for
clean and unrelated-decoy inventories. `bash -n` passes for both scripts.
Authentication/bootstrap code is unchanged. This is local regression evidence;
no real cluster, cert-manager issuance or S3 purge was tested by the harness.

`go test ./internal/keyvalue -count=1` passes (1.096s), including the existing
accepted-deletion, authorization, protected-delete and workspace-purge tests.
The package's environment-gated live tests do not establish hosted behavior in
this run.

The existing dashboard row/detail delete component suites pass: 2 files,
12 tests (2.13s). They cover typed confirmation, permission restrictions and
protected-action handling without changing UI source or geometry.
