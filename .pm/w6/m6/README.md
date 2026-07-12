# w6 · m6 — Security review: RBAC, supply chain, injection surface, network isolation

**Worker:** worker6 **Goal:** Close `GOAL.md` #7 ("Security review") — the last unowned V0 roadmap line — with a documented, evidence-backed audit: RBAC least-privilege, container supply-chain signing, injection surface across REST/GraphQL/MCP, live network-isolation verification, secrets hygiene, and OAuth/OIDC correctness. **Status:** todo

**Placement note:** filed under `w6` (not a topical fit — `w6` is otherwise workspace-lifecycle parity) because `w6` had zero open milestones at brainstorm time, per user instruction to place new security work in the least-loaded worker (2026-07-11).

**Not duplicating:** `w1/m23/t002` already owns Dependabot triage (36 findings, 2 critical/15 high) — excluded here.

## Tasks (in order)

| id   | title                                                                                                                                                                    | est | depends_on                          |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --- | ------------------------------------ |
| t001 | RBAC least-privilege audit: every ClusterRole/Role bound to the operator, bex-api, and platform components (Kratos/Hydra/OpenFGA/CNPG/OpenBao SAs) — confirm no wildcard verbs/resources, no unused grants | 30m | —                                     |
| t002 | Container supply chain: add image signing (cosign) + SBOM generation to the build/push CI step — no signing/SBOM exists anywhere in the repo today, for either the `lego/` image or CNB-built tenant images | 45m | —                                     |
| t003 | Injection/input-validation audit across REST/GraphQL/MCP: control-plane Postgres query construction (SQLi), webhook HMAC verification correctness (`BEX_WEBHOOK_SECRET` path), build-pipeline argument handling (command injection via repo/branch/root-directory fields) | 45m | —                                     |
| t004 | Network policy conformance test: live-verify the `docs/ADR022-tenant-isolation.md` reachability matrix actually holds on a running cluster (not just declared) — tenant↔tenant, tenant↔platform, tenant↔control-plane denies | 45m | —                                     |
| t005 | Secrets-hygiene audit: grep bex-api/operator logging + error paths for leaked credentials (OpenBao tokens, DB URIs, API keys, webhook secret) across all log statements and error responses | 30m | —                                     |
| t006 | OAuth/OIDC audit: verify PKCE enforcement, token-audience checks (`BEX_OAUTH_RESOURCE`), and scope enforcement match `docs/ADR012-auth.md` §7 as actually implemented, not just documented | 30m | —                                     |
| t007 | Write findings + remediations to `docs/security-review.md`; file any newly discovered issues as follow-up inbox notes rather than silently fixing everything in-milestone | 30m | t001,t002,t003,t004,t005,t006        |
| t008 | Simplify — `/simplify` over the code this milestone changed                                                                                                              | 20m | t007                                  |
| t009 | Test coverage — add meaningful tests for the behavior this milestone shipped (e.g. the network-policy conformance test from t004 as a repeatable check)                 | 30m | t007                                  |
| t010 | Closeout — verify DoD holds, then move the milestone to `done/`                                                                                                         | 10m | t008,t009                             |

## Definition of done

- `docs/security-review.md` exists with a finding-by-finding writeup (severity, evidence, remediation status).
- CI produces a signed image + SBOM on every build.
- The tenant-isolation reachability matrix has a passing, repeatable live test, not just a declared table.
- No plaintext secrets found in audited log/error paths.
- RBAC audit confirms (or fixes) least-privilege across every platform ServiceAccount.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-07-11, direct repo audit (grep for `PodDisruptionBudget`, `cosign`/SBOM, RBAC role files — zero signing/SBOM hits; RBAC already looked reasonably scoped but unverified in full).
- **Goal linkage:** `GOAL.md` #7 ("Security review") — the last unclaimed V0 roadmap item.
- **Render parity: omitted.** Internal security audit; no REST/GraphQL/MCP/UI surface change (t003 touches those surfaces as an audit, not a feature).
- **Expected outcome:** a documented, evidence-backed security posture with supply-chain signing live in CI and network isolation claims actually verified, not just declared.
- **Why now:** #7 is the last unowned V0 goal; RBAC/image-signing/injection surface have never been audited, and `docs/ADR022-tenant-isolation.md`'s reachability matrix has no live test backing its threat-model claims.
