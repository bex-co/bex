# w6 · m31 — `registryCredentialId` on Render-shaped service create/patch

**Worker:** worker6 **Goal:** the official-CLI-shaped REST create/patch path accepts and binds `registryCredentialId`, making the shipped registry-credentials feature (w2/m14) reachable from every surface instead of rejecting it as an unsupported field. **Status:** done — 2026-07-15; durable explicit binding ships across REST/GraphQL/MCP/dashboard; the unmodified official CLI live leg passed and cleaned up; Docker source-build auth is honestly rejected and filed as `w6/017`.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Verify the current rejection + capture Render's contract | 30m | — — **DONE** |
| t002 | REST create: accept + bind `registryCredentialId` | 30m | t001 — **DONE** |
| t003 | REST patch + GraphQL/MCP symmetry | 30m | t002 — **DONE** |
| t004 | Official CLI verify leg + ledger update | 30m | t003 — **DONE** |
| t005 | Render parity | 30m | t004 — **DONE** |
| t006 | Simplify | 30m | t005 — **DONE** |
| t007 | Test coverage | 45m | t005 — **DONE** |
| t008 | Closeout | 15m | t007 — **DONE** |

## Definition of done

`POST /v1/services` and `PATCH /v1/services/{id}` with `registryCredentialId` bind the credential (the resulting App pulls its private image with it, membership-checked); the `unsupportedField` rejection for it at `lego/backend/internal/apps/rest.go` is gone; an official-CLI create with the field succeeds live; the ADR018 registry-credentials row reflects full-surface coverage.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 12, 2026-07-15 — code miner (`rest.go:274,598,855`); w2/m14 shipped the feature, this residual had no owner.
- **Goal linkage:** Render REST parity; every shipped feature reachable from every surface (ADR006 one-core principle).
- **Expected outcome:** a private-registry service is creatable through the Render-compatible REST path and the official CLI.
- **Why now:** w2/m14 is closed and won't return to this code; w6 capacity placement per the m19–m28 precedent. Render parity closing task included — REST surface change.

## Completion evidence

- Render contract pinned in `docs/render-artifacts/registry-credential-service-binding.md`; official CLI source checked at `72b3fbd59068ae84d024ec2ded9df6b27dc8dd68`.
- REST create/PATCH/read, GraphQL create/setter, MCP create/setter, and the dashboard Existing Image picker all call one Core binding path. Unknown/foreign/host-mismatch classify 404/403/400; empty explicitly clears, including the dashboard's “None” option; REST reads return Render's `{id,name}` summary.
- Intent persists in Postgres migration `0036` and `App.spec.registryCredentialId`; the projector restores and preserves the deterministic docker-config Secret reference across CR recreation and repeated resyncs.
- `scripts/cli-compat.sh registry-credential-verify` passed live in dev-6 against an auth-enabled `registry:2`: anonymous manifest, node, and control-Pod pulls were refused; the unmodified CLI resolved `/v1/registrycredentials/{id}`, created `srv-d9c38chjg4r0f1h0jq2g`, the selected credential produced a kubelet `Pulled` event, the App reached `Running`, and the trap left zero resources.
- Verification green: backend `go test ./...` + lint; operator `make test`; dashboard 1,160 tests + `yarn lint` (including typecheck); shell syntax; focused canonical-route regression.
- Simplification review kept the single resolver/materializer, combined source+credential PATCH, deterministic Secret name, and shared canonical-route forwarding. No behavior-preserving deletion was found; the only uncovered scope was materially different Docker build-time auth and is filed as `w6/017`.
