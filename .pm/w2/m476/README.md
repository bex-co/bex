# w2 · m476 — Each workspace has its own OIDC issuer, as on Render: signing key, discovery and JWKS

**Worker:** worker2 **Goal:** bex publishes a per-workspace OpenID Connect issuer at `https://oidc.bex.co/{tea-…}` whose discovery document and JWKS match Render's `oidc.render.com` field for field, backed by an ES256 signer with key rotation, so AWS, GCP, Anthropic and OpenAI can trust bex as an identity provider. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design record: Managed OIDC issuer, claims, signing key and rotation; ADR018/ADR030 rows | 45m | — |
| t002 | Signer package: load the key set, mint ES256 tokens with Render's claims | 60m | t001 |
| t003 | Key bootstrap and rotation with an overlap window | 45m | t002 |
| t004 | Public discovery and JWKS endpoints with Render's exact field set | 45m | t002 |
| t005 | Issuer host `oidc.bex.co`: DNS, TLS and routing to bex-api | 45m | t004 |
| t006 | Production check: independent verification through discovery, and Render's shape | 30m | t003, t005 |
| t007 | Render parity | 20m | t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 30m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

- On production, `https://oidc.bex.co/<tea>/.well-known/openid-configuration` answers exactly Render's five fields (`issuer` = `https://oidc.bex.co/<tea>`, `jwks_uri` under the same path, `response_types_supported: ["id_token"]`, `subject_types_supported: ["public"]`, `id_token_signing_alg_values_supported: ["ES256"]`), `jwks.json` publishes the ES256 P-256 key set with RFC 7638 `kid`s, and a malformed workspace id answers 404.
- A token the signer mints for a QA workspace (`sub` `workspace:<tea>:environment:default:service:<srv>`, `exp` = `iat` + 3600) verifies with an independent OIDC library given only the issuer URL; the same token fails a verifier configured for another workspace's issuer (`scripts/oidc-issuer-verify.sh` exits 0).
- One key rotation was exercised on dev-2 with no verification gap.
- The ADR is merged; ADR018's Managed OIDC row (now listing GCP) and ADR030's Open-gap row name w2/m476–m480 as owners.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w2` (2026-11-17); the user filed all five proposed milestones with `/pm them all to w2` the same day. That reopens the 2026-08-23 deferral recorded in [`.pm/FUTURE-MAYBE.md`](../../FUTURE-MAYBE.md) (from `w2/025`) under the Render-parity-first rule (AGENTS.md, user decisions 2026-10-17/2026-10-20; precedent: `w2/m252` absorbed a FUTURE-MAYBE item under the same rule). Ledger: [ADR018](../../../docs/ADR018-render-parity.md) row "Managed OIDC for AI providers" (✖ on REST/GraphQL/MCP/UI, the last all-✖ row that is not an anti-goal) and [ADR030](../../../docs/ADR030-pricing.md)'s pricing-parity table ("Open gap | OIDC authentication").
- **Goal linkage:** Render parity first ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first)); AI-native services that reach model providers without stored keys (ADR008).
- **Expected outcome:** bex is an OIDC identity provider that AWS, GCP, Anthropic and OpenAI accept, one issuer per workspace, the foundation every later Managed OIDC milestone uses.
- **Why now:** w2/m477–m480 all mint and verify through this issuer. w2 had no actionable work (only blocked `058`, `070`, `m250`, `m322`). Render has shipped AWS since 2026-07-15 and Anthropic/OpenAI since 2026-07-24.
- **Render parity:** Included: the discovery document and JWKS are a new public, tenant-facing surface, compared field by field with `oidc.render.com`. Render exposes no REST/GraphQL/MCP/UI surface for OIDC, so the check also confirms bex adds none.
- **Depends on:** nothing.
- **Brainstorm recommendations, not user decisions** (record the chosen answer and its basis in the ADR): D2 issuer host `oidc.<root domain>` (`oidc.bex.co` in production, configurable for self-hosters); D3 deliver the token through a per-App Secret that bex-api refreshes and the pod mounts read-only (kubelet keeps Secret volumes in tmpfs; residual: a ≤60-minute token sits in etcd), with memory-only CSI delivery filed to FUTURE-MAYBE as the stricter variant; D4 below Pro the trigger variable is accepted and kept, no token is mounted, and the deploy log says why (Render's Hobby behavior is **unverified**); D5 one shared ES256 P-256 key set in a `bex-system` Secret, not OpenBao transit, so token refresh does not depend on OpenBao's manual unseal.
