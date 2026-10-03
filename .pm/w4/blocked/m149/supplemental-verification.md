# m149 supplemental verification — 2026-10-02

The drain integrated `93ee9b66e`, which independently shipped the TLS finalizer
and parked m149, before continuing with these remaining gaps. The duplicate
local implementation was archived outside the repository; the shipped protocol
and its existing tests remain the base of this supplement.

## Preserve legacy issuance during configuration changes

Changing the operator's public Key Value domain or issuer previously replaced
the saved issuance identity before checking the existing legacy Secret. The
producer read the owned Certificate but retained only its UID, so the old
Secret's valid hostname/issuer metadata failed validation and prevented the
Certificate from converging.

`keyvalue_tls.go` now seeds identity from configuration only when no identity
has been recorded. Existing history is still decoded, so malformed history
continues to prevent issuance. `keyvalue_controller.go` records the actual
owned Certificate identity and validates its output before updating issuance.
It binds an accepted legacy Secret to the current KeyValue UID with an
optimistic metadata patch first. This preserves cleanup authority if public
access is withdrawn before cert-manager updates the Secret. Key material is
unchanged; failed provenance writes stop before producer changes.

`keyvalue_tls_migration_test.go` exercises Reconcile for ownerless output,
Certificate-owned output, and a missing producer with durable historical
identity. Each changes domain/issuer, withdraws public access before issuance
catches up, restarts cleanup, and verifies the Secret and CR disappear without
starting a Free-plan backup purge. Other cases retain the old producer when
binding fails and refuse issuance for malformed history.

The migration cases fail on `93ee9b66e` with unproven-Secret errors
(`/tmp/bex-m149-migration-before.log`). All `TestKeyValueTLS*` cases pass after
the fix (0.954s, `/tmp/bex-m149-tls-focused.log`), including the original foreign
ownership, lifetime, retry and ordered-deletion controls. These are local fake
clients; they do not run cert-manager or Kubernetes garbage collection.

## Detect the complete audit inventory and read failures

The audit retains upstream's exact Certificate and issued-Secret queries. It
also counts CertificateRequests by the exact certificate-name annotation or
Certificate owner reference, using metadata only. Its shared count helper now
requests resource names and propagates query failures rather than returning
zero. No cleanup operation is added.

`delete-audit.test.sh` runs the actual script from a temporary repository without
`.env`, credentials, cluster or object-store access. Its 23 cases cover TLS,
Certificate and request-only residue; exact-name/namespace controls; API and
malformed-metadata errors; backup residue; configured/optional S3 checks; and
App/static/Postgres consumers of the shared helper. Recorded command arguments
are checked independently of the command shim, since the audit suppresses
kubectl stderr. Secret bytes are never requested. Temporary files are removed.

The same harness demonstrates that `93ee9b66e` incorrectly returns success for
request-only residue and failed backup inventory queries
(`/tmp/bex-m149-audit-upstream-regression.log`). All 23 final cases pass
(`/tmp/bex-m149-audit-root.log`). Optional S3 checks explicitly skip when
unconfigured; configured S3 cases use a shim and do not establish real cleanup.

## Preserve deletion acceptance across surfaces

`tls_deletion_contract_test.go` creates a public Free resource with the actual
TLS-only finalizer and an issued Secret. Accepted REST deletion returns 204
with an empty body; GraphQL returns true. The CR remains physically pending and
the Secret UID remains present while list/REST/GraphQL/MCP reads hide the
deleted resource. The test applies the production `gqlutil.NilOnError` schema
wrapper and reuses `kvGQLSchema`. Fresh authorization denial and protected
environment refusal leave the CR lifetime and issued Secret unchanged.

The merged backend suite passes (`/tmp/bex-m149-backend-merged.log`); the final
test-helper refactor passes its focused rerun (0.655s,
`/tmp/bex-m149-backend-final-targeted.log`). Database/OpenFGA integration
variables were unset, so this does not establish their environment-gated
integration coverage. Existing dashboard row/detail delete suites pass:
2 files, 12 tests (`/tmp/bex-m149-dashboard-delete.log`). Their hook is mocked;
they verify permissions, confirmation and callback behavior.

The [existing consumer matrix](verification.md#shared-consumer-and-parity-evidence)
still accounts for the one producer/deletion-handler pair, two delete transport
callers, two dashboard consumers, workspace/direct-CR teardown and MCP reads.
Web/static/cron/worker/private App lifecycle and PostgreSQL/CNPG remain separate.
Shared Go teardown helpers, Secret caching and RBAC are unchanged. The changed
audit helper's App/static/Postgres consumers have the shell controls above.

The official [Render deletion reference](https://api-docs.render.com/reference/delete-key-value)
continues to support the existing REST 204 contract. The
[cert-manager Certificate guide](https://cert-manager.io/docs/usage/certificate/)
and [API reference](https://cert-manager.io/docs/reference/api-docs/) confirm that
Secret templates carry labels/annotations and issued Secrets normally outlive
their Certificate. These references do not establish hosted Bex convergence.

## Review and final gates

Three parallel simplify perspectives reviewed the complete supplement. Reuse
identified the duplicate GraphQL schema setup; the existing helper is now used.
Quality and efficiency found no further actionable changes. The guarded
identity/provenance writes avoid repeated metadata updates.

Shell syntax, ShellCheck, skill-layout validation and `git diff --check` pass.
`GOWORK=off GOFLAGS=-p=2 make test` passes, including code generation and envtest
(controller package 114.551s, `/tmp/bex-m149-operator.log`). `make lint` passes:
all four modules report zero issues and whole-program reachability completes
(`/tmp/bex-m149-lint.log`). Repository-wide Markdown formatting passes.

Those full suites/lint ran with `93ee9b66e` plus this supplement. Final
synchronization to `0874761be` brought unrelated log/static-header and store
fixture changes without overlapping edits. The Key Value package (2.946s) and
all TLS controller tests (1.470s) pass again after synchronization
(`/tmp/bex-m149-keyvalue-post-pull.log`, `/tmp/bex-m149-tls-post-pull.log`).

## Remaining hosted gate

t004/t007 remain open in `blocked/m149`. The release pipeline must deploy the
operator, then QA must repeat fresh Free public/private dashboard deletion,
capture the issued Secret UID and show the complete resource inventory becomes
empty automatically, verify fresh Overview and all API read shapes, and run the
corrected audit. Clean up owned fixtures and revoke the QA session. No hosted
fixtures or sessions were created by this supplemental implementation run.
