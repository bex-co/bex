# w5 · m137 — Custom domains match Render exactly: traffic DNS is the only setup step

**Worker:** worker5 **Goal:** a tenant adds a domain, creates the one traffic record Render asks for, and verification + TLS happen automatically — no `_bex-challenge` TXT, no per-domain manual re-checks. **Status:** done

## Tasks (in order)

| id   | title                                                                     | est | depends_on                   |
| ---- | ------------------------------------------------------------------------- | --- | ---------------------------- |
| t001 | Traffic-DNS predicate replaces the TXT verifier in the core — **DONE** | 45m | —                            |
| t002 | Claim lifecycle without a challenge (store, migration, storeless mode) — **DONE** | 60m | t001                         |
| t003 | Automatic verification sweep; manual Verify shares the promote path — **DONE** | 60m | t002                         |
| t004 | REST adopts Render's wire: `unverified\|verified`, add array, verify `202` — **DONE** | 45m | t002                         |
| t005 | GraphQL + MCP converge on the same semantics; ownership fields deleted — **DONE** | 45m | t002                         |
| t006 | Dashboard: Render-shaped domain setup, one traffic record per domain — **DONE** | 60m | t005                         |
| t007 | Dangling-DNS residual: evaluate slug non-reuse, stay invisible — **DONE** | 45m | t002                         |
| t008 | Render parity — **DONE**                                                  | 30m | t003, t004, t005, t006, t007 |
| t009 | Simplify — **DONE**                                                       | 20m | t008                         |
| t010 | Test coverage — **DONE**                                                  | 45m | t008, t009                   |
| t011 | Closeout — **DONE**                                                       | 10m | t010                         |

## Definition of done

- Adding `beancount.io` shows exactly two DNS instructions in total — `ALIAS @ → <slug>.onbex.co` and `CNAME www → <slug>.onbex.co` — on the dashboard, and no surface (REST, GraphQL, MCP) mentions a TXT, `_bex-challenge`, `ownershipStatus`, or `ownershipDnsRecord` for an ordinary domain.
- Once those records propagate, both domains flip to verified with no click (one sweep interval); manual Verify still works and REST answers `202` with no body.
- REST `verificationStatus` is `unverified | verified` meaning traffic-DNS verification; REST add returns `201` with the created pair as an array the pinned Render CLI client decodes.
- An unverified host still never appears in `App.spec.host(s)`, any Ingress, or any Certificate input (the ADR045 gate, unchanged); a verified host projects and starts TLS as today.
- Rollout withdraws nothing: domains verified under the TXT regime keep serving; a claim still pending on a TXT verifies from its traffic record and its published TXT becomes inert.
- A Cloudflare-proxied record stays unverified behind DNS-only guidance (Render's Cloudflare rule); the same record DNS-only verifies, including a flattened apex.
- The dangling-DNS/slug-reuse residual is either closed invisibly or recorded as accepted in ADR005 with the reason — never by adding a tenant step.

## Source + Goal linkage

- **Source:** user decision 2026-10-19, from the live setup for `srv-dbavjsm05jqs73cjk3og` (beancount.io on dashboard.bex.co): one typed domain produced four record groups — two `_bex-challenge` TXT values on one shared host with keep-the-sibling warnings, plus ALIAS + CNAME, each manually re-checked — where Render asks for the two traffic records and verifies automatically. [ADR005](../../../../docs/ADR005-custom-domain.md) was amended the same day, before this milestone was filed: Render parity on the tenant-facing domain flow is a hard constraint, the m84 random-TXT "deliberate security extension" is withdrawn, and the residuals of dropping it are accepted there by name. Render's contract was rechecked 2026-10-19 against render.com/docs/custom-domains, the provider DNS guides (apex ALIAS/ANAME or `A 216.24.57.1`; Cloudflare DNS-only until verified), and the public OpenAPI (`verificationStatus: unverified|verified`, add `201` array, verify `202` no body); see [the pinned comparison](../../../../docs/render-artifacts/custom-domain-dns-instructions.md).
- **Goal linkage:** ADR008's Render-alternative core and the ADR018 parity ledger — custom domains are a first-run tenant surface where bex currently diverges in the tenant's face, against bex-api's Render-compatible contract (ADR006).
- **Expected outcome:** migrating a domain from Render (beancount.io's `www` still CNAMEs to `onrender.com` today) becomes the identical two-record, zero-extra-step flow on bex; the dashboard's domain panel shrinks to Render's shape; ADR018's custom-domain rows lose their tenant-visible divergences.
- **Why now:** the constraint was decided before filing — ADR first, then this milestone — and real first-party domains are mid-migration now. Every week the TXT flow stays, it is the first thing a migrating tenant hits.
- **Render parity included:** this milestone _is_ a parity change on REST, GraphQL, MCP, and the dashboard, so the standing parity task (t008) re-verifies all four surfaces against the pinned Render contract after the implementation tasks land.

## Evidence (2026-10-19)

- **DoD, item by item.** Dashboard add for a pair renders exactly the two traffic records and no TXT text anywhere (component test + dev-5 screenshot `.playwright-mcp/m137-domain-added.png`); a pointed claim promotes with no click (`TestDomainVerificationSweepPromotesWithoutAClick`, plus `verification_attempts=2` — one manual, one sweep — in the dev-5 database from the shipped binary); REST add `201` array and verify `202` are pinned by `TestRESTCustomDomains(PairedAdd)` and the OpenAPI conformance suite now needs **no** custom-domain allowlist entry (the vendored Render schema validates bex's response, including `publicSuffix`/`createdAt`/`redirectForName`); the unverified-never-routes gate holds in tests and on dev-5 (App CR `spec.hosts` empty while pending); rollout safety pinned by real-Postgres claim tests (verified rows untouched, pending TXT-era rows verify from traffic); Cloudflare-anycast negative + flattened-apex positive are `TestTrafficPredicate` cases; the dangling-DNS residual is recorded as accepted in ADR005 with the slug-tombstone rejection reason (t007).
- **Suites.** Backend full `go test -p 1 ./...` green on fresh real deps (Postgres/OpenFGA via `scripts/backend-test-deps.sh`); `make lint` 0 issues across all four modules; dashboard vitest 544 files / 5188 tests green, both `tsc` projects clean, knip clean. Four mutation classes (CNAME branch off, inverted address intersection, unconditional promotion, single-object add) each fail named tests. Dashboard eslint still cannot start in this checkout (pre-existing plugin config crash, reproduces on untouched files).
- **Environment note.** The local CAPD cluster was down (node `bex-9mp82-pk6xp` had lost `/etc/kubernetes/pki/apiserver.crt`); recovered by re-signing the apiserver cert from the in-node CA (SANs incl. the LB address) and restarting kubelets — dev-5 was then rebuilt by the harness as usual. No other workstream's stack was touched.
