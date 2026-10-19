# Custom domains — durable ownership before routing

A custom domain has two independent lifecycles:

1. **Ownership admission** — bex stores a globally unique claim in Postgres. A new claim is `pending` and carries one cryptographically random TXT challenge. Only an exact fresh DNS match can atomically promote that same row to `verified`.
2. **Serving convergence** — only verified rows project into `App.spec.host` / `App.spec.hosts`. The operator then creates the Ingress and per-host Certificate; certificate and server status remain pending until the cluster reports them active.

This split is a control-plane invariant, not a dashboard convention: no supported direct add, service create, Blueprint sync, sibling add, or projector resync can route an ownership-pending host.

## Tenant workflow

After an authorized add, REST, GraphQL, MCP, and the dashboard return two separate instructions:

| Purpose | Type | Name | Value |
| --- | --- | --- | --- |
| Prove ownership | `TXT` | `_bex-challenge.<registrable-domain>` | Durable random `bex-domain-verification=…` challenge |
| Route a subdomain | `CNAME` | labels below the registrable domain (`www`, `api.stage`; `www` for `www.example.co.uk`) | `<service>.<BEX_BASE_DOMAIN>` |
| Route an apex | `ALIAS` | `@` | `<service>.<BEX_BASE_DOMAIN>` |

Both names are relative to the same zone, the host's registrable domain under the public suffix list (w5/m121). Until then the CNAME stripped two labels, so `www.example.co.uk` read `www.example`, a name in zone `co.uk`. A host must have a registrable domain to be claimed: a bare public suffix such as `co.uk`, or the private suffix `github.io`, is refused with `CUSTOM_DOMAIN_INVALID`. Lookups, verify and delete included, do not apply the rule, so a host stored before it can still be removed. A Blueprint that still lists such a host fails validation until the host leaves it. The same holds for a name DNS cannot hold (w4/353): a label over 63 octets (RFC 1035 §2.3.4, which the DNS-1123 subdomain check never applies per label) or an all-numeric top-level label (RFC 1123 §2.1) is refused with `CUSTOM_DOMAIN_INVALID` when claimed, because its CNAME could never exist and its claim could never verify. Unlisted alphabetic TLDs (`example.c`) stay claimable, since refusing them would need a TLD list.

The tenant creates the TXT record and invokes Verify. A missing value, mismatched value, resolver error, or timeout returns the named `DOMAIN_OWNERSHIP_PENDING` conflict and leaves both the claim and serving spec unchanged. A correct value promotes by claim id plus expected challenge; if the row was deleted/recreated during DNS lookup, `DOMAIN_CLAIM_STALE` wins and the replacement remains pending.

After ownership verifies, bex projects the traffic record into the App and cert-manager issues TLS. The visible states are therefore intentionally distinct:

- ownership pending, certificate pending, server pending;
- ownership verified, certificate pending, server pending;
- ownership verified, certificate verified, server active.

The middle state is the one that can stall indefinitely, and until `w3/m85` it was mute: cert-manager knew exactly why issuance was blocked, but the tenant saw only "pending". A domain read now carries a nullable `certificateReason` — cert-manager's own words, read from the host's `Challenge` (`status.reason`, the field that says `Waiting for HTTP-01 challenge propagation: wrong status code '404'` for a proxied apex), then a failed `Order`, then the `Certificate`'s not-Ready condition. It is populated only while the certificate is pending, is read-only and namespace-scoped (the `bex-tenant-api` grant), and is a hint layered over the authoritative Secret-based status: a cluster without cert-manager, or without the grant, reports an empty reason rather than failing the read. The dashboard renders it on the domain row together with the fix-it text — the record must be DNS-only, not proxied, pointing at the service's `<name>.onbex.co` host.

An already-verified Verify is idempotent but still freshly authorized. Failed verification records only bounded attempt metadata; the challenge never enters paths, query strings, logs, metrics, Kubernetes Secrets, or operator state. It is returned only on authorized domain reads/mutations because the dashboard must be able to recover the durable instruction after the add dialog closes.

## Persistence and rollout

Migration `0086_domain_claim_state` adds the closed `pending | verified` state, challenge/version, attempt timestamps, and verification timestamp. Every pre-migration row is backfilled `verified` with its existing primary/redirect ordering intact, so rollout cannot withdraw live traffic. The schema default remains `verified` for rolling compatibility with pre-m84 replicas, whose synchronous TXT gate still runs; claim-aware writers explicitly insert `pending` together with the random challenge. The down migration refuses to erase state while any pending row exists.

`domains.host` remains globally unique across both states. Same-App add is idempotent and returns the same row/challenge; a different App receives the existing non-enumerating conflict. `ReplaceDomainClaims` preserves unchanged verified rows and challenges, inserts only new declarations pending, and deletes declarations removed by service/Blueprint intent. `ListDesiredApps` reads verified rows only and emits a redirect only when both source and target are verified.

A Blueprint is held to the same uniqueness before anything is written (w5/105). Two of its declarations that claim one host, or a host and its www or apex sibling, which a claim reserves too, are refused as `BLUEPRINT_DUPLICATE_HOST` at the second declaration's `domains[i]` (or `domain`). Validation, preview and apply all compile the manifest, so the apply no longer creates the first service and then refuses the second. One service may list its own apex and www.

Storeless bex-api has nowhere durable to hold pending state, so it retains the older fail-closed behavior: verify the deterministic app-bound TXT before writing the App spec. It never serves an unverified host.

## Projection and certificates

The operator remains mechanism-only. It does not resolve DNS or transition domain business state. For each verified custom host it renders an Ingress rule and an independent TLS secret (`<app>-tls` for the first effective host, `<app>-tls-<host>` for later hosts, a hash of the host past the 253-character name limit), isolating certificate failures. The operator and bex-api compute that name with one function, `v1alpha1.TLSSecretName` (w5/m121): bex-api used to truncate the name the operator hashes, so a long host stayed pending after its certificate was issued.

**Reserved hosts.** No tenant may claim a platform host (the dashboard, API, SSH and deploy-hook hosts) or a host in a reserved zone. By default the reserved zones are the registrable domain of each platform host other than `BEX_BASE_DOMAIN`'s, so production reserves all of `bex.co` (w4/190). `BEX_RESERVED_DOMAINS`, a comma-separated list, replaces those zones (w5/m121). A self-hoster with the dashboard at `bex.acme.com` and services on another apex sets it to `bex.acme.com`, and can then claim `www.acme.com`. The platform hosts bex-api knows stay reserved, but the zones must cover every host the platform serves: production, for one, also serves auth, OAuth, observability and schema hosts under `bex.co`. A zone covering `BEX_BASE_DOMAIN` would reserve the services' own hosts too. Unset changes nothing.

When a safe shared-hosting suffix is configured, the traffic target is the App's platform host. Apex domains use provider ALIAS/ANAME/CNAME-flattening because bex has no stable tenant-facing load-balancer IP. Production must not enable an ordinary registrable `BEX_BASE_DOMAIN`; ADR029's Public Suffix/browser-isolation gate still applies.

**HTTP redirects to HTTPS (w8/m118).** Render redirects every HTTP request to HTTPS, and bex now does the same for every tenant host: platform subdomains, verified custom domains, static sites, and a hibernated service behind the activator. The auto-paired sibling's own redirect already targets `https://`. The App's Ingress leads its Traefik middleware chain with an owned `<app>-https` RedirectScheme to `https`. It is permanent: 301, or 308 for a request other than GET or HEAD. Path and query are kept. Traefik builds a plain-HTTP router and a `websecure-` TLS sibling from each rule and applies the chain to both. A request already on https passes through, so serving and the bytes the TLS sibling carries (w1/m51) are unchanged. An untrusted client's `X-Forwarded-Proto` cannot skip the redirect. The `web` entrypoint still has no global redirect, so cert-manager's HTTP-01 solver keeps answering on :80, through its own Ingress that carries none of the App's middlewares. A throwaway kind cluster running the pinned Traefik v3.7.5 with the production values confirmed each of these behaviors (2026-10-18). Some things follow, as on Render:

- `ws://` clients and `http://` webhook senders must use `wss://` and `https://`.
- A custom domain whose certificate is still pending redirects to an https endpoint that serves the platform wildcard certificate until issuance completes.
- A Cloudflare-proxied domain must use Full (strict) SSL mode; Flexible mode reaches the origin over HTTP and loops.
- HTTPS on a port other than 443 is redirected to 443.

`BEX_HTTPS_REDIRECT=false` turns the redirect off for a cluster whose hosts get no certificates. HSTS is deliberately not sent yet. A tenant that wants it can set the header itself; a platform-wide policy on the shared suffix would bind every tenant at once, and it would need a decision on preload and subdomains first.

Cloudflare-for-SaaS registration, when used at the edge, remains an operational step outside this ownership state machine. The database claim gates Bex routing; edge custom-hostname registration and edge/origin certificate issuance are separate convergence mechanisms.

## www ↔ apex pairing

Direct add mirrors Render's pairing via the public-suffix list:

- adding `example.org` creates pending claims for it and `www.example.org`;
- adding `www.example.org` creates pending claims for it and `example.org`;
- the auto-created sibling records `redirectForName` to the explicit host;
- each row has its own durable challenge and must verify independently;
- no redirect enters the App spec until both source and target are verified;
- explicitly adding the sibling clears its redirect without rotating either claim;
- deleting the canonical/direct claim atomically deletes its generated redirecting sibling, while deleting only the generated sibling preserves the canonical claim unchanged;
- after the sibling is explicitly added and its redirect cleared, both claims are independent and deleting either preserves the other;
- the same idempotent rule applies to pending and verified claims in both add directions.

Cross-App collision covers both halves because each sibling is a real globally unique row. Wildcard tenant domains remain rejected: literal uniqueness cannot safely arbitrate a wildcard against every concrete hostname beneath it.

## Render compatibility

Render's current documented workflow is also Add → configure DNS → Verify, auto-pairs apex/`www`, and begins TLS issuance after verification. Its public docs and delete API reference still do not specify whether deleting one pair member cascades the generated sibling (rechecked 2026-08-28), so bex's pair-deletion rule above is a named divergence. Render's public REST schema exposes `verificationStatus: unverified | verified`; bex accepts official `unverified` filters and retains `pending` as its established alias. The official Verify endpoint returns `202` with no domain body, while bex returns the fresh domain view so GraphQL, MCP, and the dashboard share one result.

Bex's random ownership TXT, explicit `ownershipStatus` / `ownershipDnsRecord`, synchronous named pending conflicts, and hard non-serving pending invariant are deliberate security extensions. Render ordinarily proves service-domain control through traffic DNS and has additional TXT records only for wildcard machinery. See [the pinned comparison](render-artifacts/custom-domain-dns-instructions.md) and [ADR018](ADR018-render-parity.md).
