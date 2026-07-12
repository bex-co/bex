# ADR: SSO — social login (consumer OIDC) now, enterprise SSO deferred

**Status:** accepted. **Social login is shipped** — "Sign in with GitHub" via Ory Kratos's built-in `oidc` method (w4/003, 2026-07-11; [auth.md §10](auth.md)), extensible to more consumer OIDC providers by config alone. **Enterprise/org SSO** (per-workspace SAML/OIDC, SCIM) is a **deliberate non-goal for now** ([.pm/DO_NOT_DO.md](../.pm/DO_NOT_DO.md), [render-parity.md](render-parity.md) SSO row). This ADR keeps the settled design record so a future re-open starts from a shape, not from scratch — the same pattern [sandboxes.md](sandboxes.md) uses for a deferred surface. Do not read the enterprise-SSO section as a roadmap item; it is the boundary, not a plan.

## Context

"SSO" conflates two products with different buyers and different mechanisms:

1. **Social login / identity federation** — an _individual_ signs in with an account they already have (GitHub, Google). Convenience; no compliance obligation. One shared provider per installation.
2. **Enterprise / org SSO** — a _workspace_ forces its members to authenticate through the org's own IdP (Okta, Microsoft Entra, Google Workspace), usually over **SAML** or per-org OIDC, bundled with **SCIM** provisioning/deprovisioning, email-domain capture, JIT membership, and an enforced-SSO policy. A paid compliance feature; one IdP connection _per customer org_.

bex already runs **Ory Kratos** as its identity plane (identity is bought, not built — [auth.md §1](auth.md)). The question this ADR answers is _which SSO shapes ride that substrate now, and which are deferred_.

## Decision

### 1. Social login rides Kratos's built-in `oidc` method (shipped)

bex's users live on GitHub, so "Sign in with GitHub" is the expected first provider, and it costs nothing beyond config on the substrate already deployed. The full design is [auth.md §10](auth.md); the load-bearing choices:

- **Kratos's native `oidc` method — no custom auth code, no custom login provider** (`.pm/DO_NOT_DO.md`). Identity federation is a first-class Kratos feature, distinct from the Hydra login-challenge bridge ([auth.md §7](auth.md)), which federates bex-api's own _tokens_, not third-party _identities_.
- **Each installation owns its own OAuth app** (mirrors the self-hosted GitHub _App_ decision, [github-integration.md](github-integration.md)); a shared bex-owned app can't serve every self-hoster's callback domain. Callback `https://auth.<base-domain>/self-service/methods/oidc/callback/<id>`.
- **The provider secret rides the out-of-band `kratos` Secret, never git** — [scripts/auth-secrets.sh](../scripts/auth-secrets.sh) writes an `oidc.yaml` fragment that Kratos loads as a second `--config`; the claims→traits mapper is inlined via `base64://`. **On/off is purely the presence of the `BEX_GITHUB_OIDC_*` secrets** (set ⇒ enabled provider, unset ⇒ `enabled: false`), applied on the next Kratos rollout — no git change.
- **The dashboard renders the button with zero bespoke code** — Ory Elements' `<Login>`/`<Registration>` track whatever methods Kratos enables (the same property MFA relies on).
- **Adding providers is the same shape** — another entry in the `providers` list with its own id/secret (Google, GitLab, …); the button appears automatically. This is the whole of "SSO" bex intends to ship for the foreseeable future.

### 2. Enterprise / org SSO is deferred — a recorded non-goal, not a roadmap item

Consistent with [.pm/DO_NOT_DO.md](../.pm/DO_NOT_DO.md) ("Enterprise/managed-infra surfaces: SSO/SAML · SCIM … non-goal for now") and the `—`/`—`/`—`/`—` [render-parity.md](render-parity.md) SSO row. Why it is deferred, not built:

- **No enterprise tenant exists yet.** Enterprise SSO is a compliance sale; building per-org SAML + SCIM before a buyer needs it is speculative surface. (This is the opposite of MFA, which shipped early precisely because forcing enrollment onto _existing_ users later is worse — [auth.md §9](auth.md); enterprise SSO has no such pre-tenant trap.)
- **Open-source Kratos does not fit the enterprise shape.** Federation is single-tenant — one global `providers` list, not per-workspace IdP connections — and **Kratos OSS has no SAML** (SAML is an Ory Network / enterprise capability). Enterprise buyers routinely require SAML.
- **Enterprise SSO is a bundle, not a feature.** Per-org IdP connection management, domain verification/capture, SCIM user provisioning/deprovisioning, enforced-SSO policy, and JIT membership are each their own surface. Not a `providers[]` append.

**If it is ever re-opened**, the candidate shapes so a future start isn't from scratch:

| Shape | Fit | Cost |
| --- | --- | --- |
| **(a)** Kratos generic OIDC per _shared_ IdP | Fine for a fixed handful (Google Workspace as one more button) | None beyond §1 — but **cannot** do per-customer-org connections |
| **(b)** An SSO **broker in front of Kratos** (Dex or Keycloak) federating many upstream IdPs (SAML/OIDC/LDAP) and presenting **one** OIDC provider to Kratos | The natural path — bex already exercises **Dex** in the OIDC e2e; keeps Kratos the single identity plane | Dex connectors are static config (restart per org); dynamic per-org onboarding wants **Keycloak** (realms/identity-providers via admin API) or SCIM tooling |
| **(c)** A hosted enterprise-SSO service (Ory Network, **WorkOS**) | Buys the SAML+SCIM problem whole | SaaS dependency — acceptable only for bex.co's _hosted_ tier, never for self-hosters |

**Recommendation if built:** **(b) Keycloak as the enterprise-SSO broker**, presenting OIDC into Kratos so members/roles/MFA/audit all keep working unchanged — but only once a paying enterprise need exists. Kratos stays the identity plane in every shape; enterprise SSO federates _into_ it, never replaces it.

## Alternatives considered

- **Build SAML/OIDC federation into the bex control-plane Postgres** — owns the OIDC/SAML correctness bar forever; the entire premise of adopting Ory ([auth.md §1](auth.md)) was to _not_ own that bar. Rejected.
- **Hosted IdP for everything (Auth0 / Clerk / WorkOS as the primary IdP)** — a SaaS dependency inside a self-hostable, open-source platform, which [auth.md §1](auth.md) already rejects for the core. Reconsider only as component **(c)** for bex.co's hosted enterprise tier, never for self-hosters.
- **Replace Kratos with Keycloak as the primary IdP** — Keycloak's monolith was rejected in [auth.md §1](auth.md); Keycloak-as-_broker_ behind Kratos (shape **b**) stays on the table for enterprise SSO only.
- **Ship enterprise SSO/SAML/SCIM now** — no enterprise buyer, premature, and a `DO_NOT_DO` item. Deferred.

## Consequences

- Social login is live and extensible at near-zero marginal cost; new consumer providers are `.env` + a redeploy, gated per installation.
- bex has **no** enterprise SSO / SAML / SCIM; the [render-parity.md](render-parity.md) SSO row stays `—` on purpose. A future enterprise deal that requires SAML/SCIM is a real project (broker + connection management + SCIM), scoped only if and when demand appears.
- **Kratos remains the single identity plane.** Any future enterprise SSO federates into Kratos via the broker pattern (shape **b**), so workspace members, roles, MFA, and the audit log ([auth.md](auth.md)) all continue to work unchanged — the deferral costs nothing structurally.

## Verification

Social login (the shipped half) is exercised by [scripts/auth-oidc-e2e.sh](../scripts/auth-oidc-e2e.sh) (throwaway Dex stand-in IdP + real Kratos: login → provider → callback → first-party session) and smoke-tested against a live cluster by [scripts/auth-oidc-verify.sh](../scripts/auth-oidc-verify.sh) (Secret enabled → second `--config` mounted → rollout healthy → login flow advertises the provider). Enterprise SSO has nothing to verify — it is intentionally not built.
