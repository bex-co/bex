# w7 · m71 — Shared build-namespace Secret ownership enforcement

**Worker:** worker7 **Goal:** stop one tenant from destroying or overwriting another tenant's (or a platform) Secret in the shared `bex-build` namespace by App-controlled name collision. **Status:** done

## Tasks (in order)

| id   | title                                                                  | est  | depends_on |
| ---- | ---------------------------------------------------------------------- | ---- | ---------- |
| t001 | Ownership-check the clone/registry/predeploy Secret mirror (overwrite) — **DONE** | 40m  | —          |
| t002 | Ownership-check the finalizer Secret-delete loop — **DONE**            | 30m  | —          |
| t003 | Simplify — **DONE**                                                    | 20m  | t001, t002 |
| t004 | Test coverage — **DONE**                                               | 40m  | t003       |
| t005 | Closeout — **DONE**                                                    | 10m  | t004       |

## Definition of done

A tenant App whose `cloneSecret`/`externalRegistryPullSecret`/`envFromSecrets`/`filesFromSecrets` name collides with another tenant's copied credential or a platform build Secret cannot overwrite it (CreateOrUpdate refuses on ownership mismatch) and cannot delete it (finalizer skips non-owned targets). Only Secrets whose immutable ownership labels match the reconciling App are mutated, verified by cross-tenant overwrite-negative and delete-ownership-negative tests.

## Source + Goal linkage

- **Source:** codex-security scan findings #5 (medium, delete) and #14 (medium, overwrite), validated against HEAD. Same root cause: the operator mirrors/deletes build-namespace Secrets by App-controlled name with no ownership check, unlike `ReclaimAppArtifacts` which filters by App UID.
- **Goal linkage:** Security pillar — tenant isolation of build credentials (ADR022 § build namespace).
- **Expected outcome:** cross-tenant/platform Secret destruction and overwrite by name collision is impossible.
- **Why now:** medium-severity cross-tenant data-integrity bug; the two paths share a root cause and a single ownership-label convention, so they ship together.
- **Render parity omitted:** operator-internal mechanism; no REST/GraphQL/MCP/UI surface change.

## Ship record — DONE 2026-08-01

Shipped as `b00b40a4` (deployed → GitOps pin `49c67eeb`). `copyCloneSecret`'s `CreateOrUpdate` mutate closure now calls `execution.ArtifactIdentity.CheckOwner(dst)` when the destination already exists, refusing to overwrite a foreign-owned/platform Secret (codex-security #14) — the bytes and labels are left byte-for-byte intact and the collision surfaces as an error. The finalizer loop in `reclaimAppExecution` now calls a new `deleteOwnedSecret` helper instead of `deleteAndWait`-by-name: it deletes a build-namespace Secret only when `identity.Owns(sec)` (the `app.bex.co/app-uid` label matches), leaving foreign/platform same-named Secrets in place (codex-security #5). Both reuse the existing `execution.ArtifactIdentity` ownership model (`labelApp` + `labelAppUID`) that `ReclaimAppArtifacts` already enforces. CI: full controller suite green (incl. envtest) + `TestCopyCloneSecretRefusesForeignDestination` + `TestDeleteOwnedSecretLeavesForeignSecretIntact`; the existing `TestCopyCloneSecretAcrossNamespaces` still passes (same-App refresh unaffected).
