# ADR085: Security review round 23 — 5PePz5 disposition

- **Status**: Accepted (2026-08-29)
- **Scan**: codex-security `5PePz5`, repository revision `3cbb7b8e`, 1 low finding
- **Lineage**: twenty-third pass in the ADR028 → … → ADR084 lineage

## Summary

The report's single finding is confirmed and fixed. Re-creating a deleted managed Postgres login role could silently adopt a surviving deterministic Secret: the API returned a fresh one-time password while CNPG continued using the old password. `DeleteUser` also patched the role out of the Database before deleting its Secret, so a transient Secret deletion failure could not be retried through the API.

The impact remains low as reported: exploitation required a stranded Secret, deliberate same-name recreation, and knowledge of the earlier password; it affected only the same role on the same tenant Database. The trace nevertheless breaks the credential-issuance invariant that the revealed password must be the installed password and revocation must not resurrect an earlier credential.

## Decision

- Each `CreateUser` issuance creates a generation-specific Secret, and the Database patch carries a resource-version precondition. Exactly one concurrent creator can return a password; losing or abandoned attempts cannot overwrite or be adopted by the successful patch.
- Same-name recreation removes the legacy `<db>-user-<role>` Secret when that historical name is Kubernetes-valid, clears the matching deletion tombstone, and returns the password only after the Database references that issuance's Secret.
- A failed Database patch cleans up its unreferenced generated Secret. Cleanup failure is joined to the original error rather than hidden.
- `DeleteUser` deletes the referenced Secret before patching `spec.users` and `spec.deletedUsers`. A Secret deletion failure leaves the CR unchanged; a subsequent patch failure remains retryable because an already-missing Secret is accepted.
- The operator ignores an `ensure: absent` tombstone when the same role is active, repairing contradictory CRs created by older API behavior without a migration.

No REST, GraphQL, MCP, or CR wire shape changes. Existing `DatabaseUser.SecretName` references remain valid.

## Verification

Regression tests cover legacy-secret replacement, exact one-time response/Secret equality, tombstone clearing, failed-create cleanup, retry after Secret deletion failure, retry after Database patch failure, and operator precedence for overlapping active/tombstoned roles. The backend and operator suites, Go lint, generated manifests, Markdown formatting, and repository diff checks are the release gates.
