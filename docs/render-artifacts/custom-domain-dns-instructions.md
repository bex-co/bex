# Render custom-domain contract and Bex ownership extension

**Rechecked:** 2026-08-17. **Primary evidence:** Render's current [custom-domain guide](https://render.com/docs/custom-domains), [public OpenAPI](https://api-docs.render.com/openapi/render-public-api-1.json), [Verify endpoint](https://api-docs.render.com/reference/refresh-custom-domain), [Blueprint reference](https://render.com/docs/blueprint-spec), and the repository-pinned official CLI module `github.com/render-oss/cli@v1.1.3-0.20260721145337-d8fd7c2bb09d`.

## Current Render behavior

- Dashboard workflow: Add domain → configure traffic DNS → Verify. The guide says the domain does not yet point to the service after Add; successful Verify starts TLS issuance, and routing can still converge briefly afterward.
- Adding an apex auto-adds `www` redirecting to the apex; adding `www` auto-adds the apex redirecting to `www`. Blueprint `domains` has the same pairing rule.
- Subdomains use CNAME to the service's `onrender.com` hostname. Provider-specific apex instructions use ALIAS/ANAME/CNAME flattening or Render's IPv4 load-balancer A record. Wildcards additionally use `_acme-challenge` and `_cf-custom-hostname` CNAMEs.
- REST paths are list/add at `/services/{serviceId}/custom-domains`, get/delete at `/{customDomainNameOrID}`, and verify at `/verify`.
- OpenAPI `customDomain` fields are `id`, `name`, `domainType`, `publicSuffix`, `redirectForName`, `verificationStatus`, `createdAt`, and optional server metadata. `verificationStatus` is `unverified | verified`; list filters use that vocabulary. Add returns `201` with an array because pairing can create two rows. Cross-service add can return `409`.
- Verify is `POST` and returns `202` with no body. Render describes it as a manual trigger that is usually unnecessary because verification also runs automatically.
- The pinned official CLI's generated client contains list/create/get/delete/refresh request builders and the same `unverified | verified` types. No command outside `pkg/client` exposes custom-domain management to a CLI user, so Bex claims REST-client compatibility, not a user-facing `bex domains` command.

## Bex mapping

Traffic instructions remain:

| Domain kind | Type    | Name         | Value                         |
| ----------- | ------- | ------------ | ----------------------------- |
| subdomain   | `CNAME` | label prefix | `<service>.<BEX_BASE_DOMAIN>` |
| apex        | `ALIAS` | `@`          | `<service>.<BEX_BASE_DOMAIN>` |

Bex adds an ownership instruction for every new managed claim:

| Type | Name | Value |
| --- | --- | --- |
| `TXT` | `_bex-challenge.<registrable-domain>` | one durable random `bex-domain-verification=…` value |

All Bex adapters expose the same neutral lifecycle:

| State | `ownershipStatus` | TLS `verificationStatus` | `serverStatus` | Serving spec |
| --- | --- | --- | --- | --- |
| New/wrong DNS | `pending` | `pending` | `pending` | Host absent |
| TXT verified, cert issuing | `verified` | `pending` | `pending` | Host present |
| Certificate ready | `verified` | `verified` | `active` | Host present |

REST add/get/verify, GraphQL, MCP, and the dashboard intentionally return the authorized pending TXT instruction so it remains recoverable. Durable random values are excluded from URLs, errors, logs, metrics, Secrets, and desired-App projection. Missing/wrong/resolver-failed proofs return `DOMAIN_OWNERSHIP_PENDING`; a delete/recreate race returns `DOMAIN_CLAIM_STALE`.

## Deliberate divergences

1. **Stronger admission.** Render verifies traffic DNS. Bex requires a separate random TXT before the hostname can enter any Ingress or Certificate input, preventing an unverified database declaration from becoming routable through another write path.
2. **Synchronous Verify result.** Render accepts an asynchronous trigger with 202/no body. Bex checks now and returns the fresh view, enabling one shared REST/GraphQL/MCP/dashboard contract.
3. **Explicit states.** Bex retains TLS `verificationStatus` and adds `ownershipStatus`; `pending` remains an accepted alias while REST filters also accept Render's official `unverified` spelling.
4. **Apex record.** Bex has no stable public A record and emits ALIAS to the platform host.
5. **Wildcards.** Render supports them with extra DNS machinery. Bex rejects tenant wildcards because a literal global unique index cannot arbitrate a wildcard safely against every concrete hostname.
6. **Add response.** Bex's established REST add returns the requested domain object, while the paired sibling is discoverable from the list. Render's current OpenAPI declares an array. This remains a documented wire-shape gap; changing it would break existing Bex clients and is not required for the ownership invariant.
