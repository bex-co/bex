# Static-site trust-boundary baseline — 2026-07-29

This is a sanitized production/pre-rollout capture for w7/m54. Commands used a mode-0600 kubeconfig outside the repository and never printed Secret payloads, access-key ids, tokens, kubeconfig bytes, or hashes.

## Browser domain

- Canonical `publicsuffix/list`: `onrender.com` present; `onbex.co` absent.
- Production eligibility: 15 hosting namespaces and 17 distinct `tenant_members.subject` values. The current upstream request template's 2,000–3,000-distinct-user gate is not met.
- Real Chrome harness: `onrender.com` rejected the attempted parent cookie; `onbex.co` accepted it and sent it to the sibling. Local storage and Service Workers remained origin-local on both.
- Control-plane separation: tenant content is `*.onbex.co`; dashboard/API/auth remain `*.bex.co`.

Reproduction: `PSL_EXPECTED=absent scripts/verify-static-site-security.sh repo`.

## Kubernetes route authority

The live public path was Cloudflare/Traefik → an App-owned ExternalName alias → `bex-system/bex-static-server:8080`. The static site's Ingress named the operator alias; an older manually repaired ownerless `bex-static-server` alias was stale and not in that request path.

Effective production authorization before the new admission policy:

| Identity                     | Service create/patch/delete | Ingress create |
| ---------------------------- | --------------------------- | -------------- |
| `bex-system/bex-api`         | no / no / no                | no             |
| `bex-system/bex-ssh-gateway` | no / no / no                | no             |
| day-to-day operator          | no / no / no                | no             |
| controller manager           | yes / yes / yes             | yes            |

`allowExternalNameServices=true` was global and no admission object constrained the manager's aliases. The milestone adds a fail-closed exact-shape policy and CI/live negative probes; server-side dry-run proved its CEL compiled against the production API server before shipping.

The static handler has no request-controlled upstream fetch: `TestRewriteNeverFetchesAnUpstreamURL` proves an internal-service-looking rewrite becomes an S3 object key and returns the object-origin miss.

## Object store

Before migration, `static-s3` existed in `bex-system`, `bex-build`, and `default`; a value-equivalence comparison (without printing values/hashes) showed all three held the same account-wide credential source. That credential could reach the static bucket and the `bex-tfstate` state/backup bucket. Production backups use `bex-tfstate` prefixes for etcd, OpenBao, bex-db, and tenant Postgres.

The new out-of-band preparation created two non-secret provider identities and two distinct Kubernetes consumers:

- `bex-static-reader` → `bex-system/bex-static-read-s3`
- `bex-static-publisher` → `bex-build/bex-static-publish-s3`

The live matrix passed before either workload consumed them:

| Probe                                                 | Result |
| ----------------------------------------------------- | ------ |
| publisher put/read/delete static object               | allow  |
| reader list/read static object                        | allow  |
| reader put/delete static object                       | deny   |
| reader and publisher list `bex-tfstate`               | deny   |
| reader and publisher list an unrelated account bucket | deny   |

The probe used a random `.bex-security-probe/` object and deleted it. No key identifier or object content entered this record. The legacy Secrets remain until the new deployment, fresh static lifecycle, repeat matrix, and workload-reference scan pass; [the rotation runbook](../runbooks/static-site-s3-rotation.md) forbids earlier removal.

## Pre-rollout service control

The existing production static site was `Running` at revision 2 and its platform URL returned HTTP 200 with a 1,548-byte body. The static-server was Ready. This is the availability baseline the split-credential rollout must preserve.
