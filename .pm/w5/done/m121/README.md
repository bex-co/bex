# w5 · m121 — Custom domains: public-suffix-aware validation, correct DNS record names, one TLS secret name

**Worker:** worker5 **Goal:** bex refuses bare public suffixes, tells users the right CNAME name under multi-label suffixes, and reports long hosts as verified once their certificate exists. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Refuse hosts with no registrable domain — **DONE** | 30m | — |
| t002 | Name CNAME records relative to the registrable domain — **DONE** | 30m | — |
| t003 | One TLS secret-name function in types — **DONE** | 45m | — |
| t004 | Decide how much of the dashboard's parent domain to reserve — **DONE** | 30m | — |
| t005 | Render parity — **DONE** | 20m | t001, t002, t003, t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- `co.uk` and `github.io` are refused with a coded 400, and `example.co.uk` is accepted.
- `www.example.co.uk` gets CNAME name `www` (zone `example.co.uk`), consistent with the TXT ownership record; apex records are unchanged.
- Backend and operator compute TLS Secret names with the same `types` function, and a long host shows verified once its Secret exists.
- The parent-domain reservation decision is recorded (a user decision if it changes self-host behavior).

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `6f4ca308a` (w4/190). Code-verified at `4baa77e02`: the hostname check only requires a dot; `dnsRecordFor` strips two labels; and `tlsSecretForHost` truncates while the operator hashes.
- **Goal linkage:** ADR005 custom domains; ADR018 parity.
- **Expected outcome:** Correct DNS guidance for every public-suffix domain, and no long hosts stuck as pending.
- **Why now:** w4/190 touched this validation on 2026-10-04, and the wrong record name misleads users on common ccTLDs.
- **Render parity included:** domain errors and DNS instructions change on every surface.

## Evidence — 2026-10-06

**Public suffixes refused (t001).** A host being claimed must have a registrable domain under the public suffix list (`claimableHostname`). `co.uk` and the private suffix `github.io` answer 400 `CUSTOM_DOMAIN_INVALID` ("it is a public suffix; use a domain registered under it"); `example.co.uk` and `shop.example.com.au` are added. The rule applies where a host is claimed: `AddDomain`, and service create, update and Blueprint `spec.hosts` (`canonicalHosts`). Lookups (get, verify, delete) do not apply it, so a suffix stored before it can still be deleted (`TestAStoredPublicSuffixCanStillBeDeleted`).

**CNAME names (t002).** `dnsRecordFor` names the record by the labels below the registrable domain, the zone the ownership TXT record is named in: `www.example.co.uk` → `www` (was `www.example`), `a.b.example.com` → `a.b`, `shop.example.com.au` → `shop`; apex records are unchanged. The dashboard derives the zone it shows from this name, so its DNS panel now shows `example.co.uk` where it showed `co.uk`.

**One TLS Secret name (t003).** `v1alpha1.TLSSecretName` is the operator's former `tlsSecretName`, byte for byte, and both modules call it: the operator at its three call sites (Ingress TLS, the TLS Secret history and the owned-name set), bex-api in `tlsSecretForHost`. bex-api used to truncate at 253 characters the name the operator hashes, so a host that long stayed pending after its certificate was issued (`TestALongHostIsVerifiedOnceItsSecretExists`, a 248-character host). The types test pins every name shape, the hashed one checked against an independent `shasum`, because a renamed Secret would strand an issued certificate.

**Reserved zones (t004), decided and implemented.**

- Today: no tenant may claim a platform host (dashboard, API, SSH, deploy hooks) or anything under the registrable domain of one, unless that domain is `BEX_BASE_DOMAIN`'s. Production reserves all of `bex.co`. A self-hoster whose services live on a separate apex, the cookie-isolation layout production uses, cannot claim anything under the dashboard's own domain.
- Options: (a) keep it; (b) reserve the exact platform hosts plus a configured zone list whose default is today's rule; (c) reserve only the exact hosts by default, which would let tenants claim other `bex.co` names in production.
- Decision: (b), as `BEX_RESERVED_DOMAINS`. It changes nothing in production or for an existing self-host, so t004's rule for a user decision (a change to production behavior) does not apply, while (c) would have needed one. A self-hoster with the dashboard at `bex.acme.com` sets it to `bex.acme.com` and can claim `www.acme.com`; the platform hosts stay reserved (`TestReservedDomainsReplaceThePlatformApex`). Entries are normalized like a custom domain, and a malformed one refuses to start (`TestLoadConfigReservedDomains`).

**Render parity (t005).**

- REST, GraphQL and MCP share `AddDomain`, the create and Blueprint `canonicalHosts` path and the domain views, so the refusal code and the DNS records match on every surface; the dashboard renders the server's refusal and records.
- Render documents no rule for a bare public suffix. Refusing it is a bex superset, like the reserved-host and wildcard refusals already recorded in ADR018. Render's own DNS instructions name records relative to the zone, as bex now does for every suffix. `BEX_RESERVED_DOMAINS` is a self-host setting with no Render counterpart.
- Docs: ADR005 (DNS names, the claim rule, the shared TLS name, reserved hosts), ADR018 (custom-domain row), `lego/backend/AGENTS.md` (`BEX_RESERVED_DOMAINS`).

**`/simplify` (t006)**, three reviews. Applied:

- Both the reuse and the quality review found that the change did not meet t003's goal. bex-api decided whether a host was the App's first with its own rule, not the operator's host order (`EffectiveHosts`), and a probe confirmed it. A service created with `domains` serves its first domain as `spec.host`: the operator issued that certificate into `<app>-tls`, while bex-api looked for `<app>-tls-<host>`, so the primary domain stayed pending forever. The same held for a service whose platform subdomain is off. `AppSpec.TLSSecretNameFor` now applies the operator's order with the shared `BEX_BASE_DOMAIN`, for domain reads, certificate reasons and the product-analytics sampler (`TestTheFirstEffectiveHostIsVerifiedFromTheLegacySecret`; `TestTLSSecretNameForFollowsEffectiveHosts` covers a custom primary, the platform host first, subdomain off, not exposed, and the App's own platform host listed).
- The create and Blueprint refusal was untested: reverting it left the suite green. `co.uk` and `github.io` are now rows in the cross-surface code table (REST, GraphQL and MCP), the Blueprint validation table and the create table.
- With `BEX_RESERVED_DOMAINS` set, the exact platform-host check compared hosts as configured, so `https://Bex.Acme.com` left `bex.acme.com` claimable. Platform hosts are normalized now. The docs say the zones must cover every host the platform serves, and must not cover `BEX_BASE_DOMAIN`.
- `dnsRecordFor` normalizes a stored host first, so `WWW.Example.COM.` reads `www`, not the whole name.
- Reuse: `BEX_RESERVED_DOMAINS` parses through `apps.ParseReservedDomains` on the custom-domain hostname rule, so it also refuses single labels and IP literals. `TLSSecretName` lives in `secretnames.go` beside the other cross-boundary Secret names. The public-suffix refusals moved into existing tables.
- Efficiency: an empty platform host (SSH off, or an unset public URL) is skipped again, and zones are deduplicated.
- Docs: stale comments on `PlatformHosts`, `canonicalHostname`, `canonicalHosts`, `dnsRecordFor` and the DNS record view; the Prometheus alert's "keep in sync" note; the DNS-instructions artifact; a Blueprint that still lists a stored public suffix fails validation until the host leaves it (ADR005).

Skipped:

- Exempting a host the App already holds from the public-suffix rule. A Blueprint listing a suffix stored before the rule must drop it; ADR005 says so.
- Caching reserved zones: under a microsecond per check, and tests change the fields between calls. The real cost is the host collision sweep, filed as w5/094.

**Tests (t007).** New: `TestAStoredPublicSuffixCanStillBeDeleted`, `TestDNSRecordNameIsRelativeToTheRegistrableDomain`, `TestALongHostIsVerifiedOnceItsSecretExists`, `TestTheFirstEffectiveHostIsVerifiedFromTheLegacySecret`, `TestReservedDomainsReplaceThePlatformApex`, `TestLoadConfigReservedDomains`, and in types `TestTLSSecretName`, `TestTLSSecretNameAtTheLimit` and `TestTLSSecretNameForFollowsEffectiveHosts`. New table rows in `TestCustomDomainRefusalsAreCodedOnEverySurface`, `TestBlueprintValidateRefusesUnclaimableDomains` and `TestCreateRejectsReservedAndClaimedHosts`. The operator's own TLS-name test moved to types. Each fix fails its test when reverted (8 mutations):

- `AddDomain` without the rule fails the code table on REST, GraphQL and MCP;
- create without it fails the create and Blueprint tables;
- the two-label CNAME strip fails the DNS test;
- truncating instead of hashing fails the pinned types tests;
- the old first-host rule fails the legacy-Secret test;
- ignoring `BEX_RESERVED_DOMAINS`, or comparing platform hosts as configured, fails the reserved-zones test;
- parsing zones without the hostname rule fails the config test.

**Gates.**

- The backend suite on fresh Postgres, OpenFGA and OpenBao is green (68 packages); `/ship`'s gate runs it again before the push.
- The operator's `make test`, with codegen and envtest, passes (25 packages), and so do the types tests.
- `make lint`: 0 issues in all four modules, including the whole-program dead-code pass.
