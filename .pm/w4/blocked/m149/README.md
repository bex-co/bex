# w4 · m149 — Remove Key Value TLS artifacts on deletion

**Worker:** worker4 **Goal:** deleting a Key Value removes its issued TLS Secret as well as its workload and data volume. **Status:** blocked — t001/t002/t003/t005/t006 done; t004/t007 await operator release and live public/private acceptance

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Finalize per-instance Key Value TLS independently of backups — **DONE** | 60m | — |
| t002 | Verify lifecycle consumers and preserve other cleanup paths — **DONE** | 25m | t001 |
| t003 | Make the Key Value delete audit detect TLS residue — **DONE** | 15m | t001 |
| t004 | Render parity: preserve delete and not-found behavior | 20m | t002, t003 |
| t005 | Simplify the cleanup change — **DONE** | 10m | t004 |
| t006 | Prove ordered cleanup, ownership and retry behavior — **DONE** | 35m | t005 |
| t007 | Close out with live deletion and zero-residue evidence | 15m | t006 |

Total: 7 tasks, approximately 3 hours.

## Definition of done

These are the actual sweep 9–10 probes, repeated against the deployed fix with new owned fixtures:

- Create a Free public Key Value from the dashboard, wait for Available and its Certificate Ready, then delete it from a freshly opened detail page. The matching KeyValue/Certificate/CertificateRequest/workload/PVC/auth/connection/TLS Secret inventory eventually becomes empty, without manual Secret deletion. The TLS Secret's previously recorded UID must be absent, not merely hidden from the product.
- Independently create and delete a Free private Key Value through the same dialog. Its recorded inventory becomes empty and the new TLS cleanup obligation does not strand it.
- After each delete, fresh Overview omits the fixture; REST detail is 404, GraphQL detail is null with the current not-found error, and MCP get_key_value returns its current not-found tool error. Preserve those existing shapes.
- The Key Value section of scripts/delete-audit.sh includes the exact Certificate and issued TLS Secret. With only the issued TLS Secret left, the corrected audit is nonzero; once the TLS pair is absent, it passes. Optional S3 checks remain explicitly skipped when their prerequisites are absent.

Additional paid/backup-disabled, historical-public, interrupted issuance, restart, ownership/refusal and alternate-entrypoint cases were not exercised in this sweep. They are required verification tasks below, not claims about observed live success.

## Source + Goal linkage

- **Source:** user-requested continuous qa-find-bugs using muse.env and filing in w4; 2026-10-02 sweeps 9–10. [Finding, full requests/responses, inventory and audit output](finding.md). Screenshot: .playwright-mcp/qa-r10-public-kv-delete.png (local supplement only).
- **Goal linkage:** ADR008's reliable hosting lifecycle, ADR021's managed Key Value implementation, and the zero-leftover goal established by w7/m12.
- **Expected outcome:** per-instance TLS material is reclaimed on deletion, independently of whether the instance has paid backups; the existing auditor detects this artifact.
- **Why now:** two independent Free public fixtures retained ownerless TLS Secrets, while a private control cleaned up; today's audit falsely passed 11 checks. This is a residual inventory gap in w7/m12, not a new TLS-retention product policy.
- **Render parity included:** deleting a hosted datastore is tenant-facing across REST/GraphQL and the dashboard, with MCP read controls. Preserve the existing transport contract; no MCP delete tool is introduced.
- **Scope:** operator-owned TLS lifecycle, precise artifact audit and tests. No global cert-manager flag change, bulk production Secret purge, paid purchase, new API field, or unrelated App/Postgres rewrite. Sweep fixtures and the two exact orphan Secrets were cleaned up; both sessions were revoked.

## Parked gate — 2026-10-02

The release pipeline must deploy the operator. QA must then repeat the Free
public/private dashboard deletions, prove the recorded TLS Secret UID and full
resource inventories disappear automatically, verify fresh Overview and all
three API read shapes, and run the corrected audit. Optional S3 prerequisites
remain explicit skips. Clean up the owned fixtures and revoke the session.
[Verification](verification.md) separates local/fake-controller coverage from
real cert-manager, garbage collection and hosted acceptance.

[Supplemental verification](supplemental-verification.md) records legacy
domain/issuer migration, pending-cleanup deletion contracts and the complete
audit inventory/read-error controls. t004/t007 still require hosted acceptance.
