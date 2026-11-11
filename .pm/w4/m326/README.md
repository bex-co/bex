# w4 · m326 — A wildcard custom domain (`*.example.com`) is refused on every surface, though Render supports it; the refusal's own comment says Render does not, and the divergence has no sign-off

**Worker:** worker4 **Goal:** a service accepts a wildcard custom domain as Render does: the same DNS instructions, verification, a certificate, and routing for every hostname under it. Concrete hosts keep winning over a wildcard, and no workspace can capture another's hostnames. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Capture Render's wildcard custom-domain flow and amend ADR005 and ADR018, replacing the unsigned divergence | 45m | — |
| [t002](t002.md) | Backend: accept `*.<domain>` on add, with verification and Render's collision rules, on REST, GraphQL and MCP | 90m | t001 |
| [t003](t003.md) | Operator: route every name under the wildcard and issue its certificate by DNS-01 through the delegated `_acme-challenge` record | 120m | t001 |
| [t004](t004.md) | Dashboard: the Add Custom Domain dialog accepts a wildcard and shows both DNS records, en and zh | 45m | t002 |
| [t005](t005.md) | Blast radius: reserved hosts, public suffixes, apex/www pairing, plan limits, and existing concrete domains | 30m | t002, t003 |
| [t006](t006.md) | Live verification on a test zone bex controls | 30m | t004, t005 |
| [t007](t007.md) | Render parity | 15m | t006 |
| [t008](t008.md) | Simplify | 10m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Ten tasks, 440m (7h20m).

**Major (a Render feature is missing).** Multi-tenant apps put each customer on `<customer>.example.com`, and on Render one wildcard domain serves them all. On bex every add is refused.

## Finding

Live `/qa-find-bugs w4` loop, round 98, 2026-11-11 11:08–11:15 UTC (`muse.env`, workspace `bex-canary`; production runs the code of `7ca0227aa477`).

Fixture: Free image web service `qa-20261111-wc98` (`srv-dbq4p4a34ugc73cu1tig`, `docker.io/traefik/whoami:v1.10.1`). Deleted afterwards (DELETE 204, then GET 404). No domain was created.

### Probes

- **Dashboard.** Settings → Custom Domains → Add Custom Domain → Name `*.qa98wild.example.com` → Add Domain. The dialog stays open with `alert: Wildcard hostnames are not allowed: "*.qa98wild.example.com"`.
- **REST, from the signed-in page.** `POST /v1/services/srv-dbq4p4a34ugc73cu1tig/custom-domains {"name":"*.qa98wild.example.com"}` → `400 {"code":"CUSTOM_DOMAIN_INVALID","error":"wildcard hostnames are not allowed: \"*.qa98wild.example.com\"",…,"params":{"hostname":"*.qa98wild.example.com"}}`.
- **Nothing was created.** `GET …/custom-domains` → `[]`.

### Root cause

- **The backend refuses it.** `canonicalHostname` (`lego/backend/internal/apps/domains.go:300-302`) rejects any host containing `*`. Its comment (`:290-297`) argues a wildcard and a concrete host beneath it would both become live TLS Ingress rules, letting one tenant's wildcard capture another's host. It also says "Render offers no tenant wildcard custom domain, so nothing legitimate needs one".
- **The two records disagree.** ADR005's named divergences say the opposite of that comment (`docs/ADR005-custom-domain.md:95`): "Render supports tenant wildcards behind extra `_acme-challenge` / `_cf-custom-hostname` machinery; bex still rejects them … Revisit only as its own decision".
- **No sign-off.** ADR018's custom-domains row calls the refusal one of the "deliberate Bex security supersets" (`docs/ADR018-render-parity.md:169`) without the user's sign-off. The parity rule (AGENTS.md, ADR008 § Render parity first, 2026-10-17) lets a divergence stand only with that sign-off, and says not to park parity work on an optional decision.
- **The dashboard mirrors the refusal** in `custom-domain-name.ts:29-34` and `custom-domains-section.tsx:697`.
- **The certificate path can't issue one.** Tenant certificates are issued by HTTP-01 (ADR005 § Projection and certificates), which can't issue a wildcard. The only DNS-01 issuer bex runs is for its own `*.onbex.co` through Cloudflare (ADR093).

### Fix (target behavior)

- **Add.** A wildcard domain `*.<registrable or sub domain>` is accepted on REST `POST …/custom-domains`, GraphQL and MCP. It answers Render's `201` array.
- **DNS instructions** are Render's for a wildcard (t001 captures the exact records):
  - a CNAME for `*` to the service's `<slug>.onbex.co`;
  - the `_acme-challenge` delegation record for the certificate.
- **Verification** works like other domains: verify, then the background sweep.
- **TLS** is issued by DNS-01 through the tenant's delegated `_acme-challenge` CNAME to a bex-controlled zone. HTTP-01 stays for concrete hosts.
- **Routing.**
  - Every `x.<domain>` reaches the service, and a concrete custom domain under the wildcard keeps going to its own service.
  - Collisions follow Render's rules, which t001 captures. At least: adding a wildcard that covers another workspace's concrete or wildcard domain is refused, which keeps the hijack concern the comment names.
- **Unchanged refusals:** reserved platform hosts (`*.onbex.co` and its subdomains), bare public suffixes (`*.co.uk`), and multi-level wildcards (`*.*.example.com`).

### Blast radius

- **Host validation.** `canonicalHostname` is the shared validator. Run `grep -n "canonicalHostname(" lego/backend/internal/apps` for every add, verify and Blueprint `domains:` path; t005 lists each one.
- **Projection and certificates:** the operator's per-host Ingress and TLS projection.
- **Dashboard:** the custom-domain dialog and DNS panel.
- **The www/apex pairing** (ADR005) must not pair a wildcard.

### Unverified

- Render's exact wildcard DNS records, plan availability and collision rules, read from render.com/docs/custom-domains. ADR005:95 is the only in-repo statement of them. t001 captures them.
- The DNS-01 delegation mechanics against Cloudflare, or whichever zone bex uses to receive the `_acme-challenge` CNAMEs.

### Render

- **ADR005:95:** "Render supports tenant wildcards behind extra `_acme-challenge` / `_cf-custom-hostname` machinery".

### Dedupe

- **w4/353 (done)** and **w21/048 (done)** list the wildcard refusal as a correct refusal of their time.
- **w5/m121 (done)** called the reserved-host and wildcard refusals supersets.
- **ADR093** covers only bex's own `*.onbex.co` certificate.
- No open item proposes tenant wildcard domains: `grep -rli wildcard .pm`, open items.
- **Not deploy lag:** `domains.go:300` is unchanged on `origin/main`.

## Definition of done

Signed in to `bex-canary` after the release pins this build, on an owned Free web service and a test zone bex controls (gate: t006):

1. `POST …/custom-domains {"name":"*.qa-<date>.<test zone>"}` returns 201, and the dashboard dialog accepts the same name and shows the wildcard CNAME and the `_acme-challenge` record.
2. With both records published, the domain reads `verified`, the certificate issues, and `curl https://anything.qa-<date>.<test zone>/` and `https://other.qa-<date>.<test zone>/` both reach the service with a valid certificate.
3. A concrete domain `api.qa-<date>.<test zone>` on a second owned service keeps routing to that service.
4. Still refused: `*.qa-<date>.onbex.co`, `*.co.uk` and `*.*.<test zone>`.
5. Removing the wildcard stops routing for the names under it.

## Source + Goal linkage

- **Source:** the finding above (round 98 of the `/qa-find-bugs w4` loop, 2026-11-11). The durable evidence is the probe record.
- **Goal linkage:**
  - [ADR005](../../../docs/ADR005-custom-domain.md) (custom domains) and [ADR093](../../../docs/ADR093-onbex-wildcard-tls.md) (DNS-01 precedent).
  - [ADR018](../../../docs/ADR018-render-parity.md) custom-domains row.
  - [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** a multi-tenant app moves from Render to bex with its wildcard domain intact.
- **Why now:** the divergence has no sign-off, and the code comment that justifies it misstates Render, so the next reader would wrongly treat it as settled.
- **Render parity included (t007):** REST, GraphQL, MCP and the dashboard change.
