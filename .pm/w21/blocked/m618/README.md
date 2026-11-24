# w21 · m618 — Certificates also issue from Google Trust Services, as on Render: a custom domain whose CAA names only `pki.goog`, or a Let's Encrypt failure, no longer leaves it without a certificate

**Worker:** worker21 **Goal:** bex orders a custom domain's certificate from Let's Encrypt and, when that cannot succeed, from Google Trust Services, as Render uses both. **Status:** blocked (t001, t003 and t004 done; t002 needs a Google Trust Services External Account Binding key from the user, then t005 to t009)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | ADR005: when a host's certificate is ordered from Google Trust Services — **DONE** | 45m | — |
| [t002](t002.md) | ClusterIssuer for Google Trust Services from an EAB credential the user provides | 45m | t001 |
| [t003](done/t003.md) | The operator moves a failing host to Google Trust Services; bex-api names a CAA refusal in plain words — **DONE** | 1h30 | t001 |
| [t004](done/t004.md) | The dashboard's DNS instructions carry the CAA hint the CLI skill already gives — **DONE** | 30m | t001 |
| [t005](t005.md) | Production: a disposable domain whose CAA allows only `pki.goog` gets a certificate, and an ordinary one still gets Let's Encrypt's | 45m | t002, t003 |
| [t006](t006.md) | Render parity | 30m | t005 |
| [t007](t007.md) | Simplify | 20m | t006 |
| [t008](t008.md) | Test coverage | 40m | t006 |
| [t009](t009.md) | Closeout | 10m | t007, t008 |

## Definition of done

1. On production, a disposable custom domain whose CAA records allow only `pki.goog` reads `verified` with a Google Trust Services certificate on REST, GraphQL, MCP and the dashboard.
2. An ordinary custom domain still gets a Let's Encrypt certificate.
3. A domain whose CAA allows neither CA reads a certificate reason that names CAA and the two values to add.
4. ADR005 and ADR018's custom-domain rows record the two CAs; the dashboard's DNS instructions and the CLI skill pack agree.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w21` (2026-11-23), filed by `/pmx them all to w21`, item 3. Render's [tls](https://render.com/docs/tls) page was cited nowhere in the repository.
- **Render reference:** tls, read 2026-11-23: Render uses Let's Encrypt and Google Trust Services to issue certificates for custom domains, renews them before expiry, and covers wildcard domains.
- **bex today:** the only ClusterIssuers are `letsencrypt-staging` and `letsencrypt-prod` (`deploy/gitops/charts/cert-manager-issuers/clusterissuers.yaml`); the operator writes one issuer annotation per Ingress (`lego/operator/internal/controller/app_controller.go:606-626`, the wildcard issuer at `:4823`). bex-api already passes cert-manager's own words through (`domainCertificateReason`, `lego/backend/internal/apps/domains.go:647`). The CLI skill pack tells users to add `letsencrypt.org` and `pki.goog` CAA entries (`lego/cli/internal/skills/pack/skills/bex-domains/SKILL.md:119`, `references/dns-configuration.md:83`), promising a CA bex never uses. ADR018 records no difference.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first); custom domains ([ADR005](../../../../docs/ADR005-custom-domain.md)).
- **Expected outcome:** a domain configured for Render's two CAs works on bex, and a Let's Encrypt outage or rate limit does not stop new custom domains.
- **Why now:** since signup opened on 2026-11-21 every new custom domain depends on one ACME account, and the shipped CLI skill already states the two-CA behavior.
- **Decision state.** Let's Encrypt first, Google Trust Services when a host's order fails on CAA, a rate limit, or repeated failure (plus a CAA lookup at add time), is this proposal's recommendation, resting on ADR008 Render parity first; Render does not document its own selection rule. **Gate:** Google Trust Services' ACME needs an External Account Binding key from a Google Cloud project's Public CA API, which only the user can provide. It gates t002 and t005; t001, t003 and t004 ship first.
- **Out of scope:** a Google Trust Services variant of the wildcard DNS-01 issuer, which waits for that issuer to exist (w4/m326).
- **Render parity:** included. The certificate reason on REST, GraphQL and MCP and the dashboard's DNS instructions change.

## Parked 2026-11-24

- **Done and shipped in `323150d12d`:** t001 (ADR005 § Two certificate authorities, the ADR018 row ◐), t003 (the operator's move to `BEX_SECOND_CLUSTER_ISSUER`, inert while it is unset, and bex-api's plain-words CAA reason on REST, GraphQL and MCP), t004 (the dashboard's CAA hint in eight locales and the CLI skill's `pki.goog` records).
- **Gate:** Google Trust Services' ACME needs an External Account Binding key, which only the owner of a Google Cloud project can create. This machine has no `gcloud` and no Google Cloud credential (`.env` and `.env.example` name none), and production's only ClusterIssuers are `letsencrypt-prod` and `letsencrypt-staging` (read 2026-11-24).
- **What the user does** ([Google's steps](https://docs.cloud.google.com/certificate-manager/docs/public-ca-tutorial), read 2026-11-24): in a Google Cloud project, `gcloud services enable publicca.googleapis.com`, then `gcloud publicca external-account-keys create` (role `roles/publicca.externalAccountKeyCreator`). Put its `keyId` and `b64MacKey` in `.env` as `BEX_GTS_EAB_KEY_ID` and `BEX_GTS_EAB_HMAC_KEY`, push them with `scripts/gh-secrets.sh`, and start `/loopx w21` **within 7 days**: Google invalidates a key not bound to an ACME account by then. Once bound, the account does not expire.
- **Next, for the worker:** t002 adds those two names to `.env.example`, installs the Secret the way deploy.yml installs the onbex DNS-01 token (`scripts/onbex-dns01-token-secret.sh`), adds the `google-trust-services` ClusterIssuer (directory `https://dv.acme-v02.api.pki.goog/directory`, `externalAccountBinding`, the HTTP-01 solver of `letsencrypt-prod`), and sets `BEX_SECOND_CLUSTER_ISSUER=google-trust-services` on the operator in the prod overlay once the issuer reads Ready. Then t005 (the three production readings), t006 to t009.
