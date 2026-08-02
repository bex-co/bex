# w7 · m71 — Shared build-namespace Secret ownership enforcement

**Worker:** worker7 **Goal:** stop one tenant from destroying or overwriting another tenant's (or a platform) Secret in the shared `bex-build` namespace by App-controlled name collision. **Status:** todo

## Tasks (in order)

| id   | title                                                                 | est  | depends_on |
| ---- | --------------------------------------------------------------------- | ---- | ---------- |
| t001 | Ownership-check the clone/registry/predeploy Secret mirror (overwrite) | 40m  | —          |
| t002 | Ownership-check the finalizer Secret-delete loop                      | 30m  | —          |
| t003 | Simplify                                                              | 20m  | t001, t002 |
| t004 | Test coverage                                                        | 40m  | t003       |
| t005 | Closeout                                                             | 10m  | t004       |

## Definition of done

A tenant App whose `cloneSecret`/`externalRegistryPullSecret`/`envFromSecrets`/`filesFromSecrets` name collides with another tenant's copied credential or a platform build Secret cannot overwrite it (CreateOrUpdate refuses on ownership mismatch) and cannot delete it (finalizer skips non-owned targets). Only Secrets whose immutable ownership labels match the reconciling App are mutated, verified by cross-tenant overwrite-negative and delete-ownership-negative tests.

## Source + Goal linkage

- **Source:** codex-security scan findings #5 (medium, delete) and #14 (medium, overwrite), validated against HEAD. Same root cause: the operator mirrors/deletes build-namespace Secrets by App-controlled name with no ownership check, unlike `ReclaimAppArtifacts` which filters by App UID.
- **Goal linkage:** Security pillar — tenant isolation of build credentials (ADR022 § build namespace).
- **Expected outcome:** cross-tenant/platform Secret destruction and overwrite by name collision is impossible.
- **Why now:** medium-severity cross-tenant data-integrity bug; the two paths share a root cause and a single ownership-label convention, so they ship together.
- **Render parity omitted:** operator-internal mechanism; no REST/GraphQL/MCP/UI surface change.
