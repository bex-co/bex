# w21 · m394 — Security review round 25: the operator's tenant-facing daemons and the dashboard's server routes

**Worker:** worker21 **Goal:** every operator daemon that parses tenant-controlled input and every dashboard server route has been reviewed with the `security-audit` skill, each confirmed finding is fixed with a regression test that fails on the old code, and ADR109 records scope, findings and accepted residuals. **Status:** todo (t001-t004 and t009-t017 done: the audit ran, every finding has a verdict and every fix is shipped; t005 (ADR109), t018 (Render parity), t006-t008 open)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Scope the operator plane and run the audit on its daemons — **DONE** | 90m | — |
| [t002](done/t002.md) | Run the audit on the dashboard's server routes and its OAuth consent and device flows — **DONE** | 60m | — |
| [t003](done/t003.md) | Reproduce each finding and reject what does not hold — **DONE** | 60m | t001, t002 |
| [t004](done/t004.md) | Fix R25-08: an unmatched `/assets/**` path answers 404, never the visitor's page with a public immutable header — **DONE** | 45m | t003 |
| [t009](done/t009.md) | Fix R25-05: one client cannot occupy the static server's origin-fetch slots of other tenants' sites — **DONE** | 60m | t003 |
| [t010](done/t010.md) | Fix R25-03: the operator accepts a registry bearer realm only if it is https, or http on an exact loopback host — **DONE** | 30m | t003 |
| [t011](done/t011.md) | Fix R25-01: the activator routes only the hosts an App's spec claims, whatever its phase — **DONE** | 45m | t003 |
| [t012](done/t012.md) | Fix R25-02 and R25-11: the activator's hold slot covers only the wait, and a busy maintenance fetch falls back at once — **DONE** | 45m | t003 |
| [t013](done/t013.md) | Fix R25-10: an unverified custom-domain claim cannot hold a hostname past 48 hours by deleting and re-adding it — **DONE** | 75m | t003 |
| [t014](done/t014.md) | Fix R25-04: a stalling tenant registry cannot occupy the operator's reconcile workers — **DONE** | 60m | t003 |
| [t015](done/t015.md) | Fix R25-06 and R25-07: the edge cache charges every entry's real size, and the app behind it keeps the client's forwarding headers — **DONE** | 60m | t003 |
| [t016](done/t016.md) | Fix R25-09: a link with an embedded tab, newline or leading control character renders as external — **DONE** | 20m | t003 |
| [t017](done/t017.md) | Fix the small hardening items the audit found: a deadline on the pg SSLRequest reply, `?lang=%`, and an IPv6 /64 per-source key — **DONE** | 45m | t003 |
| [t005](t005.md) | Write ADR109 and update the review lineage | 45m | t004, t009, t010, t011, t012, t013, t014, t015, t016, t017 |
| [t018](t018.md) | Render parity | 20m | t004, t009, t010, t011, t012, t013, t014, t015, t016, t017 |
| [t006](t006.md) | Simplify | 15m | t005, t018 |
| [t007](t007.md) | Test coverage | 20m | t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Findings

Round 25 ran the `security-audit` skill at `cc671cbfd` (run 1; output outside the repository in `~/security-audit-skill/bex-w21/run-1/`: `REPORT.md`, `FINDINGS-DETAIL.md`, `findings.json`, the two inventory tables, raw hunter and validator reports). Nine hunters, seven adversarial validators and ten independent verifiers; ten findings confirmed (one Medium, nine Low), none rejected by the verifiers, nine of ten corrected for lines or wording.

| ID | Severity | Verdict | Reproduction | Fix task |
| --- | --- | --- | --- | --- |
| R25-08 | Medium | confirmed (in-repo half; Cloudflare half needs deployment testing) | real production bundle: `/assets/<missing>.css` with a session cookie returns the SSR page with `public, max-age=31536000, immutable` and the session JSON | t004 |
| R25-05 | Low | confirmed | real handler, fake S3: 0-3 attacker sites victim 300x200; 4-8 sites 300x503 | t009 |
| R25-03 | Low | confirmed; not the ADR061/063/064 residual | production transport: internal listener receives the operator's plaintext GET | t010 |
| R25-01 | Low | confirmed (custom-host middleware bypass rejected) | repo's lifecycle fixture: `Status.URLs` keeps the dropped host | t011 |
| R25-02 | Low | confirmed | real handler: 256 slots pinned, victim shed in 1 ms | t012 |
| R25-10 | Low | confirmed (apex auto-verify is accepted by ADR005) | real Postgres 16: delete and re-add resets the bound; Blueprint path never reclaims | t013 |
| R25-04 | Low | confirmed (delay figures are a simulation) | production client: worker held 5 s per attempt | t014 |
| R25-06 | Low | confirmed | real binary: 250 MiB RSS at 250K unique 404s, limit 160Mi | t015 |
| R25-07 | Low | confirmed | Traefik-shaped request: app sees the Traefik IP and `http` | t015 |
| R25-09 | Low | confirmed | real `MarkdownRenderer` in jsdom: raw-HTML link renders internal, resolves off-origin | t016 |
| R25-11 | hardening | confirmed as hardening | 16 slow URLs delay a victim 4 s | t012 |
| R25-13 | hardening | confirmed as hardening | mute backend pins slots | t017 |
| R25-15 | informational | confirmed bug | `?lang=%25` returns 500 | t017 |
| R25-12 | residual | the global-cap mechanism is real and is ADR066's accepted 'edge/LB rate limiting' residual; its premise is false | real `pg-sni-proxy`: legit 25/25 to 1/74 | IPv6 /64 in t017; rest filed by t005 |
| R25-14 | hardening | shared age key and S3 pair readable by the bex-api token; needs a prior token compromise | manifests read; envtest forged-snapshot run | filed by t005 |

Rejected claims: R25-01 'Attack B' for custom hosts (no route reaches the activator); R25-12 silent single-source variant and CPU variant. Accepted residuals not refiled: `#PSL`, `#CI-RUNNERS`, `#RUNNER-HOSTS`, `#ENV-REVIEWERS`, `#APPROVAL-MODE`; the edge-cache deception inside one tenant is Render's behaviour (ADR018).

Provable only on a live deployment: Cloudflare caching of the personalised `/assets/**` page; whether the Hetzner LB sends the PROXY header at connect; Hydra open dynamic client registration without `disallow_private_ip_ranges`; the activator's maintenance fetch to its own node's public IP (ADR103 #10's 'no Hetzner firewall' premise looks stale); Kratos rejecting `return_to=https://evil.com/`; BuildKit with a symlinked context root; Wasabi's handling of `..` in S3 keys; Traefik's `Host()` canonicalisation and `X-Forwarded-*` sanitising; a live probe from a tenant pod to `pg-sni-proxy:5432`; the Argo CD controller token through `pods/exec`; `openstatus.onbex.co` CSRF for same-site requests.

Policy note (not a vulnerability): `hydra-consent.ts` and `hydra-device.ts` implement a human consent screen and device bridge inside the dashboard, beyond '.pm/DO_NOT_DO.md: at most a headless consent acceptor'.

## Definition of done

1. `docs/ADR<next>-security-review-round25.md` exists with a scope table that gives a verdict for each `lego/operator/cmd/*` binary and each dashboard server route named in t001 and t002, and `bash scripts/adr-registry-validate.sh` passes.
2. Every confirmed finding has a regression test that fails on the pre-fix code and passes after the fix.
3. `make test` and `make lint` (from `lego/operator/`), `cd lego/backend && go test ./...` and `cd dashboard && yarn lint && yarn test` pass.
4. The accepted residuals (`#PSL`, `#CI-RUNNERS`, `#RUNNER-HOSTS`, `#ENV-REVIEWERS`, `#APPROVAL-MODE`) are reported as accepted, not refiled; anything provable only on a live deployment is listed with the check that settles it.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w21` (2026-11-14), filed by `/pmx them all to w21` on 2026-11-14, re-checked at filing against `origin/main` `cc671cbfd`, item 1. ADR107 § Residuals (Coverage) names the operator and the dashboard's server-side routes as the areas round 24 covered least; no board item picks that up. A search of the 24 review ADRs finds `kv-internal-proxy`, `egress-meter`, `auth.consent`, `auth.device` and `api.sessions` named in none.
- **Goal linkage:** V0 roadmap goal 7 (security review) and the review lineage ADR028 → ADR107 in `AGENTS.md`.
- **Expected outcome:** Every surface above has a recorded verdict, and each exploitable issue found is fixed under a test that would catch its return.
- **Why now:** ADR107 says a single run finds roughly half of what repeated runs find and asks the next one to weight these areas; round 24 closed on 2026-11-13, so the findings and the method are fresh. The operator daemons are the public-facing parsers of tenant-controlled input (SNI, HTTP, static files) and `kv-internal-proxy` is newer than every review that touched Key Value.
- **Render parity task:** included (t018), added when t003 found fixes that change tenant-visible behavior. It was omitted at filing. This is security hardening of operator and dashboard-server internals. t013 (custom-domain refusals), t015 (forwarding headers), t009 (overload 503) and t011 (a dropped host) change tenant-visible behavior, so t018 checks them against Render.

## Open decisions

None. A finding that would need a new product surface (for example an approval flow) is filed as a finding with the rule in `.pm/DO_NOT_DO.md` it runs into, not built here.
