# Static-site trust-boundary baseline and rollout — 2026-07-29

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

## Production rollout and negative proof

The signed production workflow `30496239404` completed successfully for the final admission-policy change. During rollout, two checks caught defects before closeout:

- Argo briefly projected the renamed publisher setting onto the previous manager image. The service recovered after the manifest supplied both setting names, pointed at the same least-privilege publisher Secret; the current manager reads only `BEX_STATIC_PUBLISH_S3_SECRET`.
- The first admission policy's namespace CEL match condition evaluated false and skipped all validations. A live hostile-target probe detected the gap. The final policy scopes two bindings with API-server namespace selectors (hosting namespaces and `default`) and leaves only the ExternalName transition as a CEL match condition.

The post-fix live verifier passed with `PSL_EXPECTED=absent`:

- all 20 discovered identities were denied Service and Ingress create/update/patch/delete across their 50 effective scopes in all 16 hosting namespaces (the legacy `default` namespace plus 15 tenant namespaces); every App namespace was inside that admission-protected set;
- the manager retained those verbs and exact static/maintenance alias reconciliation;
- dry-run updates targeting bex-api, Zot, OpenBao, a tenant database, and external DNS were all denied;
- every live hosting ExternalName had the exact App label, controller owner, port, and fixed platform destination;
- a disposable paid-tier web fixture returned HTTP 200, switched through its exact maintenance alias to HTTP 503, returned to HTTP 200, and was deleted.

The split S3 credential lifecycle also passed:

- manual deploy `dep-d9l7hk03q32s73ba0gi0` published revision 3 before legacy removal;
- disposable static App `m54-static-223511` published revisions 1 and 2, returned HTTP 200 for both, deleted through the finalizer, and left zero objects under its App prefix;
- the stale default static-server Deployment/Service, ownerless tenant alias, and failed old purge Job were removed after proving that no Ingress used them;
- `bex-build/static-s3`, `bex-system/static-s3`, and `default/static-s3` were removed after a zero-reference scan; no legacy Secret remained;
- post-revocation manual deploy `dep-d9l81g10o6pc73aijtrg` published revision 4 with `bex-static-publish-s3`, and the platform URL still returned HTTP 200 with a 1,548-byte body;
- the final reader/publisher positive and negative matrix passed again after revocation.

The canonical PSL still lacks `onbex.co`, and the real-browser probe still demonstrates parent-cookie crossing. On 2026-07-30 the owner explicitly waived PSL inclusion and accepted this browser-domain divergence; the milestone closeout does not treat host-only/`__Host-` guidance as equivalent browser enforcement.
