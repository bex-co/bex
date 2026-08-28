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
| Route a subdomain | `CNAME` | label prefix (`www`, `api.stage`) | `<service>.<BEX_BASE_DOMAIN>` |
| Route an apex | `ALIAS` | `@` | `<service>.<BEX_BASE_DOMAIN>` |

The tenant creates the TXT record and invokes Verify. A missing value, mismatched value, resolver error, or timeout returns the named `DOMAIN_OWNERSHIP_PENDING` conflict and leaves both the claim and serving spec unchanged. A correct value promotes by claim id plus expected challenge; if the row was deleted/recreated during DNS lookup, `DOMAIN_CLAIM_STALE` wins and the replacement remains pending.

After ownership verifies, bex projects the traffic record into the App and cert-manager issues TLS. The visible states are therefore intentionally distinct:

- ownership pending, certificate pending, server pending;
- ownership verified, certificate pending, server pending;
- ownership verified, certificate verified, server active.

An already-verified Verify is idempotent but still freshly authorized. Failed verification records only bounded attempt metadata; the challenge never enters paths, query strings, logs, metrics, Kubernetes Secrets, or operator state. It is returned only on authorized domain reads/mutations because the dashboard must be able to recover the durable instruction after the add dialog closes.

## Persistence and rollout

Migration `0086_domain_claim_state` adds the closed `pending | verified` state, challenge/version, attempt timestamps, and verification timestamp. Every pre-migration row is backfilled `verified` with its existing primary/redirect ordering intact, so rollout cannot withdraw live traffic. The schema default remains `verified` for rolling compatibility with pre-m84 replicas, whose synchronous TXT gate still runs; claim-aware writers explicitly insert `pending` together with the random challenge. The down migration refuses to erase state while any pending row exists.

`domains.host` remains globally unique across both states. Same-App add is idempotent and returns the same row/challenge; a different App receives the existing non-enumerating conflict. `ReplaceDomainClaims` preserves unchanged verified rows and challenges, inserts only new declarations pending, and deletes declarations removed by service/Blueprint intent. `ListDesiredApps` reads verified rows only and emits a redirect only when both source and target are verified.

Storeless bex-api has nowhere durable to hold pending state, so it retains the older fail-closed behavior: verify the deterministic app-bound TXT before writing the App spec. It never serves an unverified host.

## Projection and certificates

The operator remains mechanism-only. It does not resolve DNS or transition domain business state. For each verified custom host it renders an Ingress rule and an independent TLS secret (`<app>-tls` for the first effective host, `<app>-tls-<host>` for later hosts), isolating certificate failures.

When a safe shared-hosting suffix is configured, the traffic target is the App's platform host. Apex domains use provider ALIAS/ANAME/CNAME-flattening because bex has no stable tenant-facing load-balancer IP. Production must not enable an ordinary registrable `BEX_BASE_DOMAIN`; ADR029's Public Suffix/browser-isolation gate still applies.

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
