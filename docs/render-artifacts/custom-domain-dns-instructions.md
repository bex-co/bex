# Render custom-domain contract and the Bex mapping

**Rechecked:** 2026-10-19 (w5/m137: the former Bex ownership-TXT extension is withdrawn; [ADR005](../ADR005-custom-domain.md) makes Render's traffic-DNS flow the hard contract, and the mapping below describes the converged code). **Primary evidence:** Render's current [custom-domain guide](https://render.com/docs/custom-domains), [public OpenAPI](https://api-docs.render.com/openapi/render-public-api-1.json), [Verify endpoint](https://api-docs.render.com/reference/refresh-custom-domain), [Blueprint reference](https://render.com/docs/blueprint-spec), and the repository-pinned official CLI module `github.com/render-oss/cli@v1.1.3-0.20260721145337-d8fd7c2bb09d`.

## Current Render behavior

- Dashboard workflow: Add domain → configure traffic DNS → Verify. The guide says the domain does not yet point to the service after Add; successful Verify starts TLS issuance, and routing can still converge briefly afterward.
- Adding an apex auto-adds `www` redirecting to the apex; adding `www` auto-adds the apex redirecting to `www`. Blueprint `domains` has the same pairing rule.
- Subdomains use CNAME to the service's `onrender.com` hostname. Provider-specific apex instructions use ALIAS/ANAME/CNAME flattening or Render's IPv4 load-balancer A record. Wildcards additionally use `_acme-challenge` and `_cf-custom-hostname` CNAMEs.
- REST paths are list/add at `/services/{serviceId}/custom-domains`, get/delete at `/{customDomainNameOrID}`, and verify at `/verify`.
- OpenAPI `customDomain` fields are `id`, `name`, `domainType`, `publicSuffix`, `redirectForName`, `verificationStatus`, `createdAt`, and optional server metadata. `verificationStatus` is `unverified | verified`; list filters use that vocabulary. Add returns `201` with an array because pairing can create two rows. Cross-service add can return `409`.
- Verify is `POST` and returns `202` with no body. Render describes it as a manual trigger that is usually unnecessary because verification also runs automatically.
- The pinned official CLI's generated client contains list/create/get/delete/refresh request builders and the same `unverified | verified` types. No command outside `pkg/client` exposes custom-domain management to a CLI user, so Bex claims REST-client compatibility, not a user-facing `bex domains` command.

## Bex mapping

- bex-api implements the same lifecycle: Add persists the domain `unverified`, the tenant creates the one traffic record, verification checks that record, and only then is the host projected for ingress/operator TLS. Verification runs automatically (a 30s sweep in bex-api) and on manual trigger.
- Internal storage keeps a `pending | verified` claim state (pending is wire `unverified`); promotion is an id-keyed CAS, so a claim deleted/recreated during a lookup is never promoted by the stale check.
- The traffic predicate (`lego/backend/internal/apps/domains.go`, `PointsAtService`): the host's CNAME chain reaches the service's platform host, or its resolved addresses intersect the platform host's — the ALIAS/ANAME/flattening and direct-A apex forms. A proxied (Cloudflare anycast) record satisfies neither branch.
- REST returns Render's `201` array from add (requested + auto-paired sibling) and `202` with no body from verify; `verificationStatus` is `unverified | verified` meaning DNS verification, and the list filter takes the same spellings (`pending` remains an accepted alias). GraphQL/MCP return the fresh domain view from the same check.
- Cross-service writes still 409; an unverified claim older than 48h is reclaimed when a conflicting add arrives (invisible anti-squatting bound).

## Remaining divergences (all named in ADR005)

1. **Additive read fields**: `dnsRecord`, `certificateStatus`, `serverStatus`, and `certificateReason` — safe supersets; no Render field is changed and none imposes a tenant step. (The former `ownershipStatus`/`ownershipDnsRecord` and the synchronous named pending conflicts are deleted.)
2. **Apex `A` value**: guidance leads with ALIAS/ANAME as Render's does; the `A` fallback names the platform host's currently resolved edge addresses where Render prints its committed-stable `216.24.57.1`.
3. **Wildcards**: bex rejects tenant wildcards; Render supports them behind extra TXT machinery. Revisit only as its own decision.
4. **Invisible refusals**: reserved platform hosts and bare public suffixes are refused; Render documents no such rule, and neither can trigger for a servable domain.
5. **Pair deletion cascade**: Render's public references do not specify it; bex's rule is in [the pairing evidence](custom-domain-pairing.md).
6. **Storeless mode** (no Render counterpart): the same traffic predicate runs synchronously before a host enters the App spec.

The protection the withdrawn TXT bought, and why it was traded away, is recorded as accepted residuals in [ADR005](../ADR005-custom-domain.md).
