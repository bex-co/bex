# w6 · m31 — `registryCredentialId` on Render-shaped service create/patch

**Worker:** worker6 **Goal:** the official-CLI-shaped REST create/patch path accepts and binds `registryCredentialId`, making the shipped registry-credentials feature (w2/m14) reachable from every surface instead of rejecting it as an unsupported field. **Status:** todo

## Tasks (in order)

| id   | title                                                       | est | depends_on |
| ---- | ----------------------------------------------------------- | --- | ---------- |
| t001 | Verify the current rejection + capture Render's contract    | 30m | —          |
| t002 | REST create: accept + bind `registryCredentialId`           | 30m | t001       |
| t003 | REST patch + GraphQL/MCP symmetry                           | 30m | t002       |
| t004 | Official CLI verify leg + ledger update                     | 30m | t003       |
| t005 | Render parity                                               | 30m | t004       |
| t006 | Simplify                                                    | 30m | t005       |
| t007 | Test coverage                                               | 45m | t005       |
| t008 | Closeout                                                    | 15m | t007       |

## Definition of done

`POST /v1/services` and `PATCH /v1/services/{id}` with `registryCredentialId` bind the credential (the resulting App pulls its private image with it, membership-checked); the `unsupportedField` rejection for it at `lego/backend/internal/apps/rest.go` is gone; an official-CLI create with the field succeeds live; the ADR018 registry-credentials row reflects full-surface coverage.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 12, 2026-07-15 — code miner (`rest.go:274,598,855`); w2/m14 shipped the feature, this residual had no owner.
- **Goal linkage:** Render REST parity; every shipped feature reachable from every surface (ADR006 one-core principle).
- **Expected outcome:** a private-registry service is creatable through the Render-compatible REST path and the official CLI.
- **Why now:** w2/m14 is closed and won't return to this code; w6 capacity placement per the m19–m28 precedent. Render parity closing task included — REST surface change.
