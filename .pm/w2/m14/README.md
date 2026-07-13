# w2 · m14 — Registry credentials: private external image registries

**Worker:** worker2 **Goal:** A workspace can store credentials for a private external image registry (Docker Hub, GHCR, GitLab Container Registry, ECR, etc.) and deploy an existing-image service that pulls from it — closing bex's only-public-or-internal-Zot image-source limitation. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                                                                     | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Data model: `registry_credentials` control-plane table (workspace-scoped: registry host, username, opaque secret, optional `expiresAt`, created-by/at); mint the id through `lego/backend/internal/id` (new `id.Kind`, register per `docs/ADR020-identifiers.md`) | 45m | —          |
| t002 | Operator wiring: extend `imagePullSecrets` (`lego/operator/internal/controller/app_controller.go`) to also resolve a per-App `kubernetes.io/dockerconfigjson` Secret materialized from a workspace's stored credential matching the image's registry host — additive to (not replacing) the existing internal-Zot pull-secret path | 1h  | t001       |
| t003 | REST: CRUD `/v1/registry-credentials` (Render's `/registrycredentials` shape)                                                                                                                             | 40m | t001       |
| t004 | GraphQL: mirror t003                                                                                                                                                                                       | 25m | t003       |
| t005 | MCP: `list_/create_/delete_registry_credential` tools (check against Render's official MCP tool list before claiming superset status — don't over-assert)                                                | 25m | t003       |
| t006 | Dashboard: Settings → Integrations section (reuses the pattern `w3/m11`'s webhooks section establishes) to add/list/delete registry credentials, secret masked                                           | 1h  | t004, t005 |
| t007 | Expiry surfacing: REST/GraphQL/MCP/dashboard can show a credential nearing/past its `expiresAt` (Render's own docs flag ECR's 12h rotation as a real operational concern) — bex doesn't auto-rotate, but must not hide staleness | 30m | t003       |
| t008 | Live verification: create a credential for a real private registry, deploy an existing-image service referencing an image there, confirm it pulls successfully                                          | 30m | t002, t006 |

## Definition of done

A workspace stores credentials for a private external registry (host + username + secret, with optional expiry), and a service whose `image` references that host pulls successfully using a materialized `dockerconfigjson` Secret — verified against a real private registry, not just a unit test. Stale/expired credentials are surfaced, not silently hidden.

## Source + Goal linkage

- **Source:** direct mechanism check during `/pm-brainstorm more milestones to work on`, 2026-07-13, closing `docs/ADR018-render-parity.md`'s "Registry credentials" row; verified live via search that Render genuinely supports this (Docker Hub/GHCR/GitLab/Google Artifact Registry, a real "Update Registry Credential" API, and ECR's 12h rotation called out explicitly) — not a fabricated parity-ledger entry.
- **Goal linkage:** Render-parity core surface — deployment sources & IaC section of the parity ledger.
- **Expected outcome:** parity ledger's registry-credentials row flips from `✖✖✖✖` to `✅✅✅✅` (or `◐` wherever a real divergence is found and documented).
- **Why now:** genuinely unowned gap, clean scope, no DO_NOT_DO conflict, and the precedent mechanism (GitHub App credentials, `w2/m8`) already establishes the pattern this borrows (external-credential → materialized Secret → operator wiring).
