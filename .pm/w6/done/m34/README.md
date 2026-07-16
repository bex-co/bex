# w6 · m34 — Docker-build `registryCredentialId`: private base images for repo-backed Docker services

**Worker:** worker6 **Goal:** Render's second registry-credential context — `serviceDetails.envSpecificDetails.registryCredentialId` on a repository-backed Docker build — works end to end: the bound credential authenticates BuildKit's private `FROM` resolution without ever being readable by tenant Dockerfile steps, retiring the named 400 `w6/m31` shipped as an honest stopgap. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Backend: accept + persist `envSpecificDetails.registryCredentialId` for repo-backed Docker services (retire the 400) — **DONE** | 45m | —    |
| t002 | Operator: merge the bound credential into the build Job's mounted docker config (buildkitd-only boundary) — **DONE**             | 45m | t001 |
| t003 | Isolation tests: foreign/unknown credential rejected; credential invisible to tenant build steps — **DONE**                      | 30m | t002 |
| t004 | Live verify: authenticated private base-image pull builds green on a dev-N stack — **DONE**                                      | 30m | t003 |
| t005 | Render parity — **DONE**                                                                                                          | 30m | t004 |
| t006 | Simplify — **DONE**                                                                                                               | 20m | t005 |
| t007 | Test coverage — **DONE**                                                                                                          | 30m | t005 |
| t008 | Closeout — **DONE**                                                                                                               | 15m | t007 |

## Definition of done

A repository-backed Docker service with `envSpecificDetails.registryCredentialId` set builds successfully from a private base image on a live dev-N stack; the credential is provably unreadable from tenant Dockerfile `RUN` steps (never a build arg, BuildKit secret, or tenant-visible file — test-asserted); a foreign-workspace or unknown credential id is rejected with a Render-shaped error; the named 400 in `lego/backend/internal/apps/pullsecret.go` is gone and all three API surfaces (REST/GraphQL/MCP) accept and echo the field.

## Source + Goal linkage

- **Source:** `w6/017` (filed by `w6/m31`'s closeout Render-contract parity check, 2026-07-15; the note itself says "should be promoted before implementation"); promoted by `/pm-brainstorm` round 17.
- **Goal linkage:** Render API compatibility (`docs/ADR006-bex-api.md`, `docs/ADR018-render-parity.md`) — closes the second half of the registry-credential contract row; tenant-isolation boundary discipline per `docs/ADR022-tenant-isolation.md` § Registry access control.
- **Expected outcome:** private-image deploys work for both of Render's source contexts (prebuilt image *and* private Docker base image), not just the prebuilt half `w6/m31` shipped.
- **Why now:** `w6/m31` shipped its sibling yesterday — the mechanism (`BEX_REGISTRY_PUSH_SECRET`'s buildkitd-container-fs mount that tenant `RUN` steps can't read), tests, and reviewer context are hot; leaving the 400 in place makes the just-shipped feature look half-done to any Render-CLI user.
- **Render parity:** included — REST/GraphQL/MCP accept surfaces change, and the dashboard create-from-repo Docker flow should offer the same credential selector `services.new.tsx` grew for images (t005 checks all four).

## Closeout evidence

- REST accepts and echoes Render's nested Docker field; GraphQL and MCP use their existing shared `registryCredentialId` argument/verb. Unknown ids return 404, foreign-workspace ids 403, and non-Docker repository sources a named 400.
- The operator materializes one deterministic build-namespace Secret containing merged pull/push auth; push wins same-host collisions. Only buildkitd/cosign mount it, and tests exclude build args, BuildKit secrets, tenant-step env, and other mounts.
- A live CAPD app-cluster run built a private `FROM`, pushed the result, and reached `Running` with the valid credential. An intentionally wrong bound credential failed the same manifest request with `401 Unauthorized`. All disposable resources were removed.
- Dashboard create-from-Git shows the credential picker only for Docker runtime. Full backend, operator, and dashboard validation is recorded in t007.
