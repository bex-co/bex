# Custom domains — Render-identical setup: traffic DNS is the verification

**Amended 2026-10-19 (user decision). Render parity is a hard constraint on the tenant-facing custom-domain flow.** A tenant setting up a domain on bex performs exactly the steps Render documents — Add → create the traffic DNS record → Verify (usually automatic) → TLS — and no bex-only step. This supersedes the m84 design recorded here before, which added a durable random `_bex-challenge` ownership TXT on top of the traffic record as a deliberate security extension; that extension doubled the records for an apex+www pair (two TXT values on one shared host, each with keep-the-sibling warnings, plus two traffic records, each manually re-checked) and is **withdrawn**. What survives from m84 is the gate, not the proof: an unverified host still never routes (the [ADR045](ADR045-security-review-round3.md) host-hijack fix), but verification now means _the traffic DNS record points at this service_, exactly as on Render. Security reviews must not reintroduce a tenant-visible ownership TXT (or any bex-only DNS step) for ordinary domains; harden invisibly instead (see §Accepted residuals). Implemented by `w5/m137`; [ADR018](ADR018-render-parity.md)'s custom-domain rows carry the per-surface evidence.

A custom domain has two independent lifecycles:

1. **DNS verification admission** — bex stores a globally unique claim in Postgres. A new claim is `pending` (Render's `unverified`). Only a fresh resolution showing the host's traffic DNS pointing at this service's platform host (the predicate in §Tenant workflow) can atomically promote that same row to `verified`.
2. **Serving convergence** — only verified rows project into `App.spec.host` / `App.spec.hosts`. The operator then creates the Ingress and per-host Certificate; certificate and server status remain pending until the cluster reports them active.

This split is a control-plane invariant, not a dashboard convention: no supported direct add, service create, Blueprint sync, sibling add, or projector resync can route an unverified host.

## Tenant workflow

After an authorized add, REST, GraphQL, MCP, and the dashboard return one instruction per domain — the traffic record, Render's table:

| Purpose | Type | Name | Value |
| --- | --- | --- | --- |
| Route a subdomain | `CNAME` | labels below the registrable domain (`www`, `api.stage`; `www` for `www.example.co.uk`) | `<service>.<BEX_BASE_DOMAIN>` |
| Route an apex | `ALIAS` / `ANAME` / CNAME flattening | `@` | `<service>.<BEX_BASE_DOMAIN>` |
| Route an apex without ALIAS support | `A` | `@` | the platform host's current edge addresses (Render's fixed-LB-IP fallback, `216.24.57.1`, in bex form) |

Names are relative to the host's registrable domain under the public suffix list (w5/m121). Until then the CNAME stripped two labels, so `www.example.co.uk` read `www.example`, a name in zone `co.uk`. A host must have a registrable domain to be claimed: a bare public suffix such as `co.uk`, or the private suffix `github.io`, is refused with `CUSTOM_DOMAIN_INVALID`. Lookups, verify and delete included, do not apply the rule, so a host stored before it can still be removed. A Blueprint that still lists such a host fails validation until the host leaves it. The same holds for a name DNS cannot hold (w4/353): a label over 63 octets (RFC 1035 §2.3.4, which the DNS-1123 subdomain check never applies per label) or an all-numeric top-level label (RFC 1123 §2.1) is refused with `CUSTOM_DOMAIN_INVALID` when claimed, because its CNAME could never exist and its claim could never verify. Unlisted alphabetic TLDs (`example.c`) stay claimable, since refusing them would need a TLD list.

The tenant creates the traffic record; verification then runs automatically (a bounded background sweep re-checks unverified claims) and can also be triggered manually with Verify — Render's "usually unnecessary" manual trigger. The **traffic predicate** is a fresh, bounded resolution of the host: it points at this service iff (a) its CNAME chain includes the service's platform host `<slug>.<BEX_BASE_DOMAIN>`, or (b) its resolved address set intersects the platform host's resolved address set (what ALIAS/ANAME/flattening and a direct apex `A` produce). Branch (b) proves "points at the bex edge" and the globally unique claim binds that host to this one service — the same structure as Render's apex `A` to its shared load-balancer IP. A missing record, a record pointing elsewhere, a resolver error, or a timeout leaves the claim unverified and the serving spec unchanged; it is not an error state, and REST Verify answers Render's `202` with no body. Promotion is by claim id from `pending`; if the row was deleted/recreated during DNS lookup the replacement remains unverified. A Cloudflare-proxied (orange-cloud) record resolves to Cloudflare anycast addresses and satisfies neither branch, so — as Render's Cloudflare guide requires — the record must be DNS-only until the domain verifies and its certificate issues, after which the tenant may proxy it (Full (strict), per §Projection and certificates). Verification is durable once granted: later DNS changes do not un-verify a domain.

After DNS verifies, bex projects the host into the App and cert-manager issues TLS. The visible states are therefore intentionally distinct:

- unverified, certificate pending, server pending;
- verified, certificate pending, server pending;
- verified, certificate verified, server active.

The middle state is the one that can stall indefinitely, and until `w3/m85` it was mute: cert-manager knew exactly why issuance was blocked, but the tenant saw only "pending". A domain read now carries a nullable `certificateReason` — cert-manager's own words, read from the host's `Challenge` (`status.reason`, the field that says `Waiting for HTTP-01 challenge propagation: wrong status code '404'` for a proxied apex), then a failed `Order`, then the `Certificate`'s not-Ready condition. It is populated only while the certificate is pending, is read-only and namespace-scoped (the `bex-tenant-api` grant), and is a hint layered over the authoritative Secret-based status: a cluster without cert-manager, or without the grant, reports an empty reason rather than failing the read. The dashboard renders it on the domain row together with the fix-it text — the record must be DNS-only, not proxied, pointing at the service's `<name>.onbex.co` host.

An already-verified Verify is idempotent but still freshly authorized. Failed checks record only bounded attempt metadata. There is no tenant-held secret in this design: the only instruction a domain read must be able to recover after the add dialog closes is the traffic record itself.

## Persistence and rollout

Migration `0086_domain_claim_state` adds the closed `pending | verified` state, challenge/version, attempt timestamps, and verification timestamp. Every pre-migration row is backfilled `verified` with its existing primary/redirect ordering intact, so rollout cannot withdraw live traffic. The 2026-10-19 amendment keeps that state machine and retires what fed it (w5/m137): writers insert `pending` with no challenge, promotion keys on the traffic predicate instead of an expected challenge value, and migration `0152_domain_traffic_verification` drops the pending-challenge constraint and clears pending challenges (the columns themselves stay one rolling window for pre-amendment replicas; their later drop is mechanical cleanup). Rows `verified` under the TXT regime stay verified — rollout never withdraws live traffic; rows still `pending` on a TXT simply become traffic-unverified claims and verify when their traffic record lands (a tenant mid-TXT-setup creates the traffic record instead; the TXT they may already have published is inert and can be removed).

`domains.host` remains globally unique across both states. Same-App add is idempotent and returns the same row; a different App receives the existing non-enumerating conflict. An unverified claim older than 48 hours is reclaimed when a conflicting add for the same host arrives, so an unverified row cannot squat a hostname forever (an invisible rule: it adds no tenant step, and a tenant who returns later simply re-adds). `ReplaceDomainClaims` preserves unchanged verified rows, inserts only new declarations pending, and deletes declarations removed by service/Blueprint intent. `ListDesiredApps` reads verified rows only and emits a redirect only when both source and target are verified.

A Blueprint is held to the same uniqueness before anything is written (w5/105). Two of its declarations that claim one host, or a host and its www or apex sibling, which a claim reserves too, are refused as `BLUEPRINT_DUPLICATE_HOST` at the second declaration's `domains[i]` (or `domain`). Validation, preview and apply all compile the manifest, so the apply no longer creates the first service and then refuses the second. One service may list its own apex and www.

Storeless bex-api has nowhere durable to hold pending state, so it retains the older fail-closed behavior with the new proof: run the traffic predicate synchronously before writing the App spec. It never serves an unverified host.

## Projection and certificates

The operator remains mechanism-only. It does not resolve DNS or transition domain business state. For each verified custom host it renders an Ingress rule and an independent TLS secret (`<app>-tls` for the first effective host, `<app>-tls-<host>` for later hosts, a hash of the host past the 253-character name limit), isolating certificate failures. The operator and bex-api compute that name with one function, `v1alpha1.TLSSecretName` (w5/m121): bex-api used to truncate the name the operator hashes, so a long host stayed pending after its certificate was issued.

**Certificate loading isolation (w8/m118).** Independent Secret names also need independent loading: the pinned Traefik Kubernetes provider stops reading an Ingress's TLS references at the first missing Secret or empty certificate/key. The operator therefore places usable references before pending ones, preserving their relative order. Every reference remains present for ingress-shim to issue or renew it; serving-rule order, router names and the host-to-Secret mapping stay unchanged. Secret reads use the existing uncached tenant client, without widening the manager's Secret cache. Multi-host Kubernetes services, including direct-published static sites and sleeping services, reconcile at least every 30 seconds to observe issuance or deletion. Production's wildcard fallback continues to serve platform hosts while their individual certificate is pending; a ready custom certificate can load alongside it. Self-hosted clusters keep their existing per-host issuance path.

**Reserved hosts.** No tenant may claim a platform host (the dashboard, API, SSH and deploy-hook hosts) or a host in a reserved zone. By default the reserved zones are the registrable domain of each platform host other than `BEX_BASE_DOMAIN`'s, so production reserves all of `bex.co` (w4/190). `BEX_RESERVED_DOMAINS`, a comma-separated list, replaces those zones (w5/m121). A self-hoster with the dashboard at `bex.acme.com` and services on another apex sets it to `bex.acme.com`, and can then claim `www.acme.com`. The platform hosts bex-api knows stay reserved, but the zones must cover every host the platform serves: production, for one, also serves auth, OAuth, observability and schema hosts under `bex.co`. A zone covering `BEX_BASE_DOMAIN` would reserve the services' own hosts too. Unset changes nothing.

When a safe shared-hosting suffix is configured, the traffic target is the App's platform host. Apex guidance leads with provider ALIAS/ANAME/CNAME-flattening because bex has no committed-stable tenant-facing load-balancer IP; the `A` fallback value shown to a tenant is the platform host's currently resolved edge addresses, with ALIAS named as the form that survives an edge-address change (Render can print one fixed IP because it commits to its load balancer's stability). Production must not enable an ordinary registrable `BEX_BASE_DOMAIN`; ADR029's Public Suffix/browser-isolation gate still applies.

**HTTP redirects to HTTPS (w8/m118).** Render redirects every HTTP request to HTTPS, and bex now does the same for every tenant host: platform subdomains, verified custom domains, static sites, and a hibernated service behind the activator. The auto-paired sibling's own redirect already targets `https://`. The App's Ingress leads its Traefik middleware chain with an owned `<app>-https` RedirectScheme to `https`. It is permanent: 301, or 308 for a request other than GET or HEAD. Path and query are kept. Traefik builds a plain-HTTP router and a `websecure-` TLS sibling from each rule and applies the chain to both. A request already on https passes through, so serving and the bytes the TLS sibling carries (w1/m51) are unchanged. An untrusted client's `X-Forwarded-Proto` cannot skip the redirect. The `web` entrypoint still has no global redirect, so cert-manager's HTTP-01 solver keeps answering on :80, through its own Ingress that carries none of the App's middlewares. A throwaway kind cluster running the pinned Traefik v3.7.5 with the production values confirmed each of these behaviors (2026-10-18). Some things follow, as on Render:

- `ws://` clients and `http://` webhook senders must use `wss://` and `https://`.
- A custom domain whose certificate is still pending redirects to an https endpoint that serves the platform wildcard certificate until issuance completes.
- A Cloudflare-proxied domain must use Full (strict) SSL mode; Flexible mode reaches the origin over HTTP and loops.
- HTTPS on a port other than 443 is redirected to 443.

`BEX_HTTPS_REDIRECT=false` turns the redirect off for a cluster whose hosts get no certificates. HSTS is deliberately not sent yet. A tenant that wants it can set the header itself; a platform-wide policy on the shared suffix would bind every tenant at once, and it would need a decision on preload and subdomains first.

Cloudflare-for-SaaS registration, when used at the edge, remains an operational step outside this claim state machine. The database claim gates Bex routing; edge custom-hostname registration and edge/origin certificate issuance are separate convergence mechanisms.

## www ↔ apex pairing

Direct add mirrors Render's pairing via the public-suffix list:

- adding `example.org` creates pending claims for it and `www.example.org`;
- adding `www.example.org` creates pending claims for it and `example.org`;
- the auto-created sibling records `redirectForName` to the explicit host;
- each row verifies independently, by its own traffic record (the apex ALIAS and the `www` CNAME are separate records, as on Render);
- no redirect enters the App spec until both source and target are verified;
- explicitly adding the sibling clears its redirect without resetting either claim's state;
- deleting the canonical/direct claim atomically deletes its generated redirecting sibling, while deleting only the generated sibling preserves the canonical claim unchanged;
- after the sibling is explicitly added and its redirect cleared, both claims are independent and deleting either preserves the other;
- the same idempotent rule applies to pending and verified claims in both add directions.

Cross-App collision covers both halves because each sibling is a real globally unique row. Wildcard tenant domains remain rejected: literal uniqueness cannot safely arbitrate a wildcard against every concrete hostname beneath it.

## Render compatibility

Render's documented workflow is the contract (rechecked 2026-10-19 against [the custom-domain guide](https://render.com/docs/custom-domains), the provider DNS guides, and the public OpenAPI; see [the pinned comparison](render-artifacts/custom-domain-dns-instructions.md)): Add → configure traffic DNS → Verify, apex/`www` auto-pairing, TLS issuance beginning at verification, and verification that also runs automatically so the manual Verify is usually unnecessary. Bex adopts it whole on every surface a tenant sees:

- **One DNS step.** The dashboard shows the single traffic record per domain (Type/Host/Target, copy-ready) with Render's short guidance; an apex+www pair is two records total. No ownership panel, no TXT, no cross-domain record-keeping warnings — those UI artifacts existed only to serve the withdrawn TXT design and are deleted with it.
- **One verification state, Render's spelling and meaning.** REST `customDomain.verificationStatus` is `unverified | verified` and means traffic-DNS verification (the pre-amendment field reported TLS issuance instead — a silent semantic swap this amendment also corrected; certificate progress is visible through the additive `certificateStatus`/`certificateReason` and the dashboard's certificate column, never by overloading Render's field). The list filter takes the official `unverified` spelling; bex's established `pending` remains accepted as an additive alias. `ownershipStatus` and `ownershipDnsRecord` are removed from every surface — the concept they exposed no longer exists.
- **Render's wire shapes.** Add answers `201` with the array of created domains (the pair), as Render's OpenAPI declares and its generated CLI client decodes; Verify answers `202` with no body. GraphQL and MCP have no Render counterpart, so they keep returning the fresh domain view from the same core check.
- **Automatic verification.** The background sweep plus the manual trigger run one shared promote path, so a domain flips to `verified`, projects, and starts TLS without the tenant returning to click anything, per domain.

Named divergences that survive, none of which adds a tenant setup step:

- **Pair deletion cascade** (above): Render's public docs and delete reference do not specify whether deleting one pair member cascades the generated sibling (rechecked 2026-08-28), so bex's rule is a choice where Render is silent.
- **Wildcards**: Render supports tenant wildcards behind extra `_acme-challenge` / `_cf-custom-hostname` machinery; bex still rejects them (§www ↔ apex pairing). Revisit only as its own decision — it must not complicate the ordinary flow.
- **Invisible safety refusals**: reserved platform hosts and the bare-public-suffix refusal have no Render counterpart but only ever refuse a host that could never legitimately be served.
- **Additive read fields**: `dnsRecord` (Render shows the record only in its dashboard), `certificateStatus`/`serverStatus` (the TLS/serving axes Render folds into its UI), and `certificateReason` remain safe supersets in ADR018's established class — omitted/unchanged for a strict Render client, and never a changed Render field.
- **Apex `A` value**: Render prints one committed-stable load-balancer IP; bex prints the platform host's currently resolved edge addresses (§Projection and certificates).

## Accepted residuals (of dropping the TXT)

The withdrawn TXT bought real protection, and giving it up is a considered trade for parity, recorded so later reviews disposition these as **accepted per this ADR** rather than re-opening the tenant flow:

- **Dangling-DNS takeover.** A tenant who deletes a service but leaves its `CNAME` at `<old-slug>.onbex.co` leaves a record that will satisfy the predicate for whoever next holds that slug and claims the host. The TXT's per-claim random value closed this; traffic-DNS verification — Render's model — does not. The invisible candidate, never re-minting a deleted service's slug, was evaluated and **rejected** in `w5/m137` (t007): slug minting deliberately mirrors Render (bare name, random suffix only on a live collision — migration `0020_apps_slug`), so a tombstone would make a recreated service's platform URL diverge from Render's for exactly the legitimate delete-and-recreate flow, while the takeover additionally needs the victim to have deleted the service itself (while it lives, its slug cannot be re-minted) and left its DNS behind. Revisit at open signup, with the #PSL gate; any future mitigation must stay invisible (never a tenant DNS step).
- **Unverified squatting.** Anyone can add a victim's hostname and hold the globally unique row. Bounded by the 48-hour reclaim-on-conflicting-add rule (§Persistence and rollout), after which the legitimate owner's add wins; the squatter's row never routes in the meantime.
- **Shared-edge apex proof.** The address-intersection branch proves a host points at the bex edge, not at one service; service binding comes from the unique claim row. This is exactly Render's apex-`A` structure and is safe only while claim uniqueness and the unverified-never-routes gate hold — both are invariants above, unchanged by this amendment.

See [ADR018](ADR018-render-parity.md) for the per-surface ledger, which `w5/m137` updates as each surface converges.
